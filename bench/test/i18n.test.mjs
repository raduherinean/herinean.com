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

// Swaps one page's <link rel="canonical"> href in memory (the served fixture is untouched) — each
// test below mutates exactly that and nothing else.
const withCanonical = (pages, path, href) => pages.map((p) => (p.path === path ? { ...p, $: cheerio.load(p.html.replace(`<link rel="canonical" href="https://herinean.com${path}">`, `<link rel="canonical" href="${href}">`)) } : p));

test("i18n fails when a page's canonical is well-formed but names another page", async () => {
  const s = await serveFixture(FIXTURE);
  try {
    const pages = await loadPages(s.base, { include404: true });
    const [ok] = await run(ctxFor(s.base, pages));
    assert.equal(ok.pass, true, ok.value);
    const other = withCanonical(pages, "/ro/", "https://herinean.com/");
    assert.notEqual(other.find((p) => p.path === "/ro/").$('link[rel="canonical"]').attr("href"), "https://herinean.com/ro/", "the mutation did not take");
    const [bad] = await run(ctxFor(s.base, other));
    assert.equal(bad.pass, false);
    assert.match(bad.value, /\/ro\/: canonical https:\/\/herinean\.com\/ is another page/);
  } finally {
    await s.close();
  }
});

test("i18n fails on a canonical of the wrong shape: relative, http:, or without the trailing slash", async () => {
  const s = await serveFixture(FIXTURE);
  try {
    const pages = await loadPages(s.base, { include404: true });
    for (const [href, why] of [["/ro/", "relative"], ["http://herinean.com/ro/", "http:"], ["https://herinean.com/ro", "no trailing slash"]]) {
      const [bad] = await run(ctxFor(s.base, withCanonical(pages, "/ro/", href)));
      assert.equal(bad.pass, false, why);
      assert.match(bad.value, new RegExp(`/ro/: canonical ${href.replace(/[/.:]/g, "\\$&")} not absolute with a trailing slash`), why);
    }
  } finally {
    await s.close();
  }
});
