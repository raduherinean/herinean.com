#!/usr/bin/env bash
# Runs the audit bench locally against a built dist/ (or a URL). The colophon shows CI's numbers; this is for debugging a red row.
# usage: scripts/bench.sh [BASE] [--post] [--only lighthouse|checks]
set -euo pipefail
cd "$(dirname "$0")/.."
export PATH="$HOME/.local/bin:$PATH"
MODE=ci; BASE=""; ONLY=()
while [ $# -gt 0 ]; do
  case "$1" in
    --post) MODE=post; shift ;;
    --only) ONLY=(--only "$2"); shift 2 ;;
    *) BASE=$1; shift ;;
  esac
done
[ -d bench/node_modules ] || (cd bench && npm ci --silent)
(cd bench && npx playwright install chromium >/dev/null)
JAVA_OK=0
if command -v java >/dev/null; then
  JAVA_VER=$(java -version 2>&1 | awk -F'"' '/version/ { print $2; exit }')
  JAVA_MAJOR=$(printf '%s\n' "$JAVA_VER" | awk -F. '{ if ($1 == "1") print $2; else print $1 }')
  case "$JAVA_MAJOR" in ''|*[!0-9]*) JAVA_MAJOR=0 ;; esac
  [ "$JAVA_MAJOR" -ge 11 ] && JAVA_OK=1
fi
[ "$JAVA_OK" -eq 1 ] || echo "bench: no JRE >= 11 — the Nu half of the HTML row will report 'Nu did not run' (sudo apt install -y default-jre-headless)" >&2
mkdir -p .cache/bench
if [ -z "$BASE" ]; then
  go build -tags nodynamic -o .cache/site ./cmd/site
  .cache/site build
  .cache/site check --dist
  .cache/site serve --static --port 8089 >/dev/null 2>&1 &
  SRV=$!; trap 'kill $SRV 2>/dev/null || true' EXIT
  for _ in $(seq 1 50); do curl -sf -o /dev/null http://127.0.0.1:8089/ && break; sleep 0.2; done
  BASE=http://127.0.0.1:8089
fi
node bench/audit.mjs "$BASE" --mode "$MODE" "${ONLY[@]}" --dist dist --out .cache/bench/rows.json
node bench/merge.mjs --out .cache/bench/scorecard.json --build "$(git rev-parse --short HEAD)" --build-url "https://github.com/raduherinean/herinean.com/commit/$(git rev-parse HEAD)" --no-gate .cache/bench/rows.json
echo "rows: .cache/bench/scorecard.json (detail: .cache/bench/rows.json.detail.json)"
