package site

import (
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// startServe starts Serve on a free 127.0.0.1 port in a goroutine and waits for it to answer, returning "host:port".
func startServe(t *testing.T, o Options) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	go func() {
		_ = Serve(o, "127.0.0.1", port)
	}()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if resp, err := http.Get("http://" + addr + "/"); err == nil {
			resp.Body.Close()
			return addr
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("server on %s did not come up in time", addr)
	return ""
}

// A raw client can send a request-target with a ".." element; ListenAndServe here is given a bare HandlerFunc, not
// a ServeMux, so nothing upstream cleans it. The handler must reject it itself, before it ever reaches the
// filesystem, rather than resolving it under dist/ and serving whatever it points at (here: the fixture's
// site.yaml, one directory above dist/).
func TestServeRejectsDotDotPath(t *testing.T) {
	root := fixtureRoot(t)
	addr := startServe(t, Options{Root: root, Out: "dist"})

	// Built directly rather than via http.NewRequest/url.Parse, so the literal ".." element reaches the server
	// regardless of any client-side URL normalization.
	req := &http.Request{
		Method: "GET",
		URL:    &url.URL{Scheme: "http", Host: addr, Path: "/../site.yaml"},
		Host:   addr,
		Header: make(http.Header),
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404 (body: %s)", resp.StatusCode, body)
	}
	if strings.Contains(string(body), "base_url") {
		t.Fatalf("response leaked site.yaml contents: %s", body)
	}
	if !strings.Contains(string(body), "Page not found") {
		t.Fatalf("expected the 404 page body, got: %s", body)
	}
}

// Two requests in flight at once must not deadlock around the rebuild lock, and both must see the built site.
func TestServeConcurrentRequestsBothSucceed(t *testing.T) {
	root := fixtureRoot(t)
	addr := startServe(t, Options{Root: root, Out: "dist"})

	var wg sync.WaitGroup
	codes := make([]int, 2)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp, err := http.Get("http://" + addr + "/writing/paired/")
			if err != nil {
				errs[i] = err
				return
			}
			defer resp.Body.Close()
			codes[i] = resp.StatusCode
		}(i)
	}
	wg.Wait()
	for i := range codes {
		if errs[i] != nil {
			t.Fatalf("request %d: %v", i, errs[i])
		}
		if codes[i] != 200 {
			t.Errorf("request %d: status = %d, want 200", i, codes[i])
		}
	}
}

// Spec §8: `site new` (blank title/date/pillar/summary) → "write on site serve". The preview therefore renders a
// draft with visible defaults instead of answering 500 on every page, while build/check keep refusing it.
func TestServeRendersDraftPiece(t *testing.T) {
	root := fixtureRoot(t)
	draft := "---\ntitle: \"\"\ndate:\nkey: draft-piece\npillar:\nsummary: \"\"\n---\n\n## Situation\n\nStill writing.\n"
	if err := os.WriteFile(filepath.Join(root, "content", "en", "draft-piece.md"), []byte(draft), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Build(Options{Root: root, Out: "dist"}); err == nil || !strings.Contains(err.Error(), "date is empty") {
		t.Fatalf("Build: want the draft refused, got %v", err)
	}
	addr := startServe(t, Options{Root: root, Out: "dist"})
	resp, err := http.Get("http://" + addr + "/writing/draft-piece/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("draft page: status %d, want 200: %s", resp.StatusCode, body)
	}
	for _, want := range []string{"(draft) draft-piece", "Draft", "Still writing."} {
		if !strings.Contains(string(body), want) {
			t.Errorf("draft page lacks %q", want)
		}
	}
}

// The directory redirect's Location must come from the cleaned path: a raw "//writing" is still a directory after
// path.Clean, but raw+"/" would be the scheme-relative "//writing/", an open redirect to host "writing".
func TestServeDirectoryRedirectUsesCleanPath(t *testing.T) {
	root := fixtureRoot(t)
	addr := startServe(t, Options{Root: root, Out: "dist"})
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for raw, want := range map[string]string{"//writing": "/writing/", "/writing": "/writing/", "/ro//articole": "/ro/articole/"} {
		req := &http.Request{Method: "GET", URL: &url.URL{Scheme: "http", Host: addr, Path: raw}, Host: addr, Header: make(http.Header)}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusMovedPermanently {
			t.Errorf("%s: status = %d, want 301", raw, resp.StatusCode)
		}
		if got := resp.Header.Get("Location"); got != want {
			t.Errorf("%s: Location = %q, want %q", raw, got, want)
		}
	}
}

// --static serves dist/ as built: a source edit after the build must not change a response (CI audits the
// artifact that deploys), the 404 page carries a 404 status, and text is gzipped when the client accepts it.
func TestServeStaticServesBuiltDistOnly(t *testing.T) {
	root := fixtureRoot(t)
	o := Options{Root: root, Out: "dist", Static: true}
	if err := Build(Options{Root: root, Out: "dist"}); err != nil {
		t.Fatal(err)
	}
	addr := startServe(t, o)
	get := func(path string, gzip bool) (*http.Response, string) {
		req, _ := http.NewRequest("GET", "http://"+addr+path, nil)
		if gzip {
			req.Header.Set("Accept-Encoding", "gzip")
		}
		tr := &http.Transport{DisableCompression: true}
		resp, err := (&http.Client{Transport: tr}).Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return resp, string(b)
	}
	resp, body := get("/", false)
	if resp.StatusCode != 200 || !strings.Contains(body, "<html") {
		t.Fatalf("/: %d %q", resp.StatusCode, body[:min(len(body), 80)])
	}
	if resp.Header.Get("Content-Security-Policy") == "" {
		t.Error("/ lacks the _headers CSP")
	}
	// The preloaded font URL must be served with the pinned font/woff2 type, not whatever
	// mime.TypeByExtension (or an absent OS mime.types) happens to return on this machine: the
	// bench's headers row expects exactly that type, and a preload with the wrong type is wasted.
	if m := regexp.MustCompile(`/fonts/[^"]+\.woff2`).FindString(body); m == "" {
		t.Error("/ has no preloaded font URL to check")
	} else if resp, _ := get(m, false); resp.Header.Get("Content-Type") != "font/woff2" {
		t.Errorf("%s: Content-Type = %q, want font/woff2", m, resp.Header.Get("Content-Type"))
	}
	// edit a source: the static server must not notice
	home := filepath.Join(root, "content", "en", "_home.md")
	orig, _ := os.ReadFile(home)
	if err := os.WriteFile(home, append(orig, []byte("\n\nEDITED AFTER BUILD\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, body2 := get("/", false); body2 != body {
		t.Error("static serve rebuilt after a source edit")
	}
	resp, _ = get("/nope/", false)
	if resp.StatusCode != 404 {
		t.Errorf("/nope/: status %d, want 404", resp.StatusCode)
	}
	resp, _ = get("/404.html", false)
	if resp.StatusCode != 404 {
		t.Errorf("/404.html: status %d, want 404 (the page is the 404 page)", resp.StatusCode)
	}
	resp, gz := get("/", true)
	if resp.Header.Get("Content-Encoding") != "gzip" || resp.Header.Get("Vary") != "Accept-Encoding" {
		t.Errorf("gzip not negotiated: %v", resp.Header)
	}
	if len(gz) >= len(body) {
		t.Errorf("gzipped body (%d) not smaller than plain (%d)", len(gz), len(body))
	}
	resp, _ = get("/favicon.svg", true)
	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Error("svg not gzipped")
	}
	if resp, _ := get("/apple-touch-icon.png", true); resp.Header.Get("Content-Encoding") != "" {
		t.Error("png must not be gzipped")
	}
}

func fetch(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

const brokenRO = "---\ntitle: \"Stricat\"\ndate: 2026-09-01\nkey: stricat\npillar: analysis\nsummary: \"Rezumat.\"\n---\n\nReţea cu sedilă.\n"

// A failing rule mid-writing keeps the last good render on screen (the problem goes to stderr) instead of a 500 on
// every page; fixing it brings the new page up on the next request.
func TestServeKeepsTheLastGoodBuild(t *testing.T) {
	root := fixtureRoot(t)
	addr := startServe(t, Options{Root: root, Out: "dist"})
	if code, _ := fetch(t, "http://"+addr+"/"); code != 200 {
		t.Fatalf("before: status %d", code)
	}
	f := filepath.Join(root, "content", "ro", "stricat.md")
	if err := os.WriteFile(f, []byte(brokenRO), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, body := fetch(t, "http://"+addr+"/"); code != 200 {
		t.Fatalf("with a failing rule: status %d, want the last good build: %s", code, body)
	}
	if err := os.WriteFile(f, []byte(strings.Replace(brokenRO, "ţ", "ț", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, body := fetch(t, "http://"+addr+"/ro/articole/stricat/"); code != 200 || !strings.Contains(body, "Rețea") {
		t.Fatalf("after the fix: status %d, want the new page: %s", code, body)
	}
}

// With no good build yet there is nothing to fall back on: the problem list is the page.
func TestServeWithoutAGoodBuildShowsTheProblems(t *testing.T) {
	root := fixtureRoot(t)
	if err := os.WriteFile(filepath.Join(root, "content", "ro", "stricat.md"), []byte(brokenRO), 0o644); err != nil {
		t.Fatal(err)
	}
	addr := startServe(t, Options{Root: root, Out: "dist"})
	if code, body := fetch(t, "http://"+addr+"/"); code != 500 || !strings.Contains(body, "comma-below") {
		t.Fatalf("status %d, want 500 with the cedilla problem: %s", code, body)
	}
}
