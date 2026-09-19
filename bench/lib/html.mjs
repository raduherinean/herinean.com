import { mkdtempSync, writeFileSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
import { HtmlValidate } from "html-validate";
import { row, summary } from "./row.mjs";
import { sh } from "./sh.mjs";
export const name = "HTML validity";
export const modes = ["ci", "post"];
const vnuJar = () => { const rq = createRequire(import.meta.url); return rq("vnu-jar"); }; // the package's main export is the jar's path
// html-validate's constructor takes a config object, not a file path (no `configFile` option on
// `ConfigData` in v11) — read bench/.htmlvalidate.json ourselves so it stays the one source of truth.
const configPath = join(dirname(fileURLToPath(import.meta.url)), "..", ".htmlvalidate.json");
const htmlValidateConfig = JSON.parse(readFileSync(configPath, "utf8"));
// Row 2: Nu checker 0 errors 0 warnings; html-validate (recommended + a11y + document) 0 errors — every page, /404.html included.
export async function run(ctx) {
  const problems = [];
  const pages = ctx.mode === "post" ? ctx.pages.filter((p) => p.path === "/colophon/") : ctx.pages;
  const dir = mkdtempSync(join(tmpdir(), "nu-"));
  try {
    const files = pages.map((p, i) => { const f = join(dir, `${i}.html`); writeFileSync(f, p.html); return [f, p.path]; });
    const nu = sh("java", ["-jar", vnuJar(), "--stdout", "--format", "json", "--exit-zero-always", ...files.map((f) => f[0])]);
    // --exit-zero-always means Nu itself always exits 0; a non-zero code (or a spawn failure) means the
    // checker did not run at all, which must fail loudly rather than read as "0 messages".
    if (nu.error) problems.push(`Nu did not run: ${nu.error.message}`);
    else if (nu.code !== 0) problems.push(`Nu did not run: exit ${nu.code}: ${nu.err.trim().split("\n")[0]}`);
    else {
      const msgs = JSON.parse(nu.out || '{"messages":[]}').messages;
      for (const m of msgs) if (m.type === "error" || (m.type === "info" && m.subType === "warning")) problems.push(`Nu ${m.type}${m.subType ? "/" + m.subType : ""} ${files.find((f) => m.url?.endsWith(f[0]))?.[1] || ""}:${m.lastLine}: ${m.message}`);
    }
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
  const hv = new HtmlValidate(htmlValidateConfig);
  for (const p of pages) {
    const report = await hv.validateString(p.html, p.path);
    for (const r of report.results) for (const m of r.messages) if (m.severity === 2) problems.push(`html-validate ${p.path}:${m.line}: ${m.ruleId} ${m.message}`);
  }
  const check = ctx.mode === "post" ? `${name} (production)` : name;
  return [row(check, problems.length === 0, summary(`${pages.length} pages: Nu 0 errors 0 warnings; html-validate 0 errors`, problems), ctx, problems)];
}
