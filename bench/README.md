# The bench

The audit bench: one Node entry point that measures a served build against the scorecard rows
of the design spec (spec §7, `docs/specs/2026-09-19-m2a-ci-bench-scorecard-design.md`) and writes
rows in the exact JSON shape `internal/site/scorecard.go`'s `ciRow` reads: `{check, pass, value,
when, link, link_text}`. CI runs it against `site serve --static` on every pull request and push
(mode `ci`), and again against production after a deploy (mode `post`); `scripts/bench.sh`
(arrives in a later task) is the local escape hatch — the colophon only ever shows CI's numbers.

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

Modules implemented so far:

| Module | Row | What it checks |
|---|---|---|
| `lighthouse.mjs` | Lighthouse | the [`lighthouse`](https://www.npmjs.com/package/lighthouse) API against Playwright's Chromium, launched via `chrome-launcher`; mobile and desktop; 3 runs per page, median per category; every sitemap page (`/404.html` excluded) must score 100 on performance, accessibility, best-practices and seo. Sharded — see "Sharding Lighthouse" below. `modes = ["ci", "post"]` |
| `i18n.mjs` | i18n | `<html lang>` matches the path; exactly one absolute canonical with a trailing slash; hreflang alternates are symmetric (the other page links back to me under my own language) plus `x-default` |
| `social.mjs` | Social previews | every page has `og:*` + `twitter:card=summary_large_image`; `og:image` is a 1200×630 PNG under 200 KB at a content-hashed `/og/` URL |
| `wellknown.mjs` | Well-known files | `security.txt` (RFC 9116 fields, `Expires` valid and ≤ 1 year out), `robots.txt` with a `Sitemap:` line, `sitemap.xml` entries carry `lastmod` and `x-default`, `llms.txt`, favicons, and a real 404 status on an unknown path |
| `feeds.mjs` | Feeds | RSS 2.0 (`/feed.xml`, `.en`, `.ro`) strict-parses, required channel elements, `atom:link rel=self`, items have full text and only absolute URLs; JSON Feed 1.1 required fields |
| `jsonld.mjs` | Structured data | every `ld+json` block parses; each known `@type` (`WebSite`, `Person`, `BlogPosting`, `BreadcrumbList`) carries its required fields; `Person.sameAs` has LinkedIn, X and GitHub |
| `headers.mjs` | Security headers | served headers per path class (`/`, a preloaded font, `og:image`, `/img/*`, feeds, `sitemap.xml`, `robots.txt`, `llms.txt`, `security.txt`) match spec §6.2; the CSP's style-src hash equals the sha256 of each page's own inline `<style>`; HSTS and no `Set-Cookie` everywhere. `modes = ["ci", "post"]` |
| `privacy.mjs` | Privacy | a real Chromium load of every page: no `Set-Cookie`, no third-party request, no `/cdn-cgi/` in the HTML, no `<script>` besides `application/ld+json`. `modes = ["ci", "post"]` |
| `weight.mjs` | Weight | HTML+CSS ≤ 30 KB brotli; fonts ≤ 100 KB total with exactly one preloaded; first view (Chromium, mobile viewport) ≤ 6 requests and ≤ 150 KB brotli-equivalent; 0 bytes of executable JS |
| `links.mjs` | Links | every same-origin `href`/`src`/`srcset` across every page (incl. `/404.html`) resolves with 200, or the row fails; external links get a 10 s timeout and only warn, checked against `bench/links-allow.txt` |
| `html.mjs` | HTML validity | the [Nu Html Checker](https://validator.github.io/validator/) (`vnu-jar`) reports 0 errors and 0 warnings; `html-validate` (`bench/.htmlvalidate.json`: `recommended` + `a11y` + `document`) reports 0 errors — every page, `/404.html` included. `modes = ["ci", "post"]` |
| `a11y.mjs` | Accessibility | [`@axe-core/playwright`](https://github.com/dequelabs/axe-core-npm) against WCAG 2.2 AA in a real Chromium, in both `light` and `dark` `prefers-color-scheme`; AAA contrast (`color-contrast-enhanced`) everywhere except the `--ink-2` secondary-text selectors (`bench/lib/a11y.mjs`'s `SECONDARY`, kept in step with `assets/css/site.css`); skip link is the first tab stop; every interactive element gets a visible `:focus-visible` outline; tab stops account for every interactive element; the language-switch link carries `lang`. `modes = ["ci", "post"]` |

Later tasks add one module per remaining spec §5 row (`transport`, `observatory`, `dns`,
`caching`, `fonts` — all `post`-only); `audit.mjs`'s `MODULES.checks` list grows to name each one
as it lands.

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

## CLI

```
node bench/audit.mjs BASE --mode ci|post [--only lighthouse|checks] [--form-factor mobile|desktop] \
  [--shard i/n] [--dist DIR] [--link URL] [--link-text TEXT] --out FILE
```

Fetches `BASE`'s sitemap, runs every wanted module, writes `FILE` (the row array) and
`FILE.detail.json`. Always exits 0, even when rows fail — `merge.mjs` is the gate, not the audit.

```
node bench/merge.mjs --out scorecard.json [--build SHA --build-url URL] [--summary FILE] [--no-gate] rows.json…
```

Folds any number of row files into one `scorecard.json`: orders rows per the spec (`ORDER` in
`merge.mjs`), folds Lighthouse shard rows (`Lighthouse [mobile 1/2]`, …) into a single row, and
prepends an `Audited build` row when `--build` is given. Writes the Markdown table to `--summary`
(appending, for a CI job summary) and exits 1 when any row is red, unless `--no-gate`.

## Testing

`node --test test/*.test.mjs` (via `npm test`). `bench/test/helpers.mjs`'s `serveFixture(dir)`
serves a built-site directory the way `site serve --static` would (`_headers` applied,
`index.html` for directories, `404.html` on a miss) without depending on the Go binary.
`bench/fixtures/site-ok/` is a minimal built site (see `bench/fixtures/README.md`) used by every
module test.

## Pins

`bench/package.json` pins every dependency to an exact version (no `^`/`~`); `bench/package-lock.json`
is committed. `bench/node_modules/` is gitignored.
