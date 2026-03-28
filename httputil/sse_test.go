package httputil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSSEWriter_Flushable(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	sw, ok := NewSSEWriter(w)
	require.True(t, ok)
	require.NotNil(t, sw)
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNewSSEWriter_NotFlushable(t *testing.T) {
	t.Parallel()
	w := &nonFlushWriter{ResponseWriter: httptest.NewRecorder()}
	sw, ok := NewSSEWriter(w)
	assert.False(t, ok)
	assert.Nil(t, sw)
}

type nonFlushWriter struct {
	http.ResponseWriter
}

func TestSSEWriter_Send(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	sw, ok := NewSSEWriter(w)
	require.True(t, ok)
	err := sw.Send("ev", "line1")
	require.NoError(t, err)
	body := w.Body.String()
	assert.Contains(t, body, "event: ev\n")
	assert.Contains(t, body, "data: line1\n")
	assert.True(t, strings.HasSuffix(body, "\n\n"))
}

func TestSSEWriter_Send_NoEvent(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	sw, _ := NewSSEWriter(w)
	err := sw.Send("", "data")
	require.NoError(t, err)
	assert.NotContains(t, w.Body.String(), "event:")
	assert.Contains(t, w.Body.String(), "data: data\n")
}

func TestSSEWriter_Send_MultilineData(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	sw, _ := NewSSEWriter(w)
	err := sw.Send("e", "a\nb")
	require.NoError(t, err)
	body := w.Body.String()
	assert.Contains(t, body, "data: a\n")
	assert.Contains(t, body, "data: b\n")
}

func TestSSEWriter_SendJSON(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	sw, _ := NewSSEWriter(w)
	err := sw.SendJSON("msg", map[string]int{"x": 1})
	require.NoError(t, err)
	body := w.Body.String()
	assert.Contains(t, body, "event: msg\n")
	assert.Contains(t, body, `data: {"x":1}`)
}

func TestSSEWriter_Close_NoOpAfter(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	sw, _ := NewSSEWriter(w)
	sw.Close()
	err := sw.Send("e", "d")
	require.ErrorIs(t, err, ErrSSEClosed)
	assert.Empty(t, w.Body.String())
}

func TestSSEWriter_Heartbeat_WritesComment(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	sw, ok := NewSSEWriter(w)
	require.True(t, ok)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		sw.Heartbeat(ctx, 20*time.Millisecond)
	}()

	time.Sleep(60 * time.Millisecond)
	cancel()
	<-done

	body := w.Body.String()
	assert.Contains(t, body, ": ping\n\n")
}

func TestSSEWriter_Heartbeat_StopsWhenClosed(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	sw, _ := NewSSEWriter(w)

	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		defer close(done)
		sw.Heartbeat(ctx, 10*time.Millisecond)
	}()

	time.Sleep(25 * time.Millisecond)
	sw.Close()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Heartbeat did not stop after Close()")
	}
}
