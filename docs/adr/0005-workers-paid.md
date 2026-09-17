# ADR-0005 — Workers Paid instead of the free tier
**Status:** accepted 2026-09-17 · **Spec:** §6.1, §6.5
## Context
The free tier hard-fails at 100k Worker requests/day; one good LinkedIn day plus bots would serve errors.
## Decision
Workers Paid ($5/month) from day one; billing alert set.
## Consequences
The site cannot fail on quota. Cost line: $5/month beyond domains and Workspace.
