#!/usr/bin/env bash
# One-time developer setup: hooks path and tool check.
set -euo pipefail
cd "$(dirname "$0")/.."
git config core.hooksPath .githooks
chmod +x .githooks/*
export PATH="$HOME/.local/bin:$PATH"
for t in go tofu jq dig curl node fonttools pyftsubset; do command -v "$t" >/dev/null || echo "missing: $t (see RUNBOOK.md)"; done
[ -f /usr/share/fonts/truetype/liberation/LiberationSerif-Regular.ttf ] || echo "missing: fonts-liberation (sudo apt install -y fonts-liberation)"
# node comes from nvm on the build machine; the pre-commit hook only sees ~/.local/bin, so link it there.
for t in node npx; do command -v "$t" >/dev/null && [ ! -e "$HOME/.local/bin/$t" ] && ln -sfn "$(command -v "$t")" "$HOME/.local/bin/$t"; done
go version
echo "hooks installed; build with 'go build -tags nodynamic -o site ./cmd/site', then './site serve' to preview"
