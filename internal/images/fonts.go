package images

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
)

// RequiredRunes: Romanian comma-below forms and the three other diacritics, both cases. A font missing any of these
// would let the browser substitute a cedilla lookalike, which is exactly the failure the spec forbids.
var RequiredRunes = []rune("ȘșȚțĂăÂâÎî")

// OGFontFiles live in assets/fonts/og/: static instances (see Task 5 step 1).
var OGFontFiles = []string{"Newsreader72pt-Medium.ttf", "SourceSerif4-Regular.ttf"}

type Fonts struct {
	Display *opentype.Font // Newsreader 72pt Medium: titles
	Body    *opentype.Font // Source Serif 4 Regular: name, labels
	Hash    string         // of both files; part of every OG cache key
}

func CheckGlyphs(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	f, err := sfnt.Parse(b)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	var buf sfnt.Buffer
	var missing []rune
	for _, r := range RequiredRunes {
		gi, err := f.GlyphIndex(&buf, r)
		if err != nil || gi == 0 {
			missing = append(missing, r)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s lacks glyphs for %q", path, string(missing))
	}
	return nil
}

func LoadFonts(dir string) (*Fonts, error) {
	var all []byte
	load := func(name string) (*opentype.Font, error) {
		p := filepath.Join(dir, name)
		if err := CheckGlyphs(p); err != nil {
			return nil, err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		all = append(all, b...)
		return opentype.Parse(b)
	}
	d, err := load(OGFontFiles[0])
	if err != nil {
		return nil, err
	}
	b, err := load(OGFontFiles[1])
	if err != nil {
		return nil, err
	}
	return &Fonts{Display: d, Body: b, Hash: Hash8(all)}, nil
}
