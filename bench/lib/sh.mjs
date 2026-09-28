// sh.mjs — run a command, capture stdout/stderr, never throw on a non-zero exit (callers decide).
import { spawnSync } from "node:child_process";
export function sh(cmd, args, opts = {}) {
  const r = spawnSync(cmd, args, { encoding: "utf8", maxBuffer: 64 << 20, ...opts });
  return { code: r.status ?? -1, out: r.stdout || "", err: r.stderr || "", error: r.error };
}
