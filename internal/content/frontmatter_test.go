package content

import (
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 9, 27, 12, 0, 0, 0, Bucharest)

const good = `---
title: "Which AI projects are worth funding in 2027"
date: 2026-09-27
key: ai-funding-2027
pillar: analysis
summary: "A filter for boards."
linkedin: https://www.linkedin.com/posts/x
---
Body here.
`

func TestParseGood(t *testing.T) {
	p, body, line, probs := ParseFrontMatter("content/en/which-ai.md", []byte(good), "en", now)
	if err := probs.Err(); err != nil {
		t.Fatal(err)
	}
	if p.Slug != "which-ai" || p.Key != "ai-funding-2027" || p.Pillar != "analysis" || p.Lang != "en" {
		t.Errorf("fields: %+v", p)
	}
	if p.Date.Year() != 2026 || p.Date.Location() != Bucharest {
		t.Errorf("date %v", p.Date)
	}
	if strings.TrimSpace(string(body)) != "Body here." || line != 9 {
		t.Errorf("body %q line %d", body, line)
	}
}

func TestParseProblems(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"no front matter", "Body", "front matter"},
		{"empty date", strings.Replace(good, "date: 2026-09-27", "date:", 1), "date"},
		{"future date", strings.Replace(good, "2026-09-27", "2026-09-28", 1), "after"},
		{"bad pillar", strings.Replace(good, "analysis", "rants", 1), "pillar"},
		{"long summary", strings.Replace(good, "A filter for boards.", strings.Repeat("x", 161), 1), "160"},
		{"missing key", strings.Replace(good, "key: ai-funding-2027\n", "", 1), "key"},
		{"draft field", strings.Replace(good, "pillar:", "draft: true\npillar:", 1), "draft"},
		{"cedilla in title", strings.Replace(good, "Which", "Reţea", 1), "comma-below"},
	}
	for _, c := range cases {
		_, _, _, probs := ParseFrontMatter("content/en/x.md", []byte(c.src), "en", now)
		if err := probs.Err(); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want error containing %q, got %v", c.name, c.want, err)
		}
	}
}

func TestBadSlugFromFilename(t *testing.T) {
	_, _, _, probs := ParseFrontMatter("content/en/Bad Slug.md", []byte(good), "en", now)
	if err := probs.Err(); err == nil || !strings.Contains(err.Error(), "slug") {
		t.Errorf("want slug error, got %v", err)
	}
}

func TestCheckDiacritics(t *testing.T) {
	probs := CheckDiacritics("f.md", []byte("ok ș\nbad ţ here\n"), 10)
	if len(probs) != 1 || probs[0].Line != 11 {
		t.Fatalf("got %+v", probs)
	}
}

// A discussion link points at one host, compared exactly: a look-alike, a bare domain, a subdomain, a userinfo
// trick or plain http is refused.
func TestDiscussionHosts(t *testing.T) {
	cases := []struct{ name, line, want string }{
		{"linkedin ok", "linkedin: https://www.linkedin.com/feed/update/urn:li:activity:1", ""},
		{"medium ok", "medium: https://medium.com/@someone/a-title-1", ""},
		{"linkedin look-alike", "linkedin: https://www.linkedin.com.evil.example/posts/x", "www.linkedin.com"},
		{"linkedin bare domain", "linkedin: https://linkedin.com/posts/x", "www.linkedin.com"},
		{"linkedin userinfo", "linkedin: https://www.linkedin.com@evil.example/x", "www.linkedin.com"},
		{"linkedin http", "linkedin: http://www.linkedin.com/posts/x", "https"},
		{"medium subdomain", "medium: https://evil.medium.com/x", "medium.com"},
	}
	for _, c := range cases {
		src := strings.Replace(good, "linkedin: https://www.linkedin.com/posts/x", c.line, 1)
		_, _, _, probs := ParseFrontMatter("content/en/x.md", []byte(src), "en", now)
		err := probs.Err()
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: want no problem, got %v", c.name, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
			t.Errorf("%s: want a problem containing %q, got %v", c.name, c.want, err)
		}
	}
}

func TestDiscussionField(t *testing.T) {
	for raw, want := range map[string]string{
		"https://www.linkedin.com/feed/update/urn:li:activity:1": "linkedin",
		"https://medium.com/@someone/a-title-1":                  "medium",
	} {
		if got, err := DiscussionField(raw); err != nil || got != want {
			t.Errorf("DiscussionField(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
	if _, err := DiscussionField("https://example.com/x"); err == nil {
		t.Error("DiscussionField(example.com): want an error")
	}
}
