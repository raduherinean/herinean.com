#!/usr/bin/env bash
# Sorted sha256 list of a built dist/: the reproducibility check in CI (built twice, compared) and the one-off arm64/amd64 comparison (RUNBOOK).
set -euo pipefail
d=${1:-dist}
(cd "$d" && find . -type f | LC_ALL=C sort | xargs sha256sum)
