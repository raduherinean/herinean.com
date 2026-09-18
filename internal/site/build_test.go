package site

import (
	"os"
	"path/filepath"
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
