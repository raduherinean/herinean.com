# ADR-0012 — Zone configuration as code (OpenTofu); Worker via wrangler
**Status:** accepted 2026-09-17 · **Spec:** §6.4
## Context
Four zones, TLS settings, mail records, redirects and injector switches configured by hand are unreviewable and drift silently. On the first evening a dashboard edit and a tofu import raced each other within the same minute; the audit log told the story, tofu's plan told the fix.
## Decision
`infra/` (OpenTofu, `cloudflare ~> 5.0`) owns zone settings, DNS, mail, DNSSEC and redirect rulesets, applied from the operator's machine with a scoped token; local state, backed up. wrangler owns the Worker, assets and custom domains. `scripts/verify-edge.sh` checks observable behaviour, not state.
## Consequences
The entire edge is readable in the public repo. Zone ids, account id, DMARC report addresses and the DKIM public key are committed (not secrets). Drift shows up in `tofu plan` — demonstrated on day one when a temporary manual TLS change was reverted by the next apply — and in the weekly verification.
