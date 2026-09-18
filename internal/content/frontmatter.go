package content

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

type frontMatter struct {
	Title    string `yaml:"title"`
	Date     string `yaml:"date"`
	Updated  string `yaml:"updated"`
	Key      string `yaml:"key"`
	Pillar   string `yaml:"pillar"`
	Summary  string `yaml:"summary"`
	LinkedIn string `yaml:"linkedin"`
	Medium   string `yaml:"medium"`
	Draft    *bool  `yaml:"draft"` // forbidden: drafts are unpublished branches
}

const summaryMax = 160

var cedilla = regexp.MustCompile("[ŞşŢţ]")

// ParseFrontMatter splits a piece file, validates the front matter and returns the Piece (without Body), the Markdown body and its first line number.
func ParseFrontMatter(file string, src []byte, lang string, now time.Time) (*Piece, []byte, int, Problems) {
	var probs Problems
	if !bytes.HasPrefix(src, []byte("---\n")) {
		probs.Add(file, 1, "missing front matter (file must start with ---)")
		return nil, nil, 0, probs
	}
	end := bytes.Index(src[4:], []byte("\n---\n"))
	if end < 0 {
		probs.Add(file, 1, "unterminated front matter")
		return nil, nil, 0, probs
	}
	fmBytes := src[4 : 4+end]
	body := src[4+end+5:]
	bodyLine := bytes.Count(src[:4+end+5], []byte("\n")) + 1

	var fm frontMatter
	if err := yaml.Unmarshal(fmBytes, &fm); err != nil {
		probs.Add(file, 1, "front matter: %v", err)
		return nil, nil, 0, probs
	}
	p := &Piece{Lang: lang, File: file, Title: strings.TrimSpace(fm.Title), Key: strings.TrimSpace(fm.Key),
		Pillar: strings.TrimSpace(fm.Pillar), Summary: strings.TrimSpace(fm.Summary),
		LinkedIn: strings.TrimSpace(fm.LinkedIn), Medium: strings.TrimSpace(fm.Medium), bodyLine: bodyLine}
	p.Slug = strings.TrimSuffix(filepath.Base(file), ".md")

	if fm.Draft != nil {
		probs.Add(file, 1, "draft: is not a field; drafts are unpublished branches on the private remote")
	}
	if p.Title == "" {
		probs.Add(file, 1, "title is required")
	}
	if cedilla.MatchString(p.Title) || cedilla.MatchString(p.Summary) {
		probs.Add(file, 1, "cedilla ş/ţ found; Romanian uses comma-below ș/ț")
	}
	if !ValidSlug(p.Slug) {
		probs.Add(file, 1, "slug %q (from the file name) must match [a-z0-9]+(-[a-z0-9]+)*", p.Slug)
	}
	if p.Key == "" {
		probs.Add(file, 1, "key is required (pairs the EN and RO versions)")
	} else if !ValidSlug(p.Key) {
		probs.Add(file, 1, "key %q must look like a slug", p.Key)
	}
	if !contains(Pillars, p.Pillar) {
		probs.Add(file, 1, "pillar %q must be one of %s", p.Pillar, strings.Join(Pillars, ", "))
	}
	if p.Summary == "" {
		probs.Add(file, 1, "summary is required")
	} else if n := utf8.RuneCountInString(p.Summary); n > summaryMax {
		probs.Add(file, 1, "summary is %d characters; max %d", n, summaryMax)
	}
	if strings.TrimSpace(fm.Date) == "" {
		probs.Add(file, 1, "date is empty; /publish-piece stamps it")
	} else if d, err := parseDate(fm.Date); err != nil {
		probs.Add(file, 1, "date: %v", err)
	} else {
		p.Date = d
		if d.After(now) {
			probs.Add(file, 1, "date %s is after the build/commit date %s", d.Format("2006-01-02"), now.In(Bucharest).Format("2006-01-02"))
		}
	}
	if strings.TrimSpace(fm.Updated) != "" {
		u, err := parseDate(fm.Updated)
		switch {
		case err != nil:
			probs.Add(file, 1, "updated: %v", err)
		case !p.Date.IsZero() && u.Before(p.Date):
			probs.Add(file, 1, "updated is before date")
		default:
			p.Updated = &u
		}
	}
	for _, u := range []string{p.LinkedIn, p.Medium} {
		if u != "" && !strings.HasPrefix(u, "https://") {
			probs.Add(file, 1, "%q must be an https URL", u)
		}
	}
	return p, body, bodyLine, probs
}

func parseDate(s string) (time.Time, error) {
	d, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(s), Bucharest)
	if err != nil {
		return time.Time{}, fmt.Errorf("want YYYY-MM-DD, got %q", s)
	}
	return d, nil
}

// CheckDiacritics reports cedilla forms in a body, one problem per offending line.
func CheckDiacritics(file string, body []byte, startLine int) Problems {
	var probs Problems
	for i, line := range bytes.Split(body, []byte("\n")) {
		if cedilla.Match(line) {
			probs.Add(file, startLine+i, "cedilla ş/ţ found; Romanian uses comma-below ș/ț")
		}
	}
	return probs
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}
