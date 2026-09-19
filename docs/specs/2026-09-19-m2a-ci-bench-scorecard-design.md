# M2a — CI, bench, scorecard pipeline, deploy: design

**Status (2026-09-19):** designed; each section reviewed adversarially against the north star and the design spec before approval (see "Decisions for Radu" at the end for the calls made under delegation). Implementation plan: `docs/plans/2026-09-19-m2a-ci-bench-scorecard.md`.

**Parent spec:** `docs/specs/2026-09-17-herinean-com-design.md` — §3 (the scorecard), §7 (CI, scorecard pipeline, colophon), §11 (order of work). M2 as the spec describes it is delivered in two halves: **M2a** (this document) makes `main` deployable with every CI-enforced and post-deploy row measured by machines; **M2b** (next design) adds the weekly job, the uptime monitor, the four skills, and the repository guardrails that need nothing from M2a but time.

## 1. Goal

Every pull request is built, checked and audited against the scorecard by GitHub Actions; a merge to `main` deploys the audited build to Cloudflare, verifies it on production, rolls back if the deploy itself is broken, measures the post-deploy rows, and writes the whole scorecard to KV, where the Worker fills the colophon from it. Nothing on the colophon is typed by hand except the four external-tool rows in `data/scorecard-manual.yaml`.

## 2. Scope

### 2.1 M2a delivers

1. **`.github/workflows/ci.yml`** and the reusable **`audit.yml`**: build → audit (sharded) → preview on pull requests → deploy on `main` once launched → post-deploy → publish.
2. **`bench/`**: one Node entry point, pinned by lockfile, that audits one base URL and writes scorecard rows. It covers the CI rows 1–9, 14, 15, 16, 21 and the post rows 1, 2, 3, 9, 10, 12, 13 (the DNS half), 15, 17 of spec §3.
3. **The scorecard pipeline**: bench rows → `scorecard.json` → `site scorecard` → a Nu-validated HTML fragment → KV (`scorecard`, `scorecard.json`), before every deploy and again after the post-deploy rows exist.
4. **Deploy with rollback**, gated by a committed launch marker.
5. **Generator changes**: `site serve --static`, the `launched:` field, stale marking on every scorecard row.
6. **Hygiene the M1a/M1b reviews required**: Dependabot; actions pinned by commit; `GOTOOLCHAIN` pinned; `-tags nodynamic` everywhere; full-depth checkout; a per-run reproducibility check and the one-off arm64/amd64 hash comparison; wrangler pinned by a lockfile instead of a shell variable; scripts that run in CI without the operator's environment file.
7. **Leftovers that sit on M2a's path**: the Worker's generated responses carry the `_headers` security set; `/colophon/scorecard.json` is served on every host; three stale doc comments.
8. **Docs**: RUNBOOK "CI" section, README badge, ADR-0015, spec amendments as their own commit.

### 2.2 M2b (next design)

`verify.yml` weekly (external links with lychee, `security.txt` expiry, RDAP domain expiry, globalping latency, websitecarbon, analytics snapshot → KV); the external uptime monitor; the four skills and the repository `CLAUDE.md`, including the pre-commit gap for dateless drafts on `piece/*`; CodeQL and OpenSSF Scorecard with their badges; the display-face cold-visit probe from a far region and, on its number, a second preload or Early Hints; the optional Android fallback face; the Newsreader name table; Worker tests under workerd.

### 2.3 Ordering

M2b lands before the first `Publish:` pull request: the colophon says a fact-check pass exists before the first piece is published, and `/publish-piece` is what stamps a piece's date. Launch (spec §11) needs M2b's weekly rows for row 19 in any case. M2a alone makes `main` deployable; it does not launch.

### 2.4 Known follow-up

Row 14's "one preloaded" is encoded in `check --dist` and in the bench. If M2b's cold-visit probe decides on a second preload, row 14, `checkdist` and the bench change together.

## 3. Decisions taken in the design (with the alternatives rejected)

1. **The bench is CI's; locally it is an escape hatch.** `scripts/bench.sh` runs the same entry point when the tools are present and says what is missing otherwise. The colophon shows only CI's numbers. Rejected: a container image pinned by digest for bit-identical local runs (two image builds, a slower pipeline, Chromium in Docker) and no local story at all.
2. **Row 1 stays as written and is sharded.** Every page, mobile and desktop, three runs, median — split across a parallel matrix whose size follows the sitemap. Wall clock stays flat as pieces accumulate; runner minutes are free on a public repository. Rejected: one run per page in CI (a noisy runner fails a PR on one bad run; the CI row would no longer say what the spec row says) and auditing only the pages a PR touched (a template change touches every page, so it degrades to the full set exactly when it matters).
3. **The launch is a commit.** `site.yaml` gains `launched: YYYY-MM-DD`; the deploy job runs only when it is set. Rejected: a `production` environment with a required reviewer (a click before every publish, against spec §8's "deployed … after the merge") and a repository variable (invisible in the history).
4. **One Node orchestrator, tools as libraries.** Lighthouse, Playwright with axe, and html-validate are used programmatically; Nu is spawned from `vnu-jar`; the custom checks are plain JavaScript. Rejected: a Go `site audit` shelling out to CLIs (the JS tools are libraries first; the platform's ~10 dependencies would absorb the bench's hundreds, the wrong way around per spec §7) and one marketplace action per tool (nothing runs locally; results scattered; each action a third party on the deploy path).
5. **Post-deploy failures are two tiers.** Tier 1 verifies the deploy (the site's own invariants on production) and rolls back on failure. Tier 2 measures the post rows and only reports. Spec §3's "post … cannot block" is tier 2.
6. **Internal links are a bench check, not a `check --dist` rule.** A link check needs the served pages anyway; one implementation. External links in CI are the bench's own fetch, warn-only with an allowlist; lychee is the weekly job's tool (M2b).
7. **KV is read and written over the REST API** (`scripts/kv.sh`), not wrangler's `kv` subcommands: the REST call returns metadata, which the rollback stash needs, and it needs nothing but the token.

## 4. Workflow topology

**Files:** `.github/workflows/ci.yml` (the pipeline) and `.github/workflows/audit.yml` (a `workflow_call` that audits one base URL, called against the local server on every run and against production after a deploy).

**Triggers:** `pull_request` targeting `main`; `push` to `main`; `workflow_dispatch` (re-verify production and republish the scorecard without deploying).

**Jobs in `ci.yml`:**

1. **`build`** — checkout with full history; Go from `go.mod` with `GOTOOLCHAIN=local`; `gofmt -l`, `go vet`, `go tool staticcheck`, `go test ./...`, all with `-tags nodynamic`; `go tool actionlint`; Node 24 and the Worker tests; `site check` (exit 3 fails here — author inputs are not optional in CI); `site build` twice, hash lists compared (reproducibility, spec row 20); `site check --dist`. Artifacts: `dist/`, the `site` binary, `dist.sha256`. Outputs: `launched` (from `site.yaml`), `shards` = ⌈sitemap URLs ÷ 4⌉.
2. **`audit`** — calls `audit.yml` with `base=http://127.0.0.1:8080`, `mode=ci`. Inside: a **`lighthouse`** matrix of form factor × shard, each shard serving the artifact with `site serve --static` and running the three-run median on its slice of the sitemap; one **`checks`** job for every other row; a **`merge`** job that folds the row files into `scorecard.json`, renders the fragment with `site scorecard`, validates the fragment with Nu, writes the table to the job summary, uploads both, and **fails when any row is red** — the gate.
3. **`preview`** — pull requests from this repository, not from forks and not Dependabot's (neither receives secrets); runs after `build`, in parallel with the audit, so the URL arrives early: `wrangler versions upload` from the artifact, then one sticky PR comment (the URL at once, the scorecard table appended when `merge` finishes). Skipped with a summary line when the Cloudflare secret is absent. Never writes KV.
4. **`deploy`** — `push` to `main`, `launched` set, `merge` green, environment `production` (deployment branch `main`, no reviewer): verify the artifact against `dist.sha256`; record the live version id; stash the current KV scorecard (fragment, metadata, JSON) as an artifact; publish the CI rows; `wrangler deploy`; output the old and new version ids. Any failure after the publish restores the stash.
5. **`post`** — needs `deploy`, or runs on `workflow_dispatch` once launched. **Tier 1:** `scripts/verify-preview.sh https://herinean.com --prod`; on failure `wrangler rollback <old id>` with the run URL as the message, restore the stash, fail the job. On a dispatch nothing was deployed, so tier 1 checks but never rolls back. **Tier 2:** `audit.yml` with `base=https://herinean.com`, `mode=post`. Reported, never rolls back, `always()`.
6. **`publish`** — CI rows + post rows → fragment → Nu → KV `scorecard` (with the `etag` metadata) and `scorecard.json`. Skipped when tier 1 rolled back: the stash is the truth then.

**Before launch** the `main` graph is build → audit → publish CI rows (the preview colophon carries real rows); `deploy` and `post` are skipped with a summary line. Fork pull requests get build + audit only.

**Permissions:** `contents: read` at the workflow level; `preview` adds `pull-requests: write`. **Secrets:** `CLOUDFLARE_API_TOKEN` (the CI token: this Worker, its KV namespace, Analytics read — no zone scope) and `CLOUDFLARE_ACCOUNT_ID`, repository-level, one copy. The `production` environment adds the branch restriction and the deployments log. **Concurrency:** pull requests cancel superseded runs per PR; `main` serialises without cancelling. Every job has a timeout. Every action is pinned by commit SHA with a version comment; only `actions/*` are used — the PR comment goes through the runner's `gh` with `github.token`, wrangler comes from the lockfile.

**Budget:** build ≈ 1.5 min → max(lighthouse shard ≈ 2, checks ≈ 2) → merge ≈ 0.5: about 4–4.5 min for a pull request, flat in page count. After launch, merge-to-live ≈ 5 min (the audit must be green first), post rows and the second publish about 4 min later.

## 5. The bench

**Layout:** `bench/package.json` (exact versions, lockfile, `npm ci`); `bench/audit.mjs` — `node bench/audit.mjs <base> --mode ci|post [--only lighthouse|checks] [--form-factor mobile|desktop] [--shard i/n] --out rows.json`; `bench/merge.mjs`; `bench/lib/` one module per row; `bench/fixtures/` and `bench/test/` (`node --test`); `bench/links-allow.txt`; `bench/README.md`.

`pages.mjs` reads `/sitemap.xml` once, fetches every URL (headers and body) and hands the set to each module; `/404.html` is added for Nu and axe only. A module returns `{check, pass, value, link, link_text, detail}`; `detail` goes to a debug artifact, the rest is exactly the JSON `site scorecard` reads.

| Row | Module | CI (`site serve --static`) | Post (production) |
|---|---|---|---|
| 1 | `lighthouse.mjs` | Lighthouse API on Playwright's Chromium; mobile and desktop; three runs, median; every category 100 on every page; sharded | same, sharded |
| 2 | `html.mjs` | Nu (`vnu-jar`, one JVM over every dist HTML file, 0 errors, 0 warnings) + `html-validate` strict config | Nu + html-validate on `/colophon/` (edge-filled) |
| 3 | `a11y.mjs` | axe via Playwright, tags wcag2a/2aa/21a/21aa/22aa, light and dark; AAA contrast on everything except the named secondary selectors (listed in the module and printed in the row's value), which must pass AA; the skip link is the first tab stop; focused links have a visible outline; tab stops = interactive elements; the language link carries `lang` | on `/colophon/` |
| 4 | `links.mjs` | internal: every same-origin href/src resolves on the served dist — fail; external: fetch with timeout, allowlist — warn only | — (weekly, M2b) |
| 5 | `feeds.mjs` | strict XML parse; RSS 2.0 required elements; `content:encoded` full text; every URL absolute; `atom:link rel=self` equals its own URL; JSON Feed 1.1's required and typed fields (jsonfeed.org publishes no JSON Schema; the fields are checked directly) | — |
| 6 | `jsonld.mjs` | every `application/ld+json` block parsed; required fields per type (WebSite; Person with three `sameAs`; BlogPosting; BreadcrumbList) | — |
| 7 | `social.mjs` | `og:*` and `twitter:card=summary_large_image` on every page; `og:image` resolves, PNG header 1200×630, < 200 KB, content-hashed URL | — |
| 8 | `i18n.mjs` | `<html lang>`; hreflang pairs symmetric plus `x-default`; exactly one absolute canonical with a trailing slash | — |
| 9 | `headers.mjs` | served headers per path class against spec §6.2; the CSP hash equals the page's inline style hash | same, plus HSTS |
| 10 | `transport.mjs` | — | TLS 1.3-only (`openssl s_client -tls1_2` refused, `-tls1_3` accepted); HTTP/3 with an h3-capable curl (a pinned static build if the runner's lacks it; an `alt-svc` header alone is not proof); IPv6 as the AAAA record (GitHub-hosted runners have no IPv6 egress, so reachability over v6 is not measured from CI); 0-RTT reported as *configured off* (`infra/settings.tf`), a configuration claim, not a measurement |
| 12 | `observatory.mjs` | — | `@mdn/mdn-http-observatory` → A+ |
| 13 | `dns.mjs` | — | wraps `scripts/verify-edge.sh --ci` (dig and HTTP sections; no API section) — the M0 suite stays the one truth |
| 14 | `weight.mjs` | HTML+CSS brotli ≤ 30 KB; fonts ≤ 100 KB, one preload; Playwright first view: ≤ 6 requests, ≤ 150 KB brotli-equivalent; 0 bytes of executable script | — |
| 15 | `privacy.mjs` | no `Set-Cookie`; every request same-origin (Playwright request log); no `/cdn-cgi/`; every `<script>` is `ld+json` | same on production HTML |
| 16 | `wellknown.mjs` | `security.txt` RFC 9116 fields and `Expires` window; `robots.txt` with `Sitemap:`; `sitemap.xml` hreflang and lastmod; `llms.txt`; SVG, ICO and apple-touch favicons; `/404.html` served with 404 | — |
| 17 | `caching.mjs` | — | `Cache-Control`, `ETag`, `Content-Type` per class as row 17 lists |
| 21 | `fonts.mjs` | Playwright: layout-shift observer with `/fonts/*` delayed 600 ms → CLS 0; page height web vs fallback within 5 %; each paragraph within ±1 line; glyph coverage cited from `site check`, which already refuses a face without the Romanian glyphs and content outside the subset | — |

Rows 11, 18, 19 and 20 are external or weekly and are not the bench's. Row 5's value says what was checked ("strict parse, full text, absolute URLs, self link; JSON Feed schema-valid"); the W3C Feed Validator is a web service and becomes a manual row with its URL, run at launch. Row 3's foreign-language-fragment rule inside a piece is an editorial check in `/review-piece` (M2b); the bench verifies the masthead language link.

`merge.mjs` folds the Lighthouse shards into one row (pass = every shard passed; value = pages × form factors and the worst score) and adds a first row **Audited build** = the short commit hash, linking to the commit, so the colophon says which build the numbers describe. CI rows link to the Actions run, `link_text` "CI run"; post rows are named "… (production)".

**Local escape hatch:** `scripts/bench.sh [base]` — `npm ci` when needed; Chromium via `npx playwright install chromium`; Java ≥ 8 for Nu (prints the install line and marks the Nu row *skipped* when absent); serves `dist/` itself when no base is given; runs unsharded; prints the table; never publishes.

**Pins:** exact npm versions and a lockfile (Dependabot `npm` for `/bench`); Chromium pinned by the Playwright version and cached on the lockfile hash; Temurin 21 through `setup-java`; `.node-version` = 24 at the root.

**Expected first contact:** the first real Lighthouse and axe runs may raise audits the generator has never faced (`unsized-images`, `tap-targets`, `heading-order` on entry lists). That is the bench doing its job; fixing what it finds is inside M2a, planned as a task.

## 6. Scorecard data flow

- One schema, one merger: `bench/merge.mjs` takes any number of row files and writes `scorecard.json` — `Audited build` first, CI rows in spec order, then post rows. A row's `when` is the UTC date of the run that measured it. Manual rows stay in `data/scorecard-manual.yaml`; `site scorecard` appends them and marks *stale* (90 days) on **every** row.
- Two publishes per deploy, both through `scripts/scorecard-publish.sh`: CI rows before `wrangler deploy` (the launched colophon is never empty), CI + post rows after `post`. Post rows are absent for the minutes between a deploy and its `post` — they have not been measured against this build yet.
- KV: `scorecard` (fragment with `etag` metadata), `scorecard.json`. `scripts/kv.sh get|put` over the REST API; wrangler for `versions upload`, `deploy`, `rollback`.
- The publish step wraps the fragment in a minimal document and validates it with Nu; production's `/colophon/` is validated again by post row 2.
- Worker: `/colophon/scorecard.json` on every host with the `_headers` security set and `Cache-Control: public, max-age=300`; the same header set on the Worker's other generated responses (preview `robots.txt`, the mta-sts 404).
- Pre-launch, `main` publishes CI rows only. Pull-request runs never write KV.

## 7. Deploy, rollback, launch

- `launched:` — optional in `site.yaml` (`Config.Launched`), `YYYY-MM-DD` or empty; anything else fails `check`. `build` exposes it; `deploy` and `post` run only when it is set. Launch = one commit that fills it; the RUNBOOK's launch-day list starts there and continues with spec §11.
- `deploy`: verify `dist.sha256` → record the live version id → stash KV → publish CI rows → `wrangler deploy` → outputs. Failure after the publish restores the stash.
- `post` tier 1 rolls back with `wrangler rollback <old id>`; when Cloudflare refuses a rollback (bindings changed between versions; more than ten versions back) the job fails with the error in its summary and the RUNBOOK says to re-deploy the previous commit by hand. The placeholder is never the automatic fallback: it is worse than the previous build.
- A runner killed between the publish and `wrangler deploy` leaves the new rows on the old build; the `Audited build` row makes it visible and the RUNBOOK says to re-run the workflow. This is the window the M1b review accepted for `deploy.sh`.
- Manual fallbacks stay: `scripts/deploy.sh --yes-production` publishes a fallback row *"not audited by CI (manual deploy)"* — true by definition; `scripts/deploy-placeholder.sh` is the last resort.

## 8. Generator changes

1. `site serve --static [--host H] [--port P]`: serves `dist/` as built — no rebuild, no draft mode; `_headers` applied; gzip for text responses with `Vary: Accept-Encoding`; `/404.html` with a real 404; clean exit on SIGTERM. Row 14's sizes are computed by the bench with brotli from the bodies; gzip on the server is for Lighthouse's and Playwright's view.
2. `Config.Launched` with validation.
3. `site scorecard`: stale on every row.
4. `check` and `check --dist`: unchanged. They remain the fast local gate; the bench measures.

## 9. Hygiene

- Root `package.json` + lockfile with `wrangler` exact, `private: true`, `engines.node >= 24`; `.node-version`; scripts call `npx wrangler` from `node_modules/.bin`; `env.sh` loses the `WRANGLER` variable; `setup.sh` runs `npm ci` at the root and in `bench/`.
- `env.sh` in CI: when `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` are already set it exports what it must and returns; the nine-variable check applies only to the operator's file. Infra scripts still require the file.
- Go tools as `tool` directives in `go.mod`: `staticcheck` and `actionlint`, run as `go tool …`, pinned in `go.sum`, bumped by Dependabot. The pre-commit hook runs `actionlint` when `.github/` changes. The colophon's dependency count comes from the built binary's build info, which never links tools; the plan asserts the count is unchanged.
- Toolchain: `setup-go` with `go-version-file: go.mod` and `GOTOOLCHAIN=local` — the RUNBOOK's `GOTOOLCHAIN=go1.27.1` intent (no silent download) with one source of truth; RUNBOOK updated.
- Full-depth checkout. On pull requests the checkout is GitHub's synthetic merge commit, whose time is "now", so PR builds differ between runs; irrelevant for previews, and the build-twice check is within one run. `main` builds use the real commit time.
- `scripts/dist-hash.sh` produces the sorted hash list locally and in CI; the first green run's list is compared once against a dgx build of the same commit (RUNBOOK).
- `.github/dependabot.yml`: gomod, github-actions, npm `/`, npm `/bench`; weekly; minor and patch grouped.
- `.gitignore`: `/node_modules/`, `/bench/.cache/`.

## 10. Testing

- Go: `serve --static` (no rebuild after a source edit; gzip negotiated; 404 status; headers present), `Launched` validation, stale on every row.
- Bench: `node --test` over fixtures — every custom check has a passing fixture and a failing fixture per rule. Lighthouse, axe, Nu and the Playwright font test are exercised by integration.
- Workflows: `actionlint` in the hook and in `build`; `act` tried locally for `build` and `checks` as evidence only.
- End to end without a push: `scripts/bench.sh` against `site serve --static` on the real `dist/` → every CI row green → `site scorecard` → Nu-valid fragment. The first Actions run, the preview comment and the amd64 hash list need a push.

## 11. Docs

- RUNBOOK: "CI" section (jobs and gates; the two secrets; reading a red row; `workflow_dispatch` to re-verify; manual rollback; "a manual deploy is unaudited"; launch day); Generator and Scorecard sections updated; every "M2" mention resolved to done or M2b.
- README: CI badge; "designed, next milestone" → "runs on every pull request".
- ADR-0015 — *CI is the deployer; the launch is a commit; post-deploy has two tiers*; spec §12 gains seed 15.
- Spec amendments, one commit, revertable: §8 step 5 "deployed in about a minute" → "live a few minutes after the merge, once the audit is green"; §7's tool list places lychee in the weekly job; row 10's enforcement notes 0-RTT as configured, not measured; §13 lists `ci.yml, audit.yml` for M2a and `verify.yml, codeql.yml` for M2b.
- The three stale doc comments (`worker/index.js`, RUNBOOK, `scripts/fontface/main.go`) fixed.

## 12. Decisions for Radu (made under delegation; each revertable)

1. `GOTOOLCHAIN=local` with `go-version-file` instead of the literal `go1.27.1` (§9).
2. Row 5's W3C Feed Validator becomes a manual row, run at launch (§5).
3. 0-RTT reported as configuration, not measurement (§5); zone-read on the CI token declined by Radu on 2026-09-19.
4. lychee leaves M2a; the CI external-link check is the bench's own fetch, warn-only (§3.6).
5. Post rows are absent between a deploy and its `post`, rather than carried over from the previous build (§6).
6. `scripts/kv.sh` over REST replaces wrangler's `kv` subcommands in the scripts (§3.7).
7. `staticcheck` and `actionlint` as `tool` directives in `go.mod` (§9).
8. The PR comment uses the runner's `gh`, not a marketplace action (§4).
9. Spec §8 step 5 reworded; §13 workflow list reworded (§11).
10. JSON Feed 1.1 checked field by field, not against a third-party JSON Schema (§5).
11. IPv6 measured as the AAAA record only; runners cannot connect over v6 (§5).

## 13. Waits for Radu

- Create the CI token (this Worker, its KV namespace, Analytics read) and add `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` as repository secrets; until then `preview` and `deploy` skip with a summary line.
- Push the branch for the first Actions run; then the amd64 hash comparison.
- HSTS preload submission, earliest 2026-09-25 (M0 plan Task 9 step 4) — independent of M2a.
