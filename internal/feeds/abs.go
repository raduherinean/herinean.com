package feeds

import (
	"regexp"
	"strings"
)

var srcsetAttr = regexp.MustCompile(`srcset="([^"]*)"`)

// rootAttr matches href/src values rooted at the site ("/writing/x/") but not
// protocol-relative URLs ("//cdn.example/x"), which must be left alone.
var rootAttr = regexp.MustCompile(`\b(href|src)="/([^/"][^"]*)?"`)

// Absolutize rewrites site-relative href/src/srcset values to absolute URLs and roots #fragments at the page, for feed readers and aggregators.
func Absolutize(html, base, pageURL string) string {
	html = rootAttr.ReplaceAllString(html, `$1="`+base+`/$2"`)
	html = strings.ReplaceAll(html, `href="#`, `href="`+pageURL+`#`)
	return srcsetAttr.ReplaceAllStringFunc(html, func(m string) string {
		parts := strings.Split(m[len(`srcset="`):len(m)-1], ",")
		for i, c := range parts {
			c = strings.TrimSpace(c)
			if strings.HasPrefix(c, "/") && !strings.HasPrefix(c, "//") {
				c = base + c
			}
			parts[i] = c
		}
		return `srcset="` + strings.Join(parts, ", ") + `"`
	})
}
