#!/usr/bin/env bash
# KV over the REST API: get/put/delete one key in the SCORECARD namespace (id from wrangler.toml).
# REST rather than `wrangler kv`: it returns metadata (the rollback stash needs it) and needs only the token.
# usage: kv.sh get KEY [--metadata] | kv.sh put KEY FILE [--metadata JSON] | kv.sh delete KEY
set -euo pipefail
cd "$(dirname "$0")/.."
. scripts/env.sh
NS=$(sed -n 's/^id = "\([0-9a-f]*\)"/\1/p' wrangler.toml | head -1)
[ -n "$NS" ] || { echo "kv.sh: no KV namespace id in wrangler.toml" >&2; exit 1; }
API="https://api.cloudflare.com/client/v4/accounts/$CLOUDFLARE_ACCOUNT_ID/storage/kv/namespaces/$NS"
auth=(-H "Authorization: Bearer $CLOUDFLARE_API_TOKEN")
cmd=${1:?usage}; key=${2:?key}; shift 2
case "$cmd" in
  get)
    if [ "${1:-}" = "--metadata" ]; then
      curl -sSf "${auth[@]}" "$API/metadata/$key" | jq -c '.result'
    else
      curl -sSf "${auth[@]}" "$API/values/$key"
    fi ;;
  put)
    file=${1:?file}; shift
    meta='{}'; [ "${1:-}" = "--metadata" ] && meta=${2:?json}
    curl -sSf "${auth[@]}" -X PUT "$API/values/$key" -F "value=@$file" -F "metadata=$meta" | jq -e '.success' >/dev/null ;;
  delete)
    curl -sSf "${auth[@]}" -X DELETE "$API/values/$key" | jq -e '.success' >/dev/null ;;
  *) echo "kv.sh: unknown command $cmd" >&2; exit 2 ;;
esac
