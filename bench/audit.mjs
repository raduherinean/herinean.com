#!/usr/bin/env node
// Audits one served site against the scorecard rows and writes them as JSON (the shape site scorecard reads).
// usage: node bench/audit.mjs BASE --mode ci|post [--only lighthouse|checks] [--form-factor mobile|desktop] [--shard i/n] [--dist DIR] [--link URL] [--link-text T] --out FILE
import { writeFileSync } from "node:fs";
import { parseArgs } from "node:util";
import { loadPages } from "./lib/pages.mjs";
import { today } from "./lib/row.mjs";
import { makeBrowser } from "./lib/browser.mjs";

const { values: a, positionals } = parseArgs({ allowPositionals: true, options: {
  mode: { type: "string", default: "ci" }, only: { type: "string" }, "form-factor": { type: "string" }, shard: { type: "string", default: "1/1" },
  dist: { type: "string" }, link: { type: "string" }, "link-text": { type: "string" }, out: { type: "string" } } });
const base = (positionals[0] || "").replace(/\/$/, "");
if (!base || !a.out) { console.error("usage: audit.mjs BASE --out FILE [--mode ci|post] …"); process.exit(2); }

// Every module under bench/lib/ that produces a row; a module is listed here only once it exists,
// so `audit.mjs` never fails on an import of a row that has not been written yet.
const MODULES = {
  checks: ["i18n", "social", "wellknown", "feeds", "jsonld", "headers", "privacy", "weight", "links", "html", "a11y", "fonts", "transport", "observatory", "dns", "caching"],
  lighthouse: ["lighthouse"],
};
const wanted = a.only ? MODULES[a.only] : [...MODULES.checks, ...MODULES.lighthouse];
const ctx = { base, mode: a.mode, dist: a.dist, when: today(), link: a.link, linkText: a["link-text"], formFactor: a["form-factor"], shard: a.shard, browser: makeBrowser() };
ctx.pages = await loadPages(base, { include404: true });
// A sitemap page that does not answer 200 (or a /404.html that does not answer 404) means the served
// build is not the one the sitemap describes; every row would then measure the wrong thing, so the
// audit stops here, before any module runs. The job fails, and merge.mjs --expect marks every row
// of this mode "not measured" rather than letting a partial set read as an audit.
const misserved = ctx.pages.filter((p) => (p.path === "/404.html" ? p.status !== 404 : p.status !== 200)).map((p) => `${p.path} → ${p.status}`);
if (misserved.length) throw new Error(`pages not served as the sitemap says (want 200, /404.html 404): ${misserved.join(", ")}`);
const rows = [], detail = {};
for (const m of wanted) {
  const mod = await import(`./lib/${m}.mjs`);
  if (!mod.modes.includes(ctx.mode)) continue;
  const t0 = Date.now();
  try {
    for (const r of await mod.run(ctx)) { detail[r.check] = r.detail; delete r.detail; rows.push(r); console.error(`${r.pass ? "ok  " : "FAIL"} ${r.check}: ${r.value} (${Date.now() - t0} ms)`); }
  } catch (e) {
    rows.push({ check: mod.name, pass: false, value: `bench error: ${e.message}`, when: ctx.when, link: ctx.link || "", link_text: ctx.link ? "CI run" : "" });
    console.error(`FAIL ${mod.name}: ${e.stack}`);
  }
}
await ctx.browser.close();
writeFileSync(a.out, JSON.stringify(rows, null, 1) + "\n");
writeFileSync(a.out + ".detail.json", JSON.stringify(detail, null, 1) + "\n");
