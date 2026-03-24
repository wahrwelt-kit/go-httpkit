package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	logger "github.com/wahrwelt-kit/go-logkit"
)

func TestRecoverer_NoPanic(t *testing.T) {
	t.Parallel()
	log, err := logger.New(logger.WithLevel(logger.InfoLevel), logger.WithOutput(logger.ConsoleOutput))
	require.NoError(t, err)
	chain := Recoverer(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	chain.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRecoverer_Panic_500(t *testing.T) {
	t.Parallel()
	chain := Recoverer(logger.Noop())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	chain.ServeHTTP(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "Internal server error")
}

func TestRecoverer_NilLogger(t *testing.T) {
	t.Parallel()
	chain := Recoverer(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("x")
	}))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	chain.ServeHTTP(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
