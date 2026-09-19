// Package seo writes the machine-readable files: sitemap, robots, llms.txt, security.txt.
package seo

import (
	"bytes"
	"encoding/xml"
	"time"
)

type Alt struct{ Lang, Href string }

type URLEntry struct {
	Loc        string
	LastMod    time.Time
	Alternates []Alt
}

func Sitemap(entries []URLEntry) []byte {
	var b bytes.Buffer
	b.WriteString(xml.Header)
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">` + "\n")
	for _, e := range entries {
		b.WriteString("  <url>\n    <loc>")
		xml.EscapeText(&b, []byte(e.Loc))
		b.WriteString("</loc>\n    <lastmod>" + e.LastMod.Format("2006-01-02") + "</lastmod>\n")
		for _, a := range e.Alternates {
			b.WriteString(`    <xhtml:link rel="alternate" hreflang="` + a.Lang + `" href="`)
			xml.EscapeText(&b, []byte(a.Href))
			b.WriteString(`"/>` + "\n")
		}
		b.WriteString("  </url>\n")
	}
	b.WriteString("</urlset>\n")
	return b.Bytes()
}
