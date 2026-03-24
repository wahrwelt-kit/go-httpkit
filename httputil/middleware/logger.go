package middleware

import (
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

var sensitiveQueryParams = map[string]struct{}{
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

func clientIPForLog(r *http.Request, trustedNets []*net.IPNet) string {
	if ip := GetClientIPFromContext(r.Context()); ip != "" {
		return ip
	}
	return httputil.GetClientIPWithNets(r, trustedNets)
}

func redactQuery(raw string) string {
	if raw == "" {
		return ""
	}
	vals, err := url.ParseQuery(raw)
	if err != nil {
		return "[unparseable]"
	}
	redacted := false
	for key := range vals {
		if _, sensitive := sensitiveQueryParams[strings.ToLower(key)]; sensitive {
			vals[key] = []string{"[REDACTED]"}
			redacted = true
		}
	}
	if !redacted {
		return raw
	}
	return vals.Encode()
}

// Logger returns middleware that logs each request (method, path, redacted query, IP, user-agent, request_id) and after the handler adds status, latency_ms, and bytes. Sensitive query params are redacted. If log is nil, the middleware is a no-op.
func Logger(log logger.Logger, trustedProxyCIDRs []string) func(next http.Handler) http.Handler {
	trustedNets, parseErr := httputil.ParseTrustedProxyCIDRs(trustedProxyCIDRs)
	if log != nil && parseErr != nil && len(trustedProxyCIDRs) > 0 {
		log.Warn("invalid trusted proxy CIDRs, using RemoteAddr only", logger.Fields{"error": parseErr.Error()})
	}
	return func(next http.Handler) http.Handler {
		if log == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			fields := map[string]any{
				"method":     r.Method,
				"path":       r.URL.Path,
				"query":      redactQuery(r.URL.RawQuery),
				"ip":         clientIPForLog(r, trustedNets),
				"user_agent": r.UserAgent(),
			}
			if reqID := GetRequestID(r.Context()); reqID != "" {
				for k, v := range logger.RequestID(reqID) {
					fields[k] = v
				}
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
			if ww.Status() >= statusServerError {
				reqLogger.Error("http request failed")
			} else if ww.Status() >= statusClientError {
				reqLogger.Warn("http request error")
			} else {
				reqLogger.Info("http request")
			}
		})
	}
}
