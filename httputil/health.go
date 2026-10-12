package httputil

import (
	"context"
	"maps"
	"net/http"
	"sync"
	"time"
)

const (
	defaultHealthCheckTimeout = 5 * time.Second
	healthStatusError         = "error"
)

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

type healthResult struct {
	name   string
	status string
}

type healthRunner struct {
	checkers map[string]*healthCheckerState
}

type healthCheckerState struct {
	checker Checker
	mu      sync.Mutex
	current *healthRun
}

type healthRun struct {
	done   chan struct{}
	result healthResult
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
	o := newHealthOpts(opts...)
	runner := newHealthRunner(checkers)
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), o.timeout)
		defer cancel()
		results := runner.run(ctx)
		status, code := healthStatus(results)
		body := map[string]any{"status": status}
		if !o.hideDetails {
			body["checks"] = results
		}
		enc, encErr := marshalJSON(body)
		if encErr != nil {
			if o.onEncodeError != nil {
				o.onEncodeError(encErr)
			}
			code = http.StatusInternalServerError
			enc = []byte(`{"status":"error"}`)
		}
		w.Header().Set("Content-Type", mimeApplicationJSON)
		w.Header().Set("Cache-Control", "no-cache, no-store")
		w.WriteHeader(code)
		_, _ = w.Write(enc)
	}
}

func newHealthOpts(opts ...HealthHandlerOption) healthOpts {
	var o healthOpts
	for _, opt := range opts {
		opt(&o)
	}
	if o.timeout <= 0 {
		o.timeout = defaultHealthCheckTimeout
	}
	return o
}

func newHealthRunner(checkers map[string]Checker) *healthRunner {
	runner := &healthRunner{checkers: make(map[string]*healthCheckerState, len(checkers))}
	for name, c := range checkers {
		runner.checkers[name] = &healthCheckerState{checker: c}
	}
	return runner
}

func (r *healthRunner) run(ctx context.Context) map[string]string {
	results := make(map[string]string, len(r.checkers))
	pending := make(map[string]struct{}, len(r.checkers))
	resultCh := make(chan healthResult, len(r.checkers))
	for name, state := range r.checkers {
		if state.checker == nil {
			results[name] = healthStatusError
			continue
		}
		run := state.start(ctx, name)
		pending[name] = struct{}{}
		go waitHealthRun(ctx, run, resultCh)
	}
	if len(pending) == 0 {
		return results
	}
	maps.Copy(results, collectHealthResults(ctx, pending, resultCh))
	return results
}

func (s *healthCheckerState) start(ctx context.Context, name string) *healthRun {
	s.mu.Lock()
	if s.current != nil {
		run := s.current
		s.mu.Unlock()
		return run
	}
	run := &healthRun{done: make(chan struct{})}
	s.current = run
	checker := s.checker
	s.mu.Unlock()

	go s.run(ctx, name, checker, run)
	return run
}

func (s *healthCheckerState) run(ctx context.Context, name string, checker Checker, run *healthRun) {
	status := "ok"
	defer func() {
		if p := recover(); p != nil {
			status = healthStatusError
		}
		run.result = healthResult{name: name, status: status}
		close(run.done)
		s.mu.Lock()
		if s.current == run {
			s.current = nil
		}
		s.mu.Unlock()
	}()
	err := checker.Check(ctx)
	if err != nil {
		status = healthStatusError
	}
}

func waitHealthRun(ctx context.Context, run *healthRun, resultCh chan<- healthResult) {
	select {
	case <-run.done:
		resultCh <- run.result
	case <-ctx.Done():
		return
	}
}

func collectHealthResults(ctx context.Context, pending map[string]struct{}, resultCh <-chan healthResult) map[string]string {
	results := make(map[string]string, len(pending))
	for len(pending) > 0 {
		select {
		case result := <-resultCh:
			if _, ok := pending[result.name]; ok {
				results[result.name] = result.status
				delete(pending, result.name)
			}
		case <-ctx.Done():
			for name := range pending {
				results[name] = healthStatusError
			}
			pending = nil
		}
	}
	return results
}

func healthStatus(results map[string]string) (status string, code int) {
	for _, v := range results {
		if v == healthStatusError {
			return "degraded", http.StatusServiceUnavailable
		}
	}
	return "ok", http.StatusOK
}
