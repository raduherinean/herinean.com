package content

import (
	"html/template"
	"time"

	"github.com/yuin/goldmark/ast"
)

// Bucharest is the editorial time zone: dates in front matter are local dates there.
var Bucharest = mustLoad("Europe/Bucharest")

func mustLoad(name string) *time.Location {
	l, err := time.LoadLocation(name)
	if err != nil {
		panic(err) // cmd/site imports time/tzdata, so this cannot fail in the binary
	}
	return l
}

// Pillars, in the order they are listed on the site.
var Pillars = []string{"build-log", "opportunity", "governance", "analysis"}

// Piece is one published article in one language.
type Piece struct {
	Lang     string
	Slug     string
	Key      string
	Title    string
	Date     time.Time
	Updated  *time.Time
	Pillar   string
	Summary  string
	LinkedIn string
	Medium   string

	Body           template.HTML // rendered by Render
	Words          int
	ReadingMinutes int
	Images         []ImageRef // referenced by the body, relative names
	Links          []LinkRef  // internal links found in the body

	File        string
	Translation *Piece // the same key in the other language, if any

	doc      ast.Node
	src      []byte
	bodyLine int
}

type ImageRef struct {
	Dest string // as written: "diagram.svg", "photo.jpg"
	Alt  string
	Line int
}

type LinkRef struct {
	Dest string // site-absolute path, e.g. /writing/x/
	Line int
}

// ImageInfo is what the renderer needs to emit a responsive <img>/<picture>. Filled by the images package via site.
type ImageInfo struct {
	Src    string // largest fallback URL (original format)
	Srcset string // "url 720w, url 1440w" in WebP
	Width  int
	Height int
	IsSVG  bool
}

func (p *Piece) URLPath(indexURL string) string { return indexURL + p.Slug + "/" }
