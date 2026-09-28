import { row, summary } from "./row.mjs";
export const name = "Font correctness";
export const modes = ["ci"];
// Row 21: three contexts per page × viewport. `fast` (no interception) is the real web-font
// render — proved by an M1b-style width probe, not `document.fonts.check()`: font-display:
// optional commits to the fallback once its ~100 ms block period elapses, so `check()` (which
// reports load state, not what got painted) can read true on a page that never painted the web
// face. `delayed` (/fonts/** +600 ms) carries the CLS claim: even arriving that late, `optional`
// means the swap never happens and nothing shifts. `blocked` (/fonts/** aborted) is the
// metric-matched fallback render, compared against `fast` for page height (≤ 5 %) and paragraph
// line counts (±1). Glyph coverage (U+0218–021B, ă â î) is enforced by `site check` on the shipped faces.
const PROBE_TEXT = "Cărturești finanțate rapidă";

// A hidden span set in the web face vs. plain monospace: a >5 px width difference means the web
// face actually painted (monospace's fixed advance widths are nothing like a proportional serif's).
async function measureApplied(page) {
  return page.evaluate(async (text) => {
    await document.fonts.ready;
    const span = document.createElement("span");
    span.style.cssText = "position:absolute;visibility:hidden;white-space:nowrap;font-size:19px;font-weight:400";
    span.textContent = text;
    document.body.appendChild(span);
    span.style.fontFamily = '"Herinean Serif", monospace';
    const webWidth = span.getBoundingClientRect().width;
    span.style.fontFamily = "monospace";
    const monoWidth = span.getBoundingClientRect().width;
    span.remove();
    return Math.abs(webWidth - monoWidth) > 5;
  }, PROBE_TEXT);
}

// A paragraph's line count is its box height over its computed line-height. `line-height: normal`
// computes to the keyword, not a length, so parseFloat gives NaN and the division would report
// NaN lines — which `Math.abs(NaN - n) > 1` reads as "within one line", i.e. a silent pass. Such a
// paragraph is reported as unmeasurable (`null`) and run() turns it into a problem, rather than
// estimated from fontSize × 1.2: `normal` is font-dependent (about 1.15–1.3), and a guessed
// denominator would make the ±1-line claim rest on a guess. The site's CSS sets a line-height on
// body, so this only fires on a page that lost it.
const measureLayout = (page) => page.evaluate(() => ({
  h: document.documentElement.scrollHeight,
  lines: [...document.querySelectorAll("main p")].map((e) => { const lh = parseFloat(getComputedStyle(e).lineHeight); return Number.isFinite(lh) && lh > 0 ? Math.round(e.getBoundingClientRect().height / lh) : null; }),
}));

export async function run(ctx) {
  const pages = ctx.pages.filter((p) => p.path !== "/404.html");
  if (pages.length === 0) return [row(name, false, "no pages to audit", ctx)];
  const problems = [], measured = [];
  const browser = await ctx.browser();
  for (const vp of [{ width: 390, height: 844 }, { width: 1280, height: 800 }]) {
    // One set of contexts per viewport, shared across pages, is deliberate here (unlike weight.mjs):
    // Playwright routing disables the HTTP cache, so `delayed` and `blocked` are cold on every page,
    // and `fast` measures whether the web font rendered, which holds whether or not it came from cache.
    const fast = await browser.newContext({ viewport: vp });
    const delayed = await browser.newContext({ viewport: vp });
    const blocked = await browser.newContext({ viewport: vp });
    try {
      await delayed.route("**/fonts/**", async (r) => { await new Promise((z) => setTimeout(z, 600)); await r.continue(); });
      await blocked.route("**/fonts/**", (r) => r.abort());
      for (const p of pages) {
        const a = await fast.newPage();
        const d = await delayed.newPage();
        const b = await blocked.newPage();
        try {
          await a.goto(ctx.base + p.path, { waitUntil: "load" });
          const applied = await measureApplied(a);
          const web = await measureLayout(a);

          await d.addInitScript(() => { window.__cls = 0; new PerformanceObserver((l) => { for (const e of l.getEntries()) if (!e.hadRecentInput) window.__cls += e.value; }).observe({ type: "layout-shift", buffered: true }); });
          await d.goto(ctx.base + p.path, { waitUntil: "load" });
          await d.waitForTimeout(1500);
          const cls = +(await d.evaluate(() => window.__cls)).toFixed(4);

          await b.goto(ctx.base + p.path, { waitUntil: "load" });
          const fb = await measureLayout(b);

          if (cls > 0) problems.push(`${vp.width} ${p.path}: CLS ${cls}`);
          if (!applied) problems.push(`${vp.width} ${p.path}: web font not applied`);
          const dh = Math.abs(web.h - fb.h) / fb.h;
          if (dh > 0.05) problems.push(`${vp.width} ${p.path}: height ${web.h} vs ${fb.h} (${(dh * 100).toFixed(1)} %)`);
          web.lines.forEach((l, k) => {
            if (fb.lines[k] === undefined) return;
            if (l === null || fb.lines[k] === null) problems.push(`${vp.width} ${p.path}: paragraph ${k + 1} has line-height: normal, lines not measurable`);
            else if (Math.abs(l - fb.lines[k]) > 1) problems.push(`${vp.width} ${p.path}: paragraph ${k + 1} ${l} vs ${fb.lines[k]} lines`);
          });
          measured.push({ vp: vp.width, path: p.path, cls, height: [web.h, fb.h], applied });
        } finally {
          await a.close(); await d.close(); await b.close();
        }
      }
    } finally {
      await fast.close(); await delayed.close(); await blocked.close();
    }
  }
  const worstH = Math.max(...measured.map((m) => Math.abs(m.height[0] - m.height[1]) / m.height[1]));
  return [row(name, problems.length === 0, summary(`${pages.length} pages × 2 widths: web font painted (width probe); CLS 0 with fonts delayed 600 ms; height Δ ≤ ${(worstH * 100).toFixed(1)} % vs fallback; paragraphs within ±1 line; glyphs U+0218–021B enforced by site check`, problems), ctx, measured)];
}
