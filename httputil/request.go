package httputil

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"strings"

	playvalidator "github.com/go-playground/validator/v10"

	"github.com/wahrwelt-kit/go-httpkit/httperr"
)

// MaxRequestBodySize is the default body size limit (1 MiB) for DecodeAndValidate, DecodeAndValidateE, and DecodeJSON
const MaxRequestBodySize = 1 << 20

// ErrRequestBodyTooLarge is returned by DecodeJSON when the request body exceeds the configured limit
var ErrRequestBodyTooLarge = errors.New(msgBodyTooLarge)

type decodeConfig struct {
	maxBodySize int64
}

type decodeErrorKind int

const (
	decodeErrorBadRequest decodeErrorKind = iota + 1
	decodeErrorInvalidJSON
	decodeErrorBodyTooLarge
	decodeErrorTrailingData
)

type decodeError struct {
	kind decodeErrorKind
	err  error
}

func (e *decodeError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
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

func hasTrailingJSONData(limited io.Reader, dec *jsontext.Decoder) (bool, error) {
	r := io.MultiReader(bytes.NewReader(dec.UnreadBuffer()), limited)
	buf := make([]byte, 512)
	for {
		n, err := r.Read(buf)
		for _, b := range buf[:n] {
			if !isJSONWhitespace(b) {
				return true, nil
			}
		}
		if err != nil {
			if err == io.EOF {
				return false, nil
			}
			return false, err
		}
	}
}

func isJSONWhitespace(b byte) bool {
	switch b {
	case ' ', '\t', '\r', '\n':
		return true
	default:
		return false
	}
}

func decodeJSONBody[T any](r *http.Request, opts []DecodeOption) (T, *decodeError) {
	var req T
	cfg := applyDecodeOptions(opts)
	if r == nil || r.Body == nil {
		return req, &decodeError{kind: decodeErrorBadRequest, err: errors.New("request or body is nil")}
	}
	hitLimit := false
	limited := &limitTrackingReader{r: r.Body, limit: cfg.maxBodySize + 1, hitLimit: &hitLimit}
	dec := jsontext.NewDecoder(limited)
	err := json.UnmarshalDecode(dec, &req,
		json.RejectUnknownMembers(true),
		json.MatchCaseInsensitiveNames(true),
		json.WithUnmarshalers(durationJSONUnmarshalers),
	)
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return req, &decodeError{kind: decodeErrorBodyTooLarge, err: ErrRequestBodyTooLarge}
		}
		return req, &decodeError{kind: decodeErrorInvalidJSON, err: err}
	}
	if hitLimit {
		return req, &decodeError{kind: decodeErrorBodyTooLarge, err: ErrRequestBodyTooLarge}
	}
	hasTrailing, err := hasTrailingJSONData(limited, dec)
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return req, &decodeError{kind: decodeErrorBodyTooLarge, err: ErrRequestBodyTooLarge}
		}
		return req, &decodeError{kind: decodeErrorInvalidJSON, err: err}
	}
	if hasTrailing {
		_, _ = io.Copy(io.Discard, limited)
		if hitLimit {
			return req, &decodeError{kind: decodeErrorBodyTooLarge, err: ErrRequestBodyTooLarge}
		}
		return req, &decodeError{kind: decodeErrorTrailingData, err: errors.New("trailing data after JSON")}
	}
	if hitLimit {
		return req, &decodeError{kind: decodeErrorBodyTooLarge, err: ErrRequestBodyTooLarge}
	}
	return req, nil
}

func (e *decodeError) httpError() *httperr.HTTPError {
	switch e.kind {
	case decodeErrorBadRequest:
		return httperr.New(errors.New("request or body is nil"), http.StatusBadRequest, httperr.CodeBadRequest)
	case decodeErrorBodyTooLarge:
		return httperr.New(errors.New(msgBodyTooLarge), http.StatusRequestEntityTooLarge, httperr.CodeRequestEntityTooLarge)
	case decodeErrorInvalidJSON:
		return httperr.New(errors.New("invalid JSON in request body"), http.StatusBadRequest, httperr.CodeInvalidJSON)
	case decodeErrorTrailingData:
		return httperr.New(errors.New("trailing data after JSON"), http.StatusBadRequest, httperr.CodeInvalidJSON)
	}
	return httperr.New(errors.New("request or body is nil"), http.StatusBadRequest, httperr.CodeBadRequest)
}

func (e *decodeError) writeResponse(w http.ResponseWriter) {
	status, body := e.response()
	writeJSON(w, status, body)
}

func (e *decodeError) response() (int, ErrorResponse) {
	switch e.kind {
	case decodeErrorBadRequest:
		return http.StatusBadRequest, ErrorResponse{Code: httperr.CodeBadRequest, Message: "request body is nil"}
	case decodeErrorBodyTooLarge:
		return http.StatusRequestEntityTooLarge, ErrorResponse{Code: httperr.CodeRequestEntityTooLarge, Message: msgBodyTooLarge}
	case decodeErrorInvalidJSON:
		return http.StatusBadRequest, ErrorResponse{Code: httperr.CodeInvalidJSON, Message: "invalid JSON format"}
	case decodeErrorTrailingData:
		return http.StatusBadRequest, ErrorResponse{Code: httperr.CodeInvalidJSON, Message: "trailing data after JSON"}
	default:
		return http.StatusBadRequest, ErrorResponse{Code: httperr.CodeBadRequest, Message: "invalid request body"}
	}
}

func (e *decodeError) jsonError() error {
	switch e.kind {
	case decodeErrorBodyTooLarge:
		return ErrRequestBodyTooLarge
	case decodeErrorBadRequest:
		return errors.New("request or body is nil")
	case decodeErrorInvalidJSON, decodeErrorTrailingData:
		return e.err
	}
	return e.err
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

// Validator tag values recognised by sanitizeValidationMessage
const (
	tagRequired    = "required"
	tagNotEmpty    = "not_empty"
	tagEmail       = "email"
	tagCustomEmail = "custom_email"
)

func sanitizeValidationMessage(e playvalidator.FieldError) string {
	switch e.Tag() {
	case tagRequired, tagNotEmpty:
		return "Required"
	case tagEmail, tagCustomEmail:
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

// DecodeAndValidate reads JSON from the request body (limit from WithMaxBodySize or MaxRequestBodySize, no unknown fields, no trailing non-whitespace data),
// then validates with v. On error it writes the appropriate JSON response and returns (zero, false)
func DecodeAndValidate[T any](w http.ResponseWriter, r *http.Request, v Validator, opts ...DecodeOption) (T, bool) {
	var req T
	if w == nil || r == nil {
		if w != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Code: httperr.CodeBadRequest, Message: "request or response writer is nil"})
		}
		return req, false
	}
	req, decErr := decodeJSONBody[T](r, opts)
	if decErr != nil {
		decErr.writeResponse(w)
		return req, false
	}
	if v == nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Code: httperr.CodeInternalError, Message: msgInternalServerError})
		return req, false
	}
	if err := v.Validate(req); err != nil {
		if valErr, ok := errors.AsType[playvalidator.ValidationErrors](err); ok {
			items := validationErrorsToItems(valErr)
			writeJSON(w, http.StatusBadRequest, ValidationErrorResponse{Code: httperr.CodeValidationError, Message: msgValidationFailed, Errors: items})
		} else {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Code: httperr.CodeValidationError, Message: msgValidationFailed})
		}
		return req, false
	}

	return req, true
}

// DecodeAndValidateE reads and validates JSON from the request body and returns an error without writing a response
// Returns *httperr.HTTPError for invalid JSON, trailing non-whitespace data, body too large, or validation failure
func DecodeAndValidateE[T any](r *http.Request, v Validator, opts ...DecodeOption) (T, error) {
	req, decErr := decodeJSONBody[T](r, opts)
	if decErr != nil {
		return req, decErr.httpError()
	}
	if v == nil {
		return req, httperr.New(errors.New("validator is nil"), http.StatusInternalServerError, httperr.CodeInternalError)
	}
	if err := v.Validate(req); err != nil {
		if valErr, ok := errors.AsType[playvalidator.ValidationErrors](err); ok {
			items := validationErrorsToItems(valErr)
			return req, &ValidationHTTPError{
				HTTPError: httperr.New(err, http.StatusBadRequest, httperr.CodeValidationError),
				Errors:    items,
			}
		}
		return req, httperr.New(errors.New("validation failed"), http.StatusBadRequest, httperr.CodeValidationError)
	}
	return req, nil
}

// DecodeJSON decodes JSON from the request body (limit from WithMaxBodySize or MaxRequestBodySize, no unknown fields, no trailing non-whitespace data) into v
func DecodeJSON[T any](r *http.Request, v *T, opts ...DecodeOption) error {
	if v == nil {
		return errors.New("decode target is nil")
	}
	req, decErr := decodeJSONBody[T](r, opts)
	if decErr != nil {
		return decErr.jsonError()
	}
	*v = req
	return nil
}
