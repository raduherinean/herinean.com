package images

import (
	"os"
	"path/filepath"
)

// Cache stores encoded outputs under <Dir>/v1/<name>. Nil or empty Dir disables it. Never part of dist/; gitignored.
type Cache struct{ Dir string }

func (c *Cache) get(name string) ([]byte, bool) {
	if c == nil || c.Dir == "" {
		return nil, false
	}
	b, err := os.ReadFile(filepath.Join(c.Dir, "v1", filepath.FromSlash(name)))
	return b, err == nil
}

func (c *Cache) put(name string, b []byte) {
	if c == nil || c.Dir == "" {
		return
	}
	p := filepath.Join(c.Dir, "v1", filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err == nil {
		_ = os.WriteFile(p, b, 0o644)
	}
}
