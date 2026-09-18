#!/usr/bin/env bash
# Verifies a built site at BASE (a preview URL or wrangler dev). Exit 1 if anything fails.
set -uo pipefail
BASE=${1:?usage: verify-preview.sh https://…workers.dev}; BASE=${BASE%/}
FAIL=0; PASS=0
ok()  { PASS=$((PASS+1)); printf '  ok   %s\n' "$1"; }
bad() { FAIL=$((FAIL+1)); printf '  FAIL %s\n' "$1"; }
hdr() { curl -sS -o /dev/null -D - --max-time 20 "$BASE$1" 2>/dev/null | awk -v h="$2" 'BEGIN{IGNORECASE=1} tolower($1)==tolower(h":"){sub(/^[^:]*: */,""); sub(/\r$/,""); print; exit}'; }
status() { curl -sS -o /dev/null -w '%{http_code}' --max-time 20 "$BASE$1" 2>/dev/null; }
expect_status() { local c; c=$(status "$1"); [ "$c" = "$2" ] && ok "$1 → $c" || bad "$1 expected $2 got $c"; }
expect_header() { local v; v=$(hdr "$1" "$2"); printf '%s' "$v" | grep -Eq -- "$3" && ok "$1 $2: $v" || bad "$1 $2 expected /$3/ got '${v:-<absent>}'"; }
expect_body() { curl -sS --max-time 20 "$BASE$1" | grep -Eq -- "$2" && ok "$1 body ~ /$2/" || bad "$1 body lacks /$2/"; }
expect_no_body() { curl -sS --max-time 20 "$BASE$1" | grep -Eq -- "$2" && bad "$1 body matches forbidden /$2/" || ok "$1 body free of /$2/"; }

printf '\n== pages\n'
for p in / /ro/ /writing/ /ro/articole/ /colophon/ /privacy/ /ro/confidentialitate/; do expect_status "$p" 200; done
expect_status /nope/ 404; expect_body /nope/ 'Pagina nu există'
expect_status /writing 301; expect_header /writing location '/writing/$'
printf '\n== headers (asset layer applies _headers; the Worker adds only the charset)\n'
expect_header / content-security-policy "^default-src 'none'; style-src 'sha256-[A-Za-z0-9+/=]+'; img-src 'self'; font-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'$"
expect_header / strict-transport-security 'max-age=63072000; includeSubDomains; preload'
expect_header / cross-origin-resource-policy '^same-origin$'
expect_header / cache-control '^public, max-age=0, must-revalidate$'
expect_header / permissions-policy 'camera=\(\)'
expect_header / content-type '^text/html; charset=utf-8$'
printf '\n== fonts, images, feeds\n'
font=$(curl -sS --max-time 20 "$BASE/" | grep -Eo '<link rel="preload" href="[^"]+"' | grep -Eo '/fonts/[^"]+' | head -1)
[ -n "$font" ] && ok "preload found: $font" || bad "no font preload on /"
expect_header "$font" content-type '^font/woff2$'
expect_header "$font" cache-control '^public, max-age=31536000, immutable$'
og=$(curl -sS --max-time 20 "$BASE/" | grep -Eo '/og/[A-Za-z0-9.-]+\.png' | head -1)
[ -n "$og" ] && ok "og image found: $og" || bad "no og:image on /"
expect_header "$og" cross-origin-resource-policy '^cross-origin$'
expect_header "$og" cache-control 'immutable'
expect_header /feed.xml content-type '^application/rss\+xml; charset=utf-8$'
expect_header /feed.json content-type '^application/feed\+json; charset=utf-8$'
expect_header /.well-known/security.txt content-type '^text/plain; charset=utf-8$'
expect_header /llms.txt content-type '^text/plain; charset=utf-8$'
expect_header /robots.txt content-type '^text/plain; charset=utf-8$'
printf '\n== privacy: no executable script, no cookie, no cdn-cgi\n'
for p in / /colophon/; do
  n=$(curl -sS --max-time 20 "$BASE$p" | grep -o '<script' | wc -l); [ "$n" = 1 ] && ok "$p has one <script> (JSON-LD)" || bad "$p has $n <script> tags"
  expect_no_body "$p" '/cdn-cgi/'
  c=$(hdr "$p" set-cookie); [ -z "$c" ] && ok "$p no Set-Cookie" || bad "$p sets a cookie: $c"
done
printf '\n== preview marking (skip with --prod)\n'
if [ "${2:-}" != "--prod" ]; then
  expect_header / x-robots-tag 'noindex'
  expect_body /robots.txt '^Disallow: /$'
else
  v=$(hdr / x-robots-tag); [ -z "$v" ] && ok "/ no x-robots-tag" || bad "/ carries x-robots-tag on production: $v"
  expect_body /robots.txt '^Sitemap: https://herinean.com/sitemap.xml$'
fi
printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"; [ "$FAIL" -eq 0 ]
