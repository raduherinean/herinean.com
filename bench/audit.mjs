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

// Modules not yet written are kept out of this list until their task lands (see task-5's controller ruling).
const MODULES = {
  checks: ["i18n", "social", "wellknown", "feeds", "jsonld", "headers", "privacy", "weight", "links", "html", "a11y"],
  lighthouse: ["lighthouse"],
};
const wanted = a.only ? MODULES[a.only] : [...MODULES.checks, ...MODULES.lighthouse];
const ctx = { base, mode: a.mode, dist: a.dist, when: today(), link: a.link, linkText: a["link-text"], formFactor: a["form-factor"], shard: a.shard, browser: makeBrowser() };
ctx.pages = await loadPages(base, { include404: true });
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
