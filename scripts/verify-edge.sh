#!/usr/bin/env bash
# Edge verification suite. Every M0 task appends a section. Exit 1 if anything fails.
set -uo pipefail
FAIL=0; PASS=0
section() { printf '\n== %s\n' "$1"; }
ok()   { PASS=$((PASS+1)); printf '  ok   %s\n' "$1"; }
bad()  { FAIL=$((FAIL+1)); printf '  FAIL %s\n' "$1"; }
# curl through authoritative answers (1.1.1.1), so local resolver caches never fake a result
rcurl() {
  local url=$1; shift
  local host; host=$(printf '%s' "$url" | sed -E 's#^[a-z]+://([^/:]+).*#\1#')
  local ip; ip=$(dig +short A "$host" @1.1.1.1 | head -1)
  if [ -n "$ip" ]; then curl --resolve "$host:443:$ip" --resolve "$host:80:$ip" "$@" "$url"; else curl "$@" "$url"; fi
}
hdrs() { rcurl "$1" -sS -o /dev/null -D - --max-time 15 2>/dev/null; }
# expect_header URL HEADER REGEX   — header value (case-insensitive name) matches regex
expect_header() {
  local v; v=$(hdrs "$1" | awk -v h="$2" 'BEGIN{IGNORECASE=1} tolower($1)==tolower(h":"){sub(/^[^:]*: */,""); sub(/\r$/,""); print; exit}')
  if printf '%s' "$v" | grep -Eq -- "$3"; then ok "$1 $2: $v"; else bad "$1 $2 expected /$3/ got '${v:-<absent>}'"; fi
}
# expect_status URL CODE
expect_status() {
  local c; c=$(rcurl "$1" -sS -o /dev/null -w '%{http_code}' --max-time 15 2>/dev/null)
  if [ "$c" = "$2" ]; then ok "$1 → $c"; else bad "$1 expected $2 got $c"; fi
}
# expect_location URL EXPECTED  — first response is a redirect to EXPECTED
expect_location() {
  local l; l=$(hdrs "$1" | awk 'BEGIN{IGNORECASE=1} tolower($1)=="location:"{print $2; exit}' | tr -d '\r')
  if [ "$l" = "$2" ]; then ok "$1 → $l"; else bad "$1 expected Location $2 got '${l:-<absent>}'"; fi
}
# expect_txt NAME REGEX — some TXT record at NAME matches (via 1.1.1.1)
expect_txt() {
  local t; t=$(dig +short TXT "$1" @1.1.1.1 | tr -d '"' | tr '\n' ' ')
  if printf '%s' "$t" | grep -Eq -- "$2"; then ok "TXT $1 ~ /$2/"; else bad "TXT $1 expected /$2/ got '${t:-<none>}'"; fi
}
# expect_no_body URL REGEX — body must NOT match
expect_no_body() {
  if rcurl "$1" -sS --max-time 15 2>/dev/null | grep -Eq -- "$2"; then bad "$1 body matches forbidden /$2/"; else ok "$1 body free of /$2/"; fi
}
# expect_dig NAME TYPE REGEX — dig +short output matches
expect_dig() {
  local r; r=$(dig +short "$2" "$1" @1.1.1.1 | tr '\n' ' ')
  if printf '%s' "$r" | grep -Eq -- "$3"; then ok "$2 $1 ~ /$3/"; else bad "$2 $1 expected /$3/ got '${r:-<none>}'"; fi
}

section "preflight"
for t in curl dig jq; do command -v "$t" >/dev/null && ok "$t present" || bad "$t missing"; done

# --- task sections are appended below this line ---

section "zone settings (API)"
cf() { curl -sS -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" "https://api.cloudflare.com/client/v4$1"; }
for z in $ZONE_COM $ZONE_RO $ZONE_NET $ZONE_INFO; do
  for pair in min_tls_version:1.3 tls_1_3:on http3:on 0rtt:off always_use_https:on ipv6:on email_obfuscation:off rocket_loader:off fonts:off speed_brain:off browser_check:on; do
    s=${pair%%:*}; want=${pair##*:}
    got=$(cf "/zones/$z/settings/$s" | jq -r '.result.value // empty')
    [ "$got" = "$want" ] && ok "$z $s=$got" || bad "$z $s expected $want got '${got:-<none>}'"
  done
  h=$(cf "/zones/$z/settings/security_header" | jq -c '.result.value.strict_transport_security | {enabled,max_age,include_subdomains,preload}')
  [ "$h" = '{"enabled":true,"max_age":63072000,"include_subdomains":true,"preload":true}' ] && ok "$z hsts $h" || bad "$z hsts got $h"
  bfm=$(cf "/zones/$z/bot_management" | jq -r '.result.fight_mode')
  [ "$bfm" = "false" ] && ok "$z bot fight mode off" || bad "$z bot fight mode = $bfm"
done


section "redirects and CAA"
for d in herinean.ro herinean.net herinean.info; do
  expect_location "https://$d/writing/x/?ref=li" "https://herinean.com/writing/x/?ref=li"
  expect_location "https://www.$d/" "https://herinean.com/"
  expect_location "http://$d/" "https://herinean.com/"     # the redirect rule fires before the HTTPS upgrade: one hop
  expect_header "https://$d/" strict-transport-security 'max-age=63072000; includeSubDomains; preload'
done
expect_location "https://www.herinean.com/a/?b=1" "https://herinean.com/a/?b=1"
for d in herinean.com herinean.ro herinean.net herinean.info; do
  expect_dig "$d" CAA 'issue "letsencrypt.org"'
  expect_dig "$d" CAA 'iodef "mailto:security@herinean.com"'
done


section "mail DNS"
for d in herinean.com herinean.ro herinean.net herinean.info; do
  expect_dig "$d" MX '^1 smtp\.google\.com\.'
  expect_txt "$d" 'v=spf1 include:_spf\.google\.com -all'
  expect_txt "_dmarc.$d" "v=DMARC1; p=(none|reject); adkim=s; aspf=s; rua=mailto:[a-f0-9]+@dmarc-reports\.cloudflare\.net,mailto:dmarc@$d"
done
expect_txt _dmarc.herinean.ro 'p=reject'
expect_txt _dmarc.herinean.info 'p=reject'
expect_txt google._domainkey.herinean.com 'v=DKIM1; k=rsa; p='
expect_txt _mta-sts.herinean.com 'v=STSv1; id='
expect_txt _smtp._tls.herinean.com 'v=TLSRPTv1; rua=mailto:dmarc@herinean\.com'
expect_txt herinean.net 'google-site-verification='     # pre-existing, must survive untouched
expect_txt herinean.ro 'google-site-verification='

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
