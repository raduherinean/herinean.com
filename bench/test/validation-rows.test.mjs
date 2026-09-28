import { test, after } from "node:test";
import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";
import { cpSync, rmSync, mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { serveFixture, ctxFor } from "./helpers.mjs";
import { loadPages } from "../lib/pages.mjs";
import { makeBrowser } from "../lib/browser.mjs";
import { run as htmlRun } from "../lib/html.mjs";
import { run as a11yRun } from "../lib/a11y.mjs";

const FIXTURE = fileURLToPath(new URL("../fixtures/site-ok", import.meta.url));
// One Chromium for every a11y test in this file; closed in `after`.
const browser = makeBrowser();
after(async () => { await browser.close(); });

function tempCopy() {
  const dir = mkdtempSync(join(tmpdir(), "bench-validation-rows-"));
  cpSync(FIXTURE, dir, { recursive: true });
  return dir;
}

test(
  "html passes on the fixture, fails on an unclosed <div> (Nu) and separately on a missing <label> (html-validate only)",
  { timeout: 60000 },
  async () => {
    const s = await serveFixture(FIXTURE);
    try {
      const pages = await loadPages(s.base, { include404: true });
      const [ok] = await htmlRun(ctxFor(s.base, pages));
      assert.equal(ok.pass, true, ok.value);
      assert.match(ok.value, /^3 pages: Nu 0 errors 0 warnings; html-validate 0 errors$/);
    } finally {
      await s.close();
    }

    // Nu: an unclosed <div> is a real parse error the checker must report.
    const nuDir = tempCopy();
    try {
      const idx = join(nuDir, "index.html");
      const before = readFileSync(idx, "utf8");
      const after = before.replace('<main id="main">', '<main id="main"><div>');
      assert.notEqual(after, before, 'fixture index.html has no <main id="main"> to anchor the mutation');
      writeFileSync(idx, after);
      const bad = await serveFixture(nuDir);
      try {
        const pages = await loadPages(bad.base, { include404: true });
        const [row] = await htmlRun(ctxFor(bad.base, pages));
        assert.equal(row.pass, false);
        assert.match(row.value, /Nu error \/:\d+: /);
      } finally {
        await bad.close();
      }
    } finally {
      rmSync(nuDir, { recursive: true, force: true });
    }

    // html-validate only: a bare <input> without a <label> is valid HTML5 — Nu has no opinion on
    // it — but fails the a11y ruleset's input-missing-label. Proves html-validate is wired in, not
    // just Nu.
    const hvDir = tempCopy();
    try {
      const idx = join(hvDir, "index.html");
      const before = readFileSync(idx, "utf8");
      const after = before.replace('<main id="main">', '<main id="main"><input type="text">');
      assert.notEqual(after, before, 'fixture index.html has no <main id="main"> to anchor the mutation');
      writeFileSync(idx, after);
      const bad = await serveFixture(hvDir);
      try {
        const pages = await loadPages(bad.base, { include404: true });
        const [row] = await htmlRun(ctxFor(bad.base, pages));
        assert.equal(row.pass, false);
        assert.match(row.value, /html-validate \/:\d+: input-missing-label/);
      } finally {
        await bad.close();
      }
    } finally {
      rmSync(hvDir, { recursive: true, force: true });
    }
  },
);

test("html reports 'Nu did not run' — a red row — when java exits 0 without printing anything", { timeout: 30000 }, async () => {
  // A `java` stub first on PATH that returns 0 and prints nothing: the shape of a JVM that never
  // reached the checker. Before the guard, `nu.out || '{"messages":[]}'` read that as a clean page.
  const stubDir = mkdtempSync(join(tmpdir(), "bench-java-stub-"));
  const savedPath = process.env.PATH;
  try {
    writeFileSync(join(stubDir, "java"), "#!/bin/sh\nexit 0\n", { mode: 0o755 });
    process.env.PATH = `${stubDir}:${savedPath}`;
    const s = await serveFixture(FIXTURE);
    try {
      const pages = await loadPages(s.base, { include404: true });
      const [row] = await htmlRun(ctxFor(s.base, pages));
      assert.equal(row.pass, false);
      assert.match(row.value, /Nu did not run: exit 0 with no output/);
    } finally {
      await s.close();
    }
  } finally {
    process.env.PATH = savedPath;
    rmSync(stubDir, { recursive: true, force: true });
  }
});

test(
  "html and a11y both fail closed in post mode when /colophon/ is absent from the page set",
  { timeout: 30000 },
  async () => {
    const s = await serveFixture(FIXTURE);
    try {
      const pages = await loadPages(s.base, { include404: true });
      const [htmlRow] = await htmlRun(ctxFor(s.base, pages, { mode: "post", browser }));
      assert.equal(htmlRow.pass, false);
      assert.match(htmlRow.value, /\/colophon\//);
      const [a11yRow] = await a11yRun(ctxFor(s.base, pages, { mode: "post", browser }));
      assert.equal(a11yRow.pass, false);
      assert.match(a11yRow.value, /\/colophon\//);
    } finally {
      await s.close();
    }
  },
);

test(
  "a11y passes on the fixture and fails when a page carries an <img> without alt",
  { timeout: 120000 },
  async () => {
    const s = await serveFixture(FIXTURE);
    try {
      const pages = await loadPages(s.base, { include404: true });
      const [ok] = await a11yRun(ctxFor(s.base, pages, { browser }));
      assert.equal(ok.pass, true, ok.value);
      assert.match(ok.value, /^3 pages × light \+ dark: /);
    } finally {
      await s.close();
    }

    const dir = tempCopy();
    try {
      const idx = join(dir, "index.html");
      const before = readFileSync(idx, "utf8");
      const after = before.replace('<main id="main">', '<main id="main"><img src="/x.png">');
      assert.notEqual(after, before, 'fixture index.html has no <main id="main"> to anchor the mutation');
      writeFileSync(idx, after);
      const bad = await serveFixture(dir);
      try {
        const pages = await loadPages(bad.base, { include404: true });
        const [row] = await a11yRun(ctxFor(bad.base, pages, { browser }));
        assert.equal(row.pass, false);
        assert.match(row.value, /image-alt/);
      } finally {
        await bad.close();
      }
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  },
);

test(
  "a11y fails a low-contrast paragraph under AAA even though it clears AA",
  { timeout: 120000 },
  async () => {
    const dir = tempCopy();
    try {
      const idx = join(dir, "index.html");
      const before = readFileSync(idx, "utf8");
      // #656565 on the paper background is ~5.5:1 — above AA's 4.5:1, below AAA's 7:1.
      const after = before.replace('<main id="main">', '<main id="main"><p style="color:#656565">low contrast</p>');
      assert.notEqual(after, before, 'fixture index.html has no <main id="main"> to anchor the mutation');
      writeFileSync(idx, after);
      // The real _headers CSP has no 'unsafe-inline' for style attributes, so it would silently
      // block this mutation from ever rendering; strip it here — CSP enforcement is headers.mjs's
      // row, not this one's.
      const bad = await serveFixture(dir, { "content-security-policy": null });
      try {
        const pages = await loadPages(bad.base, { include404: true });
        const [row] = await a11yRun(ctxFor(bad.base, pages, { browser }));
        assert.equal(row.pass, false);
        assert.match(row.value, /AAA contrast/);
      } finally {
        await bad.close();
      }
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  },
);
