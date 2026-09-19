package seo

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/raduherinean/herinean.com/internal/config"
)

func cfg(t *testing.T) *config.Config {
	c, err := config.Load("../../testdata/site/site.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestSitemap(t *testing.T) {
	d := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	b := Sitemap([]URLEntry{{Loc: "https://herinean.com/writing/x/", LastMod: d, Alternates: []Alt{{"en", "https://herinean.com/writing/x/"}, {"ro", "https://herinean.com/ro/articole/y/"}, {"x-default", "https://herinean.com/writing/x/"}}}})
	var doc struct {
		XMLName xml.Name `xml:"urlset"`
		URLs    []struct {
			Loc     string `xml:"loc"`
			LastMod string `xml:"lastmod"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if doc.URLs[0].LastMod != "2026-09-27" || !strings.Contains(s, `xmlns:xhtml="http://www.w3.org/1999/xhtml"`) || !strings.Contains(s, `<xhtml:link rel="alternate" hreflang="x-default" href="https://herinean.com/writing/x/"/>`) {
		t.Errorf("%s", s)
	}
}

func TestFiles(t *testing.T) {
	c := cfg(t)
	if r := string(Robots(c)); !strings.Contains(r, "Sitemap: https://herinean.com/sitemap.xml") || !strings.Contains(r, "Allow: /") {
		t.Errorf("robots: %s", r)
	}
	b := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	st := string(SecurityTxt(c, b))
	if !strings.Contains(st, "Contact: mailto:security@herinean.com") || !strings.Contains(st, "Expires: 2027-09-26T10:00:00.000Z") || !strings.Contains(st, "Canonical: https://herinean.com/.well-known/security.txt") {
		t.Errorf("security.txt: %s", st)
	}
}
