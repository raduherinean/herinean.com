import { test, after } from "node:test";
import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";
import { cpSync, rmSync, mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { serveFixture, ctxFor } from "./helpers.mjs";
import { loadPages } from "../lib/pages.mjs";
import { makeBrowser } from "../lib/browser.mjs";
import { run as fontsRun } from "../lib/fonts.mjs";

const FIXTURE = fileURLToPath(new URL("../fixtures/site-ok", import.meta.url));
// One Chromium for every test in this file; closed in `after`.
const browser = makeBrowser();
after(async () => { await browser.close(); });

function tempCopy() {
  const dir = mkdtempSync(join(tmpdir(), "bench-fonts-"));
  cpSync(FIXTURE, dir, { recursive: true });
  return dir;
}

test(
  "fonts produces a Font correctness row with per-page CLS/height detail for 2 pages × 2 widths",
  { timeout: 180000 },
  async () => {
    const s = await serveFixture(FIXTURE);
    try {
      const pages = await loadPages(s.base, { include404: true });
      const [row] = await fontsRun(ctxFor(s.base, pages, { browser }));
      assert.equal(row.check, "Font correctness");
      assert.ok(Array.isArray(row.detail), "detail is an array of per-page measurements");
      assert.equal(row.detail.length, 4, "2 pages × 2 widths");
      for (const m of row.detail) {
        assert.equal(typeof m.vp, "number");
        assert.equal(typeof m.path, "string");
        assert.equal(typeof m.cls, "number");
        assert.ok(Array.isArray(m.height) && m.height.length === 2, "height carries [web, fallback]");
      }
    } finally {
      await s.close();
    }
  },
);

test(
  "fonts fails closed when /fonts/ is deleted — the web font never applies, even after the 600 ms delay",
  { timeout: 180000 },
  async () => {
    const dir = tempCopy();
    try {
      rmSync(join(dir, "fonts"), { recursive: true, force: true });
      const s = await serveFixture(dir);
      try {
        const pages = await loadPages(s.base, { include404: true });
        const [row] = await fontsRun(ctxFor(s.base, pages, { browser }));
        assert.equal(row.pass, false);
        assert.match(row.value, /not applied/);
      } finally {
        await s.close();
      }
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  },
);

test("fonts reports a red row rather than passing silently when the page scope is empty", async () => {
  const [row] = await fontsRun(ctxFor("http://127.0.0.1:1", [], { browser }));
  assert.equal(row.check, "Font correctness");
  assert.equal(row.pass, false);
  assert.match(row.value, /no pages/);
});
