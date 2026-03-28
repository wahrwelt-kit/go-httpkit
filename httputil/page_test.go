package httputil

import (
	"net/http"
	"testing"
)

func TestClampPage(t *testing.T) {
	t.Parallel()
	one := 1
	mid := 5
	maxPg := MaxPage
	over := MaxPage + 1
	zero := 0
	neg := -1
	tests := []struct {
		p    *int
		want int
	}{
		{nil, 1},
		{&zero, 1},
		{&neg, 1},
		{&one, 1},
		{&mid, 5},
		{&maxPg, MaxPage},
		{&over, MaxPage},
	}
	for _, tt := range tests {
		got := ClampPage(tt.p)
		if got != tt.want {
			t.Errorf("ClampPage(%v) = %d, want %d", tt.p, got, tt.want)
		}
	}
}

func TestClampPerPage(t *testing.T) {
	t.Parallel()
	two := 2
	five := 5
	ten := 10
	zero := 0
	neg := -1
	tests := []struct {
		p          *int
		defaultVal int
		maxVal     int
		want       int
	}{
		{nil, 10, 100, 10},
		{&zero, 10, 100, 10},
		{&neg, 10, 100, 10},
		{&two, 10, 100, 2},
		{&five, 10, 100, 5},
		{&ten, 10, 100, 10},
		{&ten, 5, 8, 8},
		{nil, 200, 100, 100},
	}
	for _, tt := range tests {
		got := ClampPerPage(tt.p, tt.defaultVal, tt.maxVal)
		if got != tt.want {
			t.Errorf("ClampPerPage(%v, %d, %d) = %d, want %d", tt.p, tt.defaultVal, tt.maxVal, got, tt.want)
		}
	}
}

func TestClampLimit(t *testing.T) {
	t.Parallel()
	p := Ptr(5) //nolint:modernize
	got := ClampLimit(p, 10, 100)
	if got != 5 {
		t.Errorf("ClampLimit(5, 10, 100) = %d, want 5", got)
	}
	got2 := ClampLimit(nil, 10, 100)
	if got2 != 10 {
		t.Errorf("ClampLimit(nil, 10, 100) = %d, want 10", got2)
	}
}

func TestParseIntQuery(t *testing.T) {
	t.Parallel()
	tests := []struct {
		rawURL  string
		key     string
		wantNil bool
		wantVal int
	}{
		{"http://localhost/?page=1", "page", false, 1},
		{"http://localhost/?page=42", "page", false, 42},
		{"http://localhost/?x=1", "page", true, 0},
		{"http://localhost/", "page", true, 0},
		{"http://localhost/?page=0", "page", true, 0},
		{"http://localhost/?page=-1", "page", true, 0},
		{"http://localhost/?page=abc", "page", true, 0},
	}
	for _, tt := range tests {
		r, _ := http.NewRequest(http.MethodGet, tt.rawURL, nil)
		got := ParseIntQuery(r, tt.key)
		if tt.wantNil {
			if got != nil {
				t.Errorf("ParseIntQuery(%q, %q) = %v, want nil", tt.rawURL, tt.key, got)
			}
			continue
		}
		if got == nil || *got != tt.wantVal {
			t.Errorf("ParseIntQuery(%q, %q) = %v, want %d", tt.rawURL, tt.key, got, tt.wantVal)
		}
	}
}

func TestTotalPages(t *testing.T) {
	t.Parallel()
	tests := []struct {
		total   int64
		perPage int
		want    int
	}{
		{0, 10, 0},
		{1, 10, 1},
		{10, 10, 1},
		{11, 10, 2},
		{25, 10, 3},
		{0, 0, 0},
		{5, -1, 0},
	}
	for _, tt := range tests {
		got := TotalPages(tt.total, tt.perPage)
		if got != tt.want {
			t.Errorf("TotalPages(%d, %d) = %d, want %d", tt.total, tt.perPage, got, tt.want)
		}
	}
}
