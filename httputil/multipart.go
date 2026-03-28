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
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			HandleError(w, r, httperr.New(errors.New("request body too large"), http.StatusRequestEntityTooLarge, "REQUEST_ENTITY_TOO_LARGE"))
			return false
		}
		HandleError(w, r, httperr.New(errors.New("invalid multipart form"), http.StatusBadRequest, "VALIDATION_ERROR"))
		return false
	}
	return true
}
