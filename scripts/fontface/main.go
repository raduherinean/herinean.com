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
	useTypo                                     bool
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
