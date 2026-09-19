package site

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/raduherinean/herinean.com/internal/content"
)

// BuildTime is the commit time: SOURCE_DATE_EPOCH if set, else the HEAD commit's timestamp. Wall-clock never enters the output.
func BuildTime(root string) (time.Time, error) {
	if v := os.Getenv("SOURCE_DATE_EPOCH"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("SOURCE_DATE_EPOCH: %w", err)
		}
		return time.Unix(n, 0).In(content.Bucharest), nil
	}
	out, err := exec.Command("git", "-C", root, "log", "-1", "--format=%ct").Output()
	if err != nil {
		return time.Time{}, fmt.Errorf("build time: set SOURCE_DATE_EPOCH or build inside a git checkout (%v)", err)
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(n, 0).In(content.Bucharest), nil
}

func Commit(root string) string {
	out, err := exec.Command("git", "-C", root, "rev-parse", "--short=10", "HEAD").Output()
	if err != nil {
		return "uncommitted"
	}
	return strings.TrimSpace(string(out))
}

// fileTime is the last commit that touched rel, so a page's sitemap lastmod only moves when the page does. Outside git: fallback.
func fileTime(root, rel string, fallback time.Time) time.Time {
	out, err := exec.Command("git", "-C", root, "log", "-1", "--format=%ct", "--", rel).Output()
	if err != nil {
		return fallback
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil || n == 0 {
		return fallback
	}
	return time.Unix(n, 0).In(content.Bucharest)
}
