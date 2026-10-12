package metrics

import (
	"bufio"
	"errors"
	"io"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	keyMethod = "method"
	keyPath   = "path"
	keyStatus = "status"

	metricHTTPRequestsTotal          = "http_requests_total"
	metricHTTPRequestDurationSeconds = "http_request_duration_seconds"
	metricHTTPRequestsInFlight       = "http_requests_in_flight"
)

// PathFromRequest returns the route pattern for the request. Used by Middleware for the path label.
// It must return a stable pattern like "/users/{id}", not the raw path, to avoid unbounded Prometheus cardinality.
// If the function is nil, path is "/unknown" or "/not-found" for 404.
type PathFromRequest func(*http.Request) string

type metricsConfig struct {
	logger       *slog.Logger
	namespace    string
	subsystem    string
	buckets      []float64
	constLabels  prometheus.Labels
	withInFlight bool
}

// Option configures Middleware.
type Option func(*metricsConfig)

// WithLogger sets a logger for metric registration and label compatibility warnings.
func WithLogger(l *slog.Logger) Option {
	return func(c *metricsConfig) { c.logger = l }
}

// WithNamespace sets the Prometheus namespace used in generated metric names.
func WithNamespace(namespace string) Option {
	return func(c *metricsConfig) { c.namespace = namespace }
}

// WithSubsystem sets the Prometheus subsystem used in generated metric names.
func WithSubsystem(subsystem string) Option {
	return func(c *metricsConfig) { c.subsystem = subsystem }
}

// WithBuckets sets histogram buckets for http_request_duration_seconds. Nil or empty uses prometheus.DefBuckets.
func WithBuckets(buckets []float64) Option {
	return func(c *metricsConfig) { c.buckets = slices.Clone(buckets) }
}

// WithConstLabels adds const labels to every metric registered by Middleware.
func WithConstLabels(labels prometheus.Labels) Option {
	return func(c *metricsConfig) { c.constLabels = maps.Clone(labels) }
}

// WithInFlight enables http_requests_in_flight gauge with method and path labels.
func WithInFlight() Option {
	return func(c *metricsConfig) { c.withInFlight = true }
}

// Middleware records http_requests_total and http_request_duration_seconds.
// reg can be nil for prometheus.DefaultRegisterer. pathFromRequest can be nil.
func Middleware(reg prometheus.Registerer, pathFromRequest PathFromRequest, opts ...Option) func(http.Handler) http.Handler {
	var cfg metricsConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	buckets := cfg.buckets
	if len(buckets) == 0 {
		buckets = prometheus.DefBuckets
	}
	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   cfg.namespace,
			Subsystem:   cfg.subsystem,
			Name:        metricHTTPRequestsTotal,
			Help:        "Total number of HTTP requests",
			ConstLabels: cfg.constLabels,
		},
		[]string{keyMethod, keyPath, keyStatus},
	)
	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   cfg.namespace,
			Subsystem:   cfg.subsystem,
			Name:        metricHTTPRequestDurationSeconds,
			Help:        "HTTP request duration in seconds",
			Buckets:     buckets,
			ConstLabels: cfg.constLabels,
		},
		[]string{keyMethod, keyPath, keyStatus},
	)
	requestsInFlight := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   cfg.namespace,
			Subsystem:   cfg.subsystem,
			Name:        metricHTTPRequestsInFlight,
			Help:        "Current number of in-flight HTTP requests",
			ConstLabels: cfg.constLabels,
		},
		[]string{keyMethod, keyPath},
	)
	requestsTotalName := prometheus.BuildFQName(cfg.namespace, cfg.subsystem, metricHTTPRequestsTotal)
	requestDurationName := prometheus.BuildFQName(cfg.namespace, cfg.subsystem, metricHTTPRequestDurationSeconds)
	requestsInFlightName := prometheus.BuildFQName(cfg.namespace, cfg.subsystem, metricHTTPRequestsInFlight)
	requestsTotal = registerCounterVec(reg, requestsTotal, requestsTotalName, cfg.logger)
	requestDuration = registerHistogramVec(reg, requestDuration, requestDurationName, cfg.logger)
	if cfg.withInFlight {
		requestsInFlight = registerGaugeVec(reg, requestsInFlight, requestsInFlightName, cfg.logger)
	}
	var requestsTotalLabelWarn sync.Once
	var requestDurationLabelWarn sync.Once
	var requestsInFlightLabelWarn sync.Once
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			method := metricMethod(r.Method)
			inFlightPath := metricPath(r, pathFromRequest, 0)
			if cfg.withInFlight {
				if !addGaugeVec(requestsInFlight, 1, method, inFlightPath) && cfg.logger != nil {
					requestsInFlightLabelWarn.Do(func() {
						cfg.logger.WarnContext(r.Context(), "metrics middleware: "+requestsInFlightName+" has incompatible labels")
					})
				}
				defer func() {
					_ = addGaugeVec(requestsInFlight, -1, method, inFlightPath)
				}()
			}
			ww := &statusWriter{ResponseWriter: w}
			next.ServeHTTP(ww, r)
			duration := time.Since(start).Seconds()
			status := strconv.Itoa(ww.Status())
			path := metricPath(r, pathFromRequest, ww.Status())
			if !incCounterVec(requestsTotal, method, path, status) && cfg.logger != nil {
				requestsTotalLabelWarn.Do(func() {
					cfg.logger.WarnContext(r.Context(), "metrics middleware: "+requestsTotalName+" has incompatible labels")
				})
			}
			if !observeHistogramVec(requestDuration, duration, method, path, status) && cfg.logger != nil {
				requestDurationLabelWarn.Do(func() {
					cfg.logger.WarnContext(r.Context(), "metrics middleware: "+requestDurationName+" has incompatible labels")
				})
			}
		})
	}
}

// Extension methods share one label to keep rejected requests from growing metrics without bound.
func metricMethod(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodDelete, http.MethodConnect, http.MethodOptions, http.MethodTrace, http.MethodPatch:
		return method
	default:
		return "unknown"
	}
}

func metricPath(r *http.Request, pathFromRequest PathFromRequest, status int) string {
	path := "/unknown"
	if pathFromRequest != nil {
		path = pathFromRequest(r)
	}
	if path != "" {
		return path
	}
	if status == http.StatusNotFound {
		return "/not-found"
	}
	return "/unknown"
}

func registerCounterVec(reg prometheus.Registerer, counter *prometheus.CounterVec, name string, l *slog.Logger) *prometheus.CounterVec {
	if err := reg.Register(counter); err != nil {
		if are, ok := errors.AsType[prometheus.AlreadyRegisteredError](err); ok {
			if cv, ok := are.ExistingCollector.(*prometheus.CounterVec); ok {
				return cv
			}
			if l != nil {
				l.Warn("metrics middleware: "+name+" already registered with different collector type", slog.Any("error", err))
			}
			return counter
		}
		if l != nil {
			l.Warn("metrics middleware: failed to register "+name, slog.Any("error", err))
		}
	}
	return counter
}

func registerHistogramVec(reg prometheus.Registerer, histogram *prometheus.HistogramVec, name string, l *slog.Logger) *prometheus.HistogramVec {
	if err := reg.Register(histogram); err != nil {
		if are, ok := errors.AsType[prometheus.AlreadyRegisteredError](err); ok {
			if hv, ok := are.ExistingCollector.(*prometheus.HistogramVec); ok {
				return hv
			}
			if l != nil {
				l.Warn("metrics middleware: "+name+" already registered with different collector type", slog.Any("error", err))
			}
			return histogram
		}
		if l != nil {
			l.Warn("metrics middleware: failed to register "+name, slog.Any("error", err))
		}
	}
	return histogram
}

func registerGaugeVec(reg prometheus.Registerer, gauge *prometheus.GaugeVec, name string, l *slog.Logger) *prometheus.GaugeVec {
	if err := reg.Register(gauge); err != nil {
		if are, ok := errors.AsType[prometheus.AlreadyRegisteredError](err); ok {
			if gv, ok := are.ExistingCollector.(*prometheus.GaugeVec); ok {
				return gv
			}
			if l != nil {
				l.Warn("metrics middleware: "+name+" already registered with different collector type", slog.Any("error", err))
			}
			return gauge
		}
		if l != nil {
			l.Warn("metrics middleware: failed to register "+name, slog.Any("error", err))
		}
	}
	return gauge
}

func incCounterVec(counter *prometheus.CounterVec, method, path, status string) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	counter.WithLabelValues(method, path, status).Inc()
	return true
}

func observeHistogramVec(histogram *prometheus.HistogramVec, duration float64, method, path, status string) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	histogram.WithLabelValues(method, path, status).Observe(duration)
	return true
}

func addGaugeVec(gauge *prometheus.GaugeVec, value float64, method, path string) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	gauge.WithLabelValues(method, path).Add(value)
	return true
}

type statusWriter struct {
	http.ResponseWriter

	mu     sync.Mutex
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.markStatus(code) {
		w.ResponseWriter.WriteHeader(code)
	}
}

func (w *statusWriter) Write(b []byte) (int, error) {
	w.writeHeaderIfNeeded()
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Status() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *statusWriter) markStatus(code int) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.status != 0 {
		return false
	}
	w.status = code
	return true
}

func (w *statusWriter) writeHeaderIfNeeded() {
	if w.markStatus(http.StatusOK) {
		w.ResponseWriter.WriteHeader(http.StatusOK)
	}
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		w.writeHeaderIfNeeded()
		f.Flush()
	}
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return h.Hijack()
}

func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *statusWriter) ReadFrom(r io.Reader) (int64, error) {
	if rf, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		w.writeHeaderIfNeeded()
		return rf.ReadFrom(r)
	}
	w.writeHeaderIfNeeded()
	return io.Copy(w.ResponseWriter, r)
}
