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
	p, body, line, _, probs := parseFrontMatter(file, src, lang, now, false)
	return p, body, line, probs
}

// The pillar a draft shows while its own is still blank; never valid in a published piece (it is not in Pillars).
const DraftPillar = "draft"

// parseFrontMatter is ParseFrontMatter with a draft mode for `site serve`: the fields `site new` leaves blank
// (title, date, pillar, summary) get visible defaults and come back as warnings instead of problems, so a piece
// can be previewed while it is written. Everything else is a problem in both modes.
func parseFrontMatter(file string, src []byte, lang string, now time.Time, draft bool) (*Piece, []byte, int, Problems, Problems) {
	var warnings, probs Problems
	if !bytes.HasPrefix(src, []byte("---\n")) {
		probs.Add(file, 1, "missing front matter (file must start with ---)")
		return nil, nil, 0, nil, probs
	}
	end := bytes.Index(src[4:], []byte("\n---\n"))
	if end < 0 {
		probs.Add(file, 1, "unterminated front matter")
		return nil, nil, 0, nil, probs
	}
	fmBytes := src[4 : 4+end]
	body := src[4+end+5:]
	bodyLine := bytes.Count(src[:4+end+5], []byte("\n")) + 1

	var fm frontMatter
	if err := yaml.Unmarshal(fmBytes, &fm); err != nil {
		probs.Add(file, 1, "front matter: %v", err)
		return nil, nil, 0, nil, probs
	}
	p := &Piece{Lang: lang, File: file, Title: strings.TrimSpace(fm.Title), Key: strings.TrimSpace(fm.Key),
		Pillar: strings.TrimSpace(fm.Pillar), Summary: strings.TrimSpace(fm.Summary),
		LinkedIn: strings.TrimSpace(fm.LinkedIn), Medium: strings.TrimSpace(fm.Medium), bodyLine: bodyLine}
	p.Slug = strings.TrimSuffix(filepath.Base(file), ".md")
	// blank reports a field the author still has to fill: a problem, or in draft mode a warning plus a default
	blank := func(msg string, fill func()) {
		if !draft {
			probs.Add(file, 1, "%s", msg)
			return
		}
		warnings.Add(file, 1, "%s (draft default in the preview)", msg)
		fill()
	}

	if fm.Draft != nil {
		probs.Add(file, 1, "draft: is not a field; drafts are unpublished branches on the private remote")
	}
	if p.Title == "" {
		blank("title is required", func() { p.Title = "(draft) " + p.Slug })
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
		blank(fmt.Sprintf("pillar %q must be one of %s", p.Pillar, strings.Join(Pillars, ", ")), func() { p.Pillar = DraftPillar })
	}
	if p.Summary == "" {
		blank("summary is required", func() { p.Summary = "(draft: summary not written yet)" })
	} else if n := utf8.RuneCountInString(p.Summary); n > summaryMax {
		probs.Add(file, 1, "summary is %d characters; max %d", n, summaryMax)
	}
	if strings.TrimSpace(fm.Date) == "" {
		blank("date is empty; /publish-piece stamps it", func() { p.Date = now })
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
	return p, body, bodyLine, warnings, probs
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
