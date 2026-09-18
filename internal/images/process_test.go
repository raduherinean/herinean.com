package images

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gen2brain/webp"
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

func tmpJPEG(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 64, 255})
		}
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "photo.jpg")
	if err := os.WriteFile(p, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestWebPEncoderIsPureGo documents the assumption the reproducibility guard in Process relies
// on: gen2brain/webp must not have dlopen'd a host libwebp on this machine, or encoded bytes
// would depend on the host's libwebp version. This fails loudly the day that assumption breaks
// (e.g. a dynamically linked test binary on a machine with libwebp installed).
func TestWebPEncoderIsPureGo(t *testing.T) {
	if webp.Dynamic() == nil {
		t.Fatal("a host libwebp was loaded; webp output on this machine would not be reproducible (build with -tags nodynamic or CGO_ENABLED=0)")
	}
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

// assertFilesEqual fails if a and b don't hold exactly the same paths with byte-identical content.
func assertFilesEqual(t *testing.T, a, b map[string][]byte) {
	t.Helper()
	if len(a) != len(b) {
		t.Fatalf("file count differs: %d vs %d", len(a), len(b))
	}
	for p, ab := range a {
		bb, ok := b[p]
		if !ok {
			t.Errorf("%s missing from second run", p)
			continue
		}
		if !bytes.Equal(ab, bb) {
			t.Errorf("%s differs between two runs on the same input", p)
		}
	}
}

func TestProcessIsByteReproducible(t *testing.T) {
	src := tmpPNG(t, 1600, 1000)
	a, err := Process(src, "media", "fig.png", Options{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Process(src, "media", "fig.png", Options{})
	if err != nil {
		t.Fatal(err)
	}
	assertFilesEqual(t, a.Files, b.Files) // covers both WebP variants and the PNG fallback
}

func TestProcessIsByteReproducibleJPEG(t *testing.T) {
	src := tmpJPEG(t, 1600, 1000)
	a, err := Process(src, "media", "photo.jpg", Options{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Process(src, "media", "photo.jpg", Options{})
	if err != nil {
		t.Fatal(err)
	}
	assertFilesEqual(t, a.Files, b.Files) // covers both WebP variants and the JPEG fallback
}

func TestProcessWidthsOrderIndependent(t *testing.T) {
	src := tmpPNG(t, 1000, 1000)
	asc, err := Process(src, "home", "portrait.png", Options{Widths: []int{320, 640}})
	if err != nil {
		t.Fatal(err)
	}
	desc, err := Process(src, "home", "portrait.png", Options{Widths: []int{640, 320}})
	if err != nil {
		t.Fatal(err)
	}
	if asc.Srcset != desc.Srcset {
		t.Errorf("width order changed Srcset: %q vs %q", asc.Srcset, desc.Srcset)
	}
	if asc.Width != desc.Width || asc.Height != desc.Height {
		t.Errorf("width order changed dimensions: %+v vs %+v", asc, desc)
	}
}
