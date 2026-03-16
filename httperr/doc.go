// Package httperr provides HTTP-aware error types for JSON APIs.
//
// # HTTPError
//
// HTTPError carries an HTTP status code and an application error code (e.g. BAD_REQUEST, NOT_FOUND).
// Use New to build a custom error from an underlying error, status, and code. IsExpected is true for 4xx
// so callers can avoid logging them as server errors. Implementations of error, HTTPStatus, and GetCode
// allow handlers to respond with the correct status and JSON body.
//
// # Status-to-code mapping
//
// CodeFromStatus returns a default application code for a given HTTP status (e.g. 404 -> "NOT_FOUND").
// Use it when rendering errors that do not implement the status/code interface.
//
// # Validation errors
//
// NewValidationErrorf builds an HTTPError with status 400 and code VALIDATION_ERROR for dynamic messages.
// For "valid JSON but business validation failed" use ErrUnprocessableEntity (422) instead.
//
// # Sentinel errors
//
// ErrInvalidID, ErrNotAuthenticated, ErrForbidden, ErrNotFound, ErrConflict, ErrGone, ErrUnprocessableEntity,
// ErrTooManyRequests, and ErrServiceUnavailable return ready-made *HTTPError for common cases.
// IsExpectedClientError reports whether an error is a 4xx client error.
package httperr
