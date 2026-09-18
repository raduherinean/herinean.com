package content

import (
	"strings"
	"testing"
	"time"
)

var fixtureNow = time.Date(2026, 12, 31, 12, 0, 0, 0, Bucharest)

func TestLoadFixture(t *testing.T) {
	s, probs := Load("../../testdata/site", fixtureNow)
	if err := probs.Err(); err != nil {
		t.Fatal(err)
	}
	if len(s.Pieces) != 4 || len(s.ByLang["en"]) != 2 || len(s.ByLang["ro"]) != 2 {
		t.Fatalf("pieces: %d en %d ro %d", len(s.Pieces), len(s.ByLang["en"]), len(s.ByLang["ro"]))
	}
	for i := 1; i < len(s.Pieces); i++ {
		if s.Pieces[i].Date.After(s.Pieces[i-1].Date) {
			t.Error("pieces must be newest first")
		}
	}
	var paired, roOnly *Piece
	for _, p := range s.Pieces {
		switch p.Slug {
		case "paired":
			paired = p
		case "doar-ro":
			roOnly = p
		}
	}
	if paired.Translation == nil || paired.Translation.Slug != "pereche" || paired.Translation.Translation != paired {
		t.Error("pairing by key failed")
	}
	if roOnly.Translation != nil {
		t.Error("RO-only piece must have no translation")
	}
	for _, k := range []string{"home.en", "home.ro", "privacy.en", "privacy.ro", "colophon.en"} {
		if s.Pages[k] == nil {
			t.Errorf("missing page %s", k)
		}
	}
	if s.T("ro", "nav.writing") != "Articole" || s.T("en", "pillar.analysis") != "Analysis" {
		t.Error("strings")
	}
	if got := s.Latest(3); len(got) != 3 {
		t.Errorf("Latest(3) = %d", len(got))
	}
}

func TestLoadRendersBodies(t *testing.T) {
	s, _ := Load("../../testdata/site", fixtureNow)
	if err := s.Render(func(key, dest string) *ImageInfo { return nil }); err != nil {
		t.Fatal(err)
	}
	for _, p := range s.Pieces {
		if !strings.Contains(string(p.Body), "<p>") {
			t.Errorf("%s: body not rendered", p.File)
		}
	}
	if !strings.Contains(string(s.Pages["home.en"].Body), "<p>") {
		t.Error("home not rendered")
	}
}

func TestI18nCompleteness(t *testing.T) {
	_, probs := loadStrings("../../testdata/i18n-broken") // ro.yaml lacks footer.promise
	if err := probs.Err(); err == nil || !strings.Contains(err.Error(), "footer.promise") {
		t.Errorf("want missing-key error, got %v", err)
	}
}

func TestDuplicateKeyInOneLanguage(t *testing.T) {
	_, probs := Load("../../testdata/site-dupkey", fixtureNow) // two EN pieces with key: pair
	if err := probs.Err(); err == nil || !strings.Contains(err.Error(), "key \"pair\"") {
		t.Errorf("want duplicate-key error, got %v", err)
	}
}
