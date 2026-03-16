package httputil

import (
	"math"
	"net/http"
	"strconv"
)

// MaxPage is the maximum allowed page number for ClampPage.
const MaxPage = 10000

// ClampPage returns the page number clamped to [1, MaxPage]; uses 1 if p is nil or *p < 1.
func ClampPage(p *int) int {
	if p == nil || *p < 1 {
		return 1
	}
	if *p > MaxPage {
		return MaxPage
	}
	return *p
}

// ClampPerPage returns perPage clamped to [defaultVal, maxVal], or defaultVal if nil/<=0.
// If defaultVal > maxVal, the effective default is maxVal so the result is never above maxVal.
func ClampPerPage(p *int, defaultVal, maxVal int) int {
	if defaultVal > maxVal {
		defaultVal = maxVal
	}
	if p == nil || *p <= 0 {
		return defaultVal
	}
	if *p > maxVal {
		return maxVal
	}
	return *p
}

// ClampLimit returns limit clamped to [defaultVal, maxVal], or defaultVal if nil/<=0.
func ClampLimit(p *int, defaultVal, maxVal int) int {
	return ClampPerPage(p, defaultVal, maxVal)
}

// ParseIntQuery parses the first query parameter key as positive int; returns nil if missing or invalid.
func ParseIntQuery(r *http.Request, key string) *int {
	q := r.URL.Query().Get(key)
	if q == "" {
		return nil
	}
	n, err := strconv.Atoi(q)
	if err != nil || n < 1 {
		return nil
	}
	return &n
}

// Ptr returns a pointer to v. Useful for optional query params.
func Ptr[T any](v T) *T {
	return &v
}

// TotalPages calculates the total number of pages for a given total and perPage.
func TotalPages(total int64, perPage int) int {
	if perPage <= 0 {
		return 0
	}
	n64 := (total + int64(perPage) - 1) / int64(perPage)
	if n64 > math.MaxInt {
		return math.MaxInt
	}
	return int(n64)
}

// PaginationMeta holds pagination metadata for a response.
type PaginationMeta struct {
	Page       int // Current page (1-based).
	PerPage    int // Items per page.
	Total      int // Total item count (capped to math.MaxInt).
	TotalPages int // Total number of pages.
}

// clampTotal clamps a total to the maximum integer value.
func clampTotal(total int64) int {
	if total <= 0 {
		return 0
	}
	if total > math.MaxInt {
		return math.MaxInt
	}
	return int(total)
}

// NewPaginationMeta builds PaginationMeta with Total and TotalPages derived from total and perPage.
func NewPaginationMeta(page, perPage int, total int64) PaginationMeta {
	return PaginationMeta{
		Page:       page,
		PerPage:    perPage,
		Total:      clampTotal(total),
		TotalPages: TotalPages(total, perPage),
	}
}
