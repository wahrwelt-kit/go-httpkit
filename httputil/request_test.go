package httputil

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type noopValidator struct{}

func (noopValidator) Validate(any) error { return nil }

type failValidator struct{}

func (failValidator) Validate(any) error { return errors.New("validation failed") }

func TestDecodeAndValidate_InvalidJSON(t *testing.T) {
	t.Parallel()
	body := bytes.NewReader([]byte("not json"))
	r := httptest.NewRequest(http.MethodPost, "/", body)
	w := httptest.NewRecorder()
	var v noopValidator
	_, ok := DecodeAndValidate[struct{ X int }](w, r, v)
	if ok {
		t.Error("expected false")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDecodeAndValidate_TrailingData(t *testing.T) {
	t.Parallel()
	body := bytes.NewReader([]byte(`{"x":1} extra`))
	r := httptest.NewRequest(http.MethodPost, "/", body)
	w := httptest.NewRecorder()
	var v noopValidator
	_, ok := DecodeAndValidate[struct{ X int }](w, r, v)
	if ok {
		t.Error("expected false")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDecodeAndValidate_ValidationError(t *testing.T) {
	t.Parallel()
	body := bytes.NewReader([]byte(`{"x":1}`))
	r := httptest.NewRequest(http.MethodPost, "/", body)
	w := httptest.NewRecorder()
	var v failValidator
	_, ok := DecodeAndValidate[struct{ X int }](w, r, v)
	if ok {
		t.Error("expected false")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDecodeAndValidate_NilRequest(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	var v noopValidator
	_, ok := DecodeAndValidate[struct{ X int }](w, nil, v)
	if ok {
		t.Error("expected false")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDecodeAndValidate_Success(t *testing.T) {
	t.Parallel()
	body := bytes.NewReader([]byte(`{"x":42}`))
	r := httptest.NewRequest(http.MethodPost, "/", body)
	w := httptest.NewRecorder()
	var v noopValidator
	type req struct {
		X int `json:"x"`
	}
	got, ok := DecodeAndValidate[req](w, r, v)
	if !ok {
		t.Fatal("expected true")
	}
	if got.X != 42 {
		t.Errorf("got.X = %d, want 42", got.X)
	}
	if w.Code != http.StatusOK && w.Code != 0 {
		t.Errorf("status = %d", w.Code)
	}
}
