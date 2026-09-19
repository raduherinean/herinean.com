package render

import (
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/raduherinean/herinean.com/internal/config"
	"github.com/raduherinean/herinean.com/internal/content"
)

type Alternate struct{ Lang, Href string }

type Entry struct {
	URL, Title, Summary, Lang, Badge, LangName, PillarText, DateISO, DateText string
	Foreign                                                                   bool
}

type YearGroup struct {
	Year    int
	Entries []Entry
}

type ScorecardRow struct {
	Check, Value, When, Link, LinkText string
	Pass, Stale                        bool
}

type PageData struct {
	Cfg         *config.Config
	Site        *content.Site
	Lang        string
	Kind        string
	Title       string // og:title / h1
	HeadTitle   string // <title>
	Description string
	Path        string // site-relative, with trailing slash
	Canonical   string
	Alternates  []Alternate
	OGImage     string
	OGType      string
	OGLocale    string
	Robots      string
	JSONLD      template.JS
	CSS         template.CSS
	PreloadFont string // absolute-path URL of the body regular; base emits one <link rel="preload"> when set
	Body        template.HTML
	Tagline     string

	Piece                                                  *content.Piece
	Page                                                   *content.Page
	Translation                                            *Entry
	DateISO, DateText, UpdatedISO, UpdatedText, PillarText string
	Entries                                                []Entry
	Years                                                  []YearGroup
	Portrait                                               *content.ImageInfo

	ScorecardHTML                      template.HTML
	Deps, Commit, BuildDate, GoVersion string
}

func (d *PageData) SiteName() string     { return d.Cfg.Name }
func (d *PageData) HomeURL() string      { return d.Cfg.HomeURL(d.Lang) }
func (d *PageData) IndexURL() string     { return d.Cfg.IndexURL(d.Lang) }
func (d *PageData) PrivacyURL() string   { return d.Cfg.PrivacyURL(d.Lang) }
func (d *PageData) FeedURL() string      { return d.Cfg.FeedURL(d.Lang) }
func (d *PageData) OtherHomeURL() string { return d.Cfg.HomeURL(other(d.Lang)) }
func (d *PageData) T(key string) string  { return d.Site.T(d.Lang, key) }

func other(lang string) string {
	if lang == "ro" {
		return "en"
	}
	return "ro"
}

// DateText formats a date the way each language reads it: "27 Sep 2026" / "27 sept. 2026".
func DateText(s *content.Site, lang string, t time.Time) string {
	months := strings.Split(s.T(lang, "date.months_short"), ",")
	return fmt.Sprintf("%d %s %d", t.Day(), months[t.Month()-1], t.Year())
}

func EntryFor(cfg *config.Config, s *content.Site, viewerLang string, p *content.Piece) Entry {
	return Entry{
		URL: cfg.PieceURL(p.Lang, p.Slug), Title: p.Title, Summary: p.Summary, Lang: p.Lang,
		Badge: s.T(viewerLang, "lang.badge."+p.Lang), LangName: s.T(viewerLang, "lang."+p.Lang),
		PillarText: s.T(viewerLang, "pillar."+p.Pillar),
		DateISO:    p.Date.Format("2006-01-02"), DateText: DateText(s, viewerLang, p.Date),
		Foreign: p.Lang != viewerLang,
	}
}

func Years(cfg *config.Config, s *content.Site, viewerLang string, ps []*content.Piece) []YearGroup {
	var out []YearGroup
	for _, p := range ps {
		y := p.Date.Year()
		if len(out) == 0 || out[len(out)-1].Year != y {
			out = append(out, YearGroup{Year: y})
		}
		out[len(out)-1].Entries = append(out[len(out)-1].Entries, EntryFor(cfg, s, viewerLang, p))
	}
	return out
}

// AlternatesFor returns hreflang links for a page that exists in both languages (or just itself).
func AlternatesFor(cfg *config.Config, enPath, roPath string) []Alternate {
	var alts []Alternate
	if enPath != "" {
		alts = append(alts, Alternate{"en", cfg.Abs(enPath)})
	}
	if roPath != "" {
		alts = append(alts, Alternate{"ro", cfg.Abs(roPath)})
	}
	if enPath != "" {
		alts = append(alts, Alternate{"x-default", cfg.Abs(enPath)})
	} else if roPath != "" {
		alts = append(alts, Alternate{"x-default", cfg.Abs(roPath)})
	}
	return alts
}
