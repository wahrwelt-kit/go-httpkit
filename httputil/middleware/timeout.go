package middleware

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	logger "github.com/wahrwelt-kit/go-logkit"
)

// ErrResponseBodyTooLarge is returned by TimeoutWithLimit when the handler writes more than maxResponseBytes
var ErrResponseBodyTooLarge = errors.New("response body size limit exceeded")

const timeoutGracePeriod = 5 * time.Second

type timeoutWriter struct {
	http.ResponseWriter
	mu             sync.Mutex
	headerCopied   bool
	timedOut       bool
	wrote          bool
	statusCode     int
	body           bytes.Buffer
	maxBodyBytes   int64
	hijackedHeader http.Header
}

func (tw *timeoutWriter) copyHeaderOnce() {
	if tw.headerCopied {
		return
	}
	tw.headerCopied = true
	maps.Copy(tw.ResponseWriter.Header(), tw.hijackedHeader)
}

func (tw *timeoutWriter) Header() http.Header {
	return tw.hijackedHeader
}

func (tw *timeoutWriter) WriteHeader(code int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	tw.copyHeaderOnce()
	if tw.wrote || tw.timedOut {
		return
	}
	if tw.statusCode == 0 {
		tw.statusCode = code
	}
}

func (tw *timeoutWriter) Write(b []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	tw.copyHeaderOnce()
	if tw.timedOut {
		return 0, context.DeadlineExceeded
	}
	if tw.maxBodyBytes > 0 && int64(tw.body.Len())+int64(len(b)) > tw.maxBodyBytes {
		return 0, ErrResponseBodyTooLarge
	}
	return tw.body.Write(b)
}

func (tw *timeoutWriter) flush() {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.wrote || tw.timedOut {
		return
	}
	tw.wrote = true
	if tw.statusCode == 0 {
		tw.statusCode = http.StatusOK
	}
	tw.copyHeaderOnce()
	tw.ResponseWriter.WriteHeader(tw.statusCode)
	_, _ = tw.body.WriteTo(tw.ResponseWriter)
}

func (tw *timeoutWriter) writeTimeoutResponse() {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.wrote || tw.timedOut {
		return
	}
	tw.timedOut = true
	tw.ResponseWriter.Header().Set("Content-Type", "application/json")
	tw.ResponseWriter.WriteHeader(http.StatusServiceUnavailable)
	_, _ = tw.ResponseWriter.Write([]byte(`{"code":"TIMEOUT","message":"request timeout"}`))
}

func (tw *timeoutWriter) writePanicResponse() {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.wrote || tw.timedOut {
		return
	}
	tw.wrote = true
	tw.ResponseWriter.Header().Set("Content-Type", "application/json")
	tw.ResponseWriter.WriteHeader(http.StatusInternalServerError)
	_, _ = tw.ResponseWriter.Write([]byte(`{"code":"INTERNAL_ERROR","message":"Internal server error"}`))
}

// Timeout returns middleware that runs the handler with a context deadline; on timeout responds with 503 JSON. The response is buffered in memory
func Timeout(d time.Duration, log ...logger.Logger) func(http.Handler) http.Handler {
	return TimeoutWithLimit(d, 0, log...)
}

// TimeoutWithLimit is like Timeout but caps the response body size buffered in memory
// When the handler's Write call would exceed maxResponseBytes, Write returns ErrResponseBodyTooLarge
// to the handler - the middleware itself does not surface this error to the chain
// Use maxResponseBytes <= 0 for no limit (equivalent to Timeout)
func TimeoutWithLimit(d time.Duration, maxResponseBytes int64, log ...logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			r = r.WithContext(ctx)
			tw := &timeoutWriter{
				ResponseWriter: w,
				hijackedHeader: make(http.Header),
				maxBodyBytes:   maxResponseBytes,
			}
			done := make(chan struct{})
			var panicLog logger.Logger
			if len(log) > 0 && log[0] != nil {
				panicLog = log[0]
			}
			go func() {
				defer func() {
					if p := recover(); p != nil {
						if panicLog != nil {
							panicLog.Error("panic recovered in timeout middleware", logger.Fields{"panic": fmt.Sprint(p), "stack": string(debug.Stack())})
						}
						tw.writePanicResponse()
					}
					close(done)
				}()
				next.ServeHTTP(tw, r)
			}()
			select {
			case <-done:
				tw.flush()
				return
			case <-ctx.Done():
				tw.writeTimeoutResponse()
				timer := time.NewTimer(timeoutGracePeriod)
				defer timer.Stop()
				select {
				case <-done:
				case <-timer.C:
					if panicLog != nil {
						panicLog.Warn("timeout middleware: handler goroutine did not finish within grace period", logger.Fields{
							"path":         r.URL.Path,
							"method":       r.Method,
							"timeout":      d.String(),
							"grace_period": timeoutGracePeriod.String(),
						})
					}
				}
			}
		})
	}
}
