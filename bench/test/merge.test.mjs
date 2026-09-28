import { test } from "node:test";
import assert from "node:assert/strict";
import { merge, ORDER, EXPECT } from "../merge.mjs";
const r = (check, pass, extra = {}) => ({ check, pass, value: pass ? "ok" : "bad", when: "2026-09-19", link: "", link_text: "", ...extra });

test("Lighthouse shards fold into one row; a missing shard is a failure; order follows the spec", () => {
  const rows = merge([r("i18n", true), r("Lighthouse [mobile 1/2]", true, { pages: 4, worst: 100 }), r("Lighthouse [mobile 2/2]", true, { pages: 3, worst: 100 }), r("Lighthouse [desktop 1/2]", true, { pages: 4, worst: 100 }), r("Lighthouse [desktop 2/2]", true, { pages: 3, worst: 100 })], { sha: "abc1234", url: "https://x/commit/abc1234" });
  assert.deepEqual(rows.map((x) => x.check), ["Audited build", "Lighthouse", "i18n"]);
  assert.match(rows[1].value, /7 pages × mobile \+ desktop/);
  const partial = merge([r("Lighthouse [mobile 1/2]", true, { pages: 4 })]);
  assert.equal(partial[0].pass, false);
  assert.match(partial[0].value, /3 shards missing/);
  assert.ok(ORDER.indexOf("Weight") > ORDER.indexOf("Security headers"));
});

// --expect: every row a mode's audit must deliver, derived from ORDER (counted here, not assumed).
test("EXPECT is derived from ORDER: 13 CI names from Lighthouse through Font correctness, 9 production names", () => {
  assert.deepEqual(EXPECT.ci, ORDER.slice(1, 14));
  assert.equal(EXPECT.ci[0], "Lighthouse");
  assert.equal(EXPECT.ci.at(-1), "Font correctness");
  assert.ok(!EXPECT.ci.includes("Audited build"), "Audited build comes from --build, never from a rows file");
  assert.equal(EXPECT.post.length, 9);
  assert.ok(EXPECT.post.every((c) => c.endsWith(" (production)")));
});

test("--expect ci with only Lighthouse rows adds 12 red 'not measured' rows in spec order and the gate goes red", () => {
  const rows = merge([r("Lighthouse [mobile 1/1]", true, { pages: 2, worst: 100 }), r("Lighthouse [desktop 1/1]", true, { pages: 2, worst: 100 })], { sha: "abc1234", url: "https://x/commit/abc1234" }, "ci");
  assert.deepEqual(rows.map((x) => x.check), ORDER.slice(0, 14));
  const missing = rows.filter((x) => x.value === "not measured (no rows file)");
  assert.equal(missing.length, 12);
  assert.ok(missing.every((x) => x.pass === false && /^\d{4}-\d{2}-\d{2}$/.test(x.when) && x.link === "" && x.link_text === ""));
  assert.equal(rows.find((x) => x.check === "Lighthouse").pass, true, "the delivered row keeps its own result");
  assert.equal(rows.filter((x) => !x.pass).length, 12, "the gate counts every not-measured row");
});

test("--expect post with all nine production rows adds nothing; a missing one is a red row", () => {
  const all = EXPECT.post.map((c) => (c.startsWith("Lighthouse") ? null : r(c, true))).filter(Boolean);
  const lh = [r("Lighthouse (production) [mobile 1/1]", true, { pages: 1, worst: 100 }), r("Lighthouse (production) [desktop 1/1]", true, { pages: 1, worst: 100 })];
  const rows = merge([...all, ...lh], null, "post");
  assert.deepEqual(rows.map((x) => x.check), EXPECT.post);
  assert.ok(rows.every((x) => x.pass));
  const short = merge(all.filter((x) => x.check !== "Caching (production)"), null, "post");
  assert.deepEqual(short.filter((x) => !x.pass).map((x) => [x.check, x.value]), [["Lighthouse (production)", "not measured (no rows file)"], ["Caching (production)", "not measured (no rows file)"]]);
});

test("--expect ci over no rows at all yields the full CI set, every row red 'not measured'", () => {
  const rows = merge([], null, "ci");
  assert.deepEqual(rows.map((x) => x.check), EXPECT.ci);
  assert.ok(rows.every((x) => !x.pass && x.value === "not measured (no rows file)"));
});

test("without --expect nothing is added (the comment and publish jobs re-merge already-merged files)", () => {
  const rows = merge([r("i18n", true), r("Caching (production)", true)]);
  assert.deepEqual(rows.map((x) => x.check), ["i18n", "Caching (production)"]);
  assert.throws(() => merge([], null, "nope"), /want ci or post/);
});
