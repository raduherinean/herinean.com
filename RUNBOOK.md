# RUNBOOK — herinean.com

Everything operational lives here. If it isn't here, it isn't a supported operation.

## Accounts and secrets
- Cloudflare: one account, 2FA. Zones: herinean.com (content), herinean.ro / .net / .info (web redirect-only). All four are Google Workspace domains of one mailbox (herinean.net is the Workspace primary) and keep their MX. Workers Paid.
- Tokens: `herinean-infra` (operator; scoped to the four zones + Workers; recreate with the same permissions when it expires) in `~/.config/herinean/cf-infra.token` (mode 600). Permissions that turned out to be needed: Zone Settings, DNS, SSL and Certificates, Single Redirect, Bot Management, Workers Routes (all Edit) on the four zones; Workers Scripts and Workers KV (Edit) on the account. `herinean-ci` (GitHub Actions; Workers Scripts Edit, Workers KV Storage Edit, Account Analytics Read, Account Settings Read) as the repo secret `CLOUDFLARE_API_TOKEN` — set with `gh secret set CLOUDFLARE_API_TOKEN --repo raduherinean/herinean.com < file`, then delete the local file. Rotate: create new → update file/secret → delete old.
- Operator inputs: `~/.config/herinean/m0.env` (see `docs/plans/2026-09-17-m0-edge-foundation.md`, "Operator inputs"). Nothing in it is secret except the token it reads from its own file.
- GitHub: `raduherinean/herinean.com` (public), owned by the `rlucian` account through the `raduherinean` organisation. `main` is protected by the ruleset "main" (PR, signed commits, squash only, no force-push, no deletion). Commits are SSH-signed with `~/.ssh/herinean_signing`; the public key is registered on GitHub as a Signing Key; `radu@herinean.com` is the commit email.
- Remotes: `origin` = GitHub (public; `main` + publish PRs), `gitea` = a private Gitea (all work branches).

## Edge as code (OpenTofu, `infra/`)
1. `. scripts/env.sh` (and `export PATH="$HOME/.local/bin:$PATH"` — tofu lives there)
2. `cd infra && tofu plan -out x.plan` — read every line. A plan that destroys anything you did not intend to destroy is wrong; stop.
3. `tofu apply x.plan && rm x.plan && ../scripts/infra-backup.sh`
State is local (`infra/terraform.tfstate`, gitignored); backups in `~/.config/herinean/tfstate/`. Lost state: `tofu import` each resource (ids from the API; `scripts/infra-import-mail.sh` prints the mail ones); about an hour.
Not in tofu: the zones themselves; the `herinean.com` and `mta-sts.herinean.com` DNS records (created by the Workers custom domains); DS records at registrars; DKIM records on .net/.info and Google site-verification TXTs (pre-existing, left unmanaged).
Drift: anything changed in the dashboard shows up in the next `tofu plan` as an update back to the code. That is the point. Do not edit the four zones in the dashboard while a plan/apply is running.

## DNSSEC
`cd infra && tofu output ds_records` → paste at each registrar (key tag 2371, algorithm 13, digest type 2). Cloudflare shows `pending` until the DS is at the registrar, then `active`. Verify: `dig +dnssec A herinean.com @1.1.1.1` shows the `ad` flag; https://dnsviz.net/d/herinean.com/dnssec/.

## Mail
All four domains: Workspace MX, SPF `include:_spf.google.com -all`, DMARC with strict alignment reporting to the zone's Cloudflare DMARC Management address and to `dmarc@` on itself. Policies: `.ro` `.info` reject; `.com` `.net` per `dmarc_policy` in `infra/zones.auto.tfvars` (flip to reject only after a signed test message shows spf/dkim/dmarc pass). herinean.com additionally: DKIM selector `google` (2048), MTA-STS (`mode: testing` → `enforce`: edit `infra/placeholder/.well-known/mta-sts.txt` (M0) or the generator's `static/` (M1+), bump `mta_sts_id` in `zones.auto.tfvars`, apply, deploy), TLS-RPT to dmarc@. Change the MX set only via `var.mx`.

## Worker
- Preview (any branch): `scripts/preview.sh` → builds, runs `check --dist`, uploads a version, prints the `workers.dev` URL (noindex, no analytics). Verify: `scripts/verify-preview.sh <url>`.
- Production: `scripts/deploy.sh --yes-production` — launch criteria in spec §11; it publishes the scorecard to KV before `wrangler deploy` and verifies afterwards, so a launched colophon is never empty; CI deploys `main` once launched; this is the manual fallback.
- Rollback to the placeholder: `scripts/deploy-placeholder.sh`. Rollback to a previous version: `. scripts/env.sh && npx wrangler rollback <version-id>` (CI's own rollback and KV-stash restore are automatic on a failed post-deploy verification — see CI, below).
- Local: `go build -tags nodynamic -o site ./cmd/site && ./site build && . scripts/env.sh && npx wrangler dev --port 8787` — wrangler dev presents requests with the first route's host, so the production path runs locally (local KV/AE); `--var PROD_HOST:other` simulates a preview host; tests `cd worker && node --test`.
- Bindings live in `wrangler.toml`: `ASSETS` (dist/), `VIEWS` (Analytics Engine `herinean_views`), `SCORECARD` (KV). The KV namespace id is committed; recreate with `wrangler kv namespace create SCORECARD` and update the id if it is ever lost.
- Scorecard: `scripts/scorecard-publish.sh [scorecard.json]` writes two KV keys — `scorecard` (the HTML fragment, with the Worker's ETag suffix as its `etag` metadata) and `scorecard.json`; the colophon reflects it on the next request. Empty KV → the built-in table from the last build (a preview-only state: `deploy.sh` publishes before it deploys, and the post-deploy verify fails if the colophon is not KV-filled).
- Fonts, images, OG cards, feeds and machine files never reach the Worker (`run_worker_first` in wrangler.toml); the Worker handles everything else — pages, `robots.txt`/`llms.txt`/`security.txt`, and the colophon's `scorecard.json`.
- Two things the Worker corrects on the asset layer's responses: `/writing` comes back as a 307 to `/writing/` and leaves as a 301 for GET and HEAD (URLs are permanent, decision #14); bare `text/html` and `text/plain` gain `charset=utf-8` (llms.txt and the Romanian pages are UTF-8 and say so).
- `/404.html` redirects to `/404/` (asset-layer canonicalisation, now a 301); the page is `noindex`, so leave it.

## CI
- `.github/workflows/ci.yml` runs on every pull request and push to `main`: `build` (gofmt, vet, staticcheck, actionlint, tests, `site check`, two builds compared, `check --dist`) → `audit` (`.github/workflows/audit.yml`: a Lighthouse matrix of form factor × shard, one `checks` job for every other row, a `merge` job that writes `scorecard.json` — a row no job delivered is a red `not measured` row — and, on pull requests as on `main`, renders the fragment with `site scorecard` and validates it with Nu before it fails on any red row) → on pull requests `preview` (`wrangler versions upload`, a sticky comment with the URL; the preview is `noindex` but readable by anyone with the link) and `comment` (adds the audit table to that same comment) → on `main`, once `site.yaml` has `launched:`, `deploy` → `post` → `post-rows` → `publish`.
- Before launch, `publish-ci` writes the CI rows to KV on every push to `main`, so the preview colophon shows real rows while production stays on the placeholder; production's `/colophon/scorecard.json` then names the audited build, which the placeholder is not, until launch. The pull-request `comment` job keeps one sticky comment per PR (marker `<!-- herinean-ci -->`) with the preview URL and the audit table.
- Secrets (repository level, one copy): `CLOUDFLARE_API_TOKEN` — a token scoped to this Worker, its KV namespace and Account Analytics read, nothing zone-wide — and `CLOUDFLARE_ACCOUNT_ID`. Without them `preview` and `publish-ci` skip with a line in the job summary; after launch `deploy` fails at its first step with a message naming the secret, and `post`, `post-rows` and `publish` are skipped behind it (on `workflow_dispatch`, `post` runs without secrets and `publish` fails the same way) — the secrets are part of the launch checklist; `build` and `audit` need nothing.
- Reading a red row: the job summary has the table; the `rows-*` artifacts carry `*.detail.json` with every problem; `scripts/bench.sh` runs the same modules locally against a built `dist/` (Java ≥ 11 for Nu; Playwright Chromium).
- `deploy`: verifies the artifact against `dist.sha256`, stashes the live KV scorecard as an artifact, publishes the CI rows, records the live version id, `wrangler deploy`. On launch day `previous` is the placeholder version, so the automatic rollback of a failed first deploy is a rollback to the placeholder. `post` tier 1 runs `scripts/verify-preview.sh https://herinean.com --prod`; on failure it rolls back to the recorded version, restores the stash only once that rollback succeeded, and fails the run (GitHub mails the committer). If Cloudflare refuses the rollback (bindings changed, or more than ten versions back) the run fails with the error — re-deploy the previous commit by hand: `git checkout <sha> && scripts/deploy.sh --yes-production`. Tier 2 (`post-rows`) measures the production rows and only reports; `publish` writes CI + post rows to KV. Post rows are absent for the minutes between a deploy and its `post`.
- A run killed between the publish and `wrangler deploy` leaves the new rows on the old build: the `Audited build` row shows it; re-run the workflow (`workflow_dispatch` re-verifies production and republishes without deploying). `workflow_dispatch` is for the commit production serves: it runs `post` only when that commit's audit is green, and the `Audited build` row names HEAD, so dispatching it on a newer, undeployed commit would mislabel the rows.
- A manual `scripts/deploy.sh --yes-production` is unaudited by definition and says so on the colophon (`Audited build … manual deploy, not audited by CI`) until the next CI publish.
- Launch day (spec §11): create the GitHub environment `production` with deployment branches limited to `main` (the workflow also refuses any other ref); set `launched: YYYY-MM-DD` in `site.yaml` in one commit (`Launch: herinean.com`), open the PR, merge; CI deploys and fills the colophon. Then: HSTS preload submission, Search Console and Bing verification, the URL into LinkedIn; re-run internet.nl, SSL Labs and securityheaders.com and replace the `measured: placeholder` rows in `data/scorecard-manual.yaml` with `measured: production`; delete the manual Observatory row (the post row replaces it); run the W3C Feed Validator once and add its row.
- Pull requests from forks and from Dependabot get `build` and `audit` only (no secrets). A bench bump that turns a row red is the signal to fix or pin — never to loosen the row.
- Reproducibility: `build` builds `dist/` twice and diffs `scripts/dist-hash.sh`; one-off cross-architecture check: download `dist.sha256` from a green run of a commit, build the same commit locally with `SOURCE_DATE_EPOCH=$(git log -1 --format=%ct)`, `scripts/dist-hash.sh | diff - dist.sha256`. Result recorded here: (pending the first push).
- Actions are pinned by commit SHA (Dependabot bumps them); `GOTOOLCHAIN=local` with `go-version-file: go.mod` — no silent toolchain download; Go tools (`staticcheck`, `actionlint`) are `tool` directives in `go.mod`.

## Fonts
- Shipped: `assets/fonts/web/*.woff2` (three files, ≤ 100 KB total, `BUILD.txt` records the exact inputs and fonttools version). Regenerate only when changing fonts: `scripts/fonts.sh` (needs `~/.local/share/fonttools`, see `scripts/setup.sh`), then paste `assets/fonts/web/fallback.css` into `site.css` — `site check` fails on drift. `font-display: optional` — the swap was measured and it shifts (ADR-0010).
- Both families are pinned from one google/fonts commit. Source Serif 4's binaries reserve the name "Source" (OFL §3) — Google's licence text declares none, the font's own copyright string does — so the subset ships as "Herinean Serif" with the copyright and licence strings untouched; `assets/fonts/web/OFL-*.txt` are the licences that apply.
- Fallbacks are Times New Roman / Liberation Serif with computed overrides; Android has neither Times nor Liberation, so a cold visit there renders a plain serif at its own metrics — no shift (nothing swaps under `optional`), just a different first page; row 21 measures on Linux CI.

## Analytics
- `scripts/analytics.sh [--days 30|90] [--by path|ref|country|referrer]` — live Analytics Engine query (token needs Account Analytics: Read). Weekly snapshots to KV and the merge are M2b.
- What is stored, verbatim from the privacy page: path, language, referring host, `ref` tag, country, 1. Nothing that identifies a reader or links two visits to the same person. Verification runs (curl, the verify scripts) match the bot filter and are never counted; neither are speculative loads (`Sec-Purpose`/`Purpose: prefetch`, prerender) or non-document fetches (an iframe, a script on someone else's page).

## Generator
- Build the binary with `go build -tags nodynamic -o site ./cmd/site` (gitignored) — the `nodynamic` tag is what keeps the WebP encoder pure Go (see below); `ci.yml` sets `GOFLAGS: -tags=nodynamic` once, so `go build`, `go vet`, `go tool staticcheck` and `go test ./...` all pick it up, since the image tests encode too. Prefer the built binary over `go run`: `go run` exits 1 for any non-zero child status, so it turns exit 3 into a plain failure and hides which case you are in.
- `./site build` — reads `site.yaml`, `content/`, `i18n/`, `assets/`, `templates/` and writes `dist/`.
- `./site check [--dist]` — validates the sources without writing; `--dist` additionally checks a built `dist/`.
- `./site serve [--host 0.0.0.0] [--port 8080]` — rebuilds on each request and serves `dist/` for local/LAN preview. It is where a piece gets written: the fields `site new` leaves blank (title, date, pillar, summary) render with visible draft defaults and are listed on stderr at each rebuild; `build` and `check` still refuse them. Everything else (cedilla, missing alt, broken link, …) fails the preview with the same `file:line` message.
- `./site new <en|ro> <slug>` — creates a new piece from `content/_template.md` under `content/<lang>/`.
- `./site scorecard` — prints the public build scorecard (also rendered on the colophon page).

Exit codes: `0` everything checks out; `1` a real problem (bad front matter, missing i18n, cedilla, missing image, broken link, …); `3` only author inputs are missing (⟨placeholder⟩ text and/or `assets/portrait.jpg`) — the pre-commit hook warns and allows this, CI fails on it.

Reproducibility: the build's notion of "now" is `SOURCE_DATE_EPOCH` if set, else the last commit's timestamp — wall-clock time never enters the output, so two builds of the same commit are byte-identical. `.cache/` is the image-processing cache (OG cards, resized images), keyed by the Go version, the encoder module versions and the in-repo encoding settings (quality, resize kernel, `ogLayout` — bump that constant when the OG card's drawing changes); it is safe to delete and will be rebuilt.
- CI must clone with `fetch-depth: 0`: sitemap `lastmod` comes from each page's last commit, and a shallow clone makes every page carry HEAD's time.
- CI pins the toolchain with `GOTOOLCHAIN=local` and `go-version-file: go.mod` — no silent toolchain download; the colophon prints the Go version, so an unpinned toolchain would change the output.
- Cross-architecture reproducibility (a CI amd64 `dist/` hashing equal to a local arm64 one) is the one-off check in CI, above.

If `site build` stops with `webp: a host libwebp was loaded…`, build with `-tags nodynamic` — the generator refuses host-dependent image bytes. `CGO_ENABLED=0` is not a remedy: purego loads the host library through its own fakecgo runtime on Linux, so only the build tag compiles the loader out.

Portrait: export a pre-rotated JPEG at least 800 px wide to `assets/portrait.jpg` (the resizer ignores EXIF orientation), metadata stripped — the repo is public and the file is served as is at 480 px. Portrait orientation is fine: the home page keeps the aspect; the OG card centre-crops a square, so keep the face near the middle.

## TLS
Minimum TLS 1.3 (see ADR-0011 for the evidence). Restricting the TLS 1.2 cipher list needs Advanced Certificate Manager ($10/month), which is why 1.2 is off rather than "on with modern ciphers".

After production cutover, re-run all four external audits (internet.nl, SSL Labs, Mozilla Observatory, securityheaders.com) against the production site and replace the placeholder-era rows in `data/scorecard-manual.yaml` with rows marked `measured: production`; the colophon labels every manual row with the deployment it was measured on. Then publish the scorecard (`scripts/deploy.sh` does).

HSTS preload: **not submitted yet.** All four zones have served `strict-transport-security: max-age=63072000; includeSubDomains; preload` since 2026-09-17 (`f41bd6b`); hstspreload.org wants the header live for a while before submission, so submit no earlier than 2026-09-25 and record the dates here, per domain.

## Verification
`scripts/verify-edge.sh` — the edge test suite (settings, redirects, CAA, mail DNS, DNSSEC, apex headers, HTTP/3, TLS). Resolves through 1.1.1.1 with `curl --resolve`, so local resolver caches cannot fake a result. Run after any infra change. This machine's `curl` lacks HTTP/3; the suite then checks the `alt-svc: h3` advertisement instead.
`scripts/bench.sh` — runs the audit bench locally against a build or a URL, for reproducing a red row from the colophon (see `bench/README.md`).
External: Mozilla Observatory (`curl -X POST 'https://observatory-api.mdn.mozilla.net/api/v2/scan?host=herinean.com'`), SSL Labs API (`https://api.ssllabs.com/api/v3/analyze?host=herinean.com&startNew=on&all=done`, poll until READY), internet.nl (start `https://internet.nl/site/herinean.com/`, poll `/site/probes/herinean.com/`, results at `/site/herinean.com/results`; retest window ~5 minutes), securityheaders.com (browser only; blocks scripted access), dnsviz. HSTS preload submitted: <dates>.

## Incidents
- Cloudflare-wide outage: nothing to do; the site has no origin. Site recovery elsewhere: `site build` → any static host with the `_headers` equivalents; DNS TTL is auto.
- Under attack: "Under Attack" mode serves a JS challenge and breaks the zero-JS promise for the duration. Prefer a WAF rate-limit rule first.
- Billing failure on Workers Paid drops the account to Free quotas (100k req/day). Billing alert is set; fix the card.
- Domains: auto-renew and registrar lock on all four; weekly job warns at 60 days to expiry (M2b).
- Production serving a broken build: `post`'s tier-1 check rolls it back automatically; a rollback Cloudflare refuses, or a rollback needed outside that window, is the manual path in CI, above (`npx wrangler rollback <version-id>`).
