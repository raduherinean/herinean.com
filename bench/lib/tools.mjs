// tools.mjs — the h3-capable curl `transport.mjs` needs. Most machines' system curl has no
// HTTP/3 support (this dev host's curl 8.5.0 does not); when that's true we fall back to a
// pinned static build (bench/tools.json: stunnel/static-curl) downloaded once into
// bench/.cache/bin/ (gitignored) and verified by sha256 before it is ever executed.
import { createHash } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, renameSync, rmSync, writeFileSync, chmodSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { sh } from "./sh.mjs";

const BENCH_DIR = fileURLToPath(new URL("..", import.meta.url));
const TOOLS = JSON.parse(readFileSync(join(BENCH_DIR, "tools.json"), "utf8"));

// The only two platforms bench/tools.json pins; anything else is a loud failure, not a silent skip.
export function platformKeyFor(platform, arch) {
  if (platform !== "linux") return null;
  if (arch === "x64") return "linux-x64";
  if (arch === "arm64") return "linux-arm64";
  return null;
}

function sha256File(path) {
  return createHash("sha256").update(readFileSync(path)).digest("hex");
}

// Returns "curl" (the system binary) when it already lists HTTP3; otherwise downloads, verifies
// and extracts the pinned static build, returning its path. Throws — never returns a bad path —
// on an unsupported platform, a download failure, a checksum mismatch or a failed extraction.
export async function curlH3() {
  const sys = sh("curl", ["--version"]);
  if (!sys.error && /HTTP3/.test(sys.out)) return "curl";

  const key = platformKeyFor(process.platform, process.arch);
  const cfg = key && TOOLS["curl-h3"][key];
  if (!cfg) throw new Error(`no pinned h3 curl for platform ${process.platform}/${process.arch}; add it to bench/tools.json`);

  const version = TOOLS["curl-h3"].version;
  const cacheDir = join(BENCH_DIR, ".cache", "bin");
  mkdirSync(cacheDir, { recursive: true });
  const binPath = join(cacheDir, `curl-h3-${version}-${key}`);
  if (existsSync(binPath)) return binPath;

  const archivePath = join(cacheDir, `curl-h3-${version}-${key}.tar.xz`);
  if (!existsSync(archivePath)) {
    const res = await fetch(cfg.url);
    if (!res.ok) throw new Error(`curlH3: download failed: ${cfg.url} -> ${res.status}`);
    writeFileSync(archivePath, Buffer.from(await res.arrayBuffer()));
  }
  const actual = sha256File(archivePath);
  if (actual !== cfg.sha256) {
    rmSync(archivePath, { force: true }); // don't leave a bad archive around to fool the next run
    throw new Error(`curlH3: checksum mismatch for ${cfg.url}: expected ${cfg.sha256}, got ${actual}`);
  }

  const ext = sh("tar", ["-xJf", archivePath, "-C", cacheDir, cfg.member]);
  if (ext.error || ext.code !== 0) throw new Error(`curlH3: tar extraction failed: ${ext.err.trim() || ext.error?.message || `exit ${ext.code}`}`);
  renameSync(join(cacheDir, cfg.member), binPath);
  chmodSync(binPath, 0o755);
  return binPath;
}
