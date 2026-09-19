package site

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/raduherinean/herinean.com/internal/images"
)

func fmtHash(b []byte) string { return images.Hash8(b) }

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
	t.Setenv("SOURCE_DATE_EPOCH", "1790000000") // 2026-09-21T14:13:20Z
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

// The home page changes when a piece is published and when the About copy is edited: its sitemap lastmod is the
// later of the two. Outside git fileTime falls back to the build time, which the fixture pins after every piece date.
func TestHomeLastModFollowsAboutEdits(t *testing.T) {
	root := fixtureRoot(t)
	b, err := load(Options{Root: root, Out: "dist"})
	if err != nil {
		t.Fatal(err)
	}
	newest := b.site.Pieces[0].Date
	if !b.now.After(newest) {
		t.Fatalf("fixture: build time %s must be after the newest piece %s", b.now, newest)
	}
	for _, e := range b.sitemapEntries() {
		switch e.Loc {
		case b.cfg.Abs(b.cfg.HomeURL("en")), b.cfg.Abs(b.cfg.HomeURL("ro")):
			if !e.LastMod.Equal(b.now) {
				t.Errorf("%s lastmod = %s, want the _home.md time %s (later than the newest piece %s)", e.Loc, e.LastMod.Format("2006-01-02"), b.now.Format("2006-01-02"), newest.Format("2006-01-02"))
			}
		case b.cfg.Abs(b.cfg.IndexURL("en")), b.cfg.Abs(b.cfg.IndexURL("ro")):
			if !e.LastMod.Equal(newest) {
				t.Errorf("%s lastmod = %s, want the newest piece %s", e.Loc, e.LastMod.Format("2006-01-02"), newest.Format("2006-01-02"))
			}
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

// CheckDist must notice a _headers file that stopped covering a path class, and a page that smuggled in an
// inline handler, a quoted style attribute or a javascript: URL through raw HTML.
func TestCheckDistCatchesHeaderGapsAndAttributes(t *testing.T) {
	root := fixtureRoot(t)
	o := Options{Root: root, Out: "dist"}
	if err := Build(o); err != nil {
		t.Fatal(err)
	}
	dist := filepath.Join(root, "dist")
	h, err := os.ReadFile(filepath.Join(dist, "_headers"))
	if err != nil {
		t.Fatal(err)
	}
	// drop the /og/* block entirely and the RSS Content-Type from /feed.ro.xml
	blocks := strings.Split(string(h), "\n\n")
	var kept []string
	for _, b := range blocks {
		if strings.HasPrefix(b, "/og/*") {
			continue
		}
		if strings.HasPrefix(b, "/feed.ro.xml") {
			b = strings.ReplaceAll(b, "  Content-Type: application/rss+xml; charset=utf-8\n", "")
		}
		kept = append(kept, b)
	}
	// the HTML class (/*) is a path class too: drop HSTS and X-Frame-Options from it
	kept[0] = strings.ReplaceAll(kept[0], "  Strict-Transport-Security: max-age=63072000; includeSubDomains; preload\n", "")
	kept[0] = strings.ReplaceAll(kept[0], "  X-Frame-Options: DENY\n", "")
	kept[0] += "\n  Set-Cookie: x=1" // the site sets no cookie, in any block
	if err := os.WriteFile(filepath.Join(dist, "_headers"), []byte(strings.Join(kept, "\n\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	page := filepath.Join(dist, "privacy", "index.html")
	p, err := os.ReadFile(page)
	if err != nil {
		t.Fatal(err)
	}
	bad := `<p onclick="x()" style='color:red'><a href="javascript:void(0)">x</a></p>` +
		`<code>a=&#34; onclick=&#34;b&quot; style=&quot;c</code>` + // escaped code must not count
		`<section id="main"></section><section id="main"></section>` + // the page already has one id="main"
		`<style>p{}</style>` // a second <style> would escape the CSP hash
	p = []byte(strings.Replace(string(p), "</main>", bad+"</main>", 1))
	if err := os.WriteFile(page, p, 0o644); err != nil {
		t.Fatal(err)
	}
	// a second page carries the same attributes only inside code: goldmark escapes < > & " but leaves ' alone, so a
	// single-quoted sample in a code span or block is text the rules must skip (one finding per page, so it needs
	// its own page to be visible)
	codePage := filepath.Join(dist, "ro", "confidentialitate", "index.html")
	cp, err := os.ReadFile(codePage)
	if err != nil {
		t.Fatal(err)
	}
	codeOnly := `<code>&lt;a onclick='x' style='y' href='javascript:z'&gt;</code>` +
		`<pre tabindex="0"><code>&lt;p onclick='x' style='y'&gt;</code></pre>`
	cp = []byte(strings.Replace(string(cp), "</main>", codeOnly+"</main>", 1))
	if err := os.WriteFile(codePage, cp, 0o644); err != nil {
		t.Fatal(err)
	}

	err = CheckDist(o)
	if err == nil {
		t.Fatal("CheckDist() = nil, want header and attribute findings")
	}
	for _, want := range []string{
		"no rule for /* with Strict-Transport-Security: max-age=63072000; includeSubDomains; preload",
		"no rule for /* with X-Frame-Options: DENY",
		"no rule for /og/* with Cross-Origin-Resource-Policy: cross-origin",
		"no rule for /og/* with max-age=31536000, immutable",
		"no rule for /feed.ro.xml with Content-Type: application/rss+xml",
		"event handler attribute found",
		"style= attribute found",
		"javascript: URL found",
		`duplicate id "main"`,
		"expected exactly one <style>, found 2",
		"_headers: must not set a cookie",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("CheckDist() error lacks %q:\n%v", want, err)
		}
	}
	for _, rule := range []string{"event handler attribute found", "style= attribute found", "javascript: URL found"} {
		if n := strings.Count(err.Error(), rule); n != 1 {
			t.Errorf("%q reported %d times, want 1 (only the page with live attributes; code is text)", rule, n)
		}
		if strings.Contains(err.Error(), "ro/confidentialitate/index.html: "+rule) {
			t.Errorf("%q reported for the page whose only match is inside <code>", rule)
		}
	}
	if n := strings.Count(err.Error(), "duplicate id"); n != 2 {
		t.Errorf("duplicate id reported %d times, want 2 (one per repeat of the page's own id=\"main\", none for unique ids)", n)
	}
	if strings.Contains(err.Error(), "no rule for /feed.ro.xml with max-age=300") {
		t.Errorf("max-age=300 is still in the /feed.ro.xml block; it must not be reported:\n%v", err)
	}
}

// write() removes the output directory before the swap; an empty or "." Out would remove the repository root.
func TestBuildRefusesToWriteIntoRoot(t *testing.T) {
	root := fixtureRoot(t)
	for _, out := range []string{"", "."} {
		err := Build(Options{Root: root, Out: out})
		if err == nil || !strings.Contains(err.Error(), "refusing to write into the root") {
			t.Errorf("Out=%q: Build() error = %v, want the root guard", out, err)
		}
		if _, err := os.Stat(filepath.Join(root, "site.yaml")); err != nil {
			t.Fatalf("Out=%q: the root was touched: %v", out, err)
		}
	}
}

// Every rune of the prose must be in the web-font subset, front matter included; code is set in the system mono stack
// and is skipped, whether a span or a fenced block.
func TestCheckContentGlyphsAgainstTheSubset(t *testing.T) {
	root := fixtureRoot(t)
	piece := filepath.Join(root, "content", "en", "glyphs.md")
	front := func(title string) string {
		return "---\ntitle: \"" + title + "\"\ndate: 2026-09-02\nkey: glyphs\npillar: analysis\nsummary: \"A short summary.\"\n---\n"
	}
	write := func(s string) {
		t.Helper()
		if err := os.WriteFile(piece, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write(front("Glyphs") + "Prose with `x ≤ y` in a span,\n\n```text\na ≤ b\n```\n\nand ``a ` ≤ b`` in a longer one.\n")
	if err := Check(Options{Root: root}); err != nil {
		t.Fatalf("Check() = %v, want nil: ≤ appears only inside code", err)
	}

	write(front("Glyphs") + "Prose with x ≤ y in the open, ≤ twice.\n")
	err := Check(Options{Root: root})
	if err == nil {
		t.Fatal("Check() = nil, want the ≤ finding")
	}
	want := `glyphs.md:8: "≤" (U+2264) is not in the web-font subset; add it to UNICODES in scripts/fonts.sh or change the text`
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Check() error lacks %q:\n%v", want, err)
	}
	if n := strings.Count(err.Error(), "U+2264"); n != 1 {
		t.Errorf("≤ reported %d times for one line, want 1", n)
	}

	write(front("Glyphs ≤ titles") + "Plain prose.\n")
	err = Check(Options{Root: root})
	if err == nil || !strings.Contains(err.Error(), `glyphs.md:2: "≤" (U+2264)`) {
		t.Fatalf("Check() error = %v, want the front-matter title's ≤ at line 2", err)
	}
}

func TestWithoutMarkdownCode(t *testing.T) {
	in := "a `≤` b\n```go\nx ≤ y\n```\nc ``d ` ≤`` e\n~~~\n≤\n~~~\nf ` unclosed ≤\n"
	got := withoutMarkdownCode(in)
	if strings.Count(got, "\n") != strings.Count(in, "\n") {
		t.Fatalf("line count changed:\n%q", got)
	}
	lines := strings.Split(got, "\n")
	for i, l := range lines[:8] {
		if strings.Contains(l, "≤") {
			t.Errorf("line %d still carries code: %q", i+1, l)
		}
	}
	if !strings.Contains(lines[8], "≤") {
		t.Errorf("an unclosed backtick is literal text; line 9 = %q", lines[8])
	}
}

// CheckDist must fire on each row-14 gap: the font budget, the single preload and its target, the CSS font urls, the
// request count and the /fonts/* cache rule.
func TestCheckDistCatchesFontAndRequestGaps(t *testing.T) {
	root := fixtureRoot(t)
	if err := Build(Options{Root: root, Out: "dist"}); err != nil {
		t.Fatal(err)
	}
	edit := func(t *testing.T, path string, f func(string) string) {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		out := f(string(b))
		if out == string(b) {
			t.Fatalf("%s: the edit changed nothing", path)
		}
		if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	page := filepath.Join("privacy", "index.html")
	preloadTag := regexp.MustCompile(`<link rel="preload"[^>]*>`)
	cases := []struct {
		name    string
		corrupt func(t *testing.T, dist string)
		want    []string
	}{
		{"fonts total out of range", func(t *testing.T, dist string) {
			if err := os.RemoveAll(filepath.Join(dist, "fonts")); err != nil {
				t.Fatal(err)
			}
		}, []string{"fonts: shipped fonts total 0 bytes; want 1..102400 (row 14)"}},
		{"two preloads", func(t *testing.T, dist string) {
			edit(t, filepath.Join(dist, page), func(s string) string { tag := preloadTag.FindString(s); return strings.Replace(s, tag, tag+tag, 1) })
		}, []string{"privacy/index.html: 2 preloads; exactly one (the body regular) is allowed"}},
		{"preload target missing", func(t *testing.T, dist string) {
			edit(t, filepath.Join(dist, page), func(s string) string {
				tag := preloadTag.FindString(s)
				return strings.Replace(s, tag, strings.Replace(tag, `href="/fonts/`, `href="/fonts/nope-`, 1), 1)
			})
		}, []string{"privacy/index.html: preload target /fonts/nope-", "is not a shipped font"}},
		{"CSS url target missing", func(t *testing.T, dist string) {
			edit(t, filepath.Join(dist, page), func(s string) string { return strings.Replace(s, "url(/fonts/", "url(/fonts/nope-", 1) })
		}, []string{"privacy/index.html: CSS references /fonts/nope-", "which is not in dist"}},
		{"more than six requests", func(t *testing.T, dist string) {
			edit(t, filepath.Join(dist, page), func(s string) string {
				return strings.Replace(s, "</main>", strings.Repeat(`<img src="/x.png" alt="">`, 5)+"</main>", 1)
			})
		}, []string{"privacy/index.html: estimated first-view requests", "> 6 (row 14): document, favicon, ", "5 eager images"}},
		{"the /fonts/* cache rule missing", func(t *testing.T, dist string) {
			edit(t, filepath.Join(dist, "_headers"), func(s string) string {
				return strings.Replace(s, "/fonts/*\n  ! Cache-Control\n  Cache-Control: public, max-age=31536000, immutable\n", "/fonts/*\n  ! Cache-Control\n", 1)
			})
		}, []string{"_headers: no rule for /fonts/* with Cache-Control: public, max-age=31536000, immutable"}},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := fmt.Sprintf("dist-%d", i)
			copyDir(t, filepath.Join(root, "dist"), filepath.Join(root, out))
			tc.corrupt(t, filepath.Join(root, out))
			err := CheckDist(Options{Root: root, Out: out})
			if err == nil {
				t.Fatal("CheckDist() = nil, want a finding")
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("CheckDist() error lacks %q:\n%v", want, err)
				}
			}
		})
	}
}
