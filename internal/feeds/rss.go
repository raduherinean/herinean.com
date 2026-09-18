// Package feeds writes RSS 2.0 and JSON Feed 1.1 with full content.
package feeds

import (
	"bytes"
	"encoding/xml"
	"time"

	"github.com/raduherinean/herinean.com/internal/config"
	"github.com/raduherinean/herinean.com/internal/content"
)

type rss struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Atom    string   `xml:"xmlns:atom,attr"`
	Content string   `xml:"xmlns:content,attr"`
	Channel channel  `xml:"channel"`
}

type channel struct {
	Title         string   `xml:"title"`
	Link          string   `xml:"link"`
	Description   string   `xml:"description"`
	Language      string   `xml:"language,omitempty"`
	LastBuildDate string   `xml:"lastBuildDate"`
	AtomLink      atomLink `xml:"atom:link"`
	Items         []item   `xml:"item"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        guid   `xml:"guid"`
	PubDate     string `xml:"pubDate"`
	Category    string `xml:"category"`
	Description string `xml:"description"`
	Content     string `xml:"content:encoded"`
}

type guid struct {
	IsPermaLink bool   `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

// pub places a date-only publication at 06:00 Bucharest so pubDate is stable and sensible.
func pub(t time.Time) string { return t.Add(6 * time.Hour).Format(time.RFC1123Z) }

func RSS(cfg *config.Config, s *content.Site, lang string, now time.Time) ([]byte, error) {
	pieces := s.Pieces
	title, desc := cfg.Name, cfg.Tagline["en"]
	if lang != "" {
		pieces = s.ByLang[lang]
		desc = cfg.Tagline[lang]
	}
	ch := channel{Title: title, Link: cfg.Abs(cfg.HomeURL(lang)), Description: desc, Language: lang,
		LastBuildDate: now.Format(time.RFC1123Z), AtomLink: atomLink{Href: cfg.Abs(cfg.FeedURL(lang)), Rel: "self", Type: "application/rss+xml"}}
	if lang == "" {
		ch.Link = cfg.Abs("/")
	}
	for _, p := range pieces {
		u := cfg.Abs(cfg.PieceURL(p.Lang, p.Slug))
		ch.Items = append(ch.Items, item{Title: p.Title, Link: u, GUID: guid{IsPermaLink: true, Value: u}, PubDate: pub(p.Date),
			Category: s.T(p.Lang, "pillar."+p.Pillar), Description: p.Summary, Content: Absolutize(string(p.Body), cfg.BaseURL, u)})
	}
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(rss{Version: "2.0", Atom: "http://www.w3.org/2005/Atom", Content: "http://purl.org/rss/1.0/modules/content/", Channel: ch}); err != nil {
		return nil, err
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}
