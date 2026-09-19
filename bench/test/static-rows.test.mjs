import { test } from "node:test";
import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";
import { cpSync, rmSync, mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import zlib from "node:zlib";
import { serveFixture, ctxFor } from "./helpers.mjs";
import { loadPages } from "../lib/pages.mjs";
import { run as socialRun } from "../lib/social.mjs";
import { run as wellknownRun } from "../lib/wellknown.mjs";
import { run as feedsRun } from "../lib/feeds.mjs";
import { run as jsonldRun } from "../lib/jsonld.mjs";

const FIXTURE = fileURLToPath(new URL("../fixtures/site-ok", import.meta.url));

// Makes a temp copy of the fixture so a mutation test can edit a file without touching the real fixture.
function tempCopy() {
  const dir = mkdtempSync(join(tmpdir(), "bench-static-rows-"));
  cpSync(FIXTURE, dir, { recursive: true });
  return dir;
}

// A minimal but valid solid-colour PNG, for the social-preview mutation (wrong dimensions).
function makePng(width, height) {
  const chunk = (type, data) => {
    const len = Buffer.alloc(4); len.writeUInt32BE(data.length);
    const typeAndData = Buffer.concat([Buffer.from(type, "ascii"), data]);
    const crc = Buffer.alloc(4); crc.writeUInt32BE(zlib.crc32(typeAndData) >>> 0);
    return Buffer.concat([len, typeAndData, crc]);
  };
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(width, 0); ihdr.writeUInt32BE(height, 4);
  ihdr[8] = 8; ihdr[9] = 2; // 8-bit depth, RGB
  const raw = Buffer.alloc(height * (1 + width * 3)); // filter byte 0 + black pixels per row
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk("IHDR", ihdr),
    chunk("IDAT", zlib.deflateSync(raw)),
    chunk("IEND", Buffer.alloc(0)),
  ]);
}

// Runs `run` against the plain fixture (expect pass) and returns nothing; always closes the server,
// even when the assertion throws, so a failing assertion never leaves a listening socket behind.
async function assertPassesOnFixture(run) {
  const s = await serveFixture(FIXTURE);
  try {
    const pages = await loadPages(s.base, { include404: true });
    const [ok] = await run(ctxFor(s.base, pages));
    assert.equal(ok.pass, true, ok.value);
  } finally {
    await s.close();
  }
}

// Copies the fixture to a temp dir, lets `mutate(dir)` break one file, serves the copy, runs `run`
// against it, and hands the resulting row to `check`. Always tears down the server and temp dir.
async function assertFailsOnMutation(run, mutate, check) {
  const dir = tempCopy();
  try {
    mutate(dir);
    const bad = await serveFixture(dir);
    try {
      const badPages = await loadPages(bad.base, { include404: true });
      const [row] = await run(ctxFor(bad.base, badPages));
      assert.equal(row.pass, false);
      check(row);
    } finally {
      await bad.close();
    }
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

test("social passes on the fixture and fails when an og:image has the wrong dimensions", async () => {
  await assertPassesOnFixture(socialRun);
  await assertFailsOnMutation(
    socialRun,
    (dir) => writeFileSync(join(dir, "og", "home-en.3b3118bf.png"), makePng(1, 1)),
    (row) => assert.match(row.value, /1×1, want 1200×630/),
  );
});

test("wellknown passes on the fixture and fails when robots.txt loses its Sitemap line", async () => {
  await assertPassesOnFixture(wellknownRun);
  await assertFailsOnMutation(
    wellknownRun,
    (dir) => writeFileSync(join(dir, "robots.txt"), "User-agent: *\nAllow: /\n"),
    (row) => assert.match(row.value, /robots\.txt.*no Sitemap line/),
  );
});

test("feeds passes on the fixture and fails when an item's content carries a relative URL", async () => {
  await assertPassesOnFixture(feedsRun);
  await assertFailsOnMutation(
    feedsRun,
    (dir) => {
      // The fixture's feeds were built with zero pieces, so feed.en.xml has no <item>; add a minimal
      // one whose content:encoded carries a relative href, which the check must reject.
      const body = "Lorem ipsum dolor sit amet, consectetur adipiscing elit sed do eiusmod tempor. ".repeat(3);
      const item = `  <item>\n    <title>Test post</title>\n    <link>https://herinean.com/writing/test-post/</link>\n    <guid isPermaLink="true">https://herinean.com/writing/test-post/</guid>\n    <pubDate>Sat, 19 Sep 2026 06:00:00 +0300</pubDate>\n    <content:encoded>&lt;p&gt;${body}&lt;/p&gt;&lt;a href=&quot;/x/&quot;&gt;relative&lt;/a&gt;</content:encoded>\n  </item>\n`;
      const feedPath = join(dir, "feed.en.xml");
      writeFileSync(feedPath, readFileSync(feedPath, "utf8").replace("</channel>", `${item}</channel>`));
    },
    (row) => assert.match(row.value, /relative URL in content/),
  );
});

test("jsonld passes on the fixture and fails when Person.sameAs loses github.com", async () => {
  await assertPassesOnFixture(jsonldRun);
  await assertFailsOnMutation(
    jsonldRun,
    (dir) => {
      const indexPath = join(dir, "index.html");
      const before = readFileSync(indexPath, "utf8");
      const after = before.replace(',"https://github.com/rlucian"', "");
      assert.notEqual(after, before, "fixture no longer contains the expected sameAs entry");
      writeFileSync(indexPath, after);
    },
    (row) => assert.match(row.value, /sameAs lacks github\.com/),
  );
});
