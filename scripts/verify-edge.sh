#!/usr/bin/env bash
# Edge verification suite. Every M0 task appends a section. Exit 1 if anything fails.
set -uo pipefail
FAIL=0; PASS=0
section() { printf '\n== %s\n' "$1"; }
ok()   { PASS=$((PASS+1)); printf '  ok   %s\n' "$1"; }
bad()  { FAIL=$((FAIL+1)); printf '  FAIL %s\n' "$1"; }
hdrs() { curl -sS -o /dev/null -D - --max-time 15 "$@" 2>/dev/null; }
# expect_header URL HEADER REGEX   — header value (case-insensitive name) matches regex
expect_header() {
  local v; v=$(hdrs "$1" | awk -v h="$2" 'BEGIN{IGNORECASE=1} tolower($1)==tolower(h":"){sub(/^[^:]*: */,""); sub(/\r$/,""); print; exit}')
  if printf '%s' "$v" | grep -Eq -- "$3"; then ok "$1 $2: $v"; else bad "$1 $2 expected /$3/ got '${v:-<absent>}'"; fi
}
# expect_status URL CODE
expect_status() {
  local c; c=$(curl -sS -o /dev/null -w '%{http_code}' --max-time 15 "$1" 2>/dev/null)
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
  if curl -sS --max-time 15 "$1" 2>/dev/null | grep -Eq -- "$2"; then bad "$1 body matches forbidden /$2/"; else ok "$1 body free of /$2/"; fi
}
# expect_dig NAME TYPE REGEX — dig +short output matches
expect_dig() {
  local r; r=$(dig +short "$2" "$1" @1.1.1.1 | tr '\n' ' ')
  if printf '%s' "$r" | grep -Eq -- "$3"; then ok "$2 $1 ~ /$3/"; else bad "$2 $1 expected /$3/ got '${r:-<none>}'"; fi
}

section "preflight"
for t in curl dig jq; do command -v "$t" >/dev/null && ok "$t present" || bad "$t missing"; done

# --- task sections are appended below this line ---

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
