import { test } from "node:test";
import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";
import { serveFixture, ctxFor } from "./helpers.mjs";
import { loadPages } from "../lib/pages.mjs";
import { run } from "../lib/lighthouse.mjs";

// serveFixture takes a directory path relative to the repo root; resolve it relative to this
// file instead of the process cwd so `npm test` (cwd = bench/) still finds it.
const FIXTURE = fileURLToPath(new URL("../fixtures/site-ok", import.meta.url));
const CATS = ["performance", "accessibility", "best-practices", "seo"];

// Smoke test only: the fixture is served without compression (serveFixture doesn't gzip), so this
// does not assert 100 — that's the real-site matrix's job, run against `site serve --static`.
test(
  "lighthouse runs one fixture page, shard 1/1, mobile only, and produces a scored row",
  { timeout: 180000 },
  async () => {
    const s = await serveFixture(FIXTURE);
    try {
      const pages = (await loadPages(s.base, { include404: true })).filter((p) => p.path === "/");
      const [row] = await run(ctxFor(s.base, pages, { formFactor: "mobile", shard: "1/1" }));
      assert.equal(row.check, "Lighthouse [mobile 1/1]");
      assert.equal(row.pages, 1);
      assert.equal(typeof row.worst, "number");
      assert.ok(row.detail && row.detail.scores && row.detail.scores["/"], "detail carries per-page scores");
      assert.deepEqual(Object.keys(row.detail.scores["/"]).sort(), [...CATS].sort());
      for (const c of CATS) assert.equal(typeof row.detail.scores["/"][c], "number");
      assert.ok(Array.isArray(row.detail.failingAudits));
    } finally {
      await s.close();
    }
  },
);
