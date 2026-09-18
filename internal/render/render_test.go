package render

import (
	"bytes"
	"flag"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/raduherinean/herinean.com/internal/config"
	"github.com/raduherinean/herinean.com/internal/content"
)

var update = flag.Bool("update", false, "rewrite golden files")

func fixture(t *testing.T) (*config.Config, *content.Site, *Renderer) {
	t.Helper()
	cfg, err := config.Load("../../testdata/site/site.yaml")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 12, 31, 12, 0, 0, 0, content.Bucharest)
	s, probs := content.Load("../../testdata/site", now)
	if err := probs.Err(); err != nil {
		t.Fatal(err)
	}
	if err := s.Render(func(key, dest string) *content.ImageInfo {
		return &content.ImageInfo{Src: "/img/" + key + "/" + dest, Width: 1440, Height: 900, Srcset: "/img/" + key + "/" + dest + " 1440w"}
	}); err != nil {
		t.Fatal(err)
	}
	r, err := New("../../templates", "../../assets/css/site.css")
	if err != nil {
		t.Fatal(err)
	}
	return cfg, s, r
}

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	p := filepath.Join("../../testdata/golden", name)
	if *update {
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("no golden %s (run with -update): %v", p, err)
	}
	if !bytes.Equal(want, got) {
		t.Errorf("%s differs from golden; diff with: diff <(cat %s) <(go test ./internal/render -run %s -v)", name, p, t.Name())
	}
}

func TestPieceGolden(t *testing.T) {
	cfg, s, r := fixture(t)
	for _, p := range s.Pieces {
		d := &PageData{Cfg: cfg, Site: s, Lang: p.Lang, Kind: "piece", Piece: p, Title: p.Title, HeadTitle: p.Title + " — " + cfg.Name,
			Description: p.Summary, Path: cfg.PieceURL(p.Lang, p.Slug), Canonical: cfg.Abs(cfg.PieceURL(p.Lang, p.Slug)),
			OGImage: cfg.Abs("/og/" + p.Lang + "-" + p.Slug + ".deadbeef.png"), OGType: "article", OGLocale: map[string]string{"en": "en_US", "ro": "ro_RO"}[p.Lang],
			Body: p.Body, DateISO: p.Date.Format("2006-01-02"), DateText: DateText(s, p.Lang, p.Date), PillarText: s.T(p.Lang, "pillar."+p.Pillar)}
		if p.Updated != nil {
			d.UpdatedISO, d.UpdatedText = p.Updated.Format("2006-01-02"), DateText(s, p.Lang, *p.Updated)
		}
		en, ro := "", ""
		if p.Lang == "en" {
			en = d.Path
		} else {
			ro = d.Path
		}
		if p.Translation != nil {
			e := EntryFor(cfg, s, p.Lang, p.Translation)
			d.Translation = &e
			if p.Lang == "en" {
				ro = e.URL
			} else {
				en = e.URL
			}
		}
		d.Alternates = AlternatesFor(cfg, en, ro)
		d.JSONLD = ArticleLD(cfg, p, d.OGImage, s.T(p.Lang, "index.title"))
		out, err := r.Render("piece", d)
		if err != nil {
			t.Fatal(err)
		}
		golden(t, "piece-"+p.Lang+"-"+p.Slug+".html", out)
		assertInvariants(t, out)
	}
}

func TestHomeIndexGolden(t *testing.T) {
	cfg, s, r := fixture(t)
	for _, lang := range content.Langs {
		var entries []Entry
		for _, p := range s.Latest(5) {
			entries = append(entries, EntryFor(cfg, s, lang, p))
		}
		home := s.Pages["home."+lang]
		d := &PageData{Cfg: cfg, Site: s, Lang: lang, Kind: "home", Page: home, Title: cfg.Name, HeadTitle: cfg.Name, Description: cfg.Tagline[lang],
			Path: cfg.HomeURL(lang), Canonical: cfg.Abs(cfg.HomeURL(lang)), OGImage: cfg.Abs("/og/home-" + lang + ".deadbeef.png"), OGType: "website",
			OGLocale: map[string]string{"en": "en_US", "ro": "ro_RO"}[lang], Body: home.Body, Tagline: cfg.Tagline[lang], Entries: entries,
			Alternates: AlternatesFor(cfg, cfg.HomeURL("en"), cfg.HomeURL("ro")), JSONLD: WebSiteLD(cfg, lang, cfg.Tagline[lang])}
		out, err := r.Render("home", d)
		if err != nil {
			t.Fatal(err)
		}
		golden(t, "home-"+lang+".html", out)
		assertInvariants(t, out)

		idx := &PageData{Cfg: cfg, Site: s, Lang: lang, Kind: "index", Title: s.T(lang, "index.title"), HeadTitle: s.T(lang, "index.title") + " — " + cfg.Name,
			Description: cfg.Tagline[lang], Path: cfg.IndexURL(lang), Canonical: cfg.Abs(cfg.IndexURL(lang)), OGImage: cfg.Abs("/og/home-" + lang + ".deadbeef.png"),
			OGType: "website", OGLocale: map[string]string{"en": "en_US", "ro": "ro_RO"}[lang], Years: Years(cfg, s, lang, s.Pieces),
			Alternates: AlternatesFor(cfg, cfg.IndexURL("en"), cfg.IndexURL("ro")), JSONLD: WebSiteLD(cfg, lang, cfg.Tagline[lang])}
		out, err = r.Render("index", idx)
		if err != nil {
			t.Fatal(err)
		}
		golden(t, "index-"+lang+".html", out)
		assertInvariants(t, out)
	}
}

func Test404AndPrivacyAndColophon(t *testing.T) {
	cfg, s, r := fixture(t)
	nf := &PageData{Cfg: cfg, Site: s, Lang: "en", Kind: "404", Title: s.T("en", "notfound.title"), HeadTitle: s.T("en", "notfound.title"), Description: s.T("en", "notfound.body"),
		Path: "/404.html", Canonical: cfg.Abs("/404.html"), OGImage: cfg.Abs("/og/home-en.deadbeef.png"), OGType: "website", OGLocale: "en_US", Robots: "noindex", JSONLD: WebSiteLD(cfg, "en", "")}
	out, err := r.Render("404", nf)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "404.html", out)

	pv := s.Pages["privacy.ro"]
	d := &PageData{Cfg: cfg, Site: s, Lang: "ro", Kind: "privacy", Page: pv, Title: pv.Title, HeadTitle: pv.Title + " — " + cfg.Name, Description: pv.Summary,
		Path: cfg.PrivacyURL("ro"), Canonical: cfg.Abs(cfg.PrivacyURL("ro")), OGImage: cfg.Abs("/og/home-ro.deadbeef.png"), OGType: "website", OGLocale: "ro_RO", Body: pv.Body,
		Alternates: AlternatesFor(cfg, cfg.PrivacyURL("en"), cfg.PrivacyURL("ro")), JSONLD: WebSiteLD(cfg, "ro", "")}
	out, err = r.Render("privacy", d)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "privacy-ro.html", out)

	frag, err := r.Fragment("scorecard", map[string]any{"Rows": []ScorecardRow{{Check: "Lighthouse", Pass: true, Value: "100/100/100/100", When: "2026-10-11", Link: "https://pagespeed.web.dev/", LinkText: "PageSpeed"}, {Check: "Uptime", Pass: false, Value: "99.2%", When: "2026-06-01", Stale: true}}})
	if err != nil {
		t.Fatal(err)
	}
	co := s.Pages["colophon.en"]
	c := &PageData{Cfg: cfg, Site: s, Lang: "en", Kind: "colophon", Page: co, Title: co.Title, HeadTitle: co.Title + " — " + cfg.Name, Description: co.Summary,
		Path: "/colophon/", Canonical: cfg.Abs("/colophon/"), OGImage: cfg.Abs("/og/home-en.deadbeef.png"), OGType: "website", OGLocale: "en_US", Body: co.Body,
		Alternates: AlternatesFor(cfg, "/colophon/", ""), JSONLD: WebSiteLD(cfg, "en", ""), ScorecardHTML: template.HTML(frag), Deps: "github.com/yuin/goldmark v1.7.8", Commit: "abc1234", BuildDate: "2026-10-11", GoVersion: "go1.27.1"}
	out, err = r.Render("colophon", c)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "colophon.html", out)
	assertInvariants(t, out)
}

func TestCSSHashMatchesInlined(t *testing.T) {
	_, _, r := fixture(t)
	if !strings.HasPrefix(r.CSSHash(), "sha256-") || CSPHash(r.CSS()) != r.CSSHash() {
		t.Error("hash must be of the exact inlined CSS")
	}
	if strings.Contains(r.CSS(), "/*") || strings.Contains(r.CSS(), "\n") {
		t.Error("CSS must be minified")
	}
}

func assertInvariants(t *testing.T, html []byte) {
	t.Helper()
	s := string(html)
	if strings.Count(s, "<script") != 1 || !strings.Contains(s, `<script type="application/ld+json">`) {
		t.Error("exactly one script, and it must be JSON-LD")
	}
	if strings.Contains(s, " style=") {
		t.Error("no style attributes (hash CSP)")
	}
	for _, must := range []string{`rel="canonical"`, `property="og:image"`, `name="twitter:card"`, `hreflang="x-default"`, `rel="alternate" type="application/rss+xml"`, `class="skip"`, `<main id="main">`} {
		if !strings.Contains(s, must) {
			t.Errorf("missing %s", must)
		}
	}
	if strings.Count(s, "<h1") != 1 {
		t.Errorf("every page has exactly one h1, got %d", strings.Count(s, "<h1"))
	}
}
