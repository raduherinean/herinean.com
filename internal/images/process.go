// Package images produces responsive variants and OG cards. Pure Go: no ImageMagick, no cgo.
package images

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/gen2brain/webp"
	"golang.org/x/image/draw"
)

// Widths are the default responsive widths; the portrait passes its own.
var Widths = []int{720, 1440}

const URLPrefix = "/img/"

type Options struct {
	Widths []int  // defaults to Widths; the largest is never exceeded by upscaling
	Cache  *Cache // optional
}

type Info struct {
	Src    string // largest fallback, original format
	Srcset string // WebP candidates
	Width  int    // of the largest emitted variant
	Height int
	IsSVG  bool
	Files  map[string][]byte // dist-relative path → bytes
}

var (
	svgTag    = regexp.MustCompile(`(?is)<svg\b[^>]*>`)
	svgW      = regexp.MustCompile(`(?i)\bwidth="(\d+)(?:px)?"`)
	svgH      = regexp.MustCompile(`(?i)\bheight="(\d+)(?:px)?"`)
	svgStyle  = regexp.MustCompile(`(?i)\bstyle="`)
	svgScript = regexp.MustCompile(`(?i)<script`)
)

// Process reads one source image and returns everything the renderer and the writer need.
func Process(srcPath, key, name string, opt Options) (*Info, error) {
	raw, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, err
	}
	h := Hash8(raw)
	base := strings.TrimSuffix(name, path.Ext(name))
	ext := strings.ToLower(path.Ext(name))
	dir := "img/" + key + "/"

	if ext == ".svg" {
		if svgStyle.Match(raw) {
			return nil, fmt.Errorf("%s: SVG uses style= attributes; use presentation attributes and currentColor", srcPath)
		}
		if svgScript.Match(raw) {
			return nil, fmt.Errorf("%s: SVG contains a script", srcPath)
		}
		tag := svgTag.Find(raw)
		mw, mh := svgW.FindSubmatch(tag), svgH.FindSubmatch(tag)
		if tag == nil || mw == nil || mh == nil {
			return nil, fmt.Errorf("%s: SVG needs explicit width and height attributes on <svg>", srcPath)
		}
		w, _ := strconv.Atoi(string(mw[1]))
		hh, _ := strconv.Atoi(string(mh[1]))
		out := dir + base + "." + h + ".svg"
		return &Info{Src: "/" + out, Width: w, Height: hh, IsSVG: true, Files: map[string][]byte{out: raw}}, nil
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", srcPath, err)
	}
	if format != "png" && format != "jpeg" {
		return nil, fmt.Errorf("%s: only PNG, JPEG and SVG are supported (got %s)", srcPath, format)
	}
	// gen2brain/webp is transpiled WASM->Go, but at init() it tries to dlopen a host libwebp via
	// purego. CGO_ENABLED=0 does NOT prevent that: purego loads libraries through its own fakecgo
	// runtime on Linux, so a cgo-free binary still picks up a host libwebp when one is installed.
	// If that happened, webp.Encode uses the host library and its output depends on the host's
	// libwebp version, breaking "same commit => byte-identical dist/". Fail loudly instead of
	// silently drifting; the only remedy is to build with -tags nodynamic, which compiles the
	// dynamic loader out entirely.
	if webp.Dynamic() == nil {
		return nil, fmt.Errorf("webp: a host libwebp was loaded, so output would depend on this machine; build with -tags nodynamic for reproducible images")
	}
	bounds := image.Rect(0, 0, cfg.Width, cfg.Height)
	var decoded image.Image // decoded lazily: a full cache hit never decodes
	decode := func() (image.Image, error) {
		if decoded == nil {
			decoded, _, err = image.Decode(bytes.NewReader(raw))
		}
		return decoded, err
	}

	widths := opt.Widths
	if len(widths) == 0 {
		widths = Widths
	}
	widths = slices.Clone(widths)
	slices.Sort(widths) // callers may pass widths in any order; largest below must be genuinely the largest
	largest := widths[len(widths)-1]
	if cfg.Width < largest {
		largest = cfg.Width // never upscale
	}
	var ws []int
	for _, w := range widths {
		if w < largest {
			ws = append(ws, w)
		}
	}
	ws = append(ws, largest)

	info := &Info{Files: map[string][]byte{}}
	var srcset []string
	fallbackExt := map[string]string{"png": "png", "jpeg": "jpg"}[format]
	for _, w := range ws {
		p := fmt.Sprintf("%s%s.%s.%d.webp", dir, base, h, w)
		wb, ok := opt.Cache.get(p)
		if !ok {
			src, err := decode()
			if err != nil {
				return nil, fmt.Errorf("%s: %w", srcPath, err)
			}
			var buf bytes.Buffer
			if err := webp.Encode(&buf, resize(src, w), webp.Options{Quality: 82}); err != nil {
				return nil, fmt.Errorf("%s: webp: %w", srcPath, err)
			}
			wb = buf.Bytes()
			opt.Cache.put(p, wb)
		}
		info.Files[p] = wb
		srcset = append(srcset, fmt.Sprintf("/%s %dw", p, w))
		if w == largest {
			fp := fmt.Sprintf("%s%s.%s.%d.%s", dir, base, h, w, fallbackExt)
			fb, ok := opt.Cache.get(fp)
			if !ok {
				src, err := decode()
				if err != nil {
					return nil, fmt.Errorf("%s: %w", srcPath, err)
				}
				var buf bytes.Buffer
				if format == "png" {
					err = png.Encode(&buf, resize(src, w))
				} else {
					err = jpeg.Encode(&buf, resize(src, w), &jpeg.Options{Quality: 85})
				}
				if err != nil {
					return nil, err
				}
				fb = buf.Bytes()
				opt.Cache.put(fp, fb)
			}
			info.Files[fp] = fb
			info.Src = "/" + fp
			info.Width, info.Height = w, heightFor(bounds, w)
		}
	}
	info.Srcset = strings.Join(srcset, ", ")
	return info, nil
}

// heightFor keeps the aspect ratio with the same rounding resize uses, so dimensions are known without decoding.
func heightFor(b image.Rectangle, w int) int {
	if b.Dx() == w {
		return b.Dy()
	}
	return int(float64(b.Dy()) * float64(w) / float64(b.Dx()))
}

func resize(src image.Image, w int) image.Image {
	b := src.Bounds()
	if b.Dx() == w {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, heightFor(b, w)))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	return dst
}
