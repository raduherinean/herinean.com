package render

import (
	"encoding/json"
	"html/template"
	"time"

	"github.com/raduherinean/herinean.com/internal/config"
	"github.com/raduherinean/herinean.com/internal/content"
)

func person(cfg *config.Config) map[string]any {
	return map[string]any{
		"@type": "Person", "name": cfg.Name, "url": cfg.BaseURL + "/",
		"sameAs": []string{cfg.Author.LinkedIn, cfg.Author.X, cfg.Author.GitHub},
	}
}

func WebSiteLD(cfg *config.Config, lang, description string) template.JS {
	return jsonLD(map[string]any{
		"@context": "https://schema.org", "@graph": []any{
			map[string]any{"@type": "WebSite", "@id": cfg.BaseURL + "/#website", "url": cfg.BaseURL + "/", "name": cfg.Name, "description": description, "inLanguage": lang, "author": map[string]any{"@id": cfg.BaseURL + "/#person"}},
			withID(person(cfg), cfg.BaseURL+"/#person"),
		},
	})
}

func ArticleLD(cfg *config.Config, p *content.Piece, ogImage, indexTitle string) template.JS {
	url := cfg.Abs(cfg.PieceURL(p.Lang, p.Slug))
	mod := p.Date
	if p.Updated != nil {
		mod = *p.Updated
	}
	crumbs := []any{
		map[string]any{"@type": "ListItem", "position": 1, "name": cfg.Name, "item": cfg.Abs(cfg.HomeURL(p.Lang))},
		map[string]any{"@type": "ListItem", "position": 2, "name": indexTitle, "item": cfg.Abs(cfg.IndexURL(p.Lang))},
		map[string]any{"@type": "ListItem", "position": 3, "name": p.Title, "item": url},
	}
	return jsonLD(map[string]any{
		"@context": "https://schema.org", "@graph": []any{
			map[string]any{
				"@type": "BlogPosting", "@id": url + "#article", "mainEntityOfPage": url, "url": url,
				"headline": p.Title, "description": p.Summary, "inLanguage": p.Lang,
				"datePublished": p.Date.Format(time.RFC3339), "dateModified": mod.Format(time.RFC3339),
				"image": ogImage, "author": map[string]any{"@id": cfg.BaseURL + "/#person"}, "publisher": map[string]any{"@id": cfg.BaseURL + "/#person"},
				"articleSection": p.Pillar, "wordCount": p.Words,
			},
			map[string]any{"@type": "BreadcrumbList", "itemListElement": crumbs},
			withID(person(cfg), cfg.BaseURL+"/#person"),
		},
	})
}

func withID(m map[string]any, id string) map[string]any { m["@id"] = id; return m }

func jsonLD(v any) template.JS {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	// html/template escapes "<" inside a script as < already via JS context; keep bytes plain.
	return template.JS(b)
}
