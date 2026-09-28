import { XMLParser, XMLValidator } from "fast-xml-parser";
import { row, summary } from "./row.mjs";
export const name = "Feeds";
export const modes = ["ci"];
// Row 5: RSS 2.0 (all, en, ro) + JSON Feed 1.1 (all): strict XML, required elements, full text, absolute URLs, atom:link rel=self. The W3C validator is a web service — a manual row.
export async function run(ctx) {
  const problems = [];
  const abs = (s) => /^https:\/\//.test(s);
  for (const path of ["/feed.xml", "/feed.en.xml", "/feed.ro.xml"]) {
    const res = await fetch(ctx.base + path);
    const text = await res.text();
    if (!res.ok || !(res.headers.get("content-type") || "").startsWith("application/rss+xml")) problems.push(`${path}: ${res.status} ${res.headers.get("content-type")}`);
    const v = XMLValidator.validate(text);
    if (v !== true) { problems.push(`${path}: ${v.err.msg}`); continue; }
    const x = new XMLParser({ ignoreAttributes: false, attributeNamePrefix: "@" }).parse(text);
    const ch = x.rss?.channel;
    if (!ch || x.rss["@version"] !== "2.0") { problems.push(`${path}: not RSS 2.0`); continue; }
    for (const k of ["title", "link", "description", "language"]) if (!ch[k]) problems.push(`${path}: channel lacks ${k}`);
    const self = ch["atom:link"]; const selfHref = (Array.isArray(self) ? self : [self]).find((l) => l?.["@rel"] === "self")?.["@href"];
    if (!selfHref || new URL(selfHref).pathname !== path) problems.push(`${path}: atom:link rel=self is ${selfHref}`);
    const items = ch.item ? (Array.isArray(ch.item) ? ch.item : [ch.item]) : [];
    for (const it of items) {
      if (!it.guid || !it.pubDate || !abs(String(it.link || ""))) problems.push(`${path}: item lacks guid/pubDate/absolute link`);
      const body = it["content:encoded"] || "";
      if (!body || body.length < 200) problems.push(`${path}: item ${it.link} has no full text`);
      for (const m of body.matchAll(/\s(?:href|src)="([^"]+)"/g)) if (!abs(m[1])) problems.push(`${path}: relative URL in content: ${m[1]}`);
    }
  }
  const jres = await fetch(ctx.base + "/feed.json");
  if (!jres.ok || !(jres.headers.get("content-type") || "").startsWith("application/feed+json")) problems.push(`feed.json: ${jres.status} ${jres.headers.get("content-type")}`);
  let j; try { j = await jres.json(); } catch (e) { problems.push(`feed.json: ${e.message}`); }
  if (j) {
    if (j.version !== "https://jsonfeed.org/version/1.1") problems.push(`feed.json: version ${j.version}`);
    for (const k of ["title", "home_page_url", "feed_url"]) if (typeof j[k] !== "string" || !j[k]) problems.push(`feed.json: ${k} missing`);
    if (!Array.isArray(j.items)) problems.push("feed.json: items is not an array");
    for (const it of j.items || []) {
      for (const k of ["id", "url", "title", "content_html", "date_published"]) if (!it[k]) problems.push(`feed.json: item lacks ${k}`);
      if (it.date_published && isNaN(Date.parse(it.date_published))) problems.push(`feed.json: bad date ${it.date_published}`);
      if (it.url && !abs(it.url)) problems.push(`feed.json: relative url ${it.url}`);
    }
  }
  return [row(name, problems.length === 0, summary("RSS ×3 strict parse, full text, absolute URLs, self link; JSON Feed 1.1 required fields", problems), ctx, problems)];
}
