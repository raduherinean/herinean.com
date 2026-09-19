# M1a — Generator Core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A single Go binary `site` that turns `content/`, `assets/`, `templates/` and `i18n/` into a complete, validated, deterministic `dist/` — every page, feed, machine file, OG image and the `_headers` file — readable locally with `site serve`, with the build refusing to produce anything that would fail the scorecard.

**Architecture:** One module, seven internal packages with one job each (`config`, `content`, `images`, `render`, `feeds`, `seo`, `edge`) orchestrated by `site`. Content is parsed once (goldmark AST), validated, then rendered with image information injected; templates are `html/template`; CSS is inlined and hashed for the CSP; output is written to a temp dir and swapped in atomically. Golden-file tests on a fixture site pin the HTML byte-for-byte.

**Tech Stack:** Go 1.27; `github.com/yuin/goldmark` (+ `goldmark-highlighting/v2`, `github.com/alecthomas/chroma/v2`); `gopkg.in/yaml.v3`; `golang.org/x/image` (draw, font, opentype, sfnt); `github.com/gen2brain/webp` (pure-Go WebP encoder). Nothing else. Templates and CSS are plain files.

**Spec:** `docs/specs/2026-09-17-herinean-com-design.md` — §4 (content model, URLs, page anatomy), §5 (generator, validation, tests), §6.2 (headers), §9 (design constraints, provisional CSS), §3 rows 2–8, 14, 16, 21.

**Status (2026-09-19):** executed 2026-09-18 on `m1a/generator` by subagent-driven development, each task reviewed and fixed before the next, and merged to `main` as PR #2 (merge commit `163673a`). The checkboxes below are the plan as written and were not ticked during execution; the branch history and the RUNBOOK are the record. Where the shipped code departs from the steps: the pre-commit hook builds a temporary binary because `go run` never propagates a child's exit code, so exit 3 was unobservable; `check` exits 3 only when placeholders or the missing portrait are the *only* problems, anything else is a real failure; `serve` renders a dateless draft with visible defaults and lists the blanks on stderr, while `build` and `check` still refuse it (`content.LoadDraft`, `site.Options.Draft`); WebP encoding refuses to run when a host `libwebp` was loaded, so every build uses `-tags nodynamic` (the dynamic path would make images depend on the machine); goldmark-highlighting drops chroma's `tabindex`, so the renderer wraps code in its own focusable `<pre>`; the colophon's dependency list comes from `debug.ReadBuildInfo`, not `go list`; `feed.json` carries the `items` array; the home meta description is `_home.md`'s summary; a body `<h1>`, a heading-level jump or a duplicate id fails `check`.

## Global Constraints

- Zero client-side JavaScript in any page. The only `<script>` allowed is `type="application/ld+json"`. No `style=` attributes anywhere (hash CSP). SVG uses `currentColor`.
- URLs: EN at root (`/writing/<slug>/`), RO under `/ro/` (`/ro/articole/<slug>/`); home `/` and `/ro/`; index `/writing/` and `/ro/articole/`; colophon `/colophon/` (EN only); privacy `/privacy/` and `/ro/confidentialitate/`; every URL ends with `/`; canonical absolute on `https://herinean.com`.
- Slugs ASCII `[a-z0-9-]` (transliterate ă→a ș→s ț→t â→a î→i). Romanian text uses comma-below ș ț (U+0219/U+021B, U+0218/U+021A); cedilla ş ţ (U+015F/U+0163, U+015E/U+0162) fail the build.
- Front matter: `title`, `date` (YYYY-MM-DD, Europe/Bucharest, ≤ commit date), `updated` (optional), `key`, `pillar` ∈ {build-log, opportunity, governance, analysis}, `summary` ≤ 160 chars, `linkedin`, `medium` (optional). No `draft` field.
- Feeds: RSS 2.0 `/feed.xml` (all), `/feed.en.xml`, `/feed.ro.xml`; JSON Feed 1.1 `/feed.json` (all). Full content, absolute URLs, `atom:link rel="self"`. EN pages advertise `feed.en.xml`, RO pages `feed.ro.xml`.
- Images: `assets/img/<key>/<name>` → `/img/<key>/<name>.<hash8>.<w>.webp` (+ original format at the largest width); widths 720 and 1440; explicit width/height; alt mandatory. OG: `/og/<lang>-<slug>.<hash8>.png`, 1200×630, < 200 KB.
- Build time = commit time (`SOURCE_DATE_EPOCH` if set, else `git log -1 --format=%ct`); `security.txt` `Expires` = build + 364 days; output files sorted, mtimes fixed to build time. Same commit ⇒ byte-identical `dist/`.
- Weight budget per content page: HTML+CSS ≤ 30 KB gzip-compressed. Fonts are M1b.
- Headers (`_headers`) exactly as spec §6.2, with `style-src 'sha256-<hash of the inlined CSS bytes>'`. **No `X-Robots-Tag` in `_headers`** (the Worker marks non-production hosts; production must be indexable).
- Commits: signed, `radu@herinean.com`, Claude co-author trailers; `main` via PR only; work branch `m1a/generator` on `gitea`, **branched from `infra/m0`** — `main` has five files; `.gitignore`, `RUNBOOK.md`, `worker/`, the design renders and the placeholder files this plan uses exist only on `infra/m0` (unmerged until `gh` is installed). The M1a PR targets `infra/m0`; GitHub retargets it to `main` when M0 merges.
- `site check` exits **3** when the only failures are ⟨author placeholders⟩ (`site.ErrAuthorInputs`); the pre-commit hook warns on 3 and blocks on anything else, so Tasks 5–12 can be committed before Radu's inputs arrive. CI (M2) treats 3 as failure.
- `gofmt`, `go vet`, `staticcheck` clean; every task ends with `go test ./...` green.

## Operator inputs

`site.yaml` (committed; nothing secret) — Radu supplies the values marked ⟨⟩ in Task 12:

```yaml
base_url: https://herinean.com
name: Radu Herinean
tagline:
  en: ⟨positioning line⟩
  ro: ⟨linia de poziționare⟩
author:
  linkedin: ⟨https://www.linkedin.com/in/…⟩
  x: ⟨https://x.com/…⟩
  github: https://github.com/rlucian
  email: security@herinean.com          # security.txt contact
ai_disclosure:
  en: ⟨one sentence⟩
  ro: ⟨o propoziție⟩
```

## File map

| Path | Responsibility |
|---|---|
| `go.mod`, `go.sum` | module `github.com/raduherinean/herinean.com`, Go 1.27, pinned deps |
| `cmd/site/main.go` | CLI: `build`, `check [--dist]`, `serve`, `new`, `scorecard` |
| `site.yaml` | site-wide values (author inputs) |
| `internal/config/config.go` | loads `site.yaml`; URL helpers (`PieceURL`, `IndexURL`, `HomeURL`, `Abs`) |
| `internal/content/{piece.go,frontmatter.go,slug.go,markdown.go,i18n.go,load.go,problems.go}` + tests | parse, validate, pair, render Markdown |
| `internal/images/{process.go,cache.go,og.go,fonts.go}` + tests | variants, WebP, OG PNG, glyph check |
| `internal/render/{render.go,page.go,jsonld.go,css.go}` + tests | templates → HTML; head metadata; inline CSS |
| `internal/feeds/{rss.go,jsonfeed.go}` + tests | feeds |
| `internal/seo/{sitemap.go,robots.go,llms.go,wellknown.go}` + tests | machine files |
| `internal/edge/headers.go` + tests | `_headers` |
| `internal/site/{build.go,check.go,serve.go,new.go,scorecard.go,clock.go}` + tests | orchestration, `dist/` invariants, local server, scaffold |
| `templates/{base,home,index,piece,colophon,privacy,404,scorecard}.html` | pages |
| `assets/css/site.css` | provisional CSS (M1b replaces values, not structure) |
| `assets/fonts/og/{Newsreader72pt-Medium,SourceSerif4-Regular}.ttf` + licences | OG rendering fonts (static instances); web fonts are M1b, in `assets/fonts/web/` |
| `content/{en,ro}/_home.md`, `content/{en,ro}/_privacy.md`, `content/en/_colophon.md`, `content/_template.md`, `content/{en,ro}/*.md` | pages and pieces |
| `i18n/{en,ro}.yaml` | UI strings |
| `static/` | favicons, `.well-known/mta-sts.txt`, copied verbatim |
| `data/scorecard-manual.yaml` | manual (ext) scorecard rows |
| `testdata/site/**`, `testdata/golden/**` | fixture site and golden output |
| `scripts/setup.sh`, `.githooks/pre-commit` | developer setup, pre-commit `gofmt` + `site check` |

---

### Task 0: Toolchain, module, CLI skeleton, pre-commit

**Files:**
- Create: `go.mod`, `cmd/site/main.go`, `internal/config/config.go`, `internal/config/config_test.go`, `site.yaml`, `scripts/setup.sh`, `.githooks/pre-commit`
- Modify: `.gitignore` (add `/dist/` already there; add `/.cache/`)

**Interfaces:**
- Produces: `config.Config{BaseURL, Name, Tagline map[string]string, Author{LinkedIn,X,GitHub,Email}, AIDisclosure map[string]string}`; `config.Load(path) (*Config, error)`; URL helpers `(*Config) PieceURL(lang, slug) string`, `IndexURL(lang)`, `HomeURL(lang)`, `PrivacyURL(lang)`, `Abs(path) string`.

- [ ] **Step 1: Install Go 1.27.1 under `~/.local` (no sudo)**

```bash
V=1.27.1; A=$([ "$(uname -m)" = aarch64 ] && echo arm64 || echo amd64)
cd /tmp && curl -sSLO "https://go.dev/dl/go${V}.linux-${A}.tar.gz" && curl -sSL "https://go.dev/dl/?mode=json&include=all" | jq -r --arg f "go${V}.linux-${A}.tar.gz" '.[] | .files[] | select(.filename==$f) | .sha256' > sum && [ "$(sha256sum go${V}.linux-${A}.tar.gz | cut -d' ' -f1)" = "$(cat sum)" ] && rm -rf ~/.local/go && tar -C ~/.local -xzf go${V}.linux-${A}.tar.gz && ln -sf ~/.local/go/bin/go ~/.local/bin/go && ln -sf ~/.local/go/bin/gofmt ~/.local/bin/gofmt && export PATH="$HOME/.local/bin:$PATH" && go version
go install honnef.co/go/tools/cmd/staticcheck@latest && ln -sf ~/go/bin/staticcheck ~/.local/bin/staticcheck
```
Expected: `go version go1.27.1 linux/arm64`. If `staticcheck` reports Go 1.27 as unsupported, install `@master`; if that fails too, drop it from this plan's gates (keep `go vet`) and say so in the PR.

- [ ] **Step 2: Module and branch**

```bash
cd "$(git rev-parse --show-toplevel)" && git checkout infra/m0 && git pull gitea infra/m0 && git checkout -b m1a/generator
go mod init github.com/raduherinean/herinean.com
```
`infra/m0`, not `main` (see Global Constraints).

- [ ] **Step 3: `site.yaml`** (values marked ⟨⟩ are author inputs; `check` fails while any remains)

```yaml
base_url: https://herinean.com
name: Radu Herinean
tagline:
  en: "⟨Practicing CTO. I ship AI in real businesses and write about which projects are worth funding.⟩"
  ro: "⟨CTO practician. Livrez AI în afaceri reale și scriu despre ce proiecte merită finanțate.⟩"
author:
  linkedin: "⟨https://www.linkedin.com/in/…⟩"
  x: "⟨https://x.com/…⟩"
  github: https://github.com/rlucian
  email: security@herinean.com
ai_disclosure:
  en: "⟨The platform was designed and built with Claude; the writing is mine, edited with AI where noted.⟩"
  ro: "⟨Platforma a fost proiectată și construită cu Claude; textele sunt ale mele, editate cu AI acolo unde e menționat.⟩"
```

- [ ] **Step 4: Write the failing config test**

`internal/config/config_test.go`:
```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

const sample = `
base_url: https://herinean.com
name: Radu Herinean
tagline: {en: "Tag EN", ro: "Tag RO"}
author: {linkedin: "https://www.linkedin.com/in/x", x: "https://x.com/x", github: "https://github.com/rlucian", email: "security@herinean.com"}
ai_disclosure: {en: "AI EN", ro: "AI RO"}
`

func write(t *testing.T, s string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "site.yaml")
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadAndURLs(t *testing.T) {
	c, err := Load(write(t, sample))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		c.HomeURL("en"):                 "/",
		c.HomeURL("ro"):                 "/ro/",
		c.IndexURL("en"):                "/writing/",
		c.IndexURL("ro"):                "/ro/articole/",
		c.PieceURL("en", "hello"):       "/writing/hello/",
		c.PieceURL("ro", "salut"):       "/ro/articole/salut/",
		c.PrivacyURL("en"):              "/privacy/",
		c.PrivacyURL("ro"):              "/ro/confidentialitate/",
		c.Abs("/writing/hello/"):        "https://herinean.com/writing/hello/",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("got %q want %q", got, want)
		}
	}
}

func TestPlaceholdersRejected(t *testing.T) {
	_, err := Load(write(t, `base_url: https://h.com
name: R
tagline: {en: "⟨fill me⟩", ro: "x"}
author: {linkedin: "a", x: "b", github: "c", email: "d"}
ai_disclosure: {en: "e", ro: "f"}`))
	if err == nil {
		t.Fatal("expected placeholder error")
	}
}

func TestMissingFieldRejected(t *testing.T) {
	_, err := Load(write(t, `base_url: https://h.com
name: R`))
	if err == nil {
		t.Fatal("expected error for missing fields")
	}
}
```

- [ ] **Step 5: Run to see it fail** — `go test ./internal/config/` → compile error: no `Load`.

- [ ] **Step 6: `internal/config/config.go`**

```go
// Package config loads site.yaml and owns the URL scheme.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Author struct {
	LinkedIn string `yaml:"linkedin"`
	X        string `yaml:"x"`
	GitHub   string `yaml:"github"`
	Email    string `yaml:"email"`
}

type Config struct {
	BaseURL      string            `yaml:"base_url"`
	Name         string            `yaml:"name"`
	Tagline      map[string]string `yaml:"tagline"`
	Author       Author            `yaml:"author"`
	AIDisclosure map[string]string `yaml:"ai_disclosure"`
}

// Placeholder markers: anything the author still has to fill in. The build refuses to ship them.
const openMark, closeMark = "⟨", "⟩"

// ErrPlaceholder marks "author inputs still missing"; site maps it to exit code 3 so the pre-commit hook warns instead of blocking before launch.
var ErrPlaceholder = errors.New("author placeholder present")

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &c, nil
}

func (c *Config) validate() error {
	req := map[string]string{
		"base_url": c.BaseURL, "name": c.Name,
		"tagline.en": c.Tagline["en"], "tagline.ro": c.Tagline["ro"],
		"author.linkedin": c.Author.LinkedIn, "author.x": c.Author.X, "author.github": c.Author.GitHub, "author.email": c.Author.Email,
		"ai_disclosure.en": c.AIDisclosure["en"], "ai_disclosure.ro": c.AIDisclosure["ro"],
	}
	for k, v := range req {
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("missing %s", k)
		}
		if strings.Contains(v, openMark) || strings.Contains(v, closeMark) {
			return fmt.Errorf("%s still contains a ⟨placeholder⟩: %w", k, ErrPlaceholder)
		}
	}
	if !strings.HasPrefix(c.BaseURL, "https://") || strings.HasSuffix(c.BaseURL, "/") {
		return fmt.Errorf("base_url must start with https:// and have no trailing slash")
	}
	return nil
}

// URL scheme (spec §4.1). English at the root, Romanian under /ro/. Every URL ends with a slash.

func (c *Config) HomeURL(lang string) string {
	if lang == "ro" {
		return "/ro/"
	}
	return "/"
}

func (c *Config) IndexURL(lang string) string {
	if lang == "ro" {
		return "/ro/articole/"
	}
	return "/writing/"
}

func (c *Config) PieceURL(lang, slug string) string { return c.IndexURL(lang) + slug + "/" }

func (c *Config) PrivacyURL(lang string) string {
	if lang == "ro" {
		return "/ro/confidentialitate/"
	}
	return "/privacy/"
}

func (c *Config) ColophonURL() string { return "/colophon/" }

func (c *Config) FeedURL(lang string) string { // "" = all languages
	if lang == "" {
		return "/feed.xml"
	}
	return "/feed." + lang + ".xml"
}

func (c *Config) Abs(path string) string { return c.BaseURL + path }
```

- [ ] **Step 7: `cmd/site/main.go`** — verbs dispatch to functions the later tasks implement; until then they return "not implemented" so the binary builds.

```go
// Command site builds herinean.com: content/ + assets/ + templates/ + i18n/ → dist/.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	_ "time/tzdata" // Europe/Bucharest travels with the binary

	"github.com/raduherinean/herinean.com/internal/site"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "build":
		err = site.Build(site.Options{Root: ".", Out: "dist"})
	case "check":
		fs := flag.NewFlagSet("check", flag.ExitOnError)
		dist := fs.Bool("dist", false, "check the built dist/ instead of the sources")
		_ = fs.Parse(os.Args[2:])
		if *dist {
			err = site.CheckDist(site.Options{Root: ".", Out: "dist"})
		} else {
			err = site.Check(site.Options{Root: "."})
		}
	case "serve":
		fs := flag.NewFlagSet("serve", flag.ExitOnError)
		host := fs.String("host", "127.0.0.1", "bind address")
		port := fs.Int("port", 8080, "port")
		_ = fs.Parse(os.Args[2:])
		err = site.Serve(site.Options{Root: ".", Out: "dist"}, *host, *port)
	case "new":
		if len(os.Args) != 4 {
			fmt.Fprintln(os.Stderr, "usage: site new <en|ro> <slug>")
			os.Exit(2)
		}
		err = site.New(site.Options{Root: "."}, os.Args[2], os.Args[3])
	case "scorecard":
		fs := flag.NewFlagSet("scorecard", flag.ExitOnError)
		in := fs.String("in", "scorecard.json", "CI results")
		manual := fs.String("manual", "data/scorecard-manual.yaml", "manual rows")
		out := fs.String("out", "scorecard.html", "HTML fragment")
		_ = fs.Parse(os.Args[2:])
		err = site.Scorecard(site.Options{Root: "."}, *in, *manual, *out)
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "site:", err)
		if errors.Is(err, site.ErrAuthorInputs) {
			os.Exit(3) // author inputs missing: the pre-commit hook warns, CI fails
		}
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  site build              content/ + assets/ + templates/ + i18n/ → dist/
  site check [--dist]     validate sources (or the built dist/)
  site serve [--host H] [--port P]
  site new <en|ro> <slug>
  site scorecard --in scorecard.json --manual data/scorecard-manual.yaml --out scorecard.html`)
}
```

`internal/site/site.go` (stub, replaced task by task):
```go
// Package site orchestrates the build.
package site

import "errors"

type Options struct {
	Root string // repository root
	Out  string // output directory (dist)
}

var errNotImplemented = errors.New("not implemented yet")

// ErrAuthorInputs wraps every "⟨placeholder⟩ still present" failure, from site.yaml or content/. Exit code 3.
var ErrAuthorInputs = errors.New("author inputs missing")

func Build(o Options) error                                   { return errNotImplemented }
func Check(o Options) error                                   { return errNotImplemented }
func CheckDist(o Options) error                               { return errNotImplemented }
func Serve(o Options, host string, port int) error            { return errNotImplemented }
func New(o Options, lang, slug string) error                  { return errNotImplemented }
func Scorecard(o Options, in, manual, out string) error       { return errNotImplemented }
```

- [ ] **Step 8: Dependencies, build, test**

```bash
go get gopkg.in/yaml.v3@latest && go mod tidy && go build ./... && go test ./... && go vet ./... && staticcheck ./...
```
Expected: config tests PASS; the placeholder test passes because `site.yaml`'s ⟨⟩ values are only checked when `site.yaml` itself is loaded (Task 12).

- [ ] **Step 9: Pre-commit hook and setup script**

`.githooks/pre-commit`:
```bash
#!/usr/bin/env bash
# gofmt + vet + site check before every commit. Install with scripts/setup.sh.
set -euo pipefail
export PATH="$HOME/.local/bin:$PATH"
if git diff --cached --name-only | grep -q '\.go$'; then
  unf=$(gofmt -l $(git diff --cached --name-only --diff-filter=ACM | grep '\.go$') || true)
  [ -z "$unf" ] || { echo "gofmt needed: $unf" >&2; exit 1; }
  go vet ./...
fi
if git diff --cached --name-only | grep -qE '^(content|i18n|assets|templates|site\.yaml)'; then
  set +e; go run ./cmd/site check; rc=$?; set -e
  if [ "$rc" -eq 3 ]; then
    echo "pre-commit: author inputs still missing (site check exit 3) — allowed before launch; CI will not allow it" >&2
  elif [ "$rc" -ne 0 ]; then
    exit "$rc"
  fi
fi
```

`scripts/setup.sh`:
```bash
#!/usr/bin/env bash
# One-time developer setup: hooks path and tool check.
set -euo pipefail
cd "$(dirname "$0")/.."
git config core.hooksPath .githooks
chmod +x .githooks/*
export PATH="$HOME/.local/bin:$PATH"
for t in go tofu jq dig curl; do command -v "$t" >/dev/null || echo "missing: $t (see RUNBOOK.md)"; done
go version
echo "hooks installed; run 'go run ./cmd/site serve' to preview"
```

- [ ] **Step 10: Commit**

```bash
chmod +x scripts/setup.sh .githooks/pre-commit && scripts/setup.sh
git add go.mod go.sum cmd internal site.yaml scripts/setup.sh .githooks
git commit -m "$(cat <<'MSG'
M1a: Go module, CLI skeleton, config loader with URL scheme, pre-commit hook

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
)"
git push -u gitea m1a/generator
```
(The hook's `site check` runs only when content/templates change, so this commit passes with the stub.)

---

### Task 1: `content` — front matter, slugs, dates, validation rules

**Files:**
- Create: `internal/content/problems.go`, `internal/content/piece.go`, `internal/content/frontmatter.go`, `internal/content/slug.go`, `internal/content/frontmatter_test.go`, `internal/content/slug_test.go`

**Interfaces:**
- Produces: `content.Problem{File, Line, Msg}` (+ `Problems.Err()`); `content.Piece` (fields below); `content.Pillars`; `content.Slugify(string) string`; `content.ValidSlug(string) bool`; `content.ParseFrontMatter(file string, src []byte, lang string, now time.Time) (*Piece, []byte /*body*/, int /*bodyLine*/, Problems)`; `content.Bucharest *time.Location`; `content.CheckDiacritics(file string, body []byte, startLine int) Problems`.

- [ ] **Step 1: Write the failing tests**

`internal/content/slug_test.go`:
```go
package content

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Which AI projects are worth funding in 2027": "which-ai-projects-are-worth-funding-in-2027",
		"Ce ar trebui să întrebe un board despre AI":   "ce-ar-trebui-sa-intrebe-un-board-despre-ai",
		"Ș ț Ă Â Î — și „ghilimele”":                   "s-t-a-a-i-si-ghilimele",
		"  double  spaces -- and_underscores ":         "double-spaces-and-underscores",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
	for _, bad := range []string{"", "-a", "a-", "a--b", "Abc", "ș", "a b"} {
		if ValidSlug(bad) {
			t.Errorf("ValidSlug(%q) should be false", bad)
		}
	}
	if !ValidSlug("ai-funding-2027") {
		t.Error("ValidSlug(ai-funding-2027) should be true")
	}
}
```

`internal/content/frontmatter_test.go`:
```go
package content

import (
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 9, 27, 12, 0, 0, 0, Bucharest)

const good = `---
title: "Which AI projects are worth funding in 2027"
date: 2026-09-27
key: ai-funding-2027
pillar: analysis
summary: "A filter for boards."
linkedin: https://www.linkedin.com/posts/x
---
Body here.
`

func TestParseGood(t *testing.T) {
	p, body, line, probs := ParseFrontMatter("content/en/which-ai.md", []byte(good), "en", now)
	if err := probs.Err(); err != nil {
		t.Fatal(err)
	}
	if p.Slug != "which-ai" || p.Key != "ai-funding-2027" || p.Pillar != "analysis" || p.Lang != "en" {
		t.Errorf("fields: %+v", p)
	}
	if p.Date.Year() != 2026 || p.Date.Location() != Bucharest {
		t.Errorf("date %v", p.Date)
	}
	if strings.TrimSpace(string(body)) != "Body here." || line != 9 {
		t.Errorf("body %q line %d", body, line)
	}
}

func TestParseProblems(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"no front matter", "Body", "front matter"},
		{"empty date", strings.Replace(good, "date: 2026-09-27", "date:", 1), "date"},
		{"future date", strings.Replace(good, "2026-09-27", "2026-09-28", 1), "after"},
		{"bad pillar", strings.Replace(good, "analysis", "rants", 1), "pillar"},
		{"long summary", strings.Replace(good, "A filter for boards.", strings.Repeat("x", 161), 1), "160"},
		{"missing key", strings.Replace(good, "key: ai-funding-2027\n", "", 1), "key"},
		{"draft field", strings.Replace(good, "pillar:", "draft: true\npillar:", 1), "draft"},
		{"cedilla in title", strings.Replace(good, "Which", "Reţea", 1), "comma-below"},
	}
	for _, c := range cases {
		_, _, _, probs := ParseFrontMatter("content/en/x.md", []byte(c.src), "en", now)
		if err := probs.Err(); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want error containing %q, got %v", c.name, c.want, err)
		}
	}
}

func TestBadSlugFromFilename(t *testing.T) {
	_, _, _, probs := ParseFrontMatter("content/en/Bad Slug.md", []byte(good), "en", now)
	if err := probs.Err(); err == nil || !strings.Contains(err.Error(), "slug") {
		t.Errorf("want slug error, got %v", err)
	}
}

func TestCheckDiacritics(t *testing.T) {
	probs := CheckDiacritics("f.md", []byte("ok ș\nbad ţ here\n"), 10)
	if len(probs) != 1 || probs[0].Line != 11 {
		t.Fatalf("got %+v", probs)
	}
}
```

- [ ] **Step 2: Run to see them fail** — `go test ./internal/content/` → undefined symbols.

- [ ] **Step 3: `problems.go`**

```go
package content

import (
	"fmt"
	"strings"
)

// Problem is one validation failure, addressed so an editor can jump to it.
type Problem struct {
	File string
	Line int
	Msg  string
}

func (p Problem) Error() string {
	if p.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", p.File, p.Line, p.Msg)
	}
	return fmt.Sprintf("%s: %s", p.File, p.Msg)
}

type Problems []Problem

func (ps Problems) Err() error {
	if len(ps) == 0 {
		return nil
	}
	var b strings.Builder
	for i, p := range ps {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(p.Error())
	}
	return fmt.Errorf("%d problem(s):\n%s", len(ps), b.String())
}

func (ps *Problems) Add(file string, line int, format string, args ...any) {
	*ps = append(*ps, Problem{File: file, Line: line, Msg: fmt.Sprintf(format, args...)})
}
```

- [ ] **Step 4: `piece.go`**

```go
package content

import (
	"html/template"
	"time"

	"github.com/yuin/goldmark/ast"
)

// Bucharest is the editorial time zone: dates in front matter are local dates there.
var Bucharest = mustLoad("Europe/Bucharest")

func mustLoad(name string) *time.Location {
	l, err := time.LoadLocation(name)
	if err != nil {
		panic(err) // cmd/site imports time/tzdata, so this cannot fail in the binary
	}
	return l
}

// Pillars, in the order they are listed on the site.
var Pillars = []string{"build-log", "opportunity", "governance", "analysis"}

// Piece is one published article in one language.
type Piece struct {
	Lang     string
	Slug     string
	Key      string
	Title    string
	Date     time.Time
	Updated  *time.Time
	Pillar   string
	Summary  string
	LinkedIn string
	Medium   string

	Body           template.HTML // rendered by Render
	Words          int
	ReadingMinutes int
	Images         []ImageRef // referenced by the body, relative names
	Links          []LinkRef  // internal links found in the body

	File        string
	Translation *Piece // the same key in the other language, if any

	doc ast.Node
	src []byte
	bodyLine int
}

type ImageRef struct {
	Dest string // as written: "diagram.svg", "photo.jpg"
	Alt  string
	Line int
}

type LinkRef struct {
	Dest string // site-absolute path, e.g. /writing/x/
	Line int
}

// ImageInfo is what the renderer needs to emit a responsive <img>/<picture>. Filled by the images package via site.
type ImageInfo struct {
	Src      string // largest fallback URL (original format)
	Srcset   string // "url 720w, url 1440w" in WebP
	Width    int
	Height   int
	IsSVG    bool
}

func (p *Piece) URLPath(indexURL string) string { return indexURL + p.Slug + "/" }
```

- [ ] **Step 5: `slug.go`**

```go
package content

import (
	"regexp"
	"strings"
	"unicode"
)

var translit = map[rune]string{
	'ă': "a", 'â': "a", 'î': "i", 'ș': "s", 'ț': "t", 'ş': "s", 'ţ': "t",
	'ä': "a", 'ö': "o", 'ü': "u", 'ß': "ss", 'é': "e", 'è': "e", 'ê': "e", 'ë': "e",
	'á': "a", 'à': "a", 'ó': "o", 'ò': "o", 'ú': "u", 'ù': "u", 'í': "i", 'ì': "i", 'ç': "c", 'ñ': "n",
}

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Slugify makes an ASCII slug from a title. Romanian diacritics transliterate; everything else non-alphanumeric becomes a hyphen.
func Slugify(s string) string {
	var b strings.Builder
	lastDash := true
	for _, r := range strings.ToLower(s) {
		if t, ok := translit[r]; ok {
			b.WriteString(t)
			lastDash = false
			continue
		}
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue // other scripts: dropped rather than guessed
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func ValidSlug(s string) bool { return slugRe.MatchString(s) }
```

- [ ] **Step 6: `frontmatter.go`**

```go
package content

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

type frontMatter struct {
	Title    string `yaml:"title"`
	Date     string `yaml:"date"`
	Updated  string `yaml:"updated"`
	Key      string `yaml:"key"`
	Pillar   string `yaml:"pillar"`
	Summary  string `yaml:"summary"`
	LinkedIn string `yaml:"linkedin"`
	Medium   string `yaml:"medium"`
	Draft    *bool  `yaml:"draft"` // forbidden: drafts are unpublished branches
}

const summaryMax = 160

var cedilla = regexp.MustCompile("[ŞşŢţ]")

// ParseFrontMatter splits a piece file, validates the front matter and returns the Piece (without Body), the Markdown body and its first line number.
func ParseFrontMatter(file string, src []byte, lang string, now time.Time) (*Piece, []byte, int, Problems) {
	var probs Problems
	if !bytes.HasPrefix(src, []byte("---\n")) {
		probs.Add(file, 1, "missing front matter (file must start with ---)")
		return nil, nil, 0, probs
	}
	end := bytes.Index(src[4:], []byte("\n---\n"))
	if end < 0 {
		probs.Add(file, 1, "unterminated front matter")
		return nil, nil, 0, probs
	}
	fmBytes := src[4 : 4+end]
	body := src[4+end+5:]
	bodyLine := bytes.Count(src[:4+end+5], []byte("\n")) + 1

	var fm frontMatter
	if err := yaml.Unmarshal(fmBytes, &fm); err != nil {
		probs.Add(file, 1, "front matter: %v", err)
		return nil, nil, 0, probs
	}
	p := &Piece{Lang: lang, File: file, Title: strings.TrimSpace(fm.Title), Key: strings.TrimSpace(fm.Key),
		Pillar: strings.TrimSpace(fm.Pillar), Summary: strings.TrimSpace(fm.Summary),
		LinkedIn: strings.TrimSpace(fm.LinkedIn), Medium: strings.TrimSpace(fm.Medium), bodyLine: bodyLine}
	p.Slug = strings.TrimSuffix(filepath.Base(file), ".md")

	if fm.Draft != nil {
		probs.Add(file, 1, "draft: is not a field; drafts are unpublished branches on the private remote")
	}
	if p.Title == "" {
		probs.Add(file, 1, "title is required")
	}
	if cedilla.MatchString(p.Title) || cedilla.MatchString(p.Summary) {
		probs.Add(file, 1, "cedilla ş/ţ found; Romanian uses comma-below ș/ț")
	}
	if !ValidSlug(p.Slug) {
		probs.Add(file, 1, "slug %q (from the file name) must match [a-z0-9]+(-[a-z0-9]+)*", p.Slug)
	}
	if p.Key == "" {
		probs.Add(file, 1, "key is required (pairs the EN and RO versions)")
	} else if !ValidSlug(p.Key) {
		probs.Add(file, 1, "key %q must look like a slug", p.Key)
	}
	if !contains(Pillars, p.Pillar) {
		probs.Add(file, 1, "pillar %q must be one of %s", p.Pillar, strings.Join(Pillars, ", "))
	}
	if p.Summary == "" {
		probs.Add(file, 1, "summary is required")
	} else if n := utf8.RuneCountInString(p.Summary); n > summaryMax {
		probs.Add(file, 1, "summary is %d characters; max %d", n, summaryMax)
	}
	if strings.TrimSpace(fm.Date) == "" {
		probs.Add(file, 1, "date is empty; /publish-piece stamps it")
	} else if d, err := parseDate(fm.Date); err != nil {
		probs.Add(file, 1, "date: %v", err)
	} else {
		p.Date = d
		if d.After(now) {
			probs.Add(file, 1, "date %s is after the build/commit date %s", d.Format("2006-01-02"), now.In(Bucharest).Format("2006-01-02"))
		}
	}
	if strings.TrimSpace(fm.Updated) != "" {
		u, err := parseDate(fm.Updated)
		switch {
		case err != nil:
			probs.Add(file, 1, "updated: %v", err)
		case !p.Date.IsZero() && u.Before(p.Date):
			probs.Add(file, 1, "updated is before date")
		default:
			p.Updated = &u
		}
	}
	for _, u := range []string{p.LinkedIn, p.Medium} {
		if u != "" && !strings.HasPrefix(u, "https://") {
			probs.Add(file, 1, "%q must be an https URL", u)
		}
	}
	return p, body, bodyLine, probs
}

func parseDate(s string) (time.Time, error) {
	d, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(s), Bucharest)
	if err != nil {
		return time.Time{}, fmt.Errorf("want YYYY-MM-DD, got %q", s)
	}
	return d, nil
}

// CheckDiacritics reports cedilla forms in a body, one problem per offending line.
func CheckDiacritics(file string, body []byte, startLine int) Problems {
	var probs Problems
	for i, line := range bytes.Split(body, []byte("\n")) {
		if cedilla.Match(line) {
			probs.Add(file, startLine+i, "cedilla ş/ţ found; Romanian uses comma-below ș/ț")
		}
	}
	return probs
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}
```

- [ ] **Step 7: Make it compile and pass**

```bash
go get github.com/yuin/goldmark@latest && go mod tidy && go test ./internal/content/ -run 'Slug|Parse|Diacritics' -v 2>&1 | tail -15
```
Expected: all PASS. (`cmd/site/main.go` already imports `_ "time/tzdata"` from Task 0, so the binary carries the Europe/Bucharest zone.)

- [ ] **Step 8: Commit** — `M1a: content — front matter, slugs, dates, diacritics rules` with trailers; push to gitea.

---

### Task 2: `content` — Markdown rendering

**Files:**
- Create: `internal/content/markdown.go`, `internal/content/figure.go`, `internal/content/markdown_test.go`

**Interfaces:**
- Consumes: `Piece`, `ImageInfo`, `ImageRef`, `LinkRef`.
- Produces: `content.ParseBody(p *Piece, body []byte) Problems` (parses into `p.doc`, fills `p.Images`, `p.Links`, `p.Words`, `p.ReadingMinutes`; alt, code-language, indented-code and diacritics problems); `content.RenderBody(p *Piece, lookup func(dest string) *ImageInfo, tableLabel string) (template.HTML, error)` (`tableLabel` names the scrollable table region; from i18n `table.label`). Highlighting CSS is hand-written in `site.css` (Task 6) from the palette tokens — nothing generated.

- [ ] **Step 1: Write the failing tests**

`internal/content/markdown_test.go`:
```go
package content

import (
	"strings"
	"testing"
)

func piece(lang, body string) *Piece {
	return &Piece{Lang: lang, File: "content/" + lang + "/t.md", bodyLine: 5, Key: "k"}
}

func TestQuotesPerLanguage(t *testing.T) {
	en, ro := piece("en", ""), piece("ro", "")
	if probs := ParseBody(en, []byte(`He said "hello" -- really...`)); probs.Err() != nil {
		t.Fatal(probs.Err())
	}
	if probs := ParseBody(ro, []byte(`A spus "salut" -- chiar...`)); probs.Err() != nil {
		t.Fatal(probs.Err())
	}
	enHTML, _ := RenderBody(en, nil, "Table")
	roHTML, _ := RenderBody(ro, nil, "Tabel")
	if !strings.Contains(string(enHTML), "&ldquo;hello&rdquo;") {
		t.Errorf("en quotes: %s", enHTML)
	}
	if !strings.Contains(string(roHTML), "„salut”") {
		t.Errorf("ro quotes: %s", roHTML)
	}
	if !strings.Contains(string(enHTML), "&ndash;") && !strings.Contains(string(enHTML), "–") {
		t.Errorf("dash: %s", enHTML)
	}
}

func TestImageAltRequired(t *testing.T) {
	p := piece("en", "")
	probs := ParseBody(p, []byte("![](photo.jpg)\n\n![ok](fig.png \"A caption\")\n"))
	if len(probs) != 1 || !strings.Contains(probs[0].Msg, "alt") || probs[0].Line != 5 {
		t.Fatalf("want one alt problem at line 5, got %+v", probs)
	}
	if len(p.Images) != 2 || p.Images[1].Dest != "fig.png" || p.Images[1].Alt != "ok" {
		t.Errorf("images: %+v", p.Images)
	}
}

func TestFigureAndResponsiveImage(t *testing.T) {
	p := piece("en", "")
	_ = ParseBody(p, []byte("Intro.\n\n![Alt text](fig.png \"A caption\")\n\nInline ![i](x.svg) here.\n"))
	lookup := func(dest string) *ImageInfo {
		switch dest {
		case "fig.png":
			return &ImageInfo{Src: "/img/k/fig.abc12345.1440.png", Srcset: "/img/k/fig.abc12345.720.webp 720w, /img/k/fig.abc12345.1440.webp 1440w", Width: 1440, Height: 900}
		case "x.svg":
			return &ImageInfo{Src: "/img/k/x.abc12345.svg", Width: 24, Height: 24, IsSVG: true}
		}
		return nil
	}
	html, err := RenderBody(p, lookup, "Table")
	if err != nil {
		t.Fatal(err)
	}
	s := string(html)
	for _, want := range []string{
		`<figure><picture><source type="image/webp" srcset="/img/k/fig.abc12345.720.webp 720w, /img/k/fig.abc12345.1440.webp 1440w" sizes="(max-width: 40rem) 100vw, 38rem"><img src="/img/k/fig.abc12345.1440.png" alt="Alt text" width="1440" height="900" loading="lazy" decoding="async"></picture><figcaption>A caption</figcaption></figure>`,
		`<img src="/img/k/x.abc12345.svg" alt="i" width="24" height="24" loading="lazy" decoding="async">`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s\nin %s", want, s)
		}
	}
	if strings.Contains(s, "<p><figure") {
		t.Error("figure must not be wrapped in <p>")
	}
}

func TestCodeUsesClassesNotStyles(t *testing.T) {
	p := piece("en", "")
	_ = ParseBody(p, []byte("```go\nfmt.Println(\"x\")\n```\n"))
	html, _ := RenderBody(p, nil, "Table")
	if strings.Contains(string(html), "style=") || !strings.Contains(string(html), `class="chroma"`) || !strings.Contains(string(html), `<pre tabindex="0"`) {
		t.Errorf("highlighting must use classes and a focusable pre: %s", html)
	}
}

func TestCodeNeedsLanguage(t *testing.T) {
	p := piece("en", "")
	probs := ParseBody(p, []byte("```\nplain\n```\n\n    indented\n"))
	if len(probs) != 2 || !strings.Contains(probs[0].Msg, "language") || !strings.Contains(probs[1].Msg, "indented") {
		t.Errorf("want a language problem and an indented-block problem, got %+v", probs)
	}
}

func TestTablesWrapAndNeverUseStyle(t *testing.T) {
	p := piece("en", "")
	_ = ParseBody(p, []byte("| a | b |\n|---:|:---|\n| 1 | 2 |\n"))
	html, _ := RenderBody(p, nil, "Table")
	s := string(html)
	if !strings.Contains(s, `<div class="table" role="region" aria-label="Table" tabindex="0"><table>`) || !strings.Contains(s, "</table>\n</div>") || strings.Contains(s, "style=") || strings.Contains(s, "align=") {
		t.Errorf("tables must be wrapped, focusable and free of style/align attributes: %s", s)
	}
}

func TestReadingTimeAndLinks(t *testing.T) {
	p := piece("en", "")
	body := strings.Repeat("word ", 450) + "\n\nSee [this](/writing/other/) and [that](https://example.com).\n"
	_ = ParseBody(p, []byte(body))
	if p.Words < 450 || p.ReadingMinutes != 3 {
		t.Errorf("words %d minutes %d", p.Words, p.ReadingMinutes)
	}
	if len(p.Links) != 1 || p.Links[0].Dest != "/writing/other/" {
		t.Errorf("links: %+v", p.Links)
	}
}

func TestFootnotesAndTables(t *testing.T) {
	p := piece("en", "")
	_ = ParseBody(p, []byte("A claim[^1].\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n[^1]: Source.\n"))
	html, _ := RenderBody(p, nil, "Table")
	s := string(html)
	if !strings.Contains(s, "<table>") || !strings.Contains(s, `class="footnotes"`) {
		t.Errorf("%s", s)
	}
}
```

- [ ] **Step 2: Run to see them fail** — undefined `ParseBody`, `RenderBody`.

- [ ] **Step 3: `figure.go`** — an AST transformer that lifts a paragraph consisting of a single image into a block-level figure, so `<figure>` never ends up inside `<p>` (Nu rejects that).

```go
package content

import (
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// Figure is a block node: an image that stood alone in its paragraph, with an optional caption (the image title).
type Figure struct {
	ast.BaseBlock
	Image *ast.Image
}

var KindFigure = ast.NewNodeKind("Figure")

func (n *Figure) Kind() ast.NodeKind { return KindFigure }
func (n *Figure) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

type figureTransformer struct{}

func (figureTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	var targets []*ast.Paragraph
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if p, ok := n.(*ast.Paragraph); ok && p.ChildCount() == 1 {
			if _, isImg := p.FirstChild().(*ast.Image); isImg {
				targets = append(targets, p)
			}
		}
		return ast.WalkContinue, nil
	})
	for _, p := range targets {
		img := p.FirstChild().(*ast.Image)
		fig := &Figure{Image: img}
		p.RemoveChild(p, img)
		p.Parent().ReplaceChild(p.Parent(), p, fig)
	}
}

// TableWrap is a block around a table: the div scrolls horizontally on narrow screens and carries tabindex so keyboard users can scroll it (axe: scrollable-region-focusable).
type TableWrap struct{ ast.BaseBlock }

var KindTableWrap = ast.NewNodeKind("TableWrap")

func (n *TableWrap) Kind() ast.NodeKind { return KindTableWrap }
func (n *TableWrap) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

type tableTransformer struct{}

func (tableTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	var tables []*extast.Table
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if t, ok := n.(*extast.Table); ok && entering {
			tables = append(tables, t)
		}
		return ast.WalkContinue, nil
	})
	for _, t := range tables {
		wrap := &TableWrap{}
		parent := t.Parent()
		parent.ReplaceChild(parent, t, wrap)
		wrap.AppendChild(wrap, t)
	}
}
```

- [ ] **Step 4: `markdown.go`**

```go
package content

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"math"
	"strings"
	"unicode"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	ghtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
)

const wordsPerMinute = 200

func typographer(lang string) goldmark.Extender {
	if lang == "ro" {
		return extension.NewTypographer(extension.WithTypographicSubstitutions(extension.TypographicSubstitutions{
			extension.LeftDoubleQuote:  []byte("„"),
			extension.RightDoubleQuote: []byte("”"),
			extension.LeftSingleQuote:  []byte("‚"),
			extension.RightSingleQuote: []byte("’"),
		}))
	}
	return extension.Typographer
}

func newMarkdown(lang string, lookup func(string) *ImageInfo, tableLabel string) goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			// Column alignment would need style= (blocked by the hash CSP) or the obsolete align= (Nu error); neither is emitted.
			extension.NewTable(extension.WithTableCellAlignMethod(extension.TableCellAlignNone)),
			extension.Footnote,
			typographer(lang),
			highlighting.NewHighlighting(
				highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(util.Prioritized(figureTransformer{}, 100), util.Prioritized(tableTransformer{}, 200)),
		),
		goldmark.WithRendererOptions(
			ghtml.WithUnsafe(), // inline SVG diagrams and the occasional raw block; check --dist polices scripts and style=
			renderer.WithNodeRenderers(util.Prioritized(&nodeRenderer{lookup: lookup, tableLabel: tableLabel}, 100)),
		),
	)
}

// ParseBody parses Markdown into p.doc and collects images, links and word count. Alt text is mandatory.
func ParseBody(p *Piece, body []byte) Problems {
	var probs Problems
	p.src = body
	md := newMarkdown(p.Lang, nil, "")
	p.doc = md.Parser().Parse(text.NewReader(body))
	p.Images, p.Links = nil, nil
	words := 0
	_ = ast.Walk(p.doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := n.(type) {
		case *ast.Image:
			alt := string(v.Text(body))
			line := lineOf(body, v, p.bodyLine)
			if strings.TrimSpace(alt) == "" {
				probs.Add(p.File, line, "image %q has no alt text", string(v.Destination))
			}
			p.Images = append(p.Images, ImageRef{Dest: string(v.Destination), Alt: alt, Line: line})
		case *ast.Link:
			if d := string(v.Destination); strings.HasPrefix(d, "/") {
				p.Links = append(p.Links, LinkRef{Dest: strings.SplitN(d, "#", 2)[0], Line: lineOf(body, v, p.bodyLine)})
			}
		case *ast.FencedCodeBlock:
			// chroma adds tabindex="0" to every <pre> it renders; a block without a language would be a plain, unfocusable <pre>.
			if v.Language(body) == nil {
				probs.Add(p.File, lineOf(body, v, p.bodyLine)-1, "fenced code needs a language (```text for plain text)")
			}
		case *ast.CodeBlock:
			probs.Add(p.File, lineOf(body, v, p.bodyLine), "indented code block; use a fenced block with a language")
		case *ast.Text:
			words += countWords(v.Segment.Value(body))
		case *ast.String:
			words += countWords(v.Value)
		}
		return ast.WalkContinue, nil
	})
	p.Words = words
	p.ReadingMinutes = int(math.Max(1, math.Ceil(float64(words)/wordsPerMinute)))
	probs = append(probs, CheckDiacritics(p.File, body, p.bodyLine)...)
	return probs
}

// RenderBody renders the parsed document with image information injected; tableLabel names scrollable table regions.
func RenderBody(p *Piece, lookup func(string) *ImageInfo, tableLabel string) (template.HTML, error) {
	if p.doc == nil {
		return "", fmt.Errorf("%s: ParseBody must run before RenderBody", p.File)
	}
	if lookup == nil {
		lookup = func(string) *ImageInfo { return nil }
	}
	var buf bytes.Buffer
	md := newMarkdown(p.Lang, lookup, tableLabel)
	if err := md.Renderer().Render(&buf, p.src, p.doc); err != nil {
		return "", fmt.Errorf("%s: render: %w", p.File, err)
	}
	return template.HTML(buf.String()), nil
}

type nodeRenderer struct {
	lookup     func(string) *ImageInfo
	tableLabel string
}

func (r *nodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindImage, r.renderImage)
	reg.Register(KindFigure, r.renderFigure)
	reg.Register(KindTableWrap, r.renderTableWrap)
}

func (r *nodeRenderer) renderTableWrap(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString(`<div class="table" role="region" aria-label="` + html.EscapeString(r.tableLabel) + `" tabindex="0">`)
	} else {
		_, _ = w.WriteString("</div>\n")
	}
	return ast.WalkContinue, nil
}

func (r *nodeRenderer) renderFigure(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	fig := n.(*Figure)
	if !entering {
		return ast.WalkContinue, nil
	}
	_, _ = w.WriteString("<figure>")
	r.writeImg(w, source, fig.Image)
	if t := string(fig.Image.Title); t != "" {
		_, _ = w.WriteString("<figcaption>" + html.EscapeString(t) + "</figcaption>")
	}
	_, _ = w.WriteString("</figure>\n")
	return ast.WalkSkipChildren, nil
}

func (r *nodeRenderer) renderImage(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	r.writeImg(w, source, n.(*ast.Image))
	return ast.WalkSkipChildren, nil
}

func (r *nodeRenderer) writeImg(w util.BufWriter, source []byte, img *ast.Image) {
	dest := string(img.Destination)
	alt := html.EscapeString(string(img.Text(source)))
	info := r.lookup(dest)
	if info == nil { // unknown image: emit as written so check --dist / link checks catch it
		_, _ = w.WriteString(`<img src="` + html.EscapeString(dest) + `" alt="` + alt + `">`)
		return
	}
	imgTag := fmt.Sprintf(`<img src="%s" alt="%s" width="%d" height="%d" loading="lazy" decoding="async">`, html.EscapeString(info.Src), alt, info.Width, info.Height)
	if info.IsSVG || info.Srcset == "" {
		_, _ = w.WriteString(imgTag)
		return
	}
	_, _ = w.WriteString(`<picture><source type="image/webp" srcset="` + html.EscapeString(info.Srcset) + `" sizes="(max-width: 40rem) 100vw, 38rem">` + imgTag + `</picture>`)
}

func lineOf(src []byte, n ast.Node, base int) int {
	// Inline nodes carry no position; use the enclosing block's first segment.
	for b := n; b != nil; b = b.Parent() {
		if b.Type() == ast.TypeBlock && b.Lines() != nil && b.Lines().Len() > 0 {
			return base + bytes.Count(src[:b.Lines().At(0).Start], []byte("\n"))
		}
	}
	return base
}

func countWords(b []byte) int {
	return len(bytes.FieldsFunc(b, func(r rune) bool { return unicode.IsSpace(r) }))
}
```

- [ ] **Step 5: Dependencies, run, fix, pass**

```bash
go get github.com/yuin/goldmark-highlighting/v2@latest github.com/alecthomas/chroma/v2@latest && go mod tidy && go test ./internal/content/ 2>&1 | tail -20
```
Expected: PASS. Known adjustment points if the goldmark version differs: the `Title` field on `ast.Image` (it is `Title []byte`), `Text(source)` deprecation (use `v.Text(body)` or walk child text nodes), `extension.Typographer`'s substitution map type name, and whether goldmark-highlighting routes language-less blocks through chroma (irrelevant once `check` requires a language, but the `<pre tabindex` assertion in `check --dist` is the backstop).

- [ ] **Step 6: Commit** — `M1a: content — Markdown rendering with per-language typography, figures, responsive images, class-based highlighting` with trailers; push.

---

### Task 3: `content` — i18n, loading, pairing

**Files:**
- Create: `internal/content/i18n.go`, `internal/content/load.go`, `internal/content/load_test.go`, `testdata/site/**` (fixture)

**Interfaces:**
- Produces: `content.Site{Pieces []*Piece (all languages, newest first), ByLang map[string][]*Piece, Pages map[string]*Page (keys "home.en", "home.ro", "privacy.en", "privacy.ro", "colophon.en"), Strings map[string]map[string]string}`; `content.Page{Lang, Kind, Title, Summary, Body template.HTML, File string}` + `(*Page).Links() []LinkRef` (body parsed/rendered like a piece; front matter `title` + `summary`); `content.Load(root string, now time.Time) (*Site, Problems)` (parse + validate, bodies unrendered); `(*Site).Render(lookup func(key, dest string) *ImageInfo) error`; `(*Site).Latest(n int) []*Piece`; `(*Site).T(lang, key string) string`.

- [ ] **Step 1: Fixture site** — `testdata/site/` mirrors the real layout with: `i18n/en.yaml`, `i18n/ro.yaml` (full key sets, below), `site.yaml` (valid, no placeholders), `content/en/_home.md`, `content/ro/_home.md`, `content/en/_privacy.md`, `content/ro/_privacy.md`, `content/en/_colophon.md`, pieces, dated so all precede the build epoch the `site` tests pin (2026-09-21): `content/en/paired.md` (2026-09-05) + `content/ro/pereche.md` (2026-09-06, same `key: pair`), `content/ro/doar-ro.md` (2026-09-10, RO-only), `content/en/with-media.md` (2026-09-15; image `fig.png` + `diagram.svg` + a ```go block + footnote + a table with one right-aligned column), and `assets/img/media/fig.png` (any 1600×1000 PNG), `assets/img/media/diagram.svg` (a 24×24 SVG using `currentColor`), `static/favicon.svg`, `static/.well-known/mta-sts.txt`, `templates/` and `assets/css/site.css` are copied from the repo by the test helper (they are not part of the fixture).

`i18n/en.yaml` (the complete key set; `ro.yaml` has the same keys, Romanian values):
```yaml
nav.writing: Writing
nav.language: Română
nav.language_lang: ro
nav.skip: Skip to content
home.latest: Latest
home.all: All writing →
index.title: Writing
piece.updated: Updated
piece.min_read: min read
piece.read_in_other: Citește în română →
piece.also_on: Also on
piece.discuss: LinkedIn
piece.medium: Medium
piece.byline_role: CTO
table.label: Table
pillar.build-log: Build log
pillar.opportunity: Opportunity
pillar.governance: AI + governance
pillar.analysis: Analysis
lang.en: English
lang.ro: Romanian
lang.badge.ro: RO
lang.badge.en: EN
footer.linkedin: LinkedIn
footer.rss: RSS
footer.colophon: Colophon
footer.privacy: Privacy
footer.promise: No JavaScript, no cookies, no trackers.
notfound.title: Page not found
notfound.body: Page not found.
notfound.home: Home
date.months: January,February,March,April,May,June,July,August,September,October,November,December
```
`ro.yaml`: `nav.writing: Articole`, `nav.language: English`, `nav.language_lang: en`, `nav.skip: Sari la conținut`, `home.latest: Recente`, `home.all: Toate articolele →`, `index.title: Articole`, `piece.updated: Actualizat`, `piece.min_read: min de citit`, `piece.read_in_other: Read in English →`, `piece.also_on: Și pe`, `piece.discuss: LinkedIn`, `piece.medium: Medium`, `piece.byline_role: CTO`, `table.label: Tabel`, `pillar.build-log: Jurnal de construcție`, `pillar.opportunity: Oportunități`, `pillar.governance: AI și guvernanță`, `pillar.analysis: Analiză`, `lang.en: Engleză`, `lang.ro: Română`, `lang.badge.ro: RO`, `lang.badge.en: EN`, `footer.linkedin: LinkedIn`, `footer.rss: RSS`, `footer.colophon: Colofon`, `footer.privacy: Confidențialitate`, `footer.promise: Fără JavaScript, fără cookie-uri, fără trackere.`, `notfound.title: Pagina nu există`, `notfound.body: Pagina nu există.`, `notfound.home: Acasă`, `date.months: ianuarie,februarie,martie,aprilie,mai,iunie,iulie,august,septembrie,octombrie,noiembrie,decembrie`.

Page files carry a two-field front matter: `---\ntitle: "…"\nsummary: "≤ 160 chars — the meta description"\n---` then Markdown.

- [ ] **Step 2: Write the failing tests**

`internal/content/load_test.go`:
```go
package content

import (
	"strings"
	"testing"
	"time"
)

var fixtureNow = time.Date(2026, 12, 31, 12, 0, 0, 0, Bucharest)

func TestLoadFixture(t *testing.T) {
	s, probs := Load("../../testdata/site", fixtureNow)
	if err := probs.Err(); err != nil {
		t.Fatal(err)
	}
	if len(s.Pieces) != 4 || len(s.ByLang["en"]) != 2 || len(s.ByLang["ro"]) != 2 {
		t.Fatalf("pieces: %d en %d ro %d", len(s.Pieces), len(s.ByLang["en"]), len(s.ByLang["ro"]))
	}
	for i := 1; i < len(s.Pieces); i++ {
		if s.Pieces[i].Date.After(s.Pieces[i-1].Date) {
			t.Error("pieces must be newest first")
		}
	}
	var paired, roOnly *Piece
	for _, p := range s.Pieces {
		switch p.Slug {
		case "paired":
			paired = p
		case "doar-ro":
			roOnly = p
		}
	}
	if paired.Translation == nil || paired.Translation.Slug != "pereche" || paired.Translation.Translation != paired {
		t.Error("pairing by key failed")
	}
	if roOnly.Translation != nil {
		t.Error("RO-only piece must have no translation")
	}
	for _, k := range []string{"home.en", "home.ro", "privacy.en", "privacy.ro", "colophon.en"} {
		if s.Pages[k] == nil {
			t.Errorf("missing page %s", k)
		}
	}
	if s.T("ro", "nav.writing") != "Articole" || s.T("en", "pillar.analysis") != "Analysis" {
		t.Error("strings")
	}
	if got := s.Latest(3); len(got) != 3 {
		t.Errorf("Latest(3) = %d", len(got))
	}
}

func TestLoadRendersBodies(t *testing.T) {
	s, _ := Load("../../testdata/site", fixtureNow)
	if err := s.Render(func(key, dest string) *ImageInfo { return nil }); err != nil {
		t.Fatal(err)
	}
	for _, p := range s.Pieces {
		if !strings.Contains(string(p.Body), "<p>") {
			t.Errorf("%s: body not rendered", p.File)
		}
	}
	if !strings.Contains(string(s.Pages["home.en"].Body), "<p>") {
		t.Error("home not rendered")
	}
}

func TestI18nCompleteness(t *testing.T) {
	_, probs := loadStrings("../../testdata/i18n-broken") // ro.yaml lacks footer.promise
	if err := probs.Err(); err == nil || !strings.Contains(err.Error(), "footer.promise") {
		t.Errorf("want missing-key error, got %v", err)
	}
}

func TestDuplicateKeyInOneLanguage(t *testing.T) {
	_, probs := Load("../../testdata/site-dupkey", fixtureNow) // two EN pieces with key: pair
	if err := probs.Err(); err == nil || !strings.Contains(err.Error(), "key \"pair\"") {
		t.Errorf("want duplicate-key error, got %v", err)
	}
}
```
Create `testdata/i18n-broken/en.yaml` (full set) and `ro.yaml` (missing `footer.promise`); `testdata/site-dupkey/` is a copy of the fixture with an extra `content/en/paired-2.md` using `key: pair`.

- [ ] **Step 3: `i18n.go`**

```go
package content

import (
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

var Langs = []string{"en", "ro"}

// loadStrings reads <dir>/<lang>.yaml for every language and requires identical key sets.
func loadStrings(dir string) (map[string]map[string]string, Problems) {
	var probs Problems
	out := map[string]map[string]string{}
	for _, lang := range Langs {
		file := filepath.Join(dir, lang+".yaml")
		b, err := os.ReadFile(file)
		if err != nil {
			probs.Add(file, 0, "%v", err)
			continue
		}
		m := map[string]string{}
		if err := yaml.Unmarshal(b, &m); err != nil {
			probs.Add(file, 0, "%v", err)
			continue
		}
		out[lang] = m
	}
	if len(out) != len(Langs) {
		return out, probs
	}
	all := map[string]bool{}
	for _, m := range out {
		for k := range m {
			all[k] = true
		}
	}
	keys := make([]string, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, lang := range Langs {
		for _, k := range keys {
			if v, ok := out[lang][k]; !ok || v == "" {
				probs.Add(filepath.Join(dir, lang+".yaml"), 0, "missing string %q", k)
			}
		}
	}
	return out, probs
}
```

- [ ] **Step 4: `load.go`**

```go
package content

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// Page is a non-piece page (home, privacy, colophon) authored in Markdown with a title.
type Page struct {
	Lang    string
	Kind    string
	Title   string
	Summary string // meta description
	Body    template.HTML
	File    string
	piece   *Piece // reuse the piece pipeline for parsing/rendering
}

// Links are the internal links in the page body, for the link check.
func (pg *Page) Links() []LinkRef { return pg.piece.Links }

type Site struct {
	Pieces  []*Piece
	ByLang  map[string][]*Piece
	Pages   map[string]*Page
	Strings map[string]map[string]string
}

var pageFiles = map[string][]string{ // kind -> languages that must have it
	"home": {"en", "ro"}, "privacy": {"en", "ro"}, "colophon": {"en"},
}

// Load parses and validates everything under root. Bodies are parsed, not yet rendered.
func Load(root string, now time.Time) (*Site, Problems) {
	var probs Problems
	s := &Site{ByLang: map[string][]*Piece{}, Pages: map[string]*Page{}}
	s.Strings, probs = loadStrings(filepath.Join(root, "i18n"))

	for _, lang := range Langs {
		dir := filepath.Join(root, "content", lang)
		entries, err := os.ReadDir(dir)
		if err != nil {
			probs.Add(dir, 0, "%v", err)
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".md") {
				continue
			}
			file := filepath.Join(dir, name)
			src, err := os.ReadFile(file)
			if err != nil {
				probs.Add(file, 0, "%v", err)
				continue
			}
			if strings.HasPrefix(name, "_") {
				kind := strings.TrimSuffix(strings.TrimPrefix(name, "_"), ".md")
				pg, ps := parsePage(file, src, lang, kind)
				probs = append(probs, ps...)
				if pg != nil {
					s.Pages[kind+"."+lang] = pg
				}
				continue
			}
			p, body, _, ps := ParseFrontMatter(file, src, lang, now)
			probs = append(probs, ps...)
			if p == nil {
				continue
			}
			probs = append(probs, ParseBody(p, body)...)
			s.Pieces = append(s.Pieces, p)
			s.ByLang[lang] = append(s.ByLang[lang], p)
		}
	}
	for kind, langs := range pageFiles {
		for _, lang := range langs {
			if s.Pages[kind+"."+lang] == nil {
				probs.Add(filepath.Join(root, "content", lang, "_"+kind+".md"), 0, "required page is missing")
			}
		}
	}
	probs = append(probs, pair(s)...)
	sortPieces(s.Pieces)
	for _, lang := range Langs {
		sortPieces(s.ByLang[lang])
	}
	return s, probs
}

func parsePage(file string, src []byte, lang, kind string) (*Page, Problems) {
	var probs Problems
	if !bytes.HasPrefix(src, []byte("---\n")) {
		probs.Add(file, 1, "page needs a front matter with title")
		return nil, probs
	}
	end := bytes.Index(src[4:], []byte("\n---\n"))
	if end < 0 {
		probs.Add(file, 1, "unterminated front matter")
		return nil, probs
	}
	var fm struct {
		Title   string `yaml:"title"`
		Summary string `yaml:"summary"`
	}
	if err := yaml.Unmarshal(src[4:4+end], &fm); err != nil || strings.TrimSpace(fm.Title) == "" {
		probs.Add(file, 1, "page front matter needs a title")
		return nil, probs
	}
	if n := utf8.RuneCountInString(fm.Summary); strings.TrimSpace(fm.Summary) == "" || n > summaryMax {
		probs.Add(file, 1, "page front matter needs a summary of 1–%d characters (the meta description)", summaryMax)
	}
	body := src[4+end+5:]
	p := &Piece{Lang: lang, File: file, Key: "_" + kind, bodyLine: bytes.Count(src[:4+end+5], []byte("\n")) + 1}
	probs = append(probs, ParseBody(p, body)...)
	return &Page{Lang: lang, Kind: kind, Title: strings.TrimSpace(fm.Title), Summary: strings.TrimSpace(fm.Summary), File: file, piece: p}, probs
}

func pair(s *Site) Problems {
	var probs Problems
	byKey := map[string]map[string]*Piece{}
	for _, p := range s.Pieces {
		if byKey[p.Key] == nil {
			byKey[p.Key] = map[string]*Piece{}
		}
		if other, dup := byKey[p.Key][p.Lang]; dup {
			probs.Add(p.File, 1, "key %q already used by %s in the same language", p.Key, other.File)
			continue
		}
		byKey[p.Key][p.Lang] = p
	}
	slugs := map[string]*Piece{}
	for _, p := range s.Pieces {
		id := p.Lang + "/" + p.Slug
		if other, dup := slugs[id]; dup {
			probs.Add(p.File, 1, "slug %q already used by %s", p.Slug, other.File)
		}
		slugs[id] = p
	}
	for _, m := range byKey {
		if en, ro := m["en"], m["ro"]; en != nil && ro != nil {
			en.Translation, ro.Translation = ro, en
		}
	}
	return probs
}

func sortPieces(ps []*Piece) {
	sort.SliceStable(ps, func(i, j int) bool {
		if !ps[i].Date.Equal(ps[j].Date) {
			return ps[i].Date.After(ps[j].Date)
		}
		return ps[i].Slug < ps[j].Slug
	})
}

// Render fills every Body. lookup resolves an image by piece key and relative name.
func (s *Site) Render(lookup func(key, dest string) *ImageInfo) error {
	for _, p := range s.Pieces {
		body, err := RenderBody(p, func(dest string) *ImageInfo { return lookup(p.Key, dest) }, s.T(p.Lang, "table.label"))
		if err != nil {
			return err
		}
		p.Body = body
	}
	for _, pg := range s.Pages {
		body, err := RenderBody(pg.piece, func(dest string) *ImageInfo { return lookup(pg.piece.Key, dest) }, s.T(pg.Lang, "table.label"))
		if err != nil {
			return err
		}
		pg.Body = body
	}
	return nil
}

// Latest returns the n newest pieces across languages.
func (s *Site) Latest(n int) []*Piece {
	if n > len(s.Pieces) {
		n = len(s.Pieces)
	}
	return s.Pieces[:n]
}

// T looks up a UI string; a missing key is a programming error caught by tests, so it panics loudly.
func (s *Site) T(lang, key string) string {
	v, ok := s.Strings[lang][key]
	if !ok {
		panic(fmt.Sprintf("i18n: missing %s.%s", lang, key))
	}
	return v
}

// AllImageRefs lists every (key, dest) the site references, for the images stage.
func (s *Site) AllImageRefs() map[string][]ImageRef {
	out := map[string][]ImageRef{}
	for _, p := range s.Pieces {
		out[p.Key] = append(out[p.Key], p.Images...)
	}
	for _, pg := range s.Pages {
		out[pg.piece.Key] = append(out[pg.piece.Key], pg.piece.Images...)
	}
	return out
}
```

- [ ] **Step 5: Run, fix, pass** — `go test ./internal/content/ 2>&1 | tail -20`. Expected PASS on all Task 1–3 tests.

- [ ] **Step 6: Commit** — `M1a: content — i18n with completeness check, site loading, pairing by key, fixture site` with trailers; push.

---

### Task 4: `images` — responsive variants with a content-hash cache

**Files:**
- Create: `internal/images/process.go`, `internal/images/hash.go`, `internal/images/cache.go`, `internal/images/process_test.go`

**Interfaces:**
- Produces: `images.Info{Src, Srcset string; Width, Height int; IsSVG bool; Files map[string][]byte}` (Files: output path → bytes, for the site writer); `images.Options{Widths []int; Cache *Cache}`; `images.Process(srcPath, key, name string, opt Options) (*Info, error)`; `images.Cache{Dir string}` (content-addressed store under `<Dir>/v1/`; a nil `*Cache` is a no-op); `images.Hash8([]byte) string`; `images.Widths = []int{720, 1440}` (default); `images.URLPrefix = "/img/"`.

- [ ] **Step 1: Write the failing test**

`internal/images/process_test.go`:
```go
package images

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tmpPNG(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "fig.png")
	if err := os.WriteFile(p, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProcessRaster(t *testing.T) {
	src := tmpPNG(t, 1600, 1000)
	info, err := Process(src, "media", "fig.png", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if info.Width != 1440 || info.Height != 900 || info.IsSVG {
		t.Errorf("dims %dx%d svg=%v", info.Width, info.Height, info.IsSVG)
	}
	if !strings.HasPrefix(info.Src, "/img/media/fig.") || !strings.HasSuffix(info.Src, ".1440.png") {
		t.Errorf("src %s", info.Src)
	}
	if !strings.Contains(info.Srcset, ".720.webp 720w") || !strings.Contains(info.Srcset, ".1440.webp 1440w") || strings.Count(info.Srcset, "1440w") != 1 {
		t.Errorf("srcset must list each width once: %s", info.Srcset)
	}
	if len(info.Files) != 3 { // 720.webp, 1440.webp, 1440.png
		t.Errorf("files: %d", len(info.Files))
	}
	for p, b := range info.Files {
		if len(b) == 0 || !strings.HasPrefix(p, "img/media/fig.") {
			t.Errorf("file %s empty or misplaced", p)
		}
	}
	again, _ := Process(src, "media", "fig.png", Options{})
	if again.Src != info.Src {
		t.Error("hash must be stable across runs")
	}
}

func TestProcessCustomWidthsAndCache(t *testing.T) {
	src := tmpPNG(t, 1000, 1000)
	c := &Cache{Dir: t.TempDir()}
	a, err := Process(src, "home", "portrait.png", Options{Widths: []int{320, 640}, Cache: c})
	if err != nil {
		t.Fatal(err)
	}
	if a.Width != 640 || !strings.Contains(a.Srcset, ".320.webp 320w") || !strings.Contains(a.Srcset, ".640.webp 640w") || strings.Contains(a.Srcset, "1440") {
		t.Errorf("%+v", a)
	}
	n := 0
	_ = filepath.WalkDir(c.Dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
		}
		return nil
	})
	if n != 3 {
		t.Errorf("cache holds %d files, want 3 (two WebP, one PNG)", n)
	}
	b, err := Process(src, "home", "portrait.png", Options{Widths: []int{320, 640}, Cache: c})
	if err != nil {
		t.Fatal(err)
	}
	if b.Width != a.Width || b.Height != a.Height || b.Srcset != a.Srcset {
		t.Errorf("cache hit changed the result: %+v vs %+v", a, b)
	}
	for p := range a.Files {
		if !bytes.Equal(a.Files[p], b.Files[p]) {
			t.Errorf("%s differs on cache hit", p)
		}
	}
}

func TestProcessSmallRaster(t *testing.T) {
	info, err := Process(tmpPNG(t, 600, 400), "k", "small.png", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if info.Width != 600 || !strings.Contains(info.Srcset, ".600.webp 600w") || strings.Contains(info.Srcset, "1440") {
		t.Errorf("small images are never upscaled: %+v", info)
	}
}

func TestProcessSVG(t *testing.T) {
	p := filepath.Join(t.TempDir(), "d.svg")
	_ = os.WriteFile(p, []byte(`<svg xmlns="http://www.w3.org/2000/svg" height="24" width="24" viewBox="0 0 24 24"><path d="M0 0h24v24H0z" fill="currentColor"/></svg>`), 0o644) // height before width on purpose
	info, err := Process(p, "k", "d.svg", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsSVG || info.Width != 24 || info.Height != 24 || !strings.HasSuffix(info.Src, ".svg") || info.Srcset != "" {
		t.Errorf("%+v", info)
	}
}

func TestProcessRejectsStyleAttrInSVG(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bad.svg")
	_ = os.WriteFile(p, []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"><rect style="fill:red"/></svg>`), 0o644)
	if _, err := Process(p, "k", "bad.svg", Options{}); err == nil || !strings.Contains(err.Error(), "style=") {
		t.Errorf("want style= error, got %v", err)
	}
}
```

- [ ] **Step 2: Run to see it fail.**

- [ ] **Step 3: `hash.go`**

```go
package images

import (
	"crypto/sha256"
	"encoding/hex"
)

// Hash8 is the first 8 hex chars of SHA-256: enough to make a URL change whenever the bytes do.
func Hash8(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])[:8]
}
```

- [ ] **Step 3b: `cache.go`** — encoded outputs are keyed by names that already embed a content hash (source bytes + width, or the OG inputs + font hashes), so a hit is valid by construction; `v1/` changes when encoder settings change.

```go
package images

import (
	"os"
	"path/filepath"
)

// Cache stores encoded outputs under <Dir>/v1/<name>. Nil or empty Dir disables it. Never part of dist/; gitignored.
type Cache struct{ Dir string }

func (c *Cache) get(name string) ([]byte, bool) {
	if c == nil || c.Dir == "" {
		return nil, false
	}
	b, err := os.ReadFile(filepath.Join(c.Dir, "v1", filepath.FromSlash(name)))
	return b, err == nil
}

func (c *Cache) put(name string, b []byte) {
	if c == nil || c.Dir == "" {
		return
	}
	p := filepath.Join(c.Dir, "v1", filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err == nil {
		_ = os.WriteFile(p, b, 0o644)
	}
}
```

- [ ] **Step 4: `process.go`**

```go
// Package images produces responsive variants and OG cards. Pure Go: no ImageMagick, no cgo.
package images

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/gen2brain/webp"
	"golang.org/x/image/draw"
)

// Widths are the default responsive widths; the portrait passes its own.
var Widths = []int{720, 1440}

const URLPrefix = "/img/"

type Options struct {
	Widths []int  // defaults to Widths; the largest is never exceeded by upscaling
	Cache  *Cache // optional
}

type Info struct {
	Src    string // largest fallback, original format
	Srcset string // WebP candidates
	Width  int    // of the largest emitted variant
	Height int
	IsSVG  bool
	Files  map[string][]byte // dist-relative path → bytes
}

var (
	svgTag    = regexp.MustCompile(`(?is)<svg\b[^>]*>`)
	svgW      = regexp.MustCompile(`(?i)\bwidth="(\d+)(?:px)?"`)
	svgH      = regexp.MustCompile(`(?i)\bheight="(\d+)(?:px)?"`)
	svgStyle  = regexp.MustCompile(`(?i)\bstyle="`)
	svgScript = regexp.MustCompile(`(?i)<script`)
)

// Process reads one source image and returns everything the renderer and the writer need.
func Process(srcPath, key, name string, opt Options) (*Info, error) {
	raw, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, err
	}
	h := Hash8(raw)
	base := strings.TrimSuffix(name, path.Ext(name))
	ext := strings.ToLower(path.Ext(name))
	dir := "img/" + key + "/"

	if ext == ".svg" {
		if svgStyle.Match(raw) {
			return nil, fmt.Errorf("%s: SVG uses style= attributes; use presentation attributes and currentColor", srcPath)
		}
		if svgScript.Match(raw) {
			return nil, fmt.Errorf("%s: SVG contains a script", srcPath)
		}
		tag := svgTag.Find(raw)
		mw, mh := svgW.FindSubmatch(tag), svgH.FindSubmatch(tag)
		if tag == nil || mw == nil || mh == nil {
			return nil, fmt.Errorf("%s: SVG needs explicit width and height attributes on <svg>", srcPath)
		}
		w, _ := strconv.Atoi(string(mw[1]))
		hh, _ := strconv.Atoi(string(mh[1]))
		out := dir + base + "." + h + ".svg"
		return &Info{Src: "/" + out, Width: w, Height: hh, IsSVG: true, Files: map[string][]byte{out: raw}}, nil
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", srcPath, err)
	}
	if format != "png" && format != "jpeg" {
		return nil, fmt.Errorf("%s: only PNG, JPEG and SVG are supported (got %s)", srcPath, format)
	}
	bounds := image.Rect(0, 0, cfg.Width, cfg.Height)
	var decoded image.Image // decoded lazily: a full cache hit never decodes
	decode := func() (image.Image, error) {
		if decoded == nil {
			decoded, _, err = image.Decode(bytes.NewReader(raw))
		}
		return decoded, err
	}

	widths := opt.Widths
	if len(widths) == 0 {
		widths = Widths
	}
	largest := widths[len(widths)-1]
	if cfg.Width < largest {
		largest = cfg.Width // never upscale
	}
	var ws []int
	for _, w := range widths {
		if w < largest {
			ws = append(ws, w)
		}
	}
	ws = append(ws, largest)

	info := &Info{Files: map[string][]byte{}}
	var srcset []string
	fallbackExt := map[string]string{"png": "png", "jpeg": "jpg"}[format]
	for _, w := range ws {
		p := fmt.Sprintf("%s%s.%s.%d.webp", dir, base, h, w)
		wb, ok := opt.Cache.get(p)
		if !ok {
			src, err := decode()
			if err != nil {
				return nil, fmt.Errorf("%s: %w", srcPath, err)
			}
			var buf bytes.Buffer
			if err := webp.Encode(&buf, resize(src, w), webp.Options{Quality: 82}); err != nil {
				return nil, fmt.Errorf("%s: webp: %w", srcPath, err)
			}
			wb = buf.Bytes()
			opt.Cache.put(p, wb)
		}
		info.Files[p] = wb
		srcset = append(srcset, fmt.Sprintf("/%s %dw", p, w))
		if w == largest {
			fp := fmt.Sprintf("%s%s.%s.%d.%s", dir, base, h, w, fallbackExt)
			fb, ok := opt.Cache.get(fp)
			if !ok {
				src, err := decode()
				if err != nil {
					return nil, fmt.Errorf("%s: %w", srcPath, err)
				}
				var buf bytes.Buffer
				if format == "png" {
					err = png.Encode(&buf, resize(src, w))
				} else {
					err = jpeg.Encode(&buf, resize(src, w), &jpeg.Options{Quality: 85})
				}
				if err != nil {
					return nil, err
				}
				fb = buf.Bytes()
				opt.Cache.put(fp, fb)
			}
			info.Files[fp] = fb
			info.Src = "/" + fp
			info.Width, info.Height = w, heightFor(bounds, w)
		}
	}
	info.Srcset = strings.Join(srcset, ", ")
	return info, nil
}

// heightFor keeps the aspect ratio with the same rounding resize uses, so dimensions are known without decoding.
func heightFor(b image.Rectangle, w int) int {
	if b.Dx() == w {
		return b.Dy()
	}
	return int(float64(b.Dy()) * float64(w) / float64(b.Dx()))
}

func resize(src image.Image, w int) image.Image {
	b := src.Bounds()
	if b.Dx() == w {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, heightFor(b, w)))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	return dst
}
```
Note: for JPEG sources the PNG encoder is never used and vice versa; `image/jpeg` and `image/png` are imported for both decode registration and encoding. EXIF orientation is not applied: rotate phone photos before committing them (`check` cannot see it; the page can).

- [ ] **Step 5: Dependencies, run, pass**

```bash
go get github.com/gen2brain/webp@latest golang.org/x/image@latest && go mod tidy && go test ./internal/images/ 2>&1 | tail -8
```
Expected PASS. `gen2brain/webp` runs libwebp under wazero: the first encode pays a warm-up and each encode is slower than native — that is why the cache exists; a `serve` rebuild with unchanged images encodes nothing.

- [ ] **Step 6: Commit** — `M1a: images — responsive WebP variants, SVG pass-through with style/script rejection` with trailers; push.

---

### Task 5: `images` — OG cards and font glyph checks

**Files:**
- Create: `internal/images/og.go`, `internal/images/fonts.go`, `internal/images/og_test.go`, `assets/fonts/og/Newsreader72pt-Medium.ttf`, `assets/fonts/og/SourceSerif4-Regular.ttf`, `assets/fonts/og/OFL-Newsreader.txt`, `assets/fonts/og/LICENSE-SourceSerif4.md` (web fonts land in `assets/fonts/web/` in M1b)

**Interfaces:**
- Produces: `images.OG{Title, Name, Domain, Pillar, Lang string; Portrait []byte /*optional JPEG/PNG*/}`; `images.Fonts{Display, Body *opentype.Font; Hash string}`; `images.LoadFonts(dir string) (*Fonts, error)` (also runs the glyph check); `images.RenderOG(og OG, f *Fonts, cache *Cache) ([]byte /*PNG*/, error)`; `images.CheckGlyphs(fontPath string) error` (requires U+0218–021B, ă â î and their capitals); `images.OGFontFiles = []string{"Newsreader72pt-Medium.ttf", "SourceSerif4-Regular.ttf"}`.

- [ ] **Step 1: Vendor the OG fonts (OFL) — static instances at the optical size and weight the cards use**

Go's `x/image` rasterizer cannot instantiate a variable font, so it would draw the variable master's default (Newsreader wght 400 / opsz 16) while page titles are wght 500. The upstream repositories ship statics; google/fonts does not.

```bash
mkdir -p assets/fonts/og && cd assets/fonts/og
curl -sSL -o Newsreader72pt-Medium.ttf 'https://github.com/productiontype/Newsreader/raw/master/fonts/static/ttf/Newsreader72pt-Medium.ttf'
curl -sSL -o OFL-Newsreader.txt        'https://github.com/productiontype/Newsreader/raw/master/OFL.txt'
curl -sSL -o SourceSerif4-Regular.ttf  'https://github.com/adobe-fonts/source-serif/raw/release/TTF/SourceSerif4-Regular.ttf'
curl -sSL -o LICENSE-SourceSerif4.md   'https://github.com/adobe-fonts/source-serif/raw/release/LICENSE.md'
file *.ttf && ls -l && cd ../../..
```
Expected: two `TrueType Font data` files (≈132 KB and ≈262 KB; URLs verified 2026-09-18) and two licence texts. Web fonts (M1b) are a separate concern: subsets of the variable masters, in `assets/fonts/web/`.

- [ ] **Step 2: Write the failing tests**

`internal/images/og_test.go`:
```go
package images

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestFontsHaveRomanianGlyphs(t *testing.T) {
	for _, f := range OGFontFiles {
		if err := CheckGlyphs("../../assets/fonts/og/" + f); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}

func TestRenderOG(t *testing.T) {
	f, err := LoadFonts("../../assets/fonts/og")
	if err != nil {
		t.Fatal(err)
	}
	if f.Hash == "" {
		t.Error("fonts must carry a hash for the OG cache key")
	}
	b, err := RenderOG(OG{Title: "Ce ar trebui să întrebe un board despre AI înainte de a aproba bugetul — și de ce contează", Name: "Radu Herinean", Domain: "herinean.com", Pillar: "AI ȘI GUVERNANȚĂ", Lang: "RO"}, f, nil)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 1200 || img.Bounds().Dy() != 630 {
		t.Errorf("size %v", img.Bounds())
	}
	if len(b) > 200*1024 {
		t.Errorf("OG PNG is %d bytes; budget 200 KB", len(b))
	}
	b2, _ := RenderOG(OG{Title: "Same", Name: "Radu Herinean", Domain: "herinean.com", Pillar: "ANALYSIS", Lang: "EN"}, f, nil)
	b3, _ := RenderOG(OG{Title: "Same", Name: "Radu Herinean", Domain: "herinean.com", Pillar: "ANALYSIS", Lang: "EN"}, f, nil)
	if !bytes.Equal(b2, b3) {
		t.Error("OG rendering must be deterministic")
	}
}

func TestRenderOGWithPortraitAndCache(t *testing.T) {
	f, err := LoadFonts("../../assets/fonts/og")
	if err != nil {
		t.Fatal(err)
	}
	portrait := image.NewRGBA(image.Rect(0, 0, 300, 400)) // not square: the crop must centre it
	for y := 0; y < 400; y++ {
		for x := 0; x < 300; x++ {
			portrait.Set(x, y, color.RGBA{uint8(x), uint8(y / 2), 90, 255})
		}
	}
	var pb bytes.Buffer
	_ = png.Encode(&pb, portrait)
	c := &Cache{Dir: t.TempDir()}
	og := OG{Title: "A tagline that is long enough to wrap beside the portrait on the home card", Name: "Radu Herinean", Domain: "herinean.com", Lang: "EN", Portrait: pb.Bytes()}
	a, err := RenderOG(og, f, c)
	if err != nil {
		t.Fatal(err)
	}
	if img, _ := png.Decode(bytes.NewReader(a)); img.Bounds().Dx() != 1200 || img.Bounds().Dy() != 630 {
		t.Fatalf("size %v", img.Bounds())
	}
	b, err := RenderOG(og, f, c)
	if err != nil || !bytes.Equal(a, b) {
		t.Error("cache hit must return identical bytes")
	}
	plain, _ := RenderOG(OG{Title: og.Title, Name: og.Name, Domain: og.Domain, Lang: og.Lang}, f, c)
	if bytes.Equal(plain, a) {
		t.Error("the portrait must change the card (and the cache key)")
	}
}
```

- [ ] **Step 3: `fonts.go`**

```go
package images

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
)

// RequiredRunes: Romanian comma-below forms and the three other diacritics, both cases. A font missing any of these
// would let the browser substitute a cedilla lookalike, which is exactly the failure the spec forbids.
var RequiredRunes = []rune("ȘșȚțĂăÂâÎî")

// OGFontFiles live in assets/fonts/og/: static instances (see Task 5 step 1).
var OGFontFiles = []string{"Newsreader72pt-Medium.ttf", "SourceSerif4-Regular.ttf"}

type Fonts struct {
	Display *opentype.Font // Newsreader 72pt Medium: titles
	Body    *opentype.Font // Source Serif 4 Regular: name, labels
	Hash    string         // of both files; part of every OG cache key
}

func CheckGlyphs(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	f, err := sfnt.Parse(b)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	var buf sfnt.Buffer
	var missing []rune
	for _, r := range RequiredRunes {
		gi, err := f.GlyphIndex(&buf, r)
		if err != nil || gi == 0 {
			missing = append(missing, r)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s lacks glyphs for %q", path, string(missing))
	}
	return nil
}

func LoadFonts(dir string) (*Fonts, error) {
	var all []byte
	load := func(name string) (*opentype.Font, error) {
		p := filepath.Join(dir, name)
		if err := CheckGlyphs(p); err != nil {
			return nil, err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		all = append(all, b...)
		return opentype.Parse(b)
	}
	d, err := load(OGFontFiles[0])
	if err != nil {
		return nil, err
	}
	b, err := load(OGFontFiles[1])
	if err != nil {
		return nil, err
	}
	return &Fonts{Display: d, Body: b, Hash: Hash8(all)}, nil
}
```

- [ ] **Step 4: `og.go`** — the export's OG template: paper ground, hairlines, pillar top-left, language top-right, title in Newsreader (≤ 3 lines, ellipsis), name bottom-left, domain bottom-right; the home variant carries the portrait (spec §9) on the right with a hairline border.

```go
package images

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type OG struct {
	Title, Name, Domain, Pillar, Lang string
	Portrait                          []byte // optional JPEG/PNG bytes; the home card
}

const ogPortrait = 200 // px, square

var (
	ogPaper = color.RGBA{0xFA, 0xF8, 0xF4, 0xFF}
	ogInk   = color.RGBA{0x1C, 0x1A, 0x17, 0xFF}
	ogInk2  = color.RGBA{0x5C, 0x57, 0x4F, 0xFF}
	ogRule  = color.RGBA{0xDC, 0xD5, 0xC9, 0xFF}
)

const (
	ogW, ogH   = 1200, 630
	ogMargin   = 80
	ogTitleMax = 3
)

func face(f *opentype.Font, size float64) (font.Face, error) {
	return opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
}

// RenderOG draws a 1200×630 PNG. Deterministic: same input, same bytes. Cached by a hash of every input.
func RenderOG(og OG, f *Fonts, cache *Cache) ([]byte, error) {
	key := "og/" + Hash8([]byte(strings.Join([]string{og.Title, og.Name, og.Domain, og.Pillar, og.Lang, Hash8(og.Portrait), f.Hash}, "\x00"))) + ".png"
	if b, ok := cache.get(key); ok {
		return b, nil
	}
	img := image.NewRGBA(image.Rect(0, 0, ogW, ogH))
	fill(img, image.Rect(0, 0, ogW, ogH), ogPaper)
	fill(img, image.Rect(ogMargin, 118, ogW-ogMargin, 119), ogRule)
	fill(img, image.Rect(ogMargin, 508, ogW-ogMargin, 509), ogRule)

	label, err := face(f.Body, 22)
	if err != nil {
		return nil, err
	}
	drawText(img, label, ogInk2, ogMargin, 96, spaced(strings.ToUpper(og.Pillar)))
	lang := spaced(strings.ToUpper(og.Lang))
	drawText(img, label, ogInk2, ogW-ogMargin-textWidth(label, lang), 96, lang)

	titleWidth := ogW - 2*ogMargin
	if len(og.Portrait) > 0 {
		src, _, err := image.Decode(bytes.NewReader(og.Portrait))
		if err != nil {
			return nil, fmt.Errorf("og portrait: %w", err)
		}
		sq := squareCrop(src)
		small := image.NewRGBA(image.Rect(0, 0, ogPortrait, ogPortrait))
		draw.CatmullRom.Scale(small, small.Bounds(), sq, sq.Bounds(), draw.Src, nil)
		x, y := ogW-ogMargin-ogPortrait, 213
		draw.Draw(img, image.Rect(x, y, x+ogPortrait, y+ogPortrait), small, image.Point{}, draw.Src)
		fill(img, image.Rect(x-1, y-1, x+ogPortrait+1, y), ogRule) // hairline, as on the page
		fill(img, image.Rect(x-1, y+ogPortrait, x+ogPortrait+1, y+ogPortrait+1), ogRule)
		fill(img, image.Rect(x-1, y, x, y+ogPortrait), ogRule)
		fill(img, image.Rect(x+ogPortrait, y, x+ogPortrait+1, y+ogPortrait), ogRule)
		titleWidth -= ogPortrait + 40
	}

	title, err := face(f.Display, 72)
	if err != nil {
		return nil, err
	}
	lines := wrap(title, og.Title, titleWidth, ogTitleMax)
	y := 250
	if len(lines) == 2 {
		y = 290
	} else if len(lines) == 1 {
		y = 330
	}
	for _, l := range lines {
		drawText(img, title, ogInk, ogMargin, y, l)
		y += 84
	}

	name, err := face(f.Body, 28)
	if err != nil {
		return nil, err
	}
	drawText(img, name, ogInk, ogMargin, 556, og.Name)
	dom, _ := face(f.Body, 24)
	drawText(img, dom, ogInk2, ogW-ogMargin-textWidth(dom, og.Domain), 556, og.Domain)

	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	cache.put(key, buf.Bytes())
	return buf.Bytes(), nil
}

// squareCrop takes the centred square of an image.
func squareCrop(m image.Image) image.Image {
	b := m.Bounds()
	side := b.Dx()
	if b.Dy() < side {
		side = b.Dy()
	}
	out := image.NewRGBA(image.Rect(0, 0, side, side))
	draw.Draw(out, out.Bounds(), m, image.Point{X: b.Min.X + (b.Dx()-side)/2, Y: b.Min.Y + (b.Dy()-side)/2}, draw.Src)
	return out
}

func fill(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func drawText(img *image.RGBA, fc font.Face, c color.RGBA, x, baseline int, s string) {
	d := &font.Drawer{Dst: img, Src: image.NewUniform(c), Face: fc, Dot: fixed.P(x, baseline)}
	d.DrawString(s)
}

func textWidth(fc font.Face, s string) int {
	return font.MeasureString(fc, s).Ceil()
}

// spaced letter-spaces a label the way the CSS does (.12em); a thin space between characters is close enough at 22px.
func spaced(s string) string {
	return strings.Join(strings.Split(s, ""), " ")
}

// wrap breaks on spaces to fit width, at most max lines, ellipsis on the last if truncated.
func wrap(fc font.Face, s string, width, max int) []string {
	words := strings.Fields(s)
	var lines []string
	cur := ""
	for i := 0; i < len(words); i++ {
		try := words[i]
		if cur != "" {
			try = cur + " " + words[i]
		}
		if textWidth(fc, try) <= width {
			cur = try
			continue
		}
		if cur == "" { // single word wider than the line: hard cut
			cur = words[i]
		} else {
			i--
		}
		lines = append(lines, cur)
		cur = ""
		if len(lines) == max {
			cur = ""
			break
		}
	}
	if cur != "" && len(lines) < max {
		lines = append(lines, cur)
	}
	if joined := strings.Join(lines, " "); len(lines) == max && joined != strings.Join(words, " ") {
		last := []rune(lines[max-1]) // runes, never bytes: "ș" must not be cut in half before the ellipsis
		for textWidth(fc, string(last)+"…") > width && len(last) > 1 {
			last = last[:len(last)-1]
		}
		lines[max-1] = strings.TrimSpace(string(last)) + "…"
	}
	return lines
}
```

- [ ] **Step 5: Run, pass** — `go test ./internal/images/ 2>&1 | tail -6`. Known limitation, accepted: `x/image/font` applies no kerning or GSUB, so an OG title is set slightly looser than the browser sets the same words. Open one PNG and compare with `docs/design/claude-design-export/renders/og-piece.png`.

- [ ] **Step 6: Commit** — `M1a: images — OG cards in Newsreader 72pt Medium / Source Serif 4, portrait variant, cache; Romanian glyph check` with trailers (fonts and licence texts included); push. The hook's `site check` exits 3 here (site.yaml placeholders) — expected, see Global Constraints.

---

### Task 6: `render` — templates, CSS, head metadata, JSON-LD, golden tests

**Files:**
- Create: `internal/render/render.go`, `internal/render/page.go`, `internal/render/jsonld.go`, `internal/render/css.go`, `internal/render/render_test.go`, `templates/base.html`, `templates/entries.html`, `templates/home.html`, `templates/index.html`, `templates/piece.html`, `templates/colophon.html`, `templates/privacy.html`, `templates/404.html`, `templates/scorecard.html`, `assets/css/site.css`
- Modify: `i18n/en.yaml`, `i18n/ro.yaml`, `testdata/site/i18n/*.yaml` (add `date.months_short`)

**Interfaces:**
- Consumes: `config.Config`, `content.Site/Piece/Page`.
- Produces: `render.New(templatesDir, cssPath string) (*Renderer, error)`; `(*Renderer).CSS() string`; `(*Renderer).CSSHash() string` (`sha256-…` base64, of the exact inlined bytes); `render.PageData{…}` (below); `(*Renderer).Render(kind string, d *PageData) ([]byte, error)` for kinds `home|index|piece|colophon|privacy|404`; `(*Renderer).Fragment(name string, data any) ([]byte, error)` (used by `site scorecard`); `render.Alternate{Lang, Href string}`; `render.Entry` (index/home rows); `render.YearGroup{Year int; Entries []Entry}`.

- [ ] **Step 1: Add the short month key** to `i18n/en.yaml`: `date.months_short: Jan,Feb,Mar,Apr,May,Jun,Jul,Aug,Sep,Oct,Nov,Dec` and `i18n/ro.yaml`: `date.months_short: ian.,feb.,mar.,apr.,mai,iun.,iul.,aug.,sept.,oct.,nov.,dec.`; same in `testdata/site/i18n/`.

- [ ] **Step 2: `assets/css/site.css`** — the export's system, hand-written. Tokens verbatim; system mono for code (decided 2026-09-18); no `data-scheme`, no `cqw`. Code colours are the palette (accent for keywords/tags, ink-2 italic for comments, ink-2 for strings) — contrast is already verified for those tokens and nothing generated can clash with `pre{background}`.

```css
/* herinean.com — one column, one accent, hairlines. Tokens from docs/design/export-review.md. */
:root{--paper:#FAF8F4;--paper-2:#F1EDE6;--ink:#1C1A17;--ink-2:#5C574F;--rule:#DCD5C9;--accent:#7D2B22;--accent-visited:#5B322C;--sel:#EBD8D1;
--serif:"Source Serif 4",Georgia,"Times New Roman",serif;--display:"Newsreader",Georgia,"Times New Roman",serif;--mono:ui-monospace,"Cascadia Mono","SF Mono",Menlo,Consolas,monospace;
--fs--2:.76rem;--fs--1:.95rem;--fs-0:1.1875rem;--fs-1:1.484rem;--fs-2:1.855rem;--fs-3:2.319rem;--fs-4:2.899rem;
--s1:.5rem;--s2:.85rem;--s3:1.05rem;--s4:1.6rem;--s5:2.2rem;--s6:3rem;color-scheme:light dark}
@media (prefers-color-scheme:dark){:root{--paper:#191714;--paper-2:#221F1B;--ink:#EAE4DA;--ink-2:#A69C8E;--rule:#38332C;--accent:#E2907E;--accent-visited:#C29C93;--sel:#4A2E27}}
*,*::before,*::after{box-sizing:border-box}
html{-webkit-text-size-adjust:100%;text-size-adjust:100%;hyphens:auto}
body{margin:0;background:var(--paper);color:var(--ink);font:400 var(--fs-0)/1.6 var(--serif);font-synthesis:none;-webkit-font-smoothing:antialiased}
.wrap{max-width:38rem;margin:0 auto;padding:0 clamp(1rem,4vw,1.5rem)}
.skip{position:absolute;left:-999px;top:0;padding:var(--s1) var(--s3);background:var(--paper);color:var(--ink)}
.skip:focus{left:var(--s3);top:var(--s3);z-index:1}
a{color:var(--accent);text-decoration-thickness:.055em;text-underline-offset:.16em}
a:visited{color:var(--accent-visited)}
a:hover{text-decoration-thickness:.11em}
a:focus-visible{outline:2px solid var(--accent);outline-offset:3px}
::selection{background:var(--sel)}
.mast{display:flex;align-items:baseline;justify-content:space-between;gap:var(--s3);padding:var(--s4) 0 var(--s2)}
.mast .name{font:500 var(--fs-3)/1.1 var(--display);letter-spacing:-.01em;margin:0}
.mast .name a{color:var(--ink);text-decoration:none}
.mast nav a{margin-left:var(--s3);font-size:var(--fs--1)}
.tagline{color:var(--ink-2);font-size:var(--fs--1);margin:0 0 var(--s4)}
.rule{border:0;border-top:1px solid var(--rule);margin:var(--s5) 0}
h1.title{font:500 clamp(var(--fs-3),7vw,var(--fs-4))/1.12 var(--display);letter-spacing:-.01em;text-wrap:balance;margin:var(--s3) 0 var(--s2)}
h2{font:500 var(--fs-2)/1.2 var(--display);text-wrap:balance;margin:var(--s5) 0 var(--s2)}
h3{font-size:var(--fs-1);line-height:1.3;margin:var(--s4) 0 var(--s1)}
p,ul,ol{margin:0 0 var(--s3)}
p{text-wrap:pretty}
li{margin-bottom:var(--s1)}
.meta{color:var(--ink-2);font-size:var(--fs--1);font-variant-numeric:tabular-nums;margin:0 0 var(--s4)}
.label{font-size:var(--fs--2);letter-spacing:.12em;text-transform:uppercase;color:var(--ink-2)}
.badge{font-family:var(--mono);font-size:var(--fs--2);letter-spacing:.08em;color:var(--ink-2)}
.switch{font-size:var(--fs--1);margin:var(--s4) 0 0}
.entries{list-style:none;margin:0;padding:0}
.entry{border-top:1px solid var(--rule);padding:var(--s3) 0}
.entry .t{font:500 var(--fs-1)/1.25 var(--display);margin:.2rem 0 .35rem}
.entry p{margin:0;color:var(--ink-2);font-size:var(--fs--1)}
.year{margin:var(--s5) 0 var(--s2)}
.portrait{float:left;width:clamp(7rem,30vw,10rem);height:auto;margin:.3rem var(--s4) var(--s1) 0;border:1px solid var(--rule)}
@media (prefers-color-scheme:dark){.portrait{border-color:var(--accent)}}
.about::after{content:"";display:block;clear:both}
blockquote{margin:var(--s4) 0;padding-left:var(--s3);border-left:1px solid var(--rule);font-style:italic}
pre{background:var(--paper-2);padding:var(--s3);overflow-x:auto;font:400 var(--fs--1)/1.5 var(--mono);tab-size:2;margin:0 0 var(--s3)}
code{font-family:var(--mono);font-size:.92em}
pre code{font-size:inherit}
pre:focus-visible,.table:focus-visible{outline:2px solid var(--accent);outline-offset:3px}
.chroma .k,.chroma .kd,.chroma .kn,.chroma .kr,.chroma .kt,.chroma .kc,.chroma .kp,.chroma .nt,.chroma .gd{color:var(--accent)}
.chroma .c,.chroma .c1,.chroma .cm,.chroma .cp,.chroma .cs,.chroma .ch{color:var(--ink-2);font-style:italic}
.chroma .s,.chroma .s1,.chroma .s2,.chroma .sb,.chroma .sd,.chroma .sh,.chroma .sx,.chroma .sr,.chroma .dl{color:var(--ink-2)}
.chroma .gh,.chroma .gu{font-weight:600}
.table{overflow-x:auto;border-top:1px solid var(--rule);border-bottom:1px solid var(--rule);margin:var(--s4) 0}
table{border-collapse:collapse;width:100%;font-size:var(--fs--1)}
th,td{text-align:left;vertical-align:top;padding:var(--s1) var(--s2) var(--s1) 0;border-bottom:1px solid var(--rule)}
th{font-weight:600}
tr:last-child td{border-bottom:0}
figure{margin:var(--s4) 0}
figure img,figure picture{display:block;max-width:100%;height:auto}
figcaption{color:var(--ink-2);font-size:var(--fs--1);margin-top:var(--s1)}
img{max-width:100%;height:auto}
.footnotes{color:var(--ink-2);font-size:var(--fs--1);margin-top:var(--s5)}
.footnotes hr{border:0;border-top:1px solid var(--rule);margin:0 0 var(--s2)}
.also{color:var(--ink-2);font-size:var(--fs--1);margin:var(--s5) 0 0;padding-top:var(--s3);border-top:1px solid var(--rule)}
.byline{color:var(--ink-2);font-size:var(--fs--1);margin-top:var(--s3)}
.vh{position:absolute;width:1px;height:1px;overflow:hidden;clip-path:inset(50%);white-space:nowrap}
footer{border-top:1px solid var(--rule);margin-top:var(--s6);padding:var(--s4) 0 var(--s6);color:var(--ink-2);font-size:var(--fs--1)}
footer p{margin:0 0 var(--s1)}
.deps{font:400 var(--fs--2)/1.5 var(--mono);white-space:pre-wrap}
.pass{color:var(--ink)}
.fail{color:var(--accent);font-weight:600}
@media (max-width:34rem){
.sc table,.sc tbody,.sc tr,.sc td{display:block;width:auto}
.sc thead{display:none}
.sc tr{border-top:1px solid var(--rule);padding:var(--s2) 0}
.sc td{padding:.1rem 0;border:0}
.sc td[data-label]::before{content:attr(data-label);display:inline-block;width:6rem;color:var(--ink-2)}
.sc td:first-child{font-weight:600;margin-bottom:.35rem}}
@media print{
.mast nav,footer,.skip,.switch,.also,.portrait{display:none}
body{background:#fff;color:#000;font-size:11pt}
a{color:#000}
a[href^="http"]::after{content:" <" attr(href) ">";font-family:var(--mono);font-size:.76em}
@page{margin:18mm 16mm}}
```

- [ ] **Step 3: Templates** — `html/template`, one file per page kind plus the shared base.

`templates/base.html` (on home the masthead name is the page's `h1`; the tagline lives inside `header` so every element sits in a landmark — axe `page-has-heading-one`, `heading-order`, `region`):
```html
{{define "base"}}<!doctype html>
<html lang="{{.Lang}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.HeadTitle}}</title>
<meta name="description" content="{{.Description}}">
{{if .Robots}}<meta name="robots" content="{{.Robots}}">{{end}}
<link rel="canonical" href="{{.Canonical}}">
{{range .Alternates}}<link rel="alternate" hreflang="{{.Lang}}" href="{{.Href}}">
{{end}}<link rel="alternate" type="application/rss+xml" title="{{.SiteName}} ({{.Lang}})" href="{{.FeedURL}}">
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
<link rel="icon" href="/favicon.ico" sizes="32x32">
<link rel="apple-touch-icon" href="/apple-touch-icon.png">
<meta property="og:site_name" content="{{.SiteName}}">
<meta property="og:type" content="{{.OGType}}">
<meta property="og:title" content="{{.Title}}">
<meta property="og:description" content="{{.Description}}">
<meta property="og:url" content="{{.Canonical}}">
<meta property="og:image" content="{{.OGImage}}">
<meta property="og:image:width" content="1200">
<meta property="og:image:height" content="630">
<meta property="og:locale" content="{{.OGLocale}}">
<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:title" content="{{.Title}}">
<meta name="twitter:description" content="{{.Description}}">
<meta name="twitter:image" content="{{.OGImage}}">
<script type="application/ld+json">{{.JSONLD}}</script>
<style>{{.CSS}}</style>
</head>
<body>
<a class="skip" href="#main">{{.T "nav.skip"}}</a>
<div class="wrap">
<header>
<div class="mast">
{{if eq .Kind "home"}}<h1 class="name"><a href="{{.HomeURL}}">{{.SiteName}}</a></h1>{{else}}<p class="name"><a href="{{.HomeURL}}">{{.SiteName}}</a></p>{{end}}
<nav><a href="{{.IndexURL}}">{{.T "nav.writing"}}</a><a href="{{.OtherHomeURL}}" lang="{{.T "nav.language_lang"}}" hreflang="{{.T "nav.language_lang"}}">{{.T "nav.language"}}</a></nav>
</div>
{{if .Tagline}}<p class="tagline">{{.Tagline}}</p>{{end}}
</header>
<main id="main">
{{template "content" .}}
</main>
<footer>
<p><a href="{{.Cfg.Author.LinkedIn}}">{{.T "footer.linkedin"}}</a> · <a href="{{.FeedURL}}">{{.T "footer.rss"}}</a> · <a href="/colophon/">{{.T "footer.colophon"}}</a> · <a href="{{.PrivacyURL}}">{{.T "footer.privacy"}}</a></p>
<p>{{.T "footer.promise"}}</p>
</footer>
</div>
</body>
</html>
{{end}}
```

`templates/home.html`:
```html
{{define "content"}}
<section class="about">
{{if .Portrait}}<picture><source type="image/webp" srcset="{{.Portrait.Srcset}}" sizes="(max-width: 34rem) 30vw, 10rem"><img class="portrait" src="{{.Portrait.Src}}" width="{{.Portrait.Width}}" height="{{.Portrait.Height}}" alt="{{.SiteName}}" fetchpriority="high"></picture>{{end}}
{{.Body}}
</section>
<hr class="rule">
<h2 class="label" id="latest">{{.T "home.latest"}}</h2>
{{template "entries" .}}
<p><a href="{{.IndexURL}}">{{.T "home.all"}}</a></p>
{{end}}
```

`templates/index.html`:
```html
{{define "content"}}
<h1 class="title">{{.T "index.title"}}</h1>
{{range .Years}}<h2 class="year label">{{.Year}}</h2>
{{template "entries" .}}
{{end}}
{{end}}
```

`templates/entries.html` (shared partial; note `lang` on foreign titles and the visually-hidden badge name):
```html
{{define "entries"}}<ul class="entries">
{{range .Entries}}<li class="entry">
<p class="meta"><time datetime="{{.DateISO}}">{{.DateText}}</time> · <span class="label">{{.PillarText}}</span>{{if .Foreign}} · <span class="badge"><span lang="{{.Lang}}">{{.Badge}}</span><span class="vh"> ({{.LangName}})</span></span>{{end}}</p>
<p class="t"><a href="{{.URL}}"{{if .Foreign}} lang="{{.Lang}}" hreflang="{{.Lang}}"{{end}}>{{.Title}}</a></p>
<p{{if .Foreign}} lang="{{.Lang}}"{{end}}>{{.Summary}}</p>
</li>
{{end}}</ul>{{end}}
```

`templates/piece.html`:
```html
{{define "content"}}
<article>
{{if .Translation}}<p class="switch"><a href="{{.Translation.URL}}" lang="{{.Translation.Lang}}" hreflang="{{.Translation.Lang}}">{{.T "piece.read_in_other"}}</a></p>{{end}}
<h1 class="title">{{.Piece.Title}}</h1>
<p class="meta"><time datetime="{{.DateISO}}">{{.DateText}}</time>{{if .UpdatedText}} · {{.T "piece.updated"}} <time datetime="{{.UpdatedISO}}">{{.UpdatedText}}</time>{{end}} · {{.Piece.ReadingMinutes}} {{.T "piece.min_read"}} · <span class="label">{{.PillarText}}</span></p>
{{.Body}}
{{if or .Piece.LinkedIn .Piece.Medium}}<p class="also">{{.T "piece.also_on"}} {{if .Piece.LinkedIn}}<a href="{{.Piece.LinkedIn}}">{{.T "piece.discuss"}}</a>{{end}}{{if and .Piece.LinkedIn .Piece.Medium}} · {{end}}{{if .Piece.Medium}}<a href="{{.Piece.Medium}}">{{.T "piece.medium"}}</a>{{end}}</p>{{end}}
<p class="byline">{{.SiteName}} — {{.T "piece.byline_role"}}. <a href="{{.Cfg.Author.LinkedIn}}">LinkedIn</a></p>
</article>
{{end}}
```

`templates/colophon.html`:
```html
{{define "content"}}
<h1 class="title">{{.Page.Title}}</h1>
{{.Body}}
<div class="sc" id="scorecard">{{.ScorecardHTML}}</div>
<h2>Dependencies</h2>
<p class="deps">{{.Deps}}</p>
<h2>Build</h2>
<p class="meta">Commit {{.Commit}} · built {{.BuildDate}} · {{.GoVersion}}</p>
{{end}}
```

`templates/privacy.html`:
```html
{{define "content"}}
<h1 class="title">{{.Page.Title}}</h1>
{{.Body}}
{{end}}
```

`templates/404.html`:
```html
{{define "content"}}
<h1 class="title">{{.T "notfound.title"}}</h1>
<p>{{.T "notfound.body"}} <a href="/">{{.T "notfound.home"}}</a> · <a href="/writing/">Writing</a></p>
<p lang="ro">Pagina nu există. <a href="/ro/" hreflang="ro">Acasă</a> · <a href="/ro/articole/" hreflang="ro">Articole</a></p>
{{end}}
```

`templates/scorecard.html` (a fragment; also rendered by `site scorecard` for KV; `data-scorecard` is the hook the Worker's HTMLRewriter fills in M1b — spec §5.4):
```html
{{define "scorecard"}}<table data-scorecard>
<thead><tr><th>Check</th><th>Result</th><th>Value</th><th>Verified</th><th>Verify</th></tr></thead>
<tbody>
{{range .Rows}}<tr>
<td>{{.Check}}</td>
<td data-label="Result"><span class="{{if .Pass}}pass{{else}}fail{{end}}">{{if .Pass}}Pass{{else}}Fail{{end}}</span></td>
<td data-label="Value">{{.Value}}</td>
<td data-label="Verified"><time datetime="{{.When}}">{{.When}}</time>{{if .Stale}} (stale){{end}}</td>
<td data-label="Verify">{{if .Link}}<a href="{{.Link}}">{{.LinkText}}</a>{{end}}</td>
</tr>
{{end}}</tbody>
</table>{{end}}
```

- [ ] **Step 4: `page.go`** — the data every template sees, and the builders that fill it.

```go
package render

import (
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/raduherinean/herinean.com/internal/config"
	"github.com/raduherinean/herinean.com/internal/content"
)

type Alternate struct{ Lang, Href string }

type Entry struct {
	URL, Title, Summary, Lang, Badge, LangName, PillarText, DateISO, DateText string
	Foreign                                                                  bool
}

type YearGroup struct {
	Year    int
	Entries []Entry
}

type ScorecardRow struct {
	Check, Value, When, Link, LinkText string
	Pass, Stale                        bool
}

type PageData struct {
	Cfg      *config.Config
	Site     *content.Site
	Lang     string
	Kind     string
	Title    string // og:title / h1
	HeadTitle string // <title>
	Description string
	Path     string // site-relative, with trailing slash
	Canonical string
	Alternates []Alternate
	OGImage  string
	OGType   string
	OGLocale string
	Robots   string
	JSONLD   template.JS
	CSS      template.CSS
	Body     template.HTML
	Tagline  string

	Piece       *content.Piece
	Page        *content.Page
	Translation *Entry
	DateISO, DateText, UpdatedISO, UpdatedText, PillarText string
	Entries     []Entry
	Years       []YearGroup
	Portrait    *content.ImageInfo

	ScorecardHTML template.HTML
	Deps, Commit, BuildDate, GoVersion string
}

func (d *PageData) SiteName() string     { return d.Cfg.Name }
func (d *PageData) HomeURL() string      { return d.Cfg.HomeURL(d.Lang) }
func (d *PageData) IndexURL() string     { return d.Cfg.IndexURL(d.Lang) }
func (d *PageData) PrivacyURL() string   { return d.Cfg.PrivacyURL(d.Lang) }
func (d *PageData) FeedURL() string      { return d.Cfg.FeedURL(d.Lang) }
func (d *PageData) OtherHomeURL() string { return d.Cfg.HomeURL(other(d.Lang)) }
func (d *PageData) T(key string) string  { return d.Site.T(d.Lang, key) }

func other(lang string) string {
	if lang == "ro" {
		return "en"
	}
	return "ro"
}

// DateText formats a date the way each language reads it: "27 Sep 2026" / "27 sept. 2026".
func DateText(s *content.Site, lang string, t time.Time) string {
	months := strings.Split(s.T(lang, "date.months_short"), ",")
	return fmt.Sprintf("%d %s %d", t.Day(), months[t.Month()-1], t.Year())
}

func EntryFor(cfg *config.Config, s *content.Site, viewerLang string, p *content.Piece) Entry {
	return Entry{
		URL: cfg.PieceURL(p.Lang, p.Slug), Title: p.Title, Summary: p.Summary, Lang: p.Lang,
		Badge: s.T(viewerLang, "lang.badge."+p.Lang), LangName: s.T(viewerLang, "lang."+p.Lang),
		PillarText: s.T(viewerLang, "pillar."+p.Pillar),
		DateISO: p.Date.Format("2006-01-02"), DateText: DateText(s, viewerLang, p.Date),
		Foreign: p.Lang != viewerLang,
	}
}

func Years(cfg *config.Config, s *content.Site, viewerLang string, ps []*content.Piece) []YearGroup {
	var out []YearGroup
	for _, p := range ps {
		y := p.Date.Year()
		if len(out) == 0 || out[len(out)-1].Year != y {
			out = append(out, YearGroup{Year: y})
		}
		out[len(out)-1].Entries = append(out[len(out)-1].Entries, EntryFor(cfg, s, viewerLang, p))
	}
	return out
}

// AlternatesFor returns hreflang links for a page that exists in both languages (or just itself).
func AlternatesFor(cfg *config.Config, enPath, roPath string) []Alternate {
	var alts []Alternate
	if enPath != "" {
		alts = append(alts, Alternate{"en", cfg.Abs(enPath)})
	}
	if roPath != "" {
		alts = append(alts, Alternate{"ro", cfg.Abs(roPath)})
	}
	if enPath != "" {
		alts = append(alts, Alternate{"x-default", cfg.Abs(enPath)})
	} else if roPath != "" {
		alts = append(alts, Alternate{"x-default", cfg.Abs(roPath)})
	}
	return alts
}
```

- [ ] **Step 5: `jsonld.go`**

```go
package render

import (
	"encoding/json"
	"html/template"
	"time"

	"github.com/raduherinean/herinean.com/internal/config"
	"github.com/raduherinean/herinean.com/internal/content"
)

func person(cfg *config.Config) map[string]any {
	return map[string]any{
		"@type": "Person", "name": cfg.Name, "url": cfg.BaseURL + "/",
		"sameAs": []string{cfg.Author.LinkedIn, cfg.Author.X, cfg.Author.GitHub},
	}
}

func WebSiteLD(cfg *config.Config, lang, description string) template.JS {
	return jsonLD(map[string]any{
		"@context": "https://schema.org", "@graph": []any{
			map[string]any{"@type": "WebSite", "@id": cfg.BaseURL + "/#website", "url": cfg.BaseURL + "/", "name": cfg.Name, "description": description, "inLanguage": lang, "author": map[string]any{"@id": cfg.BaseURL + "/#person"}},
			withID(person(cfg), cfg.BaseURL+"/#person"),
		},
	})
}

func ArticleLD(cfg *config.Config, p *content.Piece, ogImage, indexTitle string) template.JS {
	url := cfg.Abs(cfg.PieceURL(p.Lang, p.Slug))
	mod := p.Date
	if p.Updated != nil {
		mod = *p.Updated
	}
	crumbs := []any{
		map[string]any{"@type": "ListItem", "position": 1, "name": cfg.Name, "item": cfg.Abs(cfg.HomeURL(p.Lang))},
		map[string]any{"@type": "ListItem", "position": 2, "name": indexTitle, "item": cfg.Abs(cfg.IndexURL(p.Lang))},
		map[string]any{"@type": "ListItem", "position": 3, "name": p.Title, "item": url},
	}
	return jsonLD(map[string]any{
		"@context": "https://schema.org", "@graph": []any{
			map[string]any{
				"@type": "BlogPosting", "@id": url + "#article", "mainEntityOfPage": url, "url": url,
				"headline": p.Title, "description": p.Summary, "inLanguage": p.Lang,
				"datePublished": p.Date.Format(time.RFC3339), "dateModified": mod.Format(time.RFC3339),
				"image": ogImage, "author": map[string]any{"@id": cfg.BaseURL + "/#person"}, "publisher": map[string]any{"@id": cfg.BaseURL + "/#person"},
				"articleSection": p.Pillar, "wordCount": p.Words,
			},
			map[string]any{"@type": "BreadcrumbList", "itemListElement": crumbs},
			withID(person(cfg), cfg.BaseURL+"/#person"),
		},
	})
}

func withID(m map[string]any, id string) map[string]any { m["@id"] = id; return m }

func jsonLD(v any) template.JS {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	// html/template escapes "<" inside a script as < already via JS context; keep bytes plain.
	return template.JS(b)
}
```

- [ ] **Step 6: `css.go`** — minify (comments and whitespace only; the CSS is hand-written) and hash.

```go
package render

import (
	"crypto/sha256"
	"encoding/base64"
	"regexp"
	"strings"
)

var (
	cssComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	cssSpace   = regexp.MustCompile(`\s*([{}:;,>])\s*`)
	cssNL      = regexp.MustCompile(`\s*\n\s*`)
)

func minifyCSS(s string) string {
	s = cssComment.ReplaceAllString(s, "")
	s = cssNL.ReplaceAllString(s, "")
	s = cssSpace.ReplaceAllString(s, "$1")
	s = strings.ReplaceAll(s, ";}", "}")
	return strings.TrimSpace(s)
}

// CSPHash returns the style-src hash for the exact bytes inlined in <style>.
func CSPHash(css string) string {
	sum := sha256.Sum256([]byte(css))
	return "sha256-" + base64.StdEncoding.EncodeToString(sum[:])
}
```

- [ ] **Step 7: `render.go`**

```go
// Package render turns page data into HTML with html/template.
package render

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
)

type Renderer struct {
	base  *template.Template            // base + entries + scorecard
	pages map[string]*template.Template // kind → clone of base with that page's "content"
	css   string
	hash  string
}

var kinds = []string{"home", "index", "piece", "colophon", "privacy", "404"}

// New parses the shared templates once, then one clone per page kind. Every page file defines "content";
// cloning before parsing each keeps the definitions apart (in one set, the last file parsed would win for all kinds).
func New(templatesDir, cssPath string) (*Renderer, error) {
	raw, err := os.ReadFile(cssPath)
	if err != nil {
		return nil, err
	}
	css := minifyCSS(string(raw))
	shared := []string{"base.html", "entries.html", "scorecard.html"}
	for i, f := range shared {
		shared[i] = filepath.Join(templatesDir, f)
	}
	base, err := template.ParseFiles(shared...)
	if err != nil {
		return nil, err
	}
	r := &Renderer{base: base, pages: map[string]*template.Template{}, css: css, hash: CSPHash(css)}
	for _, k := range kinds {
		t, err := base.Clone()
		if err != nil {
			return nil, err
		}
		if _, err := t.ParseFiles(filepath.Join(templatesDir, k+".html")); err != nil {
			return nil, err
		}
		r.pages[k] = t
	}
	return r, nil
}

func (r *Renderer) CSS() string     { return r.css }
func (r *Renderer) CSSHash() string { return r.hash }

// Render executes the page template for kind (home, index, piece, colophon, privacy, 404).
func (r *Renderer) Render(kind string, d *PageData) ([]byte, error) {
	t := r.pages[kind]
	if t == nil {
		return nil, fmt.Errorf("render: no template for kind %q", kind)
	}
	d.CSS = template.CSS(r.css)
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "base", d); err != nil {
		return nil, fmt.Errorf("render %s: %w", kind, err)
	}
	return buf.Bytes(), nil
}

// Fragment executes a named shared template (e.g. "scorecard").
func (r *Renderer) Fragment(name string, data any) ([]byte, error) {
	var buf bytes.Buffer
	if err := r.base.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
```

- [ ] **Step 8: Golden tests**

`internal/render/render_test.go`:
```go
package render

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/raduherinean/herinean.com/internal/config"
	"github.com/raduherinean/herinean.com/internal/content"
)

var update = flag.Bool("update", false, "rewrite golden files")

func fixture(t *testing.T) (*config.Config, *content.Site, *Renderer) {
	t.Helper()
	cfg, err := config.Load("../../testdata/site/site.yaml")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 12, 31, 12, 0, 0, 0, content.Bucharest)
	s, probs := content.Load("../../testdata/site", now)
	if err := probs.Err(); err != nil {
		t.Fatal(err)
	}
	if err := s.Render(func(key, dest string) *content.ImageInfo {
		return &content.ImageInfo{Src: "/img/" + key + "/" + dest, Width: 1440, Height: 900, Srcset: "/img/" + key + "/" + dest + " 1440w"}
	}); err != nil {
		t.Fatal(err)
	}
	r, err := New("../../templates", "../../assets/css/site.css")
	if err != nil {
		t.Fatal(err)
	}
	return cfg, s, r
}

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	p := filepath.Join("../../testdata/golden", name)
	if *update {
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("no golden %s (run with -update): %v", p, err)
	}
	if !bytes.Equal(want, got) {
		t.Errorf("%s differs from golden; diff with: diff <(cat %s) <(go test ./internal/render -run %s -v)", name, p, t.Name())
	}
}

func TestPieceGolden(t *testing.T) {
	cfg, s, r := fixture(t)
	for _, p := range s.Pieces {
		d := &PageData{Cfg: cfg, Site: s, Lang: p.Lang, Kind: "piece", Piece: p, Title: p.Title, HeadTitle: p.Title + " — " + cfg.Name,
			Description: p.Summary, Path: cfg.PieceURL(p.Lang, p.Slug), Canonical: cfg.Abs(cfg.PieceURL(p.Lang, p.Slug)),
			OGImage: cfg.Abs("/og/" + p.Lang + "-" + p.Slug + ".deadbeef.png"), OGType: "article", OGLocale: map[string]string{"en": "en_US", "ro": "ro_RO"}[p.Lang],
			Body: p.Body, DateISO: p.Date.Format("2006-01-02"), DateText: DateText(s, p.Lang, p.Date), PillarText: s.T(p.Lang, "pillar."+p.Pillar)}
		en, ro := "", ""
		if p.Lang == "en" {
			en = d.Path
		} else {
			ro = d.Path
		}
		if p.Translation != nil {
			e := EntryFor(cfg, s, p.Lang, p.Translation)
			d.Translation = &e
			if p.Lang == "en" {
				ro = e.URL
			} else {
				en = e.URL
			}
		}
		d.Alternates = AlternatesFor(cfg, en, ro)
		d.JSONLD = ArticleLD(cfg, p, d.OGImage, s.T(p.Lang, "index.title"))
		out, err := r.Render("piece", d)
		if err != nil {
			t.Fatal(err)
		}
		golden(t, "piece-"+p.Lang+"-"+p.Slug+".html", out)
		assertInvariants(t, out)
	}
}

func TestHomeIndexGolden(t *testing.T) {
	cfg, s, r := fixture(t)
	for _, lang := range content.Langs {
		var entries []Entry
		for _, p := range s.Latest(5) {
			entries = append(entries, EntryFor(cfg, s, lang, p))
		}
		home := s.Pages["home."+lang]
		d := &PageData{Cfg: cfg, Site: s, Lang: lang, Kind: "home", Page: home, Title: cfg.Name, HeadTitle: cfg.Name, Description: cfg.Tagline[lang],
			Path: cfg.HomeURL(lang), Canonical: cfg.Abs(cfg.HomeURL(lang)), OGImage: cfg.Abs("/og/home-" + lang + ".deadbeef.png"), OGType: "website",
			OGLocale: map[string]string{"en": "en_US", "ro": "ro_RO"}[lang], Body: home.Body, Tagline: cfg.Tagline[lang], Entries: entries,
			Alternates: AlternatesFor(cfg, cfg.HomeURL("en"), cfg.HomeURL("ro")), JSONLD: WebSiteLD(cfg, lang, cfg.Tagline[lang])}
		out, err := r.Render("home", d)
		if err != nil {
			t.Fatal(err)
		}
		golden(t, "home-"+lang+".html", out)
		assertInvariants(t, out)

		idx := &PageData{Cfg: cfg, Site: s, Lang: lang, Kind: "index", Title: s.T(lang, "index.title"), HeadTitle: s.T(lang, "index.title") + " — " + cfg.Name,
			Description: cfg.Tagline[lang], Path: cfg.IndexURL(lang), Canonical: cfg.Abs(cfg.IndexURL(lang)), OGImage: cfg.Abs("/og/home-" + lang + ".deadbeef.png"),
			OGType: "website", OGLocale: map[string]string{"en": "en_US", "ro": "ro_RO"}[lang], Years: Years(cfg, s, lang, s.Pieces),
			Alternates: AlternatesFor(cfg, cfg.IndexURL("en"), cfg.IndexURL("ro")), JSONLD: WebSiteLD(cfg, lang, cfg.Tagline[lang])}
		out, err = r.Render("index", idx)
		if err != nil {
			t.Fatal(err)
		}
		golden(t, "index-"+lang+".html", out)
		assertInvariants(t, out)
	}
}

func Test404AndPrivacyAndColophon(t *testing.T) {
	cfg, s, r := fixture(t)
	nf := &PageData{Cfg: cfg, Site: s, Lang: "en", Kind: "404", Title: s.T("en", "notfound.title"), HeadTitle: s.T("en", "notfound.title"), Description: s.T("en", "notfound.body"),
		Path: "/404.html", Canonical: cfg.Abs("/404.html"), OGImage: cfg.Abs("/og/home-en.deadbeef.png"), OGType: "website", OGLocale: "en_US", Robots: "noindex", JSONLD: WebSiteLD(cfg, "en", "")}
	out, err := r.Render("404", nf)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "404.html", out)

	pv := s.Pages["privacy.ro"]
	d := &PageData{Cfg: cfg, Site: s, Lang: "ro", Kind: "privacy", Page: pv, Title: pv.Title, HeadTitle: pv.Title + " — " + cfg.Name, Description: pv.Summary,
		Path: cfg.PrivacyURL("ro"), Canonical: cfg.Abs(cfg.PrivacyURL("ro")), OGImage: cfg.Abs("/og/home-ro.deadbeef.png"), OGType: "website", OGLocale: "ro_RO", Body: pv.Body,
		Alternates: AlternatesFor(cfg, cfg.PrivacyURL("en"), cfg.PrivacyURL("ro")), JSONLD: WebSiteLD(cfg, "ro", "")}
	out, err = r.Render("privacy", d)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "privacy-ro.html", out)

	frag, err := r.Fragment("scorecard", map[string]any{"Rows": []ScorecardRow{{Check: "Lighthouse", Pass: true, Value: "100/100/100/100", When: "2026-10-11", Link: "https://pagespeed.web.dev/", LinkText: "PageSpeed"}, {Check: "Uptime", Pass: false, Value: "99.2%", When: "2026-06-01", Stale: true}}})
	if err != nil {
		t.Fatal(err)
	}
	co := s.Pages["colophon.en"]
	c := &PageData{Cfg: cfg, Site: s, Lang: "en", Kind: "colophon", Page: co, Title: co.Title, HeadTitle: co.Title + " — " + cfg.Name, Description: co.Summary,
		Path: "/colophon/", Canonical: cfg.Abs("/colophon/"), OGImage: cfg.Abs("/og/home-en.deadbeef.png"), OGType: "website", OGLocale: "en_US", Body: co.Body,
		Alternates: AlternatesFor(cfg, "/colophon/", ""), JSONLD: WebSiteLD(cfg, "en", ""), ScorecardHTML: template.HTML(frag), Deps: "github.com/yuin/goldmark v1.7.8", Commit: "abc1234", BuildDate: "2026-10-11", GoVersion: "go1.27.1"}
	out, err = r.Render("colophon", c)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "colophon.html", out)
	assertInvariants(t, out)
}

func TestCSSHashMatchesInlined(t *testing.T) {
	_, _, r := fixture(t)
	if !strings.HasPrefix(r.CSSHash(), "sha256-") || CSPHash(r.CSS()) != r.CSSHash() {
		t.Error("hash must be of the exact inlined CSS")
	}
	if strings.Contains(r.CSS(), "/*") || strings.Contains(r.CSS(), "\n") {
		t.Error("CSS must be minified")
	}
}

func assertInvariants(t *testing.T, html []byte) {
	t.Helper()
	s := string(html)
	if strings.Count(s, "<script") != 1 || !strings.Contains(s, `<script type="application/ld+json">`) {
		t.Error("exactly one script, and it must be JSON-LD")
	}
	if strings.Contains(s, " style=") {
		t.Error("no style attributes (hash CSP)")
	}
	for _, must := range []string{`rel="canonical"`, `property="og:image"`, `name="twitter:card"`, `hreflang="x-default"`, `rel="alternate" type="application/rss+xml"`, `class="skip"`, `<main id="main">`} {
		if !strings.Contains(s, must) {
			t.Errorf("missing %s", must)
		}
	}
	if strings.Count(s, "<h1") != 1 {
		t.Errorf("every page has exactly one h1, got %d", strings.Count(s, "<h1"))
	}
}
```
(`template` import: add `"html/template"` to the test imports for `template.HTML(frag)`.)

- [ ] **Step 9: First run writes the goldens; inspect them against the renders in `docs/design/claude-design-export/renders/`**

```bash
go test ./internal/render/ -update && go test ./internal/render/ && ls testdata/golden/
```
Open two goldens in a browser via `python3 -m http.server -d testdata/golden 8090` and compare with the renders: masthead, tagline, entries, piece meta line, colophon table. Adjust CSS only where structure differs from the export; commit the goldens.

- [ ] **Step 10: Commit** — `M1a: render — templates and CSS from the design export, head metadata, JSON-LD, golden tests` with trailers; push.

---

### Task 7: `feeds` — RSS per language + combined, JSON Feed

**Files:**
- Create: `internal/feeds/rss.go`, `internal/feeds/jsonfeed.go`, `internal/feeds/abs.go`, `internal/feeds/feeds_test.go`

**Interfaces:**
- Produces: `feeds.RSS(cfg *config.Config, s *content.Site, lang string /* "" = all */, now time.Time) ([]byte, error)`; `feeds.JSON(cfg, s, now) ([]byte, error)`; `feeds.Absolutize(html, base, pageURL string) string` (href/src/srcset rooted at `base`; `#fragment` links rooted at `pageURL`, so footnotes work in a reader).

- [ ] **Step 1: Write the failing tests**

`internal/feeds/feeds_test.go`:
```go
package feeds

import (
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/raduherinean/herinean.com/internal/config"
	"github.com/raduherinean/herinean.com/internal/content"
)

func fixture(t *testing.T) (*config.Config, *content.Site, time.Time) {
	t.Helper()
	cfg, err := config.Load("../../testdata/site/site.yaml")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 12, 31, 12, 0, 0, 0, content.Bucharest)
	s, probs := content.Load("../../testdata/site", now)
	if err := probs.Err(); err != nil {
		t.Fatal(err)
	}
	_ = s.Render(func(key, dest string) *content.ImageInfo {
		return &content.ImageInfo{Src: "/img/" + key + "/" + dest, Width: 10, Height: 10}
	})
	return cfg, s, now
}

func TestAbsolutize(t *testing.T) {
	in := `<p>prose, /not a url</p><a href="/writing/x/">a</a><a href="#fn:1">1</a><img src="/img/k/f.png" srcset="/img/k/f.720.webp 720w, /img/k/f.1440.webp 1440w"><a href="https://ext/">e</a>`
	got := Absolutize(in, "https://herinean.com", "https://herinean.com/writing/y/")
	for _, want := range []string{`<p>prose, /not a url</p>`, `href="https://herinean.com/writing/x/"`, `href="https://herinean.com/writing/y/#fn:1"`, `src="https://herinean.com/img/k/f.png"`, `srcset="https://herinean.com/img/k/f.720.webp 720w, https://herinean.com/img/k/f.1440.webp 1440w"`, `href="https://ext/"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
}

func TestRSS(t *testing.T) {
	cfg, s, now := fixture(t)
	for _, lang := range []string{"", "en", "ro"} {
		b, err := RSS(cfg, s, lang, now)
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			XMLName xml.Name `xml:"rss"`
			Channel struct {
				Title string `xml:"title"`
				Link  string `xml:"link"`
				Items []struct {
					Title   string `xml:"title"`
					Link    string `xml:"link"`
					PubDate string `xml:"pubDate"`
					Content string `xml:"encoded"`
				} `xml:"item"`
			} `xml:"channel"`
		}
		if err := xml.Unmarshal(b, &doc); err != nil {
			t.Fatalf("%s: invalid xml: %v", lang, err)
		}
		want := len(s.Pieces)
		if lang != "" {
			want = len(s.ByLang[lang])
		}
		if len(doc.Channel.Items) != want {
			t.Errorf("%s: %d items want %d", lang, len(doc.Channel.Items), want)
		}
		sb := string(b)
		if !strings.Contains(sb, `<atom:link href="https://herinean.com/feed`) || !strings.Contains(sb, `rel="self"`) {
			t.Errorf("%s: missing atom:link self", lang)
		}
		if strings.Contains(sb, `href="/`) || strings.Contains(sb, `src="/`) {
			t.Errorf("%s: relative URL leaked into the feed", lang)
		}
		if !strings.HasPrefix(doc.Channel.Items[0].Link, "https://herinean.com/") || doc.Channel.Items[0].Content == "" {
			t.Errorf("%s: item %+v", lang, doc.Channel.Items[0])
		}
	}
}

func TestJSONFeed(t *testing.T) {
	cfg, s, now := fixture(t)
	b, err := JSON(cfg, s, now)
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Version string `json:"version"`
		FeedURL string `json:"feed_url"`
		Items   []struct {
			ID          string `json:"id"`
			URL         string `json:"url"`
			ContentHTML string `json:"content_html"`
			Language    string `json:"language"`
		} `json:"items"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	if f.Version != "https://jsonfeed.org/version/1.1" || f.FeedURL != "https://herinean.com/feed.json" || len(f.Items) != len(s.Pieces) {
		t.Errorf("%+v", f)
	}
	if it := f.Items[0]; it.ID != it.URL || !strings.HasPrefix(it.URL, "https://herinean.com/") || it.ContentHTML == "" || strings.Contains(it.ContentHTML, `href="/`) || (it.Language != "en" && it.Language != "ro") {
		t.Errorf("item: %+v", it)
	}
}
```

- [ ] **Step 2: Run to see them fail.**

- [ ] **Step 3: `abs.go`**

```go
package feeds

import (
	"regexp"
	"strings"
)

var srcsetAttr = regexp.MustCompile(`srcset="([^"]*)"`)

// Absolutize rewrites site-relative href/src/srcset values to absolute URLs and roots #fragments at the page, for feed readers and aggregators.
func Absolutize(html, base, pageURL string) string {
	html = strings.NewReplacer(`href="/`, `href="`+base+`/`, `src="/`, `src="`+base+`/`, `href="#`, `href="`+pageURL+`#`).Replace(html)
	return srcsetAttr.ReplaceAllStringFunc(html, func(m string) string {
		parts := strings.Split(m[len(`srcset="`):len(m)-1], ",")
		for i, c := range parts {
			c = strings.TrimSpace(c)
			if strings.HasPrefix(c, "/") {
				c = base + c
			}
			parts[i] = c
		}
		return `srcset="` + strings.Join(parts, ", ") + `"`
	})
}
```

- [ ] **Step 4: `rss.go`**

```go
// Package feeds writes RSS 2.0 and JSON Feed 1.1 with full content.
package feeds

import (
	"bytes"
	"encoding/xml"
	"time"

	"github.com/raduherinean/herinean.com/internal/config"
	"github.com/raduherinean/herinean.com/internal/content"
)

type rss struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Atom    string   `xml:"xmlns:atom,attr"`
	Content string   `xml:"xmlns:content,attr"`
	Channel channel  `xml:"channel"`
}

type channel struct {
	Title         string   `xml:"title"`
	Link          string   `xml:"link"`
	Description   string   `xml:"description"`
	Language      string   `xml:"language,omitempty"`
	LastBuildDate string   `xml:"lastBuildDate"`
	AtomLink      atomLink `xml:"atom:link"`
	Items         []item   `xml:"item"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        guid   `xml:"guid"`
	PubDate     string `xml:"pubDate"`
	Category    string `xml:"category"`
	Description string `xml:"description"`
	Content     string `xml:"content:encoded"`
}

type guid struct {
	IsPermaLink bool   `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

// pub places a date-only publication at 06:00 Bucharest so pubDate is stable and sensible.
func pub(t time.Time) string { return t.Add(6 * time.Hour).Format(time.RFC1123Z) }

func RSS(cfg *config.Config, s *content.Site, lang string, now time.Time) ([]byte, error) {
	pieces := s.Pieces
	title, desc := cfg.Name, cfg.Tagline["en"]
	if lang != "" {
		pieces = s.ByLang[lang]
		desc = cfg.Tagline[lang]
	}
	ch := channel{Title: title, Link: cfg.Abs(cfg.HomeURL(lang)), Description: desc, Language: lang,
		LastBuildDate: now.Format(time.RFC1123Z), AtomLink: atomLink{Href: cfg.Abs(cfg.FeedURL(lang)), Rel: "self", Type: "application/rss+xml"}}
	if lang == "" {
		ch.Link = cfg.Abs("/")
	}
	for _, p := range pieces {
		u := cfg.Abs(cfg.PieceURL(p.Lang, p.Slug))
		ch.Items = append(ch.Items, item{Title: p.Title, Link: u, GUID: guid{IsPermaLink: true, Value: u}, PubDate: pub(p.Date),
			Category: s.T(p.Lang, "pillar."+p.Pillar), Description: p.Summary, Content: Absolutize(string(p.Body), cfg.BaseURL, u)})
	}
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(rss{Version: "2.0", Atom: "http://www.w3.org/2005/Atom", Content: "http://purl.org/rss/1.0/modules/content/", Channel: ch}); err != nil {
		return nil, err
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}
```

- [ ] **Step 5: `jsonfeed.go`**

```go
package feeds

import (
	"encoding/json"
	"time"

	"github.com/raduherinean/herinean.com/internal/config"
	"github.com/raduherinean/herinean.com/internal/content"
)

type jsonFeed struct {
	Version     string       `json:"version"`
	Title       string       `json:"title"`
	HomePageURL string       `json:"home_page_url"`
	FeedURL     string       `json:"feed_url"`
	Description string       `json:"description"`
	Language    string       `json:"language"`
	Authors     []jsonAuthor `json:"authors"`
	Items       []jsonItem   `json:"items"`
}

type jsonAuthor struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type jsonItem struct {
	ID            string   `json:"id"`
	URL           string   `json:"url"`
	Title         string   `json:"title"`
	ContentHTML   string   `json:"content_html"`
	Summary       string   `json:"summary"`
	DatePublished string   `json:"date_published"`
	DateModified  string   `json:"date_modified,omitempty"`
	Language      string   `json:"language"`
	Tags          []string `json:"tags"`
}

func JSON(cfg *config.Config, s *content.Site, now time.Time) ([]byte, error) {
	f := jsonFeed{Version: "https://jsonfeed.org/version/1.1", Title: cfg.Name, HomePageURL: cfg.Abs("/"), FeedURL: cfg.Abs("/feed.json"),
		Description: cfg.Tagline["en"], Language: "en", Authors: []jsonAuthor{{Name: cfg.Name, URL: cfg.Abs("/")}}}
	for _, p := range s.Pieces {
		u := cfg.Abs(cfg.PieceURL(p.Lang, p.Slug))
		it := jsonItem{ID: u, URL: u, Title: p.Title, ContentHTML: Absolutize(string(p.Body), cfg.BaseURL, u), Summary: p.Summary,
			DatePublished: p.Date.Add(6 * time.Hour).Format(time.RFC3339), Language: p.Lang, Tags: []string{p.Pillar}}
		if p.Updated != nil {
			it.DateModified = p.Updated.Add(6 * time.Hour).Format(time.RFC3339)
		}
		f.Items = append(f.Items, it)
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
```

- [ ] **Step 6: Run, pass, commit** — `M1a: feeds — RSS per language and combined, JSON Feed, absolute URLs` with trailers; push.

---

### Task 8: `seo` — sitemap, robots, llms.txt, security.txt

**Files:**
- Create: `internal/seo/sitemap.go`, `internal/seo/files.go`, `internal/seo/seo_test.go`

**Interfaces:**
- Produces: `seo.URLEntry{Loc string; LastMod time.Time; Alternates []seo.Alt}`; `seo.Alt{Lang, Href string}`; `seo.Sitemap(entries []URLEntry) []byte`; `seo.Robots(cfg) []byte`; `seo.LLMs(cfg, s *content.Site) []byte`; `seo.SecurityTxt(cfg, build time.Time) []byte`.

- [ ] **Step 1: Write the failing test**

```go
package seo

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/raduherinean/herinean.com/internal/config"
)

func cfg(t *testing.T) *config.Config {
	c, err := config.Load("../../testdata/site/site.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestSitemap(t *testing.T) {
	d := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	b := Sitemap([]URLEntry{{Loc: "https://herinean.com/writing/x/", LastMod: d, Alternates: []Alt{{"en", "https://herinean.com/writing/x/"}, {"ro", "https://herinean.com/ro/articole/y/"}, {"x-default", "https://herinean.com/writing/x/"}}}})
	var doc struct {
		XMLName xml.Name `xml:"urlset"`
		URLs    []struct {
			Loc     string `xml:"loc"`
			LastMod string `xml:"lastmod"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if doc.URLs[0].LastMod != "2026-09-27" || !strings.Contains(s, `xmlns:xhtml="http://www.w3.org/1999/xhtml"`) || !strings.Contains(s, `<xhtml:link rel="alternate" hreflang="x-default" href="https://herinean.com/writing/x/"/>`) {
		t.Errorf("%s", s)
	}
}

func TestFiles(t *testing.T) {
	c := cfg(t)
	if r := string(Robots(c)); !strings.Contains(r, "Sitemap: https://herinean.com/sitemap.xml") || !strings.Contains(r, "Allow: /") {
		t.Errorf("robots: %s", r)
	}
	b := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	st := string(SecurityTxt(c, b))
	if !strings.Contains(st, "Contact: mailto:security@herinean.com") || !strings.Contains(st, "Expires: 2027-09-26T10:00:00.000Z") || !strings.Contains(st, "Canonical: https://herinean.com/.well-known/security.txt") {
		t.Errorf("security.txt: %s", st)
	}
}
```

- [ ] **Step 2: `sitemap.go`**

```go
// Package seo writes the machine-readable files: sitemap, robots, llms.txt, security.txt.
package seo

import (
	"bytes"
	"encoding/xml"
	"time"
)

type Alt struct{ Lang, Href string }

type URLEntry struct {
	Loc        string
	LastMod    time.Time
	Alternates []Alt
}

func Sitemap(entries []URLEntry) []byte {
	var b bytes.Buffer
	b.WriteString(xml.Header)
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">` + "\n")
	for _, e := range entries {
		b.WriteString("  <url>\n    <loc>")
		xml.EscapeText(&b, []byte(e.Loc))
		b.WriteString("</loc>\n    <lastmod>" + e.LastMod.Format("2006-01-02") + "</lastmod>\n")
		for _, a := range e.Alternates {
			b.WriteString(`    <xhtml:link rel="alternate" hreflang="` + a.Lang + `" href="`)
			xml.EscapeText(&b, []byte(a.Href))
			b.WriteString(`"/>` + "\n")
		}
		b.WriteString("  </url>\n")
	}
	b.WriteString("</urlset>\n")
	return b.Bytes()
}
```

- [ ] **Step 3: `files.go`**

```go
package seo

import (
	"fmt"
	"strings"
	"time"

	"github.com/raduherinean/herinean.com/internal/config"
	"github.com/raduherinean/herinean.com/internal/content"
)

func Robots(cfg *config.Config) []byte {
	return []byte("User-agent: *\nAllow: /\n\nSitemap: " + cfg.Abs("/sitemap.xml") + "\n")
}

// SecurityTxt per RFC 9116. Expires is derived from the build (commit) time so the file is reproducible; the weekly job warns 30 days before.
func SecurityTxt(cfg *config.Config, build time.Time) []byte {
	exp := build.UTC().Add(364 * 24 * time.Hour).Format("2006-01-02T15:04:05.000Z")
	return []byte(fmt.Sprintf("Contact: mailto:%s\nExpires: %s\nPreferred-Languages: en, ro\nCanonical: %s\n", cfg.Author.Email, exp, cfg.Abs("/.well-known/security.txt")))
}

// LLMs follows llmstxt.org: a title, a blockquote summary, then sections of links with one-line descriptions.
func LLMs(cfg *config.Config, s *content.Site) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n> %s\n\n", cfg.Name, cfg.Tagline["en"])
	fmt.Fprintf(&b, "%s\n\n", cfg.AIDisclosure["en"])
	for _, lang := range content.Langs {
		if len(s.ByLang[lang]) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s (%s)\n\n", s.T(lang, "index.title"), s.T("en", "lang."+lang))
		for _, p := range s.ByLang[lang] {
			fmt.Fprintf(&b, "- [%s](%s): %s\n", p.Title, cfg.Abs(cfg.PieceURL(p.Lang, p.Slug)), p.Summary)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "## Optional\n\n- [Colophon](%s): how the site is built and audited\n- [RSS](%s)\n", cfg.Abs("/colophon/"), cfg.Abs("/feed.xml"))
	return []byte(b.String())
}
```

- [ ] **Step 4: Run, pass, commit** — `M1a: seo — sitemap with hreflang, robots, llms.txt, security.txt` with trailers; push.

---

### Task 9: `edge` — `_headers`

**Files:**
- Create: `internal/edge/headers.go`, `internal/edge/headers_test.go`

**Interfaces:**
- Produces: `edge.Headers(cssHash string) []byte`.

- [ ] **Step 1: Test**

```go
package edge

import (
	"strings"
	"testing"
)

func TestHeaders(t *testing.T) {
	h := string(Headers("sha256-abc"))
	for _, want := range []string{
		"/*\n  Content-Security-Policy: default-src 'none'; style-src 'sha256-abc'; img-src 'self'; font-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'",
		"Strict-Transport-Security: max-age=63072000; includeSubDomains; preload",
		"Cross-Origin-Resource-Policy: same-origin",
		"/img/*\n  ! Cross-Origin-Resource-Policy\n  ! Cache-Control\n  Cross-Origin-Resource-Policy: cross-origin\n  Cache-Control: public, max-age=31536000, immutable",
		"/og/*\n  ! Cross-Origin-Resource-Policy\n  ! Cache-Control\n  Cross-Origin-Resource-Policy: cross-origin",
		"/feed.xml\n  ! Cache-Control\n  Content-Type: application/rss+xml; charset=utf-8\n  Cache-Control: public, max-age=300",
		"/feed.json\n  Content-Type: application/feed+json; charset=utf-8",
		"/.well-known/security.txt\n  Content-Type: text/plain; charset=utf-8",
	} {
		if !strings.Contains(h, want) {
			t.Errorf("missing:\n%s\nin:\n%s", want, h)
		}
	}
	if strings.Contains(strings.ToLower(h), "x-robots-tag") {
		t.Error("_headers must never carry X-Robots-Tag: production is indexable; the Worker marks previews")
	}
	if strings.Contains(h, "interest-cohort") {
		t.Error("interest-cohort is a dead feature name; Chrome logs a console message for it")
	}
}
```

- [ ] **Step 2: `headers.go`**

```go
// Package edge generates the Cloudflare _headers file. The CSS hash comes from the exact bytes inlined by render.
// Cloudflare applies every matching rule; a header set by /* is not replaced by a more specific rule unless that rule
// first detaches it with "! Name". Every override below does that, so /img/* really is immutable and cross-origin.
package edge

import "fmt"

func Headers(cssHash string) []byte {
	return []byte(fmt.Sprintf(`/*
  Content-Security-Policy: default-src 'none'; style-src '%s'; img-src 'self'; font-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'
  Strict-Transport-Security: max-age=63072000; includeSubDomains; preload
  X-Content-Type-Options: nosniff
  Referrer-Policy: strict-origin-when-cross-origin
  Permissions-Policy: accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()
  Cross-Origin-Opener-Policy: same-origin
  Cross-Origin-Resource-Policy: same-origin
  X-Frame-Options: DENY
  Cache-Control: public, max-age=0, must-revalidate

/img/*
  ! Cross-Origin-Resource-Policy
  ! Cache-Control
  Cross-Origin-Resource-Policy: cross-origin
  Cache-Control: public, max-age=31536000, immutable

/og/*
  ! Cross-Origin-Resource-Policy
  ! Cache-Control
  Cross-Origin-Resource-Policy: cross-origin
  Cache-Control: public, max-age=31536000, immutable

/fonts/*
  ! Cache-Control
  Cache-Control: public, max-age=31536000, immutable

/feed.xml
  ! Cache-Control
  Content-Type: application/rss+xml; charset=utf-8
  Cache-Control: public, max-age=300

/feed.en.xml
  ! Cache-Control
  Content-Type: application/rss+xml; charset=utf-8
  Cache-Control: public, max-age=300

/feed.ro.xml
  ! Cache-Control
  Content-Type: application/rss+xml; charset=utf-8
  Cache-Control: public, max-age=300

/feed.json
  ! Cache-Control
  Content-Type: application/feed+json; charset=utf-8
  Cache-Control: public, max-age=300

/sitemap.xml
  Content-Type: application/xml; charset=utf-8

/llms.txt
  Content-Type: text/plain; charset=utf-8

/.well-known/security.txt
  Content-Type: text/plain; charset=utf-8

/.well-known/mta-sts.txt
  Content-Type: text/plain; charset=utf-8
`, cssHash))
}
```

- [ ] **Step 3: Run, pass, commit** — `M1a: edge — _headers with the CSS hash, per-path CORP and caching` with trailers; push.

To verify at the first M1b deploy (add to `scripts/verify-edge.sh` then): `/img/<any>` returns exactly one `cache-control` (`immutable`) and `cross-origin-resource-policy: cross-origin`; `/feed.xml` returns `content-type: application/rss+xml; charset=utf-8`. If Cloudflare ignores a `Content-Type` override from `_headers`, feeds must be routed through the Worker to fix it: remove `"!/feed.*"` from `run_worker_first` in `wrangler.toml` and set the header there for `/feed.*` paths (M1b's Worker; feeds otherwise never reach it).

---

### Task 10: `site` — build orchestration, reproducibility, `check`, `check --dist`

**Files:**
- Create: `internal/site/clock.go`, `internal/site/build.go`, `internal/site/pages.go`, `internal/site/check.go`, `internal/site/checkdist.go`, `internal/site/build_test.go`
- Modify: `internal/site/site.go` (keep `Options` and `ErrAuthorInputs`; remove the stubs that are now implemented)
- Create: `testdata/site/assets/portrait.jpg` (any 800×800 JPEG), `testdata/site/static/favicon.svg`, `testdata/site/static/.well-known/mta-sts.txt`

**Interfaces:**
- Produces: `site.BuildTime(root) (time.Time, error)`; `site.Commit(root) string`; `site.fileTime(root, rel string, fallback time.Time) time.Time` (last commit touching a file; sitemap lastmod for pages); `site.Build(o Options) error`; `site.Check(o) error`; `site.CheckDist(o) error`; internal `type build struct{…}` with `pages() error`; package-private `deps(root) string`, `goVersion() string`.

- [ ] **Step 1: `clock.go`**

```go
package site

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/raduherinean/herinean.com/internal/content"
)

// BuildTime is the commit time: SOURCE_DATE_EPOCH if set, else the HEAD commit's timestamp. Wall-clock never enters the output.
func BuildTime(root string) (time.Time, error) {
	if v := os.Getenv("SOURCE_DATE_EPOCH"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("SOURCE_DATE_EPOCH: %w", err)
		}
		return time.Unix(n, 0).In(content.Bucharest), nil
	}
	out, err := exec.Command("git", "-C", root, "log", "-1", "--format=%ct").Output()
	if err != nil {
		return time.Time{}, fmt.Errorf("build time: set SOURCE_DATE_EPOCH or build inside a git checkout (%v)", err)
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(n, 0).In(content.Bucharest), nil
}

func Commit(root string) string {
	out, err := exec.Command("git", "-C", root, "rev-parse", "--short=10", "HEAD").Output()
	if err != nil {
		return "uncommitted"
	}
	return strings.TrimSpace(string(out))
}

// fileTime is the last commit that touched rel, so a page's sitemap lastmod only moves when the page does. Outside git: fallback.
func fileTime(root, rel string, fallback time.Time) time.Time {
	out, err := exec.Command("git", "-C", root, "log", "-1", "--format=%ct", "--", rel).Output()
	if err != nil {
		return fallback
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil || n == 0 {
		return fallback
	}
	return time.Unix(n, 0).In(content.Bucharest)
}
```

- [ ] **Step 2: `build.go`** — the pipeline. Every output goes into `files map[string][]byte`, written sorted into a temp dir that replaces `dist/` atomically.

```go
package site

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/raduherinean/herinean.com/internal/config"
	"github.com/raduherinean/herinean.com/internal/content"
	"github.com/raduherinean/herinean.com/internal/edge"
	"github.com/raduherinean/herinean.com/internal/feeds"
	"github.com/raduherinean/herinean.com/internal/images"
	"github.com/raduherinean/herinean.com/internal/render"
	"github.com/raduherinean/herinean.com/internal/seo"
)

type build struct {
	o           Options
	cfg         *config.Config
	site        *content.Site
	now         time.Time
	commit      string
	r           *render.Renderer
	fonts       *images.Fonts
	cache       *images.Cache
	imgs        map[string]map[string]*content.ImageInfo // key → dest → info
	og          map[string]string                        // "lang-slug" or "home-lang" → /og/… path
	files       map[string][]byte                        // dist-relative path → bytes
	portrait    *content.ImageInfo
	portraitRaw []byte // source bytes, for the home OG card
}

func load(o Options) (*build, error) {
	now, err := BuildTime(o.Root)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(filepath.Join(o.Root, "site.yaml"))
	if err != nil {
		if errors.Is(err, config.ErrPlaceholder) {
			return nil, fmt.Errorf("%w: %v", ErrAuthorInputs, err)
		}
		return nil, err
	}
	if pp := checkPlaceholders(o.Root); len(pp) > 0 {
		return nil, fmt.Errorf("%w:\n%v", ErrAuthorInputs, pp.Err())
	}
	s, probs := content.Load(o.Root, now)
	if err := probs.Err(); err != nil {
		return nil, err
	}
	return &build{o: o, cfg: cfg, site: s, now: now, commit: Commit(o.Root), cache: &images.Cache{Dir: filepath.Join(o.Root, ".cache")},
		files: map[string][]byte{}, imgs: map[string]map[string]*content.ImageInfo{}, og: map[string]string{}}, nil
}

func Build(o Options) error {
	b, err := load(o)
	if err != nil {
		return err
	}
	if err := b.images(); err != nil {
		return err
	}
	if err := b.site.Render(b.lookup); err != nil {
		return err
	}
	b.r, err = render.New(filepath.Join(o.Root, "templates"), filepath.Join(o.Root, "assets", "css", "site.css"))
	if err != nil {
		return err
	}
	if err := b.ogImages(); err != nil {
		return err
	}
	if err := b.pages(); err != nil {
		return err
	}
	if err := b.machineFiles(); err != nil {
		return err
	}
	if err := b.static(); err != nil {
		return err
	}
	return b.write()
}

func (b *build) lookup(key, dest string) *content.ImageInfo {
	if m := b.imgs[key]; m != nil {
		return m[dest]
	}
	return nil
}

func (b *build) images() error {
	var probs content.Problems
	for key, refs := range b.site.AllImageRefs() {
		for _, ref := range refs {
			if b.imgs[key] != nil && b.imgs[key][ref.Dest] != nil {
				continue
			}
			src := filepath.Join(b.o.Root, "assets", "img", key, ref.Dest)
			info, err := images.Process(src, key, ref.Dest, images.Options{Cache: b.cache})
			if err != nil {
				probs.Add(src, ref.Line, "%v", err)
				continue
			}
			if b.imgs[key] == nil {
				b.imgs[key] = map[string]*content.ImageInfo{}
			}
			b.imgs[key][ref.Dest] = &content.ImageInfo{Src: info.Src, Srcset: info.Srcset, Width: info.Width, Height: info.Height, IsSVG: info.IsSVG}
			for p, data := range info.Files {
				b.files[p] = data
			}
		}
	}
	portrait := filepath.Join(b.o.Root, "assets", "portrait.jpg")
	b.portraitRaw, _ = os.ReadFile(portrait)
	// Displayed at ≤ 10rem (160 CSS px): 320 is 2×, 480 covers 3× phones. The default 720/1440 would ship a photo for a thumbnail
	// and blow row 14's 150 KB first view on the home page.
	info, err := images.Process(portrait, "home", "portrait.jpg", images.Options{Widths: []int{320, 480}, Cache: b.cache})
	if err != nil {
		probs.Add("assets/portrait.jpg", 0, "%v (the home page needs a portrait)", err)
	} else {
		b.portrait = &content.ImageInfo{Src: info.Src, Srcset: info.Srcset, Width: info.Width, Height: info.Height}
		for p, data := range info.Files {
			b.files[p] = data
		}
	}
	return probs.Err()
}

func (b *build) ogImages() error {
	var err error
	b.fonts, err = images.LoadFonts(filepath.Join(b.o.Root, "assets", "fonts", "og"))
	if err != nil {
		return err
	}
	put := func(id string, og images.OG) error {
		png, err := images.RenderOG(og, b.fonts, b.cache)
		if err != nil {
			return err
		}
		p := "og/" + id + "." + images.Hash8(png) + ".png"
		b.files[p] = png
		b.og[id] = "/" + p
		return nil
	}
	for _, p := range b.site.Pieces {
		if err := put(p.Lang+"-"+p.Slug, images.OG{Title: p.Title, Name: b.cfg.Name, Domain: strings.TrimPrefix(b.cfg.BaseURL, "https://"),
			Pillar: b.site.T(p.Lang, "pillar."+p.Pillar), Lang: strings.ToUpper(p.Lang)}); err != nil {
			return err
		}
	}
	for _, lang := range content.Langs {
		if err := put("home-"+lang, images.OG{Title: b.cfg.Tagline[lang], Name: b.cfg.Name, Domain: strings.TrimPrefix(b.cfg.BaseURL, "https://"), Lang: strings.ToUpper(lang), Portrait: b.portraitRaw}); err != nil {
			return err
		}
	}
	return nil
}

func (b *build) machineFiles() error {
	now := b.now
	for _, lang := range []string{"", "en", "ro"} {
		x, err := feeds.RSS(b.cfg, b.site, lang, now)
		if err != nil {
			return err
		}
		b.files[strings.TrimPrefix(b.cfg.FeedURL(lang), "/")] = x
	}
	j, err := feeds.JSON(b.cfg, b.site, now)
	if err != nil {
		return err
	}
	b.files["feed.json"] = j
	b.files["sitemap.xml"] = seo.Sitemap(b.sitemapEntries())
	b.files["robots.txt"] = seo.Robots(b.cfg)
	b.files["llms.txt"] = seo.LLMs(b.cfg, b.site)
	b.files[".well-known/security.txt"] = seo.SecurityTxt(b.cfg, now)
	b.files["_headers"] = edge.Headers(b.r.CSSHash())
	return nil
}

func (b *build) static() error {
	root := filepath.Join(b.o.Root, "static")
	return filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		b.files[filepath.ToSlash(rel)] = data
		return nil
	})
}

// deps lists the module graph for the colophon: what the site is built from, straight from go.mod.
func deps(root string) string {
	out, err := exec.Command("go", "list", "-m", "all").Output()
	if err != nil {
		return "module list unavailable"
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	return strings.Join(lines[1:], " · ") // first line is this module
}

func goVersion() string { return runtime.Version() }

// write puts every file into <out>.tmp (sorted, mtimes = build time), then swaps it into place.
func (b *build) write() error {
	out := filepath.Join(b.o.Root, b.o.Out)
	tmp := out + ".tmp"
	_ = os.RemoveAll(tmp)
	paths := make([]string, 0, len(b.files))
	for p := range b.files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		full := filepath.Join(tmp, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, b.files[p], 0o644); err != nil {
			return err
		}
		if err := os.Chtimes(full, b.now, b.now); err != nil {
			return err
		}
	}
	_ = os.RemoveAll(out)
	if err := os.Rename(tmp, out); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "site: %d files → %s (commit %s, %s, %s)\n", len(paths), b.o.Out, b.commit, b.now.Format("2006-01-02"), runtime.Version())
	return nil
}
```

- [ ] **Step 3: `pages.go`** — one place that knows every page's metadata; also feeds the sitemap and the link checker.

```go
package site

import (
	"html/template"
	"strings"

	"github.com/raduherinean/herinean.com/internal/content"
	"github.com/raduherinean/herinean.com/internal/render"
	"github.com/raduherinean/herinean.com/internal/seo"
)

var locales = map[string]string{"en": "en_US", "ro": "ro_RO"}

func (b *build) base(lang, kind, path, title, description string) *render.PageData {
	return &render.PageData{Cfg: b.cfg, Site: b.site, Lang: lang, Kind: kind, Title: title, HeadTitle: title + " — " + b.cfg.Name,
		Description: description, Path: path, Canonical: b.cfg.Abs(path), OGType: "website", OGLocale: locales[lang], OGImage: b.cfg.Abs(b.og["home-"+lang])}
}

func (b *build) put(path string, html []byte) {
	p := strings.TrimPrefix(path, "/")
	if strings.HasSuffix(p, "/") || p == "" {
		p += "index.html"
	}
	b.files[p] = html
}

// pages renders every HTML page. The set of paths here is the site's URL space; sitemapEntries and link checking derive from it.
func (b *build) pages() error {
	cfg, s := b.cfg, b.site
	for _, lang := range content.Langs {
		// home
		home := s.Pages["home."+lang]
		d := b.base(lang, "home", cfg.HomeURL(lang), cfg.Name, cfg.Tagline[lang])
		d.HeadTitle = cfg.Name
		d.Page, d.Body, d.Tagline, d.Portrait = home, home.Body, cfg.Tagline[lang], b.portrait
		for _, p := range s.Latest(5) {
			d.Entries = append(d.Entries, render.EntryFor(cfg, s, lang, p))
		}
		d.Alternates = render.AlternatesFor(cfg, cfg.HomeURL("en"), cfg.HomeURL("ro"))
		d.JSONLD = render.WebSiteLD(cfg, lang, cfg.Tagline[lang])
		if err := b.render("home", d); err != nil {
			return err
		}
		// index
		d = b.base(lang, "index", cfg.IndexURL(lang), s.T(lang, "index.title"), cfg.Tagline[lang])
		d.Years = render.Years(cfg, s, lang, s.Pieces)
		d.Alternates = render.AlternatesFor(cfg, cfg.IndexURL("en"), cfg.IndexURL("ro"))
		d.JSONLD = render.WebSiteLD(cfg, lang, cfg.Tagline[lang])
		if err := b.render("index", d); err != nil {
			return err
		}
		// privacy
		pv := s.Pages["privacy."+lang]
		d = b.base(lang, "privacy", cfg.PrivacyURL(lang), pv.Title, pv.Summary)
		d.Page, d.Body = pv, pv.Body
		d.Alternates = render.AlternatesFor(cfg, cfg.PrivacyURL("en"), cfg.PrivacyURL("ro"))
		d.JSONLD = render.WebSiteLD(cfg, lang, "")
		if err := b.render("privacy", d); err != nil {
			return err
		}
	}
	// pieces
	for _, p := range s.Pieces {
		path := cfg.PieceURL(p.Lang, p.Slug)
		d := b.base(p.Lang, "piece", path, p.Title, p.Summary)
		d.Piece, d.Body, d.OGType, d.OGImage = p, p.Body, "article", cfg.Abs(b.og[p.Lang+"-"+p.Slug])
		d.DateISO, d.DateText = p.Date.Format("2006-01-02"), render.DateText(s, p.Lang, p.Date)
		if p.Updated != nil {
			d.UpdatedISO, d.UpdatedText = p.Updated.Format("2006-01-02"), render.DateText(s, p.Lang, *p.Updated)
		}
		d.PillarText = s.T(p.Lang, "pillar."+p.Pillar)
		en, ro := "", ""
		if p.Lang == "en" {
			en = path
		} else {
			ro = path
		}
		if p.Translation != nil {
			e := render.EntryFor(cfg, s, p.Lang, p.Translation)
			d.Translation = &e
			if p.Lang == "en" {
				ro = e.URL
			} else {
				en = e.URL
			}
		}
		d.Alternates = render.AlternatesFor(cfg, en, ro)
		d.JSONLD = render.ArticleLD(cfg, p, d.OGImage, s.T(p.Lang, "index.title"))
		if err := b.render("piece", d); err != nil {
			return err
		}
	}
	// colophon (EN only)
	co := s.Pages["colophon.en"]
	d := b.base("en", "colophon", cfg.ColophonURL(), co.Title, co.Summary)
	d.Page, d.Body = co, co.Body
	d.Alternates = render.AlternatesFor(cfg, cfg.ColophonURL(), "")
	d.JSONLD = render.WebSiteLD(cfg, "en", "")
	frag, err := b.scorecardFragment()
	if err != nil {
		return err
	}
	d.ScorecardHTML = template.HTML(frag)
	d.Deps, d.Commit, d.BuildDate, d.GoVersion = deps(b.o.Root), b.commit, b.now.Format("2006-01-02"), goVersion()
	if err := b.render("colophon", d); err != nil {
		return err
	}
	// 404
	nf := b.base("en", "404", "/404.html", s.T("en", "notfound.title"), s.T("en", "notfound.body"))
	nf.HeadTitle, nf.Robots, nf.JSONLD = s.T("en", "notfound.title"), "noindex", render.WebSiteLD(cfg, "en", "")
	return b.render("404", nf)
}

func (b *build) render(kind string, d *render.PageData) error {
	out, err := b.r.Render(kind, d)
	if err != nil {
		return err
	}
	b.put(d.Path, out)
	return nil
}

func (b *build) sitemapEntries() []seo.URLEntry {
	cfg, s := b.cfg, b.site
	latest := b.now
	if len(s.Pieces) > 0 {
		latest = s.Pieces[0].Date
	}
	alts := func(en, ro string) []seo.Alt {
		var out []seo.Alt
		for _, a := range render.AlternatesFor(cfg, en, ro) {
			out = append(out, seo.Alt{Lang: a.Lang, Href: a.Href})
		}
		return out
	}
	var es []seo.URLEntry
	for _, lang := range content.Langs {
		es = append(es, seo.URLEntry{Loc: cfg.Abs(cfg.HomeURL(lang)), LastMod: latest, Alternates: alts(cfg.HomeURL("en"), cfg.HomeURL("ro"))})
		es = append(es, seo.URLEntry{Loc: cfg.Abs(cfg.IndexURL(lang)), LastMod: latest, Alternates: alts(cfg.IndexURL("en"), cfg.IndexURL("ro"))})
		es = append(es, seo.URLEntry{Loc: cfg.Abs(cfg.PrivacyURL(lang)), LastMod: fileTime(b.o.Root, "content/"+lang+"/_privacy.md", b.now), Alternates: alts(cfg.PrivacyURL("en"), cfg.PrivacyURL("ro"))})
	}
	for _, p := range s.Pieces {
		mod := p.Date
		if p.Updated != nil {
			mod = *p.Updated
		}
		en, ro := "", ""
		if p.Lang == "en" {
			en = cfg.PieceURL("en", p.Slug)
		} else {
			ro = cfg.PieceURL("ro", p.Slug)
		}
		if p.Translation != nil {
			if p.Lang == "en" {
				ro = cfg.PieceURL("ro", p.Translation.Slug)
			} else {
				en = cfg.PieceURL("en", p.Translation.Slug)
			}
		}
		es = append(es, seo.URLEntry{Loc: cfg.Abs(cfg.PieceURL(p.Lang, p.Slug)), LastMod: mod, Alternates: alts(en, ro)})
	}
	es = append(es, seo.URLEntry{Loc: cfg.Abs(cfg.ColophonURL()), LastMod: fileTime(b.o.Root, "content/en/_colophon.md", b.now), Alternates: alts(cfg.ColophonURL(), "")})
	return es
}
```
`scorecardFragment()` is Task 11 (returns the rendered fragment from `scorecard.json` + manual rows, or a single "not yet verified" row when `scorecard.json` is absent).

- [ ] **Step 4: `check.go`** — source validation without writing (`site check`), reusing `load` and adding the cross-file rules.

```go
package site

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/raduherinean/herinean.com/internal/content"
	"github.com/raduherinean/herinean.com/internal/images"
)

// checkPlaceholders fails on author-input markers left anywhere in content/ or site.yaml.
func checkPlaceholders(root string) content.Problems {
	var probs content.Problems
	_ = filepath.WalkDir(filepath.Join(root, "content"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") || strings.HasSuffix(p, "_template.md") {
			return nil
		}
		b, _ := os.ReadFile(p)
		for i, line := range strings.Split(string(b), "\n") {
			if strings.ContainsAny(line, "⟨⟩") {
				probs.Add(p, i+1, "author placeholder ⟨…⟩ still present")
			}
		}
		return nil
	})
	return probs
}

func Check(o Options) error {
	b, err := load(o)
	if err != nil {
		return err
	}
	var probs content.Problems
	// images exist and are processable
	for key, refs := range b.site.AllImageRefs() {
		for _, ref := range refs {
			src := filepath.Join(o.Root, "assets", "img", key, ref.Dest)
			if _, err := os.Stat(src); err != nil {
				probs.Add(src, ref.Line, "referenced image is missing")
			}
		}
	}
	if _, err := os.Stat(filepath.Join(o.Root, "assets", "portrait.jpg")); err != nil {
		probs.Add("assets/portrait.jpg", 0, "the home page needs a portrait")
	}
	// fonts carry the Romanian glyphs
	for _, f := range images.OGFontFiles {
		if err := images.CheckGlyphs(filepath.Join(o.Root, "assets", "fonts", "og", f)); err != nil {
			probs.Add(filepath.Join("assets", "fonts", "og", f), 0, "%v", err)
		}
	}
	// internal links resolve to pages or files the build will produce
	known := b.knownPaths()
	for _, p := range b.site.Pieces {
		for _, l := range p.Links {
			if !known[l.Dest] {
				probs.Add(p.File, l.Line, "internal link %s does not resolve", l.Dest)
			}
		}
	}
	for _, pg := range b.site.Pages {
		for _, l := range pg.Links() {
			if !known[l.Dest] {
				probs.Add(pg.File, l.Line, "internal link %s does not resolve", l.Dest)
			}
		}
	}
	return probs.Err()
}

// knownPaths is every URL path the build emits (pages with trailing slash, plus machine files).
func (b *build) knownPaths() map[string]bool {
	cfg, s := b.cfg, b.site
	k := map[string]bool{"/feed.xml": true, "/feed.json": true, "/sitemap.xml": true, "/robots.txt": true, "/llms.txt": true, "/colophon/": true, "/404.html": true, "/.well-known/security.txt": true}
	for _, lang := range content.Langs {
		k[cfg.HomeURL(lang)], k[cfg.IndexURL(lang)], k[cfg.PrivacyURL(lang)], k[cfg.FeedURL(lang)] = true, true, true, true
	}
	for _, p := range s.Pieces {
		k[cfg.PieceURL(p.Lang, p.Slug)] = true
	}
	return k
}
```

- [ ] **Step 5: `checkdist.go`** — the post-build invariants (`site check --dist`). These are the rows of the scorecard the binary can prove by itself.

```go
package site

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/raduherinean/herinean.com/internal/config"
	"github.com/raduherinean/herinean.com/internal/content"
)

const htmlBudget = 30 * 1024 // gzip bytes, HTML with inlined CSS

var (
	reScript   = regexp.MustCompile(`(?is)<script\b([^>]*)>`)
	reStyleAtt = regexp.MustCompile(`(?i)\sstyle="`)
	reStyleTag = regexp.MustCompile(`(?is)<style>(.*?)</style>`)
	reOGImage  = regexp.MustCompile(`property="og:image" content="([^"]+)"`)
	reHash     = regexp.MustCompile(`style-src '(sha256-[^']+)'`)
	rePre      = regexp.MustCompile(`<pre\b[^>]*>`)
)

func CheckDist(o Options) error {
	dist := filepath.Join(o.Root, o.Out)
	cfg, err := config.Load(filepath.Join(o.Root, "site.yaml"))
	if err != nil {
		return err
	}
	var probs content.Problems
	headers, err := os.ReadFile(filepath.Join(dist, "_headers"))
	if err != nil {
		return fmt.Errorf("dist: %w (run site build first)", err)
	}
	if strings.Contains(strings.ToLower(string(headers)), "x-robots-tag") {
		probs.Add("_headers", 0, "must not set X-Robots-Tag on production")
	}
	hm := reHash.FindSubmatch(headers)
	if hm == nil {
		probs.Add("_headers", 0, "no style-src hash in the CSP")
	}
	var htmlPaths []string
	_ = filepath.WalkDir(dist, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".html") {
			return nil
		}
		rel, _ := filepath.Rel(dist, p)
		htmlPaths = append(htmlPaths, "/"+filepath.ToSlash(rel))
		b, _ := os.ReadFile(p)
		s := string(b)
		for _, m := range reScript.FindAllStringSubmatch(s, -1) {
			if !strings.Contains(m[1], `type="application/ld+json"`) {
				probs.Add(rel, 0, "executable <script> found")
			}
		}
		if reStyleAtt.MatchString(s) {
			probs.Add(rel, 0, "style= attribute found (hash CSP forbids it)")
		}
		for _, pre := range rePre.FindAllString(s, -1) {
			if !strings.Contains(pre, `tabindex="0"`) {
				probs.Add(rel, 0, "%s lacks tabindex=\"0\": a scrolling block must be keyboard-focusable", pre)
			}
		}
		for _, must := range []string{`rel="canonical"`, `property="og:image"`, `name="twitter:card"`, `application/ld+json`, `rel="alternate" type="application/rss+xml"`, `<html lang="`} {
			if !strings.Contains(s, must) {
				probs.Add(rel, 0, "missing %s", must)
			}
		}
		if st := reStyleTag.FindStringSubmatch(s); st != nil && hm != nil {
			sum := sha256.Sum256([]byte(st[1]))
			if got := "sha256-" + base64.StdEncoding.EncodeToString(sum[:]); got != string(hm[1]) {
				probs.Add(rel, 0, "inlined CSS hash %s does not match _headers %s", got, hm[1])
			}
		}
		if og := reOGImage.FindStringSubmatch(s); og != nil {
			u := strings.TrimPrefix(og[1], cfg.BaseURL)
			ob, err := os.ReadFile(filepath.Join(dist, filepath.FromSlash(u)))
			if err != nil {
				probs.Add(rel, 0, "og:image %s not in dist", u)
			} else if img, err := png.Decode(bytes.NewReader(ob)); err != nil || img.Bounds().Dx() != 1200 || img.Bounds().Dy() != 630 || len(ob) > 200*1024 {
				probs.Add(rel, 0, "og:image %s must be a 1200×630 PNG under 200 KB", u)
			}
		}
		var gz bytes.Buffer
		w := gzip.NewWriter(&gz)
		_, _ = w.Write(b)
		_ = w.Close()
		if gz.Len() > htmlBudget {
			probs.Add(rel, 0, "page is %d bytes gzipped; budget %d (row 14 — every page, the colophon included)", gz.Len(), htmlBudget)
		}
		return nil
	})
	// feeds and sitemap parse; sitemap covers every page
	for _, f := range []string{"feed.xml", "feed.en.xml", "feed.ro.xml", "sitemap.xml"} {
		b, err := os.ReadFile(filepath.Join(dist, f))
		if err != nil {
			probs.Add(f, 0, "missing")
			continue
		}
		var v struct{}
		if err := xml.Unmarshal(b, &v); err != nil {
			probs.Add(f, 0, "invalid XML: %v", err)
		}
	}
	sm, _ := os.ReadFile(filepath.Join(dist, "sitemap.xml"))
	for _, p := range htmlPaths {
		u := strings.TrimSuffix(p, "index.html")
		if p == "/404.html" {
			continue
		}
		if !strings.Contains(string(sm), "<loc>"+cfg.Abs(u)+"</loc>") {
			probs.Add("sitemap.xml", 0, "does not list %s", u)
		}
	}
	for _, f := range []string{"robots.txt", "llms.txt", ".well-known/security.txt", ".well-known/mta-sts.txt", "favicon.svg", "feed.json"} {
		if _, err := os.Stat(filepath.Join(dist, f)); err != nil {
			probs.Add(f, 0, "missing from dist")
		}
	}
	return probs.Err()
}
```

- [ ] **Step 6: Build test on the fixture** — `internal/site/build_test.go`:

```go
package site

import (
	"os"
	"path/filepath"
	"testing"
)

func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	_ = filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		b, _ := os.ReadFile(p)
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
}

// fixtureRoot assembles a buildable root: the fixture site plus the real templates, CSS and fonts.
func fixtureRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyDir(t, "../../testdata/site", root)
	copyDir(t, "../../templates", filepath.Join(root, "templates"))
	copyDir(t, "../../assets/css", filepath.Join(root, "assets", "css"))
	copyDir(t, "../../assets/fonts", filepath.Join(root, "assets", "fonts"))
	t.Setenv("SOURCE_DATE_EPOCH", "1790000000") // 2026-09-21T01:33:20Z
	return root
}

func TestBuildIsDeterministicAndPassesDistChecks(t *testing.T) {
	root := fixtureRoot(t)
	o := Options{Root: root, Out: "dist"}
	if err := Check(o); err != nil {
		t.Fatal(err)
	}
	if err := Build(o); err != nil {
		t.Fatal(err)
	}
	if err := CheckDist(o); err != nil {
		t.Fatal(err)
	}
	first := snapshot(t, filepath.Join(root, "dist"))
	if err := Build(o); err != nil {
		t.Fatal(err)
	}
	second := snapshot(t, filepath.Join(root, "dist"))
	if len(first) == 0 || len(first) != len(second) {
		t.Fatalf("files %d vs %d", len(first), len(second))
	}
	for p, h := range first {
		if second[p] != h {
			t.Errorf("%s differs between two builds of the same input", p)
		}
	}
	for _, must := range []string{"index.html", "ro/index.html", "writing/index.html", "ro/articole/index.html", "writing/paired/index.html", "ro/articole/pereche/index.html", "colophon/index.html", "privacy/index.html", "ro/confidentialitate/index.html", "404.html", "feed.xml", "feed.en.xml", "feed.ro.xml", "feed.json", "sitemap.xml", "_headers", ".well-known/security.txt"} {
		if _, ok := first[must]; !ok {
			t.Errorf("missing %s", must)
		}
	}
}

func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, _ := os.ReadFile(p)
		rel, _ := filepath.Rel(dir, p)
		out[filepath.ToSlash(rel)] = fmtHash(b)
		return nil
	})
	return out
}
```
with `fmtHash` = `images.Hash8` re-exported locally (`func fmtHash(b []byte) string { return images.Hash8(b) }`, import `internal/images`).

- [ ] **Step 7: Run, fix, pass** — `go test ./... 2>&1 | tail -15`; then `go run ./cmd/site check` on the real repo is expected to exit **3** on the ⟨placeholder⟩ rule until Task 12 supplies content — that is the rule working.

- [ ] **Step 8: Commit** — `M1a: site — build pipeline, reproducible output, check and check --dist` with trailers; push.

---

### Task 11: `site serve`, `site new`, `site scorecard`

**Files:**
- Create: `internal/site/serve.go`, `internal/site/new.go`, `internal/site/scorecard.go`, `internal/site/scorecard_test.go`, `content/_template.md`, `data/scorecard-manual.yaml`

- [ ] **Step 1: `serve.go`** — a local server that applies `_headers`, redirects to the trailing slash, serves 404.html with a 404, and rebuilds when inputs changed.

```go
package site

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type headerRule struct {
	pattern string
	headers [][2]string
}

func parseHeaders(b []byte) []headerRule {
	var rules []headerRule
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") {
			rules = append(rules, headerRule{pattern: strings.TrimSpace(line)})
			continue
		}
		if len(rules) == 0 {
			continue
		}
		l := strings.TrimSpace(line)
		if name, ok := strings.CutPrefix(l, "! "); ok { // Cloudflare's detach syntax: drop a header an earlier rule set
			rules[len(rules)-1].headers = append(rules[len(rules)-1].headers, [2]string{"!", name})
			continue
		}
		k, v, ok := strings.Cut(l, ": ")
		if ok {
			rules[len(rules)-1].headers = append(rules[len(rules)-1].headers, [2]string{k, v})
		}
	}
	return rules
}

func (r headerRule) match(path string) bool {
	if strings.HasSuffix(r.pattern, "*") {
		return strings.HasPrefix(path, strings.TrimSuffix(r.pattern, "*"))
	}
	return r.pattern == path
}

func newestInput(root string) time.Time {
	var newest time.Time
	for _, d := range []string{"content", "assets", "templates", "i18n", "static", "site.yaml", "data"} {
		_ = filepath.WalkDir(filepath.Join(root, d), func(p string, e os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if info, err := e.Info(); err == nil && info.ModTime().After(newest) {
				newest = info.ModTime()
			}
			return nil
		})
	}
	return newest
}

func Serve(o Options, host string, port int) error {
	dist := filepath.Join(o.Root, o.Out)
	var mu sync.Mutex
	var built time.Time
	rebuild := func() error {
		mu.Lock()
		defer mu.Unlock()
		if n := newestInput(o.Root); !built.IsZero() && !n.After(built) {
			return nil
		}
		if err := Build(o); err != nil {
			return err
		}
		built = time.Now()
		return nil
	}
	if err := rebuild(); err != nil {
		fmt.Fprintln(os.Stderr, "site serve: build failed:", err)
	}
	h := func(w http.ResponseWriter, r *http.Request) {
		if err := rebuild(); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		hb, _ := os.ReadFile(filepath.Join(dist, "_headers"))
		rules := parseHeaders(hb)
		p := r.URL.Path
		fp := filepath.Join(dist, filepath.FromSlash(p))
		if st, err := os.Stat(fp); err == nil && st.IsDir() {
			if !strings.HasSuffix(p, "/") {
				http.Redirect(w, r, p+"/", http.StatusMovedPermanently)
				return
			}
			fp = filepath.Join(fp, "index.html")
		}
		for _, rule := range rules {
			if rule.match(p) {
				for _, kv := range rule.headers {
					if kv[0] == "!" {
						w.Header().Del(kv[1])
						continue
					}
					w.Header().Set(kv[0], kv[1])
				}
			}
		}
		b, err := os.ReadFile(fp)
		if err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(404)
			nf, _ := os.ReadFile(filepath.Join(dist, "404.html"))
			_, _ = w.Write(nf)
			return
		}
		if strings.HasSuffix(fp, ".html") {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		}
		http.ServeContent(w, r, fp, built, strings.NewReader(string(b)))
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	fmt.Fprintf(os.Stderr, "site serve: http://%s/ (rebuilds when inputs change)\n", addr)
	return http.ListenAndServe(addr, http.HandlerFunc(h))
}
```
(`http.ServeContent` sets Content-Type from the extension for non-HTML files; `strings.NewReader` satisfies `io.ReadSeeker`.)

- [ ] **Step 2: `content/_template.md`** — the editorial template (`site new` copies it; `check` ignores it).

```markdown
---
title: ""
date:
key: KEY
pillar:
summary: ""
---

## Situation

What was true when the decision came up, in two paragraphs an executive can read cold.

## Trade-off

The two or three options that were actually on the table, and what each one cost.

## Decision

What was decided, by whom, and the number or date that made it concrete.

## What I'd change

The thing that looks different a quarter later.

<!-- No embeds of any kind (video, X, LinkedIn, Gists, Mermaid): link out, or use a static SVG/PNG in assets/img/KEY/. Every image needs alt text. Romanian uses ș ț with comma below. -->
```

- [ ] **Step 3: `new.go`**

```go
package site

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/raduherinean/herinean.com/internal/content"
)

func New(o Options, lang, slug string) error {
	if lang != "en" && lang != "ro" {
		return fmt.Errorf("lang must be en or ro")
	}
	if !content.ValidSlug(slug) {
		return fmt.Errorf("slug %q must match [a-z0-9]+(-[a-z0-9]+)*", slug)
	}
	dst := filepath.Join(o.Root, "content", lang, slug+".md")
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("%s already exists", dst)
	}
	tpl, err := os.ReadFile(filepath.Join(o.Root, "content", "_template.md"))
	if err != nil {
		return err
	}
	out := strings.ReplaceAll(string(tpl), "KEY", slug)
	if err := os.WriteFile(dst, []byte(out), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "site new: %s (branch piece/%s, fill title/pillar/summary; date is stamped by /publish-piece)\n", dst, slug)
	return nil
}
```

- [ ] **Step 4: `data/scorecard-manual.yaml`** (rows the CI cannot run; `when` is a date the operator sets; stale after 90 days)

```yaml
rows:
  - check: "internet.nl website test"
    value: "100%"
    when: 2026-09-18
    link: https://internet.nl/site/herinean.com/
    link_text: internet.nl
  - check: "SSL Labs"
    value: "A (TLS 1.3 only; A+ is withheld from 1.3-only servers — ADR-0011)"
    when: 2026-09-17
    link: https://www.ssllabs.com/ssltest/analyze.html?d=herinean.com
    link_text: SSL Labs
  - check: "Mozilla HTTP Observatory"
    value: "A+ (145)"
    when: 2026-09-17
    link: https://developer.mozilla.org/en-US/observatory/analyze?host=herinean.com
    link_text: Observatory
  - check: "securityheaders.com"
    value: "A+"
    when: 2026-09-18
    link: https://securityheaders.com/?q=herinean.com&followRedirects=on
    link_text: securityheaders.com
```

- [ ] **Step 5: `scorecard.go`** + test

```go
package site

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/raduherinean/herinean.com/internal/render"
	"gopkg.in/yaml.v3"
)

// ciRow is what the bench writes into scorecard.json (M2).
type ciRow struct {
	Check    string `json:"check"`
	Pass     bool   `json:"pass"`
	Value    string `json:"value"`
	When     string `json:"when"` // YYYY-MM-DD
	Link     string `json:"link"`
	LinkText string `json:"link_text"`
}

type manualFile struct {
	Rows []struct {
		Check    string    `yaml:"check"`
		Value    string    `yaml:"value"`
		When     time.Time `yaml:"when"`
		Link     string    `yaml:"link"`
		LinkText string    `yaml:"link_text"`
	} `yaml:"rows"`
}

const staleAfter = 90 * 24 * time.Hour

func scorecardRows(ciPath, manualPath string, now time.Time) ([]render.ScorecardRow, error) {
	var rows []render.ScorecardRow
	if b, err := os.ReadFile(ciPath); err == nil {
		var ci []ciRow
		if err := json.Unmarshal(b, &ci); err != nil {
			return nil, fmt.Errorf("%s: %w", ciPath, err)
		}
		for _, r := range ci {
			rows = append(rows, render.ScorecardRow{Check: r.Check, Pass: r.Pass, Value: r.Value, When: r.When, Link: r.Link, LinkText: r.LinkText})
		}
	} else {
		rows = append(rows, render.ScorecardRow{Check: "CI audit (Lighthouse, HTML, a11y, links, feeds, headers)", Pass: false, Value: "not yet run on this build", When: now.Format("2006-01-02")})
	}
	if b, err := os.ReadFile(manualPath); err == nil {
		var m manualFile
		if err := yaml.Unmarshal(b, &m); err != nil {
			return nil, fmt.Errorf("%s: %w", manualPath, err)
		}
		for _, r := range m.Rows {
			rows = append(rows, render.ScorecardRow{Check: r.Check, Pass: true, Value: r.Value, When: r.When.Format("2006-01-02"), Link: r.Link, LinkText: r.LinkText, Stale: now.Sub(r.When) > staleAfter})
		}
	}
	return rows, nil
}

func (b *build) scorecardFragment() ([]byte, error) {
	rows, err := scorecardRows(filepath.Join(b.o.Root, "scorecard.json"), filepath.Join(b.o.Root, "data", "scorecard-manual.yaml"), b.now)
	if err != nil {
		return nil, err
	}
	return b.r.Fragment("scorecard", map[string]any{"Rows": rows})
}

// Scorecard renders the fragment for CI to validate and store in KV (M2).
func Scorecard(o Options, in, manual, out string) error {
	r, err := render.New(filepath.Join(o.Root, "templates"), filepath.Join(o.Root, "assets", "css", "site.css"))
	if err != nil {
		return err
	}
	now, err := BuildTime(o.Root)
	if err != nil {
		return err
	}
	rows, err := scorecardRows(in, manual, now)
	if err != nil {
		return err
	}
	frag, err := r.Fragment("scorecard", map[string]any{"Rows": rows})
	if err != nil {
		return err
	}
	return os.WriteFile(out, frag, 0o644)
}
```
`internal/site/scorecard_test.go`:
```go
package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScorecardRows(t *testing.T) {
	dir := t.TempDir()
	ci := filepath.Join(dir, "scorecard.json")
	_ = os.WriteFile(ci, []byte(`[{"check":"Lighthouse","pass":true,"value":"100/100/100/100","when":"2026-10-11","link":"https://pagespeed.web.dev/","link_text":"PageSpeed"}]`), 0o644)
	man := filepath.Join(dir, "manual.yaml")
	_ = os.WriteFile(man, []byte("rows:\n  - check: SSL Labs\n    value: A\n    when: 2026-01-01\n    link: https://x\n    link_text: x\n"), 0o644)
	rows, err := scorecardRows(ci, man, time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || !rows[0].Pass || rows[0].Check != "Lighthouse" || !rows[1].Stale {
		t.Errorf("%+v", rows)
	}
	rows, _ = scorecardRows(filepath.Join(dir, "nope.json"), man, time.Now())
	if len(rows) != 2 || rows[0].Pass || !strings.Contains(rows[0].Value, "not yet") {
		t.Errorf("missing CI file must yield an honest failing row: %+v", rows)
	}
}
```

- [ ] **Step 6: Run everything, commit** — `go test ./... && go vet ./... && staticcheck ./...`; commit `M1a: site serve, site new, scorecard rendering with honest missing-CI row` with trailers; push.

---

### Task 12: Content, static files, author inputs, first real build, PR

**Files:**
- Create: `content/en/_home.md`, `content/ro/_home.md`, `content/en/_privacy.md`, `content/ro/_privacy.md`, `content/en/_colophon.md`, `i18n/en.yaml`, `i18n/ro.yaml` (copies of the fixture sets), `static/favicon.svg`, `static/favicon.ico`, `static/apple-touch-icon.png`, `static/.well-known/mta-sts.txt` (moved from `infra/placeholder/`), `assets/portrait.jpg` (author), `internal/images/favicon.go`, `internal/images/favicon_test.go`
- Modify: `site.yaml` (author values), `README.md` (status → M1a), `RUNBOOK.md` (build/serve/new)

- [ ] **Step 1: Favicon generator** — option B from the export: paper lowercase "h" on oxblood, as one SVG path from the Newsreader glyph, plus a 32 px ICO and a 180 px PNG rendered from the same glyph. `internal/images/favicon.go`:

```go
package images

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

var favBg = color.RGBA{0x7D, 0x2B, 0x22, 0xFF}
var favFg = color.RGBA{0xFA, 0xF8, 0xF4, 0xFF}

// FaviconSVG traces the glyph 'h' from the display font into a path on an oxblood square: no font loads for a 16px mark.
func FaviconSVG(fontBytes []byte) ([]byte, error) {
	f, err := sfnt.Parse(fontBytes)
	if err != nil {
		return nil, err
	}
	var buf sfnt.Buffer
	const ppem = 1000
	gi, err := f.GlyphIndex(&buf, 'h')
	if err != nil || gi == 0 {
		return nil, fmt.Errorf("no glyph for h")
	}
	segs, err := f.LoadGlyph(&buf, gi, fixed.I(ppem), nil)
	if err != nil {
		return nil, err
	}
	adv, _ := f.GlyphAdvance(&buf, gi, fixed.I(ppem), font.HintingNone)
	var d bytes.Buffer
	fx := func(v fixed.Int26_6) float64 { return float64(v) / 64 }
	for _, s := range segs {
		switch s.Op {
		case sfnt.SegmentOpMoveTo:
			fmt.Fprintf(&d, "M%.1f %.1f", fx(s.Args[0].X), fx(s.Args[0].Y))
		case sfnt.SegmentOpLineTo:
			fmt.Fprintf(&d, "L%.1f %.1f", fx(s.Args[0].X), fx(s.Args[0].Y))
		case sfnt.SegmentOpQuadTo:
			fmt.Fprintf(&d, "Q%.1f %.1f %.1f %.1f", fx(s.Args[0].X), fx(s.Args[0].Y), fx(s.Args[1].X), fx(s.Args[1].Y))
		case sfnt.SegmentOpCubeTo:
			fmt.Fprintf(&d, "C%.1f %.1f %.1f %.1f %.1f %.1f", fx(s.Args[0].X), fx(s.Args[0].Y), fx(s.Args[1].X), fx(s.Args[1].Y), fx(s.Args[2].X), fx(s.Args[2].Y))
		}
	}
	d.WriteString("Z")
	// Glyph space: y grows downward in sfnt segments (already flipped); centre the advance box in a 1000-unit square with the baseline at 76%.
	w := fx(adv)
	tx := (1000 - w) / 2
	// Baseline at 840 of 1000: the h ascender (~720 units) then sits with ~120 above and ~160 below — optically centred.
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1000 1000"><rect width="1000" height="1000" fill="#7D2B22"/><path transform="translate(%.1f 840)" fill="#FAF8F4" d="%s"/></svg>`, tx, d.String())
	return []byte(svg), nil
}

// FaviconRaster renders the same mark at size px (32 for ICO, 180 for apple-touch-icon).
func FaviconRaster(f *Fonts, size int) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	fill(img, img.Bounds(), favBg)
	fc, err := face(f.Display, float64(size)*0.78)
	if err != nil {
		return nil, err
	}
	w := textWidth(fc, "h")
	drawText(img, fc, favFg, (size-w)/2, int(float64(size)*0.76), "h")
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ICO wraps one PNG in an ICO container (browsers accept PNG-in-ICO).
func ICO(pngBytes []byte, size int) []byte {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.LittleEndian, []uint16{0, 1, 1})
	b.Write([]byte{byte(size), byte(size), 0, 0})
	_ = binary.Write(&b, binary.LittleEndian, []uint16{1, 32})
	_ = binary.Write(&b, binary.LittleEndian, uint32(len(pngBytes)))
	_ = binary.Write(&b, binary.LittleEndian, uint32(22))
	b.Write(pngBytes)
	return b.Bytes()
}
```
Test (`favicon_test.go`): SVG contains `<path` and `fill="#FAF8F4"`; raster decodes to size×size; ICO starts with `00 00 01 00 01 00`. Generate the three files once with a tiny `go run` snippet (or a `site favicon` sub-step in Task 12 step 3) into `static/`; they are committed, not rebuilt every time.

- [ ] **Step 2: i18n and pages.** Copy the fixture's `i18n/en.yaml` and `ro.yaml` to the repo root (including `date.months_short`). Write the pages:

`content/en/_privacy.md`:
```markdown
---
title: "Privacy"
summary: "No cookies, no analytics scripts, no third-party requests. What Cloudflare sees, and what the edge counts."
---
This site stores no personal data and sets no cookies. There is no analytics script, no third-party request, and nothing to consent to.

Cloudflare serves the site from its edge network and, like any host, processes connection data (your IP address, the request) to deliver the page; it does not receive it from me and I do not receive it from Cloudflare. Their notice is at [cloudflare.com/privacypolicy](https://www.cloudflare.com/privacypolicy/).

I count page views: the path, the language, the referring site's host name, a `?ref=` tag when I put one in a link, and the country Cloudflare derives from the connection. I never store IP addresses, user agents or full referrer URLs, and nothing here can tell two readers apart. The numbers say which pieces were read, not who read them.

If that ever changes — a newsletter, for instance — this page changes first.

Questions: [security@herinean.com](mailto:security@herinean.com).
```

`content/ro/_privacy.md`:
```markdown
---
title: "Confidențialitate"
summary: "Fără cookie-uri, fără scripturi de analiză, fără cereri către terți. Ce vede Cloudflare și ce numără marginea rețelei."
---
Acest site nu stochează date personale și nu setează cookie-uri. Nu există niciun script de analiză, nicio cerere către terți și nimic la care să consimțiți.

Cloudflare livrează site-ul din rețeaua sa de margine și, ca orice gazdă, procesează datele conexiunii (adresa IP, cererea) pentru a servi pagina; nu le primește de la mine și eu nu le primesc de la Cloudflare. Politica lor: [cloudflare.com/privacypolicy](https://www.cloudflare.com/privacypolicy/).

Număr vizualizările: calea, limba, numele de gazdă al site-ului de proveniență, o etichetă `?ref=` când o pun eu într-un link, și țara pe care Cloudflare o deduce din conexiune. Nu stochez niciodată adrese IP, user agent sau URL-uri complete de proveniență, și nimic de aici nu poate deosebi doi cititori. Cifrele spun ce texte au fost citite, nu cine le-a citit.

Dacă asta se schimbă vreodată — un newsletter, de exemplu — această pagină se schimbă prima.

Întrebări: [security@herinean.com](mailto:security@herinean.com).
```

`content/en/_colophon.md`:
```markdown
---
title: "Colophon"
summary: "How herinean.com is built and audited: the scorecard, the stack, the dependencies, the trade-offs."
---
This page is the site's test suite, written down. Every claim below is checked on each commit; the build fails loudly rather than quietly. If a row says fail, it is failing right now and the fix is queued, not hidden.

The platform is a Go program of a few hundred lines that turns Markdown into these pages, a Cloudflare Worker of about eighty lines at the edge, and the zone configuration as code. There is no server, no database, no JavaScript in the browser, no cookie, and no third-party request. The whole thing is public: [github.com/raduherinean/herinean.com](https://github.com/raduherinean/herinean.com), including the [design specification](https://github.com/raduherinean/herinean.com/blob/main/docs/specs/2026-09-17-herinean-com-design.md) and the decisions that shaped it.

**How AI is used.** ⟨author sentence, from site.yaml ai_disclosure — repeat here in prose⟩

**Analytics.** Page views are counted at the edge with the path, language, referring host, a `?ref=` tag and country. Not stored: IP address, user agent, full referrer, or anything that could count readers rather than reads. Views include some bots; the numbers compare pieces, they don't measure reach.

**Known trade-offs.** No comments (discussion is on LinkedIn), no search (the index is small and browsers find text), no dark-mode toggle (the OS decides), no unique-visitor counts (see above), no tags or related posts, no embeds of any kind (they need scripts), TLS 1.3 only (SSL Labs withholds A+ from 1.3-only servers — see ADR-0011 — while internet.nl scores 100%), and one vendor at the edge (the runbook covers the day it fails). URLs never change and pieces are never deleted; corrections carry an "updated" date.
```
`content/en/_home.md` and `content/ro/_home.md` are the About paragraphs — **author input**; commit them with the ⟨⟩ markers (title `Radu Herinean`, summary = the tagline) so `check` exits 3 until Radu writes them, exactly as designed.

- [ ] **Step 3: Static files** — generate the favicons with a one-off run:
```bash
cat > /tmp/fav.go <<'EOF'
package main
import ("os"; "github.com/raduherinean/herinean.com/internal/images")
func main() {
	nb, _ := os.ReadFile("assets/fonts/og/Newsreader72pt-Medium.ttf")
	svg, err := images.FaviconSVG(nb); if err != nil { panic(err) }
	_ = os.WriteFile("static/favicon.svg", svg, 0o644)
	f, err := images.LoadFonts("assets/fonts/og"); if err != nil { panic(err) }
	p32, _ := images.FaviconRaster(f, 32); _ = os.WriteFile("static/favicon.ico", images.ICO(p32, 32), 0o644)
	p180, _ := images.FaviconRaster(f, 180); _ = os.WriteFile("static/apple-touch-icon.png", p180, 0o644)
}
EOF
mkdir -p static/.well-known && cp /tmp/fav.go ./fav_tmp.go && go run ./fav_tmp.go && rm fav_tmp.go
cp infra/placeholder/.well-known/mta-sts.txt static/.well-known/mta-sts.txt   # copy, not move: deploy-placeholder.sh still serves infra/placeholder until M1b points wrangler at dist/
```
Open `static/favicon.svg` in a browser and compare with `docs/design/claude-design-export/renders/favicons.png` option B; if the glyph sits too high or low, adjust the `840` baseline in `FaviconSVG` and regenerate.

- [ ] **Step 4: Author inputs** — Radu supplies: `site.yaml` values (tagline EN/RO, LinkedIn and X URLs, AI-disclosure sentence), `content/{en,ro}/_home.md` (3–4 paragraphs), the ⟨⟩ line in `_colophon.md`, and `assets/portrait.jpg` (≥ 800 px square). `go run ./cmd/site check` is the gate: it lists every placeholder left.

- [ ] **Step 5: First real build**

```bash
go run ./cmd/site check && go run ./cmd/site build && go run ./cmd/site check --dist && go run ./cmd/site serve --host 0.0.0.0 --port 8080
```
Read every page on a phone over the LAN and on a laptop; compare with the renders. Then the two invariants a browser can show and the binary cannot: view-source has exactly one `<script>` (JSON-LD); DevTools → Network on a piece page shows one document, one favicon, nothing else (fonts arrive in M1b).

- [ ] **Step 6: README status and RUNBOOK additions**, then commit and PR:
```bash
git add -A && git commit -m "$(cat <<'MSG'
M1a: content pages, favicons from the Newsreader glyph, first real build

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
)"
git push gitea m1a/generator && git push -u origin m1a/generator
gh pr create --base infra/m0 --title "M1a: generator core" --body "$(cat <<'BODY'
Bespoke Go static generator: content model with validation (dates, slugs, Romanian diacritics, alt text, i18n completeness, pairing by key), Markdown rendering with per-language typography, responsive WebP images, OG cards, templates and CSS from the Claude Design export, feeds, sitemap/robots/llms/security.txt, _headers with the CSS hash, deterministic dist/, check and check --dist, serve/new/scorecard. Golden tests pin every page.

Spec: docs/specs/2026-09-17-herinean-com-design.md §4, §5, §9. Design: docs/design/export-review.md.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

https://claude.ai/code/session_01CyBKVKXNbCCfWDPgYiMTKf
BODY
)"
```
`--base infra/m0` while M0 is unmerged (GitHub retargets the PR to `main` when M0's PR merges and its branch is deleted); drop it if M0 has merged. Merge after review (squash). M1b (Worker analytics + colophon KV, self-hosted fonts, preview deploy) follows on `main`.

---

## Self-review

**Spec coverage.** §4.1 URLs → `config` (Task 0) and `pages()` (Task 10); §4.2 front matter and rules → Task 1; typography, figures, code classes, alt, links → Task 2; i18n completeness, pairing, sorting, required pages → Task 3; images and hashed names → Task 4; OG cards and glyph check (row 21 first half) → Task 5; page anatomy, entry format, `lang` on foreign fragments, badge with hidden name, skip link, meta/OG/twitter/canonical/hreflang/feed autodiscovery, JSON-LD (rows 6–8) → Task 6; feeds (row 5) → Task 7; sitemap/robots/llms/security.txt (row 16) → Task 8; `_headers` (rows 9, 15, 17 static half) → Task 9; reproducibility, all-or-nothing writes, `check --dist` invariants (rows 2 partial, 7, 14, 15) → Task 10; `serve`, `new`, `scorecard`, manual rows with staleness → Task 11; privacy page, colophon policy text, favicon, author inputs → Task 12. Deferred by design: fonts and metric-matched fallbacks (row 21 second half, row 14 font budget), Worker analytics and KV colophon, preview deploy — M1b; external validators, Lighthouse, axe — M2.

**Placeholder scan.** ⟨⟩ markers appear only where the spec (§14) names author inputs; the build refuses them with exit 3 (warning in the hook, failure in CI).

**Adversarial review 2026-09-18, folded in.** Branch base `infra/m0` (main has five files); exit-3 hook so fonts/templates/content commit before author inputs; per-kind template clones (one `ParseGlob` set would render the last `content` everywhere); `webp.Encode(w, img, opts)`; tables: no align/style, wrapped in a focusable region; portrait at 320/480 in `<picture>`; srcset widths deduplicated; code colours from the palette (no generated chroma CSS clashing with `pre`); `! Header` detach in `_headers`; `interest-cohort` dropped; fenced code needs a language + `<pre tabindex` asserted in `check --dist`; home `h1` in the masthead and tagline inside `header`; content-hash cache for variants and OG cards; home OG carries the portrait; static OG fonts at the right optical size and weight; page bodies in the link check; fixture dates pinned before the build epoch; rune-safe ellipsis; byline role via i18n; `data-scorecard` hook; badge `lang` scoped to the code; sitemap `lastmod` from the file's last commit; placeholder `mta-sts.txt` copied, not moved; cruft removed; staticcheck fallback; SVG width/height in any order; srcset-aware `Absolutize` with page-rooted fragments; `clip-path` for `.vh`; page `summary` for meta descriptions.

**Type consistency.** `config.Config` methods `HomeURL/IndexURL/PieceURL/PrivacyURL/ColophonURL/FeedURL/Abs` used identically in Tasks 6, 7, 8, 10; `content.ImageInfo{Src, Srcset, Width, Height, IsSVG}` produced in Task 10 from `images.Info` and consumed by Task 2's renderer; `render.PageData` fields set in Task 10 match Task 6's struct; `render.ScorecardRow` is shared by Task 6 (template), Task 11 (rows) and the fragment; `images.Process(src, key, name, images.Options{…})`, `images.Cache`, `RenderOG(og, fonts, cache)`, `LoadFonts`, `OGFontFiles`, `CheckGlyphs` match between Tasks 4/5/10/12; `content.RenderBody(p, lookup, tableLabel)` between Tasks 2/3; `config.ErrPlaceholder` → `site.ErrAuthorInputs` → exit 3 between Tasks 0/10; `content.Site.T/Latest/AllImageRefs/Render/ByLang/Pages` as declared in Task 3.
