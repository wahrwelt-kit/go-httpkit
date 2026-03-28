package httputil

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"

	"github.com/wahrwelt-kit/go-httpkit/httperr"
	logger "github.com/wahrwelt-kit/go-logkit"
)

const msgInternalServerError = "Internal server error"

// ErrorHandler handles errors by optionally logging and writing a JSON error response via HandleError
type ErrorHandler struct {
	// Logger is optional; when set, Handle logs 4xx at Info and 5xx at Error
	Logger logger.Logger
}

// Handle logs err (if Logger is set) and writes a JSON error response. Returns true if err was non-nil and handled
// 4xx errors are logged at Info level, everything else at Error level
func (h *ErrorHandler) Handle(w http.ResponseWriter, r *http.Request, err error, msg string) bool {
	if err == nil {
		return false
	}
	if h.Logger != nil {
		l := h.Logger.WithError(err)
		if httperr.IsExpectedClientError(err) {
			l.Info(msg)
		} else {
			l.Error(msg)
		}
	}
	HandleError(w, r, err)
	return true
}

// ErrorResponse is the JSON body for a single error (code and message)
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ValidationErrorItem is one field-level validation error
type ValidationErrorItem struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrorResponse is the JSON body for validation errors (code, message, and optional per-field errors)
type ValidationErrorResponse struct {
	Code    string                `json:"code"`
	Message string                `json:"message"`
	Errors  []ValidationErrorItem `json:"errors,omitempty"`
}

// ValidationHTTPError is an HTTPError that carries per-field validation errors for JSON responses
type ValidationHTTPError struct {
	*httperr.HTTPError

	Errors []ValidationErrorItem
}

func (e *ValidationHTTPError) Error() string {
	if e == nil || e.HTTPError == nil {
		return ""
	}
	return e.HTTPError.Error()
}

func (e *ValidationHTTPError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.HTTPError
}

// HTTPStatus returns the HTTP status code for the validation error, or 0 if the receiver or HTTPError is nil
func (e *ValidationHTTPError) HTTPStatus() int {
	if e == nil || e.HTTPError == nil {
		return 0
	}
	return e.HTTPError.HTTPStatus()
}

// GetCode returns the error code for the validation error, or "" if the receiver or HTTPError is nil
func (e *ValidationHTTPError) GetCode() string {
	if e == nil || e.HTTPError == nil {
		return ""
	}
	return e.HTTPError.GetCode()
}

// HandleError writes a JSON error response. If err implements HTTPStatus and GetCode (e.g. *httperr.HTTPError),
// that status and code are used; otherwise 500 and INTERNAL_ERROR. For 5xx the message is replaced with msgInternalServerError
// ValidationHTTPError is rendered as ValidationErrorResponse with per-field errors
func HandleError(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil || w == nil || r == nil {
		return
	}
	var valErr *ValidationHTTPError
	if errors.As(err, &valErr) {
		if valErr.HTTPError == nil {
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, ErrorResponse{Code: httperr.CodeFromStatus(http.StatusInternalServerError), Message: msgInternalServerError})
			return
		}
		render.Status(r, valErr.HTTPStatus())
		render.JSON(w, r, ValidationErrorResponse{
			Code:    valErr.GetCode(),
			Message: "Validation failed",
			Errors:  valErr.Errors,
		})
		return
	}
	var httpErr *httperr.HTTPError
	if errors.As(err, &httpErr) {
		code := httpErr.GetCode()
		if code == "" {
			code = httperr.CodeFromStatus(httpErr.HTTPStatus())
		}
		message := httpErr.Error()
		if httpErr.HTTPStatus() >= http.StatusInternalServerError {
			message = msgInternalServerError
		}
		render.Status(r, httpErr.HTTPStatus())
		render.JSON(w, r, ErrorResponse{
			Code:    code,
			Message: message,
		})
		return
	}

	render.Status(r, http.StatusInternalServerError)
	render.JSON(w, r, ErrorResponse{Code: httperr.CodeFromStatus(http.StatusInternalServerError), Message: msgInternalServerError})
}
