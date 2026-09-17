# ADR-0011 — Minimum TLS version

**Status:** accepted 2026-09-17 · **Spec:** §6.1, §3 rows 10/11

## Context
The scorecard asks for SSL Labs A+ and internet.nl 100%. On Cloudflare's Free plan those two pull in opposite directions, measured on the placeholder deployment the evening of 2026-09-17:

| Minimum TLS | SSL Labs | internet.nl | Notes |
|---|---|---|---|
| 1.3 | **A** (`isExceptional: false`), TLS 1.3 only, 3 AEAD suites, no warnings, HSTS complete | **100%**, no warnings (result 4305743) | SSL Labs withholds A+ from 1.3-only endpoints; nothing in the report is marked as a defect |
| 1.2 + 1.3 | **A+** on all four endpoints (fresh scan) | **95%** (result 4305727): "Cipher suites — insufficiently secure" (`TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA`), "Cipher suite order — not enforced", "Hash function for key exchange — SHA-1" | Cloudflare's default 1.2 list cannot be restricted without Advanced Certificate Manager ($10/month), and server cipher ordering is not configurable |

Compatibility of TLS 1.3-only in 2026: every browser since 2018–2019 (Chrome 70, Firefox 63, Safari 12.1 / iOS 12.2, Edge 79; Chrome on Android carries its own TLS stack); corporate TLS-inspecting proxies since 2019–2021 (Fortinet 6.2, Palo Alto PAN-OS 10, Zscaler, Symantec ProxySG 7). A network that cannot speak TLS 1.3 is a security finding of its own.

## Decision
**Minimum TLS 1.3.** internet.nl is the EU-recognised standard the site's governance audience knows; SSL Labs' A is a rater quirk explained in one line on the colophon ("A, not A+: SSL Labs does not award A+ to TLS 1.3-only servers; see ADR-0011"). Paying $10/month to add a 1.2 cipher list that would still fail internet.nl's ordering test buys nothing.

Verification links: https://internet.nl/site/herinean.com/4305743/ · https://www.ssllabs.com/ssltest/analyze.html?d=herinean.com · https://developer.mozilla.org/en-US/observatory/analyze?host=herinean.com (A+, 145)

## Consequences
- Readers on clients without TLS 1.3 cannot connect. Revisit if a real reader reports being blocked: `min_tls_version = "1.2"` in `infra/settings.tf` is the whole change, and the scorecard row for internet.nl then reads 95% with the reason.
- The scorecard's SSL Labs row is redefined as "A+ where the rater allows it; A with the 1.3-only explanation otherwise" (spec §3 row 10 amended).
- Re-verified weekly by `verify.yml` (M2); the manual TLS change made during this spike was reverted by the very next `tofu apply`, which is the drift detection working as designed.
