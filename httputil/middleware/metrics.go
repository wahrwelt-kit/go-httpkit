package middleware

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	logger "github.com/wahrwelt-kit/go-logkit"
)

// PathFromRequest returns the route pattern for the request (e.g. from chi.RouteContext). Used by Metrics for the path label. Must return a stable pattern like "/users/{id}", not the raw path, to avoid unbounded Prometheus cardinality. If the function is nil, path is "/unknown" or "/not-found" for 404.
type PathFromRequest func(*http.Request) string

// Metrics returns middleware that records http_requests_total and http_request_duration_seconds. reg can be nil for DefaultRegisterer. pathFromRequest can be nil. Optional logger for registration errors; if nil, errors are silently ignored.
func Metrics(reg prometheus.Registerer, pathFromRequest PathFromRequest, log ...logger.Logger) func(http.Handler) http.Handler {
	var logger logger.Logger
	if len(log) > 0 {
		logger = log[0]
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
			} else if logger != nil {
				logger.WithError(err).Warn("metrics middleware: http_requests_total already registered with different collector type")
			}
		} else if logger != nil {
			logger.WithError(err).Warn("metrics middleware: failed to register http_requests_total")
		}
	}
	if err := reg.Register(requestDuration); err != nil {
		var are prometheus.AlreadyRegisteredError
		if errors.As(err, &are) {
			if hv, ok := are.ExistingCollector.(*prometheus.HistogramVec); ok {
				requestDuration = hv
			} else if logger != nil {
				logger.WithError(err).Warn("metrics middleware: http_request_duration_seconds already registered with different collector type")
			}
		} else if logger != nil {
			logger.WithError(err).Warn("metrics middleware: failed to register http_request_duration_seconds")
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &statusWriter{ResponseWriter: w}
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
