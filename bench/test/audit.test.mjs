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

// audit.mjs is a CLI, so this runs it as one. The abort happens right after the sitemap's pages are
// fetched, before any module is imported or Chromium launches, so the run is quick. The other
// branch — /404.html answering 200 — is not exercised: serveFixture always answers /404.html with
// 404, the way `site serve --static` does, and there is no cheap way to make it do otherwise.
test("audit aborts before any module runs when a sitemap page does not answer 200, naming path → status", { timeout: 30000 }, async () => {
  const dir = tempCopy();
  try {
    const sm = join(dir, "sitemap.xml");
    const before = readFileSync(sm, "utf8");
    const after = before.replace("</urlset>", "  <url>\n    <loc>https://herinean.com/missing/</loc>\n    <lastmod>2026-09-19</lastmod>\n  </url>\n</urlset>");
    assert.notEqual(after, before, "fixture sitemap.xml has no </urlset> to anchor the mutation");
    writeFileSync(sm, after);
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
