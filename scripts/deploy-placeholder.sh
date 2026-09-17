#!/usr/bin/env bash
# Deploys the M0 placeholder: dist/ is rebuilt from infra/placeholder/ and pushed with wrangler.
# M1 replaces this with `site build && wrangler deploy` from CI.
set -euo pipefail
cd "$(dirname "$0")/.."
. scripts/env.sh
rm -rf dist && cp -r infra/placeholder dist
npx --yes wrangler@4 deploy
