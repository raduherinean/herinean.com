package config

import (
	"os"
	"path/filepath"
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
