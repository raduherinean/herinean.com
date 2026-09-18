# RUNBOOK — herinean.com

Everything operational lives here. If it isn't here, it isn't a supported operation.

## Accounts and secrets
- Cloudflare: one account, 2FA. Zones: herinean.com (content), herinean.ro / .net / .info (web redirect-only). All four are Google Workspace domains of one mailbox (herinean.net is the Workspace primary) and keep their MX. Workers Paid.
- Tokens: `herinean-infra` (operator; scoped to the four zones + Workers; recreate with the same permissions when it expires) in `~/.config/herinean/cf-infra.token` (mode 600). Permissions that turned out to be needed: Zone Settings, DNS, SSL and Certificates, Single Redirect, Bot Management, Workers Routes (all Edit) on the four zones; Workers Scripts and Workers KV (Edit) on the account. `herinean-ci` (GitHub Actions; Workers Scripts Edit, Workers KV Storage Edit, Account Analytics Read, Account Settings Read) as the repo secret `CLOUDFLARE_API_TOKEN` — set with `gh secret set CLOUDFLARE_API_TOKEN --repo raduherinean/herinean.com < file`, then delete the local file. Rotate: create new → update file/secret → delete old.
- Operator inputs: `~/.config/herinean/m0.env` (see `docs/plans/2026-09-17-m0-edge-foundation.md`, "Operator inputs"). Nothing in it is secret except the token it reads from its own file.
- GitHub: `raduherinean/herinean.com` (public), owned by the `rlucian` account through the `raduherinean` organisation. `main` is protected by the ruleset "main" (PR, signed commits, squash only, no force-push, no deletion). Commits are SSH-signed with `~/.ssh/herinean_signing`; the public key is registered on GitHub as a Signing Key; `radu@herinean.com` is the commit email.
- Remotes: `origin` = GitHub (public; `main` + publish PRs), `gitea` = git.thrac.com (private; all work branches).

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
- M0 placeholder: `scripts/deploy-placeholder.sh`. M1+: CI deploys on merge to `main`.
- Roll back: `npx wrangler@4 rollback` (previous version) — or `npx wrangler@4 deployments list` then `rollback <id>`.
- Preview: `npx wrangler@4 versions upload` prints a `workers.dev` preview URL. Cloudflare stamps every preview response `x-robots-tag: noindex` itself (it overrides the Worker's own value); the Worker additionally serves a disallow-all robots.txt there.
- Custom domains are declared in `wrangler.toml`; wrangler creates their DNS records. `run_worker_first` limits the Worker to page routes.
- `_headers` apply to asset responses only, not to responses the Worker builds itself.

## Generator
- Build the binary with `go build -tags nodynamic -o site ./cmd/site` — the `nodynamic` tag is what keeps the WebP encoder pure Go (see below); `ci.yml` (M2) must pass the same tag. The `go run` forms below take the same `-tags nodynamic`; without it the build refuses to run on a machine that has libwebp installed.
- `go run ./cmd/site build` — reads `site.yaml`, `content/`, `i18n/`, `assets/`, `templates/` and writes `dist/`.
- `go run ./cmd/site check [--dist]` — validates the sources without writing; `--dist` additionally checks a built `dist/`.
- `go run ./cmd/site serve [--host 0.0.0.0] [--port 8080]` — rebuilds on each request and serves `dist/` for local/LAN preview.
- `go run ./cmd/site new <en|ro> <slug>` — creates a new piece from `content/_template.md` under `content/<lang>/`.
- `go run ./cmd/site scorecard` — prints the public build scorecard (also rendered on the colophon page).

Exit codes: `0` everything checks out; `1` a real problem (bad front matter, missing i18n, cedilla, missing image, broken link, …); `3` only author inputs are missing (⟨placeholder⟩ text and/or `assets/portrait.jpg`) — the pre-commit hook warns and allows this, CI fails on it.

Reproducibility: the build's notion of "now" is `SOURCE_DATE_EPOCH` if set, else the last commit's timestamp — wall-clock time never enters the output, so two builds of the same commit are byte-identical. `.cache/` is the image-processing cache (OG cards, resized images), keyed by the Go version and the encoder module versions; it is safe to delete and will be rebuilt.
- CI must clone with `fetch-depth: 0`: sitemap `lastmod` comes from each page's last commit, and a shallow clone makes every page carry HEAD's time.
- CI pins `GOTOOLCHAIN=go1.27.1`: the colophon prints the Go version, so a different toolchain changes the output.
- M2 verifies once that a CI (amd64) `dist/` hashes equal a local (arm64) one before claiming cross-machine byte identity — float rounding in the resizer may differ between architectures.

If `site build` stops with `webp: a host libwebp was loaded…`, build with `-tags nodynamic` — the generator refuses host-dependent image bytes. `CGO_ENABLED=0` is not a remedy: purego loads the host library through its own fakecgo runtime on Linux, so only the build tag compiles the loader out.

Portrait: export a square, pre-rotated JPEG ≥ 800 px to `assets/portrait.jpg` — the resizer ignores EXIF orientation.

## TLS
Minimum TLS 1.3 (see ADR-0011 for the evidence). Restricting the TLS 1.2 cipher list needs Advanced Certificate Manager ($10/month), which is why 1.2 is off rather than "on with modern ciphers".

## Verification
`scripts/verify-edge.sh` — the edge test suite (settings, redirects, CAA, mail DNS, DNSSEC, apex headers, HTTP/3, TLS). Resolves through 1.1.1.1 with `curl --resolve`, so local resolver caches cannot fake a result. Run after any infra change. This machine's `curl` lacks HTTP/3; the suite then checks the `alt-svc: h3` advertisement instead.
External: Mozilla Observatory (`curl -X POST 'https://observatory-api.mdn.mozilla.net/api/v2/scan?host=herinean.com'`), SSL Labs API (`https://api.ssllabs.com/api/v3/analyze?host=herinean.com&startNew=on&all=done`, poll until READY), internet.nl (start `https://internet.nl/site/herinean.com/`, poll `/site/probes/herinean.com/`, results at `/site/herinean.com/results`; retest window ~5 minutes), securityheaders.com (browser only; blocks scripted access), dnsviz. HSTS preload submitted: <dates>.

## Incidents
- Cloudflare-wide outage: nothing to do; the site has no origin. Site recovery elsewhere: `site build` → any static host with the `_headers` equivalents; DNS TTL is auto.
- Under attack: "Under Attack" mode serves a JS challenge and breaks the zero-JS promise for the duration. Prefer a WAF rate-limit rule first.
- Billing failure on Workers Paid drops the account to Free quotas (100k req/day). Billing alert is set; fix the card.
- Domains: auto-renew and registrar lock on all four; weekly job warns at 60 days to expiry (M2).
