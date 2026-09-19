#!/usr/bin/env bash
# Renders the scorecard fragment from scorecard.json (+ manual rows) and writes it and the JSON to KV. CI runs this after a green post-deploy check.
set -euo pipefail
cd "$(dirname "$0")/.."
[ -d node_modules/wrangler ] || npm ci --silent
export PATH="$HOME/.local/bin:$PATH"
. scripts/env.sh
mkdir -p .cache
# CI hands the job the built binary; locally we build it.
if [ -x ./site ]; then SITE=./site; else go build -tags nodynamic -o .cache/site ./cmd/site; SITE=.cache/site; fi
IN=${1:-scorecard.json}
"$SITE" scorecard --in "$IN" --manual data/scorecard-manual.yaml --out .cache/scorecard.html
grep -q '<table data-scorecard>' .cache/scorecard.html
if command -v java >/dev/null && [ -f bench/node_modules/vnu-jar/build/dist/vnu.jar ]; then
  { printf '<!doctype html><html lang="en"><head><title>scorecard</title></head><body>'; cat .cache/scorecard.html; printf '</body></html>'; } > .cache/scorecard-doc.html
  java -jar bench/node_modules/vnu-jar/build/dist/vnu.jar --errors-only --exit-zero-always .cache/scorecard-doc.html 2>&1 | tee .cache/scorecard-nu.txt
  ! grep -q 'error:' .cache/scorecard-nu.txt || { echo "scorecard fragment is not valid HTML" >&2; exit 1; }
else
  echo "scorecard-publish: Nu not available here; the fragment is validated in CI" >&2
fi
# The metadata is the Worker's ETag suffix: one key carries the fragment and its validator, so the Worker reads KV once.
scripts/kv.sh put scorecard .cache/scorecard.html --metadata "{\"etag\":\"$(sha256sum .cache/scorecard.html | cut -c1-8)\"}"
if [ -f "$IN" ]; then scripts/kv.sh put scorecard.json "$IN"; fi
echo "published: $(wc -c < .cache/scorecard.html) bytes"
