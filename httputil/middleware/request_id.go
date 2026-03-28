package middleware

import (
	"context"
	"net/http"
	"unicode"

	"github.com/google/uuid"
)

type requestIDKey struct{}

func validRequestID(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII || unicode.IsControl(r) || r == '\r' || r == '\n' {
			return false
		}
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '_' && r != '.' {
			return false
		}
	}
	return len(s) > 0 && len(s) <= 128
}

// RequestID returns middleware that sets X-Request-ID from the request header or generates a new UUID, and stores it in the context. Invalid header values are replaced with a new UUID to prevent response splitting
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if id == "" || !validRequestID(id) {
				id = uuid.New().String()
			}
			w.Header().Set("X-Request-ID", id)
			ctx := context.WithValue(r.Context(), requestIDKey{}, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetRequestID returns the request ID from the context (set by RequestID middleware), or "" if not set
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey{}).(string); ok {
		return id
	}
	return ""
}
