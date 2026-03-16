package httputil

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

const healthCheckTimeout = 5 * time.Second

// Checker performs a single health check. Check returns nil for success, non-nil for failure.
type Checker interface {
	Check(ctx context.Context) error
}

// HealthHandlerOption configures HealthHandler (e.g. to hide check details from the response).
type HealthHandlerOption func(*healthOpts)

// healthOpts configures HealthHandler.
type healthOpts struct {
	hideDetails   bool
	onEncodeError func(error)
}

// HealthOnEncodeError sets a callback invoked when JSON encoding of the response body fails (e.g. for logging).
func HealthOnEncodeError(f func(error)) HealthHandlerOption {
	return func(o *healthOpts) { o.onEncodeError = f }
}

// HealthHideDetails omits the "checks" map from the JSON response so only "status" is returned (e.g. for public /health to avoid exposing internal component names).
func HealthHideDetails() HealthHandlerOption {
	return func(o *healthOpts) { o.hideDetails = true }
}

// HealthHandler returns a handler that runs all checkers in parallel with a 5s timeout.
// Response JSON: {"status":"ok"|"degraded"}[,"checks":{name:"ok"|"error"}]. Status 200 when all pass, 503 when any fail.
// Nil checkers are treated as ok. Use HealthHideDetails() to return only status and avoid exposing checker names.
func HealthHandler(checkers map[string]Checker, opts ...HealthHandlerOption) http.HandlerFunc {
	var o healthOpts
	for _, opt := range opts {
		opt(&o)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), healthCheckTimeout)
		defer cancel()
		results := make(map[string]string, len(checkers))
		var mu sync.Mutex
		g, gCtx := errgroup.WithContext(ctx)
		for name, c := range checkers {
			g.Go(func() error {
				defer func() {
					if p := recover(); p != nil {
						mu.Lock()
						results[name] = "error"
						mu.Unlock()
					}
				}()
				if c == nil {
					mu.Lock()
					results[name] = "ok"
					mu.Unlock()
					return nil
				}
				err := c.Check(gCtx)
				mu.Lock()
				if err != nil {
					results[name] = "error"
				} else {
					results[name] = "ok"
				}
				mu.Unlock()
				return nil
			})
		}
		_ = g.Wait()
		allOk := true
		for _, v := range results {
			if v == "error" {
				allOk = false
				break
			}
		}
		status := "ok"
		code := http.StatusOK
		if !allOk {
			status = "degraded"
			code = http.StatusServiceUnavailable
		}
		body := map[string]any{"status": status}
		if !o.hideDetails {
			body["checks"] = results
		}
		enc, encErr := json.Marshal(body)
		if encErr != nil {
			if o.onEncodeError != nil {
				o.onEncodeError(encErr)
			}
			code = http.StatusInternalServerError
			enc = []byte(`{"status":"error"}`)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache, no-store")
		w.WriteHeader(code)
		_, _ = w.Write(enc)
	}
}
