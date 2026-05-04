package httputil

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	playvalidator "github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wahrwelt-kit/go-httpkit/httperr"
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

func TestDecodeAndValidate_BodyTooLarge(t *testing.T) {
	t.Parallel()
	big := append([]byte(`{"x":1}`), make([]byte, MaxRequestBodySize+100)...)
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(big))
	w := httptest.NewRecorder()
	var v noopValidator
	_, ok := DecodeAndValidate[struct{ X int }](w, r, v)
	assert.False(t, ok)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Contains(t, w.Body.String(), "REQUEST_ENTITY_TOO_LARGE")
}

func TestDecodeAndValidate_TrailingDataWithinLimit(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{"x":1}garbage`)))
	w := httptest.NewRecorder()
	var v noopValidator
	_, ok := DecodeAndValidate[struct{ X int }](w, r, v)
	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_JSON")
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

func TestDecodeAndValidateE_InvalidJSON(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not json")))
	var v noopValidator
	_, err := DecodeAndValidateE[struct{ X int }](r, v)
	require.Error(t, err)
	var he *httperr.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.HTTPStatus())
	assert.Equal(t, "INVALID_JSON", he.GetCode())
}

func TestDecodeAndValidateE_TrailingData(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{"x":1} extra`)))
	var v noopValidator
	_, err := DecodeAndValidateE[struct{ X int }](r, v)
	require.Error(t, err)
	var he *httperr.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, "INVALID_JSON", he.GetCode())
}

func TestDecodeAndValidateE_ValidationError(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{"x":1}`)))
	var v failValidator
	_, err := DecodeAndValidateE[struct{ X int }](r, v)
	require.Error(t, err)
	var he *httperr.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.HTTPStatus())
	assert.Equal(t, "VALIDATION_ERROR", he.GetCode())
}

func TestDecodeAndValidateE_NilRequest(t *testing.T) {
	t.Parallel()
	var v noopValidator
	_, err := DecodeAndValidateE[struct{ X int }](nil, v)
	require.Error(t, err)
	var he *httperr.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusBadRequest, he.HTTPStatus())
	assert.Equal(t, "BAD_REQUEST", he.GetCode())
}

func TestDecodeAndValidateE_NilValidator(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{"x":1}`)))
	_, err := DecodeAndValidateE[struct{ X int }](r, nil)
	require.Error(t, err)
	var he *httperr.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusInternalServerError, he.HTTPStatus())
	assert.Equal(t, "INTERNAL_ERROR", he.GetCode())
}

func TestDecodeAndValidateE_BodyTooLarge(t *testing.T) {
	t.Parallel()
	validThenPad := append([]byte(`{"x":1}`), make([]byte, MaxRequestBodySize+2)...)
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(validThenPad))
	var v noopValidator
	_, err := DecodeAndValidateE[struct{ X int }](r, v)
	require.Error(t, err)
	var he *httperr.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusRequestEntityTooLarge, he.HTTPStatus())
	assert.Equal(t, "REQUEST_ENTITY_TOO_LARGE", he.GetCode())
}

func TestDecodeAndValidateE_BodyExactlyLimit(t *testing.T) {
	t.Parallel()
	body := append([]byte(`{"x":1}`), make([]byte, MaxRequestBodySize+1-7)...)
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	var v noopValidator
	_, err := DecodeAndValidateE[struct{ X int }](r, v)
	require.Error(t, err)
	var he *httperr.HTTPError
	require.ErrorAs(t, err, &he)
	assert.Equal(t, http.StatusRequestEntityTooLarge, he.HTTPStatus())
	assert.Equal(t, "REQUEST_ENTITY_TOO_LARGE", he.GetCode())
}

func TestDecodeAndValidateE_Success(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{"x":42}`)))
	var v noopValidator
	type req struct {
		X int `json:"x"`
	}
	got, err := DecodeAndValidateE[req](r, v)
	require.NoError(t, err)
	assert.Equal(t, 42, got.X)
}

func TestDecodeJSON_Success(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{"x":10}`)))
	var out struct {
		X int `json:"x"`
	}
	err := DecodeJSON(r, &out)
	require.NoError(t, err)
	assert.Equal(t, 10, out.X)
}

func TestDecodeJSON_InvalidJSON(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not json")))
	var out struct{ X int }
	err := DecodeJSON(r, &out)
	require.Error(t, err)
}

func TestDecodeJSON_TrailingData(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{"x":1} x`)))
	var out struct{ X int }
	err := DecodeJSON(r, &out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trailing")
}

func TestDecodeJSON_NilRequest(t *testing.T) {
	t.Parallel()
	var out struct{ X int }
	err := DecodeJSON(nil, &out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestDecodeJSON_NilTarget(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{}`)))
	err := DecodeJSON(r, (*struct{ X int })(nil))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestDecodeJSON_BodyTooLarge(t *testing.T) {
	t.Parallel()
	body := append([]byte(`{"x":1}`), make([]byte, MaxRequestBodySize+2)...)
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	var out struct{ X int }
	err := DecodeJSON(r, &out)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRequestBodyTooLarge)
}

type playgroundValidator struct{ v *playvalidator.Validate }

func (p playgroundValidator) Validate(val any) error { return p.v.Struct(val) }

func TestDecodeAndValidate_PlaygroundValidation(t *testing.T) {
	t.Parallel()
	v := playgroundValidator{v: playvalidator.New()}
	type req struct {
		Email string `json:"email" validate:"required,email"`
		Name  string `json:"name" validate:"required"`
	}
	body := bytes.NewReader([]byte(`{"email":"bad","name":""}`))
	r := httptest.NewRequest(http.MethodPost, "/", body)
	w := httptest.NewRecorder()
	_, ok := DecodeAndValidate[req](w, r, v)
	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "VALIDATION_ERROR")
	assert.Contains(t, w.Body.String(), "errors")
}

func TestDecodeAndValidateE_PlaygroundValidation(t *testing.T) {
	t.Parallel()
	v := playgroundValidator{v: playvalidator.New()}
	type req struct {
		Email string `json:"email" validate:"required,email"`
	}
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{"email":"bad"}`)))
	_, err := DecodeAndValidateE[req](r, v)
	require.Error(t, err)
	var valErr *ValidationHTTPError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, http.StatusBadRequest, valErr.HTTPStatus())
	assert.NotEmpty(t, valErr.Errors)
	assert.Equal(t, "Email", valErr.Errors[0].Field)
}

func TestDecodeAndValidate_NilBody(t *testing.T) {
	t.Parallel()
	r := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	r.Body = nil
	w := httptest.NewRecorder()
	var v noopValidator
	_, ok := DecodeAndValidate[struct{ X int }](w, r, v)
	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDecodeAndValidate_NilValidator(t *testing.T) {
	t.Parallel()
	body := bytes.NewReader([]byte(`{"x":1}`))
	r := httptest.NewRequest(http.MethodPost, "/", body)
	w := httptest.NewRecorder()
	_, ok := DecodeAndValidate[struct{ X int }](w, r, nil)
	assert.False(t, ok)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDecodeAndValidate_WithMaxBodySize(t *testing.T) {
	t.Parallel()
	body := bytes.NewReader([]byte(`{"x":1}`))
	r := httptest.NewRequest(http.MethodPost, "/", body)
	w := httptest.NewRecorder()
	var v noopValidator
	type req struct {
		X int `json:"x"`
	}
	got, ok := DecodeAndValidate[req](w, r, v, WithMaxBodySize(1024))
	assert.True(t, ok)
	assert.Equal(t, 1, got.X)
}

func TestSanitizeValidationField(t *testing.T) {
	t.Parallel()
	const fieldEmail = "Email"
	tests := []struct {
		in   string
		want string
	}{
		{fieldEmail, fieldEmail},
		{"User.Email", fieldEmail},
		{"a.b.c", "c"},
		{"X", "X"},
	}
	for _, tt := range tests {
		got := sanitizeValidationField(tt.in)
		assert.Equal(t, tt.want, got, "sanitizeValidationField(%q)", tt.in)
	}
}

func TestSanitizeValidationMessage(t *testing.T) {
	t.Parallel()
	v := playvalidator.New()
	type s struct {
		A string `validate:"required"`
		B string `validate:"email"`
		C int    `validate:"min=5"`
	}
	err := v.Struct(s{})
	var valErrs playvalidator.ValidationErrors
	require.ErrorAs(t, err, &valErrs)
	items := validationErrorsToItems(valErrs)
	require.Len(t, items, 3)
	assert.Equal(t, "A", items[0].Field)
	assert.Equal(t, "Required", items[0].Message)
	assert.Equal(t, "B", items[1].Field)
	assert.Equal(t, "Invalid format", items[1].Message)
	assert.Equal(t, "C", items[2].Field)
	assert.Equal(t, "Invalid value", items[2].Message)
}

func BenchmarkDecodeAndValidate(b *testing.B) {
	body := []byte(`{"x":42}`)
	var v noopValidator
	type req struct {
		X int `json:"x"`
	}
	for b.Loop() {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		_, _ = DecodeAndValidate[req](w, r, v)
	}
}

func BenchmarkDecodeAndValidateE(b *testing.B) {
	body := []byte(`{"x":42}`)
	var v noopValidator
	type req struct {
		X int `json:"x"`
	}
	for b.Loop() {
		r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		_, _ = DecodeAndValidateE[req](r, v)
	}
}
