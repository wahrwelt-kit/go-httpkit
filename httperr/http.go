package httperr

import (
	"errors"
	"net/http"
)

// CodeFromStatus returns the default application error code for the given HTTP status
// (e.g. 404 ->  CodeNotFound, 500 ->  CodeInternalError)
func CodeFromStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return CodeBadRequest
	case http.StatusUnauthorized:
		return CodeUnauthorized
	case http.StatusForbidden:
		return CodeForbidden
	case http.StatusNotFound:
		return CodeNotFound
	case http.StatusConflict:
		return CodeConflict
	case http.StatusGone:
		return CodeGone
	case http.StatusPaymentRequired:
		return CodePaymentRequired
	case http.StatusUnprocessableEntity:
		return CodeValidationError
	case http.StatusTooManyRequests:
		return CodeRateLimitExceeded
	case http.StatusServiceUnavailable:
		return CodeServiceUnavailable
	default:
		return CodeInternalError
	}
}

// HTTPError is an error that carries HTTP status and application error code for JSON responses
type HTTPError struct {
	err           error
	statusCode    int
	code          string
	isClientError bool
}

// Error implements error and returns the wrapped error's message
func (e *HTTPError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

// Unwrap returns the wrapped error for errors.Is/As
func (e *HTTPError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// HTTPStatus returns the HTTP status code to send in the response
func (e *HTTPError) HTTPStatus() int {
	if e == nil {
		return 0
	}
	return e.statusCode
}

// GetCode returns the application error code for the JSON body
func (e *HTTPError) GetCode() string {
	if e == nil {
		return ""
	}
	return e.code
}

// IsClientError reports whether this error represents a client error (4xx)
func (e *HTTPError) IsClientError() bool {
	if e == nil {
		return false
	}
	return e.isClientError
}

// New builds an HTTPError with the given underlying error, HTTP status, and application code
// IsClientError is set to true when status is 4xx
func New(err error, status int, code string) *HTTPError {
	if err == nil {
		err = errors.New(code)
	}
	return &HTTPError{
		err:           err,
		statusCode:    status,
		code:          code,
		isClientError: status >= http.StatusBadRequest && status < 500,
	}
}

// IsExpectedClientError reports whether err is an HTTPError with status 4xx (client error)
func IsExpectedClientError(err error) bool {
	if he, ok := errors.AsType[*HTTPError](err); ok {
		return he.isClientError
	}
	return false
}
