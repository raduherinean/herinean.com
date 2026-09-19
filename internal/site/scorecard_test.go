package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScorecardRows(t *testing.T) {
	dir := t.TempDir()
	ci := filepath.Join(dir, "scorecard.json")
	_ = os.WriteFile(ci, []byte(`[{"check":"Lighthouse","pass":true,"value":"100/100/100/100","when":"2026-10-11","link":"https://pagespeed.web.dev/","link_text":"PageSpeed"}]`), 0o644)
	man := filepath.Join(dir, "manual.yaml")
	_ = os.WriteFile(man, []byte("rows:\n  - check: SSL Labs\n    value: A\n    when: 2026-01-01\n    measured: placeholder\n    link: https://x\n    link_text: x\n"), 0o644)
	rows, err := scorecardRows(ci, man, time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || !rows[0].Pass || rows[0].Check != "Lighthouse" || !rows[1].Stale || rows[1].Measured != "placeholder" {
		t.Errorf("%+v", rows)
	}
	// a CI row measured more than 90 days ago is stale like a manual one; the colophon must say so
	old := filepath.Join(dir, "old.json")
	_ = os.WriteFile(old, []byte(`[{"check":"Lighthouse","pass":true,"value":"100","when":"2026-01-01"}]`), 0o644)
	rows, err = scorecardRows(old, man, time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC))
	if err != nil || !rows[0].Stale {
		t.Errorf("old CI row must be stale: %v %+v", err, rows)
	}
	bad := filepath.Join(dir, "bad.yaml")
	_ = os.WriteFile(bad, []byte("rows:\n  - check: SSL Labs\n    value: A\n    when: 2026-01-01\n    link: https://x\n    link_text: x\n"), 0o644)
	if _, err := scorecardRows(ci, bad, time.Now()); err == nil || !strings.Contains(err.Error(), "measured") {
		t.Errorf("a manual row without `measured` must fail the build, got %v", err)
	}
	rows, _ = scorecardRows(filepath.Join(dir, "nope.json"), man, time.Now())
	if len(rows) != 2 || rows[0].Pass || !strings.Contains(rows[0].Value, "not yet") {
		t.Errorf("missing CI file must yield an honest failing row: %+v", rows)
	}
}

// The KV fragment judges "stale" by the wall clock, not the commit time: a row measured long ago
// must read stale on a republish even when the repository has been quiet since. The build time is
// pinned to the rows' own day, years back, so a Scorecard that fell back to BuildTime would find
// nothing stale here, while the wall clock finds both rows stale.
func TestScorecardFragmentUsesWallClock(t *testing.T) {
	root := fixtureRoot(t)
	const epoch = 1577836800 // 2020-01-01T00:00:00Z
	t.Setenv("SOURCE_DATE_EPOCH", "1577836800")
	when := time.Unix(epoch, 0).UTC().Format("2006-01-02")
	ci := filepath.Join(root, "scorecard.json")
	_ = os.WriteFile(ci, []byte(`[{"check":"Lighthouse","pass":true,"value":"100","when":"`+when+`","link":"","link_text":""}]`), 0o644)
	man := filepath.Join(root, "manual.yaml")
	_ = os.WriteFile(man, []byte("rows:\n  - check: SSL Labs\n    value: A\n    when: "+when+"\n    measured: placeholder\n    link: https://x\n    link_text: x\n"), 0o644)
	out := filepath.Join(root, "scorecard.html")
	if err := Scorecard(Options{Root: root}, ci, man, out); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(out)
	if n := strings.Count(string(b), "(stale)"); n != 2 {
		t.Errorf("want both rows stale by the wall clock, got %d (stale) markers in:\n%s", n, b)
	}
}
