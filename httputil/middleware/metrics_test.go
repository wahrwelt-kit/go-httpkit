package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetrics_RecordsRequest(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	pathFn := func(*http.Request) string { return "/test" }
	chain := Metrics(reg, pathFn)
	handler := chain(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	metrics, err := reg.Gather()
	require.NoError(t, err)
	var found bool
	for _, m := range metrics {
		if m.GetName() == "http_requests_total" {
			found = true
			break
		}
	}
	assert.True(t, found, "http_requests_total should be registered")
}
