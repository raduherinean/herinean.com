# herinean.com — Design Specification

| | |
|---|---|
| Date | 2026-09-17 |
| Status | Approved after section-by-section review with adversarial passes |
| Author | Radu Herinean, with Claude (Opus 5) |
| Next | Implementation plan (`docs/plans/`), then M0 → M1 → M2 → launch |

---

## 1. Purpose and north star

herinean.com is the canonical archive for Radu Herinean's writing: a practicing CTO who ships AI in real businesses and writes about which projects are worth funding. Readers are executives, boards and founders — not developers. Pieces are in the language of their reader: governance, board and CEE-market pieces in Romanian; build and analysis pieces in English.

**North star.** When anyone examines the site, its hosting or its stack from a technical perspective, it scores the maximum on every measurable axis, for a global audience, continuously — and the platform is itself the subject of one of the first pieces: how AI was used to build the perfect platform for a specific purpose.

Two consequences drive every decision below:

1. **"100/100" is a scorecard, enforced in CI on every commit and re-verified weekly**, displayed on a public colophon with links to reproduce every result. Undefined, the claim is unfalsifiable; unenforced, it decays by the third post.
2. **The perfect platform for this purpose is the least platform that scores perfect.** Boring for the executive reader, immaculate for the peer who opens view-source. Nothing is added that a validator or a reader would not miss.

## 2. Readers, strategy, and what the site is not

**Readers.** (a) The executive, on a phone, arriving from LinkedIn's in-app browser; reads 1,500–3,000 words if the page gets out of the way. (b) The peer CTO or technical advisor doing due diligence on the author; opens view-source and the colophon.

**Channels.** LinkedIn is where the author writes short and engages; herinean.com is the canonical archive; X is repurposing only; Medium is optional via import with canonical set to herinean.com. Every piece ends with a link to its LinkedIn discussion. Long-form never originates on LinkedIn Articles or Medium.

**Content tiers.** Own projects: no approval needed. Generalized lessons from work: no names, no numbers. Named client case studies: after sign-off, drafted privately (see §8).

**Pillars.** Build logs · Opportunity spotting · AI + governance · Analysis. Editorial shape: situation → trade-off → decision → what I'd change (a template, not a build rule).

**Not** a developer blog, a portfolio, a company site, or a newsletter (until three pieces exist). No "Evolution Labs" branding; Evolution Labs is linked from the About text as one of the things the author does.

## 3. Definition of done — the scorecard

Every row is pass/fail. **CI** blocks deploy. **post** runs against production right after deploy and weekly, and is reported (cannot block). **ext** is an external tool run manually, linked from the colophon with a "verify it yourself" URL and a date; marked *stale* on the colophon after 90 days.

| # | Check | Threshold | Enforced |
|---|---|---|---|
| 1 | Lighthouse, every page, mobile + desktop (3 runs, median; `/404.html` excluded) | 100 / 100 / 100 / 100 | CI (local server) + post (production). Never against a preview host (noindex fails SEO). |
| 2 | HTML validity | Nu checker 0 errors 0 warnings; `html-validate` strict | CI; post on `/colophon/` (edge-filled) |
| 3 | Accessibility | axe-core 0 violations (WCAG 2.2 AA), run in **both** colour schemes; body-text contrast ≥ 7:1 (AAA), secondary ≥ 4.5:1; skip link; visible focus; keyboard-complete; `lang` on every foreign-language fragment | CI; post on `/colophon/` |
| 4 | Links | 0 broken internal; external checked, failures warn (allowlist for known-flaky) | CI + weekly |
| 5 | Feeds | RSS 2.0 (all, en, ro) + JSON Feed (all), **full text**, absolute URLs, `atom:link rel=self`, validate clean | CI |
| 6 | Structured data | JSON-LD `WebSite`, `Person` (sameAs LinkedIn/X/GitHub), `BlogPosting` (`inLanguage`, `image`, `author`, `datePublished`, `dateModified`), `BreadcrumbList`; required fields validated | CI; Google Rich Results ext |
| 7 | Social previews | `og:*` + `twitter:card=summary_large_image` on every page; per-page 1200×630 image < 200 KB at a content-hashed URL | CI; LinkedIn Post Inspector + X validator ext (once per template) |
| 8 | i18n | `<html lang>` correct; hreflang pairs symmetric + `x-default`; absolute canonical; one canonical URL per page (trailing slash) | CI |
| 9 | Security headers | securityheaders.com A+; CSP `default-src 'none'` with hashed inline style; `base-uri 'none'`; `form-action 'none'` (one origin when the newsletter lands); `frame-ancestors 'none'`; HSTS 2 y + includeSubDomains + preload (submitted); COOP; CORP (`same-origin` HTML, `cross-origin` on `/img/*` `/og/*`); Referrer-Policy; Permissions-Policy; nosniff; X-Frame-Options | CI on local preview; post |
| 10 | Transport | TLS 1.3 minimum (decided: ADR-0011); SSL Labs **A** with the 1.3-only explanation on the colophon (the rater withholds A+ from 1.3-only endpoints; A+ would cost internet.nl 100%); HTTP/3; IPv6; 0-RTT off | post (`curl --http3`, TLS version); SSL Labs ext |
| 11 | internet.nl website test | 100% | ext (the mail test is out of scope: Google Workspace has no DANE; the colophon says so) |
| 12 | Mozilla HTTP Observatory | A+ | post (CLI) |
| 13 | DNS, mail, domains | DNSSEC valid; CAA present; `.com`: Workspace MX, SPF `include:_spf.google.com -all`, DKIM 2048, DMARC `p=reject; adkim=s; aspf=s`, MTA-STS, TLS-RPT; `.ro` `.net` `.info` (Workspace alias domains of the same mailbox): MX kept, SPF `include:_spf.google.com -all`, DMARC `p=reject; adkim=s; aspf=s`; 301 to `.com` with path and query; `www` → apex; all four domains auto-renew, locked, expiry > 60 days (RDAP) | post + weekly |
| 14 | Weight | Content page: HTML+CSS ≤ 30 KB compressed; fonts ≤ 100 KB total, one preloaded; total first-view transfer ≤ 150 KB; ≤ 6 requests; **0 bytes of executable JavaScript** (JSON-LD data blocks are not JavaScript) | CI |
| 15 | Privacy | 0 cookies; 0 third-party origins; no consent UI — by construction; no Cloudflare-injected scripts (`/cdn-cgi/`) in production HTML | CI (no `Set-Cookie`; CSP) + post (no executable `<script>` in served HTML) |
| 16 | Well-known files | `/.well-known/security.txt` (RFC 9116, `Expires` = commit date + 364 d, weekly warning at 30 d); `robots.txt` (+ sitemap ref); `sitemap.xml` with hreflang alternates + lastmod; `llms.txt`; favicons (SVG + ICO + apple-touch); custom 404 with real 404 status | CI |
| 17 | Caching | `/img/*` `/og/*` content-hashed, `max-age=31536000, immutable`; HTML `max-age=0, must-revalidate`, edge-cached with ETag; feeds `max-age=300`; correct `Content-Type` on feeds and `security.txt` | post |
| 18 | Sustainability | websitecarbon A+ | ext (API-automatable in the weekly job) |
| 19 | Global latency and uptime | TTFB p50 < 100 ms from 5 regions (globalping in the weekly job); external uptime monitor with alerting | weekly + ext |
| 20 | Repo hygiene | README, LICENSE (code MIT; content CC BY-NC-ND 4.0), ADRs, this spec, RUNBOOK, CI badge, dependency count on colophon (the modules linked into the generator, from the binary's build info), reproducible build (`-trimpath`, pinned toolchain, `SOURCE_DATE_EPOCH` = commit time), Dependabot (modules, actions, bench), CodeQL, OpenSSF Scorecard badge (shown honestly; 10/10 is structurally unreachable) | ext (Scorecard/CodeQL run on their own) |
| 21 | Font correctness | Vendored fonts contain U+0218–021B (ș ț Ș Ț comma-below) and ă â î; no layout shift from type (Playwright: layout-shift observer reports CLS 0 with the font delayed 600 ms); the metric-matched fallback holds the measure — page height within 5 % and every paragraph within ±1 line of the web-font rendering | CI |

## 4. Content model, URLs, pages

### 4.1 Languages and URLs

English at the root, Romanian under `/ro/`, `x-default` → English. Slugs are per-language, ASCII-only (`ă→a ș→s ț→t â→a î→i`, enforced). Every URL ends with a slash; the edge 301s the other form. One canonical per page.

| Page | EN | RO |
|---|---|---|
| Home = About + latest five (any language, badge when not the page language) | `/` | `/ro/` |
| Writing index (all pieces, newest first, grouped by year; pillar label + language badge; one-line summary) | `/writing/` | `/ro/articole/` |
| Piece | `/writing/<slug>/` | `/ro/articole/<slug>/` |
| Colophon (scorecard, stack, dependencies, AI disclosure, known trade-offs, repo) | `/colophon/` | — (EN only) |
| Privacy notice (what is and isn't processed; Cloudflare as CDN; analytics statement) | `/privacy/` | `/ro/confidentialitate/` |
| Feeds (full text) | `/feed.xml` (all languages, for newsletter/Medium), `/feed.en.xml`, `/feed.ro.xml`, `/feed.json` (all) | EN pages advertise `feed.en.xml`; RO pages `feed.ro.xml` via `<link rel="alternate">` |
| Machine files | `/sitemap.xml`, `/robots.txt`, `/llms.txt`, `/.well-known/security.txt`, `/.well-known/mta-sts.txt` (served on `mta-sts.herinean.com` only), `/404.html`, favicons | |
| Generated images | `/og/<lang>-<slug>.<hash>.png`, `/img/<key>/<name>.<hash>.<w>.webp` (+ original format fallback) | |

Deliberately absent: a separate About page (home is the About), pillar pages (return at ~12 pieces), tags, search, comments.

### 4.2 A piece

One Markdown file: `content/en/<slug>.md` or `content/ro/<slug>.md`. Front matter:

```yaml
title:     "Which AI projects are worth funding in 2027"
date:      2026-09-27          # set by /publish-piece; Europe/Bucharest; must be ≤ commit date
updated:   2026-10-03          # optional; shown, sets sitemap lastmod and JSON Feed date_modified; feed pubDate unchanged
key:       ai-funding-2027     # pairs EN and RO; the language switch appears iff both exist
pillar:    analysis            # build-log | opportunity | governance | analysis
summary:   "≤ 160 chars. Meta description, OG description, index blurb, feed summary."
linkedin:  https://…           # optional; added by /link-piece; renders "Also on LinkedIn"
medium:    https://…           # optional; renders "· Medium"; Medium import sets canonical here
```

There is no `draft` field: drafts are unpublished branches on the private remote (§8). Body: CommonMark + tables + footnotes + fenced code (highlighted at build with CSS classes), typographer per language („…” for `ro`, "…" for `en`). Comma-below diacritics are mandatory in Romanian content; cedilla forms fail the build.

Images live in `assets/img/<key>/`, referenced relatively. Alt text is mandatory. Diagrams are inline SVG using `currentColor`. Charts are build-time static SVG. No third-party embeds of any kind (CSP forbids them): video is a self-hosted thumbnail linking out.

Site UI strings live in `i18n/en.yaml` and `i18n/ro.yaml`; a string missing in one language fails the build. Home About copy: `content/en/_home.md`, `content/ro/_home.md`. Editorial template: `content/_template.md`.

### 4.3 Page anatomy

- **Piece:** language switch (only when the pair exists; link carries `hreflang` and `lang`) → title → meta line (date, updated, reading time, pillar) → body → footnotes → "Also on LinkedIn · Medium" (when set; excluded from feeds) → one-line author footer with LinkedIn link. No prev/next, related, tags, comments.
- **Home:** masthead with name + tagline → portrait (home only) → 3–4 About paragraphs → rule → Latest (five entries) → "All writing →". `Person` and `WebSite` JSON-LD here.
- **Index:** entries under year headings. **Entry format everywhere:** date (tabular) · pillar (small caps) · language badge when foreign (plain text, visually-hidden full name, `lang` attribute) · title · one-line summary.
- **Colophon:** five-line summary → scorecard table (status, value, when, verify-yourself link; stacked label/value on narrow screens) → dependency list with versions + bench tool versions → analytics statement → AI disclosure → privacy link → known trade-offs → repo, spec, "how it was built" piece → commit SHA, build date, Go version.
- **404:** one sentence in each language, links to both homes and indexes.

## 5. The generator

One Go module, one binary `site`, five verbs:

```
site build        content/ + assets/ + templates/ + i18n/ → dist/    all-or-nothing, deterministic
site check        every in-process validation; writes nothing       CI + pre-commit
site check --dist post-build invariants on dist/                    CI
site serve        local preview (:8080, --host), applies _headers, rebuilds on request when inputs changed
site new <lang> <slug>   scaffold from content/_template.md; empty date, blank pillar; writes one file, touches nothing else
site scorecard    scorecard.json (+ data/scorecard-manual.yaml) → validated HTML fragment for the colophon (CI only)
```

### 5.1 Packages

| Package | Job | Depends on |
|---|---|---|
| `content` | Walk `content/`, parse front matter + Markdown → typed `Piece`/`Page`; slug, URL, reading time; pair translations by `key`; all content validation | goldmark (+footnotes, tables, typographer per language), chroma (classes, no inline styles), yaml |
| `images` | Resize to two widths, encode WebP + original, record dimensions; render per-piece OG PNG (title wrapped ≤ 3 lines, name, pillar, language) from vendored OFL fonts; content-hash cache in `.cache/` | x/image (draw, font/sfnt), pure-Go WebP encoder |
| `render` | `html/template` + `SiteContext` (strings, nav, alternates, canonical, JSON-LD) → HTML per page type; inlines minified `site.css` | stdlib |
| `feeds` | RSS (all/en/ro) via `encoding/xml`, JSON Feed via `encoding/json`; full content with absolute URLs | stdlib |
| `seo` | sitemap (hreflang alternates, lastmod), robots, llms.txt, security.txt, mta-sts.txt, JSON-LD builders | stdlib |
| `edge` | Generate `_headers` (CSP style hash computed from the exact inlined CSS bytes; per-path CORP and Cache-Control; feed content types) and `_redirects` | stdlib |
| `site` | Orchestrate: load → validate → images → render → feeds/seo/edge → write `dist/` in sorted order, fixed mtimes; `SOURCE_DATE_EPOCH` from the commit | all |

Target: under a dozen modules; the modules linked into the generator (from the binary's build info, so the list is reproducible per build) are printed on the colophon. The build time is the commit timestamp, so the same commit yields byte-identical `dist/` (security.txt `Expires` = commit + 364 d; colophon build date = commit date).

### 5.2 Validation (`check`) — fails with `file:line`

Missing/invalid front matter; empty `date`; `date` after commit date (Europe/Bucharest); unknown pillar; duplicate slug; slug not `[a-z0-9-]`; `key` used twice in one language; `summary` > 160 chars; image without alt; image file missing; i18n string missing in one language; internal link to a non-existent page; hreflang pair asymmetric; cedilla ş/ţ (U+015F/U+0163) in Romanian content; vendored font lacking U+0218–021B or ă â î (via `sfnt`).

`check --dist`: no executable `<script>` (data blocks allowed); no `style=` attributes; every page has canonical, og:*, twitter:card, JSON-LD, `<link rel=alternate>` feed; OG image exists, 1200×630, < 200 KB; weight budget (row 14); `_headers` covers every path class; CSP hash matches inlined CSS; no `Set-Cookie`.

### 5.3 Tests

Unit tests per package (front-matter edge cases, pairing, slug transliteration, reading time, XML/JSON escaping, hreflang symmetry, CSP hash, diacritics rule, date rule). Golden-file tests on `testdata/` (an EN/RO pair, an RO-only piece, a piece with images/code/footnotes/table, home, index, colophon, 404) — rendered HTML compared byte-for-byte. `gofmt`, `go vet`, `staticcheck` in CI. External validators are the integration tests. Cold build < 1 s without images.

### 5.4 The edge function

`worker/index.js`, plain JavaScript, ~100 lines, no npm runtime dependencies (wrangler is a deploy tool). Routes by host:

- `herinean.com`: serve `dist/` via the assets binding. For HTML responses only, in `waitUntil`, write one Analytics Engine datapoint: `path, lang, referrer host, ref (li|x|nl|md|other|none), country, 1`. Known bots skipped by UA. On `/colophon/`, fill `<table data-scorecard>` from the KV `scorecard` key (pre-rendered, pre-validated HTML fragment) via HTMLRewriter; also serve `/colophon/scorecard.json`. Never on the response path: analytics is async; any Worker error falls through to the plain asset.
- `mta-sts.herinean.com`: only `/.well-known/mta-sts.txt`; everything else 404.
- any other host (previews): serve assets, add `X-Robots-Tag: noindex`, serve a disallow-all `robots.txt`, no analytics.

Static assets config: `html_handling = force-trailing-slash`, `not_found_handling = 404-page`; the Worker runs first only for HTML routes — `/img/*`, `/og/*`, feeds and machine files are served by the asset layer directly.

## 6. The edge

### 6.1 Zones (all on Cloudflare; managed as code, §6.4)

| | `herinean.com` | `.ro` `.net` `.info` |
|---|---|---|
| DNS | DNSSEC (DS at registrar); CAA for Cloudflare Universal SSL CAs + `iodef`; IPv6 | same |
| Web | Workers custom domain on the apex; `www` proxied placeholder + redirect rule → apex 301 | proxied placeholders on apex + `www`; redirect rule `*` → `https://herinean.com{path}{query}` 301 |
| TLS | Minimum TLS 1.3 (ADR-0011: internet.nl 100% vs SSL Labs A; enabling 1.2 flips both to A+ / 95%; if the audience is ever blocked, 1.2 and re-verify), HTTP/3, 0-RTT **off**, Always-HTTPS, HSTS 2 y + includeSubDomains + preload, submitted after one week live | same HSTS |
| Mail | Workspace MX; SPF `v=spf1 include:_spf.google.com -all`; DKIM 2048 (`google._domainkey`); DMARC `v=DMARC1; p=reject; adkim=s; aspf=s; rua=mailto:<Cloudflare DMARC Management address>,mailto:dmarc@herinean.com` (reject after one signed message verifies); MTA-STS (`_mta-sts` TXT + policy on `mta-sts.herinean.com`, mode enforce after testing); TLS-RPT `_smtp._tls` | Workspace alias domains of the same mailbox: MX kept; SPF `v=spf1 include:_spf.google.com -all`; DMARC `p=reject; adkim=s; aspf=s; rua=mailto:dmarc@<that domain>` (same inbox via the alias, so no cross-domain report authorisation). Pre-existing verification TXT records stay, unmanaged. |
| Injected-script features | **Disabled:** Email Address Obfuscation, Rocket Loader, Mirage, Auto Minify, Web Analytics auto-injection, Zaraz, Cloudflare Fonts, Bot Fight Mode. Browser Integrity Check may stay (header-based). | same |
| `workers.dev` | Production disabled; per-version preview URLs enabled | — |
| Plan | Free zones; **Workers Paid ($5/month)** so quota can never fail the site; billing alert | Free |

Aliases: `security@` (for `security.txt`) and `dmarc@` (DMARC reports) on the mailbox — Workspace applies them across all four domains.

### 6.2 Headers (`_headers`, generated)

```
/*
  Content-Security-Policy: default-src 'none'; style-src 'sha256-<css>'; img-src 'self'; font-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'
  Strict-Transport-Security: max-age=63072000; includeSubDomains; preload
  X-Content-Type-Options: nosniff
  Referrer-Policy: strict-origin-when-cross-origin
  Permissions-Policy: <compact deny list>
  Cross-Origin-Opener-Policy: same-origin
  Cross-Origin-Resource-Policy: same-origin
  X-Frame-Options: DENY
  Cache-Control: public, max-age=0, must-revalidate
/img/*   Cross-Origin-Resource-Policy: cross-origin ; Cache-Control: public, max-age=31536000, immutable
/og/*    (same as /img/*)
/feed*   Cache-Control: public, max-age=300 ; Content-Type per format
```

Consequences enforced by `check --dist`: no `style=` attributes anywhere; chroma emits classes; SVG uses `currentColor`.

### 6.3 Analytics

One Analytics Engine dataset. Per HTML view: `path, lang, referrer host, ref, country, 1`. Not stored: IP, user agent, full referrer URL, or anything that counts *readers* rather than *reads*. Views include some bots; numbers compare pieces, they don't measure reach. The weekly job snapshots per-piece totals (by `ref`, by country) into a KV key so history survives platform retention. `scripts/analytics.sh` merges live + snapshot: views per piece, by `ref`, by country, 30/90 days. The statement above appears, in substance, on the colophon and the privacy page (each in its own words: the colophon speaks to the peer, the privacy page to the reader, in both languages).

### 6.4 Infrastructure as code

`infra/` holds OpenTofu configuration for the four zones: DNS records, DNSSEC, zone settings (TLS, HTTP/3, 0-RTT, HSTS, Always-HTTPS, every injector off, Bot Fight Mode off), redirect rulesets, mail records. It runs from the operator's machine with a scoped, short-lived token; state is a local file (gitignored) backed up by `scripts/infra-backup.sh` — it contains nothing secret (zone IDs, public DNS records) and is recoverable by `import` in minutes, so a remote backend would add credentials for no gain. Zone IDs, account ID and the DKIM public key are committed in `infra/*.auto.tfvars`: none of them is secret, and the edge is meant to be readable. The Worker, its bindings (assets, Analytics Engine, KV) and custom domains are wrangler's job (`wrangler.toml`), deployed by CI. The weekly job verifies observable behaviour (§7), not state.

Not automatable (dashboard/registrar only): adding zones and changing nameservers at registrars, DS records at registrars, Workers Paid subscription and billing, 2FA, API token creation, Workspace DKIM generation and aliases, domain auto-renew and locks.

### 6.5 Cost

$5/month (Workers Paid) beyond domains and Workspace.

## 7. CI, scorecard pipeline, colophon

Audit tools live in `bench/` (Node/Rust/Python, pinned): Lighthouse CI, axe-core, html-validate, Nu checker, lychee, feed validator + JSON Feed schema, JSON-LD field check, Playwright (font-swap layout test, dark/light axe), Observatory CLI, globalping. The platform has ~10 dependencies; the bench that audits it has hundreds — the right way around.

**`ci.yml` — every PR and push to `main`:**
1. **build** — pinned Go; `gofmt`, `go vet`, `staticcheck`, `go test ./...`, `site check`, `site build`, `site check --dist`; `dist/` artifact.
2. **audit** — `site serve` hosts `dist/` with `_headers`; the bench runs against every sitemap URL (`/404.html` via Nu + axe only). Results → `scorecard.json`.
3. **preview** (PRs) — `wrangler versions upload`; URL commented on the PR.
4. **deploy** (`main`, audit green) — `wrangler deploy`.
5. **post-deploy** (`main`) — against production: headers, HTTP/3, TLS version, redirects (`www` + three zones), `dig` DNSSEC/CAA/SPF/DKIM/DMARC/MX, Observatory, Lighthouse, Nu + axe on `/colophon/`, no executable script in served HTML. Merge → `site scorecard` renders the HTML fragment from the same templates → Nu-validated → written with the JSON to the KV `scorecard` key.

**`verify.yml` — weekly:** step 5 again + external links + `security.txt` expiry + RDAP domain expiry + globalping latency + websitecarbon + analytics snapshot. Writes KV. Red stays red on the colophon until fixed.

**Repo guardrails:** `main` protected (PR + green CI + signed commits); Actions pinned by SHA; Dependabot weekly (modules, actions, bench); CodeQL; OpenSSF Scorecard. Secrets: one CI token scoped to this Worker + KV + Analytics read, and the account ID. CI budget: < 6 minutes.

## 8. Authoring and publishing

Local requirements: Go, git, `gh`, Claude Code. Remotes: `gitea` (private; every `piece/*` branch, as often as you like) and `origin` (GitHub, public; `main` and publish PRs only).

1. `site new en <slug>` → one file (empty `date`, blank `pillar`). Branch `piece/<slug>`, tracking `gitea`.
2. Write on `site serve` (`--host 0.0.0.0` to read on a phone over LAN).
3. `/review-piece` — editorial template, tier check by judgement (no denylist of client names in a public repo), summary length, diacritics, embeds/charts constraints, a **fact-check pass** (an agent with repository access checks each technical claim against the code, the commits and the logs and marks it confirmed, wrong or unverifiable — the colophon describes this process, so it must exist before launch; M2), and a **native LinkedIn post** (complete, not a teaser; link goes in the first comment). `/translate-piece` — either direction, shared `key`.
4. `/publish-piece` — stamps `date` (Europe/Bucharest), runs `check`, pushes to `origin`, opens the PR. CI audits and comments the preview (the only real-edge preview; the cost of private drafts).
5. Squash-merge: `Publish: <title>`. Deployed in about a minute.
6. Distribute: native LinkedIn post with `…?ref=li` in the first comment; X with `?ref=x`; Medium optional. `/link-piece` → `Link: <title> → LinkedIn` PR, auto-merge.
7. `scripts/analytics.sh` decides what gets translated.
8. Corrections set `updated:`; feeds don't re-notify. URLs never change; pieces are never deleted (a retraction is a note at the top).

A pre-commit hook (`.githooks/`, installed by `scripts/setup.sh`) runs `gofmt` + `site check`. The repo `CLAUDE.md` and the four skills are public and written as if clients read them.

## 9. Design constraints

Typography, spacing, colour and hairlines carry the whole design. One column, measure ≤ 38rem, 16px gutters, no sidebar/hero/cards/gradients/shadows/animation/icon library/illustration.

- **Type:** one self-hosted OFL body serif (screen-sturdy regular weight; real italic shipped; comma-below diacritics verified), optionally one display serif for masthead and titles; Latin + Latin Extended-A + U+0218–021B subsets; body regular preloaded; `font-display: optional` with metric-matched fallback (a swap was measured and it shifts — ADR-0010) (`size-adjust`, ascent/descent/line-gap overrides); `font-synthesis: none`; real small caps or letter-spaced uppercase, never synthesized. Body 18–19px on phone, line-height ≈ 1.6, modular scale ≈ 1.25, `text-wrap: balance/pretty`, `hyphens: auto`.
- **Colour:** warm near-black on paper-white; one accent for links and focus only; visited not purple; selection from the accent; dark scheme via `prefers-color-scheme` (warm dark grey, accent re-tuned; portrait gets a hairline border). Tokens on `:root`. Contrast per row 3.
- **Structure:** hairlines only. Masthead: name (home), Writing, language link; tagline on home only. Footer: LinkedIn · RSS · Colophon · Privacy, and "No JavaScript, no cookies, no trackers." Skip link first.
- **Print:** nav/footer hidden, URLs after links, black on white, `@page` margins, portrait hidden.
- **OG images:** paper background, title in the display face, name + `herinean.com`, pillar, language; home variant with portrait. **Favicon:** SVG monogram.

Pixels come from the Claude Design pass (`docs/design/claude-design-brief.md`); its export is reference only; production CSS is hand-written (~200 lines). Fonts and licences are vendored and listed on the colophon.

## 10. Never, not now, policies

**Never** (listed on the colophon as known trade-offs, each with its reason): comments · search · dark-mode toggle · unique-visitor counts · tags/series · related posts · CMS/admin · auto-posting to LinkedIn/X · multi-author · Romanian colophon · webmentions · Cloudflare script-injecting features · third-party embeds of any kind (video, X/LinkedIn posts, Gists, Mermaid, MathJax) · cookie banner and third-party analytics · language auto-redirect by `Accept-Language` · long-form originating on LinkedIn Articles or Medium · PWA/service worker/offline · A/B testing or personalization · paywall/membership · changing a URL or deleting a published piece.

**Not now, with triggers:**

| Feature | Trigger | Change |
|---|---|---|
| Newsletter | Three published pieces | One provider origin in `form-action`; form on home and at piece end; double opt-in; privacy page updated. Provider chosen on EU data handling, no tracking pixels, and whether to run anything (Buttondown = zero-ops; Listmonk on Coolify = EU-data, the only reason Coolify ever touches this). |
| Notes (short posts archive) | First LinkedIn post you want to cite and can't find | Same content type with `kind: note`. |
| Pillar pages | ~12 pieces | Four index pages per language. |
| Client-facing gated previews | First pre-sign-off client review on a real URL | Preview deploy from a private branch behind Cloudflare Access (free). |
| Translation of a piece | It performed | `/translate-piece`. |
| AVIF | A fast pure-Go encoder | Config in `images`. |
| Typeface change or system-stack revert | Design review | CSS + font files + one `check` rule. |

**Policies on the colophon from day one:** an AI-disclosure line stating exactly how AI is used in building and in writing; the analytics statement; the privacy page; the known trade-offs.

## 11. Order of work and launch criteria

- **M0 — one evening.** Cloudflare: 2FA; add four zones (verify imported Workspace records on `.com` before switching nameservers); nameservers at registrars; DS records at registrars; Workers Paid + billing alert; API tokens (infra, short-lived; CI, scoped). Workspace: DKIM 2048, `security@`, `dmarc@` aliases. Registrars: auto-renew + lock. GitHub: public repo under the `raduherinean` organisation (owned by the 2010 `rlucian` account — account age is a credibility signal, and GitHub allows one free personal account), 2FA, SSH signing key; Gitea private repo. Then as code: zone settings, injectors off, DNS/mail records (MX kept on all four; SPF `-all` and DMARC reject everywhere), DNSSEC, CAA, redirects, and a **placeholder Worker on the apex** (one noindex line with the full header set) so TLS/HSTS/internet.nl can be verified on the real domain and preload submitted early. TLS 1.3 spike.
- **M1 — weekend one.** Generator, templates, content model, `check`, `serve`, `_headers`, Worker; home/index/colophon skeleton/privacy/404 with real About copy and portrait; Claude Design export as reference; deploy to a `noindex` preview only.
- **M2 — weekend two plus evenings.** Bench, scorecard pipeline, KV colophon, post-deploy + weekly verify, RUNBOOK, ADRs, four skills, uptime monitor. Manual ext audits once.
- **Launch =** scorecard green **and** real About + portrait **and ≥ 2 pieces** (written during the build weeks). Then production cutover, HSTS preload submission, Search Console + Bing (DNS TXT), URL into LinkedIn contact info and website field. Cutover waits for green, not for a date.
- **Then** pieces; "How this site was built" is written from the ADRs and this spec.

## 12. Decisions register (ADR seeds)

1. Bespoke Go generator over Hugo/Astro — the whole platform fits in one context window; exact control of every audited byte; CI scorecard bounds the risk.
2. Cloudflare Workers static assets over Coolify/origin — no server to patch; global edge; `_headers`; the "don't fund what you don't need" proof.
3. Zero client-side JavaScript; one ~100-line edge function.
4. Analytics without a tracker — Analytics Engine, no cookies/IP/UA, counts reads not readers.
5. Workers Paid over free — quota can never fail the site.
6. KV + HTMLRewriter for the scorecard over bot commits — no lag, no history noise.
7. Hashed OG/image URLs — LinkedIn cache and `immutable` are compatible only with content-addressed URLs.
8. Commit timestamp as build time — reproducible output.
9. Drafts on a private remote; no `draft` field — a public repo cannot hold pre-sign-off drafts.
10. Self-hosted fonts over system stack — control is craft; `font-display: optional` keeps CLS at zero by construction and the metric-matched fallback keeps the measure; CI tests both.
11. TLS 1.3 minimum — pending the internet.nl spike; audience blocked → 1.2 modern ciphers.
12. Infrastructure as code (OpenTofu) for zones — reviewable, reproducible, drift-detectable; wrangler for the Worker.
13. Full-text feeds; per-language advertised; combined for newsletter/Medium.
14. URLs never change; pieces never deleted.

## 13. Repository layout

```
herinean.com/
  cmd/site/                 CLI (build, check, serve, new)
  internal/{content,images,render,feeds,seo,edge,site}/
  templates/                base, home, index, piece, colophon, privacy, 404
  content/{en,ro}/          pieces, _home.md;  content/_template.md
  i18n/{en,ro}.yaml
  assets/css/site.css  assets/fonts/ (+ licences)  assets/img/<key>/
  static/                   favicons, verbatim files
  worker/index.js  wrangler.toml
  infra/                    OpenTofu for the four zones
  bench/                    audit tools, pinned
  scripts/                  setup.sh, analytics.sh, audit helpers
  .github/workflows/        ci.yml, verify.yml, codeql.yml, scorecard.yml
  .claude/                  CLAUDE.md, skills: review-piece, translate-piece, publish-piece, link-piece
  docs/specs/               this spec;  docs/adr/;  docs/design/;  docs/plans/
  data/scorecard-manual.yaml
  README.md  RUNBOOK.md  LICENSE (MIT)  LICENSE-content (CC BY-NC-ND 4.0)
```

## 14. Author inputs required before M1

Tagline; About copy in EN and RO (role, company, what shipped, years; Evolution Labs link); portrait (≥ 800 px square, rights owned); LinkedIn, X, GitHub URLs; the AI-disclosure sentence; typeface pairing and accent from the Claude Design pass; two pieces in draft by M2.
