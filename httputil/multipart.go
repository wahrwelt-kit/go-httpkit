package httputil

import (
	"errors"
	"net/http"

	"github.com/TakuyaYagam1/go-httpkit/httperr"
)

// ParseMultipartFormLimit parses the multipart form with the given body size and memory limits. On error writes an error response and returns false.
func ParseMultipartFormLimit(w http.ResponseWriter, r *http.Request, maxBodySize, maxMemory int64) bool {
	if r.Body == nil {
		r.Body = http.NoBody
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		HandleError(w, r, httperr.New(errors.New("invalid multipart form"), http.StatusBadRequest, "VALIDATION_ERROR"))
		return false
	}
	return true
}
