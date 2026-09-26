package site

import (
	"compress/gzip"
	"context"
	"fmt"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type headerRule struct {
	pattern string
	headers [][2]string
}

func parseHeaders(b []byte) []headerRule {
	var rules []headerRule
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") {
			rules = append(rules, headerRule{pattern: strings.TrimSpace(line)})
			continue
		}
		if len(rules) == 0 {
			continue
		}
		l := strings.TrimSpace(line)
		if name, ok := strings.CutPrefix(l, "! "); ok { // Cloudflare's detach syntax: drop a header an earlier rule set
			rules[len(rules)-1].headers = append(rules[len(rules)-1].headers, [2]string{"!", name})
			continue
		}
		k, v, ok := strings.Cut(l, ": ")
		if ok {
			rules[len(rules)-1].headers = append(rules[len(rules)-1].headers, [2]string{k, v})
		}
	}
	return rules
}

func (r headerRule) match(path string) bool {
	if strings.HasSuffix(r.pattern, "*") {
		return strings.HasPrefix(path, strings.TrimSuffix(r.pattern, "*"))
	}
	return r.pattern == path
}

func newestInput(root string) time.Time {
	var newest time.Time
	for _, d := range []string{"content", "assets", "templates", "i18n", "static", "site.yaml", "data"} {
		_ = filepath.WalkDir(filepath.Join(root, d), func(p string, e os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if info, err := e.Info(); err == nil && info.ModTime().After(newest) {
				newest = info.ModTime()
			}
			return nil
		})
	}
	return newest
}

// hasDotDot reports whether p (the raw, unclean request path) contains a ".." path element, e.g. "/../x" or
// "/a/../../b". Checked against the original path, before path.Clean removes the evidence.
func hasDotDot(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return true
		}
	}
	return false
}

// contentTypes pins the types the audit asserts on (row 9): Go's builtin table lacks woff2 and the OS table
// is not the same on every machine.
var contentTypes = map[string]string{
	".html": "text/html; charset=utf-8", ".css": "text/css; charset=utf-8", ".txt": "text/plain; charset=utf-8",
	".xml": "application/xml; charset=utf-8", ".json": "application/json; charset=utf-8", ".svg": "image/svg+xml",
	".png": "image/png", ".webp": "image/webp", ".jpg": "image/jpeg", ".ico": "image/x-icon", ".woff2": "font/woff2",
}

// compressible lists the types the edge compresses; the audit's view of weight must be the edge's, not raw bytes.
func compressible(ct string) bool {
	for _, p := range []string{"text/", "application/json", "application/xml", "application/rss+xml", "application/feed+json", "image/svg+xml"} {
		if strings.HasPrefix(ct, p) {
			return true
		}
	}
	return false
}

func writeBody(w http.ResponseWriter, r *http.Request, status int, b []byte) {
	ct := w.Header().Get("Content-Type")
	if compressible(ct) && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		w.WriteHeader(status)
		gz := gzip.NewWriter(w)
		_, _ = gz.Write(b)
		_ = gz.Close()
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

func Serve(o Options, host string, port int) error {
	if !o.Static {
		o.Draft = true // the preview is where a piece gets written (spec §8): blanks render with defaults, not as a 500
	}
	dist := filepath.Join(o.Root, o.Out)
	var mu sync.Mutex
	var built, failed time.Time // start of the last good build; start of the last failed one since
	var buildErr error
	hasGood := func() bool { _, err := os.Stat(filepath.Join(dist, "index.html")); return err == nil }
	serve404 := func(w http.ResponseWriter, r *http.Request) {
		nf, _ := os.ReadFile(filepath.Join(dist, "404.html"))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		writeBody(w, r, 404, nf)
	}
	// rebuild assumes the caller already holds mu: every read of dist/ (headers, files, the 404 page) and every
	// read of built must be serialized against write()'s remove-then-rename swap of the output directory, so the
	// whole request is one critical section rather than just the decision to rebuild. The watermarks are taken
	// BEFORE Build runs, so an input saved mid-build is still picked up by the next request; a failed build is not
	// retried until an input changes again.
	rebuild := func() error {
		if o.Static {
			if built.IsZero() {
				st, err := os.Stat(filepath.Join(dist, "index.html"))
				if err != nil {
					return fmt.Errorf("--static: %s has no index.html (run site build first)", dist)
				}
				built = st.ModTime()
			}
			return nil
		}
		start := time.Now()
		if (built.IsZero() && failed.IsZero()) || newestInput(o.Root).After(later(built, failed)) {
			if err := Build(o); err != nil {
				failed, buildErr = start, err
				if hasGood() {
					fmt.Fprintf(os.Stderr, "site serve: build failed; serving the last good build:\n%v\n", err)
				}
			} else {
				built, failed, buildErr = start, time.Time{}, nil
			}
		}
		if buildErr != nil && !hasGood() {
			return buildErr // nothing good to fall back on: the problem list is the page
		}
		return nil
	}
	mu.Lock()
	err := rebuild()
	mu.Unlock()
	if err != nil {
		fmt.Fprintln(os.Stderr, "site serve: build failed:", err)
	}
	// Static mode never rewrites dist/, so _headers can be read and parsed once, up front, instead of on every
	// request; dynamic mode still re-reads it per request because the file changes when sources change.
	var staticRules []headerRule
	if o.Static {
		hb, _ := os.ReadFile(filepath.Join(dist, "_headers"))
		staticRules = parseHeaders(hb)
	}
	h := func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if err := rebuild(); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		raw := r.URL.Path
		if hasDotDot(raw) {
			serve404(w, r)
			return
		}
		// http.ListenAndServe is given a bare HandlerFunc, not a ServeMux, so nothing upstream cleans the path:
		// do it here before it ever reaches filepath.Join. path.Clean drops a trailing slash, so the
		// is-a-directory redirect below still keys its decision on raw's suffix, not p's.
		p := path.Clean("/" + raw)
		fp := filepath.Join(dist, filepath.FromSlash(p))
		if p == "/404.html" {
			serve404(w, r)
			return
		}
		rules := staticRules
		if !o.Static {
			hb, _ := os.ReadFile(filepath.Join(dist, "_headers"))
			rules = parseHeaders(hb)
		}
		if st, err := os.Stat(fp); err == nil && st.IsDir() {
			if !strings.HasSuffix(raw, "/") {
				// Location is built from the cleaned path: a raw "//writing" would otherwise redirect to the
				// scheme-relative "//writing/", i.e. to another host.
				http.Redirect(w, r, p+"/", http.StatusMovedPermanently)
				return
			}
			fp = filepath.Join(fp, "index.html")
		}
		for _, rule := range rules {
			if rule.match(p) {
				for _, kv := range rule.headers {
					if kv[0] == "!" {
						w.Header().Del(kv[1])
						continue
					}
					w.Header().Set(kv[0], kv[1])
				}
			}
		}
		b, err := os.ReadFile(fp)
		if err != nil {
			serve404(w, r)
			return
		}
		ct := contentTypes[filepath.Ext(fp)]
		if ct == "" {
			ct = mime.TypeByExtension(filepath.Ext(fp))
		}
		if ct != "" && w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", ct)
		}
		writeBody(w, r, 200, b)
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	srv := &http.Server{Addr: addr, Handler: http.HandlerFunc(h)}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() { <-ctx.Done(); _ = srv.Shutdown(context.Background()) }()
	mode := "rebuilds when inputs change"
	if o.Static {
		mode = "static: serves " + dist + " as built"
	}
	fmt.Fprintf(os.Stderr, "site serve: http://%s/ (%s)\n", addr, mode)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func later(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
