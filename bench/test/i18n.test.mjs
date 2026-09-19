import { test } from "node:test";
import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";
import * as cheerio from "cheerio";
import { serveFixture, ctxFor } from "./helpers.mjs";
import { loadPages } from "../lib/pages.mjs";
import { run } from "../lib/i18n.mjs";

// serveFixture takes a directory path relative to the repo root; resolve it relative to this
// file instead of the process cwd so `npm test` (cwd = bench/) still finds it.
const FIXTURE = fileURLToPath(new URL("../fixtures/site-ok", import.meta.url));

test("i18n passes on the fixture and fails when a hreflang pair is broken", async () => {
  const s = await serveFixture(FIXTURE);
  const pages = await loadPages(s.base, { include404: true });
  const [ok] = await run(ctxFor(s.base, pages));
  assert.equal(ok.pass, true, ok.value);
  const broken = pages.map((p) => (p.path === "/ro/" ? { ...p, $: cheerio.load(p.html.replace('hreflang="en"', 'hreflang="de"')) } : p));
  const [bad] = await run(ctxFor(s.base, broken));
  assert.equal(bad.pass, false);
  assert.match(bad.value, /does not point back|not in the sitemap/);
  await s.close();
});
