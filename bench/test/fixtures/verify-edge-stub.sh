#!/usr/bin/env bash
# Stands in for scripts/verify-edge.sh --ci in tests: one failure, matching its output shape.
printf '\n== fake section\n'
printf '  ok   fake.example.com A -> 203.0.113.1\n'
printf '  FAIL fake.example.com CAA expected /issue "letsencrypt.org"/ got '"'"'<absent>'"'"'\n'
printf '\n3 passed, 1 failed\n'
exit 1
