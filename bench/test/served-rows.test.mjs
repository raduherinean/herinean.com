import { test, after } from "node:test";
import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";
import { cpSync, rmSync, mkdtempSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, dirname } from "node:path";
import { randomBytes } from "node:crypto";
import { serveFixture, ctxFor } from "./helpers.mjs";
import { loadPages } from "../lib/pages.mjs";
import { makeBrowser } from "../lib/browser.mjs";
import { run as headersRun } from "../lib/headers.mjs";
import { run as privacyRun } from "../lib/privacy.mjs";
import { run as weightRun } from "../lib/weight.mjs";
import { run as linksRun } from "../lib/links.mjs";

const FIXTURE = fileURLToPath(new URL("../fixtures/site-ok", import.meta.url));
// One Chromium for every Playwright-based test in this file (privacy, weight); closed in `after`.
const browser = makeBrowser();
after(async () => { await browser.close(); });

function tempCopy() {
  const dir = mkdtempSync(join(tmpdir(), "bench-served-rows-"));
  cpSync(FIXTURE, dir, { recursive: true });
  return dir;
}

test("headers passes on the fixture and fails when the server overrides x-frame-options", async () => {
  const s = await serveFixture(FIXTURE);
  try {
    const pages = await loadPages(s.base, { include404: true });
    const [ok] = await headersRun(ctxFor(s.base, pages));
    assert.equal(ok.pass, true, ok.value);
  } finally {
    await s.close();
  }

  // The fixture's real _headers file is correct; override the served header directly (no file mutation
  // needed) to prove the check reads the actual response, not just the static _headers text.
  const bad = await serveFixture(FIXTURE, { "x-frame-options": "SAMEORIGIN" });
  try {
    const badPages = await loadPages(bad.base, { include404: true });
    const [row] = await headersRun(ctxFor(bad.base, badPages));
    assert.equal(row.pass, false);
    assert.match(row.value, /x-frame-options = SAMEORIGIN/);
  } finally {
    await bad.close();
  }
});

test(
  "privacy passes on the fixture and fails when a page carries an executable script",
  { timeout: 120000 },
  async () => {
    const s = await serveFixture(FIXTURE);
    try {
      const pages = await loadPages(s.base, { include404: true });
      const [ok] = await privacyRun(ctxFor(s.base, pages, { browser }));
      assert.equal(ok.pass, true, ok.value);
    } finally {
      await s.close();
    }

    const dir = tempCopy();
    try {
      // The CSP would block this in a real browser, but the row must fail on the tag count in the HTML
      // itself — a served page should never carry an executable <script> in the first place.
      const idx = join(dir, "index.html");
      writeFileSync(idx, readFileSync(idx, "utf8").replace("</body>", "<script>alert(1)</script></body>"));
      const bad = await serveFixture(dir);
      try {
        const badPages = await loadPages(bad.base, { include404: true });
        const [row] = await privacyRun(ctxFor(bad.base, badPages, { browser }));
        assert.equal(row.pass, false);
        assert.match(row.value, /executable <script>/);
      } finally {
        await bad.close();
      }
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  },
);

test(
  "weight passes on the fixture and fails when a page is padded past 30 KB brotli",
  { timeout: 120000 },
  async () => {
    const s = await serveFixture(FIXTURE);
    try {
      const pages = await loadPages(s.base, { include404: true });
      const [ok] = await weightRun(ctxFor(s.base, pages, { browser }));
      assert.equal(ok.pass, true, ok.value);
    } finally {
      await s.close();
    }

    const dir = tempCopy();
    try {
      // Random hex, not a repeated string: brotli would squash a repetitive pad well under 30 KB.
      const idx = join(dir, "index.html");
      const pad = randomBytes(60 * 1024).toString("hex");
      writeFileSync(idx, readFileSync(idx, "utf8").replace("</body>", `<!--${pad}--></body>`));
      const bad = await serveFixture(dir);
      try {
        const badPages = await loadPages(bad.base, { include404: true });
        const [row] = await weightRun(ctxFor(bad.base, badPages, { browser }));
        assert.equal(row.pass, false);
        assert.match(row.value, /HTML\+CSS \d+ bytes brotli > 30 KB/);
      } finally {
        await bad.close();
      }
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  },
);

// A minimal, valid stub page for an internal path the fixture's two pages link to but that the
// two-page fixture doesn't itself contain (the real site also has /writing/, /colophon/, /privacy/,
// /ro/articole/, /ro/confidentialitate/ — see bench/fixtures/README.md).
function stubPage(dir, urlPath) {
  const file = join(dir, urlPath.replace(/^\//, ""), "index.html");
  mkdirSync(dirname(file), { recursive: true });
  writeFileSync(file, "<!doctype html>\n<html lang=\"en\"><head><meta charset=\"utf-8\"><title>stub</title></head><body>stub</body></html>\n");
}

test(
  "links passes on a fixture with its internal targets stubbed, ignores a href that only 404.html carries, and fails when a real page's href points at /missing/",
  { timeout: 30000 },
  async () => {
    const dir = tempCopy();
    try {
      for (const p of ["/writing/", "/colophon/", "/privacy/", "/ro/articole/", "/ro/confidentialitate/"]) stubPage(dir, p);

      const s = await serveFixture(dir);
      try {
        const pages = await loadPages(s.base, { include404: true });
        const [ok] = await linksRun(ctxFor(s.base, pages));
        assert.equal(ok.pass, true, ok.value);
      } finally {
        await s.close();
      }

      // 404.html is not a link source (controller ruling: spec §3 row 1 carves it out to Nu +
      // axe only — its self-canonical answering 404 is by design, and its other links are the
      // shared masthead/footer every other page already carries). A href that ONLY 404.html
      // carries, pointing at a path that doesn't exist anywhere, must not be checked at all —
      // the row stays green.
      const nf = join(dir, "404.html");
      const nfBefore = readFileSync(nf, "utf8");
      const nfAfter = nfBefore.replace("</body>", '<a href="/only-on-404/">only on 404</a></body>');
      assert.notEqual(nfAfter, nfBefore, "fixture 404.html has no </body> to anchor the mutation");
      writeFileSync(nf, nfAfter);
      const onlyOn404 = await serveFixture(dir);
      try {
        const pages = await loadPages(onlyOn404.base, { include404: true });
        const [ok] = await linksRun(ctxFor(onlyOn404.base, pages));
        assert.equal(ok.pass, true, ok.value);
        assert.doesNotMatch(ok.value, /only-on-404/);
      } finally {
        await onlyOn404.close();
      }

      // Break one internal href on a real page (the first "Writing" link on the home page) so it
      // points nowhere — this one must still fail the row.
      const idx = join(dir, "index.html");
      const before = readFileSync(idx, "utf8");
      const after = before.replace('href="/writing/"', 'href="/missing/"');
      assert.notEqual(after, before, "fixture no longer contains the expected /writing/ link");
      writeFileSync(idx, after);
      const bad = await serveFixture(dir);
      try {
        const badPages = await loadPages(bad.base, { include404: true });
        const [row] = await linksRun(ctxFor(bad.base, badPages));
        assert.equal(row.pass, false);
        assert.match(row.value, /\/missing\/ → 404/);
      } finally {
        await bad.close();
      }
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  },
);
