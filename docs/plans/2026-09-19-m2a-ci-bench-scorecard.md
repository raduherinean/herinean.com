# M2a — CI, Bench, Scorecard Pipeline, Deploy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every pull request is audited against the scorecard by GitHub Actions from one Node bench; a merge to `main` deploys the audited build once the site is launched, verifies it on production, rolls back a broken deploy, measures the post-deploy rows and writes the whole scorecard to KV for the colophon.

**Architecture:** `bench/` is one Node entry point (`audit.mjs`) with a module per scorecard row; every module receives the served pages once and returns rows in the JSON shape `site scorecard` already reads; `merge.mjs` folds Lighthouse shards and post rows into `scorecard.json`. `ci.yml` runs build → audit (a reusable `audit.yml` with a Lighthouse matrix) → preview on pull requests → deploy/post/publish on `main` when `site.yaml` carries `launched:`. The generator gains `serve --static` (serve `dist/` as built, gzip) so the audit sees the bytes that deploy; KV is read and written by `scripts/kv.sh` over the REST API; wrangler is pinned by a root lockfile.

**Tech Stack:** Go 1.27 (existing module; `tool` directives for staticcheck and actionlint); Node 24 (`bench/` with lighthouse, playwright, @axe-core/playwright, html-validate, vnu-jar, cheerio, fast-xml-parser; root `package.json` for wrangler); GitHub Actions (`actions/*` only, SHA-pinned); Cloudflare Workers versions API via wrangler; KV via REST.

**Spec:** `docs/specs/2026-09-19-m2a-ci-bench-scorecard-design.md` (this plan implements it; the parent is `docs/specs/2026-09-17-herinean-com-design.md`, §3, §6.2, §7).

## Global Constraints

- Public repository: every file, commit message and comment is read by strangers. No private hosts, no session links, no `/home/...` paths. Commit messages: `M2a: <component> — <what>`; a WHY body when the decision is not obvious from the diff; the trailer `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>` exactly (the repository's line overrides any harness reminder); **no `Claude-Session:` trailer**. Commits are SSH-signed by the repository's git config; never push.
- Every Go invocation uses `-tags nodynamic` (`go build`, `go test`, `go vet`, `go tool staticcheck`).
- Build the binary (`go build -tags nodynamic -o .cache/site ./cmd/site`) and run it; never `go run` (it hides exit 3).
- `gofmt` clean; `go vet` clean; `go tool staticcheck ./...` clean; `go test -tags nodynamic ./...` green before every commit.
- `dist/` invariants: `check --dist` exits 0; two builds byte-identical.
- The bench never writes KV; only `scripts/scorecard-publish.sh` does, and during this plan it is run only against a scratch key or not at all. Production (`herinean.com`) is the M0 placeholder and is never deployed to by this plan. `wrangler versions upload` (preview) is allowed.
- The spec's row numbers and thresholds (spec §3) are the definition of done for every check; where a check cannot measure a threshold, the row's `value` says what was measured — never claim more than was measured.
- Node modules: exact versions in `package.json` (no `^`/`~`), lockfile committed, `npm ci`.
- Exit codes: `site check` `3` = author inputs missing (allowed by the hook, a failure in CI); `1` = a real problem.
- The scorecard row JSON shape (unchanged, `internal/site/scorecard.go` `ciRow`): `{"check","pass","value","when","link","link_text"}`; `when` is `YYYY-MM-DD`.

## Working conventions for this plan

- Work in the worktree on branch `m2a/ci-bench` (created from `main` @ `f6ddfd8`).
- Before the first code change, `scripts/setup.sh` has run (hooks path set). Node 24 is `~/.local/bin/node`; `java` 1.8 exists on the build machine (enough for Nu); Playwright Chromium builds are cached under `~/.cache/ms-playwright`.
- Executors record every departure from a step, with the reason, in the ledger the controller keeps; do not silently change a threshold.
- The M0 operator file `~/.config/herinean/m0.env` exists on the build machine; `scripts/env.sh` reads it. CI does not have it (Task 0 makes the scripts work from two variables).

---

## File structure

**Create**
- `.node-version` — `24`
- `package.json`, `package-lock.json` — wrangler pinned (root)
- `bench/package.json`, `bench/package-lock.json`, `bench/README.md`, `bench/.htmlvalidate.json`, `bench/links-allow.txt`, `bench/tools.json`
- `bench/audit.mjs` — CLI entry; `bench/merge.mjs` — shard/row merger and gate
- `bench/lib/pages.mjs` (page set), `bench/lib/row.mjs` (row helpers), `bench/lib/browser.mjs` (Playwright launcher), `bench/lib/tools.mjs` (pinned binaries: h3 curl), `bench/lib/sh.mjs` (spawn helper)
- `bench/lib/i18n.mjs`, `social.mjs`, `wellknown.mjs`, `feeds.mjs`, `jsonld.mjs`, `headers.mjs`, `privacy.mjs`, `weight.mjs`, `links.mjs`, `html.mjs`, `a11y.mjs`, `lighthouse.mjs`, `fonts.mjs`, `transport.mjs`, `observatory.mjs`, `dns.mjs`, `caching.mjs`
- `bench/fixtures/**` and `bench/test/*.test.mjs`
- `scripts/kv.sh`, `scripts/dist-hash.sh`, `scripts/bench.sh`, `scripts/pr-comment.sh`
- `.github/workflows/ci.yml`, `.github/workflows/audit.yml`, `.github/dependabot.yml`
- `docs/adr/0015-ci-deploys-launch-is-a-commit.md`

**Modify**
- `go.mod`, `go.sum` — tool directives
- `.gitignore`, `.githooks/pre-commit`, `scripts/setup.sh`, `scripts/env.sh`, `scripts/preview.sh`, `scripts/deploy.sh`, `scripts/deploy-placeholder.sh`, `scripts/scorecard-publish.sh`, `scripts/verify-edge.sh`
- `internal/site/site.go` (Options.Static), `internal/site/serve.go`, `internal/site/serve_test.go`, `cmd/site/main.go`
- `internal/config/config.go`, `internal/config/config_test.go` (Launched)
- `internal/site/scorecard.go`, `internal/site/scorecard_test.go` (stale on every row)
- `worker/index.js`, `worker/index.test.mjs` (security headers on generated responses; `scorecard.json` on every host)
- `RUNBOOK.md`, `README.md`, `docs/specs/2026-09-17-herinean-com-design.md` (amendments), `scripts/fontface/main.go` (comment)

---

### Task 0: Toolchain pins — wrangler by lockfile, Go tools by `go.mod`, scripts that run in CI

**Files:**
- Create: `package.json`, `package-lock.json`, `.node-version`
- Modify: `go.mod`, `go.sum`, `.gitignore`, `.githooks/pre-commit`, `scripts/setup.sh`, `scripts/env.sh`, `scripts/preview.sh`, `scripts/deploy.sh`, `scripts/deploy-placeholder.sh`, `scripts/scorecard-publish.sh`

**Interfaces:**
- Produces: `npx wrangler` resolves to `node_modules/.bin/wrangler` (exact version from the lockfile); `go tool staticcheck`, `go tool actionlint`; `. scripts/env.sh` succeeds with only `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` exported (CI mode) and still loads the operator file otherwise.

- [ ] **Step 1: Root `package.json` with wrangler pinned**

```json
{
  "name": "herinean-com",
  "private": true,
  "description": "Deploy tooling for herinean.com; the site itself has no JavaScript.",
  "engines": { "node": ">=24" },
  "devDependencies": {
    "wrangler": "4.135.0"
  }
}
```

Then `npm install --package-lock-only --ignore-scripts` (creates `package-lock.json`), `npm ci`, and `npx wrangler --version` prints `4.135.0`. Write `.node-version` containing `24`.

- [ ] **Step 2: `.gitignore`** — add under "build output and caches":

```
/node_modules/
/bench/.cache/
```

(`/bench/node_modules/` is already there.)

- [ ] **Step 3: Go tools as `tool` directives**

```bash
go get -tool honnef.co/go/tools/cmd/staticcheck@2025.1.1
go get -tool github.com/rhysd/actionlint/cmd/actionlint@v1.7.7
go mod tidy
go tool staticcheck -tags nodynamic ./...
go tool actionlint -version
```

Use the newest release of each at execution time if newer than the versions above; record the versions in the commit body. `go.mod` gains two `tool` lines and their requires. Verify the colophon's dependency count is unchanged: build the fixture (`go test -tags nodynamic ./internal/site/ -run TestBuild`) — the golden comparison fails if the list changed; it must pass unchanged.

- [ ] **Step 4: `scripts/env.sh` — CI mode**

Replace the file with:

```bash
#!/usr/bin/env bash
# Source this: `. scripts/env.sh`. Loads operator inputs from outside the repo — or, in CI, takes the two
# variables the workflow exports and asks for nothing else. The infra scripts need the zone ids and refuse to run without the file.
set -euo pipefail
if [ -n "${CLOUDFLARE_API_TOKEN:-}" ] && [ -n "${CLOUDFLARE_ACCOUNT_ID:-}" ] && [ -z "${HERINEAN_ENV_FILE:-}" ]; then
  export TF_VAR_account_id="$CLOUDFLARE_ACCOUNT_ID"
  return 0 2>/dev/null || exit 0
fi
ENV_FILE="${HERINEAN_ENV_FILE:-${HOME}/.config/herinean/m0.env}"
if [ ! -r "$ENV_FILE" ]; then
  echo "missing $ENV_FILE — see docs/plans/2026-09-17-m0-edge-foundation.md 'Operator inputs'" >&2
  return 1 2>/dev/null || exit 1
fi
# shellcheck disable=SC1090
. "$ENV_FILE"
for v in CLOUDFLARE_API_TOKEN CLOUDFLARE_ACCOUNT_ID ZONE_COM ZONE_RO ZONE_NET ZONE_INFO GITHUB_OWNER GITEA_REMOTE DKIM_TXT; do
  [ -n "${!v:-}" ] || { echo "$v is empty in $ENV_FILE" >&2; return 1 2>/dev/null || exit 1; }
done
export CLOUDFLARE_API_TOKEN CLOUDFLARE_ACCOUNT_ID
export TF_VAR_account_id="$CLOUDFLARE_ACCOUNT_ID"
```

Note the `export` of the two Cloudflare variables: wrangler and curl in child processes need them (the M1b session found `npx wrangler kv` returning `[]` because the file's variables were not exported).

- [ ] **Step 5: Scripts use `npx wrangler`** — in `scripts/preview.sh`, `scripts/deploy.sh`, `scripts/deploy-placeholder.sh`, `scripts/scorecard-publish.sh` replace every `npx --yes "$WRANGLER"` with `npx wrangler`, and add after `cd "$(dirname "$0")/.."` in each: `[ -d node_modules/wrangler ] || npm ci --silent`. Remove the `WRANGLER=` comment and export from nowhere else (env.sh no longer sets it). Grep: `grep -rn 'WRANGLER' scripts/ RUNBOOK.md` → only RUNBOOK mentions remain (Task 14 rewrites them).

- [ ] **Step 6: `scripts/setup.sh`** — after the tool loop add:

```bash
for t in java; do command -v "$t" >/dev/null || echo "missing: $t (Nu checker needs a JRE ≥ 8: sudo apt install -y default-jre-headless)"; done
[ -d node_modules/wrangler ] || npm ci --silent
[ -d bench/node_modules ] || (cd bench && [ -f package.json ] && npm ci --silent) || true
```

- [ ] **Step 7: Pre-commit hook runs actionlint on workflow changes** — append to `.githooks/pre-commit` before the content block:

```bash
if git diff --cached --name-only | grep -q '^\.github/'; then
  go tool actionlint
fi
```

and change the vet line to also run staticcheck: after `go vet -tags nodynamic ./...` add `go tool staticcheck -tags nodynamic ./...`.

- [ ] **Step 8: Verify** — `. scripts/env.sh` from a shell with the operator file → exit 0; `env -i HOME=$HOME PATH=$PATH bash -c 'CLOUDFLARE_API_TOKEN=x CLOUDFLARE_ACCOUNT_ID=y . scripts/env.sh && echo ok'` → `ok`; `env -i HOME=/nonexistent PATH=$PATH bash -c '. scripts/env.sh'` → "missing …" exit 1. `npx wrangler --version` → 4.135.0. `go tool staticcheck -tags nodynamic ./...` clean. `go test -tags nodynamic ./...` green.

- [ ] **Step 9: Commit** — `M2a: toolchain — wrangler by lockfile, Go tools by go.mod, scripts run from two variables` with a WHY body (the M1b review's supply-chain finding: a shell variable is not a pin Dependabot can bump; CI has no operator file).

---

### Task 1: `site serve --static` — serve `dist/` as built, gzip, real 404, clean shutdown

**Files:**
- Modify: `internal/site/site.go` (Options), `internal/site/serve.go`, `cmd/site/main.go`
- Test: `internal/site/serve_test.go`

**Interfaces:**
- Consumes: `Options{Root, Out, Draft}`; `Serve(o Options, host string, port int) error`.
- Produces: `Options.Static bool` — when true, `Serve` never rebuilds, never enables draft mode, reads `_headers` once, gzips `text/*`, `application/json`, `application/xml`, `application/rss+xml`, `application/feed+json`, `image/svg+xml` when the client accepts it, and returns on SIGINT/SIGTERM. CLI: `site serve --static [--host H] [--port P]`.

- [ ] **Step 1: Failing tests** — append to `internal/site/serve_test.go`:

```go
// --static serves dist/ as built: a source edit after the build must not change a response (CI audits the
// artifact that deploys), the 404 page carries a 404 status, and text is gzipped when the client accepts it.
func TestServeStaticServesBuiltDistOnly(t *testing.T) {
	root := fixtureRoot(t)
	o := Options{Root: root, Out: "dist", Static: true}
	if err := Build(Options{Root: root, Out: "dist"}); err != nil {
		t.Fatal(err)
	}
	addr := startServe(t, o)
	get := func(path string, gzip bool) (*http.Response, string) {
		req, _ := http.NewRequest("GET", "http://"+addr+path, nil)
		if gzip {
			req.Header.Set("Accept-Encoding", "gzip")
		}
		tr := &http.Transport{DisableCompression: true}
		resp, err := (&http.Client{Transport: tr}).Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return resp, string(b)
	}
	resp, body := get("/", false)
	if resp.StatusCode != 200 || !strings.Contains(body, "<html") {
		t.Fatalf("/: %d %q", resp.StatusCode, body[:min(len(body), 80)])
	}
	if resp.Header.Get("Content-Security-Policy") == "" {
		t.Error("/ lacks the _headers CSP")
	}
	// edit a source: the static server must not notice
	home := filepath.Join(root, "content", "en", "_home.md")
	orig, _ := os.ReadFile(home)
	if err := os.WriteFile(home, append(orig, []byte("\n\nEDITED AFTER BUILD\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, body2 := get("/", false); body2 != body {
		t.Error("static serve rebuilt after a source edit")
	}
	resp, _ = get("/nope/", false)
	if resp.StatusCode != 404 {
		t.Errorf("/nope/: status %d, want 404", resp.StatusCode)
	}
	resp, _ = get("/404.html", false)
	if resp.StatusCode != 404 {
		t.Errorf("/404.html: status %d, want 404 (the page is the 404 page)", resp.StatusCode)
	}
	resp, gz := get("/", true)
	if resp.Header.Get("Content-Encoding") != "gzip" || resp.Header.Get("Vary") != "Accept-Encoding" {
		t.Errorf("gzip not negotiated: %v", resp.Header)
	}
	if len(gz) >= len(body) {
		t.Errorf("gzipped body (%d) not smaller than plain (%d)", len(gz), len(body))
	}
	resp, _ = get("/favicon.svg", true)
	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Error("svg not gzipped")
	}
	if resp, _ := get("/apple-touch-icon.png", true); resp.Header.Get("Content-Encoding") != "" {
		t.Error("png must not be gzipped")
	}
}
```

Run: `go test -tags nodynamic ./internal/site/ -run TestServeStatic -v` → FAIL: `unknown field Static`.

- [ ] **Step 2: `Options.Static`** — in `internal/site/site.go`:

```go
type Options struct {
	Root   string // repository root
	Out    string // output directory (dist)
	Draft  bool   // serve only: render pieces whose title/date/pillar/summary are still blank, with visible defaults
	Static bool   // serve only: serve dist/ as built — no rebuild, no draft mode; what CI audits is what deploys
}
```

- [ ] **Step 3: `serve.go`** — restructure `Serve`:

```go
func Serve(o Options, host string, port int) error {
	if !o.Static {
		o.Draft = true // the preview is where a piece gets written (spec §8): blanks render with defaults, not as a 500
	}
	dist := filepath.Join(o.Root, o.Out)
	var mu sync.Mutex
	var built time.Time
	serve404 := func(w http.ResponseWriter, r *http.Request) {
		nf, _ := os.ReadFile(filepath.Join(dist, "404.html"))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		writeBody(w, r, 404, nf)
	}
	rebuild := func() error {
		if o.Static {
			if built.IsZero() {
				if st, err := os.Stat(filepath.Join(dist, "index.html")); err != nil {
					return fmt.Errorf("--static: %s has no index.html (run site build first)", dist)
				} else {
					built = st.ModTime()
				}
			}
			return nil
		}
		start := time.Now()
		if built.IsZero() || newestInput(o.Root).After(built) {
			if err := Build(o); err != nil {
				return err
			}
			built = start
		}
		return nil
	}
	// ... (existing initial rebuild, handler) ...
```

In the handler: replace `serve404(w)` calls with `serve404(w, r)`; `/404.html` requested directly is the 404 page: after computing `fp`, add

```go
		if p == "/404.html" {
			serve404(w, r)
			return
		}
```

Replace the final `http.ServeContent(...)` with:

```go
		ct := mime.TypeByExtension(filepath.Ext(fp))
		if strings.HasSuffix(fp, ".html") {
			ct = "text/html; charset=utf-8"
		}
		if ct != "" && w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", ct)
		}
		writeBody(w, r, 200, b)
```

and add the helper (ETag/Last-Modified are not needed by the audit; CI asserts caching on production, row 17):

```go
// compressible lists the types the edge compresses; the audit's view of weight must be the edge's, not raw bytes.
func compressible(ct string) bool {
	for _, p := range []string{"text/", "application/json", "application/xml", "application/rss+xml", "application/feed+json", "image/svg+xml"} {
		if strings.HasPrefix(ct, p) {
			return true
		}
	}
	return false
}

func writeBody(w http.ResponseWriter, r *http.Request, status int, b []byte) {
	ct := w.Header().Get("Content-Type")
	if compressible(ct) && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		w.WriteHeader(status)
		gz := gzip.NewWriter(w)
		_, _ = gz.Write(b)
		_ = gz.Close()
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	w.WriteHeader(status)
	_, _ = w.Write(b)
}
```

Imports: `compress/gzip`, `mime`, `strconv`, `context`, `os/signal`, `syscall`. Replace `http.ListenAndServe` with a server that stops on a signal:

```go
	srv := &http.Server{Addr: addr, Handler: http.HandlerFunc(h)}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() { <-ctx.Done(); _ = srv.Shutdown(context.Background()) }()
	mode := "rebuilds when inputs change"
	if o.Static {
		mode = "static: serves " + dist + " as built"
	}
	fmt.Fprintf(os.Stderr, "site serve: http://%s/ (%s)\n", addr, mode)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
```

Keep the `_headers` read per request in dynamic mode; in static mode read it once before the handler (`rules := parseHeaders(hb)` outside `h` when `o.Static`).

- [ ] **Step 4: CLI** — `cmd/site/main.go` `serve` case: `static := fs.Bool("static", false, "serve dist/ as built: no rebuild, no draft mode (what CI audits)")` and pass `site.Options{Root: ".", Out: "dist", Static: *static}`; usage line `site serve [--static] [--host H] [--port P]`.

- [ ] **Step 5: Run** — `go test -tags nodynamic ./internal/site/ -v -run 'TestServe'` → all PASS (the existing four too). `gofmt -l .` empty. Build the binary, `./.cache/site build && ./.cache/site serve --static --port 8090 &`, `curl -sI -H 'Accept-Encoding: gzip' localhost:8090/ | grep -i 'content-encoding\|content-security'`, `kill %1` → exits 0 promptly.

- [ ] **Step 6: Commit** — `M2a: serve — --static serves dist/ as built, gzipped, with a real 404` (WHY: CI must audit the artifact that deploys, not a rebuild; sizes must be the edge's).

---

### Task 2: `launched:` in `site.yaml`; stale on every scorecard row

**Files:**
- Modify: `internal/config/config.go`, `internal/site/scorecard.go`
- Test: `internal/config/config_test.go`, `internal/site/scorecard_test.go`

**Interfaces:**
- Produces: `Config.Launched string` (`yaml:"launched"`, empty or `YYYY-MM-DD`; anything else is a load error); `scorecardRows` marks `Stale` on CI rows whose `when` is older than 90 days.

- [ ] **Step 1: Failing tests**

Append to `internal/config/config_test.go`:

```go
func TestLaunchedIsEmptyOrADate(t *testing.T) {
	if c, err := Load(write(t, sample)); err != nil || c.Launched != "" {
		t.Fatalf("no launched field: %v %q", err, c.Launched)
	}
	if c, err := Load(write(t, sample+"launched: 2026-10-01\n")); err != nil || c.Launched != "2026-10-01" {
		t.Fatalf("launched date: %v %q", err, c.Launched)
	}
	if _, err := Load(write(t, sample+"launched: soon\n")); err == nil || !strings.Contains(err.Error(), "launched") {
		t.Fatalf("a non-date launched must fail: %v", err)
	}
}
```

In `internal/site/scorecard_test.go` add to `TestScorecardRows` after the first assertions:

```go
	// a CI row measured more than 90 days ago is stale like a manual one; the colophon must say so
	old := filepath.Join(dir, "old.json")
	_ = os.WriteFile(old, []byte(`[{"check":"Lighthouse","pass":true,"value":"100","when":"2026-01-01"}]`), 0o644)
	rows, err = scorecardRows(old, man, time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC))
	if err != nil || !rows[0].Stale {
		t.Errorf("old CI row must be stale: %v %+v", err, rows)
	}
```

Run both → FAIL.

- [ ] **Step 2: Implement** — `config.go`: add `Launched string \`yaml:"launched"\` // YYYY-MM-DD once production serves the site (spec §11); empty before launch. CI deploys only when set.` to `Config`; in `validate()` before the placeholder scan:

```go
	if c.Launched != "" {
		if _, err := time.Parse("2006-01-02", c.Launched); err != nil {
			return fmt.Errorf("launched must be YYYY-MM-DD or empty, got %q", c.Launched)
		}
	}
```

(import `time`). `scorecard.go`: in the CI loop compute `stale := false; if t, err := time.Parse("2006-01-02", r.When); err == nil { stale = now.Sub(t) > staleAfter }` and set `Stale: stale` on the row.

- [ ] **Step 3: Run** — `go test -tags nodynamic ./internal/config/ ./internal/site/` → PASS.

- [ ] **Step 4: Commit** — `M2a: config — launched: marks the day production serves the site; stale applies to every scorecard row`.

---

### Task 3: Worker — the security header set on generated responses; `scorecard.json` on every host

**Files:**
- Modify: `worker/index.js`
- Test: `worker/index.test.mjs`

**Interfaces:**
- Produces: every `new Response(...)` the Worker builds itself (preview `robots.txt`, mta-sts 404, `scorecard.json` and its 404) carries the `/*` security headers of `_headers` (mirrored as a constant; the CSP is `default-src 'none'` — these responses have no style); `/colophon/scorecard.json` is answered from KV on every host.

- [ ] **Step 1: Failing tests** — append to `worker/index.test.mjs`:

```js
const SECURITY = ["content-security-policy", "strict-transport-security", "x-content-type-options", "referrer-policy", "permissions-policy", "cross-origin-opener-policy", "cross-origin-resource-policy", "x-frame-options"];

test("responses the Worker builds itself carry the security header set", async () => {
  const kv = { getWithMetadata: async () => ({ value: null }), get: async (k) => (k === "scorecard.json" ? '[{"check":"x"}]' : null) };
  const h = harness({ kv });
  for (const [url, status] of [["https://preview.example.workers.dev/robots.txt", 200], ["https://mta-sts.herinean.com/", 404], ["https://herinean.com/colophon/scorecard.json", 200], ["https://preview.example.workers.dev/colophon/scorecard.json", 200]]) {
    const res = await worker.fetch(req(url), h.env, h.ctx);
    assert.equal(res.status, status, url);
    for (const name of SECURITY) assert.ok(res.headers.get(name), `${url} lacks ${name}`);
    assert.equal(res.headers.get("content-security-policy"), "default-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'", url);
  }
  const json = await worker.fetch(req("https://preview.example.workers.dev/colophon/scorecard.json"), h.env, h.ctx);
  assert.equal(json.headers.get("content-type"), "application/json; charset=utf-8");
  assert.equal(json.headers.get("cache-control"), "public, max-age=300");
  assert.equal(json.headers.get("x-robots-tag"), "noindex, nofollow");
});

test("scorecard.json without a KV value is a 404 with the header set", async () => {
  const h = harness({ kv: { getWithMetadata: async () => ({ value: null }), get: async () => null } });
  const res = await worker.fetch(req("https://herinean.com/colophon/scorecard.json"), h.env, h.ctx);
  assert.equal(res.status, 404);
  assert.ok(res.headers.get("strict-transport-security"));
});
```

Run `cd worker && node --test` → FAIL.

- [ ] **Step 2: Implement** — in `worker/index.js` add near the top:

```js
// The asset layer applies dist/_headers to what it serves; responses built here must carry the same set (spec §6.2, row 9).
// Mirrors the /* block written by internal/edge — keep the two in step (verify-preview checks both on the live preview).
const SECURITY_HEADERS = {
  "content-security-policy": "default-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'",
  "strict-transport-security": "max-age=63072000; includeSubDomains; preload",
  "x-content-type-options": "nosniff",
  "referrer-policy": "strict-origin-when-cross-origin",
  "permissions-policy": "accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()",
  "cross-origin-opener-policy": "same-origin",
  "cross-origin-resource-policy": "same-origin",
  "x-frame-options": "DENY",
};
const own = (body, status, headers) => new Response(body, { status, headers: { ...SECURITY_HEADERS, ...headers } });
```

Replace the three `new Response(...)` sites (mta-sts 404, preview robots.txt, scorecardJSON's two) with `own(...)`. Move the `scorecard.json` route before the host switch so previews get it too (the preview branch then adds `x-robots-tag` to it like any other response — restructure so the preview branch wraps the result of a shared `route(request, url, env)` helper):

```js
      if (url.pathname === "/colophon/scorecard.json") {
        const res = await scorecardJSON(env);
        if (url.host === prodHost) return res;
        const headers = new Headers(res.headers); headers.set("x-robots-tag", "noindex, nofollow");
        return new Response(res.body, { status: res.status, headers });
      }
```

placed before `if (url.host !== prodHost)`. In `scorecardJSON` drop `access-control-allow-origin: *` (CORP is same-origin on every generated response; the JSON is for readers of the colophon, not for other sites) and keep `cache-control: public, max-age=300`.

- [ ] **Step 3: Run** — `cd worker && node --test` → all pass (existing 16 + 2). Update the file's header comment (line 2) and the stale line-65 comment: `_headers` now types robots.txt; the Worker still adds the charset to bare `text/html`/`text/plain` from the asset layer.

- [ ] **Step 4: Commit** — `M2a: worker — generated responses carry the security header set; scorecard.json on every host` (WHY: row 9 is measured on production URLs the Worker answers itself; the preview 404 was on the M1b after-merge list).

---

### Task 4: Shell — `kv.sh`, `dist-hash.sh`, `scorecard-publish.sh` on REST, `deploy.sh`'s honest fallback row, `verify-edge.sh --ci`

**Files:**
- Create: `scripts/kv.sh`, `scripts/dist-hash.sh`
- Modify: `scripts/scorecard-publish.sh`, `scripts/deploy.sh`, `scripts/verify-edge.sh`

**Interfaces:**
- Produces: `scripts/kv.sh get <key> [--metadata]` (prints the value; with `--metadata` prints the metadata JSON instead), `scripts/kv.sh put <key> <file> [--metadata '<json>']`, `scripts/kv.sh delete <key>`; namespace id read from `wrangler.toml`. `scripts/dist-hash.sh [dist]` prints `sha256  path` lines sorted by path. `scripts/scorecard-publish.sh [scorecard.json]` unchanged contract, REST underneath. `scripts/verify-edge.sh --ci` skips the API section and accepts the site's CSP.

- [ ] **Step 1: `scripts/kv.sh`**

```bash
#!/usr/bin/env bash
# KV over the REST API: get/put/delete one key in the SCORECARD namespace (id from wrangler.toml).
# REST rather than `wrangler kv`: it returns metadata (the rollback stash needs it) and needs only the token.
# usage: kv.sh get KEY [--metadata] | kv.sh put KEY FILE [--metadata JSON] | kv.sh delete KEY
set -euo pipefail
cd "$(dirname "$0")/.."
. scripts/env.sh
NS=$(sed -n 's/^id = "\([0-9a-f]*\)"/\1/p' wrangler.toml | head -1)
[ -n "$NS" ] || { echo "kv.sh: no KV namespace id in wrangler.toml" >&2; exit 1; }
API="https://api.cloudflare.com/client/v4/accounts/$CLOUDFLARE_ACCOUNT_ID/storage/kv/namespaces/$NS"
auth=(-H "Authorization: Bearer $CLOUDFLARE_API_TOKEN")
cmd=${1:?usage}; key=${2:?key}; shift 2
case "$cmd" in
  get)
    if [ "${1:-}" = "--metadata" ]; then
      curl -sSf "${auth[@]}" "$API/metadata/$key" | jq -c '.result'
    else
      curl -sSf "${auth[@]}" "$API/values/$key"
    fi ;;
  put)
    file=${1:?file}; shift
    meta='{}'; [ "${1:-}" = "--metadata" ] && meta=${2:?json}
    curl -sSf "${auth[@]}" -X PUT "$API/values/$key" -F "value=@$file" -F "metadata=$meta" | jq -e '.success' >/dev/null ;;
  delete)
    curl -sSf "${auth[@]}" -X DELETE "$API/values/$key" | jq -e '.success' >/dev/null ;;
  *) echo "kv.sh: unknown command $cmd" >&2; exit 2 ;;
esac
```

Verify against the live namespace with a scratch key only: `printf 'hello' > /tmp/x; scripts/kv.sh put ci-scratch /tmp/x --metadata '{"etag":"abc"}'; scripts/kv.sh get ci-scratch` → `hello`; `scripts/kv.sh get ci-scratch --metadata` → `{"etag":"abc"}`; `scripts/kv.sh delete ci-scratch`; `scripts/kv.sh get ci-scratch` → curl 404 (exit 22). Never touch `scorecard` or `scorecard.json` in this task. Check that the existing `scorecard` key's metadata reads back: `scripts/kv.sh get scorecard --metadata` → `{"etag":"…"}`.

- [ ] **Step 2: `scripts/dist-hash.sh`**

```bash
#!/usr/bin/env bash
# Sorted sha256 list of a built dist/: the reproducibility check in CI (built twice, compared) and the one-off arm64/amd64 comparison (RUNBOOK).
set -euo pipefail
d=${1:-dist}
(cd "$d" && find . -type f | LC_ALL=C sort | xargs sha256sum)
```

Verify: build twice, `scripts/dist-hash.sh > a; rm -rf dist; ./.cache/site build; scripts/dist-hash.sh > b; diff a b` → empty.

- [ ] **Step 3: `scripts/scorecard-publish.sh` on REST** — first, the binary: CI's `deploy`/`publish` jobs have the `site` artifact and no Go toolchain, so replace the `go build …` line with

```bash
# CI hands the job the built binary; locally we build it.
if [ -x ./site ]; then SITE=./site; else go build -tags nodynamic -o .cache/site ./cmd/site; SITE=.cache/site; fi
```

and use `"$SITE" scorecard …` below. Then replace the two `npx … kv key put` lines with:

```bash
scripts/kv.sh put scorecard .cache/scorecard.html --metadata "{\"etag\":\"$(sha256sum .cache/scorecard.html | cut -c1-8)\"}"
if [ -f "$IN" ]; then scripts/kv.sh put scorecard.json "$IN"; fi
```

Also add Nu validation of the fragment when Java is present (the design's "Nu-validated"; CI always has Java, the build machine has 1.8):

```bash
if command -v java >/dev/null && [ -f bench/node_modules/vnu-jar/build/dist/vnu.jar ]; then
  { printf '<!doctype html><html lang="en"><head><title>scorecard</title></head><body>'; cat .cache/scorecard.html; printf '</body></html>'; } > .cache/scorecard-doc.html
  java -jar bench/node_modules/vnu-jar/build/dist/vnu.jar --errors-only --exit-zero-always .cache/scorecard-doc.html 2>&1 | tee .cache/scorecard-nu.txt
  ! grep -q 'error:' .cache/scorecard-nu.txt || { echo "scorecard fragment is not valid HTML" >&2; exit 1; }
else
  echo "scorecard-publish: Nu not available here; the fragment is validated in CI" >&2
fi
```

(The jar path is confirmed in Task 8 when `vnu-jar` is installed; adjust if the package lays it out differently and record the ruling.) Do **not** run the script end to end in this task — it publishes the live key; the RUNBOOK's manual path is exercised by the first CI run.

- [ ] **Step 4: `scripts/deploy.sh` — the honest fallback row** — before `scripts/scorecard-publish.sh`:

```bash
# A manual deploy is unaudited by definition: say so on the colophon instead of pretending a CI row is pending.
if [ ! -f scorecard.json ]; then
  printf '[{"check":"Audited build","pass":false,"value":"%s — manual deploy, not audited by CI","when":"%s","link":"","link_text":""}]\n' "$(git rev-parse --short HEAD)" "$(date -u +%F)" > .cache/scorecard-manual-deploy.json
  scripts/scorecard-publish.sh .cache/scorecard-manual-deploy.json
else
  scripts/scorecard-publish.sh scorecard.json
fi
```

(replacing the bare `scripts/scorecard-publish.sh` line).

- [ ] **Step 5: `scripts/verify-edge.sh --ci`** — at the top after `set -uo pipefail`: `CI_MODE=0; [ "${1:-}" = "--ci" ] && CI_MODE=1`. Wrap the "zone settings (API)" section: `if [ "$CI_MODE" = 0 ]; then section "zone settings (API)"; …; else section "zone settings (API) — skipped: the CI token has no zone scope"; fi`. In "apex headers and routing" replace the CSP regex with one that accepts the placeholder's and the site's:

```bash
expect_header "$U/" content-security-policy "^default-src 'none'; (style-src 'sha256-[A-Za-z0-9+/=]+'; img-src 'self'; font-src 'self'; )?base-uri 'none'; form-action 'none'; frame-ancestors 'none'$"
```

Run `scripts/verify-edge.sh` (full, with the operator file) → passes as before; `scripts/verify-edge.sh --ci` with only the two variables → passes with the API section skipped. Note: the section list must still end with `[ "$FAIL" -eq 0 ]`.

- [ ] **Step 6: shellcheck** — if `shellcheck` is installed run it on the five scripts; if not, note it. `bash -n` on each.

- [ ] **Step 7: Commit** — `M2a: scripts — KV over REST, dist hashes, an honest row for manual deploys, verify-edge --ci` (WHY: the rollback stash needs metadata; CI has no zone scope; the site's CSP differs from the placeholder's).

---

### Task 5: Bench skeleton — pages, row contract, CLI, merger, first module (`i18n`), fixture tests

**Files:**
- Create: `bench/package.json`, `bench/package-lock.json`, `bench/README.md`, `bench/audit.mjs`, `bench/merge.mjs`, `bench/lib/pages.mjs`, `bench/lib/row.mjs`, `bench/lib/sh.mjs`, `bench/lib/i18n.mjs`, `bench/fixtures/site-ok/**` (a tiny built site), `bench/test/helpers.mjs`, `bench/test/i18n.test.mjs`, `bench/test/merge.test.mjs`

**Interfaces:**
- Produces:
  - `Page = { url, path, status, headers: Headers, html: string, $: CheerioAPI }`; `loadPages(base, { include404 = false } = {}) → Promise<Page[]>` (reads `/sitemap.xml` at `base`, rewrites each `<loc>` origin to `base`, fetches each; `include404` adds `/404.html`).
  - `Ctx = { base, mode: "ci"|"post", pages, dist, when, link, linkText, browser }` where `browser()` returns a shared Playwright `Browser` (Task 7).
  - `row(check, pass, value, ctx, detail?) → Row` in `row.mjs` (fills `when`, `link`, `link_text`).
  - Every module: `export async function run(ctx) → Promise<Row[]>`; `export const name` (the row's `check` string); `export const modes = ["ci"]` or `["ci","post"]`.
  - `audit.mjs` CLI: `node bench/audit.mjs <base> --mode ci|post [--only lighthouse|checks] [--form-factor mobile|desktop] [--shard i/n] [--dist DIR] [--link URL] [--link-text TEXT] --out FILE`. Writes `FILE` (rows) and `FILE.detail.json`. Exit 0 even when rows fail (the merger gates).
  - `merge.mjs` CLI: `node bench/merge.mjs --out scorecard.json [--build SHA --build-url URL] [--summary FILE] [--no-gate] rows...` — orders rows by `ORDER` (below), folds Lighthouse shard rows, prepends `Audited build`, writes the Markdown table to `--summary`, exits 1 when a row fails unless `--no-gate`.

- [ ] **Step 1: `bench/package.json`** (fill exact current versions at execution with `npm view <pkg> version`; no ranges):

```json
{
  "name": "herinean-com-bench",
  "private": true,
  "type": "module",
  "description": "The audit bench: measures the scorecard rows against a served build. Runs in CI; scripts/bench.sh runs it locally.",
  "engines": { "node": ">=24" },
  "scripts": { "test": "node --test test/" },
  "dependencies": {
    "@axe-core/playwright": "<exact>",
    "cheerio": "<exact>",
    "chrome-launcher": "<exact>",
    "fast-xml-parser": "<exact>",
    "html-validate": "<exact>",
    "lighthouse": "<exact>",
    "playwright": "<exact>",
    "vnu-jar": "<exact>",
    "@mdn/mdn-http-observatory": "<exact>"
  }
}
```

`npm install --package-lock-only`, `npm ci`. If `@mdn/mdn-http-observatory` is not the package that provides the Observatory scanner CLI, find the current one on npm (`npm search http-observatory`), record the ruling, and use it (Task 11 needs it).

- [ ] **Step 2: `bench/lib/row.mjs` and `sh.mjs`**

```js
// row.mjs — the one shape every module returns; identical to internal/site/scorecard.go's ciRow.
export function row(check, pass, value, ctx, detail) {
  const r = { check, pass: !!pass, value, when: ctx.when, link: ctx.link || "", link_text: ctx.link ? ctx.linkText || "CI run" : "" };
  if (detail !== undefined) r.detail = detail;
  return r;
}
export const today = () => new Date().toISOString().slice(0, 10);
// summarise a list of problems into a value string: "ok" or "N problems: first; second; …"
export function summary(okText, problems, max = 3) {
  if (problems.length === 0) return okText;
  const shown = problems.slice(0, max).join("; ");
  return `${problems.length} problem${problems.length === 1 ? "" : "s"}: ${shown}${problems.length > max ? "; …" : ""}`;
}
```

```js
// sh.mjs — run a command, capture stdout/stderr, never throw on a non-zero exit (callers decide).
import { spawnSync } from "node:child_process";
export function sh(cmd, args, opts = {}) {
  const r = spawnSync(cmd, args, { encoding: "utf8", maxBuffer: 64 << 20, ...opts });
  return { code: r.status ?? -1, out: r.stdout || "", err: r.stderr || "", error: r.error };
}
```

- [ ] **Step 3: `bench/lib/pages.mjs`**

```js
import * as cheerio from "cheerio";
export async function fetchPage(base, path) {
  const res = await fetch(base + path, { redirect: "manual", headers: { "user-agent": "herinean-bench" } });
  const html = await res.text();
  return { url: base + path, path, status: res.status, headers: res.headers, html, $: cheerio.load(html) };
}
export async function loadPages(base, { include404 = false } = {}) {
  const sm = await fetch(base + "/sitemap.xml");
  if (!sm.ok) throw new Error(`sitemap.xml: ${sm.status}`);
  const locs = [...(await sm.text()).matchAll(/<loc>([^<]+)<\/loc>/g)].map((m) => new URL(m[1]).pathname);
  const paths = [...new Set(locs)].sort();
  if (include404) paths.push("/404.html");
  return Promise.all(paths.map((p) => fetchPage(base, p)));
}
export const sitemapPaths = (pages) => pages.filter((p) => p.path !== "/404.html");
```

- [ ] **Step 4: `bench/lib/i18n.mjs`** (row 8)

```js
import { row, summary } from "./row.mjs";
export const name = "i18n";
export const modes = ["ci"];
// Row 8: <html lang>, symmetric hreflang pairs + x-default, exactly one absolute canonical with a trailing slash.
export async function run(ctx) {
  const problems = [];
  const pages = ctx.pages.filter((p) => p.path !== "/404.html");
  const byPath = new Map(pages.map((p) => [p.path, p]));
  for (const p of pages) {
    const lang = p.$("html").attr("lang");
    const expect = p.path.startsWith("/ro/") ? "ro" : "en";
    if (lang !== expect) problems.push(`${p.path}: lang=${lang} want ${expect}`);
    const canon = p.$('link[rel="canonical"]');
    if (canon.length !== 1) problems.push(`${p.path}: ${canon.length} canonical links`);
    const href = canon.attr("href") || "";
    if (!/^https:\/\/[^/]+\/.*\/$|^https:\/\/[^/]+\/$/.test(href)) problems.push(`${p.path}: canonical ${href} not absolute with a trailing slash`);
    const alts = p.$('link[rel="alternate"][hreflang]').toArray().map((e) => ({ l: p.$(e).attr("hreflang"), h: p.$(e).attr("href") }));
    if (alts.length && !alts.some((a) => a.l === "x-default")) problems.push(`${p.path}: hreflang without x-default`);
    for (const a of alts) {
      if (a.l === "x-default") continue;
      const other = byPath.get(new URL(a.h).pathname);
      if (!other) { problems.push(`${p.path}: hreflang ${a.l} → ${a.h} not in the sitemap`); continue; }
      const back = other.$('link[rel="alternate"][hreflang]').toArray().map((e) => other.$(e).attr("href"));
      if (!back.includes(href)) problems.push(`${p.path}: ${a.h} does not point back`);
    }
  }
  return [row(name, problems.length === 0, summary(`${pages.length} pages: lang, canonical, hreflang symmetric`, problems), ctx, problems)];
}
```

- [ ] **Step 5: `bench/audit.mjs`**

```js
#!/usr/bin/env node
// Audits one served site against the scorecard rows and writes them as JSON (the shape site scorecard reads).
// usage: node bench/audit.mjs BASE --mode ci|post [--only lighthouse|checks] [--form-factor mobile|desktop] [--shard i/n] [--dist DIR] [--link URL] [--link-text T] --out FILE
import { writeFileSync } from "node:fs";
import { parseArgs } from "node:util";
import { loadPages } from "./lib/pages.mjs";
import { today } from "./lib/row.mjs";
import { makeBrowser } from "./lib/browser.mjs";

const { values: a, positionals } = parseArgs({ allowPositionals: true, options: {
  mode: { type: "string", default: "ci" }, only: { type: "string" }, "form-factor": { type: "string" }, shard: { type: "string", default: "1/1" },
  dist: { type: "string" }, link: { type: "string" }, "link-text": { type: "string" }, out: { type: "string" } } });
const base = (positionals[0] || "").replace(/\/$/, "");
if (!base || !a.out) { console.error("usage: audit.mjs BASE --out FILE [--mode ci|post] …"); process.exit(2); }

const MODULES = {
  checks: ["i18n", "social", "wellknown", "feeds", "jsonld", "headers", "privacy", "weight", "links", "html", "a11y", "fonts", "transport", "observatory", "dns", "caching"],
  lighthouse: ["lighthouse"],
};
const wanted = a.only ? MODULES[a.only] : [...MODULES.checks, ...MODULES.lighthouse];
const ctx = { base, mode: a.mode, dist: a.dist, when: today(), link: a.link, linkText: a["link-text"], formFactor: a["form-factor"], shard: a.shard, browser: makeBrowser() };
ctx.pages = await loadPages(base, { include404: true });
const rows = [], detail = {};
for (const m of wanted) {
  const mod = await import(`./lib/${m}.mjs`);
  if (!mod.modes.includes(ctx.mode)) continue;
  const t0 = Date.now();
  try {
    for (const r of await mod.run(ctx)) { detail[r.check] = r.detail; delete r.detail; rows.push(r); console.error(`${r.pass ? "ok  " : "FAIL"} ${r.check}: ${r.value} (${Date.now() - t0} ms)`); }
  } catch (e) {
    rows.push({ check: mod.name, pass: false, value: `bench error: ${e.message}`, when: ctx.when, link: ctx.link || "", link_text: ctx.link ? "CI run" : "" });
    console.error(`FAIL ${mod.name}: ${e.stack}`);
  }
}
await ctx.browser.close();
writeFileSync(a.out, JSON.stringify(rows, null, 1) + "\n");
writeFileSync(a.out + ".detail.json", JSON.stringify(detail, null, 1) + "\n");
```

`bench/lib/browser.mjs` (used from Task 7; create now):

```js
import { chromium } from "playwright";
// One browser per audit run, launched on first use; every module that needs a page gets its own context.
export function makeBrowser() {
  let b;
  const get = async () => (b ??= await chromium.launch());
  get.close = async () => { if (b) await b.close(); };
  return get;
}
```

Modules not yet written are skipped by `audit.mjs` only if the file exists: until Task 11 finishes, the executor keeps `MODULES.checks` limited to the modules that exist (add a name in the task that creates it).

- [ ] **Step 6: `bench/merge.mjs`**

```js
#!/usr/bin/env node
// Folds row files into scorecard.json in spec order, merges Lighthouse shards, prepends the audited build, gates.
import { readFileSync, writeFileSync } from "node:fs";
import { parseArgs } from "node:util";
const { values: a, positionals: files } = parseArgs({ allowPositionals: true, options: { out: { type: "string" }, build: { type: "string" }, "build-url": { type: "string" }, summary: { type: "string" }, "no-gate": { type: "boolean", default: false } } });
if (!a.out || files.length === 0) { console.error("usage: merge.mjs --out scorecard.json [--build SHA --build-url URL] [--summary FILE] [--no-gate] rows.json…"); process.exit(2); }

export const ORDER = ["Audited build", "Lighthouse", "HTML validity", "Accessibility", "Links", "Feeds", "Structured data", "Social previews", "i18n", "Security headers", "Weight", "Privacy", "Well-known files", "Font correctness",
  "Lighthouse (production)", "HTML validity (production)", "Accessibility (production)", "Security headers (production)", "Transport (production)", "Mozilla HTTP Observatory (production)", "DNS, mail, domains (production)", "Privacy (production)", "Caching (production)"];

export function merge(rowsIn, build) {
  const rows = [];
  const lh = new Map(); // check → shard rows
  for (const r of rowsIn) {
    const m = /^(Lighthouse(?: \(production\))?) \[(mobile|desktop) (\d+)\/(\d+)\]$/.exec(r.check);
    if (m) { (lh.get(m[1]) || lh.set(m[1], []).get(m[1])).push({ ...r, ff: m[2], i: +m[3], n: +m[4] }); continue; }
    rows.push(r);
  }
  for (const [check, shards] of lh) {
    const n = shards[0].n, have = new Set(shards.map((s) => `${s.ff}${s.i}`));
    const missing = ["mobile", "desktop"].flatMap((ff) => Array.from({ length: n }, (_, k) => `${ff}${k + 1}`)).filter((k) => !have.has(k));
    const pass = missing.length === 0 && shards.every((s) => s.pass);
    const pages = shards.reduce((t, s) => t + (s.pages || 0), 0) / 2;
    const worst = Math.min(...shards.map((s) => s.worst ?? 100));
    const failing = shards.filter((s) => !s.pass).map((s) => s.value);
    const value = missing.length ? `${missing.length} shards missing (${missing.join(", ")})` : pass ? `${pages} pages × mobile + desktop, 3-run median: all 100 (worst ${worst})` : failing.join("; ");
    rows.push({ check, pass, value, when: shards[0].when, link: shards[0].link, link_text: shards[0].link_text });
  }
  if (build) rows.unshift({ check: "Audited build", pass: true, value: build.sha, when: rows[0]?.when || new Date().toISOString().slice(0, 10), link: build.url, link_text: "commit" });
  const idx = (c) => { const i = ORDER.indexOf(c); return i < 0 ? ORDER.length : i; };
  rows.sort((x, y) => idx(x.check) - idx(y.check));
  return rows;
}

export const table = (rows) => ["| Check | Result | Value |", "|---|---|---|", ...rows.map((r) => `| ${r.check} | ${r.pass ? "✅ pass" : "❌ FAIL"} | ${r.value.replace(/\|/g, "\\|")} |`)].join("\n") + "\n";

if (import.meta.url === `file://${process.argv[1]}`) {
  const rowsIn = files.flatMap((f) => JSON.parse(readFileSync(f, "utf8")));
  const rows = merge(rowsIn, a.build ? { sha: a.build, url: a["build-url"] || "" } : null);
  writeFileSync(a.out, JSON.stringify(rows, null, 1) + "\n");
  if (a.summary) writeFileSync(a.summary, table(rows), { flag: "a" });
  const red = rows.filter((r) => !r.pass);
  console.error(table(rows));
  if (red.length && !a["no-gate"]) { console.error(`${red.length} row(s) red`); process.exit(1); }
}
```

(Shard rows carry `pages` and `worst` as extra fields — Task 9 writes them; `audit.mjs` must not strip them: it only strips `detail`.)

- [ ] **Step 7: Fixture and tests** — `bench/fixtures/site-ok/` is a minimal *built* site: `index.html`, `ro/index.html`, `sitemap.xml` with both URLs, `_headers` copied from a real build, `robots.txt`, `llms.txt`, `.well-known/security.txt`, `feed.xml`, `feed.json`, `favicon.svg`, `favicon.ico`, `apple-touch-icon.png` (1×1), `404.html` — the simplest way: copy the real `dist/` of the current build (`./.cache/site build`) and delete the piece-specific files; it must stay small (no fonts: replace the three woff2 with the real ones only if under 100 KB total — they are). Record the build commit in `bench/fixtures/README.md`. Tests serve fixtures with `site serve --static`? No — the bench tests must not depend on the Go binary. `bench/test/helpers.mjs`:

```js
import { createServer } from "node:http";
import { readFileSync, statSync, existsSync } from "node:fs";
import { join } from "node:path";
// Serves a fixture directory like site serve --static would: _headers applied, index.html for directories, 404.html with 404.
export function serveFixture(dir, headersOverride) {
  const rules = parseHeaders(existsSync(join(dir, "_headers")) ? readFileSync(join(dir, "_headers"), "utf8") : "");
  const srv = createServer((req, res) => {
    let p = decodeURIComponent(new URL(req.url, "http://x").pathname);
    let fp = join(dir, p);
    if (existsSync(fp) && statSync(fp).isDirectory()) fp = join(fp, "index.html");
    for (const r of rules) if (r.match(p)) for (const [k, v] of r.headers) v === null ? res.removeHeader(k) : res.setHeader(k, v);
    if (headersOverride) for (const [k, v] of Object.entries(headersOverride)) res.setHeader(k, v);
    if (!existsSync(fp) || p === "/404.html") { res.statusCode = 404; res.setHeader("content-type", "text/html; charset=utf-8"); res.end(existsSync(join(dir, "404.html")) ? readFileSync(join(dir, "404.html")) : "nope"); return; }
    const ext = fp.split(".").pop();
    const types = { html: "text/html; charset=utf-8", xml: "application/xml; charset=utf-8", json: "application/json", txt: "text/plain; charset=utf-8", svg: "image/svg+xml", png: "image/png", ico: "image/x-icon", woff2: "font/woff2", webp: "image/webp", jpg: "image/jpeg", css: "text/css" };
    if (!res.getHeader("content-type")) res.setHeader("content-type", types[ext] || "application/octet-stream");
    res.end(readFileSync(fp));
  });
  return new Promise((resolve) => srv.listen(0, "127.0.0.1", () => resolve({ base: `http://127.0.0.1:${srv.address().port}`, close: () => new Promise((r) => srv.close(r)) })));
}
function parseHeaders(text) {
  const rules = [];
  for (const line of text.split("\n")) {
    if (!line.trim()) continue;
    if (!line.startsWith(" ")) { rules.push({ pattern: line.trim(), headers: [], match(p) { return this.pattern.endsWith("*") ? p.startsWith(this.pattern.slice(0, -1)) : this.pattern === p; } }); continue; }
    const l = line.trim();
    if (l.startsWith("! ")) { rules.at(-1).headers.push([l.slice(2), null]); continue; }
    const i = l.indexOf(": "); if (i > 0) rules.at(-1).headers.push([l.slice(0, i), l.slice(i + 2)]);
  }
  return rules;
}
export const ctxFor = (base, pages, extra = {}) => ({ base, mode: "ci", pages, when: "2026-09-19", link: "https://example.test/run", linkText: "CI run", ...extra });
```

`bench/test/i18n.test.mjs`:

```js
import { test } from "node:test";
import assert from "node:assert/strict";
import * as cheerio from "cheerio";
import { serveFixture, ctxFor } from "./helpers.mjs";
import { loadPages } from "../lib/pages.mjs";
import { run } from "../lib/i18n.mjs";

test("i18n passes on the fixture and fails when a hreflang pair is broken", async () => {
  const s = await serveFixture("bench/fixtures/site-ok");
  const pages = await loadPages(s.base, { include404: true });
  const [ok] = await run(ctxFor(s.base, pages));
  assert.equal(ok.pass, true, ok.value);
  const broken = pages.map((p) => (p.path === "/ro/" ? { ...p, $: cheerio.load(p.html.replace('hreflang="en"', 'hreflang="de"')) } : p));
  const [bad] = await run(ctxFor(s.base, broken));
  assert.equal(bad.pass, false);
  assert.match(bad.value, /does not point back|not in the sitemap/);
  await s.close();
});
```

`bench/test/merge.test.mjs`:

```js
import { test } from "node:test";
import assert from "node:assert/strict";
import { merge, ORDER } from "../merge.mjs";
const r = (check, pass, extra = {}) => ({ check, pass, value: pass ? "ok" : "bad", when: "2026-09-19", link: "", link_text: "", ...extra });

test("Lighthouse shards fold into one row; a missing shard is a failure; order follows the spec", () => {
  const rows = merge([r("i18n", true), r("Lighthouse [mobile 1/2]", true, { pages: 4, worst: 100 }), r("Lighthouse [mobile 2/2]", true, { pages: 3, worst: 100 }), r("Lighthouse [desktop 1/2]", true, { pages: 4, worst: 100 }), r("Lighthouse [desktop 2/2]", true, { pages: 3, worst: 100 })], { sha: "abc1234", url: "https://x/commit/abc1234" });
  assert.deepEqual(rows.map((x) => x.check), ["Audited build", "Lighthouse", "i18n"]);
  assert.match(rows[1].value, /7 pages × mobile \+ desktop/);
  const partial = merge([r("Lighthouse [mobile 1/2]", true, { pages: 4 })]);
  assert.equal(partial[0].pass, false);
  assert.match(partial[0].value, /3 shards missing/);
  assert.ok(ORDER.indexOf("Weight") > ORDER.indexOf("Security headers"));
});
```

Run `cd bench && npm test` → both pass. Run `node bench/audit.mjs http://127.0.0.1:8090 --only checks --out /tmp/rows.json` against `site serve --static` on the real dist → `ok   i18n: 7 pages: …`.

- [ ] **Step 8: `bench/README.md`** — what the bench is (spec §7), how CI runs it, how to run it locally (`scripts/bench.sh`, Task 12), the row contract, the module interface, one line per module (added as modules land).

- [ ] **Step 9: Commit** — `M2a: bench — skeleton: pages, row contract, audit CLI, merger with the Lighthouse fold, i18n row` (WHY: one schema for every row, the merger is the gate).

---

### Task 6: Static rows — `social` (7), `wellknown` (16), `feeds` (5), `jsonld` (6)

**Files:**
- Create: `bench/lib/social.mjs`, `bench/lib/wellknown.mjs`, `bench/lib/feeds.mjs`, `bench/lib/jsonld.mjs`, `bench/test/static-rows.test.mjs`, fixtures under `bench/fixtures/site-ok/` as needed (an `og/` PNG of 1200×630 under 200 KB — generate with the real build's OG card)
- Modify: `bench/audit.mjs` (`MODULES.checks` gains the four names)

**Interfaces:** each exports `name`, `modes = ["ci"]`, `run(ctx)`; `name` values: `"Social previews"`, `"Well-known files"`, `"Feeds"`, `"Structured data"`.

- [ ] **Step 1: `social.mjs`**

```js
import { row, summary } from "./row.mjs";
export const name = "Social previews";
export const modes = ["ci"];
const meta = ($, sel) => $(`meta[property="${sel}"], meta[name="${sel}"]`).attr("content");
// Row 7: og:* + twitter:card=summary_large_image on every page; og:image is a 1200×630 PNG < 200 KB at a content-hashed URL.
export async function run(ctx) {
  const problems = [];
  const images = new Map();
  for (const p of ctx.pages.filter((p) => p.path !== "/404.html")) {
    for (const k of ["og:title", "og:description", "og:url", "og:image", "og:type", "og:locale"]) if (!meta(p.$, k)) problems.push(`${p.path}: no ${k}`);
    if (meta(p.$, "twitter:card") !== "summary_large_image") problems.push(`${p.path}: twitter:card is ${meta(p.$, "twitter:card")}`);
    const img = meta(p.$, "og:image");
    if (img) images.set(img, p.path);
  }
  for (const [img, from] of images) {
    const u = new URL(img);
    if (!/^\/og\/[A-Za-z0-9._-]+\.[0-9a-f]{8}\.png$/.test(u.pathname)) problems.push(`${from}: og:image ${u.pathname} is not a hashed /og/ PNG`);
    const res = await fetch(ctx.base + u.pathname);
    if (!res.ok) { problems.push(`${from}: og:image ${u.pathname} → ${res.status}`); continue; }
    const buf = Buffer.from(await res.arrayBuffer());
    if (buf.length >= 200 * 1024) problems.push(`${from}: og:image ${buf.length} bytes ≥ 200 KB`);
    if (buf.readUInt32BE(16) !== 1200 || buf.readUInt32BE(20) !== 630) problems.push(`${from}: og:image is ${buf.readUInt32BE(16)}×${buf.readUInt32BE(20)}, want 1200×630`);
  }
  return [row(name, problems.length === 0, summary(`${ctx.pages.length - 1} pages, ${images.size} OG images: 1200×630 PNG < 200 KB, hashed URLs`, problems), ctx, problems)];
}
```

- [ ] **Step 2: `wellknown.mjs`**

```js
import { row, summary } from "./row.mjs";
export const name = "Well-known files";
export const modes = ["ci"];
const get = async (base, path) => { const r = await fetch(base + path, { redirect: "manual" }); return { status: r.status, type: r.headers.get("content-type") || "", body: r.status === 200 ? await r.text() : "" }; };
// Row 16: security.txt (RFC 9116 fields, Expires ≤ 1 year ahead, still valid), robots.txt with Sitemap, sitemap with hreflang + lastmod, llms.txt, favicons, 404 with a 404 status.
export async function run(ctx) {
  const problems = [];
  const sec = await get(ctx.base, "/.well-known/security.txt");
  if (sec.status !== 200 || !sec.type.startsWith("text/plain")) problems.push(`security.txt: ${sec.status} ${sec.type}`);
  for (const f of ["Contact:", "Expires:", "Preferred-Languages:", "Canonical:"]) if (!sec.body.includes(f)) problems.push(`security.txt: no ${f}`);
  const exp = /Expires:\s*(\S+)/.exec(sec.body)?.[1];
  if (exp) { const d = new Date(exp), now = new Date(); if (isNaN(d)) problems.push("security.txt: Expires unparsable"); else if (d < now) problems.push("security.txt: expired"); else if (d - now > 366 * 864e5) problems.push("security.txt: Expires more than a year ahead"); else if (d - now < 30 * 864e5) problems.push(`security.txt: expires in ${Math.floor((d - now) / 864e5)} days (renew)`); }
  const robots = await get(ctx.base, "/robots.txt");
  if (robots.status !== 200 || !/^Sitemap: https:\/\/\S+\/sitemap\.xml$/m.test(robots.body)) problems.push(`robots.txt: ${robots.status}, no Sitemap line`);
  const sm = await get(ctx.base, "/sitemap.xml");
  if (sm.status !== 200 || !sm.type.startsWith("application/xml")) problems.push(`sitemap.xml: ${sm.status} ${sm.type}`);
  const urls = sm.body.split("<url>").slice(1);
  for (const u of urls) { if (!/<lastmod>\d{4}-\d{2}-\d{2}/.test(u)) problems.push("sitemap: a url lacks lastmod"); if (!/hreflang="x-default"/.test(u)) problems.push("sitemap: a url lacks x-default"); }
  const llms = await get(ctx.base, "/llms.txt");
  if (llms.status !== 200 || !llms.type.startsWith("text/plain")) problems.push(`llms.txt: ${llms.status} ${llms.type}`);
  for (const [path, type] of [["/favicon.svg", "image/svg+xml"], ["/favicon.ico", "image/"], ["/apple-touch-icon.png", "image/png"]]) { const r = await get(ctx.base, path); if (r.status !== 200 || !r.type.startsWith(type)) problems.push(`${path}: ${r.status} ${r.type}`); }
  const nf = await get(ctx.base, "/this-page-does-not-exist/");
  if (nf.status !== 404) problems.push(`unknown path → ${nf.status}, want 404`);
  const home = ctx.pages.find((p) => p.path === "/");
  if (home && home.$('link[rel="icon"][type="image/svg+xml"]').length === 0) problems.push("home: no SVG favicon link");
  return [row(name, problems.length === 0, summary("security.txt, robots.txt, sitemap.xml, llms.txt, favicons, 404", problems), ctx, problems)];
}
```

- [ ] **Step 3: `feeds.mjs`**

```js
import { XMLParser, XMLValidator } from "fast-xml-parser";
import { row, summary } from "./row.mjs";
export const name = "Feeds";
export const modes = ["ci"];
// Row 5: RSS 2.0 (all, en, ro) + JSON Feed 1.1 (all): strict XML, required elements, full text, absolute URLs, atom:link rel=self. The W3C validator is a web service — a manual row.
export async function run(ctx) {
  const problems = [];
  const abs = (s) => /^https:\/\//.test(s);
  for (const path of ["/feed.xml", "/feed.en.xml", "/feed.ro.xml"]) {
    const res = await fetch(ctx.base + path);
    const text = await res.text();
    if (!res.ok || !(res.headers.get("content-type") || "").startsWith("application/rss+xml")) problems.push(`${path}: ${res.status} ${res.headers.get("content-type")}`);
    const v = XMLValidator.validate(text);
    if (v !== true) { problems.push(`${path}: ${v.err.msg}`); continue; }
    const x = new XMLParser({ ignoreAttributes: false, attributeNamePrefix: "@" }).parse(text);
    const ch = x.rss?.channel;
    if (!ch || x.rss["@version"] !== "2.0") { problems.push(`${path}: not RSS 2.0`); continue; }
    for (const k of ["title", "link", "description", "language"]) if (!ch[k]) problems.push(`${path}: channel lacks ${k}`);
    const self = ch["atom:link"]; const selfHref = (Array.isArray(self) ? self : [self]).find((l) => l?.["@rel"] === "self")?.["@href"];
    if (!selfHref || new URL(selfHref).pathname !== path) problems.push(`${path}: atom:link rel=self is ${selfHref}`);
    const items = ch.item ? (Array.isArray(ch.item) ? ch.item : [ch.item]) : [];
    for (const it of items) {
      if (!it.guid || !it.pubDate || !abs(String(it.link || ""))) problems.push(`${path}: item lacks guid/pubDate/absolute link`);
      const body = it["content:encoded"] || "";
      if (!body || body.length < 200) problems.push(`${path}: item ${it.link} has no full text`);
      for (const m of body.matchAll(/\s(?:href|src)="([^"]+)"/g)) if (!abs(m[1])) problems.push(`${path}: relative URL in content: ${m[1]}`);
    }
  }
  const jres = await fetch(ctx.base + "/feed.json");
  if (!jres.ok || !(jres.headers.get("content-type") || "").startsWith("application/feed+json")) problems.push(`feed.json: ${jres.status} ${jres.headers.get("content-type")}`);
  let j; try { j = await jres.json(); } catch (e) { problems.push(`feed.json: ${e.message}`); }
  if (j) {
    if (j.version !== "https://jsonfeed.org/version/1.1") problems.push(`feed.json: version ${j.version}`);
    for (const k of ["title", "home_page_url", "feed_url"]) if (typeof j[k] !== "string" || !j[k]) problems.push(`feed.json: ${k} missing`);
    if (!Array.isArray(j.items)) problems.push("feed.json: items is not an array");
    for (const it of j.items || []) {
      for (const k of ["id", "url", "title", "content_html", "date_published"]) if (!it[k]) problems.push(`feed.json: item lacks ${k}`);
      if (it.date_published && isNaN(Date.parse(it.date_published))) problems.push(`feed.json: bad date ${it.date_published}`);
      if (it.url && !abs(it.url)) problems.push(`feed.json: relative url ${it.url}`);
    }
  }
  return [row(name, problems.length === 0, summary("RSS ×3 strict parse, full text, absolute URLs, self link; JSON Feed 1.1 required fields", problems), ctx, problems)];
}
```

- [ ] **Step 4: `jsonld.mjs`**

```js
import { row, summary } from "./row.mjs";
export const name = "Structured data";
export const modes = ["ci"];
const REQUIRED = {
  WebSite: ["name", "url", "inLanguage"],
  Person: ["name", "url", "sameAs"],
  BlogPosting: ["headline", "inLanguage", "image", "author", "datePublished", "dateModified", "url"],
  BreadcrumbList: ["itemListElement"],
};
// Row 6: every ld+json block parses; each object of a known type carries its required fields; Person.sameAs has LinkedIn, X and GitHub.
export async function run(ctx) {
  const problems = [];
  let blocks = 0;
  for (const p of ctx.pages.filter((p) => p.path !== "/404.html")) {
    const scripts = p.$('script[type="application/ld+json"]').toArray();
    if (scripts.length === 0) problems.push(`${p.path}: no JSON-LD`);
    for (const s of scripts) {
      blocks++;
      let data; try { data = JSON.parse(p.$(s).text()); } catch (e) { problems.push(`${p.path}: JSON-LD unparsable: ${e.message}`); continue; }
      const objs = Array.isArray(data["@graph"]) ? data["@graph"] : Array.isArray(data) ? data : [data];
      for (const o of objs) {
        const type = Array.isArray(o["@type"]) ? o["@type"][0] : o["@type"];
        const req = REQUIRED[type]; if (!req) continue;
        for (const k of req) if (o[k] === undefined || o[k] === "" ) problems.push(`${p.path}: ${type} lacks ${k}`);
        if (type === "Person") for (const host of ["linkedin.com", "x.com", "github.com"]) if (!(o.sameAs || []).some((u) => u.includes(host))) problems.push(`${p.path}: Person.sameAs lacks ${host}`);
      }
    }
  }
  return [row(name, problems.length === 0, summary(`${blocks} JSON-LD blocks: WebSite, Person (sameAs ×3), BlogPosting, BreadcrumbList fields present`, problems), ctx, problems)];
}
```

Check the real pages' JSON-LD shape first (`grep -o '<script type="application/ld+json">[^<]*' dist/index.html`) and adjust `REQUIRED` to what the generator emits and the spec row lists; if the generator lacks a required field, that is a real finding: fix the generator (`internal/seo`), not the check, and record it.

- [ ] **Step 5: Tests** — `bench/test/static-rows.test.mjs`: for each module, pass on the fixture, then one mutation that must fail (og:image dims wrong → serve a fixture copy with a 1×1 PNG; robots.txt without Sitemap; a feed with a relative href; a Person without github). Use `serveFixture` with a temp copy (`fs.cpSync` to `os.tmpdir()`), mutate the file, run, assert `pass === false` and the message.

- [ ] **Step 6: Run on the real dist** — `node bench/audit.mjs http://127.0.0.1:8090 --only checks --out /tmp/rows.json`; every row must be `ok`. Anything red is a real finding: fix the generator or templates in this task (small) or record it for the controller if large.

- [ ] **Step 7: Commit** — `M2a: bench — social previews, well-known files, feeds, structured data rows`.

---

### Task 7: Served-response rows — `headers` (9), `privacy` (15), `weight` (14), `links` (4)

**Files:**
- Create: `bench/lib/headers.mjs`, `bench/lib/privacy.mjs`, `bench/lib/weight.mjs`, `bench/lib/links.mjs`, `bench/links-allow.txt`, `bench/test/served-rows.test.mjs`
- Modify: `bench/audit.mjs` (`MODULES.checks`)

**Interfaces:** `name` values `"Security headers"`, `"Privacy"`, `"Weight"`, `"Links"`; `headers` and `privacy` have `modes = ["ci","post"]` and name their post rows `"… (production)"` via `ctx.mode`.

- [ ] **Step 1: `headers.mjs`** — expectations per path class from spec §6.2 (the same set `internal/edge` writes):

```js
import { createHash } from "node:crypto";
import { row, summary } from "./row.mjs";
export const name = "Security headers";
export const modes = ["ci", "post"];
const COMMON = { "x-content-type-options": /^nosniff$/, "referrer-policy": /^strict-origin-when-cross-origin$/, "permissions-policy": /camera=\(\)/, "cross-origin-opener-policy": /^same-origin$/, "x-frame-options": /^DENY$/, "strict-transport-security": /^max-age=63072000; includeSubDomains; preload$/ };
const CLASSES = [
  { path: "/", expect: { ...COMMON, "content-security-policy": /^default-src 'none'; style-src 'sha256-[A-Za-z0-9+/=]+'; img-src 'self'; font-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'$/, "cross-origin-resource-policy": /^same-origin$/, "cache-control": /^public, max-age=0, must-revalidate$/, "content-type": /^text\/html; charset=utf-8$/ } },
  { find: (pages) => pages[0].$('link[rel="preload"][as="font"]').attr("href"), expect: { ...COMMON, "cross-origin-resource-policy": /^same-origin$/, "cache-control": /^public, max-age=31536000, immutable$/, "content-type": /^font\/woff2$/ } },
  { find: (pages) => pages[0].$('meta[property="og:image"]').attr("content") && new URL(pages[0].$('meta[property="og:image"]').attr("content")).pathname, expect: { ...COMMON, "cross-origin-resource-policy": /^cross-origin$/, "cache-control": /^public, max-age=31536000, immutable$/, "content-type": /^image\/png$/ } },
  { find: (pages) => pages.map((p) => p.$("img[src^='/img/']").attr("src")).find(Boolean), expect: { ...COMMON, "cross-origin-resource-policy": /^cross-origin$/, "cache-control": /^public, max-age=31536000, immutable$/ } },
  { path: "/feed.xml", expect: { ...COMMON, "content-type": /^application\/rss\+xml; charset=utf-8$/, "cache-control": /^public, max-age=300$/ } },
  { path: "/feed.json", expect: { ...COMMON, "content-type": /^application\/feed\+json; charset=utf-8$/, "cache-control": /^public, max-age=300$/ } },
  { path: "/sitemap.xml", expect: { ...COMMON, "content-type": /^application\/xml; charset=utf-8$/ } },
  { path: "/robots.txt", expect: { ...COMMON, "content-type": /^text\/plain; charset=utf-8$/ } },
  { path: "/llms.txt", expect: { ...COMMON, "content-type": /^text\/plain; charset=utf-8$/ } },
  { path: "/.well-known/security.txt", expect: { ...COMMON, "content-type": /^text\/plain; charset=utf-8$/ } },
];
// Row 9: the served headers per path class match spec §6.2; the CSP's style hash is the page's own <style>; HSTS everywhere.
export async function run(ctx) {
  const problems = [];
  const pages = ctx.pages.filter((p) => p.path !== "/404.html");
  for (const c of CLASSES) {
    const path = c.path || (c.find && c.find(pages));
    if (!path) { problems.push(`no URL found for a path class (${JSON.stringify(Object.keys(c.expect).slice(0, 2))})`); continue; }
    const res = await fetch(ctx.base + path, { redirect: "manual" });
    if (res.status !== 200) { problems.push(`${path}: ${res.status}`); continue; }
    for (const [h, re] of Object.entries(c.expect)) { const v = res.headers.get(h); if (v === null || !re.test(v)) problems.push(`${path}: ${h} = ${v === null ? "<absent>" : v}`); }
    if (res.headers.has("set-cookie")) problems.push(`${path}: Set-Cookie`);
  }
  for (const p of pages) {
    const csp = p.headers.get("content-security-policy") || "";
    const want = "'sha256-" + createHash("sha256").update(p.$("style").first().html() || "").digest("base64") + "'";
    if (!csp.includes(want)) problems.push(`${p.path}: CSP style hash does not match the inline <style>`);
  }
  const check = ctx.mode === "post" ? `${name} (production)` : name;
  return [row(check, problems.length === 0, summary(`${CLASSES.length} path classes match §6.2; CSP hash = inline style on ${pages.length} pages`, problems), ctx, problems)];
}
```

(`cheerio`'s `.html()` on `<style>` returns the raw text; confirm the hash equals `_headers`' on the real dist — if the generator hashes the raw CSS bytes with a different whitespace, use `p.html` and a regex `/<style>([\s\S]*?)<\/style>/` instead. Record which.)

- [ ] **Step 2: `privacy.mjs`** — Playwright request log:

```js
import { row, summary } from "./row.mjs";
export const name = "Privacy";
export const modes = ["ci", "post"];
// Row 15: no Set-Cookie, no third-party request, no /cdn-cgi/, no executable script (every <script> is ld+json) — measured in a real browser.
export async function run(ctx) {
  const problems = [];
  const origin = new URL(ctx.base).origin;
  const browser = await ctx.browser();
  const bctx = await browser.newContext();
  for (const p of ctx.pages.filter((p) => p.path !== "/404.html")) {
    const page = await bctx.newPage();
    const third = [], cookies = [];
    page.on("request", (r) => { if (new URL(r.url()).origin !== origin) third.push(r.url()); });
    page.on("response", (r) => { if (r.headers()["set-cookie"]) cookies.push(r.url()); });
    await page.goto(ctx.base + p.path, { waitUntil: "networkidle" });
    if (third.length) problems.push(`${p.path}: third-party requests: ${third.slice(0, 2).join(", ")}`);
    if (cookies.length) problems.push(`${p.path}: Set-Cookie from ${cookies[0]}`);
    if ((await bctx.cookies()).length) problems.push(`${p.path}: a cookie was stored`);
    if (p.html.includes("/cdn-cgi/")) problems.push(`${p.path}: /cdn-cgi/ in HTML`);
    const scripts = p.$("script").toArray().filter((s) => p.$(s).attr("type") !== "application/ld+json");
    if (scripts.length) problems.push(`${p.path}: ${scripts.length} executable <script>`);
    await page.close();
  }
  await bctx.close();
  const check = ctx.mode === "post" ? `${name} (production)` : name;
  return [row(check, problems.length === 0, summary(`${ctx.pages.length - 1} pages: 0 cookies, 0 third-party requests, 0 executable scripts, no /cdn-cgi/`, problems), ctx, problems)];
}
```

- [ ] **Step 3: `weight.mjs`**

```js
import { brotliCompressSync, constants } from "node:zlib";
import { row, summary } from "./row.mjs";
export const name = "Weight";
export const modes = ["ci"];
const br = (buf) => brotliCompressSync(buf, { params: { [constants.BROTLI_PARAM_QUALITY]: 5 } }).length; // Cloudflare's default level is in this range
// Row 14: HTML+CSS ≤ 30 KB brotli; fonts ≤ 100 KB, exactly one preloaded; first view ≤ 6 requests and ≤ 150 KB; 0 bytes of executable JS.
export async function run(ctx) {
  const problems = [], per = [];
  const pages = ctx.pages.filter((p) => p.path !== "/404.html");
  const fontURLs = new Set();
  for (const p of pages) for (const m of p.html.matchAll(/url\((\/fonts\/[^)]+)\)/g)) fontURLs.add(m[1]);
  let fontBytes = 0;
  for (const u of fontURLs) fontBytes += (await (await fetch(ctx.base + u)).arrayBuffer()).byteLength;
  if (fontBytes > 100 * 1024) problems.push(`fonts total ${fontBytes} bytes > 100 KB`);
  const browser = await ctx.browser();
  const bctx = await browser.newContext({ viewport: { width: 390, height: 844 } });
  for (const p of pages) {
    const html = br(Buffer.from(p.html));
    if (html > 30 * 1024) problems.push(`${p.path}: HTML+CSS ${html} bytes brotli > 30 KB`);
    const preloads = p.$('link[rel="preload"][as="font"]').length;
    if (preloads !== 1) problems.push(`${p.path}: ${preloads} font preloads, want 1`);
    const page = await bctx.newPage();
    const reqs = [];
    page.on("response", async (r) => { try { const body = await r.body(); const ct = r.headers()["content-type"] || ""; reqs.push({ url: r.url(), bytes: /^(text\/|application\/(json|xml|rss|feed)|image\/svg)/.test(ct) ? br(body) : body.length, js: /javascript/.test(ct) }); } catch {} });
    await page.goto(ctx.base + p.path, { waitUntil: "networkidle" });
    await page.close();
    const total = reqs.reduce((t, r) => t + r.bytes, 0);
    if (reqs.length > 6) problems.push(`${p.path}: ${reqs.length} requests > 6`);
    if (total > 150 * 1024) problems.push(`${p.path}: first view ${total} bytes > 150 KB`);
    if (reqs.some((r) => r.js)) problems.push(`${p.path}: JavaScript fetched`);
    per.push({ path: p.path, html, requests: reqs.length, total });
  }
  await bctx.close();
  const worst = per.reduce((w, x) => (x.total > w.total ? x : w), per[0]);
  return [row(name, problems.length === 0, summary(`worst first view ${worst.path}: ${worst.requests} requests, ${(worst.total / 1024).toFixed(1)} KB brotli-equivalent; fonts ${(fontBytes / 1024).toFixed(1)} KB, one preload; 0 bytes JS`, problems), ctx, per)];
}
```

- [ ] **Step 4: `links.mjs`**

```js
import { readFileSync } from "node:fs";
import { row, summary } from "./row.mjs";
export const name = "Links";
export const modes = ["ci"];
// Row 4: every same-origin href/src resolves (fail); external links are fetched with a timeout and only warn (allowlist bench/links-allow.txt).
export async function run(ctx) {
  const problems = [], warnings = [];
  const allow = readFileSync(new URL("../links-allow.txt", import.meta.url), "utf8").split("\n").map((s) => s.trim()).filter((s) => s && !s.startsWith("#"));
  const internal = new Map(), external = new Map();
  for (const p of ctx.pages) {
    for (const el of p.$("a[href], img[src], link[href], source[srcset], img[srcset]").toArray()) {
      const attr = p.$(el).attr("href") || p.$(el).attr("src") || p.$(el).attr("srcset") || "";
      for (const raw of attr.split(",").map((s) => s.trim().split(" ")[0]).filter(Boolean)) {
        if (raw.startsWith("#") || raw.startsWith("mailto:") || raw.startsWith("data:")) continue;
        const u = new URL(raw, ctx.base + p.path);
        if (u.origin === new URL(ctx.base).origin || u.host === "herinean.com") (internal.get(u.pathname) || internal.set(u.pathname, []).get(u.pathname)).push(p.path);
        else if (!allow.some((a) => u.href.startsWith(a))) (external.get(u.href) || external.set(u.href, []).get(u.href)).push(p.path);
      }
    }
  }
  for (const [path, from] of internal) {
    const res = await fetch(ctx.base + path, { method: "HEAD", redirect: "manual" });
    if (res.status !== 200) problems.push(`${path} → ${res.status} (from ${from[0]})`);
  }
  await Promise.all([...external].map(async ([href, from]) => {
    try {
      const res = await fetch(href, { method: "GET", redirect: "follow", signal: AbortSignal.timeout(10000), headers: { "user-agent": "Mozilla/5.0 (compatible; herinean-bench link check)" } });
      if (res.status >= 400 && res.status !== 403 && res.status !== 429) warnings.push(`${href} → ${res.status} (from ${from[0]})`);
    } catch (e) { warnings.push(`${href}: ${e.name} (from ${from[0]})`); }
  }));
  const value = summary(`${internal.size} internal links resolve; ${external.size} external checked${warnings.length ? `, ${warnings.length} unreachable (warning): ${warnings.slice(0, 2).join("; ")}` : ""}`, problems);
  return [row(name, problems.length === 0, value, ctx, { problems, warnings })];
}
```

`bench/links-allow.txt`: a header comment (`# external URLs known to refuse robots; one prefix per line`) and no entries yet.

- [ ] **Step 5: Tests** — `served-rows.test.mjs`: headers pass on the fixture (its `_headers` is real) and fail when the fixture server overrides `x-frame-options: SAMEORIGIN`; privacy fails when a page carries `<script>alert(1)</script>` (mutated temp copy); weight fails when a page is padded past 30 KB; links fails when an internal href points to `/missing/`. Playwright-based tests launch Chromium (`ctxFor(base, pages, { browser: makeBrowser() })`; close it in `after`).

- [ ] **Step 6: Run on the real dist** → all four `ok`. The real-dist weight numbers go into the ledger.

- [ ] **Step 7: Commit** — `M2a: bench — security headers, privacy, weight and links rows`.

---

### Task 8: Validation rows — `html` (2: Nu + html-validate) and `a11y` (3: axe, both schemes, AAA/AA split)

**Files:**
- Create: `bench/lib/html.mjs`, `bench/lib/a11y.mjs`, `bench/.htmlvalidate.json`, `bench/test/validation-rows.test.mjs`
- Modify: `bench/audit.mjs`

**Interfaces:** `name` `"HTML validity"` / `"Accessibility"`; `modes = ["ci","post"]`; in post mode only `/colophon/` is checked and the row is `"… (production)"`.

- [ ] **Step 1: `.htmlvalidate.json`**

```json
{
  "extends": ["html-validate:recommended", "html-validate:a11y", "html-validate:document"],
  "rules": {
    "no-inline-style": "off"
  }
}
```

(`no-inline-style` flags `style=""` attributes, which the site never uses; the inline `<style>` element is not affected — keep the rule on if the first run shows no hits, and drop the override. Every other rule disabled later must carry a comment in `bench/README.md` with the reason.)

- [ ] **Step 2: `html.mjs`**

```js
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createRequire } from "node:module";
import { HtmlValidate } from "html-validate";
import { row, summary } from "./row.mjs";
import { sh } from "./sh.mjs";
export const name = "HTML validity";
export const modes = ["ci", "post"];
const vnuJar = () => { const rq = createRequire(import.meta.url); return rq("vnu-jar"); }; // the package's main export is the jar's path
// Row 2: Nu checker 0 errors 0 warnings; html-validate (recommended + a11y + document) 0 errors — every page, /404.html included.
export async function run(ctx) {
  const problems = [];
  const pages = ctx.mode === "post" ? ctx.pages.filter((p) => p.path === "/colophon/") : ctx.pages;
  const dir = mkdtempSync(join(tmpdir(), "nu-"));
  const files = pages.map((p, i) => { const f = join(dir, `${i}.html`); writeFileSync(f, p.html); return [f, p.path]; });
  const nu = sh("java", ["-jar", vnuJar(), "--stdout", "--format", "json", "--exit-zero-always", ...files.map((f) => f[0])]);
  if (nu.error) problems.push(`Nu did not run: ${nu.error.message}`);
  else {
    const msgs = JSON.parse(nu.out || '{"messages":[]}').messages;
    for (const m of msgs) if (m.type === "error" || (m.type === "info" && m.subType === "warning")) problems.push(`Nu ${m.type}${m.subType ? "/" + m.subType : ""} ${files.find((f) => m.url?.endsWith(f[0]))?.[1] || ""}:${m.lastLine}: ${m.message}`);
  }
  const hv = new HtmlValidate({ root: true, extends: ["html-validate:recommended", "html-validate:a11y", "html-validate:document"], rules: { "no-inline-style": "off" } });
  for (const p of pages) {
    const report = await hv.validateString(p.html, p.path);
    for (const r of report.results) for (const m of r.messages) if (m.severity === 2) problems.push(`html-validate ${p.path}:${m.line}: ${m.ruleId} ${m.message}`);
  }
  const check = ctx.mode === "post" ? `${name} (production)` : name;
  return [row(check, problems.length === 0, summary(`${pages.length} pages: Nu 0 errors 0 warnings; html-validate 0 errors`, problems), ctx, problems)];
}
```

(Load the config from `bench/.htmlvalidate.json` instead of the inline object if `HtmlValidate` accepts a `configFile`; keep one source — prefer the file and delete the inline copy, or the reverse; record.)

- [ ] **Step 3: `a11y.mjs`**

```js
import AxeBuilder from "@axe-core/playwright";
import { row, summary } from "./row.mjs";
export const name = "Accessibility";
export const modes = ["ci", "post"];
const TAGS = ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"];
// Secondary text is set in --ink-2 (site.css): it must pass AA (4.5:1); everything else must pass AAA (7:1). Keep in step with site.css.
export const SECONDARY = [".tagline", ".meta", ".label", ".badge", ".entry p", ".chroma .c", ".chroma .c1", ".chroma .cm", ".chroma .cp", ".chroma .cs", ".chroma .ch", ".chroma .s", ".chroma .s1", ".chroma .s2", ".chroma .sb", ".chroma .sd", ".chroma .sh", ".chroma .sx", ".chroma .sr", ".chroma .dl", "th", "figcaption", ".footnotes", ".also", ".byline", "footer"];
// Row 3: axe WCAG 2.2 AA in light and dark; AAA contrast outside the secondary selectors; skip link first; visible focus; tab stops = interactive elements; the language link carries lang.
export async function run(ctx) {
  const problems = [];
  const pages = ctx.mode === "post" ? ctx.pages.filter((p) => p.path === "/colophon/") : ctx.pages;
  const browser = await ctx.browser();
  for (const scheme of ["light", "dark"]) {
    const bctx = await browser.newContext({ colorScheme: scheme });
    for (const p of pages) {
      const page = await bctx.newPage();
      await page.goto(ctx.base + p.path, { waitUntil: "load" });
      const aa = await new AxeBuilder({ page }).withTags(TAGS).analyze();
      for (const v of aa.violations) problems.push(`${scheme} ${p.path}: ${v.id} (${v.nodes.length}) ${v.help}`);
      let aaa = new AxeBuilder({ page }).withRules(["color-contrast-enhanced"]);
      for (const s of SECONDARY) aaa = aaa.exclude(s);
      for (const v of (await aaa.analyze()).violations) problems.push(`${scheme} ${p.path}: AAA contrast: ${v.nodes.slice(0, 2).map((n) => n.target.join(" ")).join(", ")}`);
      if (scheme === "light" && p.path !== "/404.html") {
        await page.keyboard.press("Tab");
        const first = await page.evaluate(() => ({ cls: document.activeElement.className, href: document.activeElement.getAttribute("href") }));
        if (!first.cls.includes("skip") || first.href !== "#main") problems.push(`${p.path}: first tab stop is not the skip link (${first.cls} ${first.href})`);
        const stops = await page.evaluate(async () => {
          const interactive = document.querySelectorAll("a[href], button, input, select, textarea, summary, [tabindex]:not([tabindex='-1'])").length;
          return { interactive };
        });
        let n = 0, prev = null, noOutline = [];
        for (let i = 0; i < stops.interactive + 2; i++) {
          const cur = await page.evaluate(() => { const e = document.activeElement; if (!e || e === document.body) return null; const cs = getComputedStyle(e); return { tag: e.tagName, text: (e.textContent || "").trim().slice(0, 20), outline: cs.outlineStyle !== "none" && parseFloat(cs.outlineWidth) > 0, boxShadow: cs.boxShadow !== "none" }; });
          if (!cur || JSON.stringify(cur) === JSON.stringify(prev)) break;
          n++; prev = cur;
          if (!cur.outline && !cur.boxShadow) noOutline.push(`${cur.tag} ${cur.text}`);
          await page.keyboard.press("Tab");
        }
        if (n < stops.interactive) problems.push(`${p.path}: ${n} tab stops for ${stops.interactive} interactive elements`);
        if (noOutline.length) problems.push(`${p.path}: no visible focus on ${noOutline.slice(0, 2).join(", ")}`);
        const langLink = p.$(".switch a, a[hreflang]").first();
        if (langLink.length && !langLink.attr("lang")) problems.push(`${p.path}: language link lacks lang`);
      }
      await page.close();
    }
    await bctx.close();
  }
  const check = ctx.mode === "post" ? `${name} (production)` : name;
  return [row(check, problems.length === 0, summary(`${pages.length} pages × light + dark: WCAG 2.2 AA 0 violations; AAA contrast outside ${SECONDARY.length} secondary selectors; skip link, focus, keyboard`, problems), ctx, problems)];
}
```

Adjust the skip-link and language-link selectors to the real templates (`grep -n 'skip\|switch\|hreflang' templates/base.html`) before running; the check must read the real markup, not guess.

- [ ] **Step 4: Tests** — pass on the fixture (the fixture pages are real builds); fail when a page gets `<img src="/x.png">` without `alt` (axe `image-alt`), and when a `<p style="color:#888">` (AAA) is added. Nu test: a fixture page with an unclosed `<div>` → Nu error reported.

- [ ] **Step 5: Run on the real dist** — expect the first real findings here (row 3's focus outline, `tap-targets` come in Task 9, `heading-order`, …). Fix the CSS/templates when the finding is right (a `:focus-visible` outline is part of the design system, spec §9 "visible focus"); record every fix and every disabled rule.

- [ ] **Step 6: Commit** — `M2a: bench — HTML validity (Nu, html-validate) and accessibility (axe, both schemes) rows`, plus separate commits for any generator/CSS fix the rows demanded (`M2a: a11y — …`).

---

### Task 9: `lighthouse` (row 1), sharded

**Files:**
- Create: `bench/lib/lighthouse.mjs`, `bench/test/lighthouse.test.mjs`
- Modify: `bench/audit.mjs` (`MODULES.lighthouse` exists already)

**Interfaces:** `name = "Lighthouse"`, `modes = ["ci","post"]`; the row's `check` is `"Lighthouse [<form-factor> <i>/<n>]"` (or `"Lighthouse (production) [...]"`), with extra fields `pages` (count in this shard) and `worst` (lowest category score ×100) that `merge.mjs` folds. `ctx.formFactor` (`mobile`|`desktop`, default both) and `ctx.shard` (`i/n`) select the work.

- [ ] **Step 1: `lighthouse.mjs`**

```js
import lighthouse from "lighthouse";
import desktopConfig from "lighthouse/core/config/desktop-config.js";
import { launch } from "chrome-launcher";
import { chromium } from "playwright";
import { row } from "./row.mjs";
export const name = "Lighthouse";
export const modes = ["ci", "post"];
const CATS = ["performance", "accessibility", "best-practices", "seo"];
const median = (xs) => { const s = [...xs].sort((a, b) => a - b); return s[Math.floor(s.length / 2)]; };
// Row 1: every sitemap page (never /404.html), mobile + desktop, 3 runs, median per category, all four = 100. Never against a preview host.
export async function run(ctx) {
  const [i, n] = ctx.shard.split("/").map(Number);
  const all = ctx.pages.filter((p) => p.path !== "/404.html").map((p) => p.path);
  const mine = all.filter((_, k) => k % n === i - 1);
  const ffs = ctx.formFactor ? [ctx.formFactor] : ["mobile", "desktop"];
  const chrome = await launch({ chromePath: chromium.executablePath(), chromeFlags: ["--headless=new", "--no-sandbox", "--disable-gpu"] });
  const rows = [];
  try {
    for (const ff of ffs) {
      const problems = [], scores = [];
      let worst = 100;
      for (const path of mine) {
        const runs = [];
        for (let r = 0; r < 3; r++) {
          const res = await lighthouse(ctx.base + path, { port: chrome.port, output: "json", logLevel: "error", onlyCategories: CATS, formFactor: ff, screenEmulation: ff === "desktop" ? desktopConfig.settings.screenEmulation : undefined, throttling: ff === "desktop" ? desktopConfig.settings.throttling : undefined }, ff === "desktop" ? desktopConfig : undefined);
          runs.push(Object.fromEntries(CATS.map((c) => [c, Math.round((res.lhr.categories[c].score ?? 0) * 100)])));
          if (r === 0) for (const [id, a] of Object.entries(res.lhr.audits)) if (a.score !== null && a.score < 1 && a.scoreDisplayMode === "binary") scores.push(`${path} ${id}`);
        }
        const med = Object.fromEntries(CATS.map((c) => [c, median(runs.map((x) => x[c]))]));
        const failing = CATS.filter((c) => med[c] < 100);
        if (failing.length) problems.push(`${path}: ${failing.map((c) => `${c} ${med[c]}`).join(", ")}`);
        worst = Math.min(worst, ...Object.values(med));
      }
      const check = `${ctx.mode === "post" ? name + " (production)" : name} [${ff} ${i}/${n}]`;
      const value = problems.length ? problems.join("; ") : `${mine.length} pages ${ff}: 100/100/100/100`;
      rows.push({ ...row(check, problems.length === 0, value, ctx, { failingAudits: scores }), pages: mine.length, worst });
    }
  } finally { await chrome.kill(); }
  return rows;
}
```

Verify the exact import path of the desktop config in the installed Lighthouse version (`ls bench/node_modules/lighthouse/core/config/`); if the API differs (e.g. `lighthouse/core/config/desktop-config.js` vs `lighthouse/lighthouse-core/...`), use the installed one and record it.

- [ ] **Step 2: Test** — `lighthouse.test.mjs` runs one page of the fixture, `--shard 1/1`, form factor mobile only, and asserts a row with four scores is produced (not that they are 100 — the fixture is served without compression by the test server, which is fine for a smoke test). Mark it `{ timeout: 120000 }`.

- [ ] **Step 3: Run on the real dist through `site serve --static`** (gzip on): `node bench/audit.mjs http://127.0.0.1:8090 --only lighthouse --form-factor mobile --shard 1/2 --out /tmp/lh1.json` and `2/2`, then desktop; `node bench/merge.mjs --out /tmp/sc.json /tmp/lh*.json`. Expect 100/100/100/100. Anything under 100 is a real finding (`tap-targets`, `unsized-images`, `font-size`, `heading-order`): fix the generator/CSS/templates when the audit is right, record when it is not (e.g. `uses-text-compression` cannot fire with gzip on; `is-crawlable` must pass because the local `robots.txt` has no Disallow).

- [ ] **Step 4: Commit** — `M2a: bench — Lighthouse row, sharded by form factor and page slice, 3-run median` (+ separate commits for any fixes).

---

### Task 10: `fonts` (row 21) — CLS with delayed fonts, page height and paragraph lines web vs fallback

**Files:**
- Create: `bench/lib/fonts.mjs`, `bench/test/fonts.test.mjs`
- Modify: `bench/audit.mjs`

**Interfaces:** `name = "Font correctness"`, `modes = ["ci"]`.

- [ ] **Step 1: `fonts.mjs`** (the M1b landing harness, made a row):

```js
import { row, summary } from "./row.mjs";
export const name = "Font correctness";
export const modes = ["ci"];
// Row 21: layout-shift observer with /fonts/* delayed 600 ms → CLS 0; page height web vs metric-matched fallback ≤ 5 %;
// every paragraph within ±1 line. Glyph coverage (U+0218–021B, ă â î) is enforced by `site check` on the shipped faces.
export async function run(ctx) {
  const problems = [], measured = [];
  const browser = await ctx.browser();
  const pages = ctx.pages.filter((p) => p.path !== "/404.html");
  for (const vp of [{ width: 390, height: 844 }, { width: 1280, height: 800 }]) {
    const delayed = await browser.newContext({ viewport: vp });
    await delayed.route("**/fonts/**", async (r) => { await new Promise((z) => setTimeout(z, 600)); await r.continue(); });
    const blocked = await browser.newContext({ viewport: vp });
    await blocked.route("**/fonts/**", (r) => r.abort());
    for (const p of pages) {
      const a = await delayed.newPage();
      await a.addInitScript(() => { window.__cls = 0; new PerformanceObserver((l) => { for (const e of l.getEntries()) if (!e.hadRecentInput) window.__cls += e.value; }).observe({ type: "layout-shift", buffered: true }); });
      await a.goto(ctx.base + p.path, { waitUntil: "load" });
      await a.waitForTimeout(1500);
      const web = await a.evaluate(async () => { await document.fonts.ready; return { cls: +window.__cls.toFixed(4), h: document.documentElement.scrollHeight, lines: [...document.querySelectorAll("main p")].map((e) => Math.round(e.getBoundingClientRect().height / parseFloat(getComputedStyle(e).lineHeight))), loaded: [...document.fonts].some((f) => f.status === "loaded" && f.family.includes("Herinean")) }; });
      const b = await blocked.newPage();
      await b.goto(ctx.base + p.path, { waitUntil: "load" });
      const fb = await b.evaluate(() => ({ h: document.documentElement.scrollHeight, lines: [...document.querySelectorAll("main p")].map((e) => Math.round(e.getBoundingClientRect().height / parseFloat(getComputedStyle(e).lineHeight))) }));
      if (web.cls > 0) problems.push(`${vp.width} ${p.path}: CLS ${web.cls}`);
      if (!web.loaded) problems.push(`${vp.width} ${p.path}: web font not applied after 1.5 s`);
      const dh = Math.abs(web.h - fb.h) / fb.h;
      if (dh > 0.05) problems.push(`${vp.width} ${p.path}: height ${web.h} vs ${fb.h} (${(dh * 100).toFixed(1)} %)`);
      web.lines.forEach((l, k) => { if (fb.lines[k] !== undefined && Math.abs(l - fb.lines[k]) > 1) problems.push(`${vp.width} ${p.path}: paragraph ${k + 1} ${l} vs ${fb.lines[k]} lines`); });
      measured.push({ vp: vp.width, path: p.path, cls: web.cls, height: [web.h, fb.h] });
      await a.close(); await b.close();
    }
    await delayed.close(); await blocked.close();
  }
  const worstH = Math.max(...measured.map((m) => Math.abs(m.height[0] - m.height[1]) / m.height[1]));
  return [row(name, problems.length === 0, summary(`${pages.length} pages × 2 widths: CLS 0 with fonts delayed 600 ms; height Δ ≤ ${(worstH * 100).toFixed(1)} %; paragraphs within ±1 line; glyphs U+0218–021B enforced by site check`, problems), ctx, measured)];
}
```

- [ ] **Step 2: Test** — smoke on the fixture (fonts included in the fixture): a row is produced; CLS field present. `{ timeout: 120000 }`.
- [ ] **Step 3: Run on the real dist** — CLS 0 on 7 pages × 2 widths, height Δ ≤ 1.9 % (the M1b measurement).
- [ ] **Step 4: Commit** — `M2a: bench — font correctness row (CLS, height, lines) from the M1b harness`.

---

### Task 11: Post-only rows — `transport` (10), `observatory` (12), `dns` (13), `caching` (17); the h3 curl

**Files:**
- Create: `bench/lib/transport.mjs`, `bench/lib/observatory.mjs`, `bench/lib/dns.mjs`, `bench/lib/caching.mjs`, `bench/lib/tools.mjs`, `bench/tools.json`, `bench/test/post-rows.test.mjs`
- Modify: `bench/audit.mjs`

**Interfaces:** all `modes = ["post"]`; names `"Transport (production)"`, `"Mozilla HTTP Observatory (production)"`, `"DNS, mail, domains (production)"`, `"Caching (production)"`. `tools.mjs` exports `async function curlH3() → path` (the system curl if it lists HTTP3, else a pinned static build downloaded into `bench/.cache/bin/` and verified by sha256).

- [ ] **Step 1: `bench/tools.json`** — pin the current release of a static curl with HTTP/3 (for example the `stunnel/static-curl` GitHub releases: pick the newest, take `linux-x86_64` and `linux-aarch64` archives and their sha256 from the release's checksum file; verify the archive really lists `HTTP3` in `curl --version` before pinning):

```json
{
  "curl-h3": {
    "version": "<release tag>",
    "linux-x64": { "url": "<archive url>", "sha256": "<hex>", "member": "curl" },
    "linux-arm64": { "url": "<archive url>", "sha256": "<hex>", "member": "curl" }
  }
}
```

`tools.mjs`: download to `bench/.cache/bin/curl-h3-<version>.tar.xz`, verify sha256, extract the member, `chmod 755`, return the path; if the system `curl --version` contains `HTTP3`, return `"curl"`. Throw on checksum mismatch.

- [ ] **Step 2: `transport.mjs`**

```js
import { row, summary } from "./row.mjs";
import { sh } from "./sh.mjs";
import { curlH3 } from "./tools.mjs";
export const name = "Transport (production)";
export const modes = ["post"];
// Row 10: TLS 1.3-only, HTTP/3 negotiated (an h3-capable curl; Alt-Svc alone is not proof), IPv6 AAAA present; 0-RTT is reported as configured off (infra/settings.tf) — a runner cannot provoke early data.
export async function run(ctx) {
  const problems = [];
  const host = new URL(ctx.base).host;
  const t12 = sh("openssl", ["s_client", "-connect", `${host}:443`, "-servername", host, "-tls1_2"], { input: "" });
  if (!/alert protocol version|wrong version number|handshake failure/.test(t12.err + t12.out)) problems.push("TLS 1.2 was accepted");
  const t13 = sh("openssl", ["s_client", "-connect", `${host}:443`, "-servername", host, "-tls1_3"], { input: "" });
  if (!/TLSv1\.3/.test(t13.out)) problems.push("TLS 1.3 not negotiated");
  const curl = await curlH3();
  const h3 = sh(curl, ["-sS", "--http3-only", "-o", "/dev/null", "-w", "%{http_version}", "--max-time", "15", ctx.base + "/"]);
  if (h3.out.trim() !== "3") problems.push(`HTTP/3: got '${h3.out.trim() || h3.err.trim()}'`);
  const aaaa = sh("dig", ["+short", "AAAA", host, "@1.1.1.1"]);
  if (!aaaa.out.trim()) problems.push("no AAAA record");
  return [row(name, problems.length === 0, summary("TLS 1.3 only (1.2 refused); HTTP/3 negotiated; AAAA present; 0-RTT configured off (infra/settings.tf), not measured", problems), ctx, problems)];
}
```

- [ ] **Step 3: `observatory.mjs`** — run the package's CLI (or its library) for the host; parse the grade; pass iff `A+`; value `A+ (score N)` with the link `https://developer.mozilla.org/en-US/observatory/analyze?host=<host>` and `link_text` "Observatory" (override `ctx.link` for this row — a row's `link` is "verify it yourself"). Record the exact CLI name/flags.

- [ ] **Step 4: `dns.mjs`**

```js
import { row } from "./row.mjs";
import { sh } from "./sh.mjs";
export const name = "DNS, mail, domains (production)";
export const modes = ["post"];
// Row 13: the M0 edge suite (dig/HTTP sections; the API section needs zone scope the CI token does not have) is the one truth; this row wraps it.
export async function run(ctx) {
  const r = sh("scripts/verify-edge.sh", ["--ci"], { env: { ...process.env, PATH: process.env.PATH } });
  const m = /(\d+) passed, (\d+) failed/.exec(r.out);
  const fails = r.out.split("\n").filter((l) => l.includes("  FAIL "));
  const pass = r.code === 0 && m && m[2] === "0";
  return [row(name, pass, pass ? `verify-edge.sh --ci: ${m[1]} checks (DNSSEC, CAA, MX/SPF/DKIM/DMARC/MTA-STS/TLS-RPT ×4 zones, redirects, apex headers)` : `verify-edge.sh --ci: ${fails.length} failed: ${fails.slice(0, 3).map((l) => l.trim()).join("; ")}`, ctx, r.out)];
}
```

(Domain expiry via RDAP is the weekly job's, M2b.)

- [ ] **Step 5: `caching.mjs`** — fetch `/` (expect `cache-control: public, max-age=0, must-revalidate` and an `etag`, and a second request with `If-None-Match` → 304), the preloaded font and the OG image (`max-age=31536000, immutable`), `/feed.xml` (`max-age=300`, `application/rss+xml; charset=utf-8`), `/.well-known/security.txt` (`text/plain; charset=utf-8`), `/colophon/` (`etag` of the composed form `W/"<hex>-<8 hex>"`, and 304 on match).

- [ ] **Step 6: Tests** — `dns`: run against a stub script (`bench/test/fixtures/verify-edge-stub.sh` printing `3 passed, 1 failed` and a FAIL line) by overriding the script path via `ctx.verifyEdge` (add that optional field; default `scripts/verify-edge.sh`); `caching`/`transport`: unit-test only the parsers (the pass/fail decisions from canned `sh` output) by exporting the decision functions.
- [ ] **Step 7: Live check against the placeholder** (read-only, allowed): `node bench/audit.mjs https://herinean.com --mode post --only checks --out /tmp/post.json` — transport, dns, headers pass today on the placeholder; observatory A+; caching and html/a11y on `/colophon/` fail (the placeholder has no colophon) — expected, recorded; this proves the post modules run.
- [ ] **Step 8: Commit** — `M2a: bench — post-deploy rows: transport, Observatory, DNS via verify-edge, caching; pinned h3 curl`.

---

### Task 12: `scripts/bench.sh` — the local escape hatch; first full local run

**Files:**
- Create: `scripts/bench.sh`
- Modify: `bench/README.md`, `RUNBOOK.md` (a line under Verification; the CI section comes in Task 14)

- [ ] **Step 1: `scripts/bench.sh`**

```bash
#!/usr/bin/env bash
# Runs the audit bench locally against a built dist/ (or a URL). The colophon shows CI's numbers; this is for debugging a red row.
# usage: scripts/bench.sh [BASE] [--post]
set -euo pipefail
cd "$(dirname "$0")/.."
export PATH="$HOME/.local/bin:$PATH"
BASE=${1:-}; MODE=ci; [ "${2:-}" = "--post" ] && MODE=post
[ -d bench/node_modules ] || (cd bench && npm ci --silent)
(cd bench && npx playwright install chromium >/dev/null)
command -v java >/dev/null || echo "bench: no java — the Nu half of the HTML row will report 'Nu did not run' (sudo apt install -y default-jre-headless)" >&2
mkdir -p .cache/bench
if [ -z "$BASE" ]; then
  go build -tags nodynamic -o .cache/site ./cmd/site
  .cache/site build
  .cache/site check --dist
  .cache/site serve --static --port 8089 >/dev/null 2>&1 &
  SRV=$!; trap 'kill $SRV 2>/dev/null || true' EXIT
  for _ in $(seq 1 50); do curl -sf -o /dev/null http://127.0.0.1:8089/ && break; sleep 0.2; done
  BASE=http://127.0.0.1:8089
fi
node bench/audit.mjs "$BASE" --mode "$MODE" --dist dist --out .cache/bench/rows.json
node bench/merge.mjs --out .cache/bench/scorecard.json --build "$(git rev-parse --short HEAD)" --build-url "https://github.com/raduherinean/herinean.com/commit/$(git rev-parse HEAD)" --no-gate .cache/bench/rows.json
echo "rows: .cache/bench/scorecard.json (detail: .cache/bench/rows.json.detail.json)"
```

- [ ] **Step 2: First full local run** — `scripts/bench.sh` on the real build: every CI row `ok`. Then `.cache/site scorecard --in .cache/bench/scorecard.json --manual data/scorecard-manual.yaml --out .cache/bench/scorecard.html` and validate the fragment with Nu (wrapped as in Task 4's publish step) → valid. Timing per module into the ledger (the budget model assumed ≤ 2 min for `checks`).
- [ ] **Step 3: Docs** — `bench/README.md` gets the local section; RUNBOOK Verification adds `scripts/bench.sh` in one line.
- [ ] **Step 4: Commit** — `M2a: bench — scripts/bench.sh runs the same audit locally` (WHY: decision 1 in the design — CI's numbers, local debugging).

---

### Task 13: Workflows — `audit.yml`, `ci.yml`, `dependabot.yml`, PR comment, README badge

**Files:**
- Create: `.github/workflows/audit.yml`, `.github/workflows/ci.yml`, `.github/dependabot.yml`, `scripts/pr-comment.sh`
- Modify: `README.md` (badge + sentence)

**Interfaces:**
- `audit.yml` `workflow_call` inputs: `base` (string), `mode` (`ci`|`post`), `shards` (string, JSON array of `"i/n"`), `artifact` (string, the `dist` artifact name; empty in post mode), `link` (string, the run URL); outputs: `pass` (`"true"`/`"false"`); uploads `rows-<job>-<matrix>.json` artifacts and `scorecard-<mode>` (the merged JSON + summary).
- `ci.yml` jobs: `build`, `audit` (uses audit.yml), `preview`, `deploy`, `post`, `publish`.

- [ ] **Step 1: Resolve action SHAs** — for each `actions/checkout`, `actions/setup-go`, `actions/setup-node`, `actions/setup-java`, `actions/cache`, `actions/upload-artifact`, `actions/download-artifact`: `gh api repos/actions/<name>/git/ref/tags/<latest vX.Y.Z> --jq .object.sha` (dereference annotated tags with `git/tags/<sha>` if the object type is `tag`). Write `uses: actions/checkout@<40-hex> # vX.Y.Z`.

- [ ] **Step 2: `.github/workflows/audit.yml`**

```yaml
name: audit
on:
  workflow_call:
    inputs:
      base: { type: string, required: true }
      mode: { type: string, required: true }        # ci | post
      shards: { type: string, required: true }      # JSON array, e.g. ["1/2","2/2"]
      artifact: { type: string, default: "" }       # dist artifact to serve (ci mode)
      link: { type: string, required: true }        # the run URL, printed as "verify it yourself"
    outputs:
      pass: { value: ${{ jobs.merge.outputs.pass }} }
permissions: { contents: read }
env: { NODE_OPTIONS: --max-old-space-size=4096 }
jobs:
  lighthouse:
    runs-on: ubuntu-latest
    timeout-minutes: 15
    strategy:
      fail-fast: false
      matrix: { form-factor: [mobile, desktop], shard: ${{ fromJSON(inputs.shards) }} }
    steps:
      - uses: actions/checkout@<sha> # vX
      - uses: actions/setup-node@<sha> # vX
        with: { node-version-file: .node-version, cache: npm, cache-dependency-path: bench/package-lock.json }
      - run: cd bench && npm ci
      - uses: actions/cache@<sha> # vX
        with: { path: ~/.cache/ms-playwright, key: playwright-${{ runner.os }}-${{ hashFiles('bench/package-lock.json') }} }
      - run: cd bench && npx playwright install --with-deps chromium
      - if: inputs.mode == 'ci'
        uses: actions/download-artifact@<sha> # vX
        with: { name: ${{ inputs.artifact }}, path: . }
      - if: inputs.mode == 'ci'
        run: |
          chmod +x site && ./site serve --static --port 8080 &
          for i in $(seq 1 50); do curl -sf -o /dev/null http://127.0.0.1:8080/ && break; sleep 0.2; done
      - run: node bench/audit.mjs "${{ inputs.base }}" --mode ${{ inputs.mode }} --only lighthouse --form-factor ${{ matrix.form-factor }} --shard ${{ matrix.shard }} --link "${{ inputs.link }}" --out rows-lighthouse.json
      - uses: actions/upload-artifact@<sha> # vX
        with: { name: rows-${{ inputs.mode }}-lighthouse-${{ matrix.form-factor }}-${{ strategy.job-index }}, path: rows-lighthouse.json* }
  checks:
    runs-on: ubuntu-latest
    timeout-minutes: 15
    steps:
      - uses: actions/checkout@<sha> # vX
      - uses: actions/setup-node@<sha> # vX
        with: { node-version-file: .node-version, cache: npm, cache-dependency-path: bench/package-lock.json }
      - uses: actions/setup-java@<sha> # vX
        with: { distribution: temurin, java-version: "21" }
      - run: cd bench && npm ci
      - uses: actions/cache@<sha> # vX
        with: { path: ~/.cache/ms-playwright, key: playwright-${{ runner.os }}-${{ hashFiles('bench/package-lock.json') }} }
      - run: cd bench && npx playwright install --with-deps chromium
      - if: inputs.mode == 'ci'
        uses: actions/download-artifact@<sha> # vX
        with: { name: ${{ inputs.artifact }}, path: . }
      - if: inputs.mode == 'ci'
        run: |
          chmod +x site && ./site serve --static --port 8080 &
          for i in $(seq 1 50); do curl -sf -o /dev/null http://127.0.0.1:8080/ && break; sleep 0.2; done
      - run: node bench/audit.mjs "${{ inputs.base }}" --mode ${{ inputs.mode }} --only checks --dist dist --link "${{ inputs.link }}" --out rows-checks.json
      - uses: actions/upload-artifact@<sha> # vX
        with: { name: rows-${{ inputs.mode }}-checks, path: rows-checks.json* }
  merge:
    needs: [lighthouse, checks]
    if: always()
    runs-on: ubuntu-latest
    timeout-minutes: 5
    outputs: { pass: ${{ steps.gate.outputs.pass }} }
    steps:
      - uses: actions/checkout@<sha> # vX
      - uses: actions/setup-node@<sha> # vX
        with: { node-version-file: .node-version, cache: npm, cache-dependency-path: bench/package-lock.json }
      - run: cd bench && npm ci
      - uses: actions/download-artifact@<sha> # vX
        with: { pattern: rows-${{ inputs.mode }}-*, path: rows, merge-multiple: true }
      - id: gate
        run: |
          set +e
          node bench/merge.mjs --out scorecard-${{ inputs.mode }}.json --build "${GITHUB_SHA::7}" --build-url "$GITHUB_SERVER_URL/$GITHUB_REPOSITORY/commit/$GITHUB_SHA" --summary "$GITHUB_STEP_SUMMARY" rows/rows-*.json
          rc=$?; echo "pass=$([ $rc -eq 0 ] && echo true || echo false)" >> "$GITHUB_OUTPUT"; exit $rc
      - if: always()
        uses: actions/upload-artifact@<sha> # vX
        with: { name: scorecard-${{ inputs.mode }}, path: scorecard-${{ inputs.mode }}.json }
```

(In post mode the merger runs without `--build`; the `Audited build` row already sits in the CI scorecard — `publish` merges both files.)

- [ ] **Step 3: `.github/workflows/ci.yml`**

```yaml
name: ci
on:
  pull_request: { branches: [main] }
  push: { branches: [main] }
  workflow_dispatch:
permissions: { contents: read }
concurrency:
  group: ${{ github.event_name == 'pull_request' && format('pr-{0}', github.event.pull_request.number) || 'main' }}
  cancel-in-progress: ${{ github.event_name == 'pull_request' }}
env: { GOTOOLCHAIN: local, GOFLAGS: -tags=nodynamic }
jobs:
  build:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    outputs:
      launched: ${{ steps.meta.outputs.launched }}
      shards: ${{ steps.meta.outputs.shards }}
    steps:
      - uses: actions/checkout@<sha> # vX
        with: { fetch-depth: 0 }
      - uses: actions/setup-go@<sha> # vX
        with: { go-version-file: go.mod, cache: true }
      - uses: actions/setup-node@<sha> # vX
        with: { node-version-file: .node-version }
      - run: test -z "$(gofmt -l .)" || { gofmt -l .; exit 1; }
      - run: go vet ./...
      - run: go tool staticcheck ./...
      - run: go tool actionlint
      - run: go test ./...
      - run: cd worker && node --test
      - run: go build -trimpath -o site ./cmd/site
      - run: ./site check
      - run: ./site build && scripts/dist-hash.sh > dist.sha256 && rm -rf dist && ./site build && scripts/dist-hash.sh | diff - dist.sha256
      - run: ./site check --dist
      - id: meta
        run: |
          launched=$(grep -E '^launched: *[0-9]{4}-[0-9]{2}-[0-9]{2}' site.yaml >/dev/null && echo true || echo false)
          n=$(grep -c '<loc>' dist/sitemap.xml); k=$(( (n + 3) / 4 ))
          shards=$(seq 1 "$k" | sed "s#^#\"#; s#\$#/$k\"#" | paste -sd, | sed 's/^/[/; s/$/]/')
          echo "launched=$launched" >> "$GITHUB_OUTPUT"; echo "shards=$shards" >> "$GITHUB_OUTPUT"
          echo "launched: $launched; $n pages → $k Lighthouse shards" >> "$GITHUB_STEP_SUMMARY"
      - uses: actions/upload-artifact@<sha> # vX
        with: { name: dist-${{ github.sha }}, path: "dist/\nsite\ndist.sha256", if-no-files-found: error }
  audit:
    needs: build
    uses: ./.github/workflows/audit.yml
    with:
      base: http://127.0.0.1:8080
      mode: ci
      shards: ${{ needs.build.outputs.shards }}
      artifact: dist-${{ github.sha }}
      link: ${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}
  preview:
    needs: build
    if: github.event_name == 'pull_request' && github.event.pull_request.head.repo.full_name == github.repository && github.actor != 'dependabot[bot]'
    runs-on: ubuntu-latest
    timeout-minutes: 10
    permissions: { contents: read, pull-requests: write }
    env: { CLOUDFLARE_API_TOKEN: "${{ secrets.CLOUDFLARE_API_TOKEN }}", CLOUDFLARE_ACCOUNT_ID: "${{ secrets.CLOUDFLARE_ACCOUNT_ID }}", GH_TOKEN: "${{ github.token }}" }
    steps:
      - uses: actions/checkout@<sha> # vX
      - uses: actions/setup-node@<sha> # vX
        with: { node-version-file: .node-version, cache: npm }
      - uses: actions/download-artifact@<sha> # vX
        with: { name: dist-${{ github.sha }}, path: . }
      - if: env.CLOUDFLARE_API_TOKEN == ''
        run: echo "preview skipped — CLOUDFLARE_API_TOKEN is not set (RUNBOOK, Accounts and secrets)" >> "$GITHUB_STEP_SUMMARY"
      - if: env.CLOUDFLARE_API_TOKEN != ''
        run: |
          npm ci
          out=$(npx wrangler versions upload 2>&1 | tee /dev/stderr)
          url=$(printf '%s\n' "$out" | grep -Eo 'https://[a-z0-9-]+\.[a-z0-9-]+\.workers\.dev' | head -1)
          [ -n "$url" ] || { echo "no preview URL"; exit 1; }
          echo "preview: $url" >> "$GITHUB_STEP_SUMMARY"
          scripts/verify-preview.sh "$url"
          printf '<!-- herinean-ci -->\n**Preview:** %s (noindex; readable by anyone with the link)\n\n_Scorecard follows when the audit finishes._\n' "$url" > comment.md
          scripts/pr-comment.sh "${{ github.event.pull_request.number }}" comment.md
  comment:
    needs: [audit, preview]
    if: always() && github.event_name == 'pull_request' && needs.preview.result != 'skipped'
    runs-on: ubuntu-latest
    timeout-minutes: 5
    permissions: { contents: read, pull-requests: write }
    env: { GH_TOKEN: "${{ github.token }}" }
    steps:
      - uses: actions/checkout@<sha> # vX
      - uses: actions/download-artifact@<sha> # vX
        with: { name: scorecard-ci, path: . }
        continue-on-error: true
      - run: |
          body=$(scripts/pr-comment.sh "${{ github.event.pull_request.number }}" --get | sed '/_Scorecard follows/,$d')
          { printf '%s\n\n**Audit (%s):**\n\n' "$body" "${GITHUB_SHA::7}"; node bench/merge.mjs --no-gate --out /dev/null --summary /dev/stdout scorecard-ci.json 2>/dev/null || echo "audit did not produce a scorecard"; } > comment.md
          scripts/pr-comment.sh "${{ github.event.pull_request.number }}" comment.md
  deploy:
    needs: [build, audit]
    if: github.event_name == 'push' && needs.build.outputs.launched == 'true' && needs.audit.outputs.pass == 'true'
    runs-on: ubuntu-latest
    timeout-minutes: 10
    environment: production
    env: { CLOUDFLARE_API_TOKEN: "${{ secrets.CLOUDFLARE_API_TOKEN }}", CLOUDFLARE_ACCOUNT_ID: "${{ secrets.CLOUDFLARE_ACCOUNT_ID }}" }
    outputs: { previous: ${{ steps.deploy.outputs.previous }}, current: ${{ steps.deploy.outputs.current }} }
    steps:
      - uses: actions/checkout@<sha> # vX
      - uses: actions/setup-node@<sha> # vX
        with: { node-version-file: .node-version, cache: npm }
      - uses: actions/setup-java@<sha> # vX
        with: { distribution: temurin, java-version: "21" }
      - uses: actions/download-artifact@<sha> # vX
        with: { name: dist-${{ github.sha }}, path: . }
      - uses: actions/download-artifact@<sha> # vX
        with: { name: scorecard-ci, path: . }
      - run: scripts/dist-hash.sh | diff - dist.sha256
      - run: npm ci && (cd bench && npm ci)
      - name: stash the live scorecard
        run: |
          mkdir -p stash
          scripts/kv.sh get scorecard > stash/scorecard.html || : > stash/scorecard.html
          scripts/kv.sh get scorecard --metadata > stash/scorecard.meta || echo '{}' > stash/scorecard.meta
          scripts/kv.sh get scorecard.json > stash/scorecard.json || echo '[]' > stash/scorecard.json
      - uses: actions/upload-artifact@<sha> # vX
        with: { name: kv-stash-${{ github.sha }}, path: stash/ }
      - run: chmod +x site && cp scorecard-ci.json scorecard.json && scripts/scorecard-publish.sh scorecard.json
      - id: deploy
        run: |
          previous=$(npx wrangler deployments list --json | node -e 'let s="";process.stdin.on("data",d=>s+=d).on("end",()=>{const d=JSON.parse(s);console.log(d[0].versions[0].version_id)})')
          echo "previous=$previous" >> "$GITHUB_OUTPUT"
          out=$(npx wrangler deploy 2>&1 | tee /dev/stderr)
          current=$(printf '%s\n' "$out" | grep -Eo 'Version ID: [0-9a-f-]+' | head -1 | cut -d' ' -f3)
          echo "current=$current" >> "$GITHUB_OUTPUT"
          echo "deployed $current (previous $previous)" >> "$GITHUB_STEP_SUMMARY"
      - if: failure()
        name: restore the scorecard the old build had
        run: |
          [ -s stash/scorecard.html ] && scripts/kv.sh put scorecard stash/scorecard.html --metadata "$(cat stash/scorecard.meta)" || true
          [ -s stash/scorecard.json ] && scripts/kv.sh put scorecard.json stash/scorecard.json || true
  post:
    needs: [build, deploy]
    if: always() && needs.build.outputs.launched == 'true' && (needs.deploy.result == 'success' || github.event_name == 'workflow_dispatch')
    runs-on: ubuntu-latest
    timeout-minutes: 10
    environment: production
    env: { CLOUDFLARE_API_TOKEN: "${{ secrets.CLOUDFLARE_API_TOKEN }}", CLOUDFLARE_ACCOUNT_ID: "${{ secrets.CLOUDFLARE_ACCOUNT_ID }}" }
    outputs: { verified: ${{ steps.tier1.outputs.verified }} }
    steps:
      - uses: actions/checkout@<sha> # vX
      - uses: actions/setup-node@<sha> # vX
        with: { node-version-file: .node-version, cache: npm }
      - run: npm ci
      - id: tier1
        run: |
          if scripts/verify-preview.sh https://herinean.com --prod; then echo "verified=true" >> "$GITHUB_OUTPUT"; else echo "verified=false" >> "$GITHUB_OUTPUT"; exit 1; fi
      - if: failure() && needs.deploy.result == 'success'
        name: roll back
        run: |
          npx wrangler rollback "${{ needs.deploy.outputs.previous }}" --yes --message "post-deploy verification failed: $GITHUB_SERVER_URL/$GITHUB_REPOSITORY/actions/runs/$GITHUB_RUN_ID"
          echo "rolled back to ${{ needs.deploy.outputs.previous }}" >> "$GITHUB_STEP_SUMMARY"
      - if: failure() && needs.deploy.result == 'success'
        uses: actions/download-artifact@<sha> # vX
        with: { name: kv-stash-${{ github.sha }}, path: stash }
      - if: failure() && needs.deploy.result == 'success'
        run: |
          [ -s stash/scorecard.html ] && scripts/kv.sh put scorecard stash/scorecard.html --metadata "$(cat stash/scorecard.meta)" || true
          [ -s stash/scorecard.json ] && scripts/kv.sh put scorecard.json stash/scorecard.json || true
  post-rows:
    needs: [build, post]
    if: always() && needs.post.outputs.verified == 'true'
    uses: ./.github/workflows/audit.yml
    with:
      base: https://herinean.com
      mode: post
      shards: ${{ needs.build.outputs.shards }}
      link: ${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}
  publish:
    needs: [post, post-rows]
    if: always() && needs.post.outputs.verified == 'true'
    runs-on: ubuntu-latest
    timeout-minutes: 5
    environment: production
    env: { CLOUDFLARE_API_TOKEN: "${{ secrets.CLOUDFLARE_API_TOKEN }}", CLOUDFLARE_ACCOUNT_ID: "${{ secrets.CLOUDFLARE_ACCOUNT_ID }}" }
    steps:
      - uses: actions/checkout@<sha> # vX
      - uses: actions/setup-node@<sha> # vX
        with: { node-version-file: .node-version, cache: npm }
      - uses: actions/setup-java@<sha> # vX
        with: { distribution: temurin, java-version: "21" }
      - uses: actions/download-artifact@<sha> # vX
        with: { name: dist-${{ github.sha }}, path: . }
      - uses: actions/download-artifact@<sha> # vX
        with: { pattern: scorecard-*, path: sc, merge-multiple: true }
      - run: npm ci && (cd bench && npm ci)
      - run: |
          chmod +x site
          node bench/merge.mjs --no-gate --out scorecard.json sc/scorecard-ci.json sc/scorecard-post.json
          scripts/scorecard-publish.sh scorecard.json
```

Pre-launch, `main` publishes CI rows: add a job `publish-ci` (`needs: [build, audit]`, `if: github.event_name != 'pull_request' && needs.build.outputs.launched != 'true' && needs.audit.outputs.pass == 'true'`, `environment: production`, the same secrets guard as `preview`; steps: checkout, setup-node, setup-java (Temurin 21, for the fragment's Nu check), download `dist-<sha>` and `scorecard-ci`, `npm ci && (cd bench && npm ci)`, `chmod +x site && scripts/scorecard-publish.sh scorecard-ci.json`), so the preview colophon carries real rows before launch. Keep the job skipped with a summary line when the secret is absent.

The `wrangler deployments list --json` / `Version ID:` parsing must be verified against wrangler 4.135's real output at execution (`npx wrangler deployments list --json` locally with the operator file — read-only); record the exact fields.

- [ ] **Step 4: `scripts/pr-comment.sh`** — one sticky comment per PR, found by the `<!-- herinean-ci -->` marker:

```bash
#!/usr/bin/env bash
# One sticky CI comment per pull request (marker <!-- herinean-ci -->): create or update. usage: pr-comment.sh PR FILE | pr-comment.sh PR --get
set -euo pipefail
pr=${1:?pr}; src=${2:?file or --get}
repo=${GITHUB_REPOSITORY:?}
id=$(gh api "repos/$repo/issues/$pr/comments" --paginate --jq '.[] | select(.body | startswith("<!-- herinean-ci -->")) | .id' | head -1)
if [ "$src" = "--get" ]; then [ -n "$id" ] && gh api "repos/$repo/issues/comments/$id" --jq .body || echo "<!-- herinean-ci -->"; exit 0; fi
if [ -n "$id" ]; then gh api -X PATCH "repos/$repo/issues/comments/$id" -F body=@"$src" >/dev/null
else gh api -X POST "repos/$repo/issues/$pr/comments" -F body=@"$src" >/dev/null; fi
```

- [ ] **Step 5: `.github/dependabot.yml`**

```yaml
version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    schedule: { interval: weekly }
    groups: { go: { patterns: ["*"], update-types: [minor, patch] } }
  - package-ecosystem: github-actions
    directory: /
    schedule: { interval: weekly }
  - package-ecosystem: npm
    directory: /
    schedule: { interval: weekly }
  - package-ecosystem: npm
    directory: /bench
    schedule: { interval: weekly }
    groups: { bench: { patterns: ["*"], update-types: [minor, patch] } }
```

- [ ] **Step 6: Lint** — `go tool actionlint` clean (it validates `workflow_call` inputs, expressions, shell). Fix everything it reports.

- [ ] **Step 7: Try `act` locally** (evidence only): `docker run --rm -v "$PWD":/w -w /w ghcr.io/catthehacker/ubuntu:act-latest bash -c 'true'` to see whether the image runs on arm64; if `act` (binary, pinned release) runs the `build` job, record the result; if not, record why and move on — not a criterion.

- [ ] **Step 8: README** — badge `[![ci](https://github.com/raduherinean/herinean.com/actions/workflows/ci.yml/badge.svg)](https://github.com/raduherinean/herinean.com/actions/workflows/ci.yml)` under the title; replace "The CI that will audit every commit against a public scorecard is designed in the spec (§7) and is the next milestone, M2." with "Every pull request is audited against the public scorecard by `.github/workflows/ci.yml` (spec §7); merges to `main` deploy once the site is launched (`launched:` in `site.yaml`)." Update the Status line: "M2a (CI, bench, scorecard pipeline) built; production stays on the placeholder until launch (spec §11)."

- [ ] **Step 9: Commit** — `M2a: ci — build, sharded audit, preview, deploy with rollback, post rows, publish; Dependabot` (WHY body: the two tiers, the launch marker, why only `actions/*`).

---

### Task 14: Docs — RUNBOOK "CI", ADR-0015, spec amendments, doc-comment drift

**Files:**
- Create: `docs/adr/0015-ci-deploys-launch-is-a-commit.md`
- Modify: `RUNBOOK.md`, `docs/specs/2026-09-17-herinean-com-design.md`, `worker/index.js` (if not done in Task 3), `scripts/fontface/main.go` (line 4 comment), `README.md` (if anything left)

- [ ] **Step 1: RUNBOOK** — new section `## CI` after "Worker":

```markdown
## CI
- `.github/workflows/ci.yml` on every pull request and push to `main`: `build` (gofmt, vet, staticcheck, actionlint, tests, `site check`, two builds compared, `check --dist`) → `audit` (`.github/workflows/audit.yml`: a Lighthouse matrix of form factor × shard, one `checks` job for every other row, a `merge` job that writes `scorecard.json` and fails on any red row) → on pull requests `preview` (`wrangler versions upload`, one sticky comment with the URL, then the scorecard table; the preview is `noindex` but readable by anyone with the link) → on `main`, once `site.yaml` has `launched:`, `deploy` → `post` → `post-rows` → `publish`.
- Secrets (repository level, one copy): `CLOUDFLARE_API_TOKEN` — a token scoped to this Worker, its KV namespace and Account Analytics read, nothing zone-wide — and `CLOUDFLARE_ACCOUNT_ID`. Without them `preview`, `deploy` and `publish` skip with a line in the job summary; `build` and `audit` need nothing.
- Reading a red row: the job summary has the table; the `rows-*` artifacts carry `*.detail.json` with every problem; `scripts/bench.sh` runs the same modules locally against a built `dist/` (Java for Nu; Playwright Chromium).
- `deploy`: verifies the artifact against `dist.sha256`, stashes the live KV scorecard as an artifact, publishes the CI rows, records the live version id, `wrangler deploy`. `post` tier 1 runs `scripts/verify-preview.sh https://herinean.com --prod`; on failure it rolls back to the recorded version, restores the stash and fails the run (GitHub mails the committer). If Cloudflare refuses the rollback (bindings changed, or more than ten versions back) the run fails with the error — re-deploy the previous commit by hand: `git checkout <sha> && scripts/deploy.sh --yes-production`. Tier 2 (`post-rows`) measures the production rows and only reports; `publish` writes CI + post rows to KV. Post rows are absent for the minutes between a deploy and its `post`.
- A run killed between the publish and `wrangler deploy` leaves the new rows on the old build: the `Audited build` row shows it; re-run the workflow (`workflow_dispatch` re-verifies production and republishes without deploying).
- A manual `scripts/deploy.sh --yes-production` is unaudited by definition and says so on the colophon (`Audited build … manual deploy, not audited by CI`) until the next CI publish.
- Launch day (spec §11): set `launched: YYYY-MM-DD` in `site.yaml` in one commit (`Launch: herinean.com`), open the PR, merge; CI deploys and fills the colophon. Then: HSTS preload submission, Search Console and Bing verification, the URL into LinkedIn; re-run internet.nl, SSL Labs and securityheaders.com and replace the `measured: placeholder` rows in `data/scorecard-manual.yaml` with `measured: production`; delete the manual Observatory row (the post row replaces it); run the W3C Feed Validator once and add its row.
- Pull requests from forks and from Dependabot get `build` and `audit` only (no secrets). A bench bump that turns a row red is the signal to fix or pin — never to loosen the row.
- Reproducibility: `build` builds `dist/` twice and diffs `scripts/dist-hash.sh`; one-off cross-architecture check: download `dist.sha256` from a green run of a commit, build the same commit locally with `SOURCE_DATE_EPOCH=$(git log -1 --format=%ct)`, `scripts/dist-hash.sh | diff - dist.sha256`. Result recorded here: (pending the first push).
- Actions are pinned by commit SHA (Dependabot bumps them); `GOTOOLCHAIN=local` with `go-version-file: go.mod` — no silent toolchain download; Go tools (`staticcheck`, `actionlint`) are `tool` directives in `go.mod`.
```

Update the existing lines: Worker section (`npx "$WRANGLER"` → `npx wrangler`; "after M2, CI deploys" → "CI deploys `main` once launched; this is the manual fallback"; rollback line: `npx wrangler rollback <version-id>`), Generator section (`serve --static` line; `ci.yml (M2)` → present tense; `GOTOOLCHAIN=go1.27.1` → `GOTOOLCHAIN=local` + version file; the arm64/amd64 line → points to the CI section), Analytics ("Weekly snapshots … are M2" → "M2b"), Verification (`scripts/bench.sh`), line 34's parenthetical (robots.txt is typed by `_headers`), Incidents (rollback pointer). Every remaining "M2" mention resolved (`grep -n 'M2' RUNBOOK.md`).

- [ ] **Step 2: ADR-0015**

```markdown
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
```

Add seed 15 to spec §12: `15. CI is the deployer; the launch is a commit; post-deploy failures are two tiers (deploy verification rolls back, post rows only report) — ADR-0015.`

- [ ] **Step 3: Spec amendments (one commit)** — §8 step 5: "Squash-merge: `Publish: <title>`. Live a few minutes after the merge, once the audit is green (ADR-0015)."; §7 audit tools sentence: lychee → "(lychee, in the weekly job)"; §3 row 10 "Enforced" cell: "post (`openssl` TLS version, `curl --http3`; 0-RTT is configuration, `infra/settings.tf`, not measured); SSL Labs ext"; §3 row 5 "Enforced" cell: "CI (structure, full text, self link, JSON Feed fields); W3C Feed Validator ext"; §13: `.github/workflows/  ci.yml, audit.yml (M2a); verify.yml, codeql.yml (M2b)`. Commit `Spec: what CI can measure and when a merge is live` with a WHY body.

- [ ] **Step 4: Doc drift** — `scripts/fontface/main.go` line 4 usage comment: name the current fallback family (read the shipped `assets/fonts/web/fallback.css` for the exact name); `worker/index.js` header/`charset` comment (if Task 3 left it); RUNBOOK line 34 (done in Step 1).

- [ ] **Step 5: Commit** — `M2a: docs — RUNBOOK CI section, ADR-0015, spec seed 15, stale comments` (+ the separate spec commit from Step 3).

---

### Task 15: Landing — acceptance evidence and the ledger

- [ ] **Step 1: Run every gate on the branch head**: `gofmt -l .` empty; `go vet`, `go tool staticcheck`, `go test` (all `-tags nodynamic`) green; `cd worker && node --test`; `cd bench && npm test`; `go tool actionlint`; build binary; `site check` 0; `site build` twice + `scripts/dist-hash.sh` identical; `site check --dist` 0.
- [ ] **Step 2: `scripts/bench.sh`** on the real build: every CI row green; `site scorecard` fragment Nu-valid; module timings recorded.
- [ ] **Step 3: Post modules against the placeholder** (read-only): transport, DNS, Observatory, headers pass; the rows that need the real site (`/colophon/`) fail as expected and are recorded.
- [ ] **Step 4: Preview upload** (allowed): `scripts/preview.sh` from the branch → `scripts/verify-preview.sh <url>` 38/38 (or the new count if Task 3 added checks); the preview URL into the ledger, with "readable by anyone with the link".
- [ ] **Step 5: Public-tree scan**: `git log main..HEAD --format=%B | grep -n 'Claude-Session\|/home/\|git\.'` empty; `git grep -n 'radoo\|/home/' -- . ':!docs/plans/2026-09-17*'` empty; read every commit message as a stranger.
- [ ] **Step 6: The landing report** lists each acceptance criterion below with its evidence, and separates "verified here" from "needs the first push" (the Actions run, the PR comment, the amd64 hash list, the `act` attempt's outcome).

## Acceptance criteria (binary; the landing verifies each with evidence)

1. `go test -tags nodynamic ./...` green; `go vet`, `go tool staticcheck`, `gofmt -l` clean; `go tool actionlint` clean; `cd worker && node --test` green; `cd bench && npm test` green.
2. `site serve --static` serves the built `dist/` unchanged after a source edit, gzips text, answers `/404.html` and unknown paths with 404, applies `_headers`, exits on SIGTERM.
3. `site check` refuses `launched: soon` and accepts `launched: 2026-10-01`; `site scorecard` marks a 91-day-old CI row stale.
4. Every Worker-generated response carries the eight security headers; `/colophon/scorecard.json` answers on a preview host with `x-robots-tag`.
5. `scripts/kv.sh` put/get/get --metadata/delete round-trips a scratch key against the live namespace; `scorecard`/`scorecard.json` were never written by this plan.
6. `scripts/verify-edge.sh --ci` passes against production (placeholder) with the API section skipped; the full suite still passes with the operator file.
7. `scripts/bench.sh` on the current build: every CI row (`Audited build`, Lighthouse, HTML validity, Accessibility, Links, Feeds, Structured data, Social previews, i18n, Security headers, Weight, Privacy, Well-known files, Font correctness) is `pass: true`; the fragment from `site scorecard` is Nu-valid.
8. Lighthouse: 7 pages × mobile + desktop, 3-run median, every category 100, run through `site serve --static` with gzip.
9. Every bench module has a failing-fixture test; `bench/package.json` has no version ranges; `bench/package-lock.json` and `package-lock.json` committed; `npm ci` clean in both.
10. `.github/workflows/ci.yml` and `audit.yml`: `actionlint` clean; every `uses:` is a 40-hex SHA with a version comment; only `actions/*`; `preview`/`deploy`/`publish` skip with a summary line when the secret is absent; `deploy` and `post` are gated on `launched`; `concurrency` set; every job has `timeout-minutes`.
11. `.github/dependabot.yml` covers gomod, github-actions, npm `/`, npm `/bench`.
12. RUNBOOK has the CI section; `grep -n 'M2' RUNBOOK.md README.md` shows only "M2a"/"M2b" references that are true; ADR-0015 exists; spec §12 has seed 15; the spec amendments are one separate commit.
13. `git log main..HEAD`: every commit signed (`%G?` = G), `Co-Authored-By` present, no `Claude-Session`; nothing pushed to `origin` or `gitea` by this plan.
14. `scripts/preview.sh` from the branch head uploads a preview that passes `scripts/verify-preview.sh`.
15. The colophon dependency count on the built site is unchanged from `main` (tool directives do not link into the binary).
