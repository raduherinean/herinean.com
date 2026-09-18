// Package edge generates the Cloudflare _headers file. The CSS hash comes from the exact bytes inlined by render.
// Cloudflare applies every matching rule; a header set by /* is not replaced by a more specific rule unless that rule
// first detaches it with "! Name". Every override below does that, so /img/* really is immutable and cross-origin.
package edge

import "fmt"

func Headers(cssHash string) []byte {
	return []byte(fmt.Sprintf(`/*
  Content-Security-Policy: default-src 'none'; style-src '%s'; img-src 'self'; font-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'
  Strict-Transport-Security: max-age=63072000; includeSubDomains; preload
  X-Content-Type-Options: nosniff
  Referrer-Policy: strict-origin-when-cross-origin
  Permissions-Policy: accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()
  Cross-Origin-Opener-Policy: same-origin
  Cross-Origin-Resource-Policy: same-origin
  X-Frame-Options: DENY
  Cache-Control: public, max-age=0, must-revalidate

/img/*
  ! Cross-Origin-Resource-Policy
  ! Cache-Control
  Cross-Origin-Resource-Policy: cross-origin
  Cache-Control: public, max-age=31536000, immutable

/og/*
  ! Cross-Origin-Resource-Policy
  ! Cache-Control
  Cross-Origin-Resource-Policy: cross-origin
  Cache-Control: public, max-age=31536000, immutable

/fonts/*
  ! Cache-Control
  Cache-Control: public, max-age=31536000, immutable

/feed.xml
  ! Cache-Control
  Content-Type: application/rss+xml; charset=utf-8
  Cache-Control: public, max-age=300

/feed.en.xml
  ! Cache-Control
  Content-Type: application/rss+xml; charset=utf-8
  Cache-Control: public, max-age=300

/feed.ro.xml
  ! Cache-Control
  Content-Type: application/rss+xml; charset=utf-8
  Cache-Control: public, max-age=300

/feed.json
  Content-Type: application/feed+json; charset=utf-8
  ! Cache-Control
  Cache-Control: public, max-age=300

/sitemap.xml
  Content-Type: application/xml; charset=utf-8

/llms.txt
  Content-Type: text/plain; charset=utf-8

/.well-known/security.txt
  Content-Type: text/plain; charset=utf-8

/.well-known/mta-sts.txt
  Content-Type: text/plain; charset=utf-8
`, cssHash))
}
