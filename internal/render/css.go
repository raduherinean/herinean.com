package render

import (
	"crypto/sha256"
	"encoding/base64"
	"regexp"
	"strings"
)

var (
	cssComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	cssSpace   = regexp.MustCompile(`\s*([{}:;,>])\s*`)
	cssNL      = regexp.MustCompile(`\s*\n\s*`)
)

func minifyCSS(s string) string {
	s = cssComment.ReplaceAllString(s, "")
	s = cssNL.ReplaceAllString(s, "")
	s = cssSpace.ReplaceAllString(s, "$1")
	s = strings.ReplaceAll(s, ";}", "}")
	return strings.TrimSpace(s)
}

// CSPHash returns the style-src hash for the exact bytes inlined in <style>.
func CSPHash(css string) string {
	sum := sha256.Sum256([]byte(css))
	return "sha256-" + base64.StdEncoding.EncodeToString(sum[:])
}
