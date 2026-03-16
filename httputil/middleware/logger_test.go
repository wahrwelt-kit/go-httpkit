package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	logger "github.com/TakuyaYagam1/go-logkit"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeLogger struct {
	mu            sync.Mutex
	startFields   logger.Fields
	endFields     logger.Fields
	lastLevel     string
	lastMsg       string
	withFieldsOut logger.Logger
}

func (f *fakeLogger) WithFields(fields logger.Fields) logger.Logger {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.startFields == nil {
		f.startFields = make(logger.Fields)
	}
	for k, v := range fields {
		f.startFields[k] = v
	}
	if f.withFieldsOut == nil {
		f.withFieldsOut = &fakeChildLogger{parent: f}
	}
	return f.withFieldsOut
}

func (f *fakeLogger) WithError(_ error) logger.Logger {
	if f.withFieldsOut == nil {
		f.withFieldsOut = &fakeChildLogger{parent: f}
	}
	return f.withFieldsOut
}

func (f *fakeLogger) Debug(msg string, _ ...logger.Fields) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastLevel, f.lastMsg = "debug", msg
}

func (f *fakeLogger) Info(msg string, _ ...logger.Fields) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastLevel, f.lastMsg = "info", msg
}

func (f *fakeLogger) Warn(msg string, _ ...logger.Fields) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastLevel, f.lastMsg = "warn", msg
}

func (f *fakeLogger) Error(msg string, _ ...logger.Fields) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastLevel, f.lastMsg = "error", msg
}

func (f *fakeLogger) Fatal(msg string, _ ...logger.Fields) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastLevel, f.lastMsg = "fatal", msg
}

type fakeChildLogger struct {
	parent *fakeLogger
}

func (c *fakeChildLogger) WithFields(fields logger.Fields) logger.Logger {
	c.parent.mu.Lock()
	if c.parent.endFields == nil {
		c.parent.endFields = make(logger.Fields)
	}
	for k, v := range fields {
		c.parent.endFields[k] = v
	}
	c.parent.mu.Unlock()
	return c
}

func (c *fakeChildLogger) WithError(_ error) logger.Logger { return c }

func (c *fakeChildLogger) Debug(msg string, _ ...logger.Fields) {
	c.parent.mu.Lock()
	c.parent.lastLevel, c.parent.lastMsg = "debug", msg
	c.parent.mu.Unlock()
}

func (c *fakeChildLogger) Info(msg string, _ ...logger.Fields) {
	c.parent.mu.Lock()
	c.parent.lastLevel, c.parent.lastMsg = "info", msg
	c.parent.mu.Unlock()
}

func (c *fakeChildLogger) Warn(msg string, _ ...logger.Fields) {
	c.parent.mu.Lock()
	c.parent.lastLevel, c.parent.lastMsg = "warn", msg
	c.parent.mu.Unlock()
}

func (c *fakeChildLogger) Error(msg string, _ ...logger.Fields) {
	c.parent.mu.Lock()
	c.parent.lastLevel, c.parent.lastMsg = "error", msg
	c.parent.mu.Unlock()
}

func (c *fakeChildLogger) Fatal(msg string, _ ...logger.Fields) {
	c.parent.mu.Lock()
	c.parent.lastLevel, c.parent.lastMsg = "fatal", msg
	c.parent.mu.Unlock()
}

func newFakeLogger() *fakeLogger {
	return &fakeLogger{}
}

func (f *fakeLogger) child(_ logger.Fields) logger.Logger {
	return &fakeChildLogger{parent: f}
}

func TestLogger_CallsNext(t *testing.T) {
	t.Parallel()
	log := newFakeLogger()
	log.withFieldsOut = log.child(nil)

	r := chi.NewRouter()
	r.Use(Logger(log, nil))
	called := false
	r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestLogger_LogsInfo_OnSuccess(t *testing.T) {
	t.Parallel()
	log := newFakeLogger()
	child := log.child(nil)
	log.withFieldsOut = child

	r := chi.NewRouter()
	r.Use(Logger(log, nil))
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("User-Agent", "test-agent")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, "GET", log.startFields["method"])
	assert.Equal(t, "/health", log.startFields["path"])
	assert.Equal(t, "192.168.1.1", log.startFields["ip"])
	assert.Equal(t, "test-agent", log.startFields["user_agent"])
	assert.Equal(t, http.StatusOK, log.endFields["status"])
	assert.Contains(t, log.endFields, "latency_ms")
	assert.Contains(t, log.endFields, "bytes")
	assert.Equal(t, "info", log.lastLevel)
	assert.Equal(t, "http request", log.lastMsg)
}

func TestLogger_LogsWarn_On4xx(t *testing.T) {
	t.Parallel()
	log := newFakeLogger()
	log.withFieldsOut = log.child(nil)

	r := chi.NewRouter()
	r.Use(Logger(log, nil))
	r.Get("/forbidden", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})

	req := httptest.NewRequest(http.MethodGet, "/forbidden", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, log.endFields["status"])
	assert.Equal(t, "warn", log.lastLevel)
	assert.Equal(t, "http request error", log.lastMsg)
}

func TestLogger_LogsError_On5xx(t *testing.T) {
	t.Parallel()
	log := newFakeLogger()
	log.withFieldsOut = log.child(nil)

	r := chi.NewRouter()
	r.Use(Logger(log, nil))
	r.Get("/broken", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	req := httptest.NewRequest(http.MethodGet, "/broken", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, log.endFields["status"])
	assert.Equal(t, "error", log.lastLevel)
	assert.Equal(t, "http request failed", log.lastMsg)
}

func TestLogger_IncludesQueryAndRequestID_WhenSet(t *testing.T) {
	t.Parallel()
	log := newFakeLogger()
	log.withFieldsOut = log.child(nil)

	r := chi.NewRouter()
	r.Use(RequestID())
	r.Use(Logger(log, nil))
	r.Get("/search", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/search?q=test&page=1", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, "q=test&page=1", log.startFields["query"])
	assert.Contains(t, log.startFields, "request_id")
	assert.NotEmpty(t, log.startFields["request_id"])
}

func TestLogger_RedactsSensitiveQueryParams(t *testing.T) {
	t.Parallel()
	log := newFakeLogger()
	log.withFieldsOut = log.child(nil)

	r := chi.NewRouter()
	r.Use(Logger(log, nil))
	r.Get("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/?token=secret&page=1", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.NotNil(t, log.startFields)
	query, ok := log.startFields["query"].(string)
	require.True(t, ok)
	assert.Contains(t, query, "REDACTED")
	assert.NotContains(t, query, "secret")
}

func TestLogger_DoesNotRedactSubstringParamName(t *testing.T) {
	t.Parallel()
	log := newFakeLogger()
	log.withFieldsOut = log.child(nil)

	r := chi.NewRouter()
	r.Use(Logger(log, nil))
	r.Get("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/?mytokenvalue=foo", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.NotNil(t, log.startFields)
	query, ok := log.startFields["query"].(string)
	require.True(t, ok)
	assert.Equal(t, "mytokenvalue=foo", query)
}
