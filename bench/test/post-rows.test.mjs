import { test } from "node:test";
import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";
import { mkdtempSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { sh } from "../lib/sh.mjs";
import { run as dnsRun } from "../lib/dns.mjs";
import { tls12Problem, tls13Problem, http3Problem, aaaaProblem } from "../lib/transport.mjs";
import { cachingProblems } from "../lib/caching.mjs";
import { parseScan } from "../lib/observatory.mjs";
import { curlH3, platformKeyFor } from "../lib/tools.mjs";

const ctxFor = (extra = {}) => ({ base: "https://example.test", mode: "post", when: "2026-09-19", link: "", linkText: "", ...extra });

// --- dns.mjs: exercised against stub scripts, never the real scripts/verify-edge.sh ---

const FAIL_STUB = fileURLToPath(new URL("./fixtures/verify-edge-stub.sh", import.meta.url));
const PASS_STUB = fileURLToPath(new URL("./fixtures/verify-edge-stub-pass.sh", import.meta.url));

test("dns fails and names the failure when verify-edge.sh --ci reports a FAIL line", async () => {
  const [r] = await dnsRun(ctxFor({ verifyEdge: FAIL_STUB }));
  assert.equal(r.check, "DNS, mail, domains (production)");
  assert.equal(r.pass, false);
  assert.match(r.value, /1 failed/);
  assert.match(r.value, /CAA/);
});

test("dns passes when verify-edge.sh --ci reports 0 failed", async () => {
  const [r] = await dnsRun(ctxFor({ verifyEdge: PASS_STUB }));
  assert.equal(r.pass, true);
  assert.match(r.value, /5 checks/);
});

test("dns reports a red row, not a crash, when the script itself cannot run", async () => {
  const [r] = await dnsRun(ctxFor({ verifyEdge: "/no/such/script.sh" }));
  assert.equal(r.pass, false);
  assert.match(r.value, /could not run/);
});

// --- transport.mjs: pure decision helpers over canned sh() output, no network ---

test("tls12Problem: refusal (protocol alert / wrong version / handshake failure) means the row is clean", () => {
  assert.equal(tls12Problem({ out: "", err: "140736... SSL alert number 70\n...:alert protocol version:..." }), null);
  assert.equal(tls12Problem({ out: "", err: "...:wrong version number:..." }), null);
  assert.equal(tls12Problem({ out: "", err: "...sslv3 alert handshake failure:..." }), null);
});
test("tls12Problem: a completed handshake (no refusal marker) is a problem", () => {
  assert.equal(tls12Problem({ out: "New, TLSv1.2, Cipher is ECDHE-RSA-AES128-GCM-SHA256", err: "" }), "TLS 1.2 was accepted");
});

test("tls13Problem: TLSv1.3 in stdout is clean; anything else is a problem", () => {
  assert.equal(tls13Problem({ out: "Protocol  : TLSv1.3\nNew, TLSv1.3, Cipher is TLS_AES_256_GCM_SHA384" }), null);
  assert.equal(tls13Problem({ out: "" }), "TLS 1.3 not negotiated");
  assert.equal(tls13Problem({ out: "errno=104" }), "TLS 1.3 not negotiated");
});

test("http3Problem: '3' from -w %{http_version} is clean; anything else names what curl reported", () => {
  assert.equal(http3Problem({ out: "3", err: "" }), null);
  assert.equal(http3Problem({ out: "3\n", err: "" }), null); // curl -w output is not newline-trimmed by curl itself
  assert.equal(http3Problem({ out: "2", err: "" }), "HTTP/3: got '2'");
  assert.equal(http3Problem({ out: "", err: "curl: (92) HTTP/3 stream 0 reset" }), "HTTP/3: got 'curl: (92) HTTP/3 stream 0 reset'");
});

test("aaaaProblem: an IPv6 literal on exit 0 is clean; an empty answer, a non-address answer or a non-zero exit is a problem", () => {
  assert.equal(aaaaProblem({ code: 0, out: "2606:4700::1\n" }), null);
  assert.equal(aaaaProblem({ code: 0, out: "2606:4700:3030::6815:4001\n2606:4700:3030::ac43:d4a5\n" }), null);
  assert.equal(aaaaProblem({ code: 0, out: "" }), "no AAAA record");
  assert.equal(aaaaProblem({ code: 0, out: "\n" }), "no AAAA record");
  // dig prints its own errors to stdout; non-empty stdout alone must never read as "AAAA present"
  assert.equal(aaaaProblem({ code: 0, out: ";; connection timed out; no servers could be reached\n" }), "AAAA: dig answered ';; connection timed out; no servers could be reached', not an IPv6 address");
  assert.equal(aaaaProblem({ code: 9, out: ";; connection timed out; no servers could be reached\n" }), "AAAA: dig exit 9: ;; connection timed out; no servers could be reached");
  assert.equal(aaaaProblem({ code: 9, out: "2606:4700::1\n" }), "AAAA: dig exit 9: 2606:4700::1");
});

// The same helper over a real process: a stub `dig` on disk, run through sh() exactly as run() runs
// the real one, so the {code, out, err} shape the helper sees is the one spawnSync produces.
function stubDig(script) {
  const dir = mkdtempSync(join(tmpdir(), "bench-dig-"));
  const f = join(dir, "dig");
  writeFileSync(f, `#!/bin/sh\n${script}\n`, { mode: 0o755 });
  return { f, rm: () => rmSync(dir, { recursive: true, force: true }) };
}
test("aaaaProblem: a stubbed dig that times out (its error on stdout, exit 9) is a red row; a real-looking AAAA answer is green", () => {
  const timeout = stubDig('echo ";; connection timed out; no servers could be reached"; exit 9');
  try {
    const r = sh(timeout.f, ["+short", "AAAA", "example.test", "@1.1.1.1"]);
    assert.equal(r.code, 9);
    assert.match(aaaaProblem(r), /^AAAA: dig exit 9: ;; connection timed out/);
  } finally {
    timeout.rm();
  }
  const ok = stubDig('echo "2606:4700:3030::6815:4001"; exit 0');
  try {
    assert.equal(aaaaProblem(sh(ok.f, ["+short", "AAAA", "example.test", "@1.1.1.1"])), null);
  } finally {
    ok.rm();
  }
});

// --- caching.mjs: cachingProblems() over canned plain-object responses, no network ---

const GOOD = {
  home: { status: 200, cacheControl: "public, max-age=0, must-revalidate", etag: 'W/"abc"', notModifiedStatus: 304 },
  font: { found: true, status: 200, cacheControl: "public, max-age=31536000, immutable" },
  ogImage: { found: true, status: 200, cacheControl: "public, max-age=31536000, immutable" },
  feed: { status: 200, cacheControl: "public, max-age=300", contentType: "application/rss+xml; charset=utf-8" },
  securityTxt: { status: 200, contentType: "text/plain; charset=utf-8" },
  colophon: { status: 200, etag: 'W/"6dfadd4d14d9-1a2b3c4d"', notModifiedStatus: 304 },
};

test("cachingProblems: a fully correct response set has no problems", () => {
  assert.deepEqual(cachingProblems(GOOD), []);
});

test("cachingProblems: reports the placeholder's shape — no colophon, no font, no og:image", () => {
  const placeholder = {
    ...GOOD,
    font: { found: false, status: null, cacheControl: null },
    ogImage: { found: false, status: null, cacheControl: null },
    colophon: { status: 404, etag: null, notModifiedStatus: null },
  };
  const problems = cachingProblems(placeholder);
  assert.ok(problems.some((p) => p.includes("no preloaded font link")));
  assert.ok(problems.some((p) => p.includes("no og:image")));
  assert.ok(problems.some((p) => p.includes("/colophon/: 404")));
});

test("cachingProblems: a missing etag or a failed If-None-Match round trip on / is a problem", () => {
  assert.deepEqual(cachingProblems({ ...GOOD, home: { ...GOOD.home, etag: null } }), ["/: no etag"]);
  assert.deepEqual(cachingProblems({ ...GOOD, home: { ...GOOD.home, notModifiedStatus: 200 } }), ["/: If-None-Match round trip got 200, want 304"]);
});

test("cachingProblems: a colophon etag that isn't the composed W/\"<hex>-<8hex>\" form is a problem", () => {
  const problems = cachingProblems({ ...GOOD, colophon: { status: 200, etag: 'W/"not-the-right-shape"', notModifiedStatus: 304 } });
  assert.ok(problems.some((p) => p.includes("composed")));
});

// --- observatory.mjs: parseScan() over canned CLI stdout, no network ---

test("parseScan: an A+ scan reports grade and score", () => {
  assert.deepEqual(parseScan(JSON.stringify({ scan: { grade: "A+", score: 140 } })), { grade: "A+", score: 140 });
});
test("parseScan: a lower grade still parses (the row decides pass/fail, not the parser)", () => {
  assert.deepEqual(parseScan(JSON.stringify({ scan: { grade: "B", score: 65 } })), { grade: "B", score: 65 });
});
test("parseScan: the CLI's own error shape surfaces as {error}", () => {
  assert.deepEqual(parseScan(JSON.stringify({ error: "invalid-hostname-lookup" })), { error: "invalid-hostname-lookup" });
});
test("parseScan: unparseable output throws rather than silently passing", () => {
  assert.throws(() => parseScan("not json"));
});

// --- tools.mjs: platformKeyFor is pure; curlH3() is network+download, so it's skipped unless the
// system curl already has HTTP3 (never true on this dev host's curl 8.5.0) ---

test("platformKeyFor: only linux x64/arm64 are pinned; everything else is unsupported", () => {
  assert.equal(platformKeyFor("linux", "x64"), "linux-x64");
  assert.equal(platformKeyFor("linux", "arm64"), "linux-arm64");
  assert.equal(platformKeyFor("darwin", "arm64"), null);
  assert.equal(platformKeyFor("linux", "ia32"), null);
});

test("curlH3 returns the system curl when it already has HTTP3; otherwise this is skipped (no download in tests)", async (t) => {
  const sys = sh("curl", ["--version"]);
  if (sys.error || !/HTTP3/.test(sys.out)) {
    t.skip("system curl lacks HTTP3 on this host; skipping to avoid a network download in unit tests");
    return;
  }
  const p = await curlH3();
  assert.equal(typeof p, "string");
  assert.ok(p.endsWith("curl"));
});
