package httputil

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	playvalidator "github.com/go-playground/validator/v10"

	"github.com/wahrwelt-kit/go-httpkit/httperr"
)

// MaxRequestBodySize is the default body size limit (1 MiB) for DecodeAndValidate, DecodeAndValidateE, and DecodeJSON
const MaxRequestBodySize = 1 << 20

// ErrRequestBodyTooLarge is returned by DecodeJSON when the request body exceeds the configured limit
var ErrRequestBodyTooLarge = errors.New("request body too large")

type decodeConfig struct {
	maxBodySize int64
}

// DecodeOption configures decode behaviour (e.g. body size limit)
type DecodeOption func(*decodeConfig)

// WithMaxBodySize sets the request body size limit for decode. Values <= 0 use MaxRequestBodySize
func WithMaxBodySize(n int64) DecodeOption {
	return func(c *decodeConfig) { c.maxBodySize = n }
}

func applyDecodeOptions(opts []DecodeOption) decodeConfig {
	cfg := decodeConfig{maxBodySize: MaxRequestBodySize}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.maxBodySize <= 0 {
		cfg.maxBodySize = MaxRequestBodySize
	}
	return cfg
}

// Validator validates a value (e.g. go-playground/validator). Used by DecodeAndValidate and DecodeAndValidateE
type Validator interface {
	Validate(any) error
}

func rejectTrailingJSON(limited io.Reader, dec *json.Decoder) bool {
	buf := make([]byte, 1)
	n, _ := dec.Buffered().Read(buf)
	if n > 0 {
		return true
	}
	_, err := limited.Read(buf)
	return err != io.EOF
}

type limitTrackingReader struct {
	r        io.Reader
	limit    int64
	n        int64
	hitLimit *bool
}

func (l *limitTrackingReader) Read(p []byte) (int, error) {
	if *l.hitLimit {
		return 0, io.EOF
	}
	remaining := l.limit - l.n
	if remaining <= 0 {
		*l.hitLimit = true
		return 0, io.EOF
	}
	if int64(len(p)) > remaining {
		p = p[:remaining]
	}
	n, err := l.r.Read(p)
	l.n += int64(n)
	if l.n >= l.limit {
		*l.hitLimit = true
	}
	return n, err
}

func sanitizeValidationField(field string) string {
	if i := strings.LastIndex(field, "."); i >= 0 && i+1 < len(field) {
		field = field[i+1:]
	}
	return field
}

func sanitizeValidationMessage(e playvalidator.FieldError) string {
	switch e.Tag() {
	case "required", "not_empty":
		return "Required"
	case "email", "custom_email":
		return "Invalid format"
	default:
		return "Invalid value"
	}
}

func validationErrorsToItems(valErr playvalidator.ValidationErrors) []ValidationErrorItem {
	items := make([]ValidationErrorItem, len(valErr))
	for i, e := range valErr {
		items[i] = ValidationErrorItem{
			Field:   sanitizeValidationField(e.Field()),
			Message: sanitizeValidationMessage(e),
		}
	}
	return items
}

// DecodeAndValidate reads JSON from the request body (limit from WithMaxBodySize or MaxRequestBodySize, no unknown fields, no trailing data),
// then validates with v. On error it writes the appropriate JSON response and returns (zero, false)
func DecodeAndValidate[T any](w http.ResponseWriter, r *http.Request, v Validator, opts ...DecodeOption) (T, bool) {
	var req T
	cfg := applyDecodeOptions(opts)
	if w == nil || r == nil {
		if w != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(ErrorResponse{Code: "BAD_REQUEST", Message: "request or response writer is nil"}) //nolint:errchkjson // best-effort error response when request is nil
		}
		return req, false
	}
	if r.Body == nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrorResponse{Code: "BAD_REQUEST", Message: "request body is nil"})
		return req, false
	}
	hitLimit := false
	limited := &limitTrackingReader{r: r.Body, limit: cfg.maxBodySize + 1, hitLimit: &hitLimit}
	dec := json.NewDecoder(limited)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrorResponse{Code: "INVALID_JSON", Message: "invalid JSON format"})
		return req, false
	}
	if hitLimit {
		render.Status(r, http.StatusRequestEntityTooLarge)
		render.JSON(w, r, ErrorResponse{Code: "REQUEST_ENTITY_TOO_LARGE", Message: "request body too large"})
		return req, false
	}
	buf := make([]byte, 1)
	if n, _ := limited.Read(buf); n > 0 || rejectTrailingJSON(limited, dec) {
		_, _ = io.Copy(io.Discard, limited)
		if hitLimit {
			render.Status(r, http.StatusRequestEntityTooLarge)
			render.JSON(w, r, ErrorResponse{Code: "REQUEST_ENTITY_TOO_LARGE", Message: "request body too large"})
			return req, false
		}
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrorResponse{Code: "INVALID_JSON", Message: "trailing data after JSON"})
		return req, false
	}
	if v == nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, ErrorResponse{Code: "INTERNAL_ERROR", Message: msgInternalServerError})
		return req, false
	}
	if err := v.Validate(req); err != nil {
		render.Status(r, http.StatusBadRequest)
		var valErr playvalidator.ValidationErrors
		if errors.As(err, &valErr) {
			items := validationErrorsToItems(valErr)
			render.JSON(w, r, ValidationErrorResponse{Code: "VALIDATION_ERROR", Message: "Validation failed", Errors: items})
		} else {
			render.JSON(w, r, ErrorResponse{Code: "VALIDATION_ERROR", Message: "Validation failed"})
		}
		return req, false
	}

	return req, true
}

// DecodeAndValidateE reads and validates JSON from the request body and returns an error without writing a response
// Returns *httperr.HTTPError for invalid JSON, trailing data, body too large, or validation failure
func DecodeAndValidateE[T any](r *http.Request, v Validator, opts ...DecodeOption) (T, error) {
	var req T
	cfg := applyDecodeOptions(opts)
	if r == nil || r.Body == nil {
		return req, httperr.New(errors.New("request or body is nil"), http.StatusBadRequest, "BAD_REQUEST")
	}
	hitLimit := false
	limited := &limitTrackingReader{r: r.Body, limit: cfg.maxBodySize + 1, hitLimit: &hitLimit}
	dec := json.NewDecoder(limited)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return req, httperr.New(errors.New("invalid JSON in request body"), http.StatusBadRequest, "INVALID_JSON")
	}
	if hitLimit {
		return req, httperr.New(errors.New("request body too large"), http.StatusRequestEntityTooLarge, "REQUEST_ENTITY_TOO_LARGE")
	}
	buf := make([]byte, 1)
	if n, _ := limited.Read(buf); n > 0 || rejectTrailingJSON(limited, dec) {
		_, _ = io.Copy(io.Discard, limited)
		if hitLimit {
			return req, httperr.New(errors.New("request body too large"), http.StatusRequestEntityTooLarge, "REQUEST_ENTITY_TOO_LARGE")
		}
		return req, httperr.New(errors.New("trailing data after JSON"), http.StatusBadRequest, "INVALID_JSON")
	}
	if v == nil {
		return req, httperr.New(errors.New("validator is nil"), http.StatusInternalServerError, "INTERNAL_ERROR")
	}
	if err := v.Validate(req); err != nil {
		var valErr playvalidator.ValidationErrors
		if errors.As(err, &valErr) {
			items := validationErrorsToItems(valErr)
			return req, &ValidationHTTPError{
				HTTPError: httperr.New(err, http.StatusBadRequest, "VALIDATION_ERROR"),
				Errors:    items,
			}
		}
		return req, httperr.New(errors.New("validation failed"), http.StatusBadRequest, "VALIDATION_ERROR")
	}
	return req, nil
}

// DecodeJSON decodes JSON from the request body (limit from WithMaxBodySize or MaxRequestBodySize, no unknown fields, no trailing data) into v
func DecodeJSON[T any](r *http.Request, v *T, opts ...DecodeOption) error {
	cfg := applyDecodeOptions(opts)
	if r == nil || r.Body == nil {
		return errors.New("request or body is nil")
	}
	if v == nil {
		return errors.New("decode target is nil")
	}
	hitLimit := false
	limited := &limitTrackingReader{r: r.Body, limit: cfg.maxBodySize + 1, hitLimit: &hitLimit}
	dec := json.NewDecoder(limited)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if hitLimit {
		return ErrRequestBodyTooLarge
	}
	buf := make([]byte, 1)
	if n, _ := limited.Read(buf); n > 0 || rejectTrailingJSON(limited, dec) {
		_, _ = io.Copy(io.Discard, limited)
		if hitLimit {
			return ErrRequestBodyTooLarge
		}
		return errors.New("trailing data after JSON")
	}
	return nil
}
