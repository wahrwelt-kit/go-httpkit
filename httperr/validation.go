package httperr

import (
	"fmt"
	"net/http"
)

// NewValidationErrorf creates an HTTPError with status 400 and code VALIDATION_ERROR for dynamic validation messages
// For semantic "request body valid JSON but business validation failed" use ErrUnprocessableEntity (422) from sentinels
func NewValidationErrorf(format string, args ...any) *HTTPError {
	return New(fmt.Errorf(format, args...), http.StatusBadRequest, CodeValidationError)
}
