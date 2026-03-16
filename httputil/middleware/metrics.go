package middleware

import (
	"bufio"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type metricsLogger interface {
	Printf(format string, args ...any)
}

type wrapWriter struct {
	http.ResponseWriter
	status int
}

func (w *wrapWriter) WriteHeader(code int) {
	if w.status != 0 {
		return
	}
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *wrapWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *wrapWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *wrapWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, errors.New("hijack not supported")
}

func (w *wrapWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *wrapWriter) ReadFrom(r io.Reader) (int64, error) {
	if rf, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(r)
	}
	return io.Copy(w.ResponseWriter, r)
}

// PathFromRequest returns the route pattern for the request (e.g. from chi.RouteContext). Used by Metrics for the path label. Must return a stable pattern like "/users/{id}", not the raw path, to avoid unbounded Prometheus cardinality. If the function is nil, path is "/unknown" or "/not-found" for 404.
type PathFromRequest func(*http.Request) string

// Metrics returns middleware that records http_requests_total and http_request_duration_seconds. reg can be nil for DefaultRegisterer. pathFromRequest can be nil. Optional logger for registration errors; if nil, log.Default() is used.
func Metrics(reg prometheus.Registerer, pathFromRequest PathFromRequest, logger ...metricsLogger) func(http.Handler) http.Handler {
	var logr metricsLogger = log.Default()
	if len(logger) > 0 && logger[0] != nil {
		logr = logger[0]
	}
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)
	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)
	if err := reg.Register(requestsTotal); err != nil {
		var are prometheus.AlreadyRegisteredError
		if errors.As(err, &are) {
			if cv, ok := are.ExistingCollector.(*prometheus.CounterVec); ok {
				requestsTotal = cv
			} else {
				logr.Printf("metrics middleware: http_requests_total already registered with different collector type, metrics may not be collected: %v", err)
			}
		} else {
			logr.Printf("metrics middleware: failed to register http_requests_total: %v", err)
		}
	}
	if err := reg.Register(requestDuration); err != nil {
		var are prometheus.AlreadyRegisteredError
		if errors.As(err, &are) {
			if hv, ok := are.ExistingCollector.(*prometheus.HistogramVec); ok {
				requestDuration = hv
			} else {
				logr.Printf("metrics middleware: http_request_duration_seconds already registered with different collector type, metrics may not be collected: %v", err)
			}
		} else {
			logr.Printf("metrics middleware: failed to register http_request_duration_seconds: %v", err)
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &wrapWriter{ResponseWriter: w, status: 0}
			next.ServeHTTP(ww, r)
			duration := time.Since(start).Seconds()
			status := strconv.Itoa(ww.Status())
			path := "/unknown"
			if pathFromRequest != nil {
				path = pathFromRequest(r)
			}
			if path == "" {
				if ww.Status() == http.StatusNotFound {
					path = "/not-found"
				} else {
					path = "/unknown"
				}
			}
			method := r.Method
			requestsTotal.WithLabelValues(method, path, status).Inc()
			requestDuration.WithLabelValues(method, path, status).Observe(duration)
		})
	}
}
