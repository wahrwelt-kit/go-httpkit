package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientIP_SetsContext(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	mw, err := ClientIP(nil)
	require.NoError(t, err)
	r.Use(mw)
	var capturedIP string
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		capturedIP = GetClientIPFromContext(req.Context())
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.5:12345"
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "192.168.1.5", capturedIP)
}

func TestGetClientIPFromContext_Empty(t *testing.T) {
	t.Parallel()
	assert.Empty(t, GetClientIPFromContext(context.Background()))
}

func TestClientIP_InvalidCIDRsReturnsError(t *testing.T) {
	t.Parallel()
	_, err := ClientIP([]string{"not-a-cidr"})
	require.Error(t, err)
}
