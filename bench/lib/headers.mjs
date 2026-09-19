import { createHash } from "node:crypto";
import { row, summary } from "./row.mjs";
export const name = "Security headers";
export const modes = ["ci", "post"];
const COMMON = { "x-content-type-options": /^nosniff$/, "referrer-policy": /^strict-origin-when-cross-origin$/, "permissions-policy": /camera=\(\)/, "cross-origin-opener-policy": /^same-origin$/, "x-frame-options": /^DENY$/, "strict-transport-security": /^max-age=63072000; includeSubDomains; preload$/ };
const CLASSES = [
  { path: "/", expect: { ...COMMON, "content-security-policy": /^default-src 'none'; style-src 'sha256-[A-Za-z0-9+/=]+'; img-src 'self'; font-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'$/, "cross-origin-resource-policy": /^same-origin$/, "cache-control": /^public, max-age=0, must-revalidate$/, "content-type": /^text\/html; charset=utf-8$/ } },
  { find: (pages) => pages[0].$('link[rel="preload"][as="font"]').attr("href"), expect: { ...COMMON, "cross-origin-resource-policy": /^same-origin$/, "cache-control": /^public, max-age=31536000, immutable$/, "content-type": /^font\/woff2$/ } },
  { find: (pages) => pages[0].$('meta[property="og:image"]').attr("content") && new URL(pages[0].$('meta[property="og:image"]').attr("content")).pathname, expect: { ...COMMON, "cross-origin-resource-policy": /^cross-origin$/, "cache-control": /^public, max-age=31536000, immutable$/, "content-type": /^image\/png$/ } },
  { find: (pages) => pages.map((p) => p.$("img[src^='/img/']").attr("src")).find(Boolean), expect: { ...COMMON, "cross-origin-resource-policy": /^cross-origin$/, "cache-control": /^public, max-age=31536000, immutable$/ } },
  { path: "/feed.xml", expect: { ...COMMON, "content-type": /^application\/rss\+xml; charset=utf-8$/, "cache-control": /^public, max-age=300$/ } },
  { path: "/feed.json", expect: { ...COMMON, "content-type": /^application\/feed\+json; charset=utf-8$/, "cache-control": /^public, max-age=300$/ } },
  { path: "/sitemap.xml", expect: { ...COMMON, "content-type": /^application\/xml; charset=utf-8$/ } },
  { path: "/robots.txt", expect: { ...COMMON, "content-type": /^text\/plain; charset=utf-8$/ } },
  { path: "/llms.txt", expect: { ...COMMON, "content-type": /^text\/plain; charset=utf-8$/ } },
  { path: "/.well-known/security.txt", expect: { ...COMMON, "content-type": /^text\/plain; charset=utf-8$/ } },
];
// Row 9: the served headers per path class match spec §6.2; the CSP's style hash is the page's own <style>; HSTS everywhere.
export async function run(ctx) {
  const problems = [];
  const pages = ctx.pages.filter((p) => p.path !== "/404.html");
  for (const c of CLASSES) {
    const path = c.path || (c.find && c.find(pages));
    if (!path) { problems.push(`no URL found for a path class (${JSON.stringify(Object.keys(c.expect).slice(0, 2))})`); continue; }
    const res = await fetch(ctx.base + path, { redirect: "manual" });
    if (res.status !== 200) { problems.push(`${path}: ${res.status}`); continue; }
    for (const [h, re] of Object.entries(c.expect)) { const v = res.headers.get(h); if (v === null || !re.test(v)) problems.push(`${path}: ${h} = ${v === null ? "<absent>" : v}`); }
    if (res.headers.has("set-cookie")) problems.push(`${path}: Set-Cookie`);
  }
  for (const p of pages) {
    const csp = p.headers.get("content-security-policy") || "";
    const want = "'sha256-" + createHash("sha256").update(p.$("style").first().html() || "").digest("base64") + "'";
    if (!csp.includes(want)) problems.push(`${p.path}: CSP style hash does not match the inline <style>`);
  }
  const check = ctx.mode === "post" ? `${name} (production)` : name;
  return [row(check, problems.length === 0, summary(`${CLASSES.length} path classes match §6.2; CSP hash = inline style on ${pages.length} pages`, problems), ctx, problems)];
}
