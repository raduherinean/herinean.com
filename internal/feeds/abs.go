package feeds

import (
	"regexp"
	"strings"
)

var srcsetAttr = regexp.MustCompile(`srcset="([^"]*)"`)

// Absolutize rewrites site-relative href/src/srcset values to absolute URLs and roots #fragments at the page, for feed readers and aggregators.
func Absolutize(html, base, pageURL string) string {
	html = strings.NewReplacer(`href="/`, `href="`+base+`/`, `src="/`, `src="`+base+`/`, `href="#`, `href="`+pageURL+`#`).Replace(html)
	return srcsetAttr.ReplaceAllStringFunc(html, func(m string) string {
		parts := strings.Split(m[len(`srcset="`):len(m)-1], ",")
		for i, c := range parts {
			c = strings.TrimSpace(c)
			if strings.HasPrefix(c, "/") {
				c = base + c
			}
			parts[i] = c
		}
		return `srcset="` + strings.Join(parts, ", ") + `"`
	})
}
