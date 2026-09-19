# M1b — Edge Function, Fonts, Preview Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The real site, built by M1a's generator, runs on Cloudflare as a `noindex` preview with the finished edge function (per-view analytics without a tracker, colophon scorecard filled from KV), self-hosted subset fonts whose metric-matched fallbacks make the swap invisible, and the scripts that will deploy, verify and publish from CI in M2.

**Architecture:** Fonts are static instances subset to Latin + Romanian, content-hashed into `/fonts/`, preloaded once, with `@font-face` fallbacks computed from the font files themselves (`scripts/fontface`). The Worker stays plain JavaScript with no runtime dependency: on the production host it serves assets, writes one Analytics Engine datapoint per HTML view in `waitUntil`, and rewrites `<table data-scorecard>` from KV with HTMLRewriter; on every other host it marks the response `noindex`. Deploys are `site build → site check --dist → wrangler`; M1b uploads a preview version and writes the production script without running it (spec §11: cutover waits for green).

**Tech Stack:** Go 1.27 (existing module; one new `package main` under `scripts/fontface` using `x/image/font/sfnt`); fonttools (`pyftsubset`, `varLib.instancer`) in a venv, outputs committed; Node 24 + `node --test` for the Worker; wrangler 4 via `npx`; Cloudflare Analytics Engine, Workers KV, HTMLRewriter.

**Spec:** `docs/specs/2026-09-17-herinean-com-design.md` — §5.4 (edge function), §6.3 (analytics), §6.4 (wrangler owns the Worker and bindings), §9 (type), §3 rows 14 (fonts ≤ 100 KB, one preloaded, ≤ 6 requests), 15, 17, 21; §11 (M1 ends at a `noindex` preview). Decisions register #3, #4, #6, #10. Prerequisite: M1a executed (`docs/plans/2026-09-18-m1a-generator-core.md`).

**Status (2026-09-19):** executed 2026-09-18 on `m1b/edge` by subagent-driven development, its rulings reviewed adversarially against the spec on 2026-09-19 (all upheld, three fixes in `0799746`), and merged to `main` as PR #3 (merge commit `bcdb361`). The checkboxes below are the plan as written and were not ticked during execution; the branch history and the RUNBOOK are the record. Where the shipped work departs from the steps: the body face ships as "Herinean Serif" — Source Serif 4's binaries reserve the name "Source" under the OFL, so a subset may not carry it (RUNBOOK, Fonts); `font-display: optional` instead of `swap`, measured (spec row 21 and §9 amended on the branch); no bold weight ships — a run-in `<strong>` is letter-spaced caps, any other `<strong>` is italic; `/fonts/*` bypasses the Worker; path canonicalisation answers 301, not the asset layer's 307; speculative loads (prefetch, prerender, non-document fetches) are not counted as views; `wrangler dev` presents the first route's host, so the plan's `--var PROD_HOST` was wrong; the Analytics Engine SQL API rejects `FORMAT TabSeparatedWithNames`, so the scripts read JSON; wrangler is pinned exactly in `scripts/env.sh`; `deploy.sh` publishes the scorecard to KV before every deploy. Production stays on the M0 placeholder until launch (spec §11); the preview is `noindex` on workers.dev.

## Global Constraints

- Zero client-side JavaScript; the Worker runs at the edge only and has **no npm runtime dependency** (wrangler is a deploy tool). Every Worker error falls through to the plain asset response; analytics never sits on the response path.
- Analytics datapoint per HTML view, exactly: `path, lang, referrer host, ref (li|x|nl|md|other|none), country, 1`. Never IP, user agent, full referrer URL. Known bots skipped by UA. Preview hosts: no analytics. This is the sentence on the privacy page and the colophon; the code must not drift from it.
- Fonts: three files — Source Serif 4 Regular, Source Serif 4 Italic, Newsreader Medium (opsz 36) — WOFF2, subset to `U+0020-007E, U+00A0-00FF, U+0100-017F, U+0218-021B` plus the punctuation the site uses; **total ≤ 100 KB**; only the body regular is preloaded; `font-display: swap`; every family has a metric-matched local fallback (`size-adjust`, `ascent-override`, `descent-override`, `line-gap-override`) computed from the files, not typed in; `font-synthesis: none` stays, so no weight the files lack is ever used (600 is removed from the CSS).
- Fonts are served from `/fonts/<name>.<hash8>.woff2` (immutable, M1a `_headers`), the CSS references are rewritten at build time, and `check --dist` proves: total size, one preload per page, every referenced file present.
- Production host stays on the M0 placeholder until launch. M1b's last step is `wrangler versions upload` (preview URL) and `scripts/verify-preview.sh` green. `scripts/deploy.sh` exists and refuses to run without `--yes-production`.
- Branch `m1b/edge` from `m1a/generator` (or from its merge target if M1a has merged). Commits signed, trailers, `main` via PR only.
- `gofmt`, `go vet`, `staticcheck`, `go test ./...`, `node --test worker/` green at every commit.

## Operator inputs

Done 2026-09-18: the infra token carries **Workers KV Storage: Edit** (already there) and **Account Analytics: Read** (added). Task 0 step 4 only confirms it.

Nothing else is needed from Radu; `sudo apt install -y fonts-liberation` only if Task 0 step 3 reports it missing (present on the build machine as of 2026-09-18). The KV namespace id is produced by Task 3 and committed in `wrangler.toml`; account id and token come from `~/.config/herinean/m0.env` as in M0.

## File map

| Path | Responsibility |
|---|---|
| `scripts/fonts.sh` | pinned masters → instancer → subsets (`.woff2` shipped, `.ttf` twin for `check`) → `BUILD.txt`, `fallback.css` |
| `scripts/fontface/main.go` (+ test) | metric-matched `@font-face` fallback rules from two font files |
| `assets/fonts/web/{SourceSerif4-Regular,SourceSerif4-It,Newsreader-Medium}.{woff2,ttf}`, `BUILD.txt`, `fallback.css`, licences | committed outputs |
| `assets/css/site.css` | `@font-face` blocks, tokens with fallback families, no `font-weight:600` |
| `internal/render/render.go` | `New(templatesDir, cssPath, fontURLs)` rewrites `url(/fonts/…)` to hashed paths |
| `internal/render/page.go`, `templates/base.html` | `PreloadFont` |
| `internal/site/build.go`, `check.go`, `checkdist.go` | copy + hash fonts; glyph and drift checks; dist invariants for fonts |
| `worker/index.js`, `worker/index.test.mjs` | edge function and its tests |
| `wrangler.toml` | `PROD_HOST` var, Analytics Engine and KV bindings |
| `scripts/preview.sh`, `scripts/deploy.sh`, `scripts/verify-preview.sh`, `scripts/scorecard-publish.sh`, `scripts/analytics.sh` | operations; CI (M2) calls the same scripts |
| `RUNBOOK.md`, `docs/adr/0004-analytics-without-a-tracker.md`, `docs/adr/0010-self-hosted-fonts.md`, `README.md` | docs |

---

### Task 0: Branch, tooling, permissions

**Files:**
- Modify: `scripts/setup.sh` (tool check gains `fonttools`, `node`)

- [ ] **Step 1: Branch**

```bash
cd "$(git rev-parse --show-toplevel)" && export PATH="$HOME/.local/bin:$PATH"
git checkout m1a/generator && git pull gitea m1a/generator && git checkout -b m1b/edge
go test ./... >/dev/null && echo "M1a green"
```
If `m1a/generator` has merged, branch from its merge target instead. M1a must be complete: `go run ./cmd/site build` works (exit 0 or 3).

- [ ] **Step 2: fonttools in a venv (no sudo, no global pip)**

```bash
python3 -m venv ~/.local/share/fonttools && ~/.local/share/fonttools/bin/pip install -q --upgrade pip && ~/.local/share/fonttools/bin/pip install -q 'fonttools[woff]>=4.58,<5' 'brotli>=1.1'
ln -sf ~/.local/share/fonttools/bin/pyftsubset ~/.local/share/fonttools/bin/fonttools ~/.local/bin/
~/.local/share/fonttools/bin/python -c 'import fontTools; print(fontTools.version)' && pyftsubset --help | head -1
```
Expected: a version line (the exact version is recorded into `assets/fonts/web/BUILD.txt` by `scripts/fonts.sh`, which is how the outputs stay reproducible: same inputs, same fonttools, same bytes).

- [ ] **Step 3: Liberation Serif (Times New Roman metrics) is the fallback the overrides are computed against**

```bash
ls /usr/share/fonts/truetype/liberation/LiberationSerif-Regular.ttf /usr/share/fonts/truetype/liberation/LiberationSerif-Italic.ttf
```
Expected: both present (they are, 2026-09-18). Missing → ask Radu for `sudo apt install -y fonts-liberation`. Ubuntu CI runners carry the same package, which is what makes row 21's swap test meaningful in M2.

- [ ] **Step 4: Token permissions** — after Radu adds them:

```bash
. scripts/env.sh
curl -sS -o /dev/null -w 'kv list: %{http_code}\n' -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" "https://api.cloudflare.com/client/v4/accounts/$CLOUDFLARE_ACCOUNT_ID/storage/kv/namespaces"
curl -sS -o /dev/null -w 'analytics sql: %{http_code}\n' -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" --data "SELECT 1" "https://api.cloudflare.com/client/v4/accounts/$CLOUDFLARE_ACCOUNT_ID/analytics_engine/sql"
```
Expected: `200` and `200` — a `400` from the SQL endpoint also proves the permission (the query was parsed); only `403` means it is missing. Never print the token.

- [ ] **Step 5: `scripts/setup.sh`** — extend the tool loop: `for t in go tofu jq dig curl node fonttools pyftsubset; do …` and add after it: `[ -f /usr/share/fonts/truetype/liberation/LiberationSerif-Regular.ttf ] || echo "missing: fonts-liberation (sudo apt install -y fonts-liberation)"`. Commit: `M1b: setup — fonttools and node in the tool check` with trailers; push `gitea m1b/edge`.

---

### Task 1: Fonts — pinned masters, subsets, metric-matched fallbacks, CSS

**Files:**
- Create: `scripts/fonts.sh`, `scripts/fontface/main.go`, `scripts/fontface/main_test.go`, `assets/fonts/web/*` (generated, committed)
- Modify: `assets/css/site.css`, `internal/site/check.go`, `internal/site/build_test.go` (fixture gets the fonts)

**Interfaces:**
- Produces: `assets/fonts/web/<name>.woff2` (shipped) and `<name>.ttf` (same subset, uncompressed: `check` reads its cmap — Go has no WOFF2 decoder); `assets/fonts/web/fallback.css` (three `@font-face` rules); `assets/fonts/web/BUILD.txt`; `site.WebFonts = []string{"SourceSerif4-Regular", "SourceSerif4-It", "Newsreader-Medium"}` (in `internal/site/fonts.go`, Task 2 uses it too).

- [ ] **Step 1: `scripts/fontface/main.go`** — the fallback generator. Formulas are fontaine's: scale the fallback so its average advance equals the web font's, then override its vertical metrics with the web font's, divided by that scale.

```go
// Command fontface prints a metric-matched fallback @font-face rule (spec §9, row 21).
// A local font scaled and aligned to the web font means the swap moves nothing: CLS stays 0 however slowly the font arrives.
//
//	go run ./scripts/fontface -web assets/fonts/web/SourceSerif4-Regular.ttf -fallback /usr/share/fonts/truetype/liberation/LiberationSerif-Regular.ttf -family "Source Serif 4 Fallback"
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// sample stands in for running text in both languages; the average advance over it is the width the fallback must match.
const sample = "the quick brown fox jumps over the lazy dog and writes about which projects are worth funding this year " +
	"vulpea rapidă sare peste câinele leneș și scrie despre ce proiecte merită finanțate în anul următor"

type metrics struct {
	upem, ascent, descent, lineGap, avg float64 // font units
}

func read(path string) (metrics, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return metrics{}, err
	}
	f, err := sfnt.Parse(b)
	if err != nil {
		return metrics{}, fmt.Errorf("%s: %w", path, err)
	}
	var buf sfnt.Buffer
	upem := fixed.I(int(f.UnitsPerEm()))
	m, err := f.Metrics(&buf, upem, font.HintingNone) // hhea ascender/descender/lineGap at 1 em
	if err != nil {
		return metrics{}, err
	}
	var total float64
	n := 0
	for _, r := range sample {
		gi, err := f.GlyphIndex(&buf, r)
		if err != nil || gi == 0 {
			continue
		}
		adv, err := f.GlyphAdvance(&buf, gi, upem, font.HintingNone)
		if err != nil {
			continue
		}
		total += float64(adv) / 64
		n++
	}
	if n == 0 {
		return metrics{}, fmt.Errorf("%s: no glyphs for the sample", path)
	}
	asc, desc, h := float64(m.Ascent)/64, float64(m.Descent)/64, float64(m.Height)/64
	out := metrics{upem: float64(f.UnitsPerEm()), ascent: asc, descent: desc, lineGap: h - asc - desc, avg: total / float64(n)}
	// Browsers take vertical metrics from OS/2 typo when USE_TYPO_METRICS is set; otherwise hhea (macOS, Linux) or usWin (Windows).
	// x/image/font/sfnt exposes hhea only, so OS/2 is read directly.
	if o, ok := readOS2(b); ok {
		if o.useTypo {
			out.ascent, out.descent, out.lineGap = o.typoAsc, o.typoDesc, o.typoGap
		}
		if o.typoAsc != asc || o.typoDesc != desc || o.winAsc != asc || o.winDesc != desc {
			fmt.Fprintf(os.Stderr, "fontface: %s: hhea %.0f/%.0f, typo %.0f/%.0f, win %.0f/%.0f, USE_TYPO_METRICS=%v — platforms that read a different table shift by the difference\n",
				path, asc, desc, o.typoAsc, o.typoDesc, o.winAsc, o.winDesc, o.useTypo)
		}
	}
	return out, nil
}

type os2 struct {
	useTypo                                          bool
	typoAsc, typoDesc, typoGap, winAsc, winDesc float64 // descents positive, like font.Metrics
}

// readOS2 walks the sfnt table directory to the OS/2 table: fsSelection at 62, sTypoAscender 68, sTypoDescender 70, sTypoLineGap 72, usWinAscent 74, usWinDescent 76.
func readOS2(b []byte) (os2, bool) {
	if len(b) < 12 {
		return os2{}, false
	}
	n := int(binary.BigEndian.Uint16(b[4:]))
	for i := 0; i < n; i++ {
		rec := 12 + 16*i
		if rec+16 > len(b) {
			return os2{}, false
		}
		if string(b[rec:rec+4]) != "OS/2" {
			continue
		}
		off := int(binary.BigEndian.Uint32(b[rec+8:]))
		if off+78 > len(b) {
			return os2{}, false
		}
		t := b[off:]
		i16 := func(o int) float64 { return float64(int16(binary.BigEndian.Uint16(t[o:]))) }
		return os2{
			useTypo:  binary.BigEndian.Uint16(t[62:])&(1<<7) != 0,
			typoAsc:  i16(68),
			typoDesc: -i16(70),
			typoGap:  i16(72),
			winAsc:   float64(binary.BigEndian.Uint16(t[74:])),
			winDesc:  float64(binary.BigEndian.Uint16(t[76:])),
		}, true
	}
	return os2{}, false
}

func pct(v float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", v*100), "0"), ".") + "%"
}

// rule is the @font-face text for web matched against fb.
func rule(web, fb metrics, family, style, weight, local string) string {
	sizeAdjust := (web.avg / web.upem) / (fb.avg / fb.upem)
	return fmt.Sprintf(`@font-face{font-family:%q;font-style:%s;font-weight:%s;src:%s;size-adjust:%s;ascent-override:%s;descent-override:%s;line-gap-override:%s}`,
		family, style, weight, local, pct(sizeAdjust), pct(web.ascent/web.upem/sizeAdjust), pct(web.descent/web.upem/sizeAdjust), pct(web.lineGap/web.upem/sizeAdjust))
}

func main() {
	web := flag.String("web", "", "web font (the subset .ttf)")
	fb := flag.String("fallback", "", "local fallback font file (Liberation Serif has Times New Roman's metrics)")
	family := flag.String("family", "", "CSS family name of the fallback face")
	local := flag.String("local", `local("Times New Roman"),local("Liberation Serif")`, "src: local() list")
	style := flag.String("style", "normal", "font-style")
	weight := flag.String("weight", "400", "font-weight")
	flag.Parse()
	w, err := read(*web)
	if err != nil {
		fail(err)
	}
	f, err := read(*fb)
	if err != nil {
		fail(err)
	}
	fmt.Println(rule(w, f, *family, *style, *weight, *local))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "fontface:", err)
	os.Exit(1)
}
```

`scripts/fontface/main_test.go` — a font matched against itself must produce 100% and its own metrics:
```go
package main

import (
	"os"
	"strings"
	"testing"
)

const liberation = "/usr/share/fonts/truetype/liberation/LiberationSerif-Regular.ttf"

func TestSelfMatchIsIdentity(t *testing.T) {
	if _, err := os.Stat(liberation); err != nil {
		t.Skip("fonts-liberation not installed")
	}
	m, err := read(liberation)
	if err != nil {
		t.Fatal(err)
	}
	r := rule(m, m, "X", "normal", "400", `local("X")`)
	if !strings.Contains(r, "size-adjust:100%") || !strings.Contains(r, "ascent-override:"+pct(m.ascent/m.upem)) {
		t.Errorf("self match must be identity: %s", r)
	}
	if m.upem != 2048 || m.ascent <= 0 || m.descent <= 0 {
		t.Errorf("metrics look wrong: %+v", m)
	}
}

func TestOS2(t *testing.T) {
	b, err := os.ReadFile(liberation)
	if err != nil {
		t.Skip("fonts-liberation not installed")
	}
	o, ok := readOS2(b)
	if !ok || o.typoAsc <= 0 || o.typoDesc <= 0 || o.winAsc < o.typoAsc || o.winDesc < o.typoDesc {
		t.Errorf("OS/2 metrics look wrong: %+v ok=%v", o, ok)
	}
}

func TestPct(t *testing.T) {
	for in, want := range map[float64]string{1: "100%", 0.9512: "95.12%", 0.2634567: "26.346%", 0: "0%"} {
		if got := pct(in); got != want {
			t.Errorf("pct(%v) = %s want %s", in, got, want)
		}
	}
}
```
Run: `go test ./scripts/fontface/ && go vet ./...` → PASS. (`staticcheck` covers it too; it is part of `./...`.)

- [ ] **Step 2: `scripts/fonts.sh`** — pinned by commit and checksum (all values verified 2026-09-18).

```bash
#!/usr/bin/env bash
# Regenerates assets/fonts/web/ from pinned upstream masters. Outputs are committed; run this only when the fonts change.
# Sources are pinned by commit and checksum; the fonttools version used is recorded in BUILD.txt. Same inputs → same bytes.
set -euo pipefail
cd "$(dirname "$0")/.."
export PATH="$HOME/.local/bin:$PATH"
export SOURCE_DATE_EPOCH=1758153600   # fonttools stamps head.modified from this (instancer and subsetter alike): same inputs → same bytes
SRC=.cache/fonts/src; OUT=assets/fonts/web; LIB=/usr/share/fonts/truetype/liberation
mkdir -p "$SRC" "$OUT"
GF=a54f7446f84a1125ef6bf08baa46f3639e8905e0   # google/fonts main, 2026-09-18 (Newsreader variable master + OFL)
AS=80d3f8894c09c937bebfa9011247d2e1c79fd6f4   # adobe-fonts/source-serif release, 2026-09-18 (text optical-size statics)

fetch() { # file url sha256
  [ -f "$SRC/$1" ] || curl -sSL -o "$SRC/$1" "$2"
  echo "$3  $SRC/$1" | sha256sum -c --quiet
}
fetch 'Newsreader[opsz,wght].ttf' "https://raw.githubusercontent.com/google/fonts/$GF/ofl/newsreader/Newsreader%5Bopsz%2Cwght%5D.ttf" 8a08d13f8a6c0d51be379a60af84f945f65369a67e509ee3c3bdcc421254d7c1
fetch OFL-Newsreader.txt        "https://raw.githubusercontent.com/google/fonts/$GF/ofl/newsreader/OFL.txt"                          fdfad38143ec470553cae82a1e45320bdd1b9ec70415d37bd0171051d8a4ded8
fetch SourceSerif4-Regular.ttf  "https://raw.githubusercontent.com/adobe-fonts/source-serif/$AS/TTF/SourceSerif4-Regular.ttf"       e5a4ee6a3d87bb9024796be390c6771e2a0eb1883dae25effaf57ca01668e24b
fetch SourceSerif4-It.ttf       "https://raw.githubusercontent.com/adobe-fonts/source-serif/$AS/TTF/SourceSerif4-It.ttf"            9d2950a8f1da66e21502c35d646a1d2148e79f9ea43fd2158cf02f5232e7f430
fetch LICENSE-SourceSerif4.md   "https://raw.githubusercontent.com/adobe-fonts/source-serif/$AS/LICENSE.md"                        c21d7293d87b6d7ab1d0229a2f55b77f33a7613a6a4e66f6693d68d7d8d09464
cp "$SRC/OFL-Newsreader.txt" "$SRC/LICENSE-SourceSerif4.md" "$OUT/"

# Newsreader: one static cut for titles — weight 500, optical size 36 (masthead 44px, titles 44–55px, entry titles 28px sit around it).
fonttools varLib.instancer "$SRC/Newsreader[opsz,wght].ttf" wght=500 opsz=36 -o "$SRC/Newsreader-Medium.ttf"

# Latin, Latin-1, Latin Extended-A, Romanian comma-below, and the punctuation the templates and typographer emit (– — ‘ ’ ‚ “ ” „ … ‹ › € → −).
UNICODES='U+0020-007E,U+00A0-00FF,U+0100-017F,U+0218-021B,U+02C6,U+02DC,U+2013-2014,U+2018-201A,U+201C-201E,U+2026,U+2039-203A,U+20AC,U+2192,U+2212'
# --layout-features+= APPENDS to pyftsubset's defaults (kern, liga, calt, locl, ccmp, mark, mkmk, rlig …); "=" would replace them and break combining marks.
# tnum: .meta uses tabular-nums. No smcp: labels are letter-spaced uppercase, not small caps.
subset() { # name source
  for flavor in woff2 ttf; do
    args=(--unicodes="$UNICODES" --layout-features+=tnum --name-IDs=0,1,2,3,4,5,6,13,14 --no-hinting --no-recalc-timestamp --output-file="$OUT/$1.$flavor")
    [ "$flavor" = woff2 ] && args+=(--flavor=woff2)
    pyftsubset "$2" "${args[@]}"
  done
}
subset SourceSerif4-Regular "$SRC/SourceSerif4-Regular.ttf"
subset SourceSerif4-It      "$SRC/SourceSerif4-It.ttf"
subset Newsreader-Medium    "$SRC/Newsreader-Medium.ttf"

# Metric-matched fallbacks (spec §9; row 21). Liberation Serif carries Times New Roman's metrics, so one rule serves Windows, macOS and Linux CI.
{
  go run ./scripts/fontface -web "$OUT/SourceSerif4-Regular.ttf" -fallback "$LIB/LiberationSerif-Regular.ttf" -family "Source Serif 4 Fallback"
  go run ./scripts/fontface -web "$OUT/SourceSerif4-It.ttf"      -fallback "$LIB/LiberationSerif-Italic.ttf"  -family "Source Serif 4 Fallback" -style italic -local 'local("Times New Roman Italic"),local("Liberation Serif Italic")'
  go run ./scripts/fontface -web "$OUT/Newsreader-Medium.ttf"    -fallback "$LIB/LiberationSerif-Regular.ttf" -family "Newsreader Fallback" -weight 500
} > "$OUT/fallback.css"

{
  echo "generated by scripts/fonts.sh"
  ~/.local/share/fonttools/bin/python -c 'import fontTools; print("fonttools", fontTools.version)'
  echo "google/fonts $GF"; echo "adobe-fonts/source-serif $AS"; echo "SOURCE_DATE_EPOCH $SOURCE_DATE_EPOCH"
  echo "newsreader instance: wght=500 opsz=36"; echo "unicodes: $UNICODES"; echo "features: defaults + tnum"
  for f in "$OUT"/*.woff2; do printf '%s %s\n' "$(wc -c <"$f")" "$(basename "$f")"; done
} > "$OUT/BUILD.txt"

total=$(cat "$OUT"/*.woff2 | wc -c)
echo "web fonts: $total bytes (budget 102400, row 14)"
[ "$total" -le 102400 ] || { echo "over budget — see the plan's Task 1 step 3 fallback" >&2; exit 1; }
cat "$OUT/fallback.css"
```

- [ ] **Step 3: Run it**

```bash
chmod +x scripts/fonts.sh && scripts/fonts.sh && ls -l assets/fonts/web/ && cat assets/fonts/web/BUILD.txt
```
Expected: three `.woff2` (roughly 25–35 KB each), three `.ttf` twins, two licences, `BUILD.txt`, `fallback.css` with three rules. Run it twice: `git status` must show no change the second time (that is what `SOURCE_DATE_EPOCH` buys). `fallback.css` rules whose `size-adjust` is within 90–110% and `ascent-override` around 90–100% (Source Serif 4 hhea ascender ≈ 0.918 em; Liberation ≈ 0.891). If the total exceeds 100 KB: first drop `U+0100-017F` to the Romanian and Western-European letters actually used (`U+0102-0103,U+00C2-00E2,U+00CE-00EE,U+0218-021B` plus `U+0100-017F` is mostly unused), rerun; if still over, that is a finding for Radu, not a silent budget change. If `varLib.instancer` refuses `opsz=36` (outside the axis range), read the range from the error and pick the closest value; record it in `BUILD.txt`.

Check the italic rule: Liberation Serif Italic vs Source Serif 4 It — average widths differ more than the uprights; a `size-adjust` around 92–105% is normal.

- [ ] **Step 4: `site.css`** — prepend the `@font-face` block, switch the tokens, remove every `font-weight:600`.

At the top of `assets/css/site.css`, before `:root`:
```css
@font-face{font-family:"Source Serif 4";font-style:normal;font-weight:400;font-display:swap;src:url(/fonts/SourceSerif4-Regular.woff2) format("woff2")}
@font-face{font-family:"Source Serif 4";font-style:italic;font-weight:400;font-display:swap;src:url(/fonts/SourceSerif4-It.woff2) format("woff2")}
@font-face{font-family:"Newsreader";font-style:normal;font-weight:500;font-display:swap;src:url(/fonts/Newsreader-Medium.woff2) format("woff2")}
/* metric-matched fallbacks — generated by scripts/fonts.sh into assets/fonts/web/fallback.css; `site check` fails if these drift from it */
<paste the three lines of assets/fonts/web/fallback.css here, verbatim>
```
Tokens: `--serif:"Source Serif 4","Source Serif 4 Fallback",serif;--display:"Newsreader","Newsreader Fallback",serif;` (Georgia leaves the chain: an unmatched fallback would shift the layout, which is the one thing row 21 forbids).

Weight 600 is not shipped and `font-synthesis:none` forbids faking it, so remove its three uses:
- `th{font-weight:600}` → `th{font-weight:400;font-size:var(--fs--2);letter-spacing:.12em;text-transform:uppercase;color:var(--ink-2)}` (the label style the meta line already uses)
- `.fail{color:var(--accent);font-weight:600}` → `.fail{color:var(--accent)}` (the word "Fail" and the accent carry it)
- `.chroma .gh,.chroma .gu{font-weight:600}` → delete the rule

`site.css` keeps `font-synthesis:none` and adds nothing else. (`font-optical-sizing` is irrelevant: the shipped Newsreader has no opsz axis.)

- [ ] **Step 5: `check` learns the fonts** — in `internal/site/check.go`, after the OG glyph loop:

```go
	// Web fonts: the subset twins must carry the Romanian glyphs, and site.css must embed the generated fallback rules verbatim.
	for _, name := range WebFonts {
		if err := images.CheckGlyphs(filepath.Join(o.Root, "assets", "fonts", "web", name+".ttf")); err != nil {
			probs.Add(filepath.Join("assets", "fonts", "web", name+".ttf"), 0, "%v", err)
		}
		if _, err := os.Stat(filepath.Join(o.Root, "assets", "fonts", "web", name+".woff2")); err != nil {
			probs.Add(filepath.Join("assets", "fonts", "web", name+".woff2"), 0, "missing; run scripts/fonts.sh")
		}
	}
	// Every character the UI strings use must have a glyph in the body font, or the fallback draws it (a different "→" on every page).
	if body, err := os.ReadFile(filepath.Join(o.Root, "assets", "fonts", "web", WebFonts[0]+".ttf")); err == nil {
		if f, err := sfnt.Parse(body); err == nil {
			var buf sfnt.Buffer
			for lang, m := range b.site.Strings {
				for key, v := range m {
					for _, r := range v {
						if unicode.IsSpace(r) {
							continue
						}
						if gi, err := f.GlyphIndex(&buf, r); err != nil || gi == 0 {
							probs.Add(filepath.Join("i18n", lang+".yaml"), 0, "%s uses %q (U+%04X), which the body font lacks; add it to UNICODES in scripts/fonts.sh or change the string", key, string(r), r)
						}
					}
				}
			}
		}
	}
	css, _ := os.ReadFile(filepath.Join(o.Root, "assets", "css", "site.css"))
	fb, err := os.ReadFile(filepath.Join(o.Root, "assets", "fonts", "web", "fallback.css"))
	if err != nil {
		probs.Add("assets/fonts/web/fallback.css", 0, "missing; run scripts/fonts.sh")
	} else {
		for i, line := range strings.Split(strings.TrimSpace(string(fb)), "\n") {
			if !strings.Contains(string(css), line) {
				probs.Add("assets/css/site.css", 0, "fallback rule %d differs from assets/fonts/web/fallback.css; paste it verbatim", i+1)
			}
		}
	}
```
(`check.go` imports gain `"unicode"` and `"golang.org/x/image/font/sfnt"`.) And a new file `internal/site/fonts.go`:
```go
package site

// WebFonts are the shipped families, in assets/fonts/web/<name>.woff2 (+ .ttf twin for check). The first is preloaded.
var WebFonts = []string{"SourceSerif4-Regular", "SourceSerif4-It", "Newsreader-Medium"}
```
`internal/site/build_test.go`: the fixture root now also copies `../../assets/fonts` (already does — the whole directory) ✓; nothing to change.

- [ ] **Step 6: Run, commit**

```bash
go run ./cmd/site check; echo "exit $?"     # 0 or 3 (author inputs), never 1
go test ./... && go vet ./... && staticcheck ./...
git add scripts/fonts.sh scripts/fontface assets/fonts/web assets/css/site.css internal/site/check.go internal/site/fonts.go
git commit -m "$(cat <<'MSG'
M1b: fonts — pinned masters, subset WOFF2 with Romanian glyphs, metric-matched fallbacks computed from the files

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
)" && git push gitea m1b/edge
```
(Golden tests in `internal/render` fail until Task 2 regenerates them — Task 2 step 4; commit here anyway, the render tests are the next task's first step.)

---

### Task 2: Build integration — hashed `/fonts/`, CSS rewrite, preload, `check --dist`

**Files:**
- Modify: `internal/render/render.go` (`New` gains `fontURLs`), `internal/render/page.go` (`PreloadFont`), `templates/base.html`, `internal/render/render_test.go`, `internal/site/build.go` (fonts stage, `base()` sets `PreloadFont`), `internal/site/checkdist.go`, `internal/site/scorecard.go` (`render.New` call), `testdata/golden/*` (regenerated)

**Interfaces:**
- `render.New(templatesDir, cssPath string, fontURLs map[string]string) (*Renderer, error)` — keys are the URLs as written in `site.css` (`/fonts/SourceSerif4-Regular.woff2`), values the hashed dist paths; `nil` leaves the CSS untouched.
- `render.PageData.PreloadFont string` — absolute-path URL of the body regular; base emits `<link rel="preload" … as="font" type="font/woff2" crossorigin>` when non-empty.
- `(*build).fonts() (map[string]string, error)` — copies `assets/fonts/web/*.woff2` into `dist/fonts/<name>.<hash8>.woff2`, returns the rewrite map.

- [ ] **Step 1: Failing test first** — in `internal/render/render_test.go`, the fixture passes a fixed map and every page sets `PreloadFont`:

```go
var testFonts = map[string]string{
	"/fonts/SourceSerif4-Regular.woff2": "/fonts/SourceSerif4-Regular.00000001.woff2",
	"/fonts/SourceSerif4-It.woff2":      "/fonts/SourceSerif4-It.00000002.woff2",
	"/fonts/Newsreader-Medium.woff2":    "/fonts/Newsreader-Medium.00000003.woff2",
}
```
`fixture()`: `r, err := New("../../templates", "../../assets/css/site.css", testFonts)`. Every `PageData` literal in the tests gains `PreloadFont: testFonts["/fonts/SourceSerif4-Regular.woff2"]`. `assertInvariants` gains:
```go
	if strings.Count(s, `rel="preload"`) != 1 || !strings.Contains(s, `<link rel="preload" href="/fonts/SourceSerif4-Regular.00000001.woff2" as="font" type="font/woff2" crossorigin>`) {
		t.Error("exactly one font preload, the body regular, hashed")
	}
	if strings.Contains(s, "url(/fonts/SourceSerif4-Regular.woff2)") || !strings.Contains(s, "url(/fonts/SourceSerif4-Regular.00000001.woff2)") {
		t.Error("CSS font URLs must be rewritten to the hashed paths")
	}
```
`TestCSSHashMatchesInlined` also asserts `strings.Count(r.CSS(), "@font-face") == 6` (three faces, three fallbacks).

- [ ] **Step 2: `render.go`**

```go
func New(templatesDir, cssPath string, fontURLs map[string]string) (*Renderer, error) {
	raw, err := os.ReadFile(cssPath)
	if err != nil {
		return nil, err
	}
	css := string(raw)
	for from, to := range fontURLs { // /fonts/x.woff2 → /fonts/x.<hash8>.woff2, before minify and hash
		css = strings.ReplaceAll(css, "url("+from+")", "url("+to+")")
	}
	css = minifyCSS(css)
	…unchanged…
```
(add `"strings"` to the imports.) `page.go`: `PreloadFont string` next to `CSS`. `templates/base.html`, right after the feed `<link>`:
```html
{{if .PreloadFont}}<link rel="preload" href="{{.PreloadFont}}" as="font" type="font/woff2" crossorigin>
{{end}}
```
(`crossorigin` is required for font preloads even same-origin, or the browser fetches the font twice.)

- [ ] **Step 3: `build.go`** — a fonts stage before rendering, and `base()` sets the preload:

```go
// fonts copies the shipped WOFF2 files under content-hashed names and returns the CSS rewrite map.
func (b *build) fonts() (map[string]string, error) {
	m := map[string]string{}
	for _, name := range WebFonts {
		src := filepath.Join(b.o.Root, "assets", "fonts", "web", name+".woff2")
		data, err := os.ReadFile(src)
		if err != nil {
			return nil, fmt.Errorf("fonts: %w (run scripts/fonts.sh)", err)
		}
		p := "fonts/" + name + "." + images.Hash8(data) + ".woff2"
		b.files[p] = data
		m["/fonts/"+name+".woff2"] = "/" + p
	}
	b.preload = m["/fonts/"+WebFonts[0]+".woff2"]
	return m, nil
}
```
`build` struct gains `preload string`. In `Build`, replace the `render.New` call:
```go
	fontURLs, err := b.fonts()
	if err != nil {
		return err
	}
	b.r, err = render.New(filepath.Join(o.Root, "templates"), filepath.Join(o.Root, "assets", "css", "site.css"), fontURLs)
```
`pages.go` `base()`: add `PreloadFont: b.preload` to the `PageData` literal. `scorecard.go`'s `Scorecard()`: `render.New(…, nil)` (the fragment carries no CSS).

- [ ] **Step 4: `checkdist.go`** — fonts, preload and the row 14 estimate (requests ≤ 6, first view ≤ 150 KB), so the bench never discovers them. This block runs **before** the HTML walk (the walk uses `fontBytes` and `faviconBytes`):

```go
	// Fonts: ≤ 100 KB shipped, every page preloads exactly one (the body regular), every preload and CSS url() target exists.
	var fontBytes int64
	_ = filepath.WalkDir(filepath.Join(dist, "fonts"), func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				fontBytes += info.Size()
			}
		}
		return nil
	})
	if fontBytes == 0 || fontBytes > 100*1024 {
		probs.Add("fonts", 0, "shipped fonts total %d bytes; want 1..102400 (row 14)", fontBytes)
	}
	var faviconBytes int64
	if info, err := os.Stat(filepath.Join(dist, "favicon.svg")); err == nil {
		faviconBytes = info.Size()
	}
```
and inside the per-HTML walk, after the gzip measurement:
```go
		// Row 14 estimate: document + favicon + fonts (italic only when the page uses it) + images that are not lazy.
		fonts := 2 // body regular + display
		if reItalic.MatchString(s) {
			fonts++
		}
		var eagerBytes int64
		eager := 0
		for _, m := range reEagerPicture.FindAllStringSubmatch(s, -1) { // <picture> with WebP candidates: the browser picks at most the largest
			eager++
			cands := strings.Split(m[1], ",")
			last := strings.Fields(strings.TrimSpace(cands[len(cands)-1]))[0]
			if info, err := os.Stat(filepath.Join(dist, filepath.FromSlash(last))); err == nil {
				eagerBytes += info.Size()
			}
		}
		for _, tag := range reImg.FindAllString(rePictureBlock.ReplaceAllString(s, ""), -1) { // plain <img> outside any <picture>
			if strings.Contains(tag, `loading="lazy"`) {
				continue
			}
			eager++
			if m := reSrc.FindStringSubmatch(tag); m != nil { // the fallback file: an upper bound, WebP browsers never fetch it
				if info, err := os.Stat(filepath.Join(dist, filepath.FromSlash(m[1]))); err == nil {
					eagerBytes += info.Size()
				}
			}
		}
		if n := 2 + fonts + eager; n > 6 {
			probs.Add(rel, 0, "estimated first-view requests %d > 6 (row 14): document, favicon, %d fonts, %d eager images", n, fonts, eager)
		}
		if total := int64(gz.Len()) + faviconBytes + fontBytes + eagerBytes; total > 150*1024 {
			probs.Add(rel, 0, "estimated first-view transfer %d bytes > 153600 (row 14): html %d, favicon %d, fonts %d, images %d", total, gz.Len(), faviconBytes, fontBytes, eagerBytes)
		}
		if n := strings.Count(s, `rel="preload"`); n != 1 {
			probs.Add(rel, 0, "%d preloads; exactly one (the body regular) is allowed", n)
		}
		for _, m := range rePreload.FindAllStringSubmatch(s, -1) {
			if _, err := os.Stat(filepath.Join(dist, filepath.FromSlash(m[1]))); err != nil || !strings.Contains(m[1], ".woff2") {
				probs.Add(rel, 0, "preload target %s is not a shipped font", m[1])
			}
		}
		for _, m := range reFontURL.FindAllStringSubmatch(s, -1) {
			if _, err := os.Stat(filepath.Join(dist, filepath.FromSlash(m[1]))); err != nil {
				probs.Add(rel, 0, "CSS references %s which is not in dist", m[1])
			}
		}
```
with
```go
	rePreload      = regexp.MustCompile(`<link rel="preload" href="([^"]+)" as="font"`)
	reFontURL      = regexp.MustCompile(`url\((/fonts/[^)]+)\)`)
	reItalic       = regexp.MustCompile(`<em\b|<i\b|<blockquote\b|<cite\b|class="c1?"|class="cm"|class="cs"`)
	reEagerPicture = regexp.MustCompile(`<picture><source type="image/webp" srcset="([^"]+)"[^>]*><img [^>]*fetchpriority="high"`)
	rePictureBlock = regexp.MustCompile(`(?s)<picture>.*?</picture>`)
	reImg          = regexp.MustCompile(`<img\b[^>]*>`)
	reSrc          = regexp.MustCompile(`src="([^"]+)"`)
```
(Go's `regexp` has no lookahead, hence the two-pass shape: pictures first, then plain images with the picture blocks cut out.)

- [ ] **Step 5: Regenerate goldens, run everything, look**

```bash
go test ./internal/render/ -update && go test ./... && go vet ./... && staticcheck ./...
go run ./cmd/site build && go run ./cmd/site check --dist && ls -l dist/fonts/ && go run ./cmd/site serve --host 0.0.0.0 --port 8080
```
In a browser on a piece page: DevTools → Network shows the document, `favicon.svg`, and two or three `.woff2` (italic only if the page has italics) — ≤ 6 requests with images; Response headers on a font: `cache-control: public, max-age=31536000, immutable` and `content-type: font/woff2`. Then the swap test by hand (the automated one is M2's bench): DevTools → Network → right-click a font → "Block request URL" for `/fonts/*`, reload, take a screenshot of the first screen; unblock, reload, screenshot; the two must align within a pixel. If a line breaks differently between them, the `size-adjust` is off — recheck `fallback.css` was pasted verbatim and that the browser actually used Liberation/Times (Rendered Fonts in the Computed panel).

- [ ] **Step 6: Commit** — `M1b: build — hashed /fonts/, CSS rewrite, one preload, dist invariants for fonts` with trailers; push.

---

### Task 3: The edge function — analytics, KV colophon, tests, bindings

**Files:**
- Modify: `worker/index.js`, `wrangler.toml`, `.githooks/pre-commit`
- Create: `worker/index.test.mjs`, `worker/package.json` (`{"type":"module","private":true}` — so `node --test` treats `.js` as ESM; no dependencies)

**Interfaces:**
- Bindings: `env.ASSETS` (existing), `env.VIEWS` (Analytics Engine dataset `herinean_views`), `env.SCORECARD` (KV; keys `scorecard` = validated HTML fragment `<table data-scorecard>…</table>`, `scorecard.json` = the CI results), `env.PROD_HOST` (var; `herinean.com`, overridable for `wrangler dev`).
- Datapoint: `blobs: [path, lang, referrerHost, ref, country]`, `doubles: [1]`, `indexes: [path]` — blob1…blob5 in `scripts/analytics.sh`. A view = GET, page path (`/` or `.html`), status 200 **or 304**.
- KV keys: `scorecard` (fragment), `scorecard.json`, `scorecard.etag` (8 hex chars of the fragment's sha256; the Worker composes `W/"<asset>-<tag>"`).

- [ ] **Step 1: Failing tests** — `worker/index.test.mjs` (Node's built-in runner; `Request`/`Response`/`Headers` are globals in Node 24; HTMLRewriter is not, so the KV path is verified with `wrangler dev` in step 4):

```js
import { test } from "node:test";
import assert from "node:assert/strict";
import worker from "./index.js";

const HTML = { status: 200, headers: { "content-type": "text/html; charset=utf-8" } };
function harness({ assets = () => new Response("<!doctype html><html></html>", HTML), kv } = {}) {
  const points = [];
  const waits = [];
  const env = {
    PROD_HOST: "herinean.com",
    ASSETS: { fetch: async (req) => assets(req) },
    VIEWS: { writeDataPoint: (p) => points.push(p) },
    SCORECARD: kv,
  };
  const ctx = { waitUntil: (p) => waits.push(p) };
  return { env, ctx, points, settle: () => Promise.all(waits) };
}
const req = (url, init = {}, cf) => {
  const r = new Request(url, { headers: { "user-agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/128 Safari/537.36" }, ...init });
  if (cf) r.cf = cf;
  return r;
};

test("production HTML view → one datapoint: path, lang, referrer host, ref, country", async () => {
  const h = harness();
  const res = await worker.fetch(req("https://herinean.com/ro/articole/x/?ref=li", { headers: { referer: "https://www.linkedin.com/feed/", "user-agent": "Mozilla/5.0" } }, { country: "RO" }), h.env, h.ctx);
  assert.equal(res.status, 200);
  await h.settle();
  assert.deepEqual(h.points, [{ blobs: ["/ro/articole/x/", "ro", "www.linkedin.com", "li", "RO"], doubles: [1], indexes: ["/ro/articole/x/"] }]);
});

test("ref: unknown tag is 'other', absent is 'none'; internal referrer is dropped; no country is empty", async () => {
  const h = harness();
  await worker.fetch(req("https://herinean.com/writing/x/?ref=campaign", { headers: { referer: "https://herinean.com/", "user-agent": "Mozilla/5.0" } }), h.env, h.ctx);
  await worker.fetch(req("https://herinean.com/"), h.env, h.ctx);
  await h.settle();
  assert.deepEqual(h.points.map((p) => p.blobs), [["/writing/x/", "en", "", "other", ""], ["/", "en", "", "none", ""]]);
});

test("nothing is counted for bots, previews, non-HTML, errors or HEAD", async () => {
  const h = harness({ assets: (r) => (r.url.endsWith("/feed.xml") ? new Response("<rss/>", { headers: { "content-type": "application/rss+xml" } }) : r.url.endsWith("/nope/") ? new Response("nf", { status: 404, headers: { "content-type": "text/html" } }) : new Response("<html></html>", HTML)) });
  await worker.fetch(req("https://herinean.com/", { headers: { "user-agent": "Mozilla/5.0 (compatible; Googlebot/2.1)" } }), h.env, h.ctx);
  await worker.fetch(req("https://preview-herinean-com.example.workers.dev/"), h.env, h.ctx);
  await worker.fetch(req("https://herinean.com/feed.xml"), h.env, h.ctx);
  await worker.fetch(req("https://herinean.com/nope/"), h.env, h.ctx);
  await worker.fetch(req("https://herinean.com/", { method: "HEAD" }), h.env, h.ctx);
  await h.settle();
  assert.deepEqual(h.points, []);
});

test("a 304 revalidation is a view too — returning readers", async () => {
  const h = harness({ assets: () => new Response(null, { status: 304, headers: { etag: '"abc"' } }) });
  await worker.fetch(req("https://herinean.com/writing/x/", { headers: { "if-none-match": '"abc"', "user-agent": "Mozilla/5.0" } }), h.env, h.ctx);
  await h.settle();
  assert.equal(h.points.length, 1);
  assert.equal(h.points[0].blobs[0], "/writing/x/");
});

test("colophon: composite ETag from the asset and the KV fragment; a matching If-None-Match → 304 before HTMLRewriter", async () => {
  const kv = { get: async (k) => ({ scorecard: "<table data-scorecard></table>", "scorecard.etag": "feed1234" })[k] ?? null };
  let sawConditional = null;
  const h = harness({ kv, assets: (r) => { sawConditional = r.headers.has("if-none-match"); return new Response("<html><table data-scorecard></table></html>", { status: 200, headers: { "content-type": "text/html; charset=utf-8", etag: '"abc"' } }); } });
  const res = await worker.fetch(req("https://herinean.com/colophon/", { headers: { "if-none-match": 'W/"abc-feed1234"', "user-agent": "Mozilla/5.0" } }), h.env, h.ctx);
  assert.equal(res.status, 304);
  assert.equal(res.headers.get("etag"), 'W/"abc-feed1234"');
  assert.equal(sawConditional, false, "the asset layer must never see the colophon's conditional header");
  await h.settle();
  assert.equal(h.points.length, 1, "the 304 is a view");
});

test("preview hosts: noindex on every response and a disallow-all robots.txt", async () => {
  const h = harness();
  const page = await worker.fetch(req("https://abc-herinean-com.example.workers.dev/"), h.env, h.ctx);
  assert.equal(page.headers.get("x-robots-tag"), "noindex, nofollow");
  const robots = await worker.fetch(req("https://abc-herinean-com.example.workers.dev/robots.txt"), h.env, h.ctx);
  assert.equal(await robots.text(), "User-agent: *\nDisallow: /\n");
});

test("mta-sts host serves the policy file only", async () => {
  const h = harness({ assets: () => new Response("version: STSv1\n", { headers: { "content-type": "text/plain" } }) });
  assert.equal((await worker.fetch(req("https://mta-sts.herinean.com/.well-known/mta-sts.txt"), h.env, h.ctx)).status, 200);
  assert.equal((await worker.fetch(req("https://mta-sts.herinean.com/"), h.env, h.ctx)).status, 404);
});

test("scorecard.json comes from KV, 404 when unpublished", async () => {
  const kv = { get: async (k) => (k === "scorecard.json" ? '{"rows":[]}' : null) };
  const h = harness({ kv });
  const res = await worker.fetch(req("https://herinean.com/colophon/scorecard.json"), h.env, h.ctx);
  assert.equal(res.status, 200);
  assert.equal(res.headers.get("content-type"), "application/json; charset=utf-8");
  const none = await worker.fetch(req("https://herinean.com/colophon/scorecard.json"), harness({ kv: { get: async () => null } }).env, h.ctx);
  assert.equal(none.status, 404);
});

test("a Worker error falls through to the plain asset", async () => {
  const h = harness();
  h.env.VIEWS = { writeDataPoint: () => { throw new Error("boom"); } };
  const res = await worker.fetch(req("https://herinean.com/"), h.env, h.ctx);
  assert.equal(res.status, 200);
  await assert.doesNotReject(h.settle());
});
```
Run: `cd worker && node --test` → fails on the missing behaviour.

- [ ] **Step 2: `worker/index.js`**

```js
// herinean.com edge function. Zero client-side JS is the site's rule; this runs at the edge only.
// Hosts: production apex (assets; one analytics datapoint per HTML view, off the response path; colophon scorecard from KV);
// mta-sts (one file, everything else 404); anything else is a preview (noindex, disallow-all robots, no analytics).
// Any error here degrades to the plain asset response — the site never depends on this code to render.

const MTA_STS_HOST = "mta-sts.herinean.com";
const REFS = new Set(["li", "x", "nl", "md"]);
const BOT_UA = /bot|crawl|spider|slurp|preview|fetch|lighthouse|headless|monitor|curl|wget|python-requests|facebookexternalhit|linkedinbot|twitterbot|whatsapp|telegram|discord|slack|skype/i;
const isPage = (p) => p.endsWith("/") || p.endsWith(".html");

export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);
    const prodHost = env.PROD_HOST || "herinean.com";

    if (url.host === MTA_STS_HOST) {
      if (url.pathname === "/.well-known/mta-sts.txt") return env.ASSETS.fetch(request);
      return new Response("Not found\n", { status: 404, headers: { "content-type": "text/plain; charset=utf-8" } });
    }

    if (url.host !== prodHost) {
      if (url.pathname === "/robots.txt") {
        return new Response("User-agent: *\nDisallow: /\n", {
          status: 200,
          headers: { "content-type": "text/plain; charset=utf-8", "x-robots-tag": "noindex, nofollow" },
        });
      }
      const res = await withScorecard(request, url, await env.ASSETS.fetch(assetRequest(request, url)), env);
      const headers = new Headers(res.headers);
      headers.set("x-robots-tag", "noindex, nofollow");
      return new Response(res.body, { status: res.status, statusText: res.statusText, headers });
    }

    if (url.pathname === "/colophon/scorecard.json") return scorecardJSON(env);

    const res = await withScorecard(request, url, await env.ASSETS.fetch(assetRequest(request, url)), env);
    // A view is a GET for a page path answered 200 or 304: revalidations are the returning readers, and a 304 carries no content-type.
    if (request.method === "GET" && (res.status === 200 || res.status === 304) && isPage(url.pathname)) {
      ctx.waitUntil(Promise.resolve().then(() => count(request, url, env)).catch(() => {}));
    }
    return res;
  },
};

// The colophon's validator is composed below, so the asset layer must not answer its conditional requests itself.
function assetRequest(request, url) {
  if (url.pathname !== "/colophon/" || !request.headers.has("if-none-match")) return request;
  const headers = new Headers(request.headers);
  headers.delete("if-none-match");
  return new Request(request, { headers });
}

// count writes: path, lang, referrer host, ref, country, 1. Never IP, user agent or the full referrer (spec §6.3; the privacy page says exactly this).
function count(request, url, env) {
  if (!env.VIEWS || BOT_UA.test(request.headers.get("user-agent") || "")) return;
  const tag = url.searchParams.get("ref");
  const ref = tag === null ? "none" : REFS.has(tag) ? tag : "other";
  let refHost = "";
  try {
    const r = request.headers.get("referer");
    if (r) refHost = new URL(r).hostname;
  } catch {}
  if (refHost === url.host) refHost = ""; // internal navigation is not a referral
  const path = url.pathname.slice(0, 96); // index limit
  env.VIEWS.writeDataPoint({
    blobs: [path, path.startsWith("/ro/") ? "ro" : "en", refHost, ref, (request.cf && request.cf.country) || ""],
    doubles: [1],
    indexes: [path],
  });
}

// withScorecard replaces <table data-scorecard> on the colophon with the KV fragment (rendered and validated by CI, spec §7).
// The ETag becomes asset etag + fragment hash (written by scripts/scorecard-publish.sh), so conditional requests still get 304s (row 17).
// No KV value, or any failure: the built-in table (the same rows as of the last build) stays.
async function withScorecard(request, url, res, env) {
  if (url.pathname !== "/colophon/" || res.status !== 200 || !env.SCORECARD) return res;
  try {
    const [html, tag] = await Promise.all([env.SCORECARD.get("scorecard"), env.SCORECARD.get("scorecard.etag")]);
    if (!html) return res;
    const headers = new Headers(res.headers);
    headers.delete("content-length");
    const etag = `W/"${(res.headers.get("etag") || "").replace(/^W\/|"/g, "")}-${tag || "kv"}"`;
    headers.set("etag", etag);
    if (request.headers.get("if-none-match") === etag) return new Response(null, { status: 304, headers });
    return new HTMLRewriter()
      .on("table[data-scorecard]", { element(el) { el.replace(html, { html: true }); } })
      .transform(new Response(res.body, { status: res.status, headers }));
  } catch {
    return res;
  }
}

async function scorecardJSON(env) {
  const body = env.SCORECARD ? await env.SCORECARD.get("scorecard.json").catch(() => null) : null;
  if (!body) return new Response("Not found\n", { status: 404, headers: { "content-type": "text/plain; charset=utf-8" } });
  return new Response(body, { headers: { "content-type": "application/json; charset=utf-8", "cache-control": "public, max-age=300", "access-control-allow-origin": "*", "x-content-type-options": "nosniff" } });
}
```
About 110 lines. `writeDataPoint` is synchronous, but it runs inside `waitUntil` so a thrown error can never touch the response (the tests prove it). HTMLRewriter never sees a conditional request that matches: the 304 returns first.

- [ ] **Step 3: `wrangler.toml`** — bindings and the host variable. Create the KV namespace first (spec §6.4: wrangler owns the Worker's bindings):

```bash
. scripts/env.sh && npx --yes wrangler@4 kv namespace create SCORECARD
```
Expected: an `id = "…"` line. Append to `wrangler.toml`:
```toml
[vars]
PROD_HOST = "herinean.com"   # wrangler dev: --var PROD_HOST:127.0.0.1:8787 exercises the production path locally

# One datapoint per HTML view: path, lang, referrer host, ref, country, 1 — nothing else (spec §6.3).
[[analytics_engine_datasets]]
binding = "VIEWS"
dataset = "herinean_views"

# The colophon scorecard: an HTML fragment and its JSON, written by CI after each green post-deploy check (spec §7).
[[kv_namespaces]]
binding = "SCORECARD"
id = "<the id printed above>"
```
`worker/package.json`: `{ "name": "herinean-com-worker", "private": true, "type": "module" }` — no dependencies, ever; the file exists only so `node --test` parses `index.js` as a module.

`.githooks/pre-commit` gains, after the Go block:
```bash
if git diff --cached --name-only | grep -q '^worker/'; then
  (cd worker && node --test)
fi
```

- [ ] **Step 4: Run the tests, then the Worker locally against a real build**

```bash
(cd worker && node --test) && go run ./cmd/site build
npx --yes wrangler@4 dev --port 8787 --var PROD_HOST:127.0.0.1:8787 &
sleep 5
curl -sS -o /dev/null -w '/ %{http_code}\n' http://127.0.0.1:8787/
curl -sSI http://127.0.0.1:8787/ | grep -ci 'content-security-policy\|strict-transport' # 2: _headers applied by the asset layer
curl -sSI http://127.0.0.1:8787/ | grep -ci 'x-robots-tag'                              # 0: production path
curl -sS http://127.0.0.1:8787/colophon/ | grep -c 'data-scorecard'                      # 1: built-in table (KV empty)
curl -sS -o /dev/null -w 'scorecard.json %{http_code}\n' http://127.0.0.1:8787/colophon/scorecard.json   # 404
go run ./cmd/site scorecard --out .cache/scorecard.html && npx --yes wrangler@4 kv key put --binding SCORECARD scorecard --path .cache/scorecard.html --local && npx --yes wrangler@4 kv key put --binding SCORECARD scorecard.etag cafe0001 --local
curl -sS http://127.0.0.1:8787/colophon/ | grep -c '(stale)\|not yet run'                 # ≥1: the KV fragment replaced the table
et=$(curl -sSI http://127.0.0.1:8787/colophon/ | awk 'tolower($1)=="etag:"{print $2}' | tr -d '\r'); echo "etag $et"   # W/"…-cafe0001"
curl -sS -o /dev/null -w 'colophon revalidation %{http_code}\n' -H "If-None-Match: $et" http://127.0.0.1:8787/colophon/   # 304
curl -sS -o /dev/null -w 'writing (no slash) %{http_code} → %{redirect_url}\n' http://127.0.0.1:8787/writing
kill %1
```
Expected: `200`, `2`, `0`, `1`, `404`, `≥1`, an ETag ending in `-cafe0001"`, `304`, `301 → …/writing/`. (`wrangler dev` builds the request URL from the Host header, so `--var PROD_HOST:127.0.0.1:8787` is what makes the production branch run locally.)

- [ ] **Step 5: Commit** — `M1b: edge function — analytics without a tracker, colophon scorecard from KV, preview marking; node tests; bindings` with trailers; push.

---

### Task 4: Scripts — preview, verify, publish, analytics; the first preview

**Files:**
- Create: `scripts/preview.sh`, `scripts/deploy.sh`, `scripts/verify-preview.sh`, `scripts/scorecard-publish.sh`, `scripts/analytics.sh`
- Modify: `scripts/deploy-placeholder.sh` (comment: rollback tool until launch), `scripts/verify-edge.sh` (drop the two placeholder-only apex assertions)

- [ ] **Step 1: `scripts/preview.sh`**

```bash
#!/usr/bin/env bash
# Builds, checks and uploads a preview version (never production). Prints the preview URL. CI (M2) does the same on PRs.
set -euo pipefail
cd "$(dirname "$0")/.."
export PATH="$HOME/.local/bin:$PATH"
. scripts/env.sh
mkdir -p .cache
go run ./cmd/site build && go run ./cmd/site check --dist
out=$(npx --yes wrangler@4 versions upload 2>&1 | tee /dev/stderr)
url=$(printf '%s\n' "$out" | grep -Eo 'https://[a-z0-9-]+\.[a-z0-9-]+\.workers\.dev' | head -1)
[ -n "$url" ] || { echo "no preview URL in wrangler output" >&2; exit 1; }
echo "$url" | tee .cache/preview-url
```

- [ ] **Step 2: `scripts/deploy.sh`** — production; written now, run at launch (spec §11):

```bash
#!/usr/bin/env bash
# Production deploy. Refuses without --yes-production: cutover is a launch decision (scorecard green, About + portrait, ≥ 2 pieces).
set -euo pipefail
cd "$(dirname "$0")/.."
export PATH="$HOME/.local/bin:$PATH"
[ "${1:-}" = "--yes-production" ] || { echo "usage: scripts/deploy.sh --yes-production   (preview: scripts/preview.sh)" >&2; exit 2; }
. scripts/env.sh
go run ./cmd/site check && go run ./cmd/site build && go run ./cmd/site check --dist
npx --yes wrangler@4 deploy
scripts/verify-edge.sh && scripts/verify-preview.sh https://herinean.com --prod
```
`scripts/verify-edge.sh` stays edge-only. Its apex section carries two placeholder-era assertions that the real site fails by construction (`x-robots-tag: ^noindex$`; body free of `<script`, which JSON-LD is). Delete these two lines:
```
expect_header "$U/" x-robots-tag '^noindex$'                     # placeholder only; M1 removes it on production
expect_no_body "$U/" '<script|cdn-cgi|style='
```
and put in their place `# body, robots and script expectations differ between the placeholder and the site: scripts/verify-preview.sh [--prod] owns them`. Run `bash -n scripts/verify-edge.sh && scripts/verify-edge.sh | tail -1` → `121 passed, 0 failed` (two fewer than M0's 123).

`scripts/deploy-placeholder.sh`: change its header comment to `# Rollback to the M0 placeholder (one noindex line with the full header set). Production deploys are scripts/deploy.sh.`

- [ ] **Step 3: `scripts/verify-preview.sh URL`** — the same helper style as `verify-edge.sh`, against any base URL (preview or `http://127.0.0.1:8787`):

```bash
#!/usr/bin/env bash
# Verifies a built site at BASE (a preview URL or wrangler dev). Exit 1 if anything fails.
set -uo pipefail
BASE=${1:?usage: verify-preview.sh https://…workers.dev}; BASE=${BASE%/}
FAIL=0; PASS=0
ok()  { PASS=$((PASS+1)); printf '  ok   %s\n' "$1"; }
bad() { FAIL=$((FAIL+1)); printf '  FAIL %s\n' "$1"; }
hdr() { curl -sS -o /dev/null -D - --max-time 20 "$BASE$1" 2>/dev/null | awk -v h="$2" 'BEGIN{IGNORECASE=1} tolower($1)==tolower(h":"){sub(/^[^:]*: */,""); sub(/\r$/,""); print; exit}'; }
status() { curl -sS -o /dev/null -w '%{http_code}' --max-time 20 "$BASE$1" 2>/dev/null; }
expect_status() { local c; c=$(status "$1"); [ "$c" = "$2" ] && ok "$1 → $c" || bad "$1 expected $2 got $c"; }
expect_header() { local v; v=$(hdr "$1" "$2"); printf '%s' "$v" | grep -Eq -- "$3" && ok "$1 $2: $v" || bad "$1 $2 expected /$3/ got '${v:-<absent>}'"; }
expect_body() { curl -sS --max-time 20 "$BASE$1" | grep -Eq -- "$2" && ok "$1 body ~ /$2/" || bad "$1 body lacks /$2/"; }
expect_no_body() { curl -sS --max-time 20 "$BASE$1" | grep -Eq -- "$2" && bad "$1 body matches forbidden /$2/" || ok "$1 body free of /$2/"; }

printf '\n== pages\n'
for p in / /ro/ /writing/ /ro/articole/ /colophon/ /privacy/ /ro/confidentialitate/; do expect_status "$p" 200; done
expect_status /nope/ 404; expect_body /nope/ 'Pagina nu există'
expect_status /writing 301; expect_header /writing location '/writing/$'
printf '\n== headers (asset layer applies _headers; Worker leaves them alone)\n'
expect_header / content-security-policy "^default-src 'none'; style-src 'sha256-[A-Za-z0-9+/=]+'; img-src 'self'; font-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'$"
expect_header / strict-transport-security 'max-age=63072000; includeSubDomains; preload'
expect_header / cross-origin-resource-policy '^same-origin$'
expect_header / cache-control '^public, max-age=0, must-revalidate$'
expect_header / permissions-policy 'camera=\(\)'
expect_header / content-type '^text/html; charset=utf-8$'
printf '\n== fonts, images, feeds\n'
font=$(curl -sS --max-time 20 "$BASE/" | grep -Eo '<link rel="preload" href="[^"]+"' | grep -Eo '/fonts/[^"]+' | head -1)
[ -n "$font" ] && ok "preload found: $font" || bad "no font preload on /"
expect_header "$font" content-type '^font/woff2$'
expect_header "$font" cache-control '^public, max-age=31536000, immutable$'
og=$(curl -sS --max-time 20 "$BASE/" | grep -Eo '/og/[A-Za-z0-9.-]+\.png' | head -1)
expect_header "$og" cross-origin-resource-policy '^cross-origin$'
expect_header "$og" cache-control 'immutable'
expect_header /feed.xml content-type '^application/rss\+xml; charset=utf-8$'
expect_header /feed.json content-type '^application/feed\+json; charset=utf-8$'
expect_header /.well-known/security.txt content-type '^text/plain; charset=utf-8$'
printf '\n== privacy: no executable script, no cookie, no cdn-cgi\n'
for p in / /colophon/; do
  n=$(curl -sS --max-time 20 "$BASE$p" | grep -o '<script' | wc -l); [ "$n" = 1 ] && ok "$p has one <script> (JSON-LD)" || bad "$p has $n <script> tags"
  expect_no_body "$p" '/cdn-cgi/'
  c=$(hdr "$p" set-cookie); [ -z "$c" ] && ok "$p no Set-Cookie" || bad "$p sets a cookie: $c"
done
printf '\n== preview marking (skip with --prod)\n'
if [ "${2:-}" != "--prod" ]; then
  expect_header / x-robots-tag 'noindex'
  expect_body /robots.txt '^Disallow: /$'
else
  v=$(hdr / x-robots-tag); [ -z "$v" ] && ok "/ no x-robots-tag" || bad "/ carries x-robots-tag on production: $v"
  expect_body /robots.txt '^Sitemap: https://herinean.com/sitemap.xml$'
fi
printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"; [ "$FAIL" -eq 0 ]
```
(The `<script` count is the executable-script check: one tag, and `check --dist` already proved that one is JSON-LD.)

- [ ] **Step 4: `scripts/scorecard-publish.sh`** — the CI step (M2) in script form; usable by hand now:

```bash
#!/usr/bin/env bash
# Renders the scorecard fragment from scorecard.json (+ manual rows) and writes it and the JSON to KV. CI runs this after a green post-deploy check.
set -euo pipefail
cd "$(dirname "$0")/.."
export PATH="$HOME/.local/bin:$PATH"
. scripts/env.sh
IN=${1:-scorecard.json}
go run ./cmd/site scorecard --in "$IN" --manual data/scorecard-manual.yaml --out .cache/scorecard.html
grep -q '<table data-scorecard>' .cache/scorecard.html
npx --yes wrangler@4 kv key put --binding SCORECARD --remote scorecard --path .cache/scorecard.html
npx --yes wrangler@4 kv key put --binding SCORECARD --remote scorecard.etag "$(sha256sum .cache/scorecard.html | cut -c1-8)"   # the Worker's ETag suffix
[ -f "$IN" ] && npx --yes wrangler@4 kv key put --binding SCORECARD --remote scorecard.json --path "$IN"
echo "published: $(wc -c < .cache/scorecard.html) bytes"
```
(Nu validation of the fragment is M2's bench; until then the fragment is the same template the golden tests pin.)

- [ ] **Step 5: `scripts/analytics.sh`** — reads the live dataset (the KV snapshot merge is M2's weekly job):

```bash
#!/usr/bin/env bash
# Views from Analytics Engine: per piece (default), by referral tag or by country. Usage: analytics.sh [--days 30] [--by path|ref|country|referrer]
set -euo pipefail
cd "$(dirname "$0")/.."
. scripts/env.sh
DAYS=30; BY=path
while [ $# -gt 0 ]; do case "$1" in --days) DAYS=$2; shift 2;; --by) BY=$2; shift 2;; *) echo "usage: $0 [--days N] [--by path|ref|country|referrer]" >&2; exit 2;; esac; done
case "$BY" in path) COL=blob1;; ref) COL=blob4;; country) COL=blob5;; referrer) COL=blob3;; *) echo "unknown --by $BY" >&2; exit 2;; esac
# SUM(_sample_interval) estimates the true count under sampling; each row is one view.
SQL="SELECT $COL AS $BY, SUM(_sample_interval) AS views FROM herinean_views WHERE timestamp > NOW() - INTERVAL '$DAYS' DAY GROUP BY $BY ORDER BY views DESC FORMAT TabSeparatedWithNames"
curl -sS -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" --data "$SQL" "https://api.cloudflare.com/client/v4/accounts/$CLOUDFLARE_ACCOUNT_ID/analytics_engine/sql"
```

- [ ] **Step 6: Upload the preview and verify it**

```bash
chmod +x scripts/*.sh
scripts/preview.sh && scripts/verify-preview.sh "$(cat .cache/preview-url)"
```
Expected: a `…workers.dev` URL and `N passed, 0 failed`. Then publish the scorecard (manual rows + the honest "not yet run" row, since `scorecard.json` does not exist before M2) and confirm the Worker fills it on the preview:
```bash
scripts/scorecard-publish.sh; curl -sS "$(cat .cache/preview-url)/colophon/" | grep -c 'not yet run'   # 1
```
Read the preview on a phone and a laptop: fonts (Rendered Fonts panel shows Source Serif 4 / Newsreader, not the fallback), dark scheme, a piece with code and a table, the colophon table at 390 px. Analytics cannot be exercised on a preview by design (no datapoints off production); the node tests and the `wrangler dev` run in Task 3 are its evidence until launch.

- [ ] **Step 7: Commit** — `M1b: scripts — preview upload, production deploy (gated), preview verification, scorecard publish, analytics query` with trailers; push.

---

### Task 5: Docs, ADRs, PR

**Files:**
- Modify: `RUNBOOK.md` (Worker section rewritten; new Fonts and Analytics sections), `README.md` (status)
- Create: `docs/adr/0004-analytics-without-a-tracker.md`, `docs/adr/0010-self-hosted-fonts.md`

- [ ] **Step 1: RUNBOOK** — replace the `## Worker` section with:

```markdown
## Worker

- Preview (any branch): `scripts/preview.sh` → builds, runs `check --dist`, uploads a version, prints the `workers.dev` URL (noindex, no analytics). Verify: `scripts/verify-preview.sh <url>`.
- Production: `scripts/deploy.sh --yes-production` — launch criteria in spec §11; after M2, CI deploys `main` and this is the manual fallback.
- Rollback to the placeholder: `scripts/deploy-placeholder.sh`. Rollback to a previous version: `npx wrangler@4 rollback`.
- Local: `go run ./cmd/site build && npx wrangler@4 dev --port 8787 --var PROD_HOST:127.0.0.1:8787` (production path, local KV/AE); tests `cd worker && node --test`.
- Bindings live in `wrangler.toml`: `ASSETS` (dist/), `VIEWS` (Analytics Engine `herinean_views`), `SCORECARD` (KV). The KV namespace id is committed; recreate with `wrangler kv namespace create SCORECARD` and update the id if it is ever lost.
- Scorecard: `scripts/scorecard-publish.sh [scorecard.json]` writes the fragment + JSON to KV; the colophon reflects it on the next request. Empty KV → the built-in table from the last build.

## Fonts

- Shipped: `assets/fonts/web/*.woff2` (three files, ≤ 100 KB total, `BUILD.txt` records the exact inputs and fonttools version). Regenerate only when changing fonts: `scripts/fonts.sh` (needs `~/.local/share/fonttools`, see `scripts/setup.sh`), then paste `assets/fonts/web/fallback.css` into `site.css` — `site check` fails on drift.
- Fallbacks are Times New Roman / Liberation Serif with computed overrides; Android (no Times) gets a plain serif and a small shift — accepted, row 21 measures on Linux CI.

## Analytics

- `scripts/analytics.sh [--days 30|90] [--by path|ref|country|referrer]` — live Analytics Engine query (token needs Account Analytics: Read). Weekly snapshots to KV and the merge are M2.
- What is stored, verbatim from the privacy page: path, language, referring host, `ref` tag, country, 1. Nothing that can tell two readers apart.
```

- [ ] **Step 2: ADRs** (same format as `docs/adr/0011-tls13-minimum.md`: Status, Spec, Context, Decision, Consequences)

`docs/adr/0004-analytics-without-a-tracker.md`:
```markdown
# ADR-0004 — Analytics without a tracker

**Status:** accepted 2026-09-18 · **Spec:** §6.3, §5.4, §3 row 15

## Context
The site promises zero client-side JavaScript, zero cookies and zero third-party origins, and its readers are the people who ask vendors these questions. It still needs to know which pieces are read and from where, to decide what to translate and what to write next.

## Decision
One Analytics Engine datapoint per HTML response on the production host, written by the edge function inside `waitUntil`: `path, lang, referrer host, ref (li|x|nl|md|other|none), country, 1`. Nothing else is stored — no IP, no user agent, no full referrer, no identifier of any kind. Known bots are skipped by user agent; some still get counted. Preview hosts write nothing. The statement appears verbatim on the privacy page and the colophon, and `worker/index.test.mjs` pins the exact shape of the datapoint.

## Consequences
Numbers compare pieces; they do not measure reach or unique readers, and the colophon says so. Analytics Engine retains ~90 days; a weekly job snapshots per-piece totals to KV (M2). Referral attribution depends on the `?ref=` tag in links Radu posts; organic referrers are attributed by host only.
```

`docs/adr/0010-self-hosted-fonts.md`:
```markdown
# ADR-0010 — Self-hosted fonts with computed fallbacks

**Status:** accepted 2026-09-18 · **Spec:** §9, §3 rows 14 and 21 · supersedes the "system stack" option in the spec's not-now table

## Context
A system font stack costs nothing and can never shift layout, but the design (Claude Design pass, `docs/design/export-review.md`) is carried by Source Serif 4 and Newsreader. Web fonts risk two scorecard rows: weight (row 14) and layout shift on swap (row 21); a font without comma-below Romanian glyphs would silently show cedilla lookalikes.

## Decision
Three static instances (Source Serif 4 Regular and Italic; Newsreader Medium at optical size 36), subset to Latin, Latin Extended-A, U+0218–021B and the site's punctuation, WOFF2, ≤ 100 KB together, served content-hashed and immutable from the same origin, body regular preloaded, `font-display: swap`. Each family has a fallback face — Times New Roman / Liberation Serif — with `size-adjust` and vertical overrides computed from the two font files by `scripts/fontface` (not typed in), so the swap moves nothing measurable. No weight the files lack is used; `font-synthesis: none` enforces it. `site check` verifies the glyphs and that the CSS embeds the generated rules; `site check --dist` verifies size, preload and presence; the M2 bench measures the swap in Chromium.

## Consequences
Three requests per page for type (two on pages without italics). Android has neither Times nor Liberation, and Safari before 17.4 ignores the override descriptors: those readers see a plain serif for the swap window and a small shift — accepted, and measured nowhere. Changing a typeface is `scripts/fonts.sh` plus one paste and a golden refresh.
```

- [ ] **Step 3: README status** — under the status line: `M1b: edge function, fonts, preview — done; production stays on the placeholder until launch (spec §11).`

- [ ] **Step 4: Full gate, commit, PR**

```bash
go test ./... && go vet ./... && staticcheck ./... && (cd worker && node --test) && go run ./cmd/site build && go run ./cmd/site check --dist
git add -A && git commit -m "$(cat <<'MSG'
M1b: RUNBOOK, ADR-0004 analytics, ADR-0010 fonts, README status

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
MSG
)"
git push gitea m1b/edge && git push -u origin m1b/edge
gh pr create --base m1a/generator --title "M1b: edge function, fonts, preview" --body "$(cat <<'BODY'
Edge function with analytics (path, lang, referrer host, ref, country — nothing else), colophon scorecard from KV via HTMLRewriter, preview marking; node tests pin the datapoint shape. Self-hosted subset fonts (three files, ≤ 100 KB, Romanian glyphs verified) with metric-matched fallbacks computed from the font files; hashed /fonts/, one preload, dist invariants. Scripts for preview upload, gated production deploy, preview verification, scorecard publish and analytics queries. Preview URL: <paste from .cache/preview-url>; verify-preview: N passed, 0 failed.

Spec: §5.4, §6.3, §9, §3 rows 14/15/17/21. ADR-0004, ADR-0010.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

https://claude.ai/code/session_01CyBKVKXNbCCfWDPgYiMTKf
BODY
)"
```
`--base m1a/generator` while M1a is unmerged (stacked, like M1a on M0); otherwise `--base main`. Squash-merge after review. Next: M2 (bench, scorecard pipeline, post-deploy and weekly verify, uptime monitor), then launch.

---

## Self-review

**Spec coverage.** §5.4 edge function — Task 3 (hosts, analytics in `waitUntil`, bot skip, HTMLRewriter on `table[data-scorecard]`, `/colophon/scorecard.json`, preview noindex + robots, fall-through on error); the assets config (`force-trailing-slash`, `404-page`, `run_worker_first`) is M0's `wrangler.toml`, unchanged. §6.3 analytics fields and the privacy statement — Task 3 (tests pin them), `scripts/analytics.sh` live queries; snapshot + merge deferred to M2 as the spec says (weekly job). §6.4 — bindings in `wrangler.toml`, KV created by wrangler. §9 type — Task 1 (subsets, real italic, comma-below verified, one preload, swap, metric-matched fallback, `font-synthesis: none`); row 14 fonts ≤ 100 KB and one preload — Task 2 `check --dist`; row 21 first half — Task 1 `check` on the `.ttf` twins; row 21 second half (Playwright swap test) — M2 bench, with the manual DevTools check in Task 2 step 5; row 15 — `verify-preview.sh`; row 17 fonts immutable — M1a `_headers` + `verify-preview.sh`. §11 M1 = preview only — Task 4 (deploy gated). Decisions #4 and #10 — ADRs in Task 5.

**Placeholder scan.** `<the id printed above>` in `wrangler.toml` and `<paste …>` in the PR body and `site.css` are executor actions with the source of the value named in the same step. No TBD/TODO. Every code step carries the code.

**Type consistency.** `render.New(templatesDir, cssPath, fontURLs)` — Task 2 defines it; `build.go` and `scorecard.go` call it (three args); `render_test.go` passes `testFonts`. `site.WebFonts` — declared in Task 1 (`internal/site/fonts.go`), used by `check.go` (Task 1) and `build.go` (Task 2). `images.Hash8`, `images.CheckGlyphs` — M1a. `PageData.PreloadFont` — Task 2 struct field, template and `base()`. Worker `env.PROD_HOST`, `env.VIEWS`, `env.SCORECARD` — `wrangler.toml` bindings match `index.js` and the test harness; blob order `path, lang, referrer host, ref, country` matches `analytics.sh`'s `blob1…blob5`. Scripts read `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` from `scripts/env.sh` as M0 does.

**Adversarial review 2026-09-18, folded in.** `deploy.sh` verifies with both suites and `verify-edge.sh` loses its two placeholder-only assertions; `SOURCE_DATE_EPOCH` in `fonts.sh`; views counted on 200 and 304 by page path; `--layout-features+=` keeps pyftsubset's defaults; `fontface` reads OS/2 (typo when USE_TYPO_METRICS, warns on disagreement); `check --dist` estimates row 14 (requests and first-view bytes) and the M1a portrait drops to 320/480; colophon keeps a composite ETag and answers 304 itself; fonttools version read from Python; the feed Content-Type contingency names the `run_worker_first` change; link-preview fetchers in `BOT_UA`; UI-string runes checked against the body font; nosniff on the JSON; hook runs the Worker tests; `mkdir -p .cache`; SQL-permission check reads 400 correctly; token state recorded; preload grep targets the `<link>`; Safari < 17.4 stated in ADR-0010.

**Known limits stated, not hidden.** `fontface` follows USE_TYPO_METRICS; a font that leaves the bit unset and disagrees between hhea and usWin renders differently on Windows than the override assumes — the tool prints the three sets so the executor can see whether that applies (Source Serif 4 and Newsreader are expected to agree). `BOT_UA` is a heuristic; the colophon already says views include some bots. HTMLRewriter cannot run under `node --test`; the KV fill is verified with `wrangler dev` and on the preview.
