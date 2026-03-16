package httperr

import (
	"errors"
	"net/http"
)

// Sentinel error values used as HTTPError.Err by the Err* functions below.
var (
	ErrInvalidIDSentinel           = errors.New("invalid ID")
	ErrNotAuthenticatedSentinel    = errors.New("not authenticated")
	ErrForbiddenSentinel           = errors.New("forbidden")
	ErrNotFoundSentinel            = errors.New("not found")
	ErrConflictSentinel            = errors.New("conflict")
	ErrGoneSentinel                = errors.New("gone")
	ErrUnprocessableEntitySentinel = errors.New("unprocessable entity")
	ErrTooManyRequestsSentinel     = errors.New("too many requests")
	ErrServiceUnavailableSentinel  = errors.New("service unavailable")
)

// ErrInvalidID returns an HTTPError with status 400 and code INVALID_ID.
func ErrInvalidID() *HTTPError {
	return &HTTPError{Err: ErrInvalidIDSentinel, StatusCode: http.StatusBadRequest, Code: "INVALID_ID", IsExpected: true}
}

// ErrNotAuthenticated returns an HTTPError with status 401 and code NOT_AUTHENTICATED.
func ErrNotAuthenticated() *HTTPError {
	return &HTTPError{Err: ErrNotAuthenticatedSentinel, StatusCode: http.StatusUnauthorized, Code: "NOT_AUTHENTICATED", IsExpected: true}
}

// ErrForbidden returns an HTTPError with status 403 and code FORBIDDEN.
func ErrForbidden() *HTTPError {
	return &HTTPError{Err: ErrForbiddenSentinel, StatusCode: http.StatusForbidden, Code: "FORBIDDEN", IsExpected: true}
}

// ErrNotFound returns an HTTPError with status 404 and code NOT_FOUND.
func ErrNotFound() *HTTPError {
	return &HTTPError{Err: ErrNotFoundSentinel, StatusCode: http.StatusNotFound, Code: "NOT_FOUND", IsExpected: true}
}

// ErrConflict returns an HTTPError with status 409 and code CONFLICT.
func ErrConflict() *HTTPError {
	return &HTTPError{Err: ErrConflictSentinel, StatusCode: http.StatusConflict, Code: "CONFLICT", IsExpected: true}
}

// ErrGone returns an HTTPError with status 410 and code GONE.
func ErrGone() *HTTPError {
	return &HTTPError{Err: ErrGoneSentinel, StatusCode: http.StatusGone, Code: "GONE", IsExpected: true}
}

// ErrUnprocessableEntity returns an HTTPError with status 422 and code VALIDATION_ERROR.
func ErrUnprocessableEntity() *HTTPError {
	return &HTTPError{Err: ErrUnprocessableEntitySentinel, StatusCode: http.StatusUnprocessableEntity, Code: "VALIDATION_ERROR", IsExpected: true}
}

// ErrTooManyRequests returns an HTTPError with status 429 and code RATE_LIMIT_EXCEEDED.
func ErrTooManyRequests() *HTTPError {
	return &HTTPError{Err: ErrTooManyRequestsSentinel, StatusCode: http.StatusTooManyRequests, Code: "RATE_LIMIT_EXCEEDED", IsExpected: true}
}

// ErrServiceUnavailable returns an HTTPError with status 503 and code SERVICE_UNAVAILABLE.
func ErrServiceUnavailable() *HTTPError {
	return &HTTPError{Err: ErrServiceUnavailableSentinel, StatusCode: http.StatusServiceUnavailable, Code: "SERVICE_UNAVAILABLE", IsExpected: false}
}
