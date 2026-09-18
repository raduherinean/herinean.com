package site

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/raduherinean/herinean.com/internal/content"
	"github.com/raduherinean/herinean.com/internal/images"
	"github.com/raduherinean/herinean.com/internal/render"
	"golang.org/x/image/font/sfnt"
)

// checkPlaceholders fails on author-input markers left anywhere in content/ or site.yaml.
func checkPlaceholders(root string) content.Problems {
	var probs content.Problems
	_ = filepath.WalkDir(filepath.Join(root, "content"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") || strings.HasSuffix(p, "_template.md") {
			return nil
		}
		b, _ := os.ReadFile(p)
		for i, line := range strings.Split(string(b), "\n") {
			if strings.ContainsAny(line, "⟨⟩") {
				probs.Add(p, i+1, "author placeholder ⟨…⟩ still present")
			}
		}
		return nil
	})
	return probs
}

// Check validates the sources without writing. Placeholders alone are ErrAuthorInputs (exit 3); any other problem,
// with or without placeholders around, is exit 1 and names the placeholders after it.
func Check(o Options) error {
	b, err := load(o)
	if err != nil && !errors.Is(err, ErrAuthorInputs) {
		return err
	}
	author := err
	var probs content.Problems
	// templates parse and the CSS reads: a broken template is a real problem the pre-commit hook must catch, not
	// something only `site build` discovers later
	if _, err := render.New(filepath.Join(o.Root, "templates"), filepath.Join(o.Root, "assets", "css", "site.css")); err != nil {
		probs.Add("templates", 0, "%v", err)
	}
	// images exist and are processable
	for key, refs := range b.site.AllImageRefs() {
		for _, ref := range refs {
			src := filepath.Join(o.Root, "assets", "img", key, ref.Dest)
			if _, err := os.Stat(src); err != nil {
				probs.Add(src, ref.Line, "referenced image is missing")
			}
		}
	}
	// the portrait is an author input (spec §14): missing is exit 3, not a real failure. Build still refuses without it.
	if _, err := os.Stat(filepath.Join(o.Root, "assets", "portrait.jpg")); err != nil {
		author = joinAuthor(author, fmt.Errorf("%w: assets/portrait.jpg is missing (the home page needs a portrait)", ErrAuthorInputs))
	}
	// fonts carry the Romanian glyphs
	for _, f := range images.OGFontFiles {
		if err := images.CheckGlyphs(filepath.Join(o.Root, "assets", "fonts", "og", f)); err != nil {
			probs.Add(filepath.Join("assets", "fonts", "og", f), 0, "%v", err)
		}
	}
	// Web fonts: the subset twins must carry the Romanian glyphs, and site.css must embed the generated fallback rules verbatim.
	for _, name := range WebFonts {
		if err := images.CheckGlyphs(filepath.Join(o.Root, "assets", "fonts", "web", name+".ttf")); err != nil {
			probs.Add(filepath.Join("assets", "fonts", "web", name+".ttf"), 0, "%v", err)
		}
		if _, err := os.Stat(filepath.Join(o.Root, "assets", "fonts", "web", name+".woff2")); err != nil {
			probs.Add(filepath.Join("assets", "fonts", "web", name+".woff2"), 0, "missing; run scripts/fonts.sh")
		}
	}
	// Every character the UI strings use must have a glyph in the body font, or the fallback draws it (a different "→" on every page).
	if body, err := os.ReadFile(filepath.Join(o.Root, "assets", "fonts", "web", WebFonts[0]+".ttf")); err == nil {
		if f, err := sfnt.Parse(body); err == nil {
			var buf sfnt.Buffer
			for lang, m := range b.site.Strings {
				for key, v := range m {
					for _, r := range v {
						if unicode.IsSpace(r) {
							continue
						}
						if gi, err := f.GlyphIndex(&buf, r); err != nil || gi == 0 {
							probs.Add(filepath.Join("i18n", lang+".yaml"), 0, "%s uses %q (U+%04X), which the body font lacks; add it to UNICODES in scripts/fonts.sh or change the string", key, string(r), r)
						}
					}
				}
			}
		}
	}
	css, _ := os.ReadFile(filepath.Join(o.Root, "assets", "css", "site.css"))
	fb, err := os.ReadFile(filepath.Join(o.Root, "assets", "fonts", "web", "fallback.css"))
	if err != nil {
		probs.Add("assets/fonts/web/fallback.css", 0, "missing; run scripts/fonts.sh")
	} else {
		for i, line := range strings.Split(strings.TrimSpace(string(fb)), "\n") {
			if !strings.Contains(string(css), line) {
				probs.Add("assets/css/site.css", 0, "fallback rule %d differs from assets/fonts/web/fallback.css; paste it verbatim", i+1)
			}
		}
	}
	// internal links resolve to pages or files the build will produce
	known := b.knownPaths()
	for _, p := range b.site.Pieces {
		for _, l := range p.Links {
			if !known[l.Dest] {
				probs.Add(p.File, l.Line, "internal link %s does not resolve", l.Dest)
			}
		}
	}
	for _, pg := range b.site.Pages {
		for _, l := range pg.Links() {
			if !known[l.Dest] {
				probs.Add(pg.File, l.Line, "internal link %s does not resolve", l.Dest)
			}
		}
	}
	if err := probs.Err(); err != nil {
		if author != nil {
			return fmt.Errorf("%v\n%v", err, author)
		}
		return err
	}
	return author
}

// joinAuthor accumulates author-input errors. errors.Is(result, ErrAuthorInputs) holds whenever it holds for any member,
// and every member keeps its own text, one per line.
func joinAuthor(existing, add error) error {
	if existing == nil {
		return add
	}
	return errors.Join(existing, add)
}

// knownPaths is every URL path the build emits (pages with trailing slash, plus machine files).
func (b *build) knownPaths() map[string]bool {
	cfg, s := b.cfg, b.site
	k := map[string]bool{"/feed.xml": true, "/feed.json": true, "/sitemap.xml": true, "/robots.txt": true, "/llms.txt": true, "/colophon/": true, "/404.html": true, "/.well-known/security.txt": true}
	for _, lang := range content.Langs {
		k[cfg.HomeURL(lang)], k[cfg.IndexURL(lang)], k[cfg.PrivacyURL(lang)], k[cfg.FeedURL(lang)] = true, true, true, true
	}
	for _, p := range s.Pieces {
		k[cfg.PieceURL(p.Lang, p.Slug)] = true
	}
	return k
}
