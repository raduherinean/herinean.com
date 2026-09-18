#!/usr/bin/env bash
# One-time developer setup: hooks path and tool check.
set -euo pipefail
cd "$(dirname "$0")/.."
git config core.hooksPath .githooks
chmod +x .githooks/*
export PATH="$HOME/.local/bin:$PATH"
for t in go tofu jq dig curl; do command -v "$t" >/dev/null || echo "missing: $t (see RUNBOOK.md)"; done
go version
echo "hooks installed; run 'go run ./cmd/site serve' to preview"
