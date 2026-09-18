import { test } from "node:test";
import assert from "node:assert/strict";
import worker from "./index.js";

const HTML = { status: 200, headers: { "content-type": "text/html; charset=utf-8" } };
function harness({ assets = () => new Response("<!doctype html><html></html>", HTML), kv } = {}) {
  const points = [];
  const waits = [];
  const env = {
    PROD_HOST: "herinean.com",
    ASSETS: { fetch: async (req) => assets(req) },
    VIEWS: { writeDataPoint: (p) => points.push(p) },
    SCORECARD: kv,
  };
  const ctx = { waitUntil: (p) => waits.push(p) };
  return { env, ctx, points, settle: () => Promise.all(waits) };
}
const req = (url, init = {}, cf) => {
  const r = new Request(url, { headers: { "user-agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/128 Safari/537.36" }, ...init });
  if (cf) r.cf = cf;
  return r;
};

test("production HTML view → one datapoint: path, lang, referrer host, ref, country", async () => {
  const h = harness();
  const res = await worker.fetch(req("https://herinean.com/ro/articole/x/?ref=li", { headers: { referer: "https://www.linkedin.com/feed/", "user-agent": "Mozilla/5.0" } }, { country: "RO" }), h.env, h.ctx);
  assert.equal(res.status, 200);
  await h.settle();
  assert.deepEqual(h.points, [{ blobs: ["/ro/articole/x/", "ro", "www.linkedin.com", "li", "RO"], doubles: [1], indexes: ["/ro/articole/x/"] }]);
});

test("ref: unknown tag is 'other', absent is 'none'; internal referrer is dropped; no country is empty", async () => {
  const h = harness();
  await worker.fetch(req("https://herinean.com/writing/x/?ref=campaign", { headers: { referer: "https://herinean.com/", "user-agent": "Mozilla/5.0" } }), h.env, h.ctx);
  await worker.fetch(req("https://herinean.com/"), h.env, h.ctx);
  await h.settle();
  assert.deepEqual(h.points.map((p) => p.blobs), [["/writing/x/", "en", "", "other", ""], ["/", "en", "", "none", ""]]);
});

test("nothing is counted for bots, previews, non-HTML, errors or HEAD", async () => {
  const h = harness({ assets: (r) => (r.url.endsWith("/feed.xml") ? new Response("<rss/>", { headers: { "content-type": "application/rss+xml" } }) : r.url.endsWith("/nope/") ? new Response("nf", { status: 404, headers: { "content-type": "text/html" } }) : new Response("<html></html>", HTML)) });
  await worker.fetch(req("https://herinean.com/", { headers: { "user-agent": "Mozilla/5.0 (compatible; Googlebot/2.1)" } }), h.env, h.ctx);
  await worker.fetch(req("https://preview-herinean-com.example.workers.dev/"), h.env, h.ctx);
  await worker.fetch(req("https://herinean.com/feed.xml"), h.env, h.ctx);
  await worker.fetch(req("https://herinean.com/nope/"), h.env, h.ctx);
  await worker.fetch(req("https://herinean.com/", { method: "HEAD" }), h.env, h.ctx);
  await h.settle();
  assert.deepEqual(h.points, []);
});

test("a 304 revalidation is a view too — returning readers", async () => {
  const h = harness({ assets: () => new Response(null, { status: 304, headers: { etag: '"abc"' } }) });
  await worker.fetch(req("https://herinean.com/writing/x/", { headers: { "if-none-match": '"abc"', "user-agent": "Mozilla/5.0" } }), h.env, h.ctx);
  await h.settle();
  assert.equal(h.points.length, 1);
  assert.equal(h.points[0].blobs[0], "/writing/x/");
});

test("colophon: composite ETag from the asset and the KV fragment; a matching If-None-Match → 304 before HTMLRewriter", async () => {
  const kv = { getWithMetadata: async (k) => (k === "scorecard" ? { value: "<table data-scorecard></table>", metadata: { etag: "feed1234" } } : { value: null, metadata: null }) };
  let sawConditional = null;
  const h = harness({ kv, assets: (r) => { sawConditional = r.headers.has("if-none-match"); return new Response("<html><table data-scorecard></table></html>", { status: 200, headers: { "content-type": "text/html; charset=utf-8", etag: '"abc"' } }); } });
  const res = await worker.fetch(req("https://herinean.com/colophon/", { headers: { "if-none-match": 'W/"abc-feed1234"', "user-agent": "Mozilla/5.0" } }), h.env, h.ctx);
  assert.equal(res.status, 304);
  assert.equal(res.headers.get("etag"), 'W/"abc-feed1234"');
  assert.equal(sawConditional, false, "the asset layer must never see the colophon's conditional header");
  await h.settle();
  assert.equal(h.points.length, 1, "the 304 is a view");
});

test("colophon with empty KV: the asset's own ETag still gets a 304", async () => {
  const kv = { getWithMetadata: async () => ({ value: null, metadata: null }) };
  const h = harness({ kv, assets: () => new Response("<html><table data-scorecard></table></html>", { status: 200, headers: { "content-type": "text/html; charset=utf-8", etag: '"abc"' } }) });
  const res = await worker.fetch(req("https://herinean.com/colophon/", { headers: { "if-none-match": '"abc"', "user-agent": "Mozilla/5.0" } }), h.env, h.ctx);
  assert.equal(res.status, 304);
  assert.equal(res.headers.get("etag"), '"abc"');
  await h.settle();
  assert.equal(h.points.length, 1, "the 304 is a view");
});

test("preview hosts: noindex on every response and a disallow-all robots.txt", async () => {
  const h = harness();
  const page = await worker.fetch(req("https://abc-herinean-com.example.workers.dev/"), h.env, h.ctx);
  assert.equal(page.headers.get("x-robots-tag"), "noindex, nofollow");
  const robots = await worker.fetch(req("https://abc-herinean-com.example.workers.dev/robots.txt"), h.env, h.ctx);
  assert.equal(await robots.text(), "User-agent: *\nDisallow: /\n");
});

test("mta-sts host serves the policy file only", async () => {
  const h = harness({ assets: () => new Response("version: STSv1\n", { headers: { "content-type": "text/plain" } }) });
  assert.equal((await worker.fetch(req("https://mta-sts.herinean.com/.well-known/mta-sts.txt"), h.env, h.ctx)).status, 200);
  assert.equal((await worker.fetch(req("https://mta-sts.herinean.com/"), h.env, h.ctx)).status, 404);
});

test("scorecard.json comes from KV, 404 when unpublished", async () => {
  const kv = { get: async (k) => (k === "scorecard.json" ? '{"rows":[]}' : null) };
  const h = harness({ kv });
  const res = await worker.fetch(req("https://herinean.com/colophon/scorecard.json"), h.env, h.ctx);
  assert.equal(res.status, 200);
  assert.equal(res.headers.get("content-type"), "application/json; charset=utf-8");
  const none = await worker.fetch(req("https://herinean.com/colophon/scorecard.json"), harness({ kv: { get: async () => null } }).env, h.ctx);
  assert.equal(none.status, 404);
});

test("a Worker error falls through to the plain asset", async () => {
  const h = harness();
  h.env.VIEWS = { writeDataPoint: () => { throw new Error("boom"); } };
  const res = await worker.fetch(req("https://herinean.com/"), h.env, h.ctx);
  assert.equal(res.status, 200);
  await assert.doesNotReject(h.settle());
});

test("any Worker error falls through to the plain asset", async () => {
  const boom = () => { throw new Error("boom"); };
  const h = harness({ kv: { getWithMetadata: boom, get: boom }, assets: () => new Response("<html><table data-scorecard></table></html>", HTML) });
  assert.equal((await worker.fetch(req("https://herinean.com/colophon/"), h.env, h.ctx)).status, 200, "a KV read that throws keeps the built-in table");
  assert.equal((await worker.fetch(req("https://herinean.com/colophon/scorecard.json"), h.env, h.ctx)).status, 200, "a throwing KV on the JSON path falls through to the asset");
  Object.defineProperty(h.env, "PROD_HOST", { get() { throw new Error("boom"); } });
  const res = await worker.fetch(req("https://herinean.com/writing/x/"), h.env, h.ctx);
  assert.equal(res.status, 200, "an error before any branch is taken still answers with the asset");
  assert.equal(await res.text(), "<html><table data-scorecard></table></html>");
});

test("bots and empty user agents are not counted", async () => {
  const h = harness();
  for (const ua of ["Mozilla/5.0 (compatible; Mastodon/4.2; +https://x)", "Bluesky Cardyb/1.1", "Go-http-client/2.0"]) {
    await worker.fetch(req("https://herinean.com/writing/x/", { headers: { "user-agent": ua } }), h.env, h.ctx);
  }
  await worker.fetch(new Request("https://herinean.com/writing/x/"), h.env, h.ctx); // no User-Agent header at all
  await h.settle();
  assert.deepEqual(h.points, []);
});

test("weak and list validators match the composite ETag", async () => {
  const kv = { getWithMetadata: async () => ({ value: "<table data-scorecard></table>", metadata: { etag: "feed1234" } }) };
  const h = harness({ kv, assets: () => new Response("<html><table data-scorecard></table></html>", { status: 200, headers: { "content-type": "text/html; charset=utf-8", etag: '"abc"' } }) });
  const list = await worker.fetch(req("https://herinean.com/colophon/", { headers: { "if-none-match": '"abc-feed1234", W/"other"', "user-agent": "Mozilla/5.0" } }), h.env, h.ctx);
  assert.equal(list.status, 304);
  const weak = await worker.fetch(req("https://herinean.com/colophon/", { headers: { "if-none-match": 'W/"abc-feed1234"', "user-agent": "Mozilla/5.0" } }), h.env, h.ctx);
  assert.equal(weak.status, 304);
  assert.equal(weak.headers.get("etag"), 'W/"abc-feed1234"');
  const miss = await worker.fetch(req("https://herinean.com/colophon/", { headers: { "if-none-match": 'W/"zzz-feed1234", "abc-zzz"', "user-agent": "Mozilla/5.0" } }), h.env, h.ctx);
  assert.equal(miss.status, 200, "a list with no matching tag is not a match");
});

test("charset: bare text/html and text/plain gain utf-8; an already-charset type and a binary type are untouched", async () => {
  const types = { "/": "text/html", "/llms.txt": "text/plain", "/x/": "text/html; charset=utf-8", "/fonts/a.woff2": "font/woff2" };
  const h = harness({ assets: (r) => new Response("body", { headers: { "content-type": types[new URL(r.url).pathname] } }) });
  const page = await worker.fetch(req("https://herinean.com/"), h.env, h.ctx);
  assert.equal(page.headers.get("content-type"), "text/html; charset=utf-8");
  const llms = await worker.fetch(req("https://herinean.com/llms.txt"), h.env, h.ctx);
  assert.equal(llms.headers.get("content-type"), "text/plain; charset=utf-8");
  const x = await worker.fetch(req("https://herinean.com/x/"), h.env, h.ctx);
  assert.equal(x.headers.get("content-type"), "text/html; charset=utf-8");
  const font = await worker.fetch(req("https://herinean.com/fonts/a.woff2"), h.env, h.ctx);
  assert.equal(font.headers.get("content-type"), "font/woff2");
});

test("the asset layer's 307 canonicalisation redirect becomes a 301 with the same location; POST is left alone", async () => {
  const h = harness({ assets: () => new Response(null, { status: 307, headers: { location: "https://herinean.com/writing/", "content-length": "18" } }) });
  const res = await worker.fetch(req("https://herinean.com/writing"), h.env, h.ctx);
  assert.equal(res.status, 301);
  assert.equal(res.headers.get("location"), "https://herinean.com/writing/");
  assert.equal(res.headers.get("content-length"), null);
  const post = await worker.fetch(req("https://herinean.com/writing", { method: "POST" }), h.env, h.ctx);
  assert.equal(post.status, 307);
  await h.settle();
  assert.deepEqual(h.points, [], "a redirect is not a view");
});
