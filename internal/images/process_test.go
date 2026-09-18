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
