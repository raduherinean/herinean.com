package site

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSiteYAML(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "site.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const placeholderSiteYAML = `base_url: https://h.com
name: R
tagline: {en: "⟨fill me⟩", ro: "x"}
author: {linkedin: "a", x: "b", github: "c", email: "d"}
ai_disclosure: {en: "e", ro: "f"}`

// A placeholder in site.yaml on an otherwise valid site is the only failure, so it is exit 3.
func TestCheckPlaceholderReturnsErrAuthorInputs(t *testing.T) {
	root := fixtureRoot(t)
	writeSiteYAML(t, root, placeholderSiteYAML)

	err := Check(Options{Root: root})
	if !errors.Is(err, ErrAuthorInputs) {
		t.Fatalf("Check() error = %v, want errors.Is(err, ErrAuthorInputs)", err)
	}
}

// Exit 3 only when placeholders are the only failures: a broken piece next to a placeholder is exit 1, and its
// problem must be visible in the message rather than masked by the placeholder.
func TestCheckContentProblemIsNotMaskedByPlaceholders(t *testing.T) {
	root := fixtureRoot(t)
	writeSiteYAML(t, root, placeholderSiteYAML)
	piece := filepath.Join(root, "content", "ro", "cedila.md")
	front := "---\ntitle: \"Un articol\"\ndate: 2026-09-01\nkey: cedilla\npillar: analysis\nsummary: \"Rezumat scurt.\"\n---\n"
	if err := os.WriteFile(piece, []byte(front+"Un rând cu ţ cu sedilă.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Check(Options{Root: root})
	if err == nil {
		t.Fatal("Check() = nil, want the cedilla problem")
	}
	if errors.Is(err, ErrAuthorInputs) {
		t.Fatalf("Check() error = %v, want NOT ErrAuthorInputs while a content problem exists", err)
	}
	if !strings.Contains(err.Error(), "cedilla") {
		t.Fatalf("Check() error = %v, want the cedilla problem named", err)
	}

	if err := os.WriteFile(piece, []byte(front+"Un rând cu ț cu virgulă.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = Check(Options{Root: root})
	if !errors.Is(err, ErrAuthorInputs) {
		t.Fatalf("Check() error = %v after fixing the piece, want errors.Is(err, ErrAuthorInputs)", err)
	}
}

func TestCheckMissingFieldIsNotErrAuthorInputs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SOURCE_DATE_EPOCH", "1790000000")
	writeSiteYAML(t, dir, `base_url: https://h.com
name: R`)

	err := Check(Options{Root: dir})
	if err == nil {
		t.Fatal("Check() error = nil, want an error for missing fields")
	}
	if errors.Is(err, ErrAuthorInputs) {
		t.Fatalf("Check() error = %v, want an error that is NOT ErrAuthorInputs", err)
	}
}

// A template that does not parse is a real problem (exit 1), found by check rather than only by build.
func TestCheckParsesTemplates(t *testing.T) {
	root := fixtureRoot(t)
	if err := os.WriteFile(filepath.Join(root, "templates", "piece.html"), []byte("{{define \"main\"}}{{.Unclosed{{end}}"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Check(Options{Root: root})
	if err == nil {
		t.Fatal("Check() = nil, want a template parse problem")
	}
	if errors.Is(err, ErrAuthorInputs) {
		t.Fatalf("Check() error = %v, want an error that is NOT ErrAuthorInputs", err)
	}
	if !strings.Contains(err.Error(), "template") {
		t.Fatalf("Check() error = %v, want the template named", err)
	}
}

// The portrait is an author input (spec §14): without it, Check is exit 3, not exit 1 — and it stays exit 3 next to
// a placeholder, with both named.
func TestCheckMissingPortraitIsAuthorInput(t *testing.T) {
	root := fixtureRoot(t)
	if err := os.Remove(filepath.Join(root, "assets", "portrait.jpg")); err != nil {
		t.Fatal(err)
	}

	err := Check(Options{Root: root})
	if !errors.Is(err, ErrAuthorInputs) {
		t.Fatalf("Check() error = %v, want errors.Is(err, ErrAuthorInputs)", err)
	}
	if !strings.Contains(err.Error(), "portrait") {
		t.Fatalf("Check() error = %v, want the portrait named", err)
	}

	writeSiteYAML(t, root, placeholderSiteYAML)
	err = Check(Options{Root: root})
	if !errors.Is(err, ErrAuthorInputs) {
		t.Fatalf("Check() error = %v, want errors.Is(err, ErrAuthorInputs) with a placeholder and no portrait", err)
	}
	for _, want := range []string{"portrait", "tagline.en"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Check() error lacks %q:\n%v", want, err)
		}
	}
}

const draftPiece = "---\ntitle: \"\"\ndate:\nkey: ciorna\npillar:\nsummary: \"\"\n---\n\n## Situation\n\nStill writing.\n"

func writePiece(t *testing.T, root, lang, slug, src string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "content", lang, slug+".md"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

// check --draft is what the hook runs on piece/* branches: the fields `site new` leaves blank are warnings there.
func TestCheckDraftAllowsBlankFields(t *testing.T) {
	root := fixtureRoot(t)
	writePiece(t, root, "ro", "ciorna", draftPiece)
	if err := Check(Options{Root: root}); err == nil || !strings.Contains(err.Error(), "date is empty") {
		t.Fatalf("Check: want the blank date refused, got %v", err)
	}
	if err := Check(Options{Root: root, Draft: true}); err != nil {
		t.Fatalf("Check(Draft): want nil, got %v", err)
	}
}

// Every other rule still fails under --draft: the mode forgives what `site new` leaves blank, nothing else.
func TestCheckDraftStillFailsOtherRules(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"long summary", strings.Replace(draftPiece, `summary: ""`, `summary: "`+strings.Repeat("x", 161)+`"`, 1), "160"},
		{"cedilla", draftPiece + "\nReţea.\n", "comma-below"},
		{"future date", strings.Replace(draftPiece, "date:\n", "date: 2026-09-30\n", 1), "after"},
		{"draft field", strings.Replace(draftPiece, "pillar:\n", "pillar:\ndraft: true\n", 1), "draft"},
	}
	for _, c := range cases {
		root := fixtureRoot(t)
		writePiece(t, root, "ro", "ciorna", c.src)
		if err := Check(Options{Root: root, Draft: true}); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: Check(Draft) = %v, want an error containing %q", c.name, err, c.want)
		}
	}
}

func appendSiteYAML(t *testing.T, root, line string) {
	t.Helper()
	p := filepath.Join(root, "site.yaml")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(b, []byte("\n"+line+"\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Pieces merged before the launch go live on launch day; their date must say so, or datePublished names a day on
// which the site was not public. The fixture's pieces are dated 2026-09-05 … 2026-09-15.
func TestLaunchedRuleRefusesEarlierPieces(t *testing.T) {
	root := fixtureRoot(t)
	appendSiteYAML(t, root, "launched: 2026-09-11")
	err := Check(Options{Root: root})
	if err == nil || !strings.Contains(err.Error(), "before launched 2026-09-11") {
		t.Fatalf("Check: want pieces dated before the launch refused, got %v", err)
	}
	if err := Build(Options{Root: root, Out: "dist"}); err == nil {
		t.Fatal("Build: want the same refusal")
	}
}

// A piece dated on launch day is fine: the rule is "not before".
func TestLaunchedRuleAcceptsLaunchDay(t *testing.T) {
	root := fixtureRoot(t)
	appendSiteYAML(t, root, "launched: 2026-09-05")
	if err := Check(Options{Root: root}); err != nil {
		t.Fatalf("Check: want nil, got %v", err)
	}
}

// controller ruling: a blank date in non-draft mode is already refused ("date is empty"); with a zero Date, the
// launch-date loop must not also report a misleading "0001-01-01 is before launched".
func TestLaunchedRuleSkipsBlankDate(t *testing.T) {
	root := fixtureRoot(t)
	appendSiteYAML(t, root, "launched: 2026-09-01")
	writePiece(t, root, "ro", "ciorna", draftPiece)
	err := Check(Options{Root: root})
	if err == nil || !strings.Contains(err.Error(), "date is empty") {
		t.Fatalf("Check: want the blank date refused, got %v", err)
	}
	if strings.Contains(err.Error(), "before launched") {
		t.Fatalf("Check: want no misleading \"before launched\" for a blank date, got %v", err)
	}
}
