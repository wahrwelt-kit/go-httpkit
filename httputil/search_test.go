package httputil

import (
	"strings"
	"testing"
)

func TestEscapeILIKE(t *testing.T) {
	t.Parallel()
	const escapedPercent = `a\%b`
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{"backslash", `a\b`, 10, `a\\b`},
		{"percent", "a%b", 10, escapedPercent},
		{"underscore", "a_b", 10, `a\_b`},
		{"mixed", `%\_\`, 10, "\\%\\\\\\_\\\\"},
		{"truncate", "abcdefghij", 5, "abcde"},
		{"zero maxLen uses default", "a%b", 0, escapedPercent},
	}
	for _, tt := range tests {
		got := EscapeILIKE(tt.s, tt.maxLen)
		if got != tt.want {
			t.Errorf("EscapeILIKE(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
		}
	}
}

func TestValidateSearchQ(t *testing.T) {
	t.Parallel()
	tests := []struct {
		q    string
		want bool
	}{
		{"", true},
		{"a", true},
		{"normal query", true},
		{"a\nb", false},
		{"a\rb", false},
		{"a\x00b", false},
	}
	for _, tt := range tests {
		got := ValidateSearchQ(tt.q)
		if got != tt.want {
			t.Errorf("ValidateSearchQ(%q) = %v, want %v", tt.q, got, tt.want)
		}
	}
}

func TestSanitizeSearchQ(t *testing.T) {
	t.Parallel()
	const escapedPercent = `a\%b`
	got := SanitizeSearchQ("a%b", 100)
	if got != escapedPercent {
		t.Errorf("SanitizeSearchQ(%q, 100) = %q, want %q", "a%b", got, escapedPercent)
	}
}

func BenchmarkEscapeILIKE_Short(b *testing.B) {
	s := "hello world"
	for b.Loop() {
		_ = EscapeILIKE(s, 100)
	}
}

func BenchmarkEscapeILIKE_WithSpecialChars(b *testing.B) {
	s := `%foo\_bar\baz%`
	for b.Loop() {
		_ = EscapeILIKE(s, 100)
	}
}

func BenchmarkEscapeILIKE_Long(b *testing.B) {
	s := strings.Repeat("x", 200)
	for b.Loop() {
		_ = EscapeILIKE(s, 100)
	}
}
