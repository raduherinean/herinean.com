import { row, summary } from "./row.mjs";
export const name = "Structured data";
export const modes = ["ci"];
const REQUIRED = {
  WebSite: ["name", "url", "inLanguage"],
  Person: ["name", "url", "sameAs"],
  BlogPosting: ["headline", "inLanguage", "image", "author", "datePublished", "dateModified", "url"],
  BreadcrumbList: ["itemListElement"],
};
// Which types a page must carry, by page type (spec §4.1): the two home pages declare the site and
// its author; a piece (`/writing/<slug>/`, `/ro/articole/<slug>/` — not the section indexes) is a
// BlogPosting with a BreadcrumbList. Checking only the fields of whatever happens to be present would
// pass a piece that lost its BlogPosting block altogether.
const HOME = new Set(["/", "/ro/"]);
const PIECE = /^\/(?:writing|ro\/articole)\/[^/]+\/$/;
const MUST = (path) => (HOME.has(path) ? ["WebSite", "Person"] : PIECE.test(path) ? ["BlogPosting", "BreadcrumbList"] : []);
// Row 6: every ld+json block parses; home pages carry WebSite + Person, pieces BlogPosting + BreadcrumbList; each object of a known type carries its required fields; Person.sameAs has LinkedIn, X and GitHub.
export async function run(ctx) {
  const problems = [];
  let blocks = 0, homes = 0, pieces = 0;
  for (const p of ctx.pages.filter((p) => p.path !== "/404.html")) {
    const scripts = p.$('script[type="application/ld+json"]').toArray();
    if (scripts.length === 0) problems.push(`${p.path}: no JSON-LD`);
    const present = new Set();
    for (const s of scripts) {
      blocks++;
      let data; try { data = JSON.parse(p.$(s).text()); } catch (e) { problems.push(`${p.path}: JSON-LD unparsable: ${e.message}`); continue; }
      const objs = Array.isArray(data["@graph"]) ? data["@graph"] : Array.isArray(data) ? data : [data];
      for (const o of objs) {
        const type = Array.isArray(o["@type"]) ? o["@type"][0] : o["@type"];
        present.add(type);
        const req = REQUIRED[type]; if (!req) continue;
        for (const k of req) if (o[k] === undefined || o[k] === "" ) problems.push(`${p.path}: ${type} lacks ${k}`);
        if (type === "Person") for (const host of ["linkedin.com", "x.com", "github.com"]) if (!(o.sameAs || []).some((u) => u.includes(host))) problems.push(`${p.path}: Person.sameAs lacks ${host}`);
      }
    }
    if (HOME.has(p.path)) homes++; else if (PIECE.test(p.path)) pieces++;
    for (const type of MUST(p.path)) if (!present.has(type)) problems.push(`${p.path}: no ${type}`);
  }
  return [row(name, problems.length === 0, summary(`${blocks} JSON-LD blocks: WebSite + Person (sameAs ×3) on ${homes} home pages, BlogPosting + BreadcrumbList on ${pieces} pieces, required fields present`, problems), ctx, problems)];
}
