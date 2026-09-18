package site

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/raduherinean/herinean.com/internal/config"
	"github.com/raduherinean/herinean.com/internal/content"
	"github.com/raduherinean/herinean.com/internal/edge"
	"github.com/raduherinean/herinean.com/internal/feeds"
	"github.com/raduherinean/herinean.com/internal/images"
	"github.com/raduherinean/herinean.com/internal/render"
	"github.com/raduherinean/herinean.com/internal/seo"
)

type build struct {
	o           Options
	cfg         *config.Config
	site        *content.Site
	now         time.Time
	commit      string
	r           *render.Renderer
	fonts       *images.Fonts
	cache       *images.Cache
	imgs        map[string]map[string]*content.ImageInfo // key → dest → info
	og          map[string]string                        // "lang-slug" or "home-lang" → /og/… path
	files       map[string][]byte                        // dist-relative path → bytes
	portrait    *content.ImageInfo
	portraitRaw []byte // source bytes, for the home OG card
}

// load reads everything a build or a check needs. Placeholders (site.yaml or content/) are collected into an
// ErrAuthorInputs-wrapped error but do not stop content from loading: exit 3 is only for "placeholders are the only
// failures". A content problem returns nil and a plain (exit 1) error that still lists the placeholders after it.
// Otherwise the build is returned together with the author error, which may be non-nil.
func load(o Options) (*build, error) {
	now, err := BuildTime(o.Root)
	if err != nil {
		return nil, err
	}
	var author error
	cfg, err := config.Load(filepath.Join(o.Root, "site.yaml"))
	if err != nil {
		if !errors.Is(err, config.ErrPlaceholder) {
			return nil, err
		}
		author = fmt.Errorf("%w: %v", ErrAuthorInputs, err)
	}
	if pp := checkPlaceholders(o.Root); len(pp) > 0 {
		if author == nil {
			author = fmt.Errorf("%w:\n%v", ErrAuthorInputs, pp.Err())
		} else {
			author = fmt.Errorf("%w\n%v", author, pp.Err())
		}
	}
	s, probs := content.Load(o.Root, now)
	if err := probs.Err(); err != nil {
		if author != nil {
			return nil, fmt.Errorf("%v\n%v", err, author) // %v, not %w: a content problem is exit 1 even with placeholders around
		}
		return nil, err
	}
	return &build{o: o, cfg: cfg, site: s, now: now, commit: Commit(o.Root), cache: &images.Cache{Dir: filepath.Join(o.Root, ".cache")},
		files: map[string][]byte{}, imgs: map[string]map[string]*content.ImageInfo{}, og: map[string]string{}}, author
}

func Build(o Options) error {
	b, err := load(o)
	if err != nil {
		return err
	}
	if err := b.images(); err != nil {
		return err
	}
	if err := b.site.Render(b.lookup); err != nil {
		return err
	}
	b.r, err = render.New(filepath.Join(o.Root, "templates"), filepath.Join(o.Root, "assets", "css", "site.css"))
	if err != nil {
		return err
	}
	if err := b.ogImages(); err != nil {
		return err
	}
	if err := b.pages(); err != nil {
		return err
	}
	if err := b.machineFiles(); err != nil {
		return err
	}
	if err := b.static(); err != nil {
		return err
	}
	return b.write()
}

func (b *build) lookup(key, dest string) *content.ImageInfo {
	if m := b.imgs[key]; m != nil {
		return m[dest]
	}
	return nil
}

func (b *build) images() error {
	var probs content.Problems
	for key, refs := range b.site.AllImageRefs() {
		for _, ref := range refs {
			if b.imgs[key] != nil && b.imgs[key][ref.Dest] != nil {
				continue
			}
			src := filepath.Join(b.o.Root, "assets", "img", key, ref.Dest)
			info, err := images.Process(src, key, ref.Dest, images.Options{Cache: b.cache})
			if err != nil {
				probs.Add(src, ref.Line, "%v", err)
				continue
			}
			if b.imgs[key] == nil {
				b.imgs[key] = map[string]*content.ImageInfo{}
			}
			b.imgs[key][ref.Dest] = &content.ImageInfo{Src: info.Src, Srcset: info.Srcset, Width: info.Width, Height: info.Height, IsSVG: info.IsSVG}
			for p, data := range info.Files {
				b.files[p] = data
			}
		}
	}
	portrait := filepath.Join(b.o.Root, "assets", "portrait.jpg")
	b.portraitRaw, _ = os.ReadFile(portrait)
	// Displayed at ≤ 10rem (160 CSS px): 320 is 2×, 480 covers 3× phones. The default 720/1440 would ship a photo for a thumbnail
	// and blow row 14's 150 KB first view on the home page.
	info, err := images.Process(portrait, "home", "portrait.jpg", images.Options{Widths: []int{320, 480}, Cache: b.cache})
	if err != nil {
		probs.Add("assets/portrait.jpg", 0, "%v (the home page needs a portrait)", err)
	} else {
		b.portrait = &content.ImageInfo{Src: info.Src, Srcset: info.Srcset, Width: info.Width, Height: info.Height}
		for p, data := range info.Files {
			b.files[p] = data
		}
	}
	return probs.Err()
}

func (b *build) ogImages() error {
	var err error
	b.fonts, err = images.LoadFonts(filepath.Join(b.o.Root, "assets", "fonts", "og"))
	if err != nil {
		return err
	}
	put := func(id string, og images.OG) error {
		png, err := images.RenderOG(og, b.fonts, b.cache)
		if err != nil {
			return err
		}
		p := "og/" + id + "." + images.Hash8(png) + ".png"
		b.files[p] = png
		b.og[id] = "/" + p
		return nil
	}
	for _, p := range b.site.Pieces {
		if err := put(p.Lang+"-"+p.Slug, images.OG{Title: p.Title, Name: b.cfg.Name, Domain: strings.TrimPrefix(b.cfg.BaseURL, "https://"),
			Pillar: b.site.T(p.Lang, "pillar."+p.Pillar), Lang: strings.ToUpper(p.Lang)}); err != nil {
			return err
		}
	}
	for _, lang := range content.Langs {
		if err := put("home-"+lang, images.OG{Title: b.cfg.Tagline[lang], Name: b.cfg.Name, Domain: strings.TrimPrefix(b.cfg.BaseURL, "https://"), Lang: strings.ToUpper(lang), Portrait: b.portraitRaw}); err != nil {
			return err
		}
	}
	return nil
}

func (b *build) machineFiles() error {
	now := b.now
	for _, lang := range []string{"", "en", "ro"} {
		x, err := feeds.RSS(b.cfg, b.site, lang, now)
		if err != nil {
			return err
		}
		b.files[strings.TrimPrefix(b.cfg.FeedURL(lang), "/")] = x
	}
	j, err := feeds.JSON(b.cfg, b.site, now)
	if err != nil {
		return err
	}
	b.files["feed.json"] = j
	b.files["sitemap.xml"] = seo.Sitemap(b.sitemapEntries())
	b.files["robots.txt"] = seo.Robots(b.cfg)
	b.files["llms.txt"] = seo.LLMs(b.cfg, b.site)
	b.files[".well-known/security.txt"] = seo.SecurityTxt(b.cfg, now)
	b.files["_headers"] = edge.Headers(b.r.CSSHash())
	return nil
}

func (b *build) static() error {
	root := filepath.Join(b.o.Root, "static")
	return filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		b.files[filepath.ToSlash(rel)] = data
		return nil
	})
}

// deps lists the modules linked into this binary for the colophon, from the build info the Go linker embeds: the
// same commit and toolchain give the same list on every machine, with no toolchain needed at run time.
func deps() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "module list unavailable"
	}
	parts := make([]string, 0, len(bi.Deps))
	for _, d := range bi.Deps {
		if d.Replace != nil {
			d = d.Replace
		}
		parts = append(parts, d.Path+" "+d.Version)
	}
	return strings.Join(parts, " · ")
}

func goVersion() string { return runtime.Version() }

// write puts every file into <out>.tmp (sorted, mtimes = build time), then swaps it into place.
func (b *build) write() error {
	out := filepath.Join(b.o.Root, b.o.Out)
	tmp := out + ".tmp"
	_ = os.RemoveAll(tmp)
	paths := make([]string, 0, len(b.files))
	for p := range b.files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		full := filepath.Join(tmp, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, b.files[p], 0o644); err != nil {
			return err
		}
		if err := os.Chtimes(full, b.now, b.now); err != nil {
			return err
		}
	}
	_ = os.RemoveAll(out)
	if err := os.Rename(tmp, out); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "site: %d files → %s (commit %s, %s, %s)\n", len(paths), b.o.Out, b.commit, b.now.Format("2006-01-02"), runtime.Version())
	return nil
}
