package site

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/raduherinean/herinean.com/internal/content"
)

func New(o Options, lang, slug string) error {
	if lang != "en" && lang != "ro" {
		return fmt.Errorf("lang must be en or ro")
	}
	if !content.ValidSlug(slug) {
		return fmt.Errorf("slug %q must match [a-z0-9]+(-[a-z0-9]+)*", slug)
	}
	dst := filepath.Join(o.Root, "content", lang, slug+".md")
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("%s already exists", dst)
	}
	tpl, err := os.ReadFile(filepath.Join(o.Root, "content", "_template.md"))
	if err != nil {
		return err
	}
	out := strings.ReplaceAll(string(tpl), "KEY", slug)
	if err := os.WriteFile(dst, []byte(out), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "site new: %s (branch piece/%s, fill title/pillar/summary; date is stamped by /publish-piece)\n", dst, slug)
	return nil
}
