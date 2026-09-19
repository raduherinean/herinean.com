package site

import (
	"html/template"
	"strings"

	"github.com/raduherinean/herinean.com/internal/content"
	"github.com/raduherinean/herinean.com/internal/render"
	"github.com/raduherinean/herinean.com/internal/seo"
)

var locales = map[string]string{"en": "en_US", "ro": "ro_RO"}

func (b *build) base(lang, kind, path, title, description string) *render.PageData {
	return &render.PageData{Cfg: b.cfg, Site: b.site, Lang: lang, Kind: kind, Title: title, HeadTitle: title + " — " + b.cfg.Name,
		Description: description, Path: path, Canonical: b.cfg.Abs(path), OGType: "website", OGLocale: locales[lang], OGImage: b.cfg.Abs(b.og["home-"+lang]), PreloadFont: b.preload}
}

func (b *build) put(path string, html []byte) {
	p := strings.TrimPrefix(path, "/")
	if strings.HasSuffix(p, "/") || p == "" {
		p += "index.html"
	}
	b.files[p] = html
}

// pages renders every HTML page. The set of paths here is the site's URL space; sitemapEntries and link checking derive from it.
func (b *build) pages() error {
	cfg, s := b.cfg, b.site
	for _, lang := range content.Langs {
		// home
		home := s.Pages["home."+lang]
		d := b.base(lang, "home", cfg.HomeURL(lang), cfg.Name, home.Summary) // the masthead carries the tagline; the meta description is the page's own summary
		d.HeadTitle = cfg.Name
		d.Page, d.Body, d.Tagline, d.Portrait = home, home.Body, cfg.Tagline[lang], b.portrait
		listed := s.ForLang(lang) // one entry per key (spec §4.1 rows 71–72)
		for i, p := range listed {
			if i == 5 {
				break
			}
			d.Entries = append(d.Entries, render.EntryFor(cfg, s, lang, p))
		}
		d.Alternates = render.AlternatesFor(cfg, cfg.HomeURL("en"), cfg.HomeURL("ro"))
		d.JSONLD = render.WebSiteLD(cfg, lang, cfg.Tagline[lang])
		if err := b.render("home", d); err != nil {
			return err
		}
		// index
		d = b.base(lang, "index", cfg.IndexURL(lang), s.T(lang, "index.title"), cfg.Tagline[lang])
		d.Years = render.Years(cfg, s, lang, listed)
		d.Alternates = render.AlternatesFor(cfg, cfg.IndexURL("en"), cfg.IndexURL("ro"))
		d.JSONLD = render.WebSiteLD(cfg, lang, cfg.Tagline[lang])
		if err := b.render("index", d); err != nil {
			return err
		}
		// privacy
		pv := s.Pages["privacy."+lang]
		d = b.base(lang, "privacy", cfg.PrivacyURL(lang), pv.Title, pv.Summary)
		d.Page, d.Body = pv, pv.Body
		d.Alternates = render.AlternatesFor(cfg, cfg.PrivacyURL("en"), cfg.PrivacyURL("ro"))
		d.JSONLD = render.WebSiteLD(cfg, lang, "")
		if err := b.render("privacy", d); err != nil {
			return err
		}
	}
	// pieces
	for _, p := range s.Pieces {
		path := cfg.PieceURL(p.Lang, p.Slug)
		d := b.base(p.Lang, "piece", path, p.Title, p.Summary)
		d.Piece, d.Body, d.OGType, d.OGImage = p, p.Body, "article", cfg.Abs(b.og[p.Lang+"-"+p.Slug])
		d.DateISO, d.DateText = p.Date.Format("2006-01-02"), render.DateText(s, p.Lang, p.Date)
		if p.Updated != nil {
			d.UpdatedISO, d.UpdatedText = p.Updated.Format("2006-01-02"), render.DateText(s, p.Lang, *p.Updated)
		}
		d.PillarText = s.T(p.Lang, "pillar."+p.Pillar)
		en, ro := "", ""
		if p.Lang == "en" {
			en = path
		} else {
			ro = path
		}
		if p.Translation != nil {
			e := render.EntryFor(cfg, s, p.Lang, p.Translation)
			d.Translation = &e
			if p.Lang == "en" {
				ro = e.URL
			} else {
				en = e.URL
			}
		}
		d.Alternates = render.AlternatesFor(cfg, en, ro)
		d.JSONLD = render.ArticleLD(cfg, p, d.OGImage, s.T(p.Lang, "index.title"))
		if err := b.render("piece", d); err != nil {
			return err
		}
	}
	// colophon (EN only)
	co := s.Pages["colophon.en"]
	d := b.base("en", "colophon", cfg.ColophonURL(), co.Title, co.Summary)
	d.Page, d.Body = co, co.Body
	d.Alternates = render.AlternatesFor(cfg, cfg.ColophonURL(), "")
	d.JSONLD = render.WebSiteLD(cfg, "en", "")
	frag, err := b.scorecardFragment()
	if err != nil {
		return err
	}
	d.ScorecardHTML = template.HTML(frag)
	d.Deps, d.Commit, d.BuildDate, d.GoVersion = deps(), b.commit, b.now.Format("2006-01-02"), goVersion()
	if err := b.render("colophon", d); err != nil {
		return err
	}
	// 404
	nf := b.base("en", "404", "/404.html", s.T("en", "notfound.title"), s.T("en", "notfound.body"))
	nf.HeadTitle, nf.Robots, nf.JSONLD = s.T("en", "notfound.title"), "noindex", render.WebSiteLD(cfg, "en", "")
	return b.render("404", nf)
}

func (b *build) render(kind string, d *render.PageData) error {
	out, err := b.r.Render(kind, d)
	if err != nil {
		return err
	}
	b.put(d.Path, out)
	return nil
}

func (b *build) sitemapEntries() []seo.URLEntry {
	cfg, s := b.cfg, b.site
	latest := b.now
	if len(s.Pieces) > 0 {
		latest = s.Pieces[0].Date
	}
	alts := func(en, ro string) []seo.Alt {
		var out []seo.Alt
		for _, a := range render.AlternatesFor(cfg, en, ro) {
			out = append(out, seo.Alt{Lang: a.Lang, Href: a.Href})
		}
		return out
	}
	var es []seo.URLEntry
	for _, lang := range content.Langs {
		// home = About + latest: it moves with the newest piece or with an edit to _home.md, whichever is later
		home := latest
		if about := fileTime(b.o.Root, "content/"+lang+"/_home.md", b.now); about.After(home) {
			home = about
		}
		es = append(es, seo.URLEntry{Loc: cfg.Abs(cfg.HomeURL(lang)), LastMod: home, Alternates: alts(cfg.HomeURL("en"), cfg.HomeURL("ro"))})
		es = append(es, seo.URLEntry{Loc: cfg.Abs(cfg.IndexURL(lang)), LastMod: latest, Alternates: alts(cfg.IndexURL("en"), cfg.IndexURL("ro"))})
		es = append(es, seo.URLEntry{Loc: cfg.Abs(cfg.PrivacyURL(lang)), LastMod: fileTime(b.o.Root, "content/"+lang+"/_privacy.md", b.now), Alternates: alts(cfg.PrivacyURL("en"), cfg.PrivacyURL("ro"))})
	}
	for _, p := range s.Pieces {
		mod := p.Date
		if p.Updated != nil {
			mod = *p.Updated
		}
		en, ro := "", ""
		if p.Lang == "en" {
			en = cfg.PieceURL("en", p.Slug)
		} else {
			ro = cfg.PieceURL("ro", p.Slug)
		}
		if p.Translation != nil {
			if p.Lang == "en" {
				ro = cfg.PieceURL("ro", p.Translation.Slug)
			} else {
				en = cfg.PieceURL("en", p.Translation.Slug)
			}
		}
		es = append(es, seo.URLEntry{Loc: cfg.Abs(cfg.PieceURL(p.Lang, p.Slug)), LastMod: mod, Alternates: alts(en, ro)})
	}
	es = append(es, seo.URLEntry{Loc: cfg.Abs(cfg.ColophonURL()), LastMod: fileTime(b.o.Root, "content/en/_colophon.md", b.now), Alternates: alts(cfg.ColophonURL(), "")})
	return es
}
