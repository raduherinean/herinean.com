import { row, summary } from "./row.mjs";
export const name = "i18n";
export const modes = ["ci"];
// Row 8: <html lang>, symmetric hreflang pairs + x-default, exactly one absolute canonical with a trailing slash.
export async function run(ctx) {
  const problems = [];
  const pages = ctx.pages.filter((p) => p.path !== "/404.html");
  const byPath = new Map(pages.map((p) => [p.path, p]));
  for (const p of pages) {
    const lang = p.$("html").attr("lang");
    const expect = p.path.startsWith("/ro/") ? "ro" : "en";
    if (lang !== expect) problems.push(`${p.path}: lang=${lang} want ${expect}`);
    const canon = p.$('link[rel="canonical"]');
    if (canon.length !== 1) problems.push(`${p.path}: ${canon.length} canonical links`);
    const href = canon.attr("href") || "";
    if (!/^https:\/\/[^/]+\/.*\/$|^https:\/\/[^/]+\/$/.test(href)) problems.push(`${p.path}: canonical ${href} not absolute with a trailing slash`);
    const alts = p.$('link[rel="alternate"][hreflang]').toArray().map((e) => ({ l: p.$(e).attr("hreflang"), h: p.$(e).attr("href") }));
    if (alts.length && !alts.some((a) => a.l === "x-default")) problems.push(`${p.path}: hreflang without x-default`);
    for (const a of alts) {
      if (a.l === "x-default") continue;
      const other = byPath.get(new URL(a.h).pathname);
      if (!other) { problems.push(`${p.path}: hreflang ${a.l} → ${a.h} not in the sitemap`); continue; }
      const back = other.$('link[rel="alternate"][hreflang]').toArray().map((e) => ({ l: other.$(e).attr("hreflang"), h: other.$(e).attr("href") }));
      if (!back.some((b) => b.h === href && b.l === lang)) problems.push(`${p.path}: ${a.h} does not point back with hreflang=${lang}`);
    }
  }
  return [row(name, problems.length === 0, summary(`${pages.length} pages: lang, canonical, hreflang symmetric`, problems), ctx, problems)];
}
