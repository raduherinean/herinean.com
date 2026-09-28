package site

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/raduherinean/herinean.com/internal/content"
)

// Stamp writes now's date in Europe/Bucharest into a piece's date: — or, with updated, its updated: (a correction;
// date: stays) — and returns the old and new values. It edits a source file, never dist/: wall-clock time enters
// the site only through the date the author commits.
func Stamp(file string, updated bool, now time.Time) (old, stamped string, err error) {
	stamped = now.In(content.Bucharest).Format("2006-01-02")
	if !updated {
		old, err = setField(file, "date", stamped, "title")
		return old, stamped, err
	}
	date, err := readField(file, "date")
	if err != nil {
		return "", "", err
	}
	if date == "" {
		return "", "", fmt.Errorf("%s: date is empty; a correction is for a published piece (stamp without --updated)", file)
	}
	old, err = setField(file, "updated", stamped, "date")
	return old, stamped, err
}

// Link sets a published piece's discussion URL — linkedin: or medium:, chosen by the URL's host — and returns the
// field and its old value.
func Link(file, rawURL string) (field, old string, err error) {
	field, err = content.DiscussionField(rawURL)
	if err != nil {
		return "", "", err
	}
	date, err := readField(file, "date")
	if err != nil {
		return "", "", err
	}
	if date == "" {
		return "", "", fmt.Errorf("%s: date is empty; link a published piece", file)
	}
	old, err = setField(file, field, rawURL, "linkedin", "summary")
	return field, old, err
}

// frontMatter splits src into lines, each keeping its ending, and returns the index of the closing "---" line.
func frontMatter(file string, src []byte) (lines [][]byte, end int, err error) {
	lines = bytes.SplitAfter(src, []byte("\n"))
	if string(bytes.TrimRight(lines[0], "\r\n")) != "---" {
		return nil, 0, fmt.Errorf("%s: no front matter", file)
	}
	for i := 1; i < len(lines); i++ {
		if string(bytes.TrimRight(lines[i], "\r\n")) == "---" {
			return lines, i, nil
		}
	}
	return nil, 0, fmt.Errorf("%s: front matter is not closed", file)
}

// fieldLine returns the index of the front-matter line `name: …`, or -1.
func fieldLine(lines [][]byte, end int, name string) int {
	for i := 1; i < end; i++ {
		if bytes.HasPrefix(lines[i], []byte(name+":")) {
			return i
		}
	}
	return -1
}

// fieldValue is the value on a `name: value` line; empty when blank or only a comment.
func fieldValue(line []byte, name string) string {
	v := strings.TrimSpace(strings.TrimPrefix(string(bytes.TrimRight(line, "\r\n")), name+":"))
	if strings.HasPrefix(v, "#") {
		return ""
	}
	return strings.Trim(v, `"'`)
}

// readField returns a front-matter field's value; empty when absent or blank.
func readField(file, name string) (string, error) {
	src, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	lines, end, err := frontMatter(file, src)
	if err != nil {
		return "", err
	}
	if i := fieldLine(lines, end, name); i >= 0 {
		return fieldValue(lines[i], name), nil
	}
	return "", nil
}

// setField writes `name: value` into a piece's front matter — replacing the line when it exists, else inserting it
// after the first of `after` that exists, else before the closing `---` — and keeps every other byte, line endings
// included. It returns the old value.
func setField(file, name, value string, after ...string) (old string, err error) {
	st, err := os.Stat(file)
	if err != nil {
		return "", err
	}
	src, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	lines, end, err := frontMatter(file, src)
	if err != nil {
		return "", err
	}
	eol := "\n"
	if bytes.HasSuffix(lines[0], []byte("\r\n")) {
		eol = "\r\n"
	}
	line := []byte(name + ": " + value + eol)
	if i := fieldLine(lines, end, name); i >= 0 {
		old = fieldValue(lines[i], name)
		lines[i] = line
	} else {
		at := end
		for _, a := range after {
			if i := fieldLine(lines, end, a); i >= 0 {
				at = i + 1
				break
			}
		}
		lines = append(lines[:at:at], append([][]byte{line}, lines[at:]...)...)
	}
	return old, os.WriteFile(file, bytes.Join(lines, nil), st.Mode().Perm())
}
