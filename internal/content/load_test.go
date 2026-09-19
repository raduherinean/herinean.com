package content

import (
	"os"
	"path/filepath"
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
}

// ForLang lists one piece per key: the viewer language's version when it exists, the other language's
// otherwise (it carries the badge). Newest first by the listed piece's date. The fixture holds an EN-only
// piece, a RO-only piece and one pair, so each view has three entries and the pair appears once.
func TestForLangOneEntryPerKey(t *testing.T) {
	s, probs := Load("../../testdata/site", fixtureNow)
	if err := probs.Err(); err != nil {
		t.Fatal(err)
	}
	slugs := func(ps []*Piece) []string {
		var out []string
		for _, p := range ps {
			out = append(out, p.Lang+"/"+p.Slug)
		}
		return out
	}
	want := map[string][]string{
		"en": {"en/with-media", "ro/doar-ro", "en/paired"},
		"ro": {"en/with-media", "ro/doar-ro", "ro/pereche"},
	}
	for lang, w := range want {
		got := slugs(s.ForLang(lang))
		if strings.Join(got, " ") != strings.Join(w, " ") {
			t.Errorf("ForLang(%q) = %v, want %v", lang, got, w)
		}
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

// What `site new` writes: every author field blank, key = slug.
const draftPiece = "---\ntitle: \"\"\ndate:\nkey: draft-piece\npillar:\nsummary: \"\"\n---\n\n## Situation\n\nStill writing.\n"

// `site serve` must show a piece the author is still writing (spec §8: "write on site serve"), so LoadDraft renders
// the blanks `site new` leaves with visible defaults and reports them as warnings; Load keeps refusing them.
func TestLoadDraftRendersBlankFrontMatter(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../testdata/site")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "content", "en", "draft-piece.md"), []byte(draftPiece), 0o644); err != nil {
		t.Fatal(err)
	}
	_, probs := Load(root, fixtureNow)
	if err := probs.Err(); err == nil || !strings.Contains(err.Error(), "date is empty") {
		t.Fatalf("Load: want the empty date refused, got %v", err)
	}
	s, warnings, probs := LoadDraft(root, fixtureNow)
	if err := probs.Err(); err != nil {
		t.Fatalf("LoadDraft: problems %v, want none", err)
	}
	if len(warnings) != 4 {
		t.Fatalf("LoadDraft: %d warnings, want 4 (title, date, pillar, summary): %+v", len(warnings), warnings)
	}
	var d *Piece
	for _, p := range s.Pieces {
		if p.Slug == "draft-piece" {
			d = p
		}
	}
	if d == nil {
		t.Fatal("draft piece not loaded")
	}
	if d.Title != "(draft) draft-piece" || !d.Date.Equal(fixtureNow) || d.Pillar != "draft" || d.Summary == "" {
		t.Errorf("draft defaults: title %q date %s pillar %q summary %q", d.Title, d.Date, d.Pillar, d.Summary)
	}
	if s.Pieces[0] != d {
		t.Error("a draft dated at build time must sort first")
	}
	if s.T("en", "pillar.draft") == "" {
		t.Error("pillar.draft label missing")
	}
}

func TestDuplicateKeyInOneLanguage(t *testing.T) {
	_, probs := Load("../../testdata/site-dupkey", fixtureNow) // two EN pieces with key: pair
	if err := probs.Err(); err == nil || !strings.Contains(err.Error(), "key \"pair\"") {
		t.Errorf("want duplicate-key error, got %v", err)
	}
}
