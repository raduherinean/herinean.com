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
	// ogLayout is the card's drawing version, folded into the cache root (cacheVersion): RenderOG's key covers every
	// input but not this code, so bump it whenever the drawing changes, or a warm .cache/ keeps serving the old card.
	ogLayout = 1
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
	return strings.Join(strings.Split(s, ""), " ")
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
