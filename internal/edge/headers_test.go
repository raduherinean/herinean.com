package edge

import (
	"strings"
	"testing"
)

func TestHeaders(t *testing.T) {
	h := string(Headers("sha256-abc"))
	for _, want := range []string{
		"/*\n  Content-Security-Policy: default-src 'none'; style-src 'sha256-abc'; img-src 'self'; font-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'",
		"Strict-Transport-Security: max-age=63072000; includeSubDomains; preload",
		"Cross-Origin-Resource-Policy: same-origin",
		"/img/*\n  ! Cross-Origin-Resource-Policy\n  ! Cache-Control\n  Cross-Origin-Resource-Policy: cross-origin\n  Cache-Control: public, max-age=31536000, immutable",
		"/og/*\n  ! Cross-Origin-Resource-Policy\n  ! Cache-Control\n  Cross-Origin-Resource-Policy: cross-origin",
		"/feed.xml\n  ! Cache-Control\n  Content-Type: application/rss+xml; charset=utf-8\n  Cache-Control: public, max-age=300",
		"/feed.json\n  Content-Type: application/feed+json; charset=utf-8",
		"/llms.txt\n  Content-Type: text/plain; charset=utf-8",
		"/robots.txt\n  Content-Type: text/plain; charset=utf-8",
		"/.well-known/security.txt\n  Content-Type: text/plain; charset=utf-8",
	} {
		if !strings.Contains(h, want) {
			t.Errorf("missing:\n%s\nin:\n%s", want, h)
		}
	}
	if strings.Contains(strings.ToLower(h), "x-robots-tag") {
		t.Error("_headers must never carry X-Robots-Tag: production is indexable; the Worker marks previews")
	}
	if strings.Contains(h, "interest-cohort") {
		t.Error("interest-cohort is a dead feature name; Chrome logs a console message for it")
	}
}
