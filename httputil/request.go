package httputil

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	playvalidator "github.com/go-playground/validator/v10"

	"github.com/TakuyaYagam1/go-httpkit/httperr"
)

// MaxRequestBodySize is the default body size limit (1 MiB) for DecodeAndValidate, DecodeAndValidateE, and DecodeJSON.
const MaxRequestBodySize = 1 << 20

// Validator validates a value (e.g. go-playground/validator). Used by DecodeAndValidate and DecodeAndValidateE.
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
	case "min", "max", "len", "gte", "lte", "gt", "lt":
		return "Invalid value"
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

// DecodeAndValidate reads JSON from the request body (limit MaxRequestBodySize, no unknown fields, no trailing data),
// then validates with v. On error it writes the appropriate JSON response and returns (zero, false).
func DecodeAndValidate[T any](w http.ResponseWriter, r *http.Request, v Validator) (T, bool) {
	var req T
	if w == nil || r == nil {
		if w != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(ErrorResponse{Code: "BAD_REQUEST", Message: "request or response writer is nil"})
		}
		return req, false
	}
	if r.Body == nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrorResponse{Code: "BAD_REQUEST", Message: "request body is nil"})
		return req, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodySize)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrorResponse{Code: "INVALID_JSON", Message: "invalid JSON format"})
		return req, false
	}
	if rejectTrailingJSON(r.Body, dec) {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrorResponse{Code: "INVALID_JSON", Message: "invalid JSON format"})
		return req, false
	}
	if v == nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, ErrorResponse{Code: "INTERNAL_ERROR", Message: "validation not configured"})
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

// DecodeAndValidateE reads and validates JSON from the request body and returns an error without writing a response.
// Returns *httperr.HTTPError for invalid JSON, trailing data, or validation failure.
func DecodeAndValidateE[T any](r *http.Request, v Validator) (T, error) {
	var req T
	if r == nil || r.Body == nil {
		return req, httperr.New(errors.New("request or body is nil"), http.StatusBadRequest, "BAD_REQUEST")
	}
	limited := io.LimitReader(r.Body, MaxRequestBodySize)
	dec := json.NewDecoder(limited)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return req, &httperr.HTTPError{
			Err:        errors.New("invalid JSON in request body"),
			StatusCode: http.StatusBadRequest,
			Code:       "INVALID_JSON",
			IsExpected: true,
		}
	}
	if rejectTrailingJSON(limited, dec) {
		return req, &httperr.HTTPError{
			Err:        errors.New("invalid JSON in request body"),
			StatusCode: http.StatusBadRequest,
			Code:       "INVALID_JSON",
			IsExpected: true,
		}
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
		return req, &httperr.HTTPError{
			Err:        errors.New("validation failed"),
			StatusCode: http.StatusBadRequest,
			Code:       "VALIDATION_ERROR",
			IsExpected: true,
		}
	}
	return req, nil
}

// DecodeJSON decodes JSON from the request body (limit MaxRequestBodySize, no unknown fields, no trailing data) into v.
func DecodeJSON[T any](r *http.Request, v *T) error {
	if r == nil || r.Body == nil {
		return errors.New("request or body is nil")
	}
	if v == nil {
		return errors.New("decode target is nil")
	}
	limited := io.LimitReader(r.Body, MaxRequestBodySize)
	dec := json.NewDecoder(limited)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if rejectTrailingJSON(limited, dec) {
		return errors.New("trailing data after JSON")
	}
	return nil
}
