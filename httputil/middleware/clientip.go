package middleware

import (
	"context"
	"net/http"

	"github.com/wahrwelt-kit/go-httpkit/httputil"
)

type clientIPContextKey struct{}

// ClientIP returns middleware that resolves the client IP (using trustedProxyCIDRs for X-Real-IP and X-Forwarded-For) and stores it in the request context. CIDRs are parsed once at build time. Returns an error if all entries are invalid; empty or nil slice is valid (no proxy trust).
func ClientIP(trustedProxyCIDRs []string) (func(http.Handler) http.Handler, error) {
	nets, err := httputil.ParseTrustedProxyCIDRs(trustedProxyCIDRs)
	if err != nil && nets == nil {
		return nil, err
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := httputil.GetClientIPWithNets(r, nets)
			ctx := context.WithValue(r.Context(), clientIPContextKey{}, ip)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}, nil
}

// GetClientIPFromContext returns the client IP stored by the ClientIP middleware, or "" if not set.
func GetClientIPFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if ip, ok := ctx.Value(clientIPContextKey{}).(string); ok {
		return ip
	}
	return ""
}
