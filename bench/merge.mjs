#!/usr/bin/env node
// Folds row files into scorecard.json in spec order, merges Lighthouse shards, prepends the audited build, gates.
import { readFileSync, writeFileSync } from "node:fs";
import { parseArgs } from "node:util";
import { pathToFileURL } from "node:url";
const { values: a, positionals: files } = parseArgs({ allowPositionals: true, options: { out: { type: "string" }, build: { type: "string" }, "build-url": { type: "string" }, summary: { type: "string" }, "no-gate": { type: "boolean", default: false } } });

export const ORDER = ["Audited build", "Lighthouse", "HTML validity", "Accessibility", "Links", "Feeds", "Structured data", "Social previews", "i18n", "Security headers", "Weight", "Privacy", "Well-known files", "Font correctness",
  "Lighthouse (production)", "HTML validity (production)", "Accessibility (production)", "Security headers (production)", "Transport (production)", "Mozilla HTTP Observatory (production)", "DNS, mail, domains (production)", "Privacy (production)", "Caching (production)"];

export function merge(rowsIn, build) {
  const rows = [];
  const lh = new Map(); // check → shard rows
  for (const r of rowsIn) {
    const m = /^(Lighthouse(?: \(production\))?) \[(mobile|desktop) (\d+)\/(\d+)\]$/.exec(r.check);
    if (m) { (lh.get(m[1]) || lh.set(m[1], []).get(m[1])).push({ ...r, ff: m[2], i: +m[3], n: +m[4] }); continue; }
    rows.push(r);
  }
  for (const [check, shards] of lh) {
    const n = shards[0].n, have = new Set(shards.map((s) => `${s.ff}${s.i}`));
    const missing = ["mobile", "desktop"].flatMap((ff) => Array.from({ length: n }, (_, k) => `${ff}${k + 1}`)).filter((k) => !have.has(k));
    const pass = missing.length === 0 && shards.every((s) => s.pass);
    const pages = shards.reduce((t, s) => t + (s.pages || 0), 0) / 2;
    const worst = Math.min(...shards.map((s) => s.worst ?? 100));
    const failing = shards.filter((s) => !s.pass).map((s) => s.value);
    const value = missing.length ? `${missing.length} shards missing (${missing.join(", ")})` : pass ? `${pages} pages × mobile + desktop, 3-run median: all 100 (worst ${worst})` : failing.join("; ");
    rows.push({ check, pass, value, when: shards[0].when, link: shards[0].link, link_text: shards[0].link_text });
  }
  if (build) rows.unshift({ check: "Audited build", pass: true, value: build.sha, when: rows[0]?.when || new Date().toISOString().slice(0, 10), link: build.url, link_text: "commit" });
  const idx = (c) => { const i = ORDER.indexOf(c); return i < 0 ? ORDER.length : i; };
  rows.sort((x, y) => idx(x.check) - idx(y.check));
  return rows;
}

export const table = (rows) => ["| Check | Result | Value |", "|---|---|---|", ...rows.map((r) => `| ${r.check} | ${r.pass ? "✅ pass" : "❌ FAIL"} | ${r.value.replace(/\|/g, "\\|")} |`)].join("\n") + "\n";

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  if (!a.out || files.length === 0) { console.error("usage: merge.mjs --out scorecard.json [--build SHA --build-url URL] [--summary FILE] [--no-gate] rows.json…"); process.exit(2); }
  const rowsIn = files.flatMap((f) => JSON.parse(readFileSync(f, "utf8")));
  const rows = merge(rowsIn, a.build ? { sha: a.build, url: a["build-url"] || "" } : null);
  writeFileSync(a.out, JSON.stringify(rows, null, 1) + "\n");
  if (a.summary) writeFileSync(a.summary, table(rows), { flag: "a" });
  const red = rows.filter((r) => !r.pass);
  console.error(table(rows));
  if (red.length && !a["no-gate"]) { console.error(`${red.length} row(s) red`); process.exit(1); }
}
