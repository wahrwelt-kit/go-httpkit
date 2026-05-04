package httperr

import (
	"errors"
	"net/http"
	"testing"
)

func TestCodeFromStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status int
		want   string
	}{
		{http.StatusBadRequest, CodeBadRequest},
		{http.StatusUnauthorized, CodeUnauthorized},
		{http.StatusForbidden, CodeForbidden},
		{http.StatusNotFound, CodeNotFound},
		{http.StatusConflict, CodeConflict},
		{http.StatusGone, CodeGone},
		{http.StatusPaymentRequired, CodePaymentRequired},
		{http.StatusUnprocessableEntity, CodeValidationError},
		{http.StatusTooManyRequests, CodeRateLimitExceeded},
		{http.StatusServiceUnavailable, CodeServiceUnavailable},
		{http.StatusInternalServerError, CodeInternalError},
		{999, CodeInternalError},
	}
	for _, tt := range tests {
		got := CodeFromStatus(tt.status)
		if got != tt.want {
			t.Errorf("CodeFromStatus(%d) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestNew(t *testing.T) {
	t.Parallel()
	err := New(nil, http.StatusBadRequest, "CUSTOM")
	if err == nil {
		t.Fatal("New(nil, ...) should not return nil")
	}
	if err.HTTPStatus() != http.StatusBadRequest {
		t.Errorf("HTTPStatus() = %d, want %d", err.HTTPStatus(), http.StatusBadRequest)
	}
	if err.GetCode() != "CUSTOM" {
		t.Errorf("GetCode() = %q, want CUSTOM", err.GetCode())
	}
	if err.Unwrap() == nil {
		t.Error("Unwrap() should not be nil")
	}
	if !err.IsClientError() {
		t.Error("4xx should be client error")
	}
}

func TestNew_5xx_NotClientError(t *testing.T) {
	t.Parallel()
	err := New(errors.New("x"), http.StatusInternalServerError, "INTERNAL")
	if err.IsClientError() {
		t.Error("5xx should not be client error")
	}
}

func TestNewValidationErrorf(t *testing.T) {
	t.Parallel()
	err := NewValidationErrorf("field %s invalid", "x")
	if err == nil {
		t.Fatal("NewValidationErrorf should not return nil")
	}
	if err.HTTPStatus() != http.StatusBadRequest {
		t.Errorf("HTTPStatus() = %d, want %d", err.HTTPStatus(), http.StatusBadRequest)
	}
	if err.GetCode() != CodeValidationError {
		t.Errorf("GetCode() = %q, want %q", err.GetCode(), CodeValidationError)
	}
	if err.Error() == "" {
		t.Error("Error() should not be empty")
	}
}

func TestIsExpectedClientError(t *testing.T) {
	t.Parallel()
	if IsExpectedClientError(nil) {
		t.Error("nil should not be expected client error")
	}
	if !IsExpectedClientError(ErrInvalidID()) {
		t.Error("ErrInvalidID (4xx) should be reported as expected client error")
	}
	err := New(errors.New("x"), http.StatusNotFound, CodeNotFound)
	if !IsExpectedClientError(err) {
		t.Error("4xx HTTPError should be expected client error")
	}
	err500 := New(errors.New("x"), http.StatusInternalServerError, "INTERNAL")
	if IsExpectedClientError(err500) {
		t.Error("5xx should not be expected client error")
	}
}

func TestSentinels_StatusCodeAndCode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		err        *HTTPError
		wantStatus int
		wantCode   string
	}{
		{"ErrForbidden", ErrForbidden(), http.StatusForbidden, CodeForbidden},
		{"ErrNotFound", ErrNotFound(), http.StatusNotFound, CodeNotFound},
		{"ErrConflict", ErrConflict(), http.StatusConflict, CodeConflict},
		{"ErrGone", ErrGone(), http.StatusGone, CodeGone},
		{"ErrUnprocessableEntity", ErrUnprocessableEntity(), http.StatusUnprocessableEntity, CodeValidationError},
		{"ErrTooManyRequests", ErrTooManyRequests(), http.StatusTooManyRequests, CodeRateLimitExceeded},
		{"ErrServiceUnavailable", ErrServiceUnavailable(), http.StatusServiceUnavailable, CodeServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.HTTPStatus() != tt.wantStatus {
				t.Errorf("HTTPStatus() = %d, want %d", tt.err.HTTPStatus(), tt.wantStatus)
			}
			if tt.err.GetCode() != tt.wantCode {
				t.Errorf("GetCode() = %q, want %q", tt.err.GetCode(), tt.wantCode)
			}
		})
	}
}

func TestHTTPError_NilReceiver(t *testing.T) {
	t.Parallel()
	var e *HTTPError
	if e.Error() != "" {
		t.Error("nil Error() should be empty")
	}
	if e.Unwrap() != nil {
		t.Error("nil Unwrap() should be nil")
	}
	if e.HTTPStatus() != 0 {
		t.Error("nil HTTPStatus() should be 0")
	}
	if e.GetCode() != "" {
		t.Error("nil GetCode() should be empty")
	}
	if e.IsClientError() {
		t.Error("nil IsClientError() should be false")
	}
}
