package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	logger "github.com/wahrwelt-kit/go-logkit"
	logmock "github.com/wahrwelt-kit/go-logkit/mock"
)

func TestLogger_CallsNext(t *testing.T) {
	t.Parallel()
	root := logmock.NewMockLogger(t)
	child := logmock.NewMockLogger(t)
	root.On("WithFields", mock.Anything).Return(child)
	child.On("WithFields", mock.Anything).Return(child)
	child.On("Info", "http request", mock.Anything).Return()

	r := chi.NewRouter()
	r.Use(Logger(root, nil))
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
	root := logmock.NewMockLogger(t)
	child := logmock.NewMockLogger(t)
	var startFields, endFields logger.Fields
	root.On("WithFields", mock.Anything).Run(func(args mock.Arguments) {
		startFields = args.Get(0).(logger.Fields) //nolint:forcetypeassert
	}).Return(child)
	child.On("WithFields", mock.Anything).Run(func(args mock.Arguments) {
		endFields = args.Get(0).(logger.Fields) //nolint:forcetypeassert
	}).Return(child)
	child.On("Info", "http request", mock.Anything).Return()

	r := chi.NewRouter()
	r.Use(Logger(root, nil))
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("User-Agent", "test-agent")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, "GET", startFields["method"])
	assert.Equal(t, "/health", startFields["path"])
	assert.Equal(t, "192.168.1.1", startFields["ip"])
	assert.Equal(t, "test-agent", startFields["user_agent"])
	assert.Equal(t, http.StatusOK, endFields["status"])
	assert.Contains(t, endFields, "latency_ms")
	assert.Contains(t, endFields, "bytes")
	child.AssertCalled(t, "Info", "http request")
}

func TestLogger_LogsWarn_On4xx(t *testing.T) {
	t.Parallel()
	root := logmock.NewMockLogger(t)
	child := logmock.NewMockLogger(t)
	var endFields logger.Fields
	root.On("WithFields", mock.Anything).Return(child)
	child.On("WithFields", mock.Anything).Run(func(args mock.Arguments) {
		endFields = args.Get(0).(logger.Fields) //nolint:forcetypeassert
	}).Return(child)
	child.On("Warn", "http request error", mock.Anything).Return()

	r := chi.NewRouter()
	r.Use(Logger(root, nil))
	r.Get("/forbidden", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})

	req := httptest.NewRequest(http.MethodGet, "/forbidden", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, endFields["status"])
	child.AssertCalled(t, "Warn", "http request error")
}

func TestLogger_LogsError_On5xx(t *testing.T) {
	t.Parallel()
	root := logmock.NewMockLogger(t)
	child := logmock.NewMockLogger(t)
	var endFields logger.Fields
	root.On("WithFields", mock.Anything).Return(child)
	child.On("WithFields", mock.Anything).Run(func(args mock.Arguments) {
		endFields = args.Get(0).(logger.Fields) //nolint:forcetypeassert
	}).Return(child)
	child.On("Error", "http request failed", mock.Anything).Return()

	r := chi.NewRouter()
	r.Use(Logger(root, nil))
	r.Get("/broken", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	req := httptest.NewRequest(http.MethodGet, "/broken", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, endFields["status"])
	child.AssertCalled(t, "Error", "http request failed")
}

func TestLogger_IncludesQueryAndRequestID_WhenSet(t *testing.T) {
	t.Parallel()
	root := logmock.NewMockLogger(t)
	child := logmock.NewMockLogger(t)
	var startFields logger.Fields
	root.On("WithFields", mock.Anything).Run(func(args mock.Arguments) {
		startFields = args.Get(0).(logger.Fields) //nolint:forcetypeassert
	}).Return(child)
	child.On("WithFields", mock.Anything).Return(child)
	child.On("Info", "http request", mock.Anything).Return()

	r := chi.NewRouter()
	r.Use(RequestID())
	r.Use(Logger(root, nil))
	r.Get("/search", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/search?q=test&page=1", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, "q=test&page=1", startFields["query"])
	assert.Contains(t, startFields, "request_id")
	assert.NotEmpty(t, startFields["request_id"])
}

func TestLogger_RedactsSensitiveQueryParams(t *testing.T) {
	t.Parallel()
	root := logmock.NewMockLogger(t)
	child := logmock.NewMockLogger(t)
	var startFields logger.Fields
	root.On("WithFields", mock.Anything).Run(func(args mock.Arguments) {
		startFields = args.Get(0).(logger.Fields) //nolint:forcetypeassert
	}).Return(child)
	child.On("WithFields", mock.Anything).Return(child)
	child.On("Info", "http request", mock.Anything).Return()

	r := chi.NewRouter()
	r.Use(Logger(root, nil))
	r.Get("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/?token=secret&page=1", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.NotNil(t, startFields)
	query, ok := startFields["query"].(string)
	require.True(t, ok)
	assert.Contains(t, query, "REDACTED")
	assert.NotContains(t, query, "secret")
}

func TestLogger_WithRedactedParams_RedactsCustomParam(t *testing.T) {
	t.Parallel()
	root := logmock.NewMockLogger(t)
	child := logmock.NewMockLogger(t)
	var startFields logger.Fields
	root.On("WithFields", mock.Anything).Run(func(args mock.Arguments) {
		startFields = args.Get(0).(logger.Fields) //nolint:forcetypeassert
	}).Return(child)
	child.On("WithFields", mock.Anything).Return(child)
	child.On("Info", "http request", mock.Anything).Return()

	r := chi.NewRouter()
	r.Use(Logger(root, nil, WithRedactedParams("apiToken", "x_custom")))
	r.Get("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/?apiToken=abc&x_custom=val&safe=ok", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.NotNil(t, startFields)
	query, ok := startFields["query"].(string)
	require.True(t, ok)
	assert.Contains(t, query, "REDACTED")
	assert.NotContains(t, query, "abc")
	assert.NotContains(t, query, "val")
	assert.Contains(t, query, "safe=ok")
}

func TestLogger_WithSkipPaths_DoesNotLog(t *testing.T) {
	t.Parallel()
	root := logmock.NewMockLogger(t)
	// No mock expectations - if Logger calls any method on root, testify will fail the test

	r := chi.NewRouter()
	r.Use(Logger(root, nil, WithSkipPaths("/health", "/ready")))
	called := false
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.True(t, called, "handler must still be called for skipped paths")
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestLogger_WithSkipPaths_LogsNonSkipped(t *testing.T) {
	t.Parallel()
	root := logmock.NewMockLogger(t)
	child := logmock.NewMockLogger(t)
	root.On("WithFields", mock.Anything).Return(child)
	child.On("WithFields", mock.Anything).Return(child)
	child.On("Info", "http request", mock.Anything).Return()

	r := chi.NewRouter()
	r.Use(Logger(root, nil, WithSkipPaths("/health")))
	r.Get("/api/users", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	child.AssertCalled(t, "Info", "http request")
}

func TestLogger_DoesNotRedactSubstringParamName(t *testing.T) {
	t.Parallel()
	root := logmock.NewMockLogger(t)
	child := logmock.NewMockLogger(t)
	var startFields logger.Fields
	root.On("WithFields", mock.Anything).Run(func(args mock.Arguments) {
		startFields = args.Get(0).(logger.Fields) //nolint:forcetypeassert
	}).Return(child)
	child.On("WithFields", mock.Anything).Return(child)
	child.On("Info", "http request", mock.Anything).Return()

	r := chi.NewRouter()
	r.Use(Logger(root, nil))
	r.Get("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/?mytokenvalue=foo", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.NotNil(t, startFields)
	query, ok := startFields["query"].(string)
	require.True(t, ok)
	assert.Equal(t, "mytokenvalue=foo", query)
}
