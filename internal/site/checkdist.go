package site

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/raduherinean/herinean.com/internal/config"
	"github.com/raduherinean/herinean.com/internal/content"
)

const htmlBudget = 30 * 1024 // gzip bytes, HTML with inlined CSS

// Attribute rules are anchored on the opening quote so an HTML-escaped code sample (&#34; / &quot;) never matches;
// the unquoted forms are left to the CSP, which blocks them anyway.
var (
	reScript   = regexp.MustCompile(`(?is)<script\b([^>]*)>`)
	reStyleAtt = regexp.MustCompile(`(?i)\sstyle\s*=\s*["']`)
	reHandler  = regexp.MustCompile(`(?i)\son[a-z]+\s*=\s*["']`)
	reJSURL    = regexp.MustCompile(`(?i)\s(href|src)\s*=\s*["']\s*javascript:`)
	reStyleTag = regexp.MustCompile(`(?is)<style>(.*?)</style>`)
	reOGImage  = regexp.MustCompile(`property="og:image" content="([^"]+)"`)
	reHash     = regexp.MustCompile(`style-src '(sha256-[^']+)'`)
	rePre      = regexp.MustCompile(`<pre\b[^>]*>`)
)

// headerBlocks splits a Cloudflare _headers file into its rule blocks: a line that does not start with a space opens
// a block for that path pattern; the indented lines under it are its headers. Blank lines end a block.
func headerBlocks(headers []byte) map[string]string {
	blocks := map[string]string{}
	cur := ""
	for _, line := range strings.Split(string(headers), "\n") {
		switch {
		case strings.TrimSpace(line) == "":
			cur = ""
		case !strings.HasPrefix(line, " "):
			cur = strings.TrimSpace(line)
			blocks[cur] += line + "\n"
		case cur != "":
			blocks[cur] += line + "\n"
		}
	}
	return blocks
}

// checkHeaderCoverage is the spec's "_headers covers every path class" rule: for each class the build emitted, the
// block for exactly that pattern must carry the header that defines the class.
func checkHeaderCoverage(dist string, headers []byte, probs *content.Problems) {
	blocks := headerBlocks(headers)
	exists := func(rel string) bool {
		_, err := os.Stat(filepath.Join(dist, filepath.FromSlash(rel)))
		return err == nil
	}
	require := func(pattern string, must ...string) {
		for _, h := range must {
			if !strings.Contains(blocks[pattern], h) {
				probs.Add("_headers", 0, "no rule for %s with %s", pattern, h)
			}
		}
	}
	for _, dir := range []string{"img", "og"} {
		if exists(dir) {
			require("/"+dir+"/*", "Cross-Origin-Resource-Policy: cross-origin", "max-age=31536000, immutable")
		}
	}
	for _, f := range []string{"feed.xml", "feed.en.xml", "feed.ro.xml"} {
		if exists(f) {
			require("/"+f, "Content-Type: application/rss+xml", "max-age=300")
		}
	}
	if exists("feed.json") {
		require("/feed.json", "application/feed+json", "max-age=300")
	}
	if exists(".well-known/security.txt") {
		require("/.well-known/security.txt", "Content-Type: text/plain")
	}
}

func CheckDist(o Options) error {
	dist := filepath.Join(o.Root, o.Out)
	cfg, err := config.Load(filepath.Join(o.Root, "site.yaml"))
	if err != nil {
		return err
	}
	var probs content.Problems
	headers, err := os.ReadFile(filepath.Join(dist, "_headers"))
	if err != nil {
		return fmt.Errorf("dist: %w (run site build first)", err)
	}
	if strings.Contains(strings.ToLower(string(headers)), "x-robots-tag") {
		probs.Add("_headers", 0, "must not set X-Robots-Tag on production")
	}
	hm := reHash.FindSubmatch(headers)
	if hm == nil {
		probs.Add("_headers", 0, "no style-src hash in the CSP")
	}
	checkHeaderCoverage(dist, headers, &probs)
	var htmlPaths []string
	_ = filepath.WalkDir(dist, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".html") {
			return nil
		}
		rel, _ := filepath.Rel(dist, p)
		htmlPaths = append(htmlPaths, "/"+filepath.ToSlash(rel))
		b, _ := os.ReadFile(p)
		s := string(b)
		for _, m := range reScript.FindAllStringSubmatch(s, -1) {
			if !strings.Contains(m[1], `type="application/ld+json"`) {
				probs.Add(rel, 0, "executable <script> found")
			}
		}
		if reStyleAtt.MatchString(s) {
			probs.Add(rel, 0, "style= attribute found (hash CSP forbids it)")
		}
		if reHandler.MatchString(s) {
			probs.Add(rel, 0, "event handler attribute found (zero client-side JavaScript)")
		}
		if reJSURL.MatchString(s) {
			probs.Add(rel, 0, "javascript: URL found")
		}
		for _, pre := range rePre.FindAllString(s, -1) {
			if !strings.Contains(pre, `tabindex="0"`) {
				probs.Add(rel, 0, "%s lacks tabindex=\"0\": a scrolling block must be keyboard-focusable", pre)
			}
		}
		for _, must := range []string{`rel="canonical"`, `property="og:title"`, `property="og:type"`, `property="og:url"`, `property="og:description"`, `property="og:image"`, `name="twitter:card"`, `application/ld+json`, `rel="alternate" type="application/rss+xml"`, `<html lang="`} {
			if !strings.Contains(s, must) {
				probs.Add(rel, 0, "missing %s", must)
			}
		}
		if st := reStyleTag.FindStringSubmatch(s); st != nil && hm != nil {
			sum := sha256.Sum256([]byte(st[1]))
			if got := "sha256-" + base64.StdEncoding.EncodeToString(sum[:]); got != string(hm[1]) {
				probs.Add(rel, 0, "inlined CSS hash %s does not match _headers %s", got, hm[1])
			}
		}
		if og := reOGImage.FindStringSubmatch(s); og != nil {
			u := strings.TrimPrefix(og[1], cfg.BaseURL)
			ob, err := os.ReadFile(filepath.Join(dist, filepath.FromSlash(u)))
			if err != nil {
				probs.Add(rel, 0, "og:image %s not in dist", u)
			} else if img, err := png.Decode(bytes.NewReader(ob)); err != nil || img.Bounds().Dx() != 1200 || img.Bounds().Dy() != 630 || len(ob) > 200*1024 {
				probs.Add(rel, 0, "og:image %s must be a 1200×630 PNG under 200 KB", u)
			}
		}
		var gz bytes.Buffer
		w := gzip.NewWriter(&gz)
		_, _ = w.Write(b)
		_ = w.Close()
		if gz.Len() > htmlBudget {
			probs.Add(rel, 0, "page is %d bytes gzipped; budget %d (row 14 — every page, the colophon included)", gz.Len(), htmlBudget)
		}
		return nil
	})
	// feeds and sitemap parse; sitemap covers every page
	for _, f := range []string{"feed.xml", "feed.en.xml", "feed.ro.xml", "sitemap.xml"} {
		b, err := os.ReadFile(filepath.Join(dist, f))
		if err != nil {
			probs.Add(f, 0, "missing")
			continue
		}
		var v struct{}
		if err := xml.Unmarshal(b, &v); err != nil {
			probs.Add(f, 0, "invalid XML: %v", err)
		}
	}
	sm, _ := os.ReadFile(filepath.Join(dist, "sitemap.xml"))
	for _, p := range htmlPaths {
		u := strings.TrimSuffix(p, "index.html")
		if p == "/404.html" {
			continue
		}
		if !strings.Contains(string(sm), "<loc>"+cfg.Abs(u)+"</loc>") {
			probs.Add("sitemap.xml", 0, "does not list %s", u)
		}
	}
	for _, f := range []string{"robots.txt", "llms.txt", ".well-known/security.txt", ".well-known/mta-sts.txt", "favicon.svg", "feed.json"} {
		if _, err := os.Stat(filepath.Join(dist, f)); err != nil {
			probs.Add(f, 0, "missing from dist")
		}
	}
	return probs.Err()
}
