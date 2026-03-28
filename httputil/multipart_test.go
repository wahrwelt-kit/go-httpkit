package httputil

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMultipartFormLimit_Success(t *testing.T) {
	t.Parallel()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	_ = mw.WriteField("a", "b")
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()

	ok := ParseMultipartFormLimit(w, req, 1<<20, 1<<20)
	require.True(t, ok)
	assert.Empty(t, w.Body.Bytes())
}

func TestParseMultipartFormLimit_OverLimit_Returns413(t *testing.T) {
	t.Parallel()
	body := bytes.NewReader(make([]byte, 100))
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	w := httptest.NewRecorder()

	ok := ParseMultipartFormLimit(w, req, 50, 1<<20)
	require.False(t, ok)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Contains(t, w.Body.String(), "REQUEST_ENTITY_TOO_LARGE")
}

func TestParseMultipartFormLimit_InvalidForm_Returns400(t *testing.T) {
	t.Parallel()
	// Body is within size but is not a valid multipart form
	body := bytes.NewReader([]byte("not a multipart form"))
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=validboundary")
	w := httptest.NewRecorder()

	ok := ParseMultipartFormLimit(w, req, 1<<20, 1<<20)
	require.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "VALIDATION_ERROR")
}
