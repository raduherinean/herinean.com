import { fileURLToPath } from "node:url";
import { row } from "./row.mjs";
import { sh } from "./sh.mjs";
export const name = "Mozilla HTTP Observatory (production)";
export const modes = ["post"];

// The package's CLI entry (package.json bin: mdn-http-observatory-scan -> bin/wrapper.js), run
// via `node <wrapper.js> <host>` rather than through node_modules/.bin so it works regardless of
// cwd. It prints `{scan: {grade, score, ...}, tests: {...}}` on success, `{error}` on failure —
// exit 1 either way (see @mdn/mdn-http-observatory src/scan.js).
const CLI = fileURLToPath(new URL("../node_modules/@mdn/mdn-http-observatory/bin/wrapper.js", import.meta.url));

// Pure parser — unit-tested against canned CLI stdout, no network in tests.
export function parseScan(stdout) {
  const data = JSON.parse(stdout);
  if (data.error) return { error: data.error };
  return { grade: data.scan.grade, score: data.scan.score };
}

// Row 12: https://developer.mozilla.org/en-US/observatory/ — pass iff A+.
export async function run(ctx) {
  const host = new URL(ctx.base).host;
  const r = sh("node", [CLI, host]);
  let parsed;
  try {
    parsed = parseScan(r.out);
  } catch {
    return [row(name, false, `observatory: unparseable output (exit ${r.code}): ${(r.err || r.out || "<empty>").trim().slice(0, 200)}`, ctx, r.out + r.err)];
  }
  if (parsed.error) return [row(name, false, `observatory: ${parsed.error}`, ctx, r.out)];
  const result = row(name, parsed.grade === "A+", `${parsed.grade} (score ${parsed.score})`, ctx, parsed);
  // A row's `link` is normally "verify it yourself" (the CI run); this one points straight at the
  // Observatory's own report instead, which is more useful than the bench's own CI log here.
  result.link = `https://developer.mozilla.org/en-US/observatory/analyze?host=${host}`;
  result.link_text = "Observatory";
  return [result];
}
