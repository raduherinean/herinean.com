#!/usr/bin/env bash
# Production deploy. Refuses without --yes-production: cutover is a launch decision (scorecard green, About + portrait, ≥ 2 pieces).
set -euo pipefail
cd "$(dirname "$0")/.."
export PATH="$HOME/.local/bin:$PATH"
[ "${1:-}" = "--yes-production" ] || { echo "usage: scripts/deploy.sh --yes-production   (preview: scripts/preview.sh)" >&2; exit 2; }
. scripts/env.sh
mkdir -p .cache
go build -tags nodynamic -o .cache/site ./cmd/site
.cache/site check
.cache/site build
.cache/site check --dist
# The colophon is filled from KV; publishing the scorecard is part of every deploy, never a step to remember.
scripts/scorecard-publish.sh
npx --yes "$WRANGLER" deploy
scripts/verify-edge.sh
scripts/verify-preview.sh https://herinean.com --prod
