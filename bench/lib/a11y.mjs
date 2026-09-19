import AxeBuilder from "@axe-core/playwright";
import { row, summary } from "./row.mjs";
export const name = "Accessibility";
export const modes = ["ci", "post"];
const TAGS = ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"];
// Secondary text is set in --ink-2 (site.css): it must pass AA (4.5:1); everything else must pass AAA (7:1). Keep in step with site.css.
export const SECONDARY = [".tagline", ".meta", ".label", ".badge", ".entry p", ".chroma .c", ".chroma .c1", ".chroma .cm", ".chroma .cp", ".chroma .cs", ".chroma .ch", ".chroma .s", ".chroma .s1", ".chroma .s2", ".chroma .sb", ".chroma .sd", ".chroma .sh", ".chroma .sx", ".chroma .sr", ".chroma .dl", "th", "figcaption", ".footnotes", ".also", ".byline", "footer"];
// Row 3: axe WCAG 2.2 AA in light and dark; AAA contrast outside the secondary selectors; skip link first; visible focus; tab stops = interactive elements; the language link carries lang.
export async function run(ctx) {
  const problems = [];
  const pages = ctx.mode === "post" ? ctx.pages.filter((p) => p.path === "/colophon/") : ctx.pages;
  const check = ctx.mode === "post" ? `${name} (production)` : name;
  if (pages.length === 0) return [row(check, false, ctx.mode === "post" ? "no /colophon/ page to audit" : "no pages to audit", ctx)];
  const browser = await ctx.browser();
  for (const scheme of ["light", "dark"]) {
    const bctx = await browser.newContext({ colorScheme: scheme });
    try {
      for (const p of pages) {
        const page = await bctx.newPage();
        try {
          await page.goto(ctx.base + p.path, { waitUntil: "load" });
          const aa = await new AxeBuilder({ page }).withTags(TAGS).analyze();
          for (const v of aa.violations) problems.push(`${scheme} ${p.path}: ${v.id} (${v.nodes.length}) ${v.help}`);
          let aaa = new AxeBuilder({ page }).withRules(["color-contrast-enhanced"]);
          for (const s of SECONDARY) aaa = aaa.exclude(s);
          for (const v of (await aaa.analyze()).violations) problems.push(`${scheme} ${p.path}: AAA contrast: ${v.nodes.slice(0, 2).map((n) => n.target.join(" ")).join(", ")}`);
          if (scheme === "light" && p.path !== "/404.html") {
            await page.keyboard.press("Tab");
            const first = await page.evaluate(() => ({ cls: document.activeElement.className, href: document.activeElement.getAttribute("href") }));
            if (!first.cls.includes("skip") || first.href !== "#main") problems.push(`${p.path}: first tab stop is not the skip link (${first.cls} ${first.href})`);
            const stops = await page.evaluate(async () => {
              const interactive = document.querySelectorAll("a[href], button, input, select, textarea, summary, [tabindex]:not([tabindex='-1'])").length;
              return { interactive };
            });
            let n = 0, prev = null, noOutline = [];
            for (let i = 0; i < stops.interactive + 2; i++) {
              const cur = await page.evaluate(() => { const e = document.activeElement; if (!e || e === document.body) return null; const cs = getComputedStyle(e); return { tag: e.tagName, text: (e.textContent || "").trim().slice(0, 20), outline: cs.outlineStyle !== "none" && parseFloat(cs.outlineWidth) > 0, boxShadow: cs.boxShadow !== "none" }; });
              if (!cur || JSON.stringify(cur) === JSON.stringify(prev)) break;
              n++; prev = cur;
              if (!cur.outline && !cur.boxShadow) noOutline.push(`${cur.tag} ${cur.text}`);
              await page.keyboard.press("Tab");
            }
            if (n < stops.interactive) problems.push(`${p.path}: ${n} tab stops for ${stops.interactive} interactive elements`);
            if (noOutline.length) problems.push(`${p.path}: no visible focus on ${noOutline.slice(0, 2).join(", ")}`);
            const langLink = p.$(".switch a, a[hreflang]").first();
            if (langLink.length && !langLink.attr("lang")) problems.push(`${p.path}: language link lacks lang`);
          }
        } finally {
          await page.close();
        }
      }
    } finally {
      await bctx.close();
    }
  }
  return [row(check, problems.length === 0, summary(`${pages.length} pages × light + dark: WCAG 2.2 AA 0 violations; AAA contrast outside ${SECONDARY.length} secondary selectors; skip link, focus, keyboard`, problems), ctx, problems)];
}
