# ADR-0004 — Analytics without a tracker

**Status:** accepted 2026-09-18 · **Spec:** §6.3, §5.4, §3 row 15

## Context
The site promises zero client-side JavaScript, zero cookies and zero third-party origins, and its readers are the people who ask vendors these questions. It still needs to know which pieces are read and from where, to decide what to translate and what to write next.

## Decision
One Analytics Engine datapoint per HTML response on the production host, written by the edge function inside `waitUntil`: `path, lang, referrer host, ref (li|x|nl|md|other|none), country, 1`. Nothing else is stored — no IP, no user agent, no full referrer, no identifier of any kind. Known bots are skipped by user agent; some still get counted. Preview hosts write nothing. The statement appears verbatim on the privacy page and the colophon, and `worker/index.test.mjs` pins the exact shape of the datapoint.

## Consequences
Numbers compare pieces; they do not measure reach or unique readers, and the colophon says so. Analytics Engine retains ~90 days; a weekly job snapshots per-piece totals to KV (M2). Referral attribution depends on the `?ref=` tag in links Radu posts; organic referrers are attributed by host only.
