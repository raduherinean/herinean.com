import { row } from "./row.mjs";
import { sh } from "./sh.mjs";
export const name = "DNS, mail, domains (production)";
export const modes = ["post"];
// Row 13: the M0 edge suite (dig/HTTP sections; the API section needs zone scope the CI token
// does not have) is the one truth; this row wraps it rather than re-implementing its checks.
// ctx.verifyEdge lets tests point at a stub instead of the real script.
export async function run(ctx) {
  const script = ctx.verifyEdge || "scripts/verify-edge.sh";
  const r = sh(script, ["--ci"], { env: { ...process.env, PATH: process.env.PATH } });
  if (r.error) return [row(name, false, `verify-edge.sh --ci: could not run (${r.error.message})`, ctx, r.out + r.err)];
  const m = /(\d+) passed, (\d+) failed/.exec(r.out);
  const fails = r.out.split("\n").filter((l) => l.includes("  FAIL "));
  const pass = r.code === 0 && m && m[2] === "0";
  const failMsg = m ? `${fails.length} failed: ${fails.slice(0, 3).map((l) => l.trim()).join("; ")}` : `no summary line found (exit ${r.code})`;
  return [row(name, pass, pass ? `verify-edge.sh --ci: ${m[1]} checks (DNSSEC, CAA, MX/SPF/DKIM/DMARC/MTA-STS/TLS-RPT ×4 zones, redirects, apex headers)` : `verify-edge.sh --ci: ${failMsg}`, ctx, r.out)];
}
// (Domain expiry via RDAP is the weekly job's, M2c.)
