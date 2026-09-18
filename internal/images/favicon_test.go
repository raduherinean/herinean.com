package images

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"strings"
	"testing"
)

func TestFaviconSVG(t *testing.T) {
	fontBytes, err := os.ReadFile("../../assets/fonts/og/Newsreader72pt-Medium.ttf")
	if err != nil {
		t.Fatal(err)
	}
	svg, err := FaviconSVG(fontBytes)
	if err != nil {
		t.Fatal(err)
	}
	s := string(svg)
	if !strings.Contains(s, "<path") {
		t.Errorf("SVG has no <path>: %s", s)
	}
	if !strings.Contains(s, `fill="#FAF8F4"`) {
		t.Errorf("SVG missing fill=\"#FAF8F4\": %s", s)
	}
	if !strings.Contains(s, `fill="#7D2B22"`) {
		t.Errorf("SVG missing oxblood background: %s", s)
	}
}

func TestFaviconSVGNoGlyph(t *testing.T) {
	if _, err := FaviconSVG([]byte("not a font")); err == nil {
		t.Fatal("want error for invalid font bytes")
	}
}

func TestFaviconRaster(t *testing.T) {
	f, err := LoadFonts("../../assets/fonts/og")
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{32, 180} {
		b, err := FaviconRaster(f, size)
		if err != nil {
			t.Fatalf("size %d: %v", size, err)
		}
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatalf("size %d: decode: %v", size, err)
		}
		if bounds := img.Bounds(); bounds.Dx() != size || bounds.Dy() != size {
			t.Errorf("size %d: got %dx%d", size, bounds.Dx(), bounds.Dy())
		}
	}
}

func TestICO(t *testing.T) {
	f, err := LoadFonts("../../assets/fonts/og")
	if err != nil {
		t.Fatal(err)
	}
	p32, err := FaviconRaster(f, 32)
	if err != nil {
		t.Fatal(err)
	}
	ico := ICO(p32, 32)
	want := []byte{0x00, 0x00, 0x01, 0x00, 0x01, 0x00}
	if !bytes.HasPrefix(ico, want) {
		t.Errorf("ICO header = % x, want prefix % x", ico[:6], want)
	}
	// the PNG payload must decode back to a 32x32 image, image data starts at offset 22
	img, _, err := image.Decode(bytes.NewReader(ico[22:]))
	if err != nil {
		t.Fatalf("ICO payload does not decode as an image: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 32 || b.Dy() != 32 {
		t.Errorf("ICO payload size = %v, want 32x32", b)
	}
}
