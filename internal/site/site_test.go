package site

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeSiteYAML(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "site.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckPlaceholderReturnsErrAuthorInputs(t *testing.T) {
	dir := t.TempDir()
	writeSiteYAML(t, dir, `base_url: https://h.com
name: R
tagline: {en: "⟨fill me⟩", ro: "x"}
author: {linkedin: "a", x: "b", github: "c", email: "d"}
ai_disclosure: {en: "e", ro: "f"}`)

	err := Check(Options{Root: dir})
	if !errors.Is(err, ErrAuthorInputs) {
		t.Fatalf("Check() error = %v, want errors.Is(err, ErrAuthorInputs)", err)
	}
}

func TestCheckMissingFieldIsNotErrAuthorInputs(t *testing.T) {
	dir := t.TempDir()
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
