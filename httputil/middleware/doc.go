// Package middleware provides HTTP middleware for use with go-httpkit and chi
//
// # Client IP
//
// ClientIP resolves the client IP (using trusted proxy CIDRs for X-Real-IP and X-Forwarded-For) and stores it in the request context. CIDRs are parsed once at build time. Use GetClientIPFromContext in handlers to read it. Returns an error if all CIDR entries are invalid; empty or nil slice means no proxy trust
//
// # Logging
//
// Logger logs each request (method, path, redacted query, IP, user-agent, request_id) and after the handler adds status, latency_ms, and bytes. Log level: Info for 2xx, Warn for 4xx, Error for 5xx. Sensitive query params (token, password, secret, api_key, client_secret, refresh_token, access_token, authorization, state, code) are always redacted. Use WithRedactedParams to add extra names. Use WithSkipPaths to suppress logging for specific paths (e.g. /health, /metrics) - the handler still runs, only the log entry is omitted. If log is nil, the middleware is a no-op. CIDRs are parsed once at construction
//
// # Metrics
//
// Metrics records http_requests_total and http_request_duration_seconds (Prometheus). Pass a PathFromRequest that returns route patterns (e.g. httputil.ChiPathFromRequest), not raw paths, to avoid unbounded label cardinality. reg can be nil for DefaultRegisterer. Optional logger for registration errors
//
// # Recoverer
//
// Recoverer recovers panics, logs the panic and stack trace (if log is non-nil), and responds with 500 JSON. Place it at the top of the middleware chain
//
// # Request ID
//
// RequestID sets or propagates X-Request-ID (from header or new UUID), validates format to prevent response splitting, and stores it in context. Use GetRequestID to read it
//
// # Security headers
//
// SecurityHeaders sets X-Content-Type-Options, X-Frame-Options, Referrer-Policy, Permissions-Policy, Content-Security-Policy, and optionally Strict-Transport-Security (HSTS). Set addHSTS true for HTTPS. style-src is 'self'
//
// # Timeout
//
// Timeout runs the handler with a context deadline. On timeout it responds with 503 JSON. The full response body is buffered in memory-do not use for streaming or large responses. TimeoutWithLimit adds a max response body size; when the handler's Write call would exceed it, Write returns ErrResponseBodyTooLarge to the handler - the middleware itself does not surface this to the chain caller. The handler goroutine may continue after the response; handlers should check context cancellation
package middleware
