package feeds

import (
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/raduherinean/herinean.com/internal/config"
	"github.com/raduherinean/herinean.com/internal/content"
)

func fixture(t *testing.T) (*config.Config, *content.Site, time.Time) {
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
	_ = s.Render(func(key, dest string) *content.ImageInfo {
		return &content.ImageInfo{Src: "/img/" + key + "/" + dest, Width: 10, Height: 10}
	})
	return cfg, s, now
}

func TestAbsolutize(t *testing.T) {
	in := `<p>prose, /not a url</p><a href="/writing/x/">a</a><a href="#fn:1">1</a><img src="/img/k/f.png" srcset="/img/k/f.720.webp 720w, /img/k/f.1440.webp 1440w"><a href="https://ext/">e</a><a href="//cdn.example/x">p</a><img srcset="//cdn.example/f.webp 720w"><a href="/">home</a>`
	got := Absolutize(in, "https://herinean.com", "https://herinean.com/writing/y/")
	for _, want := range []string{`<p>prose, /not a url</p>`, `href="https://herinean.com/writing/x/"`, `href="https://herinean.com/writing/y/#fn:1"`, `src="https://herinean.com/img/k/f.png"`, `srcset="https://herinean.com/img/k/f.720.webp 720w, https://herinean.com/img/k/f.1440.webp 1440w"`, `href="https://ext/"`, `href="//cdn.example/x"`, `srcset="//cdn.example/f.webp 720w"`, `href="https://herinean.com/"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
}

func TestRSS(t *testing.T) {
	cfg, s, now := fixture(t)
	for _, lang := range []string{"", "en", "ro"} {
		b, err := RSS(cfg, s, lang, now)
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			XMLName xml.Name `xml:"rss"`
			Channel struct {
				Title string `xml:"title"`
				Link  string `xml:"link"`
				Items []struct {
					Title   string `xml:"title"`
					Link    string `xml:"link"`
					PubDate string `xml:"pubDate"`
					Content string `xml:"encoded"`
				} `xml:"item"`
			} `xml:"channel"`
		}
		if err := xml.Unmarshal(b, &doc); err != nil {
			t.Fatalf("%s: invalid xml: %v", lang, err)
		}
		want := len(s.Pieces)
		if lang != "" {
			want = len(s.ByLang[lang])
		}
		if len(doc.Channel.Items) != want {
			t.Errorf("%s: %d items want %d", lang, len(doc.Channel.Items), want)
		}
		sb := string(b)
		if !strings.Contains(sb, `<atom:link href="https://herinean.com/feed`) || !strings.Contains(sb, `rel="self"`) {
			t.Errorf("%s: missing atom:link self", lang)
		}
		if strings.Contains(sb, `href="/`) || strings.Contains(sb, `src="/`) {
			t.Errorf("%s: relative URL leaked into the feed", lang)
		}
		if !strings.HasPrefix(doc.Channel.Items[0].Link, "https://herinean.com/") || doc.Channel.Items[0].Content == "" {
			t.Errorf("%s: item %+v", lang, doc.Channel.Items[0])
		}
	}
}

func TestJSONFeed(t *testing.T) {
	cfg, s, now := fixture(t)
	b, err := JSON(cfg, s, now)
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Version string `json:"version"`
		FeedURL string `json:"feed_url"`
		Items   []struct {
			ID          string `json:"id"`
			URL         string `json:"url"`
			ContentHTML string `json:"content_html"`
			Language    string `json:"language"`
		} `json:"items"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	if f.Version != "https://jsonfeed.org/version/1.1" || f.FeedURL != "https://herinean.com/feed.json" || len(f.Items) != len(s.Pieces) {
		t.Errorf("%+v", f)
	}
	if it := f.Items[0]; it.ID != it.URL || !strings.HasPrefix(it.URL, "https://herinean.com/") || it.ContentHTML == "" || strings.Contains(it.ContentHTML, `href="/`) || (it.Language != "en" && it.Language != "ro") {
		t.Errorf("item: %+v", it)
	}
}
