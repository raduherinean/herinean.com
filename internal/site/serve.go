package site

import (
	"fmt"
	"net/http"
	"os"
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

func Serve(o Options, host string, port int) error {
	dist := filepath.Join(o.Root, o.Out)
	var mu sync.Mutex
	var built time.Time
	rebuild := func() error {
		mu.Lock()
		defer mu.Unlock()
		if n := newestInput(o.Root); !built.IsZero() && !n.After(built) {
			return nil
		}
		if err := Build(o); err != nil {
			return err
		}
		built = time.Now()
		return nil
	}
	if err := rebuild(); err != nil {
		fmt.Fprintln(os.Stderr, "site serve: build failed:", err)
	}
	h := func(w http.ResponseWriter, r *http.Request) {
		if err := rebuild(); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		hb, _ := os.ReadFile(filepath.Join(dist, "_headers"))
		rules := parseHeaders(hb)
		p := r.URL.Path
		fp := filepath.Join(dist, filepath.FromSlash(p))
		if st, err := os.Stat(fp); err == nil && st.IsDir() {
			if !strings.HasSuffix(p, "/") {
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
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(404)
			nf, _ := os.ReadFile(filepath.Join(dist, "404.html"))
			_, _ = w.Write(nf)
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
