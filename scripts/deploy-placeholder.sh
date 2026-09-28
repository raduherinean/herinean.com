#!/usr/bin/env bash
# Rollback to the M0 placeholder (one noindex line with the full header set). Production deploys are scripts/deploy.sh.
set -euo pipefail
cd "$(dirname "$0")/.."
[ -d node_modules/wrangler ] || npm ci --silent
. scripts/env.sh
rm -rf dist && cp -r infra/placeholder dist
npx wrangler deploy
