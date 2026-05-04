package httputil

import (
	"errors"
	"net/http"

	"github.com/wahrwelt-kit/go-httpkit/httperr"
)

// ParseMultipartFormLimit parses the multipart form with the given body size and memory limits
// On error writes an error response and returns false
// Returns 413 REQUEST_ENTITY_TOO_LARGE when the body exceeds maxBodySize; 400 VALIDATION_ERROR for malformed forms
func ParseMultipartFormLimit(w http.ResponseWriter, r *http.Request, maxBodySize, maxMemory int64) bool {
	if r.Body == nil {
		r.Body = http.NoBody
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	// G120 is bounded by http.MaxBytesReader on the line above; gosec does not see the wrapper
	if err := r.ParseMultipartForm(maxMemory); err != nil { //nolint:gosec // body is bounded by http.MaxBytesReader above
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			HandleError(w, r, httperr.New(errors.New(msgBodyTooLarge), http.StatusRequestEntityTooLarge, httperr.CodeRequestEntityTooLarge))
			return false
		}
		HandleError(w, r, httperr.New(errors.New("invalid multipart form"), http.StatusBadRequest, httperr.CodeValidationError))
		return false
	}
	return true
}
