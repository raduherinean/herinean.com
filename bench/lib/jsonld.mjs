import { row, summary } from "./row.mjs";
export const name = "Structured data";
export const modes = ["ci"];
const REQUIRED = {
  WebSite: ["name", "url", "inLanguage"],
  Person: ["name", "url", "sameAs"],
  BlogPosting: ["headline", "inLanguage", "image", "author", "datePublished", "dateModified", "url"],
  BreadcrumbList: ["itemListElement"],
};
// Row 6: every ld+json block parses; each object of a known type carries its required fields; Person.sameAs has LinkedIn, X and GitHub.
export async function run(ctx) {
  const problems = [];
  let blocks = 0;
  for (const p of ctx.pages.filter((p) => p.path !== "/404.html")) {
    const scripts = p.$('script[type="application/ld+json"]').toArray();
    if (scripts.length === 0) problems.push(`${p.path}: no JSON-LD`);
    for (const s of scripts) {
      blocks++;
      let data; try { data = JSON.parse(p.$(s).text()); } catch (e) { problems.push(`${p.path}: JSON-LD unparsable: ${e.message}`); continue; }
      const objs = Array.isArray(data["@graph"]) ? data["@graph"] : Array.isArray(data) ? data : [data];
      for (const o of objs) {
        const type = Array.isArray(o["@type"]) ? o["@type"][0] : o["@type"];
        const req = REQUIRED[type]; if (!req) continue;
        for (const k of req) if (o[k] === undefined || o[k] === "" ) problems.push(`${p.path}: ${type} lacks ${k}`);
        if (type === "Person") for (const host of ["linkedin.com", "x.com", "github.com"]) if (!(o.sameAs || []).some((u) => u.includes(host))) problems.push(`${p.path}: Person.sameAs lacks ${host}`);
      }
    }
  }
  return [row(name, problems.length === 0, summary(`${blocks} JSON-LD blocks: WebSite, Person (sameAs ×3), BlogPosting, BreadcrumbList fields present`, problems), ctx, problems)];
}
