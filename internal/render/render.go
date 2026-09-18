// Package render turns page data into HTML with html/template.
package render

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
)

type Renderer struct {
	base  *template.Template            // base + entries + scorecard
	pages map[string]*template.Template // kind → clone of base with that page's "content"
	css   string
	hash  string
}

var kinds = []string{"home", "index", "piece", "colophon", "privacy", "404"}

// New parses the shared templates once, then one clone per page kind. Every page file defines "content";
// cloning before parsing each keeps the definitions apart (in one set, the last file parsed would win for all kinds).
func New(templatesDir, cssPath string) (*Renderer, error) {
	raw, err := os.ReadFile(cssPath)
	if err != nil {
		return nil, err
	}
	css := minifyCSS(string(raw))
	shared := []string{"base.html", "entries.html", "scorecard.html"}
	for i, f := range shared {
		shared[i] = filepath.Join(templatesDir, f)
	}
	base, err := template.ParseFiles(shared...)
	if err != nil {
		return nil, err
	}
	r := &Renderer{base: base, pages: map[string]*template.Template{}, css: css, hash: CSPHash(css)}
	for _, k := range kinds {
		t, err := base.Clone()
		if err != nil {
			return nil, err
		}
		if _, err := t.ParseFiles(filepath.Join(templatesDir, k+".html")); err != nil {
			return nil, err
		}
		r.pages[k] = t
	}
	return r, nil
}

func (r *Renderer) CSS() string     { return r.css }
func (r *Renderer) CSSHash() string { return r.hash }

// Render executes the page template for kind (home, index, piece, colophon, privacy, 404).
func (r *Renderer) Render(kind string, d *PageData) ([]byte, error) {
	t := r.pages[kind]
	if t == nil {
		return nil, fmt.Errorf("render: no template for kind %q", kind)
	}
	d.CSS = template.CSS(r.css)
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "base", d); err != nil {
		return nil, fmt.Errorf("render %s: %w", kind, err)
	}
	return buf.Bytes(), nil
}

// Fragment executes a named shared template (e.g. "scorecard").
func (r *Renderer) Fragment(name string, data any) ([]byte, error) {
	var buf bytes.Buffer
	if err := r.base.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
