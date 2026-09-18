package site

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/raduherinean/herinean.com/internal/content"
	"github.com/raduherinean/herinean.com/internal/images"
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
	// images exist and are processable
	for key, refs := range b.site.AllImageRefs() {
		for _, ref := range refs {
			src := filepath.Join(o.Root, "assets", "img", key, ref.Dest)
			if _, err := os.Stat(src); err != nil {
				probs.Add(src, ref.Line, "referenced image is missing")
			}
		}
	}
	if _, err := os.Stat(filepath.Join(o.Root, "assets", "portrait.jpg")); err != nil {
		probs.Add("assets/portrait.jpg", 0, "the home page needs a portrait")
	}
	// fonts carry the Romanian glyphs
	for _, f := range images.OGFontFiles {
		if err := images.CheckGlyphs(filepath.Join(o.Root, "assets", "fonts", "og", f)); err != nil {
			probs.Add(filepath.Join("assets", "fonts", "og", f), 0, "%v", err)
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
