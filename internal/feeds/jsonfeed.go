package feeds

import (
	"encoding/json"
	"time"

	"github.com/raduherinean/herinean.com/internal/config"
	"github.com/raduherinean/herinean.com/internal/content"
)

type jsonFeed struct {
	Version     string       `json:"version"`
	Title       string       `json:"title"`
	HomePageURL string       `json:"home_page_url"`
	FeedURL     string       `json:"feed_url"`
	Description string       `json:"description"`
	Language    string       `json:"language"`
	Authors     []jsonAuthor `json:"authors"`
	Items       []jsonItem   `json:"items"`
}

type jsonAuthor struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type jsonItem struct {
	ID            string   `json:"id"`
	URL           string   `json:"url"`
	Title         string   `json:"title"`
	ContentHTML   string   `json:"content_html"`
	Summary       string   `json:"summary"`
	DatePublished string   `json:"date_published"`
	DateModified  string   `json:"date_modified,omitempty"`
	Language      string   `json:"language"`
	Tags          []string `json:"tags"`
}

func JSON(cfg *config.Config, s *content.Site, now time.Time) ([]byte, error) {
	f := jsonFeed{Version: "https://jsonfeed.org/version/1.1", Title: cfg.Name, HomePageURL: cfg.Abs("/"), FeedURL: cfg.Abs("/feed.json"),
		Description: cfg.Tagline["en"], Language: "en", Authors: []jsonAuthor{{Name: cfg.Name, URL: cfg.Abs("/")}},
		Items: []jsonItem{}} // JSON Feed 1.1 requires an array; a nil slice would marshal as null on an empty site
	for _, p := range s.Pieces {
		u := cfg.Abs(cfg.PieceURL(p.Lang, p.Slug))
		it := jsonItem{ID: u, URL: u, Title: p.Title, ContentHTML: Absolutize(string(p.Body), cfg.BaseURL, u), Summary: p.Summary,
			DatePublished: p.Date.Add(6 * time.Hour).Format(time.RFC3339), Language: p.Lang, Tags: []string{p.Pillar}}
		if p.Updated != nil {
			it.DateModified = p.Updated.Add(6 * time.Hour).Format(time.RFC3339)
		}
		f.Items = append(f.Items, it)
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
