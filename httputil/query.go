package httputil

import (
	"net/http"
	"strings"
	"time"
)

// ParseBoolQuery parses the query parameter key as a boolean. Accepts "1", "true", "yes" for true and "0", "false", "no" for false.
// Returns (value, true) when valid, (false, false) when missing or invalid.
func ParseBoolQuery(r *http.Request, key string) (bool, bool) {
	q := strings.TrimSpace(strings.ToLower(r.URL.Query().Get(key)))
	if q == "" {
		return false, false
	}
	switch q {
	case "1", "true", "yes":
		return true, true
	case "0", "false", "no":
		return false, true
	default:
		return false, false
	}
}

// ParseEnumQuery parses the query parameter key and returns it as T if it is in allowed; otherwise returns ("", false).
func ParseEnumQuery[T ~string](r *http.Request, key string, allowed []T) (T, bool) {
	q := strings.TrimSpace(r.URL.Query().Get(key))
	if q == "" {
		return "", false
	}
	for _, a := range allowed {
		if string(a) == q {
			return T(q), true
		}
	}
	return "", false
}

// ParseSortQuery parses the "sort" query parameter. Supports "field", "-field", or "field:asc"/"field:desc".
// Returns (field, dir, true) when field is in allowedFields and dir is "asc" or "desc"; otherwise ("", "", false).
func ParseSortQuery(r *http.Request, allowedFields []string) (field, dir string, ok bool) {
	q := strings.TrimSpace(r.URL.Query().Get("sort"))
	if q == "" {
		return "", "", false
	}
	allowedSet := make(map[string]bool, len(allowedFields))
	for _, f := range allowedFields {
		allowedSet[f] = true
	}
	field = q
	dir = "asc"
	if strings.HasPrefix(q, "-") {
		field = strings.TrimPrefix(q, "-")
		dir = "desc"
	}
	if idx := strings.Index(field, ":"); idx >= 0 {
		field, dir = field[:idx], field[idx+1:]
		field = strings.TrimSpace(field)
		dir = strings.TrimSpace(strings.ToLower(dir))
		if dir != "asc" && dir != "desc" {
			return "", "", false
		}
	}
	if !allowedSet[field] {
		return "", "", false
	}
	return field, dir, true
}

// ParseTimeQuery parses the query parameter key with the given time layout (e.g. time.RFC3339). Returns (zero time.Time, false) on missing or invalid value.
func ParseTimeQuery(r *http.Request, key, layout string) (time.Time, bool) {
	q := strings.TrimSpace(r.URL.Query().Get(key))
	if q == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(layout, q)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
