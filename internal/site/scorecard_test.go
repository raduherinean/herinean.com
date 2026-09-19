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
