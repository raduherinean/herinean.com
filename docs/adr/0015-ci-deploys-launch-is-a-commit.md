# ADR-0015 — CI is the deployer; the launch is a commit; post-deploy has two tiers
**Status:** accepted 2026-09-19 · **Spec:** §7, §8 step 5, §11, §3 "post" · **Design:** docs/specs/2026-09-19-m2a-ci-bench-scorecard-design.md

## Context
Spec §7 makes CI the gate and the deployer, §11 keeps production on a placeholder until launch criteria are met, and §3 says post-deploy checks "cannot block". Three things had to be decided: what holds the deploy job back before launch and lets it run freely after; what happens when a deploy reaches production broken; and how a single token and a public repository keep the deploy path narrow.

## Decision
1. `site.yaml` carries `launched: YYYY-MM-DD`; the deploy job runs only when it is set. Launch is one commit, visible in the history, with no repository setting to remember. Rejected: a required reviewer on the `production` environment (a click before every publish, against §8's "deployed after the merge") and a repository variable (invisible in the story).
2. Post-deploy has two tiers. Tier 1 verifies the deploy itself — the site's own invariants on production (`scripts/verify-preview.sh --prod`) — and rolls back to the previous Worker version, restoring the previous KV scorecard, when it fails. Tier 2 measures the post rows (Lighthouse on production, Nu and axe on the colophon, transport, Observatory, DNS, caching, privacy) and only reports; §3's "cannot block" is tier 2. A red tier-2 row stays red on the colophon until fixed.
3. Merge-to-live takes a few minutes, not "about a minute" (§8 step 5 amended): the audit must be green before anything deploys; deploying first and rolling back on a red audit would put a red build on production for minutes.
4. Only `actions/*` run in the workflows, pinned by commit; wrangler comes from a lockfile; the pull-request comment goes through the runner's `gh`. One token, scoped to the Worker, its KV namespace and Analytics read — no zone scope — so the post rows that need the zone API (settings) stay with the infra job run locally.

## Consequences
- The colophon's rows always name the build they describe (`Audited build`); a manual deploy labels itself unaudited.
- A rollback Cloudflare refuses (bindings changed; more than ten versions back) fails the run loudly; the RUNBOOK's manual re-deploy is the remedy. The placeholder is never the automatic fallback.
- Post rows are absent for the minutes between a deploy and its `post`; carrying the previous build's numbers would be a claim about a build that no longer serves.
