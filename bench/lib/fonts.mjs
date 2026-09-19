import { row, summary } from "./row.mjs";
export const name = "Font correctness";
export const modes = ["ci"];
// Row 21: layout-shift observer with /fonts/* delayed 600 ms → CLS 0; page height web vs metric-matched fallback ≤ 5 %;
// every paragraph within ±1 line. Glyph coverage (U+0218–021B, ă â î) is enforced by `site check` on the shipped faces.
export async function run(ctx) {
  const pages = ctx.pages.filter((p) => p.path !== "/404.html");
  if (pages.length === 0) return [row(name, false, "no pages to audit", ctx)];
  const problems = [], measured = [];
  const browser = await ctx.browser();
  for (const vp of [{ width: 390, height: 844 }, { width: 1280, height: 800 }]) {
    const delayed = await browser.newContext({ viewport: vp });
    const blocked = await browser.newContext({ viewport: vp });
    try {
      await delayed.route("**/fonts/**", async (r) => { await new Promise((z) => setTimeout(z, 600)); await r.continue(); });
      await blocked.route("**/fonts/**", (r) => r.abort());
      for (const p of pages) {
        const a = await delayed.newPage();
        const b = await blocked.newPage();
        try {
          await a.addInitScript(() => { window.__cls = 0; new PerformanceObserver((l) => { for (const e of l.getEntries()) if (!e.hadRecentInput) window.__cls += e.value; }).observe({ type: "layout-shift", buffered: true }); });
          await a.goto(ctx.base + p.path, { waitUntil: "load" });
          await a.waitForTimeout(1500);
          const web = await a.evaluate(async () => {
            await document.fonts.ready;
            return {
              cls: +window.__cls.toFixed(4),
              h: document.documentElement.scrollHeight,
              lines: [...document.querySelectorAll("main p")].map((e) => Math.round(e.getBoundingClientRect().height / parseFloat(getComputedStyle(e).lineHeight))),
              applied: document.fonts.check('19px "Herinean Serif"'),
            };
          });
          await b.goto(ctx.base + p.path, { waitUntil: "load" });
          const fb = await b.evaluate(() => ({
            h: document.documentElement.scrollHeight,
            lines: [...document.querySelectorAll("main p")].map((e) => Math.round(e.getBoundingClientRect().height / parseFloat(getComputedStyle(e).lineHeight))),
          }));
          if (web.cls > 0) problems.push(`${vp.width} ${p.path}: CLS ${web.cls}`);
          if (!web.applied) problems.push(`${vp.width} ${p.path}: web font not applied after 1.5 s`);
          const dh = Math.abs(web.h - fb.h) / fb.h;
          if (dh > 0.05) problems.push(`${vp.width} ${p.path}: height ${web.h} vs ${fb.h} (${(dh * 100).toFixed(1)} %)`);
          web.lines.forEach((l, k) => { if (fb.lines[k] !== undefined && Math.abs(l - fb.lines[k]) > 1) problems.push(`${vp.width} ${p.path}: paragraph ${k + 1} ${l} vs ${fb.lines[k]} lines`); });
          measured.push({ vp: vp.width, path: p.path, cls: web.cls, height: [web.h, fb.h] });
        } finally {
          await a.close(); await b.close();
        }
      }
    } finally {
      await delayed.close(); await blocked.close();
    }
  }
  const worstH = Math.max(...measured.map((m) => Math.abs(m.height[0] - m.height[1]) / m.height[1]));
  return [row(name, problems.length === 0, summary(`${pages.length} pages × 2 widths: CLS 0 with fonts delayed 600 ms; height Δ ≤ ${(worstH * 100).toFixed(1)} %; paragraphs within ±1 line; glyphs U+0218–021B enforced by site check`, problems), ctx, measured)];
}
