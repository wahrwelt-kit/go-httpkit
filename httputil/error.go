package httputil

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"

	"github.com/TakuyaYagam1/go-httpkit/httperr"
	logger "github.com/TakuyaYagam1/go-logkit"
)

// ErrorLogEvent is the interface for logging a single error at Info or Error level.
type ErrorLogEvent interface {
	// Info logs the message at info level.
	Info(msg string)
	// Error logs the message at error level.
	Error(msg string)
}

// ErrorLogger creates an ErrorLogEvent bound to an error (e.g. for structured logging).
type ErrorLogger interface {
	// WithError returns an event that will include err in the log entry.
	WithError(err error) ErrorLogEvent
}

// logkitErrorAdapter is an implementation of ErrorLogger that uses logkit.Logger.
type logkitErrorAdapter struct{ l logger.Logger }

func (a *logkitErrorAdapter) WithError(err error) ErrorLogEvent {
	return &logkitErrorEvent{a.l.WithError(err)}
}

// logkitErrorEvent is an implementation of ErrorLogEvent that uses logkit.Logger.
type logkitErrorEvent struct{ l logger.Logger }

func (e *logkitErrorEvent) Info(msg string)  { e.l.Info(msg) }
func (e *logkitErrorEvent) Error(msg string) { e.l.Error(msg) }

// NewErrorLogger returns an ErrorLogger that uses the given logkit Logger. Returns nil if l is nil.
func NewErrorLogger(l logger.Logger) ErrorLogger {
	if l == nil {
		return nil
	}
	return &logkitErrorAdapter{l: l}
}

// ErrorHandler handles errors by optionally logging and writing a JSON error response via HandleError.
type ErrorHandler struct {
	// Logger is used to log the error; 4xx are logged at Info, 5xx at Error. May be nil.
	Logger ErrorLogger
}

// Handle logs err (if Logger is set) and writes a JSON error response. Returns true if err was non-nil and handled.
func (h *ErrorHandler) Handle(w http.ResponseWriter, r *http.Request, err error, msg string) bool {
	if err == nil {
		return false
	}
	if h.Logger != nil {
		ev := h.Logger.WithError(err)
		if httperr.IsExpectedClientError(err) {
			ev.Info(msg)
		} else {
			ev.Error(msg)
		}
	}
	HandleError(w, r, err)
	return true
}

// ErrorResponse is the JSON body for a single error (code and message).
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ValidationErrorItem is one field-level validation error.
type ValidationErrorItem struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrorResponse is the JSON body for validation errors (code, message, and optional per-field errors).
type ValidationErrorResponse struct {
	Code    string                `json:"code"`
	Message string                `json:"message"`
	Errors  []ValidationErrorItem `json:"errors,omitempty"`
}

// ValidationHTTPError is an HTTPError that carries per-field validation errors for JSON responses.
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
func (e *ValidationHTTPError) HTTPStatus() int {
	if e == nil || e.HTTPError == nil {
		return 0
	}
	return e.HTTPError.HTTPStatus()
}
func (e *ValidationHTTPError) GetCode() string {
	if e == nil || e.HTTPError == nil {
		return ""
	}
	return e.HTTPError.GetCode()
}

// HandleError writes a JSON error response. If err implements HTTPStatus and GetCode (e.g. *httperr.HTTPError),
// that status and code are used; otherwise 500 and INTERNAL_ERROR. For 5xx the message is replaced with "Internal server error".
// ValidationHTTPError is rendered as ValidationErrorResponse with per-field errors.
func HandleError(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil || w == nil || r == nil {
		return
	}
	var valErr *ValidationHTTPError
	if errors.As(err, &valErr) {
		if valErr.HTTPError == nil {
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, ErrorResponse{Code: httperr.CodeFromStatus(http.StatusInternalServerError), Message: "Internal server error"})
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
			message = "Internal server error"
		}
		render.Status(r, httpErr.HTTPStatus())
		render.JSON(w, r, ErrorResponse{
			Code:    code,
			Message: message,
		})
		return
	}

	render.Status(r, http.StatusInternalServerError)
	render.JSON(w, r, ErrorResponse{Code: httperr.CodeFromStatus(http.StatusInternalServerError), Message: "Internal server error"})
}
