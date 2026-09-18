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
