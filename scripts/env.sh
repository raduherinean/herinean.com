#!/usr/bin/env bash
# Source this: `. scripts/env.sh`. Loads operator inputs from outside the repo — or, in CI, takes the two
# variables the workflow exports and asks for nothing else. The infra scripts need the zone ids and refuse to run without the file.
set -euo pipefail
if [ -n "${CLOUDFLARE_API_TOKEN:-}" ] && [ -n "${CLOUDFLARE_ACCOUNT_ID:-}" ] && [ -z "${HERINEAN_ENV_FILE:-}" ]; then
  export TF_VAR_account_id="$CLOUDFLARE_ACCOUNT_ID"
  return 0 2>/dev/null || exit 0
fi
ENV_FILE="${HERINEAN_ENV_FILE:-${HOME}/.config/herinean/m0.env}"
if [ ! -r "$ENV_FILE" ]; then
  echo "missing $ENV_FILE — see docs/plans/2026-09-17-m0-edge-foundation.md 'Operator inputs'" >&2
  return 1 2>/dev/null || exit 1
fi
# shellcheck disable=SC1090
. "$ENV_FILE"
for v in CLOUDFLARE_API_TOKEN CLOUDFLARE_ACCOUNT_ID ZONE_COM ZONE_RO ZONE_NET ZONE_INFO GITHUB_OWNER GITEA_REMOTE DKIM_TXT; do
  [ -n "${!v:-}" ] || { echo "$v is empty in $ENV_FILE" >&2; return 1 2>/dev/null || exit 1; }
done
export CLOUDFLARE_API_TOKEN CLOUDFLARE_ACCOUNT_ID
export TF_VAR_account_id="$CLOUDFLARE_ACCOUNT_ID"
