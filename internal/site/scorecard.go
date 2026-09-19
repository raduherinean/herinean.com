package site

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/raduherinean/herinean.com/internal/render"
	"gopkg.in/yaml.v3"
)

// ciRow is what the bench writes into scorecard.json (M2).
type ciRow struct {
	Check    string `json:"check"`
	Pass     bool   `json:"pass"`
	Value    string `json:"value"`
	When     string `json:"when"` // YYYY-MM-DD
	Link     string `json:"link"`
	LinkText string `json:"link_text"`
}

type manualFile struct {
	Rows []struct {
		Check    string    `yaml:"check"`
		Value    string    `yaml:"value"`
		When     time.Time `yaml:"when"`
		Link     string    `yaml:"link"`
		LinkText string    `yaml:"link_text"`
	} `yaml:"rows"`
}

const staleAfter = 90 * 24 * time.Hour

func scorecardRows(ciPath, manualPath string, now time.Time) ([]render.ScorecardRow, error) {
	var rows []render.ScorecardRow
	if b, err := os.ReadFile(ciPath); err == nil {
		var ci []ciRow
		if err := json.Unmarshal(b, &ci); err != nil {
			return nil, fmt.Errorf("%s: %w", ciPath, err)
		}
		for _, r := range ci {
			rows = append(rows, render.ScorecardRow{Check: r.Check, Pass: r.Pass, Value: r.Value, When: r.When, Link: r.Link, LinkText: r.LinkText})
		}
	} else {
		rows = append(rows, render.ScorecardRow{Check: "CI audit (Lighthouse, HTML, a11y, links, feeds, headers)", Pass: false, Value: "not yet run on this build", When: now.Format("2006-01-02")})
	}
	if b, err := os.ReadFile(manualPath); err == nil {
		var m manualFile
		if err := yaml.Unmarshal(b, &m); err != nil {
			return nil, fmt.Errorf("%s: %w", manualPath, err)
		}
		for _, r := range m.Rows {
			rows = append(rows, render.ScorecardRow{Check: r.Check, Pass: true, Value: r.Value, When: r.When.Format("2006-01-02"), Link: r.Link, LinkText: r.LinkText, Stale: now.Sub(r.When) > staleAfter})
		}
	}
	return rows, nil
}

func (b *build) scorecardFragment() ([]byte, error) {
	rows, err := scorecardRows(filepath.Join(b.o.Root, "scorecard.json"), filepath.Join(b.o.Root, "data", "scorecard-manual.yaml"), b.now)
	if err != nil {
		return nil, err
	}
	return b.r.Fragment("scorecard", map[string]any{"Rows": rows})
}

// Scorecard renders the fragment for CI to validate and store in KV (M2).
func Scorecard(o Options, in, manual, out string) error {
	r, err := render.New(filepath.Join(o.Root, "templates"), filepath.Join(o.Root, "assets", "css", "site.css"))
	if err != nil {
		return err
	}
	now, err := BuildTime(o.Root)
	if err != nil {
		return err
	}
	rows, err := scorecardRows(in, manual, now)
	if err != nil {
		return err
	}
	frag, err := r.Fragment("scorecard", map[string]any{"Rows": rows})
	if err != nil {
		return err
	}
	return os.WriteFile(out, frag, 0o644)
}
