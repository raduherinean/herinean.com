package content

import (
	"regexp"
	"strings"
	"unicode"
)

var translit = map[rune]string{
	'ă': "a", 'â': "a", 'î': "i", 'ș': "s", 'ț': "t", 'ş': "s", 'ţ': "t",
	'ä': "a", 'ö': "o", 'ü': "u", 'ß': "ss", 'é': "e", 'è': "e", 'ê': "e", 'ë': "e",
	'á': "a", 'à': "a", 'ó': "o", 'ò': "o", 'ú': "u", 'ù': "u", 'í': "i", 'ì': "i", 'ç': "c", 'ñ': "n",
}

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Slugify makes an ASCII slug from a title. Romanian diacritics transliterate; everything else non-alphanumeric becomes a hyphen.
func Slugify(s string) string {
	var b strings.Builder
	lastDash := true
	for _, r := range strings.ToLower(s) {
		if t, ok := translit[r]; ok {
			b.WriteString(t)
			lastDash = false
			continue
		}
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue // other scripts: dropped rather than guessed
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func ValidSlug(s string) bool { return slugRe.MatchString(s) }
