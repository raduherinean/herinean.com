package site

import "github.com/raduherinean/herinean.com/internal/render"

// scorecardFragment renders the colophon's scorecard table. Until Task 11 wires scorecard.json and the manual rows in, it is one honest row.
func (b *build) scorecardFragment() ([]byte, error) {
	rows := []render.ScorecardRow{{Check: "CI audit (Lighthouse, HTML, a11y, links, feeds, headers)", Pass: false, Value: "not yet run on this build", When: b.now.Format("2006-01-02")}}
	return b.r.Fragment("scorecard", map[string]any{"Rows": rows})
}
