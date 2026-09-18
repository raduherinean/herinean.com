package seo

import (
	"fmt"
	"strings"
	"time"

	"github.com/raduherinean/herinean.com/internal/config"
	"github.com/raduherinean/herinean.com/internal/content"
)

func Robots(cfg *config.Config) []byte {
	return []byte("User-agent: *\nAllow: /\n\nSitemap: " + cfg.Abs("/sitemap.xml") + "\n")
}

// SecurityTxt per RFC 9116. Expires is derived from the build (commit) time so the file is reproducible; the weekly job warns 30 days before.
func SecurityTxt(cfg *config.Config, build time.Time) []byte {
	exp := build.UTC().Add(364 * 24 * time.Hour).Format("2006-01-02T15:04:05.000Z")
	return []byte(fmt.Sprintf("Contact: mailto:%s\nExpires: %s\nPreferred-Languages: en, ro\nCanonical: %s\n", cfg.Author.Email, exp, cfg.Abs("/.well-known/security.txt")))
}

// LLMs follows llmstxt.org: a title, a blockquote summary, then sections of links with one-line descriptions.
func LLMs(cfg *config.Config, s *content.Site) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n> %s\n\n", cfg.Name, cfg.Tagline["en"])
	fmt.Fprintf(&b, "%s\n\n", cfg.AIDisclosure["en"])
	for _, lang := range content.Langs {
		if len(s.ByLang[lang]) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s (%s)\n\n", s.T(lang, "index.title"), s.T("en", "lang."+lang))
		for _, p := range s.ByLang[lang] {
			fmt.Fprintf(&b, "- [%s](%s): %s\n", p.Title, cfg.Abs(cfg.PieceURL(p.Lang, p.Slug)), p.Summary)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "## Optional\n\n- [Colophon](%s): how the site is built and audited\n- [RSS](%s)\n", cfg.Abs("/colophon/"), cfg.Abs("/feed.xml"))
	return []byte(b.String())
}
