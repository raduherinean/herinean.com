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
| `i18n.mjs` | i18n | `<html lang>` matches the path; exactly one absolute canonical with a trailing slash; hreflang alternates are symmetric (the other page links back to me under my own language) plus `x-default` |

Later tasks add one module per remaining spec §5 row; `audit.mjs`'s `MODULES.checks` and
`MODULES.lighthouse` lists grow to name each one as it lands.

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
