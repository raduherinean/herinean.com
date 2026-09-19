import { test } from "node:test";
import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";
import { cpSync, rmSync, mkdtempSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, dirname } from "node:path";
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

// A minimal but valid solid-colour PNG, for the social-preview mutations (wrong dimensions; and,
// with `level` 0 — stored, uncompressed deflate blocks — a file as large as its raw pixels, for the
// byte-size rule).
function makePng(width, height, level = zlib.constants.Z_DEFAULT_COMPRESSION) {
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
    chunk("IDAT", zlib.deflateSync(raw, { level })),
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

// The fixture has no piece page; add one at a real piece URL (spec §4.1) with the given JSON-LD
// and list it in the sitemap, so the by-page-type presence rule has a piece to look at.
function addPiece(dir, urlPath, jsonld) {
  const file = join(dir, urlPath.replace(/^\//, ""), "index.html");
  mkdirSync(dirname(file), { recursive: true });
  writeFileSync(file, `<!doctype html>\n<html lang="en"><head><meta charset="utf-8"><title>piece</title><link rel="canonical" href="https://herinean.com${urlPath}"><script type="application/ld+json">${JSON.stringify(jsonld)}</script></head><body><main><p>piece</p></main></body></html>\n`);
  const sm = join(dir, "sitemap.xml");
  const before = readFileSync(sm, "utf8");
  const after = before.replace("</urlset>", `  <url>\n    <loc>https://herinean.com${urlPath}</loc>\n    <lastmod>2026-09-19</lastmod>\n  </url>\n</urlset>`);
  assert.notEqual(after, before, "fixture sitemap.xml has no </urlset> to anchor the mutation");
  writeFileSync(sm, after);
}
const PERSON = { "@type": "Person", "@id": "https://herinean.com/#person", name: "Radu Herinean", url: "https://herinean.com/", sameAs: ["https://www.linkedin.com/in/herinean", "https://x.com/raduherinean", "https://github.com/rlucian"] };
const POSTING = { "@type": "BlogPosting", headline: "Test post", inLanguage: "en", image: "https://herinean.com/og/en-test-post.00000000.png", author: { "@id": "https://herinean.com/#person" }, datePublished: "2026-09-19T06:00:00+03:00", dateModified: "2026-09-19T06:00:00+03:00", url: "https://herinean.com/writing/test-post/" };
const CRUMBS = { "@type": "BreadcrumbList", itemListElement: [{ "@type": "ListItem", position: 1, name: "Radu Herinean", item: "https://herinean.com/" }] };

test("jsonld requires BlogPosting + BreadcrumbList on a piece page: a complete piece passes, one without its BlogPosting block fails", async () => {
  const dir = tempCopy();
  try {
    addPiece(dir, "/writing/test-post/", { "@context": "https://schema.org", "@graph": [POSTING, CRUMBS, PERSON] });
    const s = await serveFixture(dir);
    try {
      const pages = await loadPages(s.base, { include404: true });
      const [ok] = await jsonldRun(ctxFor(s.base, pages));
      assert.equal(ok.pass, true, ok.value);
      assert.match(ok.value, /on 2 home pages, BlogPosting \+ BreadcrumbList on 1 pieces/);
    } finally {
      await s.close();
    }
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
  await assertFailsOnMutation(
    jsonldRun,
    (dir) => addPiece(dir, "/ro/articole/test-post/", { "@context": "https://schema.org", "@graph": [CRUMBS, PERSON] }),
    (row) => { assert.match(row.value, /\/ro\/articole\/test-post\/: no BlogPosting/); assert.doesNotMatch(row.value, /no BreadcrumbList/); },
  );
});

test("jsonld requires WebSite + Person on a home page: / without its WebSite object fails", async () => {
  await assertFailsOnMutation(
    jsonldRun,
    (dir) => {
      const indexPath = join(dir, "index.html");
      const before = readFileSync(indexPath, "utf8");
      const after = before.replace(/\{"@id":"https:\/\/herinean\.com\/#website","@type":"WebSite".*?\},(?=\{"@id":"https:\/\/herinean\.com\/#person")/, "");
      assert.notEqual(after, before, "fixture index.html no longer carries the expected WebSite object");
      writeFileSync(indexPath, after);
    },
    (row) => assert.match(row.value, /\/: no WebSite/),
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

// --- one failing mutation per rule (design §10) ---

test("social fails when an og:image is 200 KB or more", async () => {
  await assertFailsOnMutation(
    socialRun,
    // 1200×630 (so only the byte-size rule fires) with a stored, uncompressed IDAT: ~2.3 MB, well past 200 KB.
    (dir) => writeFileSync(join(dir, "og", "home-en.3b3118bf.png"), makePng(1200, 630, 0)),
    (row) => { assert.match(row.value, /og:image \d+ bytes ≥ 200 KB/); assert.doesNotMatch(row.value, /want 1200×630/); },
  );
});

test("social fails when an og:image URL is not content-hashed", async () => {
  await assertFailsOnMutation(
    socialRun,
    (dir) => {
      cpSync(join(dir, "og", "home-en.3b3118bf.png"), join(dir, "og", "home-en.png"));
      const idx = join(dir, "index.html");
      const before = readFileSync(idx, "utf8");
      const after = before.replace('content="https://herinean.com/og/home-en.3b3118bf.png"', 'content="https://herinean.com/og/home-en.png"');
      assert.notEqual(after, before, "fixture index.html no longer carries the expected og:image");
      writeFileSync(idx, after);
    },
    (row) => { assert.match(row.value, /\/: og:image \/og\/home-en\.png is not a hashed \/og\/ PNG/); assert.doesNotMatch(row.value, /→ 404/); },
  );
});

test("social fails when an og:image lives on a foreign host, even if that path would resolve here", async () => {
  await assertFailsOnMutation(
    socialRun,
    (dir) => {
      const idx = join(dir, "index.html");
      const before = readFileSync(idx, "utf8");
      const after = before.replace('content="https://herinean.com/og/home-en.3b3118bf.png"', 'content="https://cdn.example.net/og/home-en.3b3118bf.png"');
      assert.notEqual(after, before, "fixture index.html no longer carries the expected og:image");
      writeFileSync(idx, after);
    },
    (row) => { assert.match(row.value, /\/: og:image https:\/\/cdn\.example\.net\/og\/home-en\.3b3118bf\.png is not on the site's own origin \(https:\/\/herinean\.com\)/); assert.doesNotMatch(row.value, /→ 404|not a hashed/); },
  );
});

test("wellknown fails when security.txt loses a required field", async () => {
  await assertFailsOnMutation(
    wellknownRun,
    (dir) => {
      const f = join(dir, ".well-known", "security.txt");
      const before = readFileSync(f, "utf8");
      const after = before.replace(/^Canonical:.*\n?/m, "");
      assert.notEqual(after, before, "fixture security.txt has no Canonical: line to remove");
      writeFileSync(f, after);
    },
    (row) => assert.match(row.value, /security\.txt: no Canonical:/),
  );
});

test("wellknown fails when security.txt's Expires is in the past", async () => {
  await assertFailsOnMutation(
    wellknownRun,
    (dir) => {
      const f = join(dir, ".well-known", "security.txt");
      const before = readFileSync(f, "utf8");
      const after = before.replace(/^Expires: .*$/m, "Expires: 2020-01-01T00:00:00.000Z");
      assert.notEqual(after, before, "fixture security.txt has no Expires: line to rewrite");
      writeFileSync(f, after);
    },
    (row) => assert.match(row.value, /security\.txt: expired/),
  );
});

test("wellknown fails when an unknown path answers 200 instead of 404", async () => {
  await assertFailsOnMutation(
    wellknownRun,
    (dir) => {
      // serveFixture answers 404 for anything that is not a file; give the probe path a page so it answers 200.
      mkdirSync(join(dir, "this-page-does-not-exist"), { recursive: true });
      writeFileSync(join(dir, "this-page-does-not-exist", "index.html"), "<!doctype html><html lang=\"en\"><head><title>x</title></head><body>found</body></html>\n");
    },
    (row) => assert.match(row.value, /unknown path → 200, want 404/),
  );
});

test("feeds fails when an RSS feed loses its atom:link rel=self", async () => {
  await assertFailsOnMutation(
    feedsRun,
    (dir) => {
      const f = join(dir, "feed.en.xml");
      const before = readFileSync(f, "utf8");
      const after = before.replace(/<atom:link [^>]*rel="self"[^>]*\/?>(?:<\/atom:link>)?\s*/, "");
      assert.notEqual(after, before, "fixture feed.en.xml has no atom:link rel=self to remove");
      writeFileSync(f, after);
    },
    (row) => assert.match(row.value, /\/feed\.en\.xml: atom:link rel=self is undefined/),
  );
});

test("feeds fails when the JSON Feed loses a required field", async () => {
  await assertFailsOnMutation(
    feedsRun,
    (dir) => {
      const f = join(dir, "feed.json");
      const before = readFileSync(f, "utf8");
      const j = JSON.parse(before);
      assert.ok(j.feed_url, "fixture feed.json has no feed_url to remove");
      delete j.feed_url;
      writeFileSync(f, JSON.stringify(j, null, 2) + "\n");
    },
    (row) => assert.match(row.value, /feed\.json: feed_url missing/),
  );
});
