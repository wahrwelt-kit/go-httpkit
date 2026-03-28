package httputil

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

const defaultHealthCheckTimeout = 5 * time.Second

// Checker performs a single health check. Implementations should respect ctx cancellation
type Checker interface {
	Check(ctx context.Context) error
}

// HealthHandlerOption configures HealthHandler behaviour
type HealthHandlerOption func(*healthOpts)

type healthOpts struct {
	hideDetails   bool
	onEncodeError func(error)
	timeout       time.Duration
}

// HealthOnEncodeError sets a callback invoked when JSON encoding of the health response fails (e.g. for logging)
func HealthOnEncodeError(f func(error)) HealthHandlerOption {
	return func(o *healthOpts) { o.onEncodeError = f }
}

// HealthHideDetails omits per-check results from the JSON response; only status (ok/degraded) is returned
func HealthHideDetails() HealthHandlerOption {
	return func(o *healthOpts) { o.hideDetails = true }
}

// HealthTimeout sets the deadline for all checkers to complete. Defaults to 5 seconds when not set or <= 0
// Checkers receive a context cancelled when the timeout expires; implementations should respect ctx.Done()
func HealthTimeout(d time.Duration) HealthHandlerOption {
	return func(o *healthOpts) { o.timeout = d }
}

// HealthHandler returns an HTTP handler that runs all checkers in parallel with a configurable timeout (default 5s)
// and returns JSON with status ("ok" or "degraded") and optional per-check results
// Responds 200 when all checks pass, 503 when any check returns an error or panics
func HealthHandler(checkers map[string]Checker, opts ...HealthHandlerOption) http.HandlerFunc {
	var o healthOpts
	for _, opt := range opts {
		opt(&o)
	}
	if o.timeout <= 0 {
		o.timeout = defaultHealthCheckTimeout
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), o.timeout)
		defer cancel()
		results := make(map[string]string, len(checkers))
		var mu sync.Mutex
		var wg sync.WaitGroup
		for name, c := range checkers {
			wg.Go(func() {
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
					return
				}
				err := c.Check(ctx)
				mu.Lock()
				if err != nil {
					results[name] = "error"
				} else {
					results[name] = "ok"
				}
				mu.Unlock()
			})
		}
		wg.Wait()
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
