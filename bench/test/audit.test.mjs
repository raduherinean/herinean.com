import { test } from "node:test";
import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";
import { spawn } from "node:child_process";
import { cpSync, rmSync, mkdtempSync, existsSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { serveFixture } from "./helpers.mjs";

const FIXTURE = fileURLToPath(new URL("../fixtures/site-ok", import.meta.url));
const AUDIT = fileURLToPath(new URL("../audit.mjs", import.meta.url));

function tempCopy() {
  const dir = mkdtempSync(join(tmpdir(), "bench-audit-"));
  cpSync(FIXTURE, dir, { recursive: true });
  return dir;
}

// Asynchronous on purpose: the fixture server lives in this same process, and a spawnSync would
// block the event loop the server needs to answer the child's requests.
const run = (args) => new Promise((resolve) => {
  const child = spawn(process.execPath, args, { stdio: ["ignore", "pipe", "pipe"] });
  let stdout = "", stderr = "";
  child.stdout.on("data", (d) => { stdout += d; });
  child.stderr.on("data", (d) => { stderr += d; });
  child.on("close", (status) => resolve({ status, stdout, stderr }));
});

// Adds a sitemap entry the fixture cannot serve, so every run below aborts at the guard — right after the
// sitemap's pages are fetched, before any module is imported or Chromium launches — and the abort
// message is the evidence: which pages it names, and which it does not.
function withMissingPage(dir) {
  const sm = join(dir, "sitemap.xml");
  const before = readFileSync(sm, "utf8");
  const after = before.replace("</urlset>", "  <url>\n    <loc>https://herinean.com/missing/</loc>\n    <lastmod>2026-09-19</lastmod>\n  </url>\n</urlset>");
  assert.notEqual(after, before, "fixture sitemap.xml has no </urlset> to anchor the mutation");
  writeFileSync(sm, after);
}

// audit.mjs is a CLI, so this runs it as one.
test("audit aborts before any module runs when a sitemap page does not answer 200, naming path → status", { timeout: 30000 }, async () => {
  const dir = tempCopy();
  try {
    withMissingPage(dir);
    const s = await serveFixture(dir);
    const out = join(dir, "rows.json");
    try {
      const r = await run([AUDIT, s.base, "--mode", "ci", "--only", "checks", "--out", out]);
      assert.notEqual(r.status, 0, "audit must exit non-zero");
      assert.match(r.stderr, /pages not served as the sitemap says/);
      assert.match(r.stderr, /\/missing\/ → 404/);
      assert.ok(!existsSync(out), "no rows file may be written for a misserved build");
      assert.doesNotMatch(r.stderr, /^(ok  |FAIL) /m, "no module may have run");
    } finally {
      await s.close();
    }
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

// /404.html answering 404 is `site serve --static`'s contract, so ci mode requires it; on production the
// asset layer answers /404.html with a 301 to /404/ (force-trailing-slash), so post mode must accept that
// or every post-deploy audit would abort. The fixture server plays the edge with opts.redirect404.
test("a /404.html that redirects is named by the guard in ci mode and accepted in post mode", { timeout: 30000 }, async () => {
  const dir = tempCopy();
  try {
    withMissingPage(dir);
    const s = await serveFixture(dir, undefined, { redirect404: true });
    const out = join(dir, "rows.json");
    // Node prints the throwing source line (which itself contains "/404.html") above the error, so
    // the assertions read the "Error: …" line alone.
    const message = (r) => r.stderr.split("\n").find((l) => l.startsWith("Error: pages not served as the sitemap says")) || "";
    try {
      const ci = await run([AUDIT, s.base, "--mode", "ci", "--only", "checks", "--out", out]);
      assert.notEqual(ci.status, 0);
      assert.match(message(ci), /want 200, \/404\.html 404\)/, "ci mode states the 404 requirement");
      assert.match(message(ci), /\/404\.html → 301/, "ci mode names the redirecting 404 page");
      const post = await run([AUDIT, s.base, "--mode", "post", "--only", "checks", "--out", out]);
      assert.notEqual(post.status, 0, "the missing sitemap page still aborts post mode");
      assert.match(message(post), /want 200\)/, "post mode states no 404 requirement");
      assert.match(message(post), /\/missing\/ → 404/);
      assert.doesNotMatch(message(post), /\/404\.html/, "post mode accepts the edge's 301 on /404.html");
      assert.ok(!existsSync(out));
    } finally {
      await s.close();
    }
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

// merge.mjs --expect derives each mode's expected row set from ORDER; audit.mjs runs the modules in
// MODULES. The two lists are maintained by hand, so a module whose row name is not in ORDER would be
// sorted last AND never expected — its disappearance silent rather than red. Derive the expected sets
// from the modules themselves and hold merge.mjs to them.
test("every module's row name is in ORDER, and EXPECT matches the modules that declare each mode", async () => {
  const { ORDER, EXPECT } = await import("../merge.mjs");
  const { MODULES } = await import("../lib/modules.mjs");
  const ci = new Set(), post = new Set();
  for (const m of [...MODULES.checks, ...MODULES.lighthouse]) {
    const mod = await import(`../lib/${m}.mjs`);
    assert.ok(typeof mod.name === "string" && Array.isArray(mod.modes), `${m}.mjs exports name and modes`);
    const prod = mod.name.endsWith(" (production)") ? mod.name : `${mod.name} (production)`;
    if (mod.modes.includes("ci")) { assert.ok(ORDER.includes(mod.name), `${mod.name} (from ${m}.mjs) is not in ORDER`); ci.add(mod.name); }
    if (mod.modes.includes("post")) { assert.ok(ORDER.includes(prod), `${prod} (from ${m}.mjs) is not in ORDER`); post.add(prod); }
  }
  assert.deepEqual(new Set(EXPECT.ci), ci, "EXPECT.ci must be exactly the rows of the modules that run in ci mode");
  assert.deepEqual(new Set(EXPECT.post), post, "EXPECT.post must be exactly the rows of the modules that run in post mode");
  for (const check of ORDER) if (check !== "Audited build") assert.ok(ci.has(check) || post.has(check), `ORDER names ${check}, which no module produces`);
});
