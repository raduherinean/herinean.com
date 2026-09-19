import { row, summary } from "./row.mjs";
import { sh } from "./sh.mjs";
import { curlH3 } from "./tools.mjs";
export const name = "Transport (production)";
export const modes = ["post"];

// Pure decision helpers — unit-tested against canned sh() output, no network in tests.
export const tls12Problem = ({ out, err }) => (/alert protocol version|wrong version number|handshake failure/.test((err || "") + (out || "")) ? null : "TLS 1.2 was accepted");
export const tls13Problem = ({ out }) => (/TLSv1\.3/.test(out || "") ? null : "TLS 1.3 not negotiated");
export const http3Problem = ({ out, err }) => { const v = (out || "").trim(); return v === "3" ? null : `HTTP/3: got '${v || (err || "").trim()}'`; };
// dig writes its own failures to STDOUT (";; connection timed out; no servers could be reached") and
// exits non-zero, so "stdout is non-empty" is not "an AAAA record exists": the exit code must be 0 AND
// at least one output line must be an IPv6 literal.
export const aaaaProblem = ({ code, out, err }) => {
  const lines = (out || "").split("\n").map((l) => l.trim()).filter(Boolean);
  if (code !== 0) return `AAAA: dig exit ${code}: ${lines[0] || (err || "").trim().split("\n")[0] || "<no output>"}`;
  if (lines.some((l) => /^[0-9a-f:]+$/i.test(l) && l.includes(":"))) return null;
  return lines.length ? `AAAA: dig answered '${lines[0]}', not an IPv6 address` : "no AAAA record";
};

// Row 10: TLS 1.3-only, HTTP/3 negotiated (an h3-capable curl; Alt-Svc alone is not proof), IPv6
// AAAA present; 0-RTT is reported as configured off (infra/settings.tf) — a runner cannot provoke
// early data to measure it directly.
export async function run(ctx) {
  const host = new URL(ctx.base).host;
  const t12 = sh("openssl", ["s_client", "-connect", `${host}:443`, "-servername", host, "-tls1_2"], { input: "" });
  const t13 = sh("openssl", ["s_client", "-connect", `${host}:443`, "-servername", host, "-tls1_3"], { input: "" });
  const curl = await curlH3();
  const h3 = sh(curl, ["-sS", "--http3-only", "-o", "/dev/null", "-w", "%{http_version}", "--max-time", "15", ctx.base + "/"]);
  const aaaa = sh("dig", ["+short", "AAAA", host, "@1.1.1.1"]);
  const problems = [tls12Problem(t12), tls13Problem(t13), http3Problem(h3), aaaaProblem(aaaa)].filter(Boolean);
  return [row(name, problems.length === 0, summary("TLS 1.3 only (1.2 refused); HTTP/3 negotiated; AAAA present; 0-RTT configured off (infra/settings.tf), not measured", problems), ctx, problems)];
}
