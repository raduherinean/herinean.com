# The bench

The audit bench: one Node entry point that measures a served build against the scorecard rows
of the design spec (spec §7, `docs/specs/2026-09-19-m2a-ci-bench-scorecard-design.md`) and writes
rows in the exact JSON shape `internal/site/scorecard.go`'s `ciRow` reads: `{check, pass, value,
when, link, link_text}`. CI runs it against `site serve --static` on every pull request and push
(mode `ci`), and again against production after a deploy (mode `post`); `scripts/bench.sh` runs
the same modules locally (see "Running locally") — the colophon only ever shows CI's numbers.

## Row contract

Every row (`bench/lib/row.mjs`):

```js
{ check, pass, value, when, link, link_text }
```

`row(check, pass, value, ctx, detail?)` fills `when` (today's UTC date), `link`/`link_text` from
`ctx`, and stashes an optional `detail` (arbitrary debug payload) which `audit.mjs` strips into a
separate `*.detail.json` file before writing the row — `detail` never reaches `scorecard.json`.

## Module interface

One file per scorecard row under `bench/lib/`:

```js
export const name = "check-name";        // the row's `check` string
export const modes = ["ci"];             // or ["ci", "post"]
export async function run(ctx) {         // ctx: { base, mode, pages, dist, when, link, linkText, browser }
  return [row(name, pass, value, ctx, detail)];
}
```

`ctx.pages` is the array of `Page` objects (`{ url, path, status, headers, html, $ }`) that
`bench/lib/pages.mjs`'s `loadPages(base)` builds from `/sitemap.xml`; `ctx.browser()` (from
`bench/lib/browser.mjs`) returns a shared Playwright `Browser`, launched on first use.

The modules, one per row (`MODULES` in `audit.mjs` lists the same names):

| Module | Row | What it checks |
|---|---|---|
| `lighthouse.mjs` | Lighthouse | the [`lighthouse`](https://www.npmjs.com/package/lighthouse) API against Playwright's Chromium, launched via `chrome-launcher`; mobile and desktop; 3 runs per page, median per category; every sitemap page (`/404.html` excluded) must score 100 on performance, accessibility, best-practices and seo. Sharded — see "Sharding Lighthouse" below. `modes = ["ci", "post"]` |
| `i18n.mjs` | i18n | `<html lang>` matches the path; exactly one absolute canonical with a trailing slash, and it is the page itself (a well-formed canonical naming another page fails); hreflang alternates are symmetric (the other page links back to me under my own language) plus `x-default` |
| `social.mjs` | Social previews | every page has `og:*` + `twitter:card=summary_large_image`; `og:image` is a 1200×630 PNG under 200 KB at a content-hashed `/og/` URL on the site's own origin (the page's canonical origin — pages carry production URLs even when CI serves them from `127.0.0.1`) |
| `wellknown.mjs` | Well-known files | `security.txt` (RFC 9116 fields, `Expires` valid and ≤ 1 year out), `robots.txt` with a `Sitemap:` line, `sitemap.xml` entries carry `lastmod` and `x-default`, `llms.txt`, favicons, and a real 404 status on an unknown path. The row goes red within 30 days of `security.txt`'s `Expires` (`expires in N days (renew)`); a commit that refreshes the date clears it |
| `feeds.mjs` | Feeds | RSS 2.0 (`/feed.xml`, `.en`, `.ro`) strict-parses, required channel elements, `atom:link rel=self`, items have full text and only absolute URLs; JSON Feed 1.1 required fields |
| `jsonld.mjs` | Structured data | every `ld+json` block parses; presence is asserted by page type — the home pages (`/`, `/ro/`) must carry `WebSite` and `Person`, a piece (`/writing/<slug>/`, `/ro/articole/<slug>/`) `BlogPosting` and `BreadcrumbList`; each known `@type` carries its required fields; `Person.sameAs` has LinkedIn, X and GitHub |
| `headers.mjs` | Security headers | served headers per path class (`/`, a preloaded font, `og:image`, `/img/*`, feeds, `sitemap.xml`, `robots.txt`, `llms.txt`, `security.txt`) match spec §6.2; the CSP's style-src hash equals the sha256 of each page's own inline `<style>`; HSTS and no `Set-Cookie` everywhere. `modes = ["ci", "post"]` |
| `privacy.mjs` | Privacy | a real Chromium load of every page: no `Set-Cookie`, no third-party request, no `/cdn-cgi/` in the HTML, no `<script>` besides `application/ld+json`. `modes = ["ci", "post"]` |
| `weight.mjs` | Weight | HTML+CSS ≤ 30 KB brotli; fonts ≤ 100 KB total with exactly one preloaded; first view (Chromium, mobile viewport, a fresh browser context per page so every page is a cold load, not a cache hit on the previous page's fonts) ≤ 6 requests and ≤ 150 KB brotli-equivalent; 0 bytes of executable JS |
| `links.mjs` | Links | every same-origin `href`/`src`/`srcset` across every sitemap page resolves with 200, or the row fails; `/404.html` is not a link source — its self-canonical answers 404 by design, and its other links are the masthead and footer every other page already carries (spec §3 row 1 gives it to Nu and axe only); external links get a 10 s timeout and only warn, checked against `bench/links-allow.txt` |
| `html.mjs` | HTML validity | the [Nu Html Checker](https://validator.github.io/validator/) (`vnu-jar`) reports 0 errors and 0 warnings; `html-validate` (`bench/.htmlvalidate.json`: `recommended` + `a11y` + `document`) reports 0 errors — every page, `/404.html` included. `modes = ["ci", "post"]` |
| `a11y.mjs` | Accessibility | [`@axe-core/playwright`](https://github.com/dequelabs/axe-core-npm) against WCAG 2.2 AA in a real Chromium, in both `light` and `dark` `prefers-color-scheme`; AAA contrast (`color-contrast-enhanced`) everywhere except the `--ink-2` secondary-text selectors (`bench/lib/a11y.mjs`'s `SECONDARY`, kept in step with `assets/css/site.css`); skip link is the first tab stop; every interactive element gets a visible `:focus-visible` outline; tab stops account for every interactive element; the language-switch link carries `lang`. `modes = ["ci", "post"]` |
| `fonts.mjs` | Font correctness | Playwright, two viewports (390/1280), three contexts per page: **fast** (no interception) is the real web-font render — proved by a width probe (a hidden span set in `"Herinean Serif", monospace` vs. plain `monospace`; a >5 px difference means the web face painted), not `document.fonts.check()`, which reports load state, not paint, and reads `true` even when `font-display: optional`'s ~100 ms block period has already elapsed and the page committed to the fallback; **delayed** (`/fonts/**` +600 ms) carries the CLS claim — a layout-shift observer must read 0, which is what `optional` actually guarantees even on a late-arriving font; **blocked** (`/fonts/**` aborted) is the metric-matched fallback render, compared against `fast`'s `document.documentElement.scrollHeight` (≤ 5 %) and each `main p`'s line count (±1). Glyph coverage (U+0218–021B, ă â î) is enforced separately by `site check` on the shipped subset faces. Needs a serif that matches the metrics Liberation Serif/Times New Roman was computed against — `.github/workflows/audit.yml`'s `checks` job installs `fonts-liberation` explicitly before the audit runs, since Playwright's `--with-deps` may or may not pull it; a local run should do the same, or the fallback measurement is meaningless. `modes = ["ci"]` |
| `transport.mjs` | Transport (production) | TLS 1.2 refused (`openssl s_client … -tls1_2` must fail with a protocol-version/handshake alert) and TLS 1.3 negotiated; HTTP/3 actually negotiated — via an h3-capable curl (`--http3-only -w '%{http_version}'` must read `3`; `Alt-Svc` alone is not proof a client can complete an h3 handshake); an AAAA record exists (`dig … @1.1.1.1`). 0-RTT is reported as configured off (`infra/settings.tf`'s `"0rtt" = "off"`), not measured — a runner cannot provoke early data on demand. `modes = ["post"]` |
| `observatory.mjs` | Mozilla HTTP Observatory (production) | runs the package's own CLI (`node node_modules/@mdn/mdn-http-observatory/bin/wrapper.js <host>`, i.e. `mdn-http-observatory-scan <host>`) and parses its JSON; pass iff grade `A+`. The row's `link` points at the Observatory's own report (`…/observatory/analyze?host=<host>`) instead of the CI run — the more useful "verify it yourself" here. `modes = ["post"]` |
| `dns.mjs` | DNS, mail, domains (production) | wraps `scripts/verify-edge.sh --ci` (the M0 edge suite: DNSSEC, CAA, MX/SPF/DKIM/DMARC/MTA-STS/TLS-RPT ×4 zones, redirects, apex headers) rather than re-implementing any of it — one script stays the one truth for the edge. Domain-expiry checks via RDAP are the weekly job's (M2b), not this row's. `modes = ["post"]` |
| `caching.mjs` | Caching (production) | `/` and `/colophon/` round-trip `If-None-Match` to a `304`; the preloaded font and `og:image` (found from `/`'s own HTML) are `max-age=31536000, immutable`; `/feed.xml` is `max-age=300`; `/.well-known/security.txt` is `text/plain`. `modes = ["post"]` |

## Post-only rows

`transport`, `observatory`, `dns` and `caching` are `modes = ["post"]` only: they measure things
(TLS negotiation, DNS, an external grader, cache headers on the *served* build) that only exist
once a real deploy is live, so CI never runs them against `site serve --static`. They run after a
deploy, against production — the one environment they can say anything true about.

None of the four depend on `ctx.pages`: in `post` mode against a build that has no
`/sitemap.xml` yet (M0's placeholder, today), `bench/lib/pages.mjs`'s `loadPages` throws before
`audit.mjs` reaches any module, sitemap or no. That's a real defect for the rows that *do* need
`ctx.pages` (`html`, `a11y`, …) — a missing sitemap is itself something worth failing loudly on —
but these four fetch what they need directly instead of waiting on the sitemap, so they can still
run standalone against a host that has none.

`transport.mjs` needs an HTTP/3-capable `curl`; most system curls (including this repo's own dev
containers) don't have one, since it needs a build against ngtcp2/nghttp3. `curlH3()`
(`bench/lib/tools.mjs`) returns the system `curl` when its own `--version` output already lists
`HTTP3`, otherwise downloads, sha256-verifies and extracts the pinned static build named in
`bench/tools.json` (currently `stunnel/static-curl` [8.22.0](https://github.com/stunnel/static-curl/releases/tag/8.22.0)) into `bench/.cache/bin/`
(gitignored) — verified once, by hand, to report `HTTP3` in its own `--version` before being
pinned. A checksum mismatch throws rather than running an unverified binary.

## Sharding Lighthouse

A full Lighthouse pass (every sitemap page × mobile + desktop × 3 runs) is the slowest thing in
the bench, so `audit.mjs --only lighthouse` splits the work across several invocations instead of
running it in one:

- `--form-factor mobile|desktop` picks one form factor per invocation (omit it to run both in a
  single call, e.g. for `scripts/bench.sh`'s unsharded local run).
- `--shard i/n` slices the sitemap's page list (`/404.html` always excluded from this row): page
  `k` (0-indexed, in sitemap order) runs in shard `i` when `k % n === i - 1`. CI fans this out
  across `n` parallel jobs per form factor.

Each invocation emits one row per form factor named `Lighthouse [<mobile|desktop> i/n]` (or
`Lighthouse (production) [...]` in `post` mode), carrying two extra fields beyond the row
contract: `pages` (how many pages this shard covered) and `worst` (the lowest per-category median
score seen in this shard, out of 100). `merge.mjs` recognizes that naming pattern, folds every
shard of every form factor back into a single `Lighthouse` row — failing (and naming the missing
shard) if any expected `i/n` combination never reported in, or if any shard itself failed — and
reports the pooled page count and the overall worst score. See `bench/test/merge.test.mjs` for the
exact fold behavior.

## Disabled rules

- `html-validate` `doctype-style` (`bench/.htmlvalidate.json`): the site's `<!doctype html>` is
  valid HTML5 (the doctype is case-insensitive) and every template agrees on lowercase; the rule
  wants uppercase for style, not correctness, so it's off rather than the templates changed.

## Running locally

`scripts/bench.sh [BASE] [--post] [--only lighthouse|checks]` is the local escape hatch — the
colophon only ever shows CI's numbers, so this is for reproducing and debugging a red row on your
own machine.

- No `BASE`: builds the site (`go build -tags nodynamic -o .cache/site ./cmd/site`, then `build`
  and `check --dist`), serves it on `http://127.0.0.1:8089` with `.cache/site serve --static`, and
  audits that. Give a `BASE` (e.g. a preview URL) to audit an already-running site instead.
- No flag: mode `ci` (the checks modules, `modes = ["ci", "post"]` and `["ci"]`, then Lighthouse —
  both form factors, unsharded, 3-run median per page, the slow part). `--post` runs mode `post`
  instead, which additionally runs the production-only rows (`transport`, `observatory`, `dns`,
  `caching`) — point `BASE` at a real deploy for those to mean anything.
- `--only lighthouse` or `--only checks` runs just that half, passed straight through to
  `bench/audit.mjs`.
- Output: `.cache/bench/rows.json` (the row array) and `.cache/bench/rows.json.detail.json` (the
  per-check debug payload), folded by `bench/merge.mjs --no-gate` into `.cache/bench/scorecard.json`
  — `--no-gate` so a red row here doesn't fail your shell.
- Needs a JRE ≥ 11 on `PATH` (the script prepends `$HOME/.local/bin`) for the Nu Html Checker half
  of the `html` row; without one, that row prints "Nu did not run" rather than failing the script
  — install with `sudo apt install -y default-jre-headless`.

## CLI

```
node bench/audit.mjs BASE --mode ci|post [--only lighthouse|checks] [--form-factor mobile|desktop] \
  [--shard i/n] [--dist DIR] [--link URL] [--link-text TEXT] --out FILE
```

Fetches `BASE`'s sitemap, runs every wanted module, writes `FILE` (the row array) and
`FILE.detail.json`. Always exits 0, even when rows fail — `merge.mjs` is the gate, not the audit.

```
node bench/merge.mjs --out scorecard.json [--build SHA --build-url URL] [--summary FILE] [--no-gate] [--expect ci|post] rows.json…
```

Folds any number of row files into one `scorecard.json`: orders rows per the spec (`ORDER` in
`merge.mjs`), folds Lighthouse shard rows (`Lighthouse [mobile 1/2]`, …) into a single row, and
prepends an `Audited build` row when `--build` is given. Writes the Markdown table to `--summary`
(appending, for a CI job summary) and exits 1 when any row is red, unless `--no-gate`.

`--expect ci|post` names the row set the inputs must cover (`EXPECT` in `merge.mjs`, derived from
`ORDER`: the thirteen CI rows from `Lighthouse` through `Font correctness`, or the nine
`(production)` rows). Every expected row no file delivered — a `checks` job that died before
writing, a Lighthouse shard that never reported — is added as a red `not measured (no rows file)`
row, so a missing measurement is visible on the scorecard and counted by the gate rather than
silently absent. CI's `merge` job passes it; the `comment` and `publish` jobs, which re-fold
already-merged files, do not, and neither does `scripts/bench.sh --only …`, where the skipped rows
are the user's choice.

`audit.mjs` itself refuses to measure a build the sitemap misdescribes: a sitemap page that does not
answer 200, or a `/404.html` that does not answer 404, aborts the run before any module loads
(the error names each `path → status`), the job fails, and `--expect` marks every row of that mode
`not measured`.

## Testing

`node --test test/*.test.mjs` (via `npm test`). `bench/test/helpers.mjs`'s `serveFixture(dir)`
serves a built-site directory the way `site serve --static` would (`_headers` applied,
`index.html` for directories, `404.html` on a miss) without depending on the Go binary.
`bench/fixtures/site-ok/` is a minimal built site (see `bench/fixtures/README.md`) used by every
module test.

## Pins

`bench/package.json` pins every dependency to an exact version (no `^`/`~`); `bench/package-lock.json`
is committed. `bench/node_modules/` is gitignored.
