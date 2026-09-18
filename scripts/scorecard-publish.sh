#!/usr/bin/env bash
# Renders the scorecard fragment from scorecard.json (+ manual rows) and writes it and the JSON to KV. CI runs this after a green post-deploy check.
set -euo pipefail
cd "$(dirname "$0")/.."
export PATH="$HOME/.local/bin:$PATH"
. scripts/env.sh
mkdir -p .cache
go build -tags nodynamic -o .cache/site ./cmd/site
IN=${1:-scorecard.json}
.cache/site scorecard --in "$IN" --manual data/scorecard-manual.yaml --out .cache/scorecard.html
grep -q '<table data-scorecard>' .cache/scorecard.html
# The metadata is the Worker's ETag suffix: one key carries the fragment and its validator, so the Worker reads KV once.
npx --yes wrangler@4 kv key put --binding SCORECARD --remote scorecard --path .cache/scorecard.html --metadata "{\"etag\":\"$(sha256sum .cache/scorecard.html | cut -c1-8)\"}"
if [ -f "$IN" ]; then
  npx --yes wrangler@4 kv key put --binding SCORECARD --remote scorecard.json --path "$IN"
fi
echo "published: $(wc -c < .cache/scorecard.html) bytes"
