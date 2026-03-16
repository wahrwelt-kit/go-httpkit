package httputil

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// ChiPathFromRequest returns the chi route pattern (e.g. "/users/{id}") from the request context, or "" if not set.
func ChiPathFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	rctx := chi.RouteContext(r.Context())
	if rctx == nil {
		return ""
	}
	return rctx.RoutePattern()
}
