import { test } from "node:test";
import assert from "node:assert/strict";
import { merge, ORDER } from "../merge.mjs";
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
