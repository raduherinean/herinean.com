package content

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// Page is a non-piece page (home, privacy, colophon) authored in Markdown with a title.
type Page struct {
	Lang    string
	Kind    string
	Title   string
	Summary string // meta description
	Body    template.HTML
	File    string
	piece   *Piece // reuse the piece pipeline for parsing/rendering
}

// Links are the internal links in the page body, for the link check.
func (pg *Page) Links() []LinkRef { return pg.piece.Links }

type Site struct {
	Pieces  []*Piece
	ByLang  map[string][]*Piece
	Pages   map[string]*Page
	Strings map[string]map[string]string
}

var pageFiles = map[string][]string{ // kind -> languages that must have it
	"home": {"en", "ro"}, "privacy": {"en", "ro"}, "colophon": {"en"},
}

// Load parses and validates everything under root. Bodies are parsed, not yet rendered.
func Load(root string, now time.Time) (*Site, Problems) {
	s, _, probs := load(root, now, false)
	return s, probs
}

// LoadDraft is Load for `site serve`: a piece still missing what `site new` leaves blank (title, date, pillar,
// summary) is loaded with visible defaults, and those gaps come back as warnings rather than problems, so the
// author can write on the preview (spec §8). Every other rule still produces a problem.
func LoadDraft(root string, now time.Time) (s *Site, warnings, probs Problems) {
	return load(root, now, true)
}

func load(root string, now time.Time, draft bool) (*Site, Problems, Problems) {
	var warnings, probs Problems
	s := &Site{ByLang: map[string][]*Piece{}, Pages: map[string]*Page{}}
	s.Strings, probs = loadStrings(filepath.Join(root, "i18n"))

	for _, lang := range Langs {
		dir := filepath.Join(root, "content", lang)
		entries, err := os.ReadDir(dir)
		if err != nil {
			probs.Add(dir, 0, "%v", err)
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".md") {
				continue
			}
			file := filepath.Join(dir, name)
			src, err := os.ReadFile(file)
			if err != nil {
				probs.Add(file, 0, "%v", err)
				continue
			}
			if strings.HasPrefix(name, "_") {
				kind := strings.TrimSuffix(strings.TrimPrefix(name, "_"), ".md")
				pg, ps := parsePage(file, src, lang, kind)
				probs = append(probs, ps...)
				if pg != nil {
					s.Pages[kind+"."+lang] = pg
				}
				continue
			}
			p, body, _, ws, ps := parseFrontMatter(file, src, lang, now, draft)
			warnings = append(warnings, ws...)
			probs = append(probs, ps...)
			if p == nil {
				continue
			}
			probs = append(probs, ParseBody(p, body)...)
			s.Pieces = append(s.Pieces, p)
			s.ByLang[lang] = append(s.ByLang[lang], p)
		}
	}
	for kind, langs := range pageFiles {
		for _, lang := range langs {
			if s.Pages[kind+"."+lang] == nil {
				probs.Add(filepath.Join(root, "content", lang, "_"+kind+".md"), 0, "required page is missing")
			}
		}
	}
	probs = append(probs, pair(s)...)
	sortPieces(s.Pieces)
	for _, lang := range Langs {
		sortPieces(s.ByLang[lang])
	}
	return s, warnings, probs
}

func parsePage(file string, src []byte, lang, kind string) (*Page, Problems) {
	var probs Problems
	if !bytes.HasPrefix(src, []byte("---\n")) {
		probs.Add(file, 1, "page needs a front matter with title")
		return nil, probs
	}
	end := bytes.Index(src[4:], []byte("\n---\n"))
	if end < 0 {
		probs.Add(file, 1, "unterminated front matter")
		return nil, probs
	}
	var fm struct {
		Title   string `yaml:"title"`
		Summary string `yaml:"summary"`
	}
	if err := yaml.Unmarshal(src[4:4+end], &fm); err != nil || strings.TrimSpace(fm.Title) == "" {
		probs.Add(file, 1, "page front matter needs a title")
		return nil, probs
	}
	if n := utf8.RuneCountInString(fm.Summary); strings.TrimSpace(fm.Summary) == "" || n > summaryMax {
		probs.Add(file, 1, "page front matter needs a summary of 1–%d characters (the meta description)", summaryMax)
	}
	body := src[4+end+5:]
	p := &Piece{Lang: lang, File: file, Key: "_" + kind, bodyLine: bytes.Count(src[:4+end+5], []byte("\n")) + 1}
	probs = append(probs, ParseBody(p, body)...)
	return &Page{Lang: lang, Kind: kind, Title: strings.TrimSpace(fm.Title), Summary: strings.TrimSpace(fm.Summary), File: file, piece: p}, probs
}

func pair(s *Site) Problems {
	var probs Problems
	byKey := map[string]map[string]*Piece{}
	for _, p := range s.Pieces {
		if byKey[p.Key] == nil {
			byKey[p.Key] = map[string]*Piece{}
		}
		if other, dup := byKey[p.Key][p.Lang]; dup {
			probs.Add(p.File, 1, "key %q already used by %s in the same language", p.Key, other.File)
			continue
		}
		byKey[p.Key][p.Lang] = p
	}
	slugs := map[string]*Piece{}
	for _, p := range s.Pieces {
		id := p.Lang + "/" + p.Slug
		if other, dup := slugs[id]; dup {
			probs.Add(p.File, 1, "slug %q already used by %s", p.Slug, other.File)
		}
		slugs[id] = p
	}
	for _, m := range byKey {
		if en, ro := m["en"], m["ro"]; en != nil && ro != nil {
			en.Translation, ro.Translation = ro, en
		}
	}
	return probs
}

func sortPieces(ps []*Piece) {
	sort.SliceStable(ps, func(i, j int) bool {
		if !ps[i].Date.Equal(ps[j].Date) {
			return ps[i].Date.After(ps[j].Date)
		}
		return ps[i].Slug < ps[j].Slug
	})
}

// Render fills every Body. lookup resolves an image by piece key and relative name.
func (s *Site) Render(lookup func(key, dest string) *ImageInfo) error {
	for _, p := range s.Pieces {
		body, err := RenderBody(p, func(dest string) *ImageInfo { return lookup(p.Key, dest) }, s.T(p.Lang, "table.label"))
		if err != nil {
			return err
		}
		p.Body = body
	}
	for _, pg := range s.Pages {
		body, err := RenderBody(pg.piece, func(dest string) *ImageInfo { return lookup(pg.piece.Key, dest) }, s.T(pg.Lang, "table.label"))
		if err != nil {
			return err
		}
		pg.Body = body
	}
	return nil
}

// Latest returns the n newest pieces across languages.
func (s *Site) Latest(n int) []*Piece {
	if n > len(s.Pieces) {
		n = len(s.Pieces)
	}
	return s.Pieces[:n]
}

// T looks up a UI string; a missing key is a programming error caught by tests, so it panics loudly.
func (s *Site) T(lang, key string) string {
	v, ok := s.Strings[lang][key]
	if !ok {
		panic(fmt.Sprintf("i18n: missing %s.%s", lang, key))
	}
	return v
}

// AllImageRefs lists every (key, dest) the site references, for the images stage.
func (s *Site) AllImageRefs() map[string][]ImageRef {
	out := map[string][]ImageRef{}
	for _, p := range s.Pieces {
		out[p.Key] = append(out[p.Key], p.Images...)
	}
	for _, pg := range s.Pages {
		out[pg.piece.Key] = append(out[pg.piece.Key], pg.piece.Images...)
	}
	return out
}
