package site

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
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

func Serve(o Options, host string, port int) error {
	dist := filepath.Join(o.Root, o.Out)
	var mu sync.Mutex
	var built time.Time
	serve404 := func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(404)
		nf, _ := os.ReadFile(filepath.Join(dist, "404.html"))
		_, _ = w.Write(nf)
	}
	// rebuild assumes the caller already holds mu: every read of dist/ (headers, files, the 404 page) and every
	// read of built must be serialized against write()'s remove-then-rename swap of the output directory, so the
	// whole request is one critical section rather than just the decision to rebuild. The watermark is taken
	// BEFORE Build runs, so an input saved mid-build (whose mtime could otherwise land before the watermark) is
	// still picked up by the next request.
	rebuild := func() error {
		start := time.Now()
		if built.IsZero() || newestInput(o.Root).After(built) {
			if err := Build(o); err != nil {
				return err
			}
			built = start
		}
		return nil
	}
	mu.Lock()
	err := rebuild()
	mu.Unlock()
	if err != nil {
		fmt.Fprintln(os.Stderr, "site serve: build failed:", err)
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
			serve404(w)
			return
		}
		// http.ListenAndServe is given a bare HandlerFunc, not a ServeMux, so nothing upstream cleans the path:
		// do it here before it ever reaches filepath.Join. path.Clean drops a trailing slash, so the
		// is-a-directory redirect below still keys its decision on raw's suffix, not p's.
		p := path.Clean("/" + raw)
		hb, _ := os.ReadFile(filepath.Join(dist, "_headers"))
		rules := parseHeaders(hb)
		fp := filepath.Join(dist, filepath.FromSlash(p))
		if st, err := os.Stat(fp); err == nil && st.IsDir() {
			if !strings.HasSuffix(raw, "/") {
				http.Redirect(w, r, raw+"/", http.StatusMovedPermanently)
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
			serve404(w)
			return
		}
		if strings.HasSuffix(fp, ".html") {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		}
		http.ServeContent(w, r, fp, built, strings.NewReader(string(b)))
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	fmt.Fprintf(os.Stderr, "site serve: http://%s/ (rebuilds when inputs change)\n", addr)
	return http.ListenAndServe(addr, http.HandlerFunc(h))
}
