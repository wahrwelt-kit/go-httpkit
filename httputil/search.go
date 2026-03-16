package httputil

import (
	"math"
	"strings"
	"unicode"
	"unicode/utf8"
)

// DefaultSearchMaxLen is the default maximum rune length for search input in ValidateSearchQ and EscapeILIKE.
const DefaultSearchMaxLen = 100

// ValidateSearchQ returns true if q is valid for search (within DefaultSearchMaxLen runes and no control characters).
func ValidateSearchQ(q string) bool {
	if utf8.RuneCountInString(q) > DefaultSearchMaxLen {
		return false
	}
	for _, r := range q {
		if r == 0 || r == '\n' || r == '\r' || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// EscapeILIKE escapes %, _, and \ for safe use in PostgreSQL ILIKE. Output is truncated to maxLen runes (or DefaultSearchMaxLen if maxLen <= 0).
func EscapeILIKE(s string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = DefaultSearchMaxLen
	}
	grow := maxLen * 4
	if grow <= 0 || grow < maxLen {
		grow = maxLen
	}
	if grow > math.MaxInt/2 {
		grow = math.MaxInt / 2
	}
	var b strings.Builder
	b.Grow(grow)
	n := 0
	for _, r := range s {
		if n >= maxLen {
			break
		}
		switch r {
		case 0:
			continue
		case '\\':
			b.WriteString(`\\`)
		case '%':
			b.WriteString(`\%`)
		case '_':
			b.WriteString(`\_`)
		default:
			if unicode.IsControl(r) {
				continue
			}
			b.WriteRune(r)
		}
		n++
	}
	return b.String()
}

// SanitizeSearchQ is EscapeILIKE with default max length (DefaultSearchMaxLen when maxLen <= 0).
func SanitizeSearchQ(q string, maxLen int) string {
	return EscapeILIKE(q, maxLen)
}
