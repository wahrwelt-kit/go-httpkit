package httputil

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wahrwelt-kit/go-httpkit/httperr"
)

func TestHandleError_HTTPError(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	err := httperr.New(errors.New("not found"), http.StatusNotFound, "NOT_FOUND")
	HandleError(w, r, err)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
	var body ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != "NOT_FOUND" {
		t.Errorf("Code = %q, want NOT_FOUND", body.Code)
	}
	if body.Message != "not found" {
		t.Errorf("Message = %q", body.Message)
	}
}

func TestHandleError_HTTPError_EmptyCodeUsesCodeFromStatus(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	err := httperr.New(errors.New("bad"), http.StatusBadRequest, "")
	HandleError(w, r, err)
	var body ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != "BAD_REQUEST" {
		t.Errorf("Code = %q, want BAD_REQUEST", body.Code)
	}
}

func TestHandleError_HTTPError_5xxHidesMessage(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	err := httperr.New(errors.New("internal detail"), http.StatusInternalServerError, "INTERNAL_ERROR")
	HandleError(w, r, err)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d", w.Code)
	}
	var body ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Message != msgInternalServerError {
		t.Errorf("Message = %q, want generic message", body.Message)
	}
}

func TestHandleError_GenericError(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	HandleError(w, r, errors.New("generic"))
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	var body ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != "INTERNAL_ERROR" {
		t.Errorf("Code = %q", body.Code)
	}
	if body.Message != msgInternalServerError {
		t.Errorf("Message = %q", body.Message)
	}
}

func TestHandleError_ContextDeadlineExceeded(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	HandleError(w, r, context.DeadlineExceeded)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var body ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, httperr.CodeTimeout, body.Code)
	assert.Equal(t, "request timeout", body.Message)
}

func TestHandleError_ValidationHTTPError(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	err := &ValidationHTTPError{
		HTTPError: httperr.New(errors.New("validation failed"), http.StatusBadRequest, "VALIDATION_ERROR"),
		Errors: []ValidationErrorItem{
			{Field: tagEmail, Message: "Invalid format"},
		},
	}
	HandleError(w, r, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body ValidationErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "VALIDATION_ERROR", body.Code)
	assert.Equal(t, "Validation failed", body.Message)
	require.Len(t, body.Errors, 1)
	assert.Equal(t, tagEmail, body.Errors[0].Field)
	assert.Equal(t, "Invalid format", body.Errors[0].Message)
}

func TestHandleError_ValidationHTTPError_NilHTTPError(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	err := &ValidationHTTPError{HTTPError: nil, Errors: nil}
	HandleError(w, r, err)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var body ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "INTERNAL_ERROR", body.Code)
}

func TestHandleError_Nil(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	HandleError(w, r, nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestValidationHTTPError_NilReceiver(t *testing.T) {
	t.Parallel()
	var e *ValidationHTTPError
	assert.Empty(t, e.Error())
	require.NoError(t, e.Unwrap())
	assert.Equal(t, 0, e.HTTPStatus())
	assert.Empty(t, e.GetCode())
}

func TestValidationHTTPError_NilHTTPError(t *testing.T) {
	t.Parallel()
	e := &ValidationHTTPError{HTTPError: nil}
	assert.Empty(t, e.Error())
	assert.Equal(t, 0, e.HTTPStatus())
	assert.Empty(t, e.GetCode())
}

func TestErrorHandler_Handle_Nil(t *testing.T) {
	t.Parallel()
	h := &ErrorHandler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	assert.False(t, h.Handle(w, r, nil, "test"))
}

func TestErrorHandler_Handle_WithLogger_4xx(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	l := slog.New(slog.NewJSONHandler(&logs, nil))

	h := &ErrorHandler{Logger: l}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	err := httperr.New(errors.New("bad"), http.StatusBadRequest, "BAD_REQUEST")
	assert.True(t, h.Handle(w, r, err, "client error"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, logs.String(), `"level":"INFO"`)
	assert.Contains(t, logs.String(), `"msg":"client error"`)
	assert.Contains(t, logs.String(), `"error"`)
}

func TestErrorHandler_Handle_WithLogger_5xx(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	l := slog.New(slog.NewJSONHandler(&logs, nil))

	h := &ErrorHandler{Logger: l}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	err := errors.New("internal")
	assert.True(t, h.Handle(w, r, err, "server error"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, logs.String(), `"level":"ERROR"`)
	assert.Contains(t, logs.String(), `"msg":"server error"`)
	assert.Contains(t, logs.String(), `"error"`)
}

func TestSanitizeValidationFieldName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want string
	}{
		{tagEmail, tagEmail},
		{"user_id", "user_id"},
		{"<script>", "script"},
		{"", "field"},
		{"日本語", "field"},
		{"a-b", "a-b"},
	}
	for _, tt := range tests {
		got := sanitizeValidationFieldName(tt.in)
		assert.Equal(t, tt.want, got, "sanitizeValidationFieldName(%q)", tt.in)
	}
}

func BenchmarkHandleError_HTTPError(b *testing.B) {
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	err := httperr.New(errors.New("not found"), http.StatusNotFound, "NOT_FOUND")
	for b.Loop() {
		w := httptest.NewRecorder()
		HandleError(w, r, err)
	}
}

func BenchmarkHandleError_GenericError(b *testing.B) {
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	err := errors.New("internal failure")
	for b.Loop() {
		w := httptest.NewRecorder()
		HandleError(w, r, err)
	}
}
