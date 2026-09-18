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
