package middleware

import (
	"net/http"
	"runtime/debug"

	logger "github.com/wahrwelt-kit/go-logkit"
)

// Recoverer returns middleware that recovers panics, logs the panic and stack trace (if log is non-nil), and responds with 500 JSON. Place at the top of the chain
func Recoverer(log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := &statusWriter{ResponseWriter: w}
			defer func() {
				if err := recover(); err != nil {
					if log != nil {
						log.Error("panic recovered", logger.Fields{
							"panic": err,
							"stack": string(debug.Stack()),
						})
					}
					if rw.claimHeaderSent() {
						rw.ResponseWriter.Header().Set("Content-Type", "application/json")
						rw.ResponseWriter.WriteHeader(http.StatusInternalServerError)
						_, _ = rw.ResponseWriter.Write([]byte(`{"code":"INTERNAL_ERROR","message":"Internal server error"}`))
					}
				}
			}()
			next.ServeHTTP(rw, r)
		})
	}
}
