package images

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
)

// Cache stores encoded outputs under <Dir>/<cacheVersion>/<name>. Nil or empty Dir disables it. Never part of dist/;
// gitignored.
type Cache struct{ Dir string }

// cacheVersion folds the encoder identity into the cache root: a cached WebP/PNG/JPEG/OG byte stream is only valid for
// the toolchain, the exact module versions and the in-repo settings (quality, resize kernel, OG layout) that produced
// it. Changing any of them therefore starts a fresh cache instead of serving bytes an older encoder produced, which
// would make dist/ depend on what happened to be cached rather than on the commit. "v1-" is the layout version of the
// entries themselves.
var cacheVersion = cacheVersionFor(encodeParams())

func cacheVersionFor(params string) string {
	return "v1-" + Hash8([]byte(runtime.Version()+" "+encoderVersions()+" "+params))
}

// encodeParams names every in-repo setting whose change alters encoded bytes.
func encodeParams() string {
	return fmt.Sprintf("webp=%d jpeg=%d resize=catmullrom og=%d", webpQuality, jpegQuality, ogLayout)
}

// encoderVersions returns the linked versions of the modules whose output the cache stores, or "unknown" when the
// binary carries no build info (e.g. some test harnesses).
func encoderVersions() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	out := ""
	for _, want := range []string{"github.com/gen2brain/webp", "golang.org/x/image"} {
		v := "unknown"
		for _, d := range bi.Deps {
			if d.Replace != nil {
				d = d.Replace
			}
			if d.Path == want {
				v = d.Version
				break
			}
		}
		out += want + "@" + v + " "
	}
	return out
}

func (c *Cache) get(name string) ([]byte, bool) {
	if c == nil || c.Dir == "" {
		return nil, false
	}
	b, err := os.ReadFile(filepath.Join(c.Dir, cacheVersion, filepath.FromSlash(name)))
	return b, err == nil
}

func (c *Cache) put(name string, b []byte) {
	if c == nil || c.Dir == "" {
		return
	}
	p := filepath.Join(c.Dir, cacheVersion, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err == nil {
		_ = os.WriteFile(p, b, 0o644)
	}
}
