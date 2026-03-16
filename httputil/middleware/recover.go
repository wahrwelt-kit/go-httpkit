package middleware

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"runtime/debug"
	"sync"

	logger "github.com/TakuyaYagam1/go-logkit"
)

type recoverResponseWriter struct {
	http.ResponseWriter
	mu         sync.Mutex
	headerSent bool
}

func (rw *recoverResponseWriter) WriteHeader(code int) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	if rw.headerSent {
		return
	}
	rw.headerSent = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *recoverResponseWriter) Write(b []byte) (int, error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	if !rw.headerSent {
		rw.headerSent = true
	}
	return rw.ResponseWriter.Write(b)
}

func (rw *recoverResponseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (rw *recoverResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, errors.New("hijack not supported")
}

func (rw *recoverResponseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// Recoverer returns middleware that recovers panics, logs the panic and stack trace (if log is non-nil), and responds with 500 JSON. Place at the top of the chain.
func Recoverer(log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := &recoverResponseWriter{ResponseWriter: w}
			defer func() {
				if err := recover(); err != nil {
					if log != nil {
						log.Error("panic recovered", logger.Fields{
							"panic": err,
							"stack": string(debug.Stack()),
						})
					}
					rw.mu.Lock()
					if !rw.headerSent {
						rw.headerSent = true
						rw.ResponseWriter.Header().Set("Content-Type", "application/json")
						rw.ResponseWriter.WriteHeader(http.StatusInternalServerError)
						_, _ = rw.ResponseWriter.Write([]byte(`{"error":"Internal server error"}`))
					}
					rw.mu.Unlock()
				}
			}()
			next.ServeHTTP(rw, r)
		})
	}
}
