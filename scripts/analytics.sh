#!/usr/bin/env bash
# Views from Analytics Engine: per piece (default), by referral tag or by country. Usage: analytics.sh [--days 30] [--by path|ref|country|referrer]
set -euo pipefail
cd "$(dirname "$0")/.."
. scripts/env.sh
DAYS=30; BY=path
while [ $# -gt 0 ]; do case "$1" in --days) DAYS=$2; shift 2;; --by) BY=$2; shift 2;; *) echo "usage: $0 [--days N] [--by path|ref|country|referrer]" >&2; exit 2;; esac; done
case "$BY" in path) COL=blob1;; ref) COL=blob4;; country) COL=blob5;; referrer) COL=blob3;; *) echo "unknown --by $BY" >&2; exit 2;; esac
# SUM(_sample_interval) estimates the true count under sampling; each row is one view.
SQL="SELECT $COL AS $BY, SUM(_sample_interval) AS views FROM herinean_views WHERE timestamp > NOW() - INTERVAL '$DAYS' DAY GROUP BY $BY ORDER BY views DESC FORMAT TabSeparatedWithNames"
curl -sS -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" --data "$SQL" "https://api.cloudflare.com/client/v4/accounts/$CLOUDFLARE_ACCOUNT_ID/analytics_engine/sql"
