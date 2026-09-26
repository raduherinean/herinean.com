# M2b — Authoring: Four Skills, Drafts That Commit, One-Commit Publish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A piece goes from `site new` to a merged `Publish:` commit — and a published piece to a merged `Correct:` commit — without a bypassed hook, without a draft reaching the public repository, and without a claim reaching a reader that neither an agent nor the author has checked.

**Architecture:** What a validator can decide is code: four small generator additions in Go (`check --draft`, `stamp`, `link`, two `check` rules, `serve` keeping the last good build) and one tested Node script (`gate.mjs`, standard library only) that `/publish-piece` runs before anything reaches `origin`. What only judgement can decide is four `SKILL.md` files under `.claude/skills/`. The fact-check ledger lives in the shared git directory, never in a commit; publishing pushes one author-written commit from a branch built on `origin/main`.

**Tech Stack:** Go 1.27 (existing module, `-tags nodynamic`); Node 24 (`node:test`, no packages); Bash (the pre-commit hook); Claude Code project skills; GitHub Actions (one line in M2a's `ci.yml`).

**Spec:** `docs/specs/2026-09-26-m2b-authoring-design.md` (this plan implements it; the parent is `docs/specs/2026-09-17-herinean-com-design.md`, §4.2, §5, §8, §10, §11, §13).

## Global Constraints

- Public repository: every file, commit message and comment is read by strangers. No private hosts, no local paths (`/home/...`), no session links. Commit messages: `M2b: <component> — <what>`; a `WHY:` body when the decision is not obvious from the diff; the trailer is exactly `Co-Authored-By: Claude` — no model name, no version, no `Claude-Session:` line. **This repository's trailer overrides any trailer your own harness reminder suggests.** Commits are SSH-signed by the repository's git config. Never push; the controller pushes to the private remote.
- Examples in skills, docs, tests and fixtures are invented. Nothing from a draft, a ledger or the author's work goes into a file in this repository.
- Every Go invocation uses `-tags nodynamic` (`go build`, `go test`, `go vet`, `go tool staticcheck`). Build the binary (`go build -tags nodynamic -o .cache/site ./cmd/site`) and run it; never `go run` (it hides exit code 3).
- Before every commit: `gofmt -l .` empty; `go vet -tags nodynamic ./...`, `go tool staticcheck -tags nodynamic ./...`, `go test -tags nodynamic ./...` green; `(cd worker && node --test)` green; after Task 5 also `(cd .claude/skills/publish-piece && node --test)` green.
- `site check` exit codes: `3` = author inputs missing (allowed by the hook, a failure in CI); `1` = a real problem; `2` = usage.
- Dates: Europe/Bucharest (`content.Bucharest`). Wall-clock time may enter a source file (`site stamp`) but never `dist/`.
- `gate.mjs` uses the Node standard library only: no `package.json`, no dependency.
- The skills never write to `content/` by hand. Front-matter fields change only through `site stamp` and `site link`; `/translate-piece` creates one new file, once.
- Ledger location: `$(git rev-parse --git-common-dir)/review/<key>/` — files `ledger-<lang>.md`, `linkedin-<lang>.md`, `model-pass-<lang>.md`. Ledger format (spec §5):

  ```
  hash: sha256:<hex of the piece file as reviewed>
  repos: <path>, <path>

  ## Claims
  | # | Claim | Kind | Evidence | Agent | Author | Note |
    Kind = repo | source;  Agent = confirmed | wrong | unverifiable | blocked;  Author = ✓ | ✗ | (empty)

  ## Tier
  | # | Sentence | Tier | Why | Resolution |
    Tier = work | client;  Resolution = kept | changed (work) · signed-off | changed (client) · (empty)
  ```

## Review Focus

- **A ledger written by a model is messy:** columns in another order, a `|` inside a claim, capitalised verdicts, prose between the tables. The gate must read it by header name, honour `\|`, compare case-insensitively, and stop a table at the next `## ` heading. (Task 5 test: "columns in any order, escaped pipes, capitals".)
- **A piece branch that deletes or renames a file** (a published URL would change or vanish). The scope check refuses `D`, `R` and `C` entries, not only unexpected paths. (Task 5 test: "deletions and renames are refused".)
- **The author fixes the rule `serve` was complaining about.** The next request must rebuild and show the new page, not keep serving the old good build. (Task 4 test: the recovery half of `TestServeKeepsTheLastGoodBuild`.)
- **A piece file with CRLF line endings, or `date:` in the body text.** `stamp` and `link` must change only the front-matter line and keep every other byte, line endings included. (Task 3 tests: `TestStampKeepsCRLF`, and the body line in `TestStampWritesTheBucharestDate`.)
- **`site check --draft --dist`** — two modes that do not combine. The CLI must refuse with exit 2 rather than silently pick one. (Task 1 step: the CLI smoke check.)

## Working conventions for this plan

- Work in the worktree `.claude/worktrees/m2b-design` on branch `m2b/design` (from `m2a/ci-bench` @ `5f6bf57`; the spec is commit `627d901`).
- The hooks path is set (`git config core.hooksPath` = `.githooks`); if not, run `scripts/setup.sh` first.
- Executors record every departure from a step, with the reason, in the controller's ledger; do not silently change a rule, a threshold or a message.
- Tasks 1–5 are code with tests; Tasks 6–9 are skill files, verified by the checks in their steps and by Task 12; Tasks 10–11 are docs; Task 12 is run by the controller **with the author** (the author's marks cannot be delegated).

---

## File structure

**Create**
- `internal/site/stamp.go` — `Stamp`, `Link`, and the front-matter line editor they share (`frontMatter`, `fieldLine`, `fieldValue`, `readField`, `setField`).
- `internal/site/stamp_test.go`
- `.claude/skills/publish-piece/gate.mjs` — the gate: `path`, `hash`, `scope`, `ledger` commands; exported pure functions.
- `.claude/skills/publish-piece/test/gate.test.mjs`
- `.claude/skills/review-piece/SKILL.md`
- `.claude/skills/translate-piece/SKILL.md`, `.claude/skills/translate-piece/glossary.md`
- `.claude/skills/publish-piece/SKILL.md`
- `.claude/skills/link-piece/SKILL.md`
- `.claude/CLAUDE.md`

**Modify**
- `cmd/site/main.go` — `check --draft`, `stamp`, `link`; usage text.
- `internal/site/site.go` — `Options.Draft` comment.
- `internal/site/check.go` — print draft warnings in `Check`.
- `internal/site/build.go` — the launch-date rule in `load`.
- `internal/content/frontmatter.go` — the discussion-host rule; `CheckDiscussionURL`, `DiscussionField`.
- `internal/content/frontmatter_test.go`, `internal/site/site_test.go`, `internal/site/serve_test.go`
- `internal/site/serve.go` — keep the last good build.
- `.githooks/pre-commit` — `--draft` on `piece/*`; the gate's tests when `.claude/` changes.
- `.github/workflows/ci.yml` — the gate's tests in `build`.
- `RUNBOOK.md` — "Writing and publishing"; M2b → M2c where the weekly job is meant.
- `bench/README.md`, `bench/lib/dns.mjs`, `docs/adr/0004-analytics-without-a-tracker.md`, `docs/adr/0011-tls13-minimum.md` — M2b → M2c (the weekly job moved).
- `docs/specs/2026-09-17-herinean-com-design.md` — the amendments of design §11, one commit (Task 11).

---

### Task 1: `site check --draft` and the hook's branch switch

**Files:**
- Modify: `cmd/site/main.go` (the `check` case and `usage`)
- Modify: `internal/site/check.go:37-43` (`Check`)
- Modify: `internal/site/site.go` (`Options.Draft` comment)
- Modify: `.githooks/pre-commit`
- Test: `internal/site/site_test.go`

**Interfaces:**
- Consumes: `site.Options{Root, Draft}`; `load(o)` already calls `content.LoadDraft` when `o.Draft` and stores the warnings in `build.warnings` (`internal/site/build.go:63-75`).
- Produces: `site check --draft` (CLI). `Check(Options{Draft: true})` prints each warning as `site: draft: <file:line: msg>` on stderr and returns nil when only blanks remain.

- [ ] **Step 1: Write the failing tests** — append to `internal/site/site_test.go`:

```go
const draftPiece = "---\ntitle: \"\"\ndate:\nkey: ciorna\npillar:\nsummary: \"\"\n---\n\n## Situation\n\nStill writing.\n"

func writePiece(t *testing.T, root, lang, slug, src string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "content", lang, slug+".md"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

// check --draft is what the hook runs on piece/* branches: the fields `site new` leaves blank are warnings there.
func TestCheckDraftAllowsBlankFields(t *testing.T) {
	root := fixtureRoot(t)
	writePiece(t, root, "ro", "ciorna", draftPiece)
	if err := Check(Options{Root: root}); err == nil || !strings.Contains(err.Error(), "date is empty") {
		t.Fatalf("Check: want the blank date refused, got %v", err)
	}
	if err := Check(Options{Root: root, Draft: true}); err != nil {
		t.Fatalf("Check(Draft): want nil, got %v", err)
	}
}

// Every other rule still fails under --draft: the mode forgives what `site new` leaves blank, nothing else.
func TestCheckDraftStillFailsOtherRules(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"long summary", strings.Replace(draftPiece, `summary: ""`, `summary: "`+strings.Repeat("x", 161)+`"`, 1), "160"},
		{"cedilla", draftPiece + "\nReţea.\n", "comma-below"},
		{"future date", strings.Replace(draftPiece, "date:\n", "date: 2026-09-30\n", 1), "after"},
		{"draft field", strings.Replace(draftPiece, "pillar:\n", "pillar:\ndraft: true\n", 1), "draft"},
	}
	for _, c := range cases {
		root := fixtureRoot(t)
		writePiece(t, root, "ro", "ciorna", c.src)
		if err := Check(Options{Root: root, Draft: true}); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: Check(Draft) = %v, want an error containing %q", c.name, err, c.want)
		}
	}
}
```

(`fixtureRoot` sets `SOURCE_DATE_EPOCH` to 2026-09-21, so 2026-09-30 is in the future.)

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -tags nodynamic ./internal/site/ -run 'TestCheckDraft' -v`
Expected: both PASS already — `load` honours `Options.Draft` (`internal/site/build.go:63`). What is missing is the CLI flag and the warnings on stderr (Steps 3–4). Keep the tests: they pin the behaviour the hook relies on. If either fails, stop and record why in the ledger before going on.

- [ ] **Step 3: Print the warnings in `Check`** — in `internal/site/check.go`, right after `author := err` in `Check`:

```go
	for _, w := range b.warnings {
		fmt.Fprintln(os.Stderr, "site: draft:", w.Error())
	}
```

In `internal/site/site.go`, replace the `Draft` field comment:

```go
	Draft  bool   // serve and check --draft: blank title/date/pillar/summary are warnings with visible defaults, not problems
```

- [ ] **Step 4: Add the flag** — in `cmd/site/main.go`, replace the `check` case:

```go
	case "check":
		fs := flag.NewFlagSet("check", flag.ExitOnError)
		dist := fs.Bool("dist", false, "check the built dist/ instead of the sources")
		draft := fs.Bool("draft", false, "blank title/date/pillar/summary are warnings, not failures (piece/* branches)")
		_ = fs.Parse(os.Args[2:])
		switch {
		case *dist && *draft:
			fmt.Fprintln(os.Stderr, "site check: --dist and --draft do not combine")
			os.Exit(2)
		case *dist:
			err = site.CheckDist(site.Options{Root: ".", Out: "dist"})
		default:
			err = site.Check(site.Options{Root: ".", Draft: *draft})
		}
```

and in `usage()` replace the check line with:

```
  site check [--draft|--dist]  validate sources (--draft: blanks site new leaves are warnings) or the built dist/
```

- [ ] **Step 5: Run the tests and the CLI smoke check**

Run: `go test -tags nodynamic ./internal/site/ -run 'TestCheckDraft' -v`
Expected: PASS.
Run: `go build -tags nodynamic -o .cache/site ./cmd/site && .cache/site check --draft --dist; echo "exit $?"`
Expected: `site check: --dist and --draft do not combine` and `exit 2`.
Run: `.cache/site check --draft; echo "exit $?"` on the real tree
Expected: `exit 0` (or `3` if an author input is missing — record it).

- [ ] **Step 6: The hook's branch switch** — in `.githooks/pre-commit`, replace the line

```bash
  set +e; SOURCE_DATE_EPOCH="$(date +%s)" "$bin" check; rc=$?; set -e
```

with

```bash
  # piece/* branches hold drafts (spec §8): the blanks `site new` leaves are warnings there; everywhere else they fail.
  mode=
  case "$(git symbolic-ref --quiet --short HEAD || true)" in piece/*) mode=--draft ;; esac
  set +e; SOURCE_DATE_EPOCH="$(date +%s)" "$bin" check $mode; rc=$?; set -e
```

and change the file's first comment line to `# gofmt + vet + site check (--draft on piece/* branches) before every commit. Install with scripts/setup.sh.`

- [ ] **Step 7: Exercise the hook** (shell; nothing here is kept)

```bash
git switch -c piece/hook-check
printf -- '---\ntitle: ""\ndate:\nkey: hook-check\npillar:\nsummary: ""\n---\n\nDraft.\n' > content/en/hook-check.md
git add content/en/hook-check.md && git commit -m "hook check" ; echo "piece branch: exit $?"
git reset --hard HEAD~1
git switch m2b/design && git branch -D piece/hook-check
git switch -c hook-check
printf -- '---\ntitle: ""\ndate:\nkey: hook-check\npillar:\nsummary: ""\n---\n\nDraft.\n' > content/en/hook-check.md
git add content/en/hook-check.md && git commit -m "hook check" ; echo "other branch: exit $?"
git reset -q HEAD content/en/hook-check.md && rm content/en/hook-check.md
git switch m2b/design && git branch -D hook-check
```

Expected: `piece branch: exit 0` with `site: draft: … date is empty …` warnings; `other branch: exit 1` naming `date is empty`; `git status` clean afterwards and neither branch left.

- [ ] **Step 8: Commit**

```bash
git add cmd/site/main.go internal/site/check.go internal/site/site.go internal/site/site_test.go .githooks/pre-commit
git commit -F - <<'EOF'
M2b: generator — site check --draft; the hook uses it on piece/* branches

WHY: drafts are meant to be committed and pushed to the private remote as
often as the author likes (spec §8), but the hook ran the full check, so
every draft commit needed --no-verify. --draft reuses the loader's draft
mode (the one site serve already uses): the blanks site new leaves are
warnings; every other rule still fails. CI keeps the full check, so an
undated piece still cannot reach main.

Co-Authored-By: Claude
EOF
```

---

### Task 2: two `check` rules — discussion hosts and no piece dated before the launch

**Files:**
- Modify: `internal/content/frontmatter.go:123-127` (the https loop) and add two exported functions
- Modify: `internal/site/build.go:61-73` (`load`)
- Test: `internal/content/frontmatter_test.go`, `internal/site/site_test.go`

**Interfaces:**
- Consumes: `config.Config.Launched` (`YYYY-MM-DD` or empty; validated by `config.Load`, which returns the config even with a placeholder error); `content.Piece{File, Date}`; `content.Problems.Add(file, line, format, args...)`.
- Produces: `content.CheckDiscussionURL(field, raw string) error` (field `"linkedin"` or `"medium"`); `content.DiscussionField(raw string) (string, error)` — Task 3's `Link` uses it.

- [ ] **Step 1: Write the failing tests** — append to `internal/content/frontmatter_test.go`:

```go
// A discussion link points at one host, compared exactly: a look-alike, a bare domain, a subdomain, a userinfo
// trick or plain http is refused.
func TestDiscussionHosts(t *testing.T) {
	cases := []struct{ name, line, want string }{
		{"linkedin ok", "linkedin: https://www.linkedin.com/feed/update/urn:li:activity:1", ""},
		{"medium ok", "medium: https://medium.com/@someone/a-title-1", ""},
		{"linkedin look-alike", "linkedin: https://www.linkedin.com.evil.example/posts/x", "www.linkedin.com"},
		{"linkedin bare domain", "linkedin: https://linkedin.com/posts/x", "www.linkedin.com"},
		{"linkedin userinfo", "linkedin: https://www.linkedin.com@evil.example/x", "www.linkedin.com"},
		{"linkedin http", "linkedin: http://www.linkedin.com/posts/x", "https"},
		{"medium subdomain", "medium: https://evil.medium.com/x", "medium.com"},
	}
	for _, c := range cases {
		src := strings.Replace(good, "linkedin: https://www.linkedin.com/posts/x", c.line, 1)
		_, _, _, probs := ParseFrontMatter("content/en/x.md", []byte(src), "en", now)
		err := probs.Err()
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: want no problem, got %v", c.name, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
			t.Errorf("%s: want a problem containing %q, got %v", c.name, c.want, err)
		}
	}
}

func TestDiscussionField(t *testing.T) {
	for raw, want := range map[string]string{
		"https://www.linkedin.com/feed/update/urn:li:activity:1": "linkedin",
		"https://medium.com/@someone/a-title-1":                  "medium",
	} {
		if got, err := DiscussionField(raw); err != nil || got != want {
			t.Errorf("DiscussionField(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
	if _, err := DiscussionField("https://example.com/x"); err == nil {
		t.Error("DiscussionField(example.com): want an error")
	}
}
```

Append to `internal/site/site_test.go`:

```go
func appendSiteYAML(t *testing.T, root, line string) {
	t.Helper()
	p := filepath.Join(root, "site.yaml")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(b, []byte("\n"+line+"\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Pieces merged before the launch go live on launch day; their date must say so, or datePublished names a day on
// which the site was not public. The fixture's pieces are dated 2026-09-05 … 2026-09-15.
func TestLaunchedRuleRefusesEarlierPieces(t *testing.T) {
	root := fixtureRoot(t)
	appendSiteYAML(t, root, "launched: 2026-09-11")
	err := Check(Options{Root: root})
	if err == nil || !strings.Contains(err.Error(), "before launched 2026-09-11") {
		t.Fatalf("Check: want pieces dated before the launch refused, got %v", err)
	}
	if err := Build(Options{Root: root, Out: "dist"}); err == nil {
		t.Fatal("Build: want the same refusal")
	}
}

// A piece dated on launch day is fine: the rule is "not before".
func TestLaunchedRuleAcceptsLaunchDay(t *testing.T) {
	root := fixtureRoot(t)
	appendSiteYAML(t, root, "launched: 2026-09-05")
	if err := Check(Options{Root: root}); err != nil {
		t.Fatalf("Check: want nil, got %v", err)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -tags nodynamic ./internal/content/ ./internal/site/ -run 'TestDiscussion|TestLaunchedRule' -v`
Expected: FAIL — `DiscussionField` undefined (compile error); after stubbing, look-alike cases and `TestLaunchedRuleRefusesEarlierPieces` fail.

- [ ] **Step 3: The host rule** — in `internal/content/frontmatter.go`, add `"net/url"` to the imports; replace

```go
	for _, u := range []string{p.LinkedIn, p.Medium} {
		if u != "" && !strings.HasPrefix(u, "https://") {
			probs.Add(file, 1, "%q must be an https URL", u)
		}
	}
```

with

```go
	for _, d := range []struct{ field, url string }{{"linkedin", p.LinkedIn}, {"medium", p.Medium}} {
		if d.url != "" {
			if err := CheckDiscussionURL(d.field, d.url); err != nil {
				probs.Add(file, 1, "%v", err)
			}
		}
	}
```

and add at the end of the file:

```go
// discussionHosts maps each discussion field to the one host it may point at, compared exactly.
var discussionHosts = map[string]string{"linkedin": "www.linkedin.com", "medium": "medium.com"}

// CheckDiscussionURL reports whether raw is an https URL on field's host (no userinfo, no look-alike, no subdomain).
func CheckDiscussionURL(field, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Host != discussionHosts[field] {
		return fmt.Errorf("%s: %q must be an https URL on %s", field, raw, discussionHosts[field])
	}
	return nil
}

// DiscussionField names the front-matter field a discussion URL belongs in, by its host.
func DiscussionField(raw string) (string, error) {
	for _, f := range []string{"linkedin", "medium"} {
		if CheckDiscussionURL(f, raw) == nil {
			return f, nil
		}
	}
	return "", fmt.Errorf("%q is not an https URL on www.linkedin.com or medium.com", raw)
}
```

- [ ] **Step 4: The launch-date rule** — in `internal/site/build.go`, in `load`, insert right before `if err := probs.Err(); err != nil {`:

```go
	// Pieces merged before the launch go live on launch day (spec §11): a date before `launched:` would make
	// datePublished name a day on which the site was not public. The launch commit re-stamps them.
	if cfg != nil && cfg.Launched != "" && s != nil {
		launched, _ := time.ParseInLocation("2006-01-02", cfg.Launched, content.Bucharest) // validated by config.Load
		for _, p := range s.Pieces {
			if p.Date.Before(launched) {
				probs.Add(p.File, 1, "date %s is before launched %s: dated before the launch; site stamp it", p.Date.Format("2006-01-02"), cfg.Launched)
			}
		}
	}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -tags nodynamic ./...`
Expected: PASS (every package; the golden tests are unaffected — no rendered output changed).

- [ ] **Step 6: Commit**

```bash
git add internal/content/frontmatter.go internal/content/frontmatter_test.go internal/site/build.go internal/site/site_test.go
git commit -F - <<'EOF'
M2b: check — discussion links on one exact host; no piece dated before launched

WHY: the https prefix accepted a look-alike such as
www.linkedin.com.evil.example, and /link-piece will write these fields
from a pasted URL. Pieces merged before the launch go live on launch
day, so a date before launched: would make datePublished name a day the
site was not public; the launch commit re-stamps them.

Co-Authored-By: Claude
EOF
```

---

### Task 3: `site stamp [--updated]` and `site link`

**Files:**
- Create: `internal/site/stamp.go`
- Create: `internal/site/stamp_test.go`
- Modify: `cmd/site/main.go` (two cases, imports, usage)

**Interfaces:**
- Consumes: `content.Bucharest`; `content.DiscussionField(raw) (string, error)` (Task 2).
- Produces: `site.Stamp(file string, updated bool, now time.Time) (old, stamped string, err error)`; `site.Link(file, rawURL string) (field, old string, err error)`; CLI `site stamp [--updated] <file>` and `site link <file> <url>`, each printing one `old → new` line on stderr. `/publish-piece` and `/link-piece` call the CLI.

- [ ] **Step 1: Write the failing tests** — create `internal/site/stamp_test.go`:

```go
package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/raduherinean/herinean.com/internal/content"
)

// 22:30 UTC on 26 September is 01:30 on the 27th in Bucharest (EEST, UTC+3).
var lateEvening = time.Date(2026, 9, 26, 22, 30, 0, 0, time.UTC)

const unstamped = "---\ntitle: \"T\"\ndate:\nkey: k\npillar: analysis\nsummary: \"S\"\n---\n\ndate: in the body is prose, not a field.\n"

var published = strings.Replace(unstamped, "date:\n", "date: 2026-09-20\n", 1)

func pieceFile(t *testing.T, src string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "t.md")
	if err := os.WriteFile(f, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return f
}

func readFile(t *testing.T, f string) string {
	t.Helper()
	b, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestStampWritesTheBucharestDate(t *testing.T) {
	f := pieceFile(t, unstamped)
	old, got, err := Stamp(f, false, lateEvening)
	if err != nil || old != "" || got != "2026-09-27" {
		t.Fatalf("Stamp = %q, %q, %v; want \"\", 2026-09-27, nil", old, got, err)
	}
	want := strings.Replace(unstamped, "date:\n", "date: 2026-09-27\n", 1) // the body's "date:" line is untouched
	if s := readFile(t, f); s != want {
		t.Fatalf("file:\n%s\nwant:\n%s", s, want)
	}
	if _, _, _, probs := content.ParseFrontMatter("content/en/t.md", []byte(want), "en", lateEvening); probs.Err() != nil {
		t.Fatalf("stamped piece does not parse: %v", probs.Err())
	}
}

func TestStampRestamps(t *testing.T) {
	f := pieceFile(t, published)
	old, _, err := Stamp(f, false, lateEvening)
	if err != nil || old != "2026-09-20" || !strings.Contains(readFile(t, f), "date: 2026-09-27\n") {
		t.Fatalf("Stamp = %q, %v; file:\n%s", old, err, readFile(t, f))
	}
}

func TestStampUpdatedAddsTheLineAndKeepsDate(t *testing.T) {
	f := pieceFile(t, published)
	if _, _, err := Stamp(f, true, lateEvening); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(published, "date: 2026-09-20\n", "date: 2026-09-20\nupdated: 2026-09-27\n", 1)
	if s := readFile(t, f); s != want {
		t.Fatalf("file:\n%s\nwant:\n%s", s, want)
	}
}

func TestStampUpdatedReplaces(t *testing.T) {
	f := pieceFile(t, strings.Replace(published, "key: k\n", "updated: 2026-09-21\nkey: k\n", 1))
	old, _, err := Stamp(f, true, lateEvening)
	if err != nil || old != "2026-09-21" || !strings.Contains(readFile(t, f), "updated: 2026-09-27\n") {
		t.Fatalf("Stamp = %q, %v; file:\n%s", old, err, readFile(t, f))
	}
}

func TestStampUpdatedRefusesAnUnpublishedPiece(t *testing.T) {
	f := pieceFile(t, unstamped)
	if _, _, err := Stamp(f, true, lateEvening); err == nil || !strings.Contains(err.Error(), "date is empty") {
		t.Fatalf("Stamp(updated) on an undated piece: want refusal, got %v", err)
	}
	if readFile(t, f) != unstamped {
		t.Fatal("refused stamp changed the file")
	}
}

func TestStampKeepsCRLF(t *testing.T) {
	f := pieceFile(t, strings.ReplaceAll(unstamped, "\n", "\r\n"))
	if _, _, err := Stamp(f, false, lateEvening); err != nil {
		t.Fatal(err)
	}
	want := strings.ReplaceAll(strings.Replace(unstamped, "date:\n", "date: 2026-09-27\n", 1), "\n", "\r\n")
	if s := readFile(t, f); s != want {
		t.Fatalf("file %q, want %q", s, want)
	}
}

func TestStampRefusesWithoutFrontMatter(t *testing.T) {
	f := pieceFile(t, "Just text.\n")
	if _, _, err := Stamp(f, false, lateEvening); err == nil || !strings.Contains(err.Error(), "no front matter") {
		t.Fatalf("want refusal, got %v", err)
	}
}

func TestLinkSetsTheFieldByHost(t *testing.T) {
	f := pieceFile(t, published)
	li := "https://www.linkedin.com/feed/update/urn:li:activity:1"
	field, old, err := Link(f, li)
	if err != nil || field != "linkedin" || old != "" {
		t.Fatalf("Link = %q, %q, %v", field, old, err)
	}
	if !strings.Contains(readFile(t, f), "summary: \"S\"\nlinkedin: "+li+"\n---\n") {
		t.Fatalf("linkedin: not inserted after summary:\n%s", readFile(t, f))
	}
	md := "https://medium.com/@someone/t-1"
	if field, _, err := Link(f, md); err != nil || field != "medium" {
		t.Fatalf("Link(medium) = %q, %v", field, err)
	}
	if !strings.Contains(readFile(t, f), "linkedin: "+li+"\nmedium: "+md+"\n---\n") {
		t.Fatalf("medium: not inserted after linkedin:\n%s", readFile(t, f))
	}
}

func TestLinkReplaces(t *testing.T) {
	f := pieceFile(t, strings.Replace(published, "summary: \"S\"\n", "summary: \"S\"\nlinkedin: https://www.linkedin.com/posts/old\n", 1))
	_, old, err := Link(f, "https://www.linkedin.com/posts/new")
	if err != nil || old != "https://www.linkedin.com/posts/old" || strings.Contains(readFile(t, f), "/posts/old") {
		t.Fatalf("Link = %q, %v; file:\n%s", old, err, readFile(t, f))
	}
}

func TestLinkRefusesALookAlikeHostAndAnUnpublishedPiece(t *testing.T) {
	f := pieceFile(t, published)
	if _, _, err := Link(f, "https://www.linkedin.com.evil.example/x"); err == nil {
		t.Error("look-alike host: want refusal")
	}
	g := pieceFile(t, unstamped)
	if _, _, err := Link(g, "https://www.linkedin.com/posts/x"); err == nil || !strings.Contains(err.Error(), "link a published piece") {
		t.Errorf("unpublished piece: want refusal, got %v", err)
	}
	if readFile(t, f) != published || readFile(t, g) != unstamped {
		t.Error("a refused link changed a file")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -tags nodynamic ./internal/site/ -run 'TestStamp|TestLink' -v`
Expected: FAIL — `undefined: Stamp`, `undefined: Link`.

- [ ] **Step 3: Implement** — create `internal/site/stamp.go`:

```go
package site

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/raduherinean/herinean.com/internal/content"
)

// Stamp writes now's date in Europe/Bucharest into a piece's date: — or, with updated, its updated: (a correction;
// date: stays) — and returns the old and new values. It edits a source file, never dist/: wall-clock time enters
// the site only through the date the author commits.
func Stamp(file string, updated bool, now time.Time) (old, stamped string, err error) {
	stamped = now.In(content.Bucharest).Format("2006-01-02")
	if !updated {
		old, err = setField(file, "date", stamped, "title")
		return old, stamped, err
	}
	date, err := readField(file, "date")
	if err != nil {
		return "", "", err
	}
	if date == "" {
		return "", "", fmt.Errorf("%s: date is empty; a correction is for a published piece (stamp without --updated)", file)
	}
	old, err = setField(file, "updated", stamped, "date")
	return old, stamped, err
}

// Link sets a published piece's discussion URL — linkedin: or medium:, chosen by the URL's host — and returns the
// field and its old value.
func Link(file, rawURL string) (field, old string, err error) {
	field, err = content.DiscussionField(rawURL)
	if err != nil {
		return "", "", err
	}
	date, err := readField(file, "date")
	if err != nil {
		return "", "", err
	}
	if date == "" {
		return "", "", fmt.Errorf("%s: date is empty; link a published piece", file)
	}
	old, err = setField(file, field, rawURL, "linkedin", "summary")
	return field, old, err
}

// frontMatter splits src into lines, each keeping its ending, and returns the index of the closing "---" line.
func frontMatter(file string, src []byte) (lines [][]byte, end int, err error) {
	lines = bytes.SplitAfter(src, []byte("\n"))
	if string(bytes.TrimRight(lines[0], "\r\n")) != "---" {
		return nil, 0, fmt.Errorf("%s: no front matter", file)
	}
	for i := 1; i < len(lines); i++ {
		if string(bytes.TrimRight(lines[i], "\r\n")) == "---" {
			return lines, i, nil
		}
	}
	return nil, 0, fmt.Errorf("%s: front matter is not closed", file)
}

// fieldLine returns the index of the front-matter line `name: …`, or -1.
func fieldLine(lines [][]byte, end int, name string) int {
	for i := 1; i < end; i++ {
		if bytes.HasPrefix(lines[i], []byte(name+":")) {
			return i
		}
	}
	return -1
}

// fieldValue is the value on a `name: value` line; empty when blank or only a comment.
func fieldValue(line []byte, name string) string {
	v := strings.TrimSpace(strings.TrimPrefix(string(bytes.TrimRight(line, "\r\n")), name+":"))
	if strings.HasPrefix(v, "#") {
		return ""
	}
	return strings.Trim(v, `"'`)
}

// readField returns a front-matter field's value; empty when absent or blank.
func readField(file, name string) (string, error) {
	src, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	lines, end, err := frontMatter(file, src)
	if err != nil {
		return "", err
	}
	if i := fieldLine(lines, end, name); i >= 0 {
		return fieldValue(lines[i], name), nil
	}
	return "", nil
}

// setField writes `name: value` into a piece's front matter — replacing the line when it exists, else inserting it
// after the first of `after` that exists, else before the closing `---` — and keeps every other byte, line endings
// included. It returns the old value.
func setField(file, name, value string, after ...string) (old string, err error) {
	st, err := os.Stat(file)
	if err != nil {
		return "", err
	}
	src, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	lines, end, err := frontMatter(file, src)
	if err != nil {
		return "", err
	}
	eol := "\n"
	if bytes.HasSuffix(lines[0], []byte("\r\n")) {
		eol = "\r\n"
	}
	line := []byte(name + ": " + value + eol)
	if i := fieldLine(lines, end, name); i >= 0 {
		old = fieldValue(lines[i], name)
		lines[i] = line
	} else {
		at := end
		for _, a := range after {
			if i := fieldLine(lines, end, a); i >= 0 {
				at = i + 1
				break
			}
		}
		lines = append(lines[:at:at], append([][]byte{line}, lines[at:]...)...)
	}
	return old, os.WriteFile(file, bytes.Join(lines, nil), st.Mode().Perm())
}
```

- [ ] **Step 4: The CLI** — in `cmd/site/main.go` add `"time"` to the imports (beside `_ "time/tzdata"`) and these cases before `default:`:

```go
	case "stamp":
		fs := flag.NewFlagSet("stamp", flag.ExitOnError)
		upd := fs.Bool("updated", false, "stamp updated: (a correction) instead of date:")
		_ = fs.Parse(os.Args[2:])
		if fs.NArg() != 1 {
			fmt.Fprintln(os.Stderr, "usage: site stamp [--updated] <file>")
			os.Exit(2)
		}
		var old, stamped string
		old, stamped, err = site.Stamp(fs.Arg(0), *upd, time.Now())
		if err == nil {
			field := "date"
			if *upd {
				field = "updated"
			}
			if old == "" {
				old = "(empty)"
			}
			fmt.Fprintf(os.Stderr, "site stamp: %s: %s %s → %s\n", fs.Arg(0), field, old, stamped)
		}
	case "link":
		if len(os.Args) != 4 {
			fmt.Fprintln(os.Stderr, "usage: site link <file> <url>")
			os.Exit(2)
		}
		var field, old string
		field, old, err = site.Link(os.Args[2], os.Args[3])
		if err == nil {
			if old == "" {
				old = "(empty)"
			}
			fmt.Fprintf(os.Stderr, "site link: %s: %s %s → %s\n", os.Args[2], field, old, os.Args[3])
		}
```

and add to `usage()` after the `new` line:

```
  site stamp [--updated] <file>   today's date (Europe/Bucharest) into date: (or updated:)
  site link <file> <url>          linkedin: or medium: by the URL's host
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -tags nodynamic ./... && go vet -tags nodynamic ./... && go tool staticcheck -tags nodynamic ./...`
Expected: PASS, no findings.
Run: `go build -tags nodynamic -o .cache/site ./cmd/site && cp testdata/site/content/en/paired.md /tmp/stamp-smoke.md && .cache/site stamp /tmp/stamp-smoke.md && .cache/site link /tmp/stamp-smoke.md https://medium.com/@someone/x && rm /tmp/stamp-smoke.md`
Expected: two lines, `site stamp: …: date 2026-09-15 → <today>` and `site link: …: medium (empty) → https://medium.com/@someone/x`.

- [ ] **Step 6: Commit**

```bash
git add internal/site/stamp.go internal/site/stamp_test.go cmd/site/main.go
git commit -F - <<'EOF'
M2b: generator — site stamp [--updated] and site link

WHY: the skills must change a piece's front matter without editing the
file by hand (the model never writes to content/). stamp writes today's
Bucharest date into date:, or into updated: for a correction so the feed
keeps the original date; link sets linkedin: or medium: by the URL's
host through the same rule check applies. Both keep every other byte.

Co-Authored-By: Claude
EOF
```

---

### Task 4: `site serve` keeps the last good build

**Files:**
- Modify: `internal/site/serve.go:118-163` (the watermark variables and `rebuild`)
- Test: `internal/site/serve_test.go`

**Interfaces:**
- Consumes: `Build(o)` writes `dist/` all-or-nothing (a failed build leaves the previous `dist/` in place); `newestInput(root)`.
- Produces: no new API. Behaviour: a failing build with a good `dist/index.html` present is logged to stderr and the previous render keeps being served; with none, the handler answers 500 with the problem list; the same failing inputs are not rebuilt on every request.

- [ ] **Step 1: Write the failing tests** — append to `internal/site/serve_test.go`:

```go
func fetch(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

const brokenRO = "---\ntitle: \"Stricat\"\ndate: 2026-09-01\nkey: stricat\npillar: analysis\nsummary: \"Rezumat.\"\n---\n\nReţea cu sedilă.\n"

// A failing rule mid-writing keeps the last good render on screen (the problem goes to stderr) instead of a 500 on
// every page; fixing it brings the new page up on the next request.
func TestServeKeepsTheLastGoodBuild(t *testing.T) {
	root := fixtureRoot(t)
	addr := startServe(t, Options{Root: root, Out: "dist"})
	if code, _ := fetch(t, "http://"+addr+"/"); code != 200 {
		t.Fatalf("before: status %d", code)
	}
	f := filepath.Join(root, "content", "ro", "stricat.md")
	if err := os.WriteFile(f, []byte(brokenRO), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, body := fetch(t, "http://"+addr+"/"); code != 200 {
		t.Fatalf("with a failing rule: status %d, want the last good build: %s", code, body)
	}
	if err := os.WriteFile(f, []byte(strings.Replace(brokenRO, "ţ", "ț", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, body := fetch(t, "http://"+addr+"/ro/articole/stricat/"); code != 200 || !strings.Contains(body, "Rețea") {
		t.Fatalf("after the fix: status %d, want the new page: %s", code, body)
	}
}

// With no good build yet there is nothing to fall back on: the problem list is the page.
func TestServeWithoutAGoodBuildShowsTheProblems(t *testing.T) {
	root := fixtureRoot(t)
	if err := os.WriteFile(filepath.Join(root, "content", "ro", "stricat.md"), []byte(brokenRO), 0o644); err != nil {
		t.Fatal(err)
	}
	addr := startServe(t, Options{Root: root, Out: "dist"})
	if code, body := fetch(t, "http://"+addr+"/"); code != 500 || !strings.Contains(body, "comma-below") {
		t.Fatalf("status %d, want 500 with the cedilla problem: %s", code, body)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -tags nodynamic ./internal/site/ -run 'TestServeKeeps|TestServeWithout' -v`
Expected: `TestServeKeepsTheLastGoodBuild` FAILS with `with a failing rule: status 500`; `TestServeWithoutAGoodBuildShowsTheProblems` passes already (it pins the no-fallback case).

- [ ] **Step 3: Implement** — in `internal/site/serve.go`, replace `var built time.Time` with

```go
	var built, failed time.Time // start of the last good build; start of the last failed one since
	var buildErr error
	hasGood := func() bool { _, err := os.Stat(filepath.Join(dist, "index.html")); return err == nil }
```

and replace the non-static half of `rebuild` (from `start := time.Now()` to its `return nil`) with

```go
		start := time.Now()
		if (built.IsZero() && failed.IsZero()) || newestInput(o.Root).After(later(built, failed)) {
			if err := Build(o); err != nil {
				failed, buildErr = start, err
				if hasGood() {
					fmt.Fprintf(os.Stderr, "site serve: build failed; serving the last good build:\n%v\n", err)
				}
			} else {
				built, failed, buildErr = start, time.Time{}, nil
			}
		}
		if buildErr != nil && !hasGood() {
			return buildErr // nothing good to fall back on: the problem list is the page
		}
		return nil
```

and add after `Serve`:

```go
func later(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
```

Update the comment above `rebuild` — replace its last sentence with: `The watermarks are taken BEFORE Build runs, so an input saved mid-build is still picked up by the next request; a failed build is not retried until an input changes again.`

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -tags nodynamic ./internal/site/ -v -run 'TestServe'`
Expected: PASS, including the existing serve tests.

- [ ] **Step 5: Commit**

```bash
git add internal/site/serve.go internal/site/serve_test.go
git commit -F - <<'EOF'
M2b: serve — a failing rule keeps the last good build on screen

WHY: one broken rule (a summary over 160 characters, a cedilla) turned
every page of the writing preview into a 500. The build already swaps
dist/ all-or-nothing, so the previous render is still there: serve it,
print the problems, and retry only when an input changes. With no good
build yet, the problem list is still the page.

Co-Authored-By: Claude
EOF
```

---

### Task 5: the publish gate — `gate.mjs`, its tests, CI and the hook

**Files:**
- Create: `.claude/skills/publish-piece/gate.mjs`
- Create: `.claude/skills/publish-piece/test/gate.test.mjs`
- Modify: `.github/workflows/ci.yml` (the `build` job, after `- run: cd worker && node --test`)
- Modify: `.githooks/pre-commit`

**Interfaces:**
- Consumes: the ledger format (Global Constraints); `git rev-parse --git-common-dir`; `git diff --name-status <base>...HEAD`.
- Produces (exported from `gate.mjs`): `sha256(bytes) → 'sha256:<hex>'`; `pieceKey(src) → string`; `pieceLang(path) → 'en'|'ro'|''`; `splitRow(line) → string[]`; `table(md, heading) → object[] | null`; `checkLedger(ledger, piece, modelPass|null) → string[]`; `checkScope(changes, keyOf) → {probs, pieces}`. CLI: `node .claude/skills/publish-piece/gate.mjs path <piece>` · `hash <piece>` · `scope [--base <ref>]` · `ledger <piece>...`; exit 0 ok, 1 refused, 2 usage. Tasks 6–9 call the CLI.

- [ ] **Step 1: Write the failing tests** — create `.claude/skills/publish-piece/test/gate.test.mjs`:

```js
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { checkLedger, checkScope, pieceKey, pieceLang, sha256, splitRow, table } from '../gate.mjs';

const piece = '---\ntitle: "T"\ndate:\nkey: k\npillar: analysis\nsummary: "S"\n---\n\nBody.\n';
const row = (cells) => `| ${cells.join(' | ')} |`;
const ledger = ({ hash = sha256(piece), claims = [], tier = [], claimsHeader = '| # | Claim | Kind | Evidence | Agent | Author | Note |' } = {}) =>
  `# Review — T (en)\n\nhash: ${hash}\nrepos: /src/one\n\n## Claims\n\n${claimsHeader}\n|---|---|---|---|---|---|---|\n${claims.map(row).join('\n')}\n\nFree prose after the table.\n\n## Tier\n\n| # | Sentence | Tier | Why | Resolution |\n|---|---|---|---|---|\n${tier.map(row).join('\n')}\n\n## Editing\n\n| not | a | gate | table |\n`;

const repoOK = ['1', 'the parser rejects tabs', 'repo', 'lexer.go:42', 'confirmed', '', ''];
const sourceOK = ['2', 'revenue doubled in 2025', 'source', 'https://example.com/report — "revenue doubled"', 'confirmed', '✓', ''];
const workKept = ['1', 'our billing system runs nightly', 'work', 'names an internal system', 'kept'];

test('a complete ledger passes', () => {
  assert.deepEqual(checkLedger(ledger({ claims: [repoOK, sourceOK], tier: [workKept] }), piece, null), []);
});

test('zero claims and zero flags pass', () => {
  assert.deepEqual(checkLedger(ledger(), piece, null), []);
});

test('a stale hash is refused', () => {
  const p = checkLedger(ledger({ hash: sha256('other text') }), piece, null);
  assert.equal(p.length, 1);
  assert.match(p[0], /changed since the review/);
});

test('a missing hash line or section is refused', () => {
  assert.match(checkLedger('## Claims\n\n## Tier\n', piece, null).join('\n'), /no hash: line/);
  assert.match(checkLedger(`hash: ${sha256(piece)}\n\n## Tier\n`, piece, null).join('\n'), /no ## Claims/);
  assert.match(checkLedger(`hash: ${sha256(piece)}\n\n## Claims\n`, piece, null).join('\n'), /no ## Tier/);
});

test('a source claim needs the author\'s check mark', () => {
  const noMark = ['2', 'revenue doubled', 'source', 'https://example.com', 'confirmed', '', ''];
  assert.match(checkLedger(ledger({ claims: [noMark] }), piece, null).join('\n'), /needs the author's ✓/);
});

test('a blocked source the author checked by hand passes', () => {
  const blocked = ['3', 'the ruling cites clause 4', 'source', 'blocked; the court PDF used', 'blocked', '✓', 'read the PDF'];
  assert.deepEqual(checkLedger(ledger({ claims: [blocked] }), piece, null), []);
});

test('an author ✗ is refused', () => {
  const wrong = ['2', 'revenue doubled', 'source', 'https://example.com', 'wrong', '✗', ''];
  assert.match(checkLedger(ledger({ claims: [wrong] }), piece, null).join('\n'), /marked it wrong/);
});

test('an unconfirmed repository claim needs the author\'s ✓ and a note', () => {
  const unver = ['1', 'the cache halves latency', 'repo', 'no benchmark in the repo', 'unverifiable', '✓', ''];
  assert.match(checkLedger(ledger({ claims: [unver] }), piece, null).join('\n'), /needs the author's ✓ and a note/);
  unver[6] = 'measured by hand, numbers in the footnote';
  assert.deepEqual(checkLedger(ledger({ claims: [unver] }), piece, null), []);
});

test('tier flags: unresolved refused; a client flag kept refused; signed-off passes', () => {
  assert.match(checkLedger(ledger({ tier: [['1', 's', 'work', 'why', '']] }), piece, null).join('\n'), /kept or changed/);
  assert.match(checkLedger(ledger({ tier: [['1', 's', 'client', 'why', 'kept']] }), piece, null).join('\n'), /signed-off or changed/);
  assert.deepEqual(checkLedger(ledger({ tier: [['1', 's', 'client', 'signed off by the client, 2026-10-01', 'signed-off']] }), piece, null), []);
});

test('a translation equal to its model pass is refused', () => {
  assert.match(checkLedger(ledger(), piece, piece).join('\n'), /untouched model pass/);
  assert.deepEqual(checkLedger(ledger(), piece, piece.replace('Body.', 'Text.')), []);
});

test('columns in any order, escaped pipes, capitals', () => {
  const md = ledger({
    claimsHeader: '| Author | # | Kind | Agent | Claim | Evidence | Note |',
    claims: [['✓', '1', 'Source', 'Confirmed', 'a \\| b split', 'https://example.com', '']],
  });
  assert.deepEqual(checkLedger(md, piece, null), []);
  assert.equal(table(md, 'Claims')[0].claim, 'a | b split');
});

test('splitRow keeps empty cells and honours \\|', () => {
  assert.deepEqual(splitRow('| a \\| b | c |  |'), ['a | b', 'c', '']);
});

test('pieceKey and pieceLang', () => {
  assert.equal(pieceKey(piece), 'k');
  assert.equal(pieceKey(piece.replaceAll('\n', '\r\n')), 'k');
  assert.equal(pieceKey('no front matter'), '');
  assert.equal(pieceLang('content/ro/un-titlu.md'), 'ro');
  assert.equal(pieceLang('content/fr/x.md'), '');
});

const keyOf = (p) => ({ 'content/en/a-piece.md': 'k', 'content/ro/o-piesa.md': 'k', 'content/en/other.md': 'j' })[p] ?? '';

test('scope: one key\'s pieces and images pass', () => {
  const r = checkScope([{ status: 'A', path: 'content/en/a-piece.md' }, { status: 'M', path: 'content/ro/o-piesa.md' }, { status: 'A', path: 'assets/img/k/sub/chart.svg' }], keyOf);
  assert.deepEqual(r.probs, []);
  assert.deepEqual(r.pieces, ['content/en/a-piece.md', 'content/ro/o-piesa.md']);
});

test('scope: anything else, two keys, or no piece is refused', () => {
  assert.match(checkScope([{ status: 'A', path: 'content/en/a-piece.md' }, { status: 'M', path: 'templates/piece.html' }], keyOf).probs.join('\n'), /templates\/piece.html: a piece branch carries/);
  assert.match(checkScope([{ status: 'M', path: 'content/en/_home.md' }], keyOf).probs.join('\n'), /_home.md: a piece branch carries/);
  assert.match(checkScope([{ status: 'A', path: 'content/en/a-piece.md' }, { status: 'A', path: 'content/en/other.md' }], keyOf).probs.join('\n'), /more than one key: j, k/);
  assert.match(checkScope([{ status: 'A', path: 'assets/img/k/x.png' }], keyOf).probs.join('\n'), /no piece file changed/);
});

test('scope: deletions and renames are refused', () => {
  assert.match(checkScope([{ status: 'D', path: 'content/en/a-piece.md' }], keyOf).probs.join('\n'), /deleted; published URLs never change/);
  assert.match(checkScope([{ status: 'R', path: 'content/en/a-piece.md' }], keyOf).probs.join('\n'), /renamed or copied/);
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd .claude/skills/publish-piece && node --test; cd -`
Expected: FAIL — `Cannot find module '../gate.mjs'`.

- [ ] **Step 3: Implement** — create `.claude/skills/publish-piece/gate.mjs`:

```js
// The publish gate: what /publish-piece refuses before anything reaches origin. Node standard library only.
// From the repository root:
//   node .claude/skills/publish-piece/gate.mjs path   <piece.md>       the piece's ledger path
//   node .claude/skills/publish-piece/gate.mjs hash   <piece.md>       sha256:<hex> of the piece file
//   node .claude/skills/publish-piece/gate.mjs scope  [--base <ref>]   the branch changes one key's content only
//   node .claude/skills/publish-piece/gate.mjs ledger <piece.md>...    every mark the author owes is there
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { existsSync, readFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

export const sha256 = (bytes) => 'sha256:' + createHash('sha256').update(bytes).digest('hex');

// pieceKey reads `key:` from a piece's front matter; '' when absent.
export function pieceKey(src) {
  const fm = /^---\r?\n([\s\S]*?)\r?\n---\r?\n/.exec(String(src));
  const k = fm && /^key:[ \t]*["']?([^"'\s]+)["']?[ \t]*\r?$/m.exec(fm[1]);
  return k ? k[1] : '';
}

// pieceLang reads the language from content/<lang>/<slug>.md; '' when the path is not a piece.
export function pieceLang(path) {
  const m = /(?:^|\/)content\/(en|ro)\/[a-z0-9-]+\.md$/.exec(path);
  return m ? m[1] : '';
}

// splitRow splits one Markdown table row into trimmed cells; `\|` is a literal pipe inside a cell.
export function splitRow(line) {
  let s = line.trim();
  if (s.startsWith('|')) s = s.slice(1);
  const cells = [];
  let cur = '';
  for (let i = 0; i < s.length; i++) {
    if (s[i] === '\\' && s[i + 1] === '|') { cur += '|'; i++; continue; }
    if (s[i] === '|') { cells.push(cur.trim()); cur = ''; continue; }
    cur += s[i];
  }
  if (cur.trim() !== '') cells.push(cur.trim());
  return cells;
}

// table returns the rows of the first Markdown table under `## <heading>`, keyed by lower-cased header, or null
// when the heading is absent. The table ends at its first non-table line or at the next `## ` heading.
export function table(md, heading) {
  const lines = md.split(/\r?\n/);
  const start = lines.findIndex((l) => l.trim().toLowerCase() === `## ${heading.toLowerCase()}`);
  if (start < 0) return null;
  const rows = [];
  let header = null;
  for (let i = start + 1; i < lines.length; i++) {
    const l = lines[i].trim();
    if (l.startsWith('## ')) break;
    if (!l.startsWith('|')) { if (header) break; continue; }
    const cells = splitRow(l);
    if (!header) { header = cells.map((c) => c.toLowerCase()); continue; }
    if (cells.every((c) => /^:?-{3,}:?$/.test(c))) continue;
    rows.push(Object.fromEntries(header.map((h, j) => [h, cells[j] ?? ''])));
  }
  return rows;
}

const AGENT = ['confirmed', 'wrong', 'unverifiable', 'blocked'];
const RESOLUTIONS = { work: ['kept', 'changed'], client: ['signed-off', 'changed'] };

// checkLedger returns every reason the gate refuses one piece, given the ledger's text, the piece's bytes and the
// translation's saved model pass (null when the piece is not a translation).
export function checkLedger(ledger, piece, modelPass) {
  const probs = [];
  const h = /^hash:[ \t]*(sha256:[0-9a-f]{64})[ \t]*$/m.exec(ledger);
  if (!h) probs.push('the ledger has no hash: line');
  else if (h[1] !== sha256(piece)) probs.push('the piece changed since the review (the hash differs); re-run /review-piece');
  const claims = table(ledger, 'Claims');
  if (claims === null) probs.push('the ledger has no ## Claims section');
  for (const c of claims ?? []) {
    const id = `claim ${c['#'] || '?'}`;
    const kind = (c.kind ?? '').toLowerCase(), agent = (c.agent ?? '').toLowerCase(), author = c.author ?? '', note = c.note ?? '';
    if (kind !== 'repo' && kind !== 'source') { probs.push(`${id}: kind ${JSON.stringify(c.kind)} is not repo or source`); continue; }
    if (!AGENT.includes(agent)) probs.push(`${id}: agent verdict ${JSON.stringify(c.agent)} is not one of ${AGENT.join(', ')}`);
    if (!['✓', '✗', ''].includes(author)) probs.push(`${id}: author mark ${JSON.stringify(author)} is not ✓, ✗ or empty`);
    else if (author === '✗') probs.push(`${id}: the author marked it wrong; fix the piece and re-run /review-piece`);
    else if (kind === 'source' && author !== '✓') probs.push(`${id}: a source claim needs the author's ✓ (checked by hand against the source)`);
    else if (kind === 'repo' && agent !== 'confirmed' && !(author === '✓' && note !== '')) probs.push(`${id}: a repository claim the agent did not confirm needs the author's ✓ and a note`);
  }
  const tier = table(ledger, 'Tier');
  if (tier === null) probs.push('the ledger has no ## Tier section');
  for (const f of tier ?? []) {
    const id = `tier flag ${f['#'] || '?'}`;
    const t = (f.tier ?? '').toLowerCase(), r = (f.resolution ?? '').toLowerCase();
    const ok = RESOLUTIONS[t];
    if (!ok) probs.push(`${id}: tier ${JSON.stringify(f.tier)} is not work or client`);
    else if (!ok.includes(r)) probs.push(`${id}: a ${t} flag resolves as ${ok.join(' or ')}, not ${JSON.stringify(f.resolution ?? '')}`);
  }
  if (modelPass !== null && Buffer.compare(Buffer.from(modelPass), Buffer.from(piece)) === 0) {
    probs.push('the translation is the untouched model pass; rewrite it first');
  }
  return probs;
}

// checkScope returns every reason a branch is not a piece branch. `changes` are `git diff --name-status` entries
// ({status: first letter, path: last path}) against the base; `keyOf(path)` reads a changed piece's key.
export function checkScope(changes, keyOf) {
  const probs = [];
  const keys = new Set();
  const pieces = [];
  for (const { status, path } of changes) {
    if (status === 'D') { probs.push(`${path}: deleted; published URLs never change and pieces are never deleted`); continue; }
    if (status !== 'A' && status !== 'M') { probs.push(`${path}: renamed or copied; published URLs never change`); continue; }
    const img = /^assets\/img\/([a-z0-9-]+)\/.+$/.exec(path);
    if (pieceLang(path)) {
      pieces.push(path);
      const k = keyOf(path);
      if (k) keys.add(k); else probs.push(`${path}: no key: in the front matter`);
    } else if (img) keys.add(img[1]);
    else probs.push(`${path}: a piece branch carries content/<lang>/<slug>.md and assets/img/<key>/ only`);
  }
  if (pieces.length === 0) probs.push('no piece file changed');
  if (keys.size > 1) probs.push(`more than one key: ${[...keys].sort().join(', ')}`);
  return { probs, pieces };
}

const git = (...args) => execFileSync('git', args, { encoding: 'utf8' }).trim();
const ledgerDir = (key) => join(resolve(git('rev-parse', '--git-common-dir')), 'review', key);

function main(argv) {
  const [cmd, ...rest] = argv;
  if ((cmd === 'path' || cmd === 'hash') && rest.length === 1) {
    const bytes = readFileSync(rest[0]);
    if (cmd === 'hash') { console.log(sha256(bytes)); return 0; }
    const key = pieceKey(bytes), lang = pieceLang(rest[0]);
    if (!key || !lang) { console.error(`gate: ${rest[0]}: not a piece (content/<en|ro>/<slug>.md with a key:)`); return 2; }
    console.log(join(ledgerDir(key), `ledger-${lang}.md`));
    return 0;
  }
  if (cmd === 'scope' && (rest.length === 0 || (rest.length === 2 && rest[0] === '--base'))) {
    const base = rest[1] ?? 'origin/main';
    const out = git('diff', '--name-status', `${base}...HEAD`);
    const changes = out ? out.split('\n').map((l) => { const f = l.split('\t'); return { status: f[0][0], path: f[f.length - 1] }; }) : [];
    const { probs, pieces } = checkScope(changes, (p) => pieceKey(readFileSync(p)));
    for (const p of probs) console.error(`gate: scope: ${p}`);
    if (probs.length) return 1;
    for (const p of pieces) console.log(p);
    return 0;
  }
  if (cmd === 'ledger' && rest.length > 0) {
    let refused = 0;
    for (const file of rest) {
      const piece = readFileSync(file);
      const dir = ledgerDir(pieceKey(piece)), lang = pieceLang(file);
      const lp = join(dir, `ledger-${lang}.md`), mp = join(dir, `model-pass-${lang}.md`);
      const probs = existsSync(lp)
        ? checkLedger(readFileSync(lp, 'utf8'), piece, existsSync(mp) ? readFileSync(mp) : null)
        : [`no ledger at ${lp}; run /review-piece`];
      for (const p of probs) console.error(`gate: ${file}: ${p}`);
      if (probs.length) refused++; else console.log(`gate: ${file}: ok`);
    }
    return refused ? 1 : 0;
  }
  console.error('usage: gate.mjs path <piece> | hash <piece> | scope [--base <ref>] | ledger <piece>...');
  return 2;
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) process.exitCode = main(process.argv.slice(2));
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd .claude/skills/publish-piece && node --test; cd -`
Expected: PASS — every test.
Run: `node .claude/skills/publish-piece/gate.mjs hash testdata/site/content/en/paired.md && node .claude/skills/publish-piece/gate.mjs path testdata/site/content/en/paired.md; node .claude/skills/publish-piece/gate.mjs; echo "exit $?"`
Expected: a `sha256:` line; a path ending in `/review/<the fixture's key>/ledger-en.md` inside the shared git directory (nothing is created); the usage line and `exit 2`.

- [ ] **Step 5: Wire CI and the hook** — in `.github/workflows/ci.yml`, in the `build` job, right after `- run: cd worker && node --test`, add:

```yaml
      - run: cd .claude/skills/publish-piece && node --test
```

In `.githooks/pre-commit`, after the `worker/` block, add:

```bash
if git diff --cached --name-only | grep -q '^\.claude/'; then
  (cd .claude/skills/publish-piece && node --test)
fi
```

Run: `go tool actionlint`
Expected: no output.

- [ ] **Step 6: Commit**

```bash
git add .claude/skills/publish-piece/gate.mjs .claude/skills/publish-piece/test/gate.test.mjs .github/workflows/ci.yml .githooks/pre-commit
git commit -F - <<'EOF'
M2b: publish gate — gate.mjs refuses without the author's marks

WHY: the disclosure says the author checks by hand every claim no
repository can settle. The gate makes that a precondition of publishing:
no ledger, a ledger older than the piece, a source claim without the
author's ✓, an unresolved tier flag, a client flag kept without sign-off,
or an untouched model-pass translation, and /publish-piece stops. It also
refuses a piece branch that touches anything but one key's content, or
deletes or renames a file. Standard library only; CI and the hook run
its tests.

Co-Authored-By: Claude
EOF
```

---

### Task 6: `/review-piece`

**Files:**
- Create: `.claude/skills/review-piece/SKILL.md`

**Interfaces:**
- Consumes: `.cache/site check --draft` (Task 1); `gate.mjs path|hash` (Task 5); the ledger format.
- Produces: the ledger `ledger-<lang>.md` in the format `gate.mjs ledger` parses; proposals for the LinkedIn post (the author writes `linkedin-<lang>.md`).

- [ ] **Step 1: Write the skill** — create `.claude/skills/review-piece/SKILL.md` with exactly:

````markdown
---
name: review-piece
description: Review a draft piece before publication — mechanical checks, tier check, shape, an editing pass under the one-sentence rule, a fact-check by a fresh agent, and help with the LinkedIn post the author writes. Use on a piece/* branch when the author asks for a review.
argument-hint: content/<en|ro>/<slug>.md
---

# /review-piece

Reviews one piece and writes a private ledger. The author decides everything; this skill finds, checks and proposes.

## Rules

1. **Never write to `content/`.** Propose; the author applies. A reformulation is at most one sentence, written as an exact *before → after*; never a rewritten paragraph. The ledger records the piece's hash, so an edit the author did not make shows.
2. The ledger never enters a commit, a pull request or any public place. It lives in the shared git directory.
3. The examples in this file are invented. Never quote a draft or a ledger into a file in this repository.

## Before the passes

- The branch is `piece/*`; the argument is `content/<en|ro>/<slug>.md`.
- `LEDGER=$(node .claude/skills/publish-piece/gate.mjs path <piece>)` and `mkdir -p "$(dirname "$LEDGER")"`.
- If `$LEDGER` exists, read it: this is a re-run (see "Re-runs").
- On the first run, ask the author which repositories the piece is about — this one, and any other it names — and their local paths. They go on the `repos:` line and stay private.
- `go build -tags nodynamic -o .cache/site ./cmd/site`.

## The passes, in order

### 1. Mechanical
- `.cache/site check --draft` — report every problem with its `file:line`.
- Words in the body, front matter and footnote definitions excluded:
  `awk '/^---$/{fm++; next} fm>=2 && !/^\[\^[^]]+\]:/' <piece> | wc -w` — report it against 1,500–3,000. Reported, never a failure.
- Every footnote reference `[^x]` has a definition `[^x]:`, and every definition is referenced.
- No embeds: no `<iframe>`, `<script>`, `<video>`, `<object>`, `<embed>`. A video is a self-hosted thumbnail linking out.
- Every fragment in a language other than the piece's sits inside `<span lang="…">…</span>` — an English phrase in a Romanian piece, a Romanian name of an institution in an English one. List each unmarked fragment with its line.
- Romanian: ș ț Ș Ț with comma below (`check` already refuses the cedilla forms); list words that look like they are missing ă, â or î.

### 2. Tier, by judgement
An **own project** needs no approval. A **generalised lesson from work** carries no names and no numbers. A **named client** needs the client's sign-off first. Read every sentence that touches work other than the author's own projects; flag each one that a person at that organisation could read as a disclosure. Quote it, name the tier (`work` or `client`), say why in one line. There is no list of forbidden names: this is judgement.
Invented example: *"Our warehouse system loses a day of orders at every quarter-end."* — tier `work`: an internal system and a failure an employer's board would recognise.

### 3. Shape
The editorial template is situation → trade-off → decision → what I'd change. Say which parts the piece has and which it lacks, as a note. It is not a rule.

### 4. Editing, under the one-sentence rule
Flags, questions and suggestions, each keyed by line: an unclear sentence, a strong claim without support, a term used two ways, a repetition, a claim that needs a hedge. A proposed reformulation is one sentence, as *before → after*, copied exactly so that applying it is a paste.

### 5. Fact-check, by a fresh agent
Dispatch a new subagent that has not seen passes 1–4. Give it the piece's path, the repositories and their paths, the Claims table header below, and these instructions:
- Extract every checkable claim, footnoted or not: figures, dates, names, quotations, what code does, what a commit changed.
- **Repository claims** — the code or its history can settle them: read the code, the commits, the logs. Agent `confirmed`, `wrong` or `unverifiable`; Evidence is `path:line` or a commit hash.
- **Source claims** — everything else: fetch the cited URL. Evidence is the URL, the passage quoted exactly, and the date the page gives. Agent `confirmed`, `wrong`, `unverifiable`, or `blocked` when the site refuses the fetch — then find the primary document or a second outlet that states the same thing, and say in Evidence which one was used.
- Leave Author and Note empty. Never edit the piece.
Put the claims it returns into the Claims table.

For a translation (a `model-pass-<lang>.md` sits beside the ledger), also give the agent the source-language ledger (`ledger-<other lang>.md`): it matches each claim to its original, and every figure, date and name must cross unchanged. A mismatch is `wrong`, with both versions in Evidence.

### 6. LinkedIn post — the author writes it
Propose, in the ledger's LinkedIn section: the angle in one line, three candidate opening lines, and the points from the piece worth carrying over. The author writes the post in `linkedin-<lang>.md` beside the ledger. When that file exists, check it: native and complete, not a teaser; at most 3,000 characters (`wc -m`); the piece's URL with `?ref=li` kept for the first comment, not in the post. Propose single-sentence reformulations only, as in pass 4.

## The ledger

Write or update `$LEDGER` in this shape (the gate reads the `hash:` line and the two tables; the rest is for the author):

    # Review — <title> (<lang>)

    hash: <output of: node .claude/skills/publish-piece/gate.mjs hash <piece>>
    repos: <path>, <path>

    ## Claims

    | # | Claim | Kind | Evidence | Agent | Author | Note |
    |---|---|---|---|---|---|---|

    ## Tier

    | # | Sentence | Tier | Why | Resolution |
    |---|---|---|---|---|

    ## Mechanical
    ## Shape
    ## Editing
    ## LinkedIn
    ## Rounds

- Kind: `repo` or `source`. Agent: `confirmed`, `wrong`, `unverifiable`, `blocked`.
- Author: `✓`, `✗` or empty — only the author fills it. Resolution: `kept` or `changed` for a `work` flag; `signed-off` (who and when, in Why) or `changed` for a `client` flag — only the author fills it.
- A `|` inside a cell is written `\|`.
- `## Rounds` gets one dated line per run: what changed since the last run.

## Re-runs
Re-extract the claims and flags from the current text. Keep the author's Author, Note and Resolution on claims and flags whose text is unchanged; a changed claim or flag starts empty again. Update the `hash:` line. The last run must follow the last edit: `/publish-piece` refuses a ledger whose hash is not the piece's.

## What to tell the author
The ledger's path; claims by verdict; claims waiting for the author's ✓; tier flags waiting for a resolution; the mechanical problems; that nothing in `content/` was changed.
````

- [ ] **Step 2: Verify**

Run: `head -5 .claude/skills/review-piece/SKILL.md && grep -n -E '/home/|claude\.ai/code/session' .claude/skills/review-piece/SKILL.md; echo "scan exit $?"`
Expected: the front matter with `name: review-piece`; `scan exit 1` (no match).

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/review-piece/SKILL.md
git commit -F - <<'EOF'
M2b: skills — /review-piece

Mechanical checks, tier flags by judgement, shape, editing under the
one-sentence rule, a fact-check by a fresh agent (repository claims with
file:line or commit; source claims fetched and quoted, the author's
column left empty), and help with the LinkedIn post the author writes.
It never writes to content/; the ledger stays in the shared git
directory.

Co-Authored-By: Claude
EOF
```

---

### Task 7: `/translate-piece` and its glossary

**Files:**
- Create: `.claude/skills/translate-piece/SKILL.md`
- Create: `.claude/skills/translate-piece/glossary.md`

**Interfaces:**
- Consumes: `gate.mjs path` (Task 5); `.cache/site check --draft` (Task 1).
- Produces: one new `content/<to>/<slug>.md`; `model-pass-<to>.md` beside the ledger (Task 5's `checkLedger` compares it).

- [ ] **Step 1: Write the skill** — create `.claude/skills/translate-piece/SKILL.md` with exactly:

````markdown
---
name: translate-piece
description: Write the first-pass translation of a piece into the other language (en ↔ ro) as a new file with the same key, for the author to rewrite. Use when the author asks for a translation.
argument-hint: content/<from>/<slug>.md <to>
---

# /translate-piece

Writes one new file: the model's first pass. The author rewrites it — the colophon says translations "start from a model's pass and are rewritten by me".

## Rules

1. This is the only skill that creates a file in `content/`, and only this one file, once. If the target exists, stop: never overwrite the author's rewrite.
2. The untouched pass is saved privately beside the ledger, so `/publish-piece` can refuse a translation nobody rewrote.
3. The examples here and in `glossary.md` are invented or are house terms. Never quote a draft into this repository.

## Steps

1. Read the source piece and `glossary.md` (beside this file).
2. Propose the target slug: lowercase ASCII words joined by `-`; Romanian letters without diacritics (`ș`→`s`, `ț`→`t`, `ă`/`â`→`a`, `î`→`i`). Ask the author to confirm or change it — a published URL never changes. The target is `content/<to>/<slug>.md`. If it exists, stop.
3. Translate:
   - Idiomatic, not literal: a reader of the target language should not feel the source language underneath. Keep the argument, its order, the facts and the author's register; write new sentences.
   - Front matter: the same `key` and `pillar`; `title` and `summary` translated, the summary at most 160 characters; `date:` empty; no `linkedin:`, `medium:` or `updated:`.
   - Follow `glossary.md`.
   - Quotations: translated in the body; the footnote keeps the original inside `<span lang="…">…</span>`.
   - Footnotes keep their sources and URLs; translate their prose.
   - Figures, dates and names cross unchanged (their format follows the glossary).
4. Write the file. Then save the same bytes as the model pass:
   `LEDGER=$(node .claude/skills/publish-piece/gate.mjs path content/<to>/<slug>.md)`;
   `mkdir -p "$(dirname "$LEDGER")"`;
   `cp content/<to>/<slug>.md "$(dirname "$LEDGER")/model-pass-<to>.md"`.
5. `go build -tags nodynamic -o .cache/site ./cmd/site && .cache/site check --draft` — report every problem, and the summary's length.
6. Tell the author: the new file, the model pass's path, the glossary rules applied, and every term you were unsure of, each with a proposed `glossary.md` line. Next: the author rewrites the file; then `/review-piece` on it.

## When the author corrects a pass
Propose the line to add to `glossary.md`: a term and its translation, or a rule. Terms and rules only — never a sentence from a draft.
````

Create `.claude/skills/translate-piece/glossary.md` with exactly:

```markdown
# Translation glossary

House rules and terms for `/translate-piece`. Terms and rules only, never a sentence from a draft. The author adds a line each time they correct a pass.

## Romanian (en → ro)

- Address the reader in the plural (*voi*, *vă*), never the singular *tu*.
- ș ț Ș Ț with comma below, never ş ţ with cedilla.
- Quotation marks are the typographer's job („…”): write straight quotes.
- Numbers: decimal comma, thousands point (3,5; 1.500).
- *incumbent* → *jucătorul consacrat*.

## English (ro → en)

- Spelling follows the existing English pieces; never mix British and American spelling within a piece.
```

- [ ] **Step 2: Verify**

Run: `head -5 .claude/skills/translate-piece/SKILL.md && grep -n -E '/home/|claude\.ai/code/session' .claude/skills/translate-piece/*.md; echo "scan exit $?"`
Expected: front matter with `name: translate-piece`; `scan exit 1` (the controller also runs its private-host scan, which names nothing here).

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/translate-piece/
git commit -F - <<'EOF'
M2b: skills — /translate-piece and the house glossary

One new file with the same key, never an overwrite; the untouched pass
is kept beside the ledger so /publish-piece can refuse a translation
nobody rewrote. The glossary holds house terms and rules only.

Co-Authored-By: Claude
EOF
```

---

### Task 8: `/publish-piece`

**Files:**
- Create: `.claude/skills/publish-piece/SKILL.md`

**Interfaces:**
- Consumes: `gate.mjs scope|ledger` (Task 5); `site stamp [--updated]`, `site check`, `site build`, `site check --dist` (Tasks 1, 3); `gh`; the `main` ruleset (covers `refs/heads/main` only, so `--force-with-lease` to a publish branch is allowed); squash settings `COMMIT_OR_PR_TITLE` / `COMMIT_MESSAGES`.
- Produces: the branch `publish/<slug>` or `correct/<slug>` on `origin` with one signed commit and an open PR; nothing else on `origin`.

- [ ] **Step 1: Write the skill** — create `.claude/skills/publish-piece/SKILL.md` with exactly:

````markdown
---
name: publish-piece
description: Publish a reviewed piece, or a correction to a published one — gate, stamp, check, one author-written commit on a branch built from origin/main, push, pull request. Re-stamps an open publish PR. Rehearses without committing. Only when the author asks.
argument-hint: "[--rehearse <base>]"
disable-model-invocation: true
---

# /publish-piece

Invoking this skill is the author's ask to reach `origin`. Nothing is pushed before the author has typed the commit body.

## Rules

1. Push only the publish branch (`publish/<slug>` or `correct/<slug>`), never `piece/<slug>`: a pull request keeps every commit public forever, and the draft history belongs on the private remote.
2. The commit carries the author's body and no trailer. The pull request carries no AI attribution line: the colophon's disclosure covers the process.
3. Stop at the first failed step; say what failed and how to fix it.

## Steps — from the piece's worktree, on `piece/<slug>`

1. **Clean tree:** `git status --porcelain` prints nothing; `git fetch origin`.
2. **Base:** `BASE=origin/main`; with `--rehearse <base>`, `BASE=<base>`.
3. **Scope:** `node .claude/skills/publish-piece/gate.mjs scope --base "$BASE"` — it prints the piece files, or refuses.
4. **Gate:** `node .claude/skills/publish-piece/gate.mjs ledger <each piece file>`. For a translation (a `model-pass-<lang>.md` beside its ledger), also show how much the author changed: `git diff --no-index --stat <model pass> <piece>`.
5. **Mode:** for each piece file, `git cat-file -e "$BASE:<file>"` fails → a publication; succeeds → a correction. Mixed → stop: publish and correct in separate runs.
6. **Binary:** `go build -tags nodynamic -o .cache/site ./cmd/site` and `SITE=$PWD/.cache/site`.
7. **Publish worktree:** `<slug>` is the first piece file's; `B=publish/<slug>` (a correction: `correct/<slug>`; a rehearsal: `rehearse/<slug>`); `W=.claude/worktrees/$(echo "$B" | tr / -)`.
   - `git rev-parse --verify --quiet "$B"` fails → `git worktree add -b "$B" "$W" "$BASE"`.
   - It succeeds and `gh pr view "$B" --json state -q .state` prints `OPEN` → a **re-stamp**: `git worktree add "$W" "$B"`.
   - It succeeds without an open PR → stop and ask the author (a leftover branch).
8. **Copy:** each piece file to `$W/<file>`; when `assets/img/<key>/` exists, `rm -rf "$W/assets/img/<key>" && mkdir -p "$W/assets/img" && cp -r assets/img/<key> "$W/assets/img/"`.
9. **Stamp:** in `$W`, `"$SITE" stamp <file>` for a publication, `"$SITE" stamp --updated <file>` for a correction.
10. **Verify:** in `$W`, `SOURCE_DATE_EPOCH=$(date +%s) "$SITE" check && SOURCE_DATE_EPOCH=$(date +%s) "$SITE" build && "$SITE" check --dist`. (The epoch is now: `$W`'s last commit predates today's stamp.)
11. **Rehearsal stops here:** show `git -C "$W" status --short` and the stamped dates; `git worktree remove --force "$W"`; `git branch -D "$B"`; say "rehearsal complete — nothing committed or pushed".
12. **The author's commit:** show the file list, `git -C "$W" diff --stat` and the stamped dates.
    - A publication or correction: ask *"Write the commit body in your words — it becomes the commit on main."* and wait. Typing it is the confirmation. Title: `Publish: <title>` or `Correct: <title>`, `<title>` from the piece's front matter (for a pair, the English title). Write title, a blank line and the body to a file outside the repository; `git -C "$W" add -A && git -C "$W" commit -F <that file>`. No trailer. The repository's git config signs it.
    - A re-stamp: `git -C "$W" add -A && git -C "$W" commit --amend --no-edit` — the author's message is kept; the PR stays one commit.
13. **Push:** `git -C "$W" push -u origin "$B"`; a re-stamp: `git -C "$W" push --force-with-lease origin "$B"`.
14. **Pull request** (not for a re-stamp): `gh pr create --base main --head "$B" --title "<commit title>" --body-file <file>` with the body

        Preview: CI comments the preview URL on this pull request once the build is up.
        Stamped <date> (Europe/Bucharest). Merge today, or re-run /publish-piece to re-stamp.
        Merge with squash: the commit on main is the author's.

15. **Clean up:** `git worktree remove "$W"` (the local branch stays, for a re-stamp). Tell the author: the PR's URL, the stamped date, and that `piece/<slug>` is untouched on the private remote.

## After the merge
Before the launch, the merge publishes the CI rows and deploys nothing; the launch commit re-stamps every piece dated before `launched:` (`site check` refuses them otherwise). After the launch, the piece is live a few minutes after the merge, once the audit is green. Then the author posts on LinkedIn, and `/link-piece` adds the post's URL.
````

- [ ] **Step 2: Verify**

Run: `head -6 .claude/skills/publish-piece/SKILL.md && grep -n -E '/home/|claude\.ai/code/session' .claude/skills/publish-piece/SKILL.md; echo "scan exit $?"`
Expected: front matter with `name: publish-piece` and `disable-model-invocation: true`; `scan exit 1` (the controller also runs its private-host scan, which names nothing here).

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/publish-piece/SKILL.md
git commit -F - <<'EOF'
M2b: skills — /publish-piece

WHY: a pull request keeps every commit public after a squash, so pushing
the draft branch would publish every draft sentence the review removed.
The skill builds a publish branch from origin/main in its own worktree,
copies the piece, stamps it (updated: for a correction), checks and
builds it, asks for the author's commit body, and pushes that one
commit. A re-stamp amends it; a rehearsal stops before the commit.

Co-Authored-By: Claude
EOF
```

---

### Task 9: `/link-piece`

**Files:**
- Create: `.claude/skills/link-piece/SKILL.md`

**Interfaces:**
- Consumes: `site link` and `site check` (Tasks 2–3); `gh`; `gh api repos/{owner}/{repo} --jq .allow_auto_merge` and the `main` ruleset's rules.
- Produces: `link/<slug>` on `origin` with one signed commit and a PR, auto-merged only when the repository allows it.

- [ ] **Step 1: Write the skill** — create `.claude/skills/link-piece/SKILL.md` with exactly:

````markdown
---
name: link-piece
description: Add the LinkedIn (or Medium) discussion URL to a published piece — one commit on a branch built from origin/main, a pull request, auto-merge when the repository allows it. Only when the author asks.
argument-hint: content/<lang>/<slug>.md <url>
disable-model-invocation: true
---

# /link-piece

Invoking this skill is the author's ask to reach `origin`.

## Rules

- The piece must already be on `origin/main`.
- The front matter changes only through `site link`; nothing else in the file changes. It is not a correction: `updated:` stays as it is.
- The commit carries no trailer.

## Steps — from any worktree of the repository

1. `git fetch origin`; `git cat-file -e origin/main:<file>` — if it fails, stop: the piece is not published.
2. `B=link/<slug>`; `W=.claude/worktrees/link-<slug>`; `git worktree add -b "$B" "$W" origin/main`.
3. `go build -tags nodynamic -o .cache/site ./cmd/site && SITE=$PWD/.cache/site`; then `(cd "$W" && "$SITE" link <file> <url> && SOURCE_DATE_EPOCH=$(date +%s) "$SITE" check)` — `site link` refuses any host but `www.linkedin.com` and `medium.com`.
4. `git -C "$W" commit -am "Link: <title> → LinkedIn"` (or `→ Medium`), `<title>` from the front matter. No trailer.
5. `git -C "$W" push -u origin "$B"`; `gh pr create --base main --head "$B" --title "Link: <title> → LinkedIn" --body "Adds the discussion link. The page changes, so the full audit runs."`.
6. Auto-merge only when both hold: `gh api repos/{owner}/{repo} --jq .allow_auto_merge` prints `true`, and `gh api repos/{owner}/{repo}/rulesets --jq '.[].id'` lists a ruleset whose rules (`gh api repos/{owner}/{repo}/rulesets/<id> --jq '.rules[].type'`) include `required_status_checks`. Then `gh pr merge "$B" --auto --squash`. Otherwise tell the author: "auto-merge is not set up; merge it yourself once CI is green."
7. `git worktree remove "$W"`. Tell the author the PR's URL and whether it will auto-merge.
````

- [ ] **Step 2: Verify**

Run: `head -6 .claude/skills/link-piece/SKILL.md && grep -n -E '/home/|claude\.ai/code/session' .claude/skills/link-piece/SKILL.md; echo "scan exit $?"`
Expected: front matter with `name: link-piece` and `disable-model-invocation: true`; `scan exit 1` (the controller also runs its private-host scan, which names nothing here).

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/link-piece/SKILL.md
git commit -F - <<'EOF'
M2b: skills — /link-piece

site link on a branch from origin/main, one commit, a pull request that
takes the full audit (the page changes). It auto-merges only when the
repository allows auto-merge and the main ruleset requires a status
check; until both exist the author merges it.

Co-Authored-By: Claude
EOF
```

---

### Task 10: `.claude/CLAUDE.md`, the RUNBOOK, and M2b → M2c

**Files:**
- Create: `.claude/CLAUDE.md`
- Modify: `RUNBOOK.md` (a new section after `## Generator`; lines 56 and 94)
- Modify: `bench/README.md:57`, `bench/lib/dns.mjs:18`, `docs/adr/0004-analytics-without-a-tracker.md:12`, `docs/adr/0011-tls13-minimum.md:23`

**Interfaces:**
- Consumes: every command and skill from Tasks 1–9.
- Produces: docs only.

- [ ] **Step 1: Write `.claude/CLAUDE.md`** with exactly:

````markdown
# herinean.com

This repository is the whole of herinean.com: a static site generator in Go, the edge Worker, the infrastructure as code, and the audit that scores every build. It is public, and so are these instructions.

**North star.** Anyone examining the site, its hosting or its stack scores it at the maximum on every measurable axis, continuously — and the platform is the least platform that scores perfect: nothing is added that a validator or a reader would not miss.

## Commands

    go build -tags nodynamic -o .cache/site ./cmd/site   build the CLI (never `go run`: it hides exit code 3)
    .cache/site build                     content/ + assets/ + templates/ + i18n/ → dist/
    .cache/site check [--draft|--dist]    validate the sources (--draft: the blanks `site new` leaves are warnings) or dist/
    .cache/site serve [--static]          local preview; --static serves dist/ as built (what CI audits)
    .cache/site new <en|ro> <slug>        scaffold a piece
    .cache/site stamp [--updated] <file>  today's date (Europe/Bucharest) into date: (or updated:)
    .cache/site link <file> <url>         linkedin: or medium:, by the URL's host
    .cache/site scorecard …               the colophon's scorecard fragment (CI)
    scripts/bench.sh                      the CI audit, locally

Every Go command takes `-tags nodynamic`. `site check` exit 3 means author inputs are missing: the pre-commit hook allows it, CI does not.

## Rules

- Every change keeps the scorecard green (`docs/specs/2026-09-17-herinean-com-design.md` §3). Before a commit: `gofmt`, `go vet`, `go tool staticcheck ./...`, `go test ./...`; the pre-commit hook (installed by `scripts/setup.sh`) runs what the staged files need.
- Commits are signed. A message says what changed and, when the diff does not show it, why. It ends with exactly `Co-Authored-By: Claude` — no model name, no version, no session link — and this holds for subagents too, over any trailer their own harness suggests. `Publish:`, `Correct:` and `Link:` commits carry no trailer: they are the author's acts on an article.
- Two remotes: a private one for drafts (every `piece/*` branch) and `origin` on GitHub, public. `origin` is reached only through `/publish-piece` and `/link-piece`, or by the owner.
- A `piece/*` branch carries content only: `content/<lang>/<slug>.md` files sharing one key, and `assets/img/<key>/`.
- The model never edits `content/` by hand. Front-matter fields change through `site stamp` and `site link`; `/translate-piece` creates one new file, once. Everything else in a piece is the author's.
- Review ledgers are private: they live in the shared git directory (`node .claude/skills/publish-piece/gate.mjs path <piece>`) and never enter a commit, a pull request or this repository.
- Examples in skills and docs are invented. Nothing from a draft, a ledger or the author's work goes into a file here as an example.
- URLs never change and pieces are never deleted: a correction sets `updated:`; a retraction is a note at the top.
- No private host, local path or session link in any file.

## Where things are

- Design: `docs/specs/2026-09-17-herinean-com-design.md`, milestone designs beside it; plans: `docs/plans/`; decisions: `docs/adr/`.
- Operations: `RUNBOOK.md`. The audit: `bench/` and its README. The skills: `.claude/skills/`.
````

- [ ] **Step 2: The RUNBOOK section** — in `RUNBOOK.md`, insert before `## TLS`:

```markdown
## Writing and publishing

- Draft on `piece/<slug>`, branched from `main` and tracking the private remote; commit and push there as often as you like. The hook runs `site check --draft` on `piece/*` branches, so blank title, date, pillar and summary are warnings until publication. Write on `site serve`: a failing rule keeps the last good render on screen and prints the problem.
- `/review-piece content/<lang>/<slug>.md` — mechanical checks, tier flags, shape, editing proposals (one sentence at most, never applied for you), a fact-check by a fresh agent, and help with the LinkedIn post, which you write. It never edits the piece. Re-run it after every round of edits; the last run must follow the last edit.
- Ledgers: `node .claude/skills/publish-piece/gate.mjs path <piece>` prints the path — inside the shared git directory, so every worktree sees it and removing a worktree keeps it. Mark each source claim `✓` after checking it by hand against its source; resolve each tier flag (`kept`/`changed`; a client flag `signed-off`/`changed`). They are private and live on one disk: back them up now and then with `cp -r "$(git rev-parse --git-common-dir)/review" ~/.config/herinean/review-$(date -u +%Y%m%dT%H%M%SZ)`.
- `/translate-piece content/<from>/<slug>.md <to>` — writes the first pass as a new file (never over an existing one) and keeps the untouched pass beside the ledger; rewrite it, then review it. House terms: `.claude/skills/translate-piece/glossary.md`.
- `/publish-piece` — refuses when the branch touches anything but one key's content, when a ledger is missing or older than the piece, when a source claim lacks your ✓, a tier flag is unresolved, or a translation is the untouched model pass. Then it stamps the date on `publish/<slug>`, a branch built from `origin/main` (never the piece branch), runs check/build/check --dist, asks for your commit body, pushes and opens the PR. Merge the same day, or re-run `/publish-piece` to re-stamp (it amends the one commit). Merge with squash. `/publish-piece --rehearse <base>` does everything up to the body prompt against another base and commits nothing.
- Corrections: the same skill, on a piece already on `main` — it stamps `updated:` instead of `date:`, and the commit reads `Correct: <title>`; feeds keep the original date.
- `/link-piece content/<lang>/<slug>.md <url>` once the LinkedIn post is up — one commit on `link/<slug>` and a PR. It auto-merges once the `main` ruleset requires a status check and auto-merge is enabled in the repository settings; until then, merge it yourself when CI is green.
- Launch day: set `launched:`; `site check` then refuses pieces dated earlier, so `site stamp` each of them in the launch commit — `datePublished` becomes launch day.
```

- [ ] **Step 3: M2b → M2c** — the weekly job moved to M2c. Apply exactly:
  - `RUNBOOK.md:56`: `Weekly snapshots to KV and the merge are M2b.` → `Weekly snapshots to KV and the merge are M2c.`
  - `RUNBOOK.md:94`: `(M2b)` → `(M2c)`.
  - `bench/README.md:57`: the `M2b` in that row → `M2c`.
  - `bench/lib/dns.mjs:18`: `// (Domain expiry via RDAP is the weekly job's, M2b.)` → `// (Domain expiry via RDAP is the weekly job's, M2c.)`
  - `docs/adr/0004-analytics-without-a-tracker.md:12`: `(M2b)` → `(M2c)`.
  - `docs/adr/0011-tls13-minimum.md:23`: `verify.yml` (M2b)` → `verify.yml` (M2c)`.

Run: `grep -rn 'M2b' RUNBOOK.md README.md bench/ docs/adr/ internal/ scripts/ --exclude-dir=node_modules`
Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add .claude/CLAUDE.md RUNBOOK.md bench/README.md bench/lib/dns.mjs docs/adr/0004-analytics-without-a-tracker.md docs/adr/0011-tls13-minimum.md
git commit -F - <<'EOF'
M2b: docs — the repository CLAUDE.md; RUNBOOK writing and publishing

The public CLAUDE.md states the rules any agent working here follows:
the commit trailer (subagents included), origin only through the two
publishing skills, piece branches carry content only, the model never
edits content/ by hand, ledgers stay private, examples are invented.
The weekly job is M2c now, so its mentions say so.

Co-Authored-By: Claude
EOF
```

---

### Task 11: parent spec amendments (one commit)

**Files:**
- Modify: `docs/specs/2026-09-17-herinean-com-design.md` (§4.2, §5, §5.2, §7, §8, §11, §13)

**Interfaces:**
- Consumes: the text of §8 after `colophon/one-ai-disclosure` merged (its step 3 names the editing pass and the fact-check against repositories; its step 5 has "body written by the author").
- Produces: docs only.

- [ ] **Step 1: Precondition** — the disclosure branch must be in this branch's history:

Run: `grep -c 'body written by the author' docs/specs/2026-09-17-herinean-com-design.md`
Expected: `1`. If it prints `0`, stop: record in the ledger that Task 11 waits for `colophon/one-ai-disclosure` to merge and M2b to be rebased; go on to Task 12 and come back.

- [ ] **Step 2: Apply the amendments** — each replacement exactly:
  - §4.2, the `date:` comment: `# set by /publish-piece; Europe/Bucharest; must be ≤ commit date` → `# set by site stamp (/publish-piece); Europe/Bucharest; ≤ commit date and ≥ launched:`
  - §4.2, the `linkedin:` line: `# optional; added by /link-piece; renders "Also on LinkedIn"` → `# optional; set by site link (/link-piece); host www.linkedin.com; renders "Also on LinkedIn"`
  - §4.2, the `medium:` line: `# optional; renders "· Medium"; Medium import sets canonical here` → `# optional; set by site link; host medium.com; renders "· Medium"; Medium import sets canonical here`
  - §5: `One Go module, one binary `site`, five verbs:` → `One Go module, one binary `site`, seven verbs:`; in the block, after the `site check --dist` line add
    ```
    site check --draft  as check, but the blanks `site new` leaves are warnings     pre-commit on piece/*
    ```
    append to the `site serve` line `; a failing rule keeps the last good build on screen`; after the `site new` line add
    ```
    site stamp [--updated] <file>   today's date (Europe/Bucharest) into date: (or updated:)   /publish-piece
    site link <file> <url>   linkedin: or medium:, by the URL's host                             /link-piece
    ```
  - §5.2: `Missing/invalid front matter; empty `date`;` → `Missing/invalid front matter; empty `date` (with blank title, pillar and summary, a warning under `--draft`); `date` before `launched:`; `linkedin:`/`medium:` not an https URL on www.linkedin.com / medium.com;`
  - §7: `lychee (weekly job, M2b)` → `lychee (weekly job, M2c)`.
  - §8 step 3: `and a **native LinkedIn post** (complete, not a teaser; link goes in the first comment).` → `and help with the **native LinkedIn post**, which the author writes (complete, not a teaser; link goes in the first comment). Results go to a private ledger in the shared git directory; the skill never edits the piece.` — and in the same step `must exist before launch; M2)` → `must exist before launch; M2b)`.
  - §8 step 4: replace the step with `4. `/publish-piece` — refuses without the author's marks in the ledger (every source claim ✓, every tier flag resolved) or with a ledger older than the piece; stamps `date` (Europe/Bucharest) on a one-commit `publish/<slug>` branch built from `origin/main` — the draft history never reaches `origin` — runs `check`, asks for the commit body in the author's words, pushes, opens the PR. CI audits and comments the preview (the only real-edge preview; the cost of private drafts).`
  - §8 step 5: replace the step with `5. Squash-merge: the commit is already the author's `Publish: <title>`, with no co-author line (that belongs on the commits that built the site, not on the one that introduces an article). Live a few minutes after the merge, once the audit is green (ADR-0015).`
  - §8 step 6: `→ `Link: <title> → LinkedIn` PR, auto-merge.` → `→ `Link: <title> → LinkedIn` PR, auto-merged once the `main` ruleset requires the CI check.`
  - §8 step 8: `Corrections set `updated:`; feeds don't re-notify.` → `Corrections go through `/publish-piece` too: it stamps `updated:`, and the commit reads `Correct: <title>`; feeds don't re-notify.`
  - §8 closing paragraph: `runs `gofmt` + `site check`.` → `runs `gofmt` + `site check` (`--draft` on `piece/*` branches).`
  - §11, the M2 bullet's parenthesis: `(split into M2a — CI, bench, scorecard pipeline — and M2b — weekly verify, uptime, skills, CodeQL; design: `docs/specs/2026-09-19-m2a-ci-bench-scorecard-design.md`)` → `(split into M2a — CI, bench, scorecard pipeline; M2b — authoring: the four skills, `CLAUDE.md`, drafts that commit; M2c — weekly verify, uptime, build id, CodeQL and OpenSSF Scorecard; M2d — fonts and Worker tests. Designs: `docs/specs/2026-09-19-m2a-ci-bench-scorecard-design.md`, `docs/specs/2026-09-26-m2b-authoring-design.md`)`
  - §13: `cmd/site/                 CLI (build, check, serve, new)` → `cmd/site/                 CLI (build, check, serve, new, scorecard, stamp, link)`; `ci.yml, audit.yml (M2a); verify.yml, codeql.yml (M2b)` → `ci.yml, audit.yml (M2a); verify.yml, codeql.yml (M2c)`; `CLAUDE.md, skills: review-piece, translate-piece, publish-piece, link-piece` → `CLAUDE.md, skills/: review-piece, translate-piece (+ glossary.md), publish-piece (+ gate.mjs, test/), link-piece`

Where a quoted "before" text is not found verbatim (the merged wording differs), apply the same change to the merged wording and record the exact before/after in the ledger.

- [ ] **Step 3: Verify**

Run: `grep -n 'M2b\|M2c\|site stamp\|site link\|--draft' docs/specs/2026-09-17-herinean-com-design.md`
Expected: the lines above and no other `M2b`/`M2c`.

- [ ] **Step 4: Commit**

```bash
git add docs/specs/2026-09-17-herinean-com-design.md
git commit -F - <<'EOF'
M2b: spec — authoring as built

WHY: the spec is the one description of the site; it must say what the
skills and the generator now do. §8's publish flow pushes a one-commit
branch, not the drafts, so the step about GitHub pre-filling the drafts'
messages goes; corrections, the LinkedIn post the author writes, the new
verbs and check rules, and the M2b/M2c/M2d split are recorded.

Co-Authored-By: Claude
EOF
```

---

### Task 12: acceptance — the first parked piece (controller with the author; not dispatched)

**Files:** none committed. Everything in this task is local and discarded, except the ledgers (private, in the shared git directory).

- [ ] **Step 1: A local rehearsal branch** — in this worktree: `git switch -c piece/acceptance`; copy the parked piece's English and Romanian files from its branch on the private remote (`git checkout <that branch> -- <the two files>`; the names are withheld here because the piece is unpublished and this plan is public) and commit locally (`git commit -m "acceptance draft"` — the hook runs `check --draft`). Never push this branch.
- [ ] **Step 2: Move the September ledger** into the new location as reference: `mkdir -p <ledger dir>` and copy the old `fact-check.md` there as `september-reference.md` (not a gate file).
- [ ] **Step 3: `/review-piece`** on the English file, then the Romanian one. Record: the ledger paths, claim counts by verdict, that `git status` shows no change under `content/`.
- [ ] **Step 4: The gate refuses** — `node .claude/skills/publish-piece/gate.mjs ledger <en file>` with the author's column still empty → exit 1 naming the source claims; append a space to the English file and run again → exit 1 with "changed since the review"; revert the space.
- [ ] **Step 5: The author marks** every source claim and resolves every tier flag (the author, by hand); `/review-piece` re-run after any edit; `gate.mjs ledger` → exit 0 for both files.
- [ ] **Step 6: Rehearse** — `/publish-piece --rehearse m2b/design` → scope prints the two files; the gate passes; mode = publication; the rehearsal worktree is stamped, `check`/`build`/`check --dist` pass; it stops before the body prompt and removes its worktree and branch.
- [ ] **Step 7: Clean up** — `git switch m2b/design && git branch -D piece/acceptance`; `git worktree list` shows no `rehearse-*` worktree; `git status` clean.
- [ ] **Step 8: Record** the evidence (commands and outputs) in the controller's landing report. The first real `/publish-piece` after M2b merges is the author's.
