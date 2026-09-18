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
