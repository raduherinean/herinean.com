#!/usr/bin/env bash
# Copies tofu state out of the repo. State holds zone ids and public DNS data only, but losing it costs an hour of imports.
set -euo pipefail
cd "$(dirname "$0")/../infra"
dest="${HOME}/.config/herinean/tfstate"; mkdir -p "$dest"; chmod 700 "$dest"
[ -f terraform.tfstate ] || { echo "no state yet"; exit 0; }
cp terraform.tfstate "$dest/$(date -u +%Y%m%dT%H%M%SZ).json"
ls -1 "$dest" | tail -3
