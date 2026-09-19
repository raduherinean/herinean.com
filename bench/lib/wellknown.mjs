import { row, summary } from "./row.mjs";
export const name = "Well-known files";
export const modes = ["ci"];
const get = async (base, path) => { const r = await fetch(base + path, { redirect: "manual" }); return { status: r.status, type: r.headers.get("content-type") || "", body: r.status === 200 ? await r.text() : "" }; };
// Row 16: security.txt (RFC 9116 fields, Expires ≤ 1 year ahead, still valid), robots.txt with Sitemap, sitemap with hreflang + lastmod, llms.txt, favicons, 404 with a 404 status.
export async function run(ctx) {
  const problems = [];
  const sec = await get(ctx.base, "/.well-known/security.txt");
  if (sec.status !== 200 || !sec.type.startsWith("text/plain")) problems.push(`security.txt: ${sec.status} ${sec.type}`);
  for (const f of ["Contact:", "Expires:", "Preferred-Languages:", "Canonical:"]) if (!sec.body.includes(f)) problems.push(`security.txt: no ${f}`);
  const exp = /Expires:\s*(\S+)/.exec(sec.body)?.[1];
  if (exp) { const d = new Date(exp), now = new Date(); if (isNaN(d)) problems.push("security.txt: Expires unparsable"); else if (d < now) problems.push("security.txt: expired"); else if (d - now > 366 * 864e5) problems.push("security.txt: Expires more than a year ahead"); else if (d - now < 30 * 864e5) problems.push(`security.txt: expires in ${Math.floor((d - now) / 864e5)} days (renew)`); }
  const robots = await get(ctx.base, "/robots.txt");
  if (robots.status !== 200 || !/^Sitemap: https:\/\/\S+\/sitemap\.xml$/m.test(robots.body)) problems.push(`robots.txt: ${robots.status}, no Sitemap line`);
  const sm = await get(ctx.base, "/sitemap.xml");
  if (sm.status !== 200 || !sm.type.startsWith("application/xml")) problems.push(`sitemap.xml: ${sm.status} ${sm.type}`);
  const urls = sm.body.split("<url>").slice(1);
  for (const u of urls) { if (!/<lastmod>\d{4}-\d{2}-\d{2}/.test(u)) problems.push("sitemap: a url lacks lastmod"); if (!/hreflang="x-default"/.test(u)) problems.push("sitemap: a url lacks x-default"); }
  const llms = await get(ctx.base, "/llms.txt");
  if (llms.status !== 200 || !llms.type.startsWith("text/plain")) problems.push(`llms.txt: ${llms.status} ${llms.type}`);
  for (const [path, type] of [["/favicon.svg", "image/svg+xml"], ["/favicon.ico", "image/"], ["/apple-touch-icon.png", "image/png"]]) { const r = await get(ctx.base, path); if (r.status !== 200 || !r.type.startsWith(type)) problems.push(`${path}: ${r.status} ${r.type}`); }
  const nf = await get(ctx.base, "/this-page-does-not-exist/");
  if (nf.status !== 404) problems.push(`unknown path → ${nf.status}, want 404`);
  const home = ctx.pages.find((p) => p.path === "/");
  if (home && home.$('link[rel="icon"][type="image/svg+xml"]').length === 0) problems.push("home: no SVG favicon link");
  return [row(name, problems.length === 0, summary("security.txt, robots.txt, sitemap.xml, llms.txt, favicons, 404", problems), ctx, problems)];
}
