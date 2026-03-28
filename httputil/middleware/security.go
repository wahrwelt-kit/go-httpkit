package middleware

import "net/http"

const defaultCSP = "default-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; script-src 'self'; style-src 'self'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'"

type securityOpts struct {
	csp string
}

// SecurityOption configures SecurityHeaders
type SecurityOption func(*securityOpts)

// WithCSP sets the Content-Security-Policy header. Empty string leaves CSP unset for this middleware
func WithCSP(csp string) SecurityOption {
	return func(o *securityOpts) { o.csp = csp }
}

// SecurityHeaders returns middleware that sets common security headers (X-Content-Type-Options, X-Frame-Options, Referrer-Policy, Permissions-Policy, Content-Security-Policy). If addHSTS is true, adds Strict-Transport-Security (max-age=2 years, includeSubDomains, preload). Set addHSTS for HTTPS-only services. Options (e.g. WithCSP) override defaults
func SecurityHeaders(addHSTS bool, opts ...SecurityOption) func(http.Handler) http.Handler {
	cfg := securityOpts{csp: defaultCSP}
	for _, opt := range opts {
		opt(&cfg)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
			if cfg.csp != "" {
				w.Header().Set("Content-Security-Policy", cfg.csp)
			}
			if addHSTS {
				w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
			}
			next.ServeHTTP(w, r)
		})
	}
}
