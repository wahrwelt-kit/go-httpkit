package httputil

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderJSONAttachment(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	data := map[string]int{"a": 1}
	err := RenderJSONAttachment(w, data, "export.json")
	if err != nil {
		t.Fatalf("RenderJSONAttachment: %v", err)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q", ct)
	}
	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "filename=") {
		t.Errorf("Content-Disposition = %q", cd)
	}
	if !strings.Contains(cd, "export.json") {
		t.Errorf("Content-Disposition = %q", cd)
	}
	body := strings.TrimSpace(w.Body.String())
	if body != `{"a":1}` {
		t.Errorf("body = %q", body)
	}
}

func TestRenderJSONAttachment_FilenameSanitized(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	err := RenderJSONAttachment(w, struct{}{}, "../../../etc/passwd")
	if err != nil {
		t.Fatalf("RenderJSONAttachment: %v", err)
	}
	cd := w.Header().Get("Content-Disposition")
	if strings.Contains(cd, "..") || strings.Contains(cd, "etc") {
		t.Errorf("path traversal in filename: Content-Disposition = %q", cd)
	}
}

func TestRenderJSONAttachment_FilenameSpecialChars(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	err := RenderJSONAttachment(w, struct{}{}, "file\x00name")
	if err != nil {
		t.Fatalf("RenderJSONAttachment: %v", err)
	}
	cd := w.Header().Get("Content-Disposition")
	if cd == "" {
		t.Error("Content-Disposition empty")
	}
	if strings.Contains(cd, "\x00") {
		t.Error("control char in Content-Disposition")
	}
}

func TestRenderStream(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	body := []byte("stream data")
	err := RenderStream(w, "text/plain", "data.txt", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("RenderStream: %v", err)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "text/plain" {
		t.Errorf("Content-Type = %q", ct)
	}
	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "data.txt") {
		t.Errorf("Content-Disposition = %q", cd)
	}
	if !bytes.Equal(w.Body.Bytes(), body) {
		t.Errorf("body = %q", w.Body.Bytes())
	}
}

func TestRenderBytes(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	data := []byte("raw bytes")
	err := RenderBytes(w, "application/octet-stream", "file.bin", data)
	if err != nil {
		t.Fatalf("RenderBytes: %v", err)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "application/octet-stream" {
		t.Errorf("Content-Type = %q", ct)
	}
	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "file.bin") {
		t.Errorf("Content-Disposition = %q", cd)
	}
	if !bytes.Equal(w.Body.Bytes(), data) {
		t.Errorf("body = %q", w.Body.Bytes())
	}
}

func TestRenderStream_AllInvalidFilename(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	err := RenderStream(w, "text/plain", "\x00\x01\x02", bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("RenderStream: %v", err)
	}
	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "download") {
		t.Errorf("fallback filename not used: Content-Disposition = %q", cd)
	}
}

func TestRenderStreamLimited_Truncated(t *testing.T) {
	t.Parallel()
	// Source has 10 bytes, limit is 5 - should return ErrStreamTruncated
	w := httptest.NewRecorder()
	src := bytes.NewReader([]byte("0123456789"))
	err := RenderStreamLimited(w, "application/octet-stream", "out.bin", src, 5)
	require.ErrorIs(t, err, ErrStreamTruncated)
	// The first 5 bytes must have been written
	assert.Equal(t, "01234", w.Body.String())
}

func TestRenderStreamLimited_NoLimit(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	src := bytes.NewReader([]byte("hello"))
	err := RenderStreamLimited(w, "application/octet-stream", "out.bin", src, 0)
	require.NoError(t, err)
	assert.Equal(t, "hello", w.Body.String())
}

func TestRenderStream_InvalidContentType(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	err := RenderStream(w, "text/plain\r\nX-Injected: evil", "file.txt", bytes.NewReader(nil))
	require.ErrorIs(t, err, ErrInvalidContentType)
}

func TestRenderBytes_InvalidContentType(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	err := RenderBytes(w, "application/octet-stream\nEvil: header", "file.bin", []byte("data"))
	require.ErrorIs(t, err, ErrInvalidContentType)
}

func TestRenderStreamLimited_NilReader(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	err := RenderStreamLimited(w, "text/plain", "file.txt", nil, 0)
	require.Error(t, err)
}
