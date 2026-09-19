# ADR-0002 — Cloudflare Workers static assets, no origin server
**Status:** accepted 2026-09-17 · **Spec:** §6
## Context
A static site of a few dozen pages for a global audience, judged on TTFB and on what it costs to run.
## Decision
Serve `dist/` from Cloudflare Workers static assets with a ~80-line edge function; no origin, no Coolify.
## Consequences
Nothing to patch or back up; global edge; `_headers`; one vendor (see RUNBOOK "Cloudflare-wide outage"). Coolify's only possible future role is a newsletter host.
