package httputil

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthHandler_AllOk(t *testing.T) {
	t.Parallel()
	handler := HealthHandler(map[string]Checker{
		"db":    okChecker{},
		"redis": okChecker{},
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	handler(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	if ct := w.Header().Get("Content-Type"); ct != mimeApplicationJSON {
		t.Fatalf("Content-Type = %q, want %q", ct, mimeApplicationJSON)
	}
	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "ok", body.Status)
	assert.Equal(t, "ok", body.Checks["db"])
	assert.Equal(t, "ok", body.Checks["redis"])
}

func TestHealthHandler_Degraded(t *testing.T) {
	t.Parallel()
	handler := HealthHandler(map[string]Checker{
		"db":    okChecker{},
		"redis": errChecker{},
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	handler(w, r)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "degraded", body.Status)
	assert.Equal(t, "ok", body.Checks["db"])
	assert.Equal(t, "error", body.Checks["redis"])
}

func TestHealthHandler_NilChecker(t *testing.T) {
	t.Parallel()
	handler := HealthHandler(map[string]Checker{
		"db": nil,
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	handler(w, r)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "degraded", body.Status)
	assert.Equal(t, healthStatusError, body.Checks["db"])
}

func TestHealthHandler_CustomTimeout(t *testing.T) {
	t.Parallel()
	var gotDeadline time.Time
	checker := checkerFunc(func(ctx context.Context) error {
		if d, ok := ctx.Deadline(); ok {
			gotDeadline = d
		}
		return nil
	})
	handler := HealthHandler(map[string]Checker{
		"db": checker,
	}, HealthTimeout(1*time.Second))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	before := time.Now()
	handler(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, gotDeadline.After(before), "deadline should be in the future")
	assert.True(t, gotDeadline.Before(before.Add(2*time.Second)), "deadline should be within 1s timeout")
}

func TestHealthHandler_TimeoutDoesNotWaitForBlockedChecker(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	handler := HealthHandler(map[string]Checker{
		"slow": checkerFunc(func(context.Context) error {
			<-release
			return nil
		}),
	}, HealthTimeout(50*time.Millisecond))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)

	done := make(chan struct{})
	go func() {
		handler(w, r)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		close(release)
		t.Fatal("health handler waited for a checker after timeout")
	}
	close(release)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "degraded", body.Status)
	assert.Equal(t, healthStatusError, body.Checks["slow"])
}

func TestHealthHandler_TimeoutDoesNotStartDuplicateBlockedChecker(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	var calls atomic.Int32
	handler := HealthHandler(map[string]Checker{
		"slow": checkerFunc(func(context.Context) error {
			calls.Add(1)
			<-release
			return nil
		}),
	}, HealthTimeout(25*time.Millisecond))
	r := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)

	w1 := httptest.NewRecorder()
	handler(w1, r)
	require.Equal(t, http.StatusServiceUnavailable, w1.Code)
	require.Equal(t, int32(1), calls.Load())

	w2 := httptest.NewRecorder()
	handler(w2, r)
	close(release)

	assert.Equal(t, http.StatusServiceUnavailable, w2.Code)
	assert.Equal(t, int32(1), calls.Load())
}

type okChecker struct{}

func (okChecker) Check(context.Context) error { return nil }

type errChecker struct{}

func (errChecker) Check(context.Context) error { return errors.New("unavailable") }

type checkerFunc func(ctx context.Context) error

func (f checkerFunc) Check(ctx context.Context) error { return f(ctx) }
