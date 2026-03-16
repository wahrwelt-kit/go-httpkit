package middleware

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	logger "github.com/TakuyaYagam1/go-logkit"
)

// ErrResponseBodyTooLarge is returned by timeoutWriter.Write when the buffered response body would exceed the configured maxResponseBytes.
var ErrResponseBodyTooLarge = errors.New("response body size limit exceeded")

const timeoutGracePeriod = 5 * time.Second

type timeoutWriter struct {
	http.ResponseWriter
	mu             sync.Mutex
	headerMu       sync.Mutex
	headerCopied   bool
	timedOut       bool
	wrote          bool
	statusCode     int
	body           bytes.Buffer
	maxBodyBytes   int64
	hijackedHeader http.Header
}

func (tw *timeoutWriter) copyHeaderOnce() {
	tw.headerMu.Lock()
	defer tw.headerMu.Unlock()
	if tw.headerCopied {
		return
	}
	tw.headerCopied = true
	for k, v := range tw.hijackedHeader {
		tw.ResponseWriter.Header()[k] = v
	}
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
	_, _ = tw.ResponseWriter.Write([]byte(`{"error":"request timeout"}`))
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
	_, _ = tw.ResponseWriter.Write([]byte(`{"error":"Internal server error"}`))
}

// Timeout returns middleware that runs the handler with a context deadline of d. If the deadline is exceeded, responds with 503 JSON. The response body is buffered in memory—do not use for streaming or large responses. The handler may keep running after the response; it should respect context cancellation. After sending the timeout response, the middleware waits up to timeoutGracePeriod for the handler to finish; if the handler does not complete in that time, its goroutine may still be running—handlers should check ctx.Done() to exit promptly, and operators should monitor for goroutine leaks. Optional log is used to log recovered panics (panic value and stack) from the handler goroutine.
func Timeout(d time.Duration, log ...logger.Logger) func(http.Handler) http.Handler {
	return TimeoutWithLimit(d, 0, log...)
}

// TimeoutWithLimit is like Timeout but rejects response body writes when total buffered size would exceed maxResponseBytes (0 = unlimited). Optional log is used to log recovered panics.
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
				// Handler may still be running; we wait up to timeoutGracePeriod then return. The handler goroutine is not killed—handlers must respect ctx.Done() to avoid leaks.
				timer := time.NewTimer(timeoutGracePeriod)
				defer timer.Stop()
				select {
				case <-done:
				case <-timer.C:
				}
			}
		})
	}
}
