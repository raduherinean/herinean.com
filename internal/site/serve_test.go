package site

import (
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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
