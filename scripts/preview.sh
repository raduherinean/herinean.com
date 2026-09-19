#!/usr/bin/env bash
# Builds, checks and uploads a preview version (never production). Prints the preview URL. CI does the same on PRs.
set -euo pipefail
cd "$(dirname "$0")/.."
[ -d node_modules/wrangler ] || npm ci --silent
export PATH="$HOME/.local/bin:$PATH"
. scripts/env.sh
mkdir -p .cache
go build -tags nodynamic -o .cache/site ./cmd/site
.cache/site build
.cache/site check --dist
out=$(npx wrangler versions upload 2>&1 | tee /dev/stderr)
url=$(printf '%s\n' "$out" | grep -Eo 'https://[a-z0-9-]+\.[a-z0-9-]+\.workers\.dev' | head -1 || true)
[ -n "$url" ] || { echo "no preview URL in wrangler output" >&2; exit 1; }
echo "$url" | tee .cache/preview-url
