#!/usr/bin/env bash
# Production deploy. Refuses without --yes-production: cutover is a launch decision (scorecard green, About + portrait, ≥ 2 pieces).
set -euo pipefail
cd "$(dirname "$0")/.."
[ -d node_modules/wrangler ] || npm ci --silent
export PATH="$HOME/.local/bin:$PATH"
[ "${1:-}" = "--yes-production" ] || { echo "usage: scripts/deploy.sh --yes-production   (preview: scripts/preview.sh)" >&2; exit 2; }
. scripts/env.sh
mkdir -p .cache
go build -tags nodynamic -o .cache/site ./cmd/site
.cache/site check
.cache/site build
.cache/site check --dist
# The colophon is filled from KV; publishing the scorecard is part of every deploy, never a step to remember.
# A manual deploy is unaudited by definition: say so on the colophon instead of pretending a CI row is pending.
if [ ! -f scorecard.json ]; then
  printf '[{"check":"Audited build","pass":false,"value":"%s — manual deploy, not audited by CI","when":"%s","link":"","link_text":""}]\n' "$(git rev-parse --short HEAD)" "$(date -u +%F)" > .cache/scorecard-manual-deploy.json
  scripts/scorecard-publish.sh .cache/scorecard-manual-deploy.json
else
  scripts/scorecard-publish.sh scorecard.json
fi
npx wrangler deploy
scripts/verify-edge.sh
scripts/verify-preview.sh https://herinean.com --prod
