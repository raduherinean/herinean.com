#!/usr/bin/env bash
# Source this: `. scripts/env.sh`. Loads operator inputs for M0 from outside the repo.
set -euo pipefail
ENV_FILE="${HOME}/.config/herinean/m0.env"
if [ ! -r "$ENV_FILE" ]; then
  echo "missing $ENV_FILE — see docs/plans/2026-09-17-m0-edge-foundation.md 'Operator inputs'" >&2
  return 1 2>/dev/null || exit 1
fi
# shellcheck disable=SC1090
. "$ENV_FILE"
for v in CLOUDFLARE_API_TOKEN CLOUDFLARE_ACCOUNT_ID ZONE_COM ZONE_RO ZONE_NET ZONE_INFO GITHUB_OWNER GITEA_REMOTE DKIM_TXT; do
  [ -n "${!v:-}" ] || { echo "$v is empty in $ENV_FILE" >&2; return 1 2>/dev/null || exit 1; }
done
export TF_VAR_account_id="$CLOUDFLARE_ACCOUNT_ID"
