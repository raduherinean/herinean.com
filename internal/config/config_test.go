package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sample = `
base_url: https://herinean.com
name: Radu Herinean
tagline: {en: "Tag EN", ro: "Tag RO"}
author: {linkedin: "https://www.linkedin.com/in/x", x: "https://x.com/x", github: "https://github.com/rlucian", email: "security@herinean.com"}
ai_disclosure: {en: "AI EN", ro: "AI RO"}
`

func write(t *testing.T, s string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "site.yaml")
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadAndURLs(t *testing.T) {
	c, err := Load(write(t, sample))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		c.HomeURL("en"):           "/",
		c.HomeURL("ro"):           "/ro/",
		c.IndexURL("en"):          "/writing/",
		c.IndexURL("ro"):          "/ro/articole/",
		c.PieceURL("en", "hello"): "/writing/hello/",
		c.PieceURL("ro", "salut"): "/ro/articole/salut/",
		c.PrivacyURL("en"):        "/privacy/",
		c.PrivacyURL("ro"):        "/ro/confidentialitate/",
		c.Abs("/writing/hello/"):  "https://herinean.com/writing/hello/",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("got %q want %q", got, want)
		}
	}
}

func TestPlaceholdersRejected(t *testing.T) {
	_, err := Load(write(t, `base_url: https://h.com
name: R
tagline: {en: "⟨fill me⟩", ro: "x"}
author: {linkedin: "a", x: "b", github: "c", email: "d"}
ai_disclosure: {en: "e", ro: "f"}`))
	if err == nil {
		t.Fatal("expected placeholder error")
	}
}

func TestMissingFieldRejected(t *testing.T) {
	_, err := Load(write(t, `base_url: https://h.com
name: R`))
	if err == nil {
		t.Fatal("expected error for missing fields")
	}
}

// A missing field is a real error and must win over a placeholder elsewhere, so exit 3 is never handed out while
// site.yaml is also broken.
func TestMissingFieldBeatsPlaceholder(t *testing.T) {
	_, err := Load(write(t, `base_url: https://h.com
name: R
tagline: {en: "⟨fill me⟩", ro: "x"}
author: {linkedin: "a", x: "b", github: "c"}
ai_disclosure: {en: "e", ro: "f"}`))
	if err == nil {
		t.Fatal("expected an error")
	}
	if errors.Is(err, ErrPlaceholder) {
		t.Fatalf("error = %v, want NOT ErrPlaceholder while author.email is missing", err)
	}
	if !strings.Contains(err.Error(), "missing author.email") {
		t.Fatalf("error = %v, want the missing field named", err)
	}
}

// Every placeholder is listed, in struct order, and the parsed config still comes back.
func TestPlaceholdersListedInOrder(t *testing.T) {
	c, err := Load(write(t, `base_url: https://h.com
name: R
tagline: {en: "⟨fill me⟩", ro: "x"}
author: {linkedin: "⟨li⟩", x: "b", github: "c", email: "d"}
ai_disclosure: {en: "e", ro: "⟨ro⟩"}`))
	if !errors.Is(err, ErrPlaceholder) {
		t.Fatalf("error = %v, want ErrPlaceholder", err)
	}
	if !strings.Contains(err.Error(), "tagline.en, author.linkedin, ai_disclosure.ro still contain a ⟨placeholder⟩") {
		t.Fatalf("error = %v, want all three keys in order", err)
	}
	if c == nil || c.Name != "R" {
		t.Fatalf("config = %+v, want the parsed config alongside the placeholder error", c)
	}
}
