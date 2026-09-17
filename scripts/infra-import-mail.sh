#!/usr/bin/env bash
# Prints `tofu import` commands for pre-existing mail records on all four zones. Run from repo root before the first apply of dns-mail.tf.
set -euo pipefail
. "$(dirname "$0")/env.sh"
cf() { curl -sS -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" "https://api.cloudflare.com/client/v4$1"; }
for pair in com:$ZONE_COM ro:$ZONE_RO net:$ZONE_NET info:$ZONE_INFO; do
  z=${pair%%:*}; id=${pair##*:}
  cf "/zones/$id/dns_records?per_page=200" | jq -r --arg z "$z" --arg id "$id" '
    .result[] as $r
    | if ($r.type=="MX" and ($r.name|test("^herinean\\.")))
        then "cd infra && tofu import '"'"'cloudflare_dns_record.mx[\"\($z).\($r.content)\"]'"'"' \($id)/\($r.id) && cd .."
      elif ($r.type=="TXT" and ($r.name|test("^herinean\\.")) and ($r.content|test("v=spf1")))
        then "cd infra && tofu import '"'"'cloudflare_dns_record.spf[\"\($z)\"]'"'"' \($id)/\($r.id) && cd .."
      elif ($r.type=="TXT" and ($r.name|test("^_dmarc\\.")))
        then "cd infra && tofu import '"'"'cloudflare_dns_record.dmarc[\"\($z)\"]'"'"' \($id)/\($r.id) && cd .."
      elif ($r.type=="TXT" and $r.name=="google._domainkey.herinean.com")
        then "cd infra && tofu import cloudflare_dns_record.com_dkim \($id)/\($r.id) && cd .."
      else empty end'
done | sort -u
