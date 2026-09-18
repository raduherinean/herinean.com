#!/usr/bin/env bash
# Builds, checks and uploads a preview version (never production). Prints the preview URL. CI (M2) does the same on PRs.
set -euo pipefail
cd "$(dirname "$0")/.."
export PATH="$HOME/.local/bin:$PATH"
. scripts/env.sh
mkdir -p .cache
go build -tags nodynamic -o .cache/site ./cmd/site
.cache/site build && .cache/site check --dist
out=$(npx --yes wrangler@4 versions upload 2>&1 | tee /dev/stderr)
url=$(printf '%s\n' "$out" | grep -Eo 'https://[a-z0-9-]+\.[a-z0-9-]+\.workers\.dev' | head -1)
[ -n "$url" ] || { echo "no preview URL in wrangler output" >&2; exit 1; }
echo "$url" | tee .cache/preview-url
