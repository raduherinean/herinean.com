package content

import (
	"strings"
	"testing"
)

func piece(lang, body string) *Piece {
	return &Piece{Lang: lang, File: "content/" + lang + "/t.md", bodyLine: 5, Key: "k"}
}

func TestQuotesPerLanguage(t *testing.T) {
	en, ro := piece("en", ""), piece("ro", "")
	if probs := ParseBody(en, []byte(`He said "hello" -- really...`)); probs.Err() != nil {
		t.Fatal(probs.Err())
	}
	if probs := ParseBody(ro, []byte(`A spus "salut" -- chiar...`)); probs.Err() != nil {
		t.Fatal(probs.Err())
	}
	enHTML, _ := RenderBody(en, nil, "Table")
	roHTML, _ := RenderBody(ro, nil, "Tabel")
	if !strings.Contains(string(enHTML), "&ldquo;hello&rdquo;") {
		t.Errorf("en quotes: %s", enHTML)
	}
	if !strings.Contains(string(roHTML), "„salut”") {
		t.Errorf("ro quotes: %s", roHTML)
	}
	if !strings.Contains(string(enHTML), "&ndash;") && !strings.Contains(string(enHTML), "–") {
		t.Errorf("dash: %s", enHTML)
	}
}

func TestImageAltRequired(t *testing.T) {
	p := piece("en", "")
	probs := ParseBody(p, []byte("![](photo.jpg)\n\n![ok](fig.png \"A caption\")\n"))
	if len(probs) != 1 || !strings.Contains(probs[0].Msg, "alt") || probs[0].Line != 5 {
		t.Fatalf("want one alt problem at line 5, got %+v", probs)
	}
	if len(p.Images) != 2 || p.Images[1].Dest != "fig.png" || p.Images[1].Alt != "ok" {
		t.Errorf("images: %+v", p.Images)
	}
}

func TestFigureAndResponsiveImage(t *testing.T) {
	p := piece("en", "")
	_ = ParseBody(p, []byte("Intro.\n\n![Alt text](fig.png \"A caption\")\n\nInline ![i](x.svg) here.\n"))
	lookup := func(dest string) *ImageInfo {
		switch dest {
		case "fig.png":
			return &ImageInfo{Src: "/img/k/fig.abc12345.1440.png", Srcset: "/img/k/fig.abc12345.720.webp 720w, /img/k/fig.abc12345.1440.webp 1440w", Width: 1440, Height: 900}
		case "x.svg":
			return &ImageInfo{Src: "/img/k/x.abc12345.svg", Width: 24, Height: 24, IsSVG: true}
		}
		return nil
	}
	html, err := RenderBody(p, lookup, "Table")
	if err != nil {
		t.Fatal(err)
	}
	s := string(html)
	for _, want := range []string{
		`<figure><picture><source type="image/webp" srcset="/img/k/fig.abc12345.720.webp 720w, /img/k/fig.abc12345.1440.webp 1440w" sizes="(max-width: 40rem) 100vw, 38rem"><img src="/img/k/fig.abc12345.1440.png" alt="Alt text" width="1440" height="900" loading="lazy" decoding="async"></picture><figcaption>A caption</figcaption></figure>`,
		`<img src="/img/k/x.abc12345.svg" alt="i" width="24" height="24" loading="lazy" decoding="async">`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s\nin %s", want, s)
		}
	}
	if strings.Contains(s, "<p><figure") {
		t.Error("figure must not be wrapped in <p>")
	}
}

func TestCodeUsesClassesNotStyles(t *testing.T) {
	p := piece("en", "")
	_ = ParseBody(p, []byte("```go\nfmt.Println(\"x\")\n```\n"))
	html, _ := RenderBody(p, nil, "Table")
	if strings.Contains(string(html), "style=") || !strings.Contains(string(html), `class="chroma"`) || !strings.Contains(string(html), `<pre tabindex="0"`) {
		t.Errorf("highlighting must use classes and a focusable pre: %s", html)
	}
}

func TestCodeNeedsLanguage(t *testing.T) {
	p := piece("en", "")
	probs := ParseBody(p, []byte("```\nplain\n```\n\n    indented\n"))
	if len(probs) != 2 || !strings.Contains(probs[0].Msg, "language") || !strings.Contains(probs[1].Msg, "indented") {
		t.Errorf("want a language problem and an indented-block problem, got %+v", probs)
	}
}

func TestTablesWrapAndNeverUseStyle(t *testing.T) {
	p := piece("en", "")
	_ = ParseBody(p, []byte("| a | b |\n|---:|:---|\n| 1 | 2 |\n"))
	html, _ := RenderBody(p, nil, "Table")
	s := string(html)
	if !strings.Contains(s, `<div class="table" role="region" aria-label="Table" tabindex="0"><table>`) || !strings.Contains(s, "</table>\n</div>") || strings.Contains(s, "style=") || strings.Contains(s, "align=") {
		t.Errorf("tables must be wrapped, focusable and free of style/align attributes: %s", s)
	}
}

func TestReadingTimeAndLinks(t *testing.T) {
	p := piece("en", "")
	body := strings.Repeat("word ", 450) + "\n\nSee [this](/writing/other/) and [that](https://example.com).\n"
	_ = ParseBody(p, []byte(body))
	if p.Words < 450 || p.ReadingMinutes != 3 {
		t.Errorf("words %d minutes %d", p.Words, p.ReadingMinutes)
	}
	if len(p.Links) != 1 || p.Links[0].Dest != "/writing/other/" {
		t.Errorf("links: %+v", p.Links)
	}
}

func TestFootnotesAndTables(t *testing.T) {
	p := piece("en", "")
	_ = ParseBody(p, []byte("A claim[^1].\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n[^1]: Source.\n"))
	html, _ := RenderBody(p, nil, "Table")
	s := string(html)
	if !strings.Contains(s, "<table>") || !strings.Contains(s, `class="footnotes"`) {
		t.Errorf("%s", s)
	}
}

// The title is the page's h1; a body may start at ## or ### and must then descend one level at a time.
func TestBodyHeadingRules(t *testing.T) {
	for _, tc := range []struct {
		body string
		want int
		msg  string
	}{
		{"# Top\n\nText.\n", 1, "level-1"},
		{"## A\n\n#### B\n", 1, "jumps"},
		{"## A\n\n### B\n\n## C\n", 0, ""},
		{"### A\n\n#### B\n", 0, ""}, // the first heading may be ###
		{"## A\n\n### B\n\n## C\n\n#### D\n", 1, "jumps from h2 to h4"},
	} {
		p := piece("en", "")
		probs := ParseBody(p, []byte(tc.body))
		if len(probs) != tc.want {
			t.Errorf("%q: %d problems, want %d: %+v", tc.body, len(probs), tc.want, probs)
			continue
		}
		if tc.want == 1 && !strings.Contains(probs[0].Msg, tc.msg) {
			t.Errorf("%q: problem %q does not mention %q", tc.body, probs[0].Msg, tc.msg)
		}
	}
	// the finding points at the heading's own line (body starts at bodyLine 5 in this fixture)
	probs := ParseBody(piece("en", ""), []byte("Intro.\n\n## A\n\n#### B\n"))
	if len(probs) != 1 || probs[0].Line != 9 {
		t.Errorf("want the jump reported at line 9, got %+v", probs)
	}
}
