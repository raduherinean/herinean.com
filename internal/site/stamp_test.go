package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/raduherinean/herinean.com/internal/content"
)

// 22:30 UTC on 26 September is 01:30 on the 27th in Bucharest (EEST, UTC+3).
var lateEvening = time.Date(2026, 9, 26, 22, 30, 0, 0, time.UTC)

const unstamped = "---\ntitle: \"T\"\ndate:\nkey: k\npillar: analysis\nsummary: \"S\"\n---\n\ndate: in the body is prose, not a field.\n"

var published = strings.Replace(unstamped, "date:\n", "date: 2026-09-20\n", 1)

func pieceFile(t *testing.T, src string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "t.md")
	if err := os.WriteFile(f, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return f
}

func readFile(t *testing.T, f string) string {
	t.Helper()
	b, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestStampWritesTheBucharestDate(t *testing.T) {
	f := pieceFile(t, unstamped)
	old, got, err := Stamp(f, false, lateEvening)
	if err != nil || old != "" || got != "2026-09-27" {
		t.Fatalf("Stamp = %q, %q, %v; want \"\", 2026-09-27, nil", old, got, err)
	}
	want := strings.Replace(unstamped, "date:\n", "date: 2026-09-27\n", 1) // the body's "date:" line is untouched
	if s := readFile(t, f); s != want {
		t.Fatalf("file:\n%s\nwant:\n%s", s, want)
	}
	if _, _, _, probs := content.ParseFrontMatter("content/en/t.md", []byte(want), "en", lateEvening); probs.Err() != nil {
		t.Fatalf("stamped piece does not parse: %v", probs.Err())
	}
}

func TestStampRestamps(t *testing.T) {
	f := pieceFile(t, published)
	old, _, err := Stamp(f, false, lateEvening)
	if err != nil || old != "2026-09-20" || !strings.Contains(readFile(t, f), "date: 2026-09-27\n") {
		t.Fatalf("Stamp = %q, %v; file:\n%s", old, err, readFile(t, f))
	}
}

func TestStampUpdatedAddsTheLineAndKeepsDate(t *testing.T) {
	f := pieceFile(t, published)
	if _, _, err := Stamp(f, true, lateEvening); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(published, "date: 2026-09-20\n", "date: 2026-09-20\nupdated: 2026-09-27\n", 1)
	if s := readFile(t, f); s != want {
		t.Fatalf("file:\n%s\nwant:\n%s", s, want)
	}
}

func TestStampUpdatedReplaces(t *testing.T) {
	f := pieceFile(t, strings.Replace(published, "key: k\n", "updated: 2026-09-21\nkey: k\n", 1))
	old, _, err := Stamp(f, true, lateEvening)
	if err != nil || old != "2026-09-21" || !strings.Contains(readFile(t, f), "updated: 2026-09-27\n") {
		t.Fatalf("Stamp = %q, %v; file:\n%s", old, err, readFile(t, f))
	}
}

func TestStampUpdatedRefusesAnUnpublishedPiece(t *testing.T) {
	f := pieceFile(t, unstamped)
	if _, _, err := Stamp(f, true, lateEvening); err == nil || !strings.Contains(err.Error(), "date is empty") {
		t.Fatalf("Stamp(updated) on an undated piece: want refusal, got %v", err)
	}
	if readFile(t, f) != unstamped {
		t.Fatal("refused stamp changed the file")
	}
}

func TestStampKeepsCRLF(t *testing.T) {
	f := pieceFile(t, strings.ReplaceAll(unstamped, "\n", "\r\n"))
	if _, _, err := Stamp(f, false, lateEvening); err != nil {
		t.Fatal(err)
	}
	want := strings.ReplaceAll(strings.Replace(unstamped, "date:\n", "date: 2026-09-27\n", 1), "\n", "\r\n")
	if s := readFile(t, f); s != want {
		t.Fatalf("file %q, want %q", s, want)
	}
}

func TestStampRefusesWithoutFrontMatter(t *testing.T) {
	f := pieceFile(t, "Just text.\n")
	if _, _, err := Stamp(f, false, lateEvening); err == nil || !strings.Contains(err.Error(), "no front matter") {
		t.Fatalf("want refusal, got %v", err)
	}
}

func TestLinkSetsTheFieldByHost(t *testing.T) {
	f := pieceFile(t, published)
	li := "https://www.linkedin.com/feed/update/urn:li:activity:1"
	field, old, err := Link(f, li)
	if err != nil || field != "linkedin" || old != "" {
		t.Fatalf("Link = %q, %q, %v", field, old, err)
	}
	if !strings.Contains(readFile(t, f), "summary: \"S\"\nlinkedin: "+li+"\n---\n") {
		t.Fatalf("linkedin: not inserted after summary:\n%s", readFile(t, f))
	}
	md := "https://medium.com/@someone/t-1"
	if field, _, err := Link(f, md); err != nil || field != "medium" {
		t.Fatalf("Link(medium) = %q, %v", field, err)
	}
	if !strings.Contains(readFile(t, f), "linkedin: "+li+"\nmedium: "+md+"\n---\n") {
		t.Fatalf("medium: not inserted after linkedin:\n%s", readFile(t, f))
	}
}

func TestLinkReplaces(t *testing.T) {
	f := pieceFile(t, strings.Replace(published, "summary: \"S\"\n", "summary: \"S\"\nlinkedin: https://www.linkedin.com/posts/old\n", 1))
	_, old, err := Link(f, "https://www.linkedin.com/posts/new")
	if err != nil || old != "https://www.linkedin.com/posts/old" || strings.Contains(readFile(t, f), "/posts/old") {
		t.Fatalf("Link = %q, %v; file:\n%s", old, err, readFile(t, f))
	}
}

func TestLinkRefusesALookAlikeHostAndAnUnpublishedPiece(t *testing.T) {
	f := pieceFile(t, published)
	if _, _, err := Link(f, "https://www.linkedin.com.evil.example/x"); err == nil {
		t.Error("look-alike host: want refusal")
	}
	g := pieceFile(t, unstamped)
	if _, _, err := Link(g, "https://www.linkedin.com/posts/x"); err == nil || !strings.Contains(err.Error(), "link a published piece") {
		t.Errorf("unpublished piece: want refusal, got %v", err)
	}
	if readFile(t, f) != published || readFile(t, g) != unstamped {
		t.Error("a refused link changed a file")
	}
}
