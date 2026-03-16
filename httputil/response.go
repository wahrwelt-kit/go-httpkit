package httputil

import (
	"mime"
	"net/http"
	"strings"

	"github.com/go-chi/render"

	"github.com/TakuyaYagam1/go-httpkit/httperr"
)

// RenderJSON writes data as JSON with the given HTTP status.
func RenderJSON[T any](w http.ResponseWriter, r *http.Request, status int, data T) {
	render.Status(r, status)
	render.JSON(w, r, data)
}

// RenderNoContent sends 204 No Content with no body.
func RenderNoContent(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// RenderCreated sends 201 Created with data as JSON body.
func RenderCreated[T any](w http.ResponseWriter, r *http.Request, data T) {
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, data)
}

// RenderAccepted sends 202 Accepted with data as JSON body.
func RenderAccepted[T any](w http.ResponseWriter, r *http.Request, data T) {
	render.Status(r, http.StatusAccepted)
	render.JSON(w, r, data)
}

// RenderOK sends 200 OK with data as JSON body.
func RenderOK[T any](w http.ResponseWriter, r *http.Request, data T) {
	render.Status(r, http.StatusOK)
	render.JSON(w, r, data)
}

// RenderError sends JSON error with status and message; code is derived from status.
// For 5xx status, message is replaced with "Internal server error" to avoid leaking details.
func RenderError(w http.ResponseWriter, r *http.Request, status int, message string) {
	render.Status(r, status)
	if status >= http.StatusInternalServerError {
		message = "Internal server error"
	}
	render.JSON(w, r, ErrorResponse{Code: httperr.CodeFromStatus(status), Message: message})
}

// RenderErrorWithCode sends JSON error with explicit code.
// For 5xx status, message is replaced with "Internal server error" to avoid leaking details.
func RenderErrorWithCode(w http.ResponseWriter, r *http.Request, status int, message, code string) {
	render.Status(r, status)
	if status >= http.StatusInternalServerError {
		message = "Internal server error"
	}
	render.JSON(w, r, ErrorResponse{Code: code, Message: message})
}

// RenderInvalidID sends 400 Bad Request with code INVALID_ID.
func RenderInvalidID(w http.ResponseWriter, r *http.Request) {
	RenderErrorWithCode(w, r, http.StatusBadRequest, "invalid ID", "INVALID_ID")
}

var safeTextContentTypes = map[string]struct{}{
	"text/plain": {}, "application/json": {}, "application/octet-stream": {},
}

// RenderText writes a plain text response with the given status, Content-Type, and body.
// Only text/plain, application/json, and application/octet-stream are allowed; any other Content-Type is forced to "text/plain; charset=utf-8" to avoid reflected XSS when body contains user input.
// Do not pass user-controlled content that could be interpreted as HTML; ensure clients do not render the response as HTML.
func RenderText(w http.ResponseWriter, _ *http.Request, status int, contentType, body string) {
	if mt, _, err := mime.ParseMediaType(contentType); err == nil {
		base := strings.ToLower(strings.TrimSpace(mt))
		if _, ok := safeTextContentTypes[base]; !ok {
			contentType = "text/plain; charset=utf-8"
		}
	} else {
		contentType = "text/plain; charset=utf-8"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}
