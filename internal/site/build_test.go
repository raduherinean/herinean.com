package site

import (
	"os"
	"path/filepath"
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

	err = CheckDist(o)
	if err == nil {
		t.Fatal("CheckDist() = nil, want header and attribute findings")
	}
	for _, want := range []string{
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
	if n := strings.Count(err.Error(), "event handler attribute found"); n != 1 {
		t.Errorf("event handler reported %d times, want 1 (escaped code must not count)", n)
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
