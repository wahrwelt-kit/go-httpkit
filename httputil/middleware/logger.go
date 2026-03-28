package middleware

import (
	"maps"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wahrwelt-kit/go-httpkit/httputil"
	logger "github.com/wahrwelt-kit/go-logkit"
)

const (
	statusServerError = http.StatusInternalServerError
	statusClientError = http.StatusBadRequest
)

// defaultSensitiveQueryParams is the built-in set of query parameter names that are always redacted
var defaultSensitiveQueryParams = map[string]struct{}{
	"token":         {},
	"state":         {},
	"code":          {},
	"password":      {},
	"secret":        {},
	"api_key":       {},
	"apikey":        {},
	"client_secret": {},
	"refresh_token": {},
	"access_token":  {},
	"authorization": {},
}

// LoggerOption configures the Logger middleware
type LoggerOption func(*loggerConfig)

type loggerConfig struct {
	extraParams []string
	skipPaths   []string
}

// WithRedactedParams adds extra query parameter names to redact in addition to the built-in sensitive list
// (token, password, secret, api_key, client_secret, refresh_token, access_token, authorization, state, code)
// Names are matched case-insensitively
func WithRedactedParams(params ...string) LoggerOption {
	return func(c *loggerConfig) { c.extraParams = append(c.extraParams, params...) }
}

// WithSkipPaths suppresses logging for requests whose path exactly matches one of the given paths
// The handler still runs normally; only the log entry is omitted
// Paths are matched as-is against r.URL.Path (exact, case-sensitive)
// Useful for suppressing noise from health-check and metrics endpoints
func WithSkipPaths(paths ...string) LoggerOption {
	return func(c *loggerConfig) { c.skipPaths = append(c.skipPaths, paths...) }
}

func clientIPForLog(r *http.Request, trustedNets []*net.IPNet) string {
	if ip := GetClientIPFromContext(r.Context()); ip != "" {
		return ip
	}
	return httputil.GetClientIPWithNets(r, trustedNets)
}

func redactQuery(raw string, sensitive map[string]struct{}) string {
	if raw == "" {
		return ""
	}
	vals, err := url.ParseQuery(raw)
	if err != nil {
		return "[unparseable]"
	}
	redacted := false
	for key := range vals {
		if _, ok := sensitive[strings.ToLower(key)]; ok {
			vals[key] = []string{"[REDACTED]"}
			redacted = true
		}
	}
	if !redacted {
		return raw
	}
	return vals.Encode()
}

// Logger returns middleware that logs each request (method, path, redacted query, IP, user-agent, request_id)
// and after the handler adds status, latency_ms, and bytes. Log level: Info for 2xx, Warn for 4xx, Error for 5xx
// Sensitive query params (token, password, secret, api_key, etc.) are always redacted; use WithRedactedParams
// to extend the list. If log is nil, the middleware is a no-op. CIDRs are parsed once at construction
func Logger(log logger.Logger, trustedProxyCIDRs []string, opts ...LoggerOption) func(next http.Handler) http.Handler {
	var cfg loggerConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	sensitive := defaultSensitiveQueryParams
	if len(cfg.extraParams) > 0 {
		sensitive = make(map[string]struct{}, len(defaultSensitiveQueryParams)+len(cfg.extraParams))
		for k := range defaultSensitiveQueryParams {
			sensitive[k] = struct{}{}
		}
		for _, p := range cfg.extraParams {
			sensitive[strings.ToLower(p)] = struct{}{}
		}
	}
	skip := make(map[string]struct{}, len(cfg.skipPaths))
	for _, p := range cfg.skipPaths {
		skip[p] = struct{}{}
	}
	trustedNets, parseErr := httputil.ParseTrustedProxyCIDRs(trustedProxyCIDRs)
	if log != nil && parseErr != nil && len(trustedProxyCIDRs) > 0 {
		log.Warn("invalid trusted proxy CIDRs, using RemoteAddr only", logger.Fields{"error": parseErr.Error()})
	}
	return func(next http.Handler) http.Handler {
		if log == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := skip[r.URL.Path]; ok {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()

			fields := map[string]any{
				"method":     r.Method,
				"path":       r.URL.Path,
				"query":      redactQuery(r.URL.RawQuery, sensitive),
				"ip":         clientIPForLog(r, trustedNets),
				"user_agent": r.UserAgent(),
			}
			if reqID := GetRequestID(r.Context()); reqID != "" {
				maps.Copy(fields, logger.RequestID(reqID))
			}
			reqLogger := log.WithFields(fields)
			ctx := logger.IntoContext(r.Context(), reqLogger)

			ww := &statusWriter{ResponseWriter: w}
			next.ServeHTTP(ww, r.WithContext(ctx))

			latency := time.Since(start)
			reqLogger = reqLogger.WithFields(logger.Fields{
				"status":     ww.Status(),
				"latency_ms": latency.Milliseconds(),
				"bytes":      ww.BytesWritten(),
			})
			switch {
			case ww.Status() >= statusServerError:
				reqLogger.Error("http request failed")
			case ww.Status() >= statusClientError:
				reqLogger.Warn("http request error")
			default:
				reqLogger.Info("http request")
			}
		})
	}
}
