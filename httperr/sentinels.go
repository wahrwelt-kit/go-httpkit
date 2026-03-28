package httperr

import (
	"errors"
	"net/http"
)

// Sentinel error values used as HTTPError.Unwrap() targets by the Err* functions below
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

// ErrInvalidID returns an HTTPError with status 400 and code INVALID_ID
func ErrInvalidID() *HTTPError {
	return New(ErrInvalidIDSentinel, http.StatusBadRequest, "INVALID_ID")
}

// ErrNotAuthenticated returns an HTTPError with status 401 and code NOT_AUTHENTICATED
func ErrNotAuthenticated() *HTTPError {
	return New(ErrNotAuthenticatedSentinel, http.StatusUnauthorized, "NOT_AUTHENTICATED")
}

// ErrForbidden returns an HTTPError with status 403 and code FORBIDDEN
func ErrForbidden() *HTTPError {
	return New(ErrForbiddenSentinel, http.StatusForbidden, "FORBIDDEN")
}

// ErrNotFound returns an HTTPError with status 404 and code NOT_FOUND
func ErrNotFound() *HTTPError {
	return New(ErrNotFoundSentinel, http.StatusNotFound, "NOT_FOUND")
}

// ErrConflict returns an HTTPError with status 409 and code CONFLICT
func ErrConflict() *HTTPError {
	return New(ErrConflictSentinel, http.StatusConflict, "CONFLICT")
}

// ErrGone returns an HTTPError with status 410 and code GONE
func ErrGone() *HTTPError {
	return New(ErrGoneSentinel, http.StatusGone, "GONE")
}

// ErrUnprocessableEntity returns an HTTPError with status 422 and code VALIDATION_ERROR
func ErrUnprocessableEntity() *HTTPError {
	return New(ErrUnprocessableEntitySentinel, http.StatusUnprocessableEntity, "VALIDATION_ERROR")
}

// ErrTooManyRequests returns an HTTPError with status 429 and code RATE_LIMIT_EXCEEDED
func ErrTooManyRequests() *HTTPError {
	return New(ErrTooManyRequestsSentinel, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED")
}

// ErrServiceUnavailable returns an HTTPError with status 503 and code SERVICE_UNAVAILABLE
func ErrServiceUnavailable() *HTTPError {
	return New(ErrServiceUnavailableSentinel, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
}
