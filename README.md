# go-httpkit

[![CI](https://github.com/wahrwelt-kit/go-httpkit/actions/workflows/ci.yml/badge.svg)](https://github.com/wahrwelt-kit/go-httpkit/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/wahrwelt-kit/go-httpkit.svg)](https://pkg.go.dev/github.com/wahrwelt-kit/go-httpkit)
[![Go Report Card](https://goreportcard.com/badge/github.com/wahrwelt-kit/go-httpkit)](https://goreportcard.com/report/github.com/wahrwelt-kit/go-httpkit)

HTTP helpers for JSON APIs: errors, responses, decoding, validation, pagination, query parsing, SSE, health, and middleware.

## Install

Requires Go 1.27.2 or later.

```bash
go get github.com/wahrwelt-kit/go-httpkit
go get github.com/wahrwelt-kit/go-httpkit/metrics github.com/wahrwelt-kit/go-httpkit/localization
```

The root module contains the JSON API helpers and generic middleware. Prometheus and go-i18n integrations are separate modules so projects that do not use them keep a smaller dependency graph.

```go
import (
	"log/slog"

	"github.com/wahrwelt-kit/go-httpkit/httperr"
	"github.com/wahrwelt-kit/go-httpkit/httputil"
	"github.com/wahrwelt-kit/go-httpkit/httputil/middleware"
	"github.com/wahrwelt-kit/go-httpkit/localization"
	"github.com/wahrwelt-kit/go-httpkit/metrics"
)
```

## Subpackages

### httperr

HTTP-aware errors: HTTPError (Err, StatusCode, Code, IsExpected), New, CodeFromStatus, IsExpectedClientError. Sentinels: ErrInvalidID, ErrNotAuthenticated, ErrForbidden, ErrNotFound, ErrConflict, ErrGone, ErrUnprocessableEntity, ErrTooManyRequests, ErrTimeout, ErrServiceUnavailable. NewValidationErrorf for 400 validation errors.

### httputil

- **Responses**: RenderJSON, RenderOK, RenderCreated, RenderAccepted, RenderNoContent, RenderError, RenderErrorWithCode, RenderInvalidID, RenderText
- **Errors**: HandleError, ErrorHandler; ErrorResponse, ValidationErrorResponse
- **Request**: DecodeAndValidate[T], DecodeAndValidateE[T], DecodeJSON[T] with strict unknown-field rejection and trailing non-whitespace rejection
- **Params**: ParseUUID, ParseUUIDField, ParseAuthUserID, GetUserID(ctx)
- **Pagination**: ClampPage, ClampPerPage, ClampLimit, ParseIntQuery, TotalPages, NewPaginationMeta
- **Query**: ParseBoolQuery, ParseEnumQuery[T], ParseSortQuery, ParseTimeQuery
- **Search**: EscapeILIKE, ValidateSearchQ, SanitizeSearchQ
- **IP**: GetClientIPWithNets, GetClientIPE, ParseTrustedProxyCIDRs
- **Multipart**: ParseMultipartFormLimit
- **Download**: RenderJSONAttachment, RenderStream, RenderStreamLimited, RenderBytes
- **SSE**: SSEWriter, NewSSEWriter, NewSSEWriterWithLimit(MaxEventBytes), Send, SendJSON, Close, Heartbeat
- **Health**: Checker, HealthHandler(checkers, HealthTimeout, HealthHideDetails) -> JSON status and checks

JSON request decoding uses Go's `encoding/json/v2`. It matches field names case-insensitively and rejects unknown or duplicate members, invalid UTF-8, trailing data, and bodies over the configured limit. Response helpers keep nil maps and slices as `null`, sort map keys, escape HTML, and retain their existing newline behavior. `time.Duration` values remain nanosecond integers. Since v2 defines `omitempty` by the encoded JSON value, use `omitzero` for zero scalar fields that should be omitted.

### httputil/middleware

- **Logger(log, cidrs, opts...)**: request logging via `*slog.Logger`; WithRedactedParams to add sensitive query params; WithSkipPaths to suppress health/metrics noise
- **Recoverer(log)**: panic recovery, 500 response, stack log via `*slog.Logger`
- **RequireJSON(maxBodyBytes)**: requires JSON Content-Type for JSON body routes and optionally limits request body size
- **ContextTimeout(d)**: attaches a request context deadline; HandleError maps context.DeadlineExceeded to 503 TIMEOUT
- **SecurityHeaders**: X-Content-Type-Options, X-Frame-Options, Referrer-Policy, Permissions-Policy, CSP, COOP, CORP; WithHSTS for HTTPS-only services
- **RequestID**: X-Request-ID from header or new UUID; GetRequestID(ctx)
- **ClientIP(cidrs)**: resolves and stores client IP in context; GetClientIPFromContext

### metrics

Prometheus middleware: request count, duration, and optional in-flight gauge. Pass a stable route-pattern function for the path label; options include logger, namespace, subsystem, buckets, and const labels.

Standard HTTP methods keep their metric labels; extension methods share `unknown`.
The original request method still reaches the handler unchanged.

### localization

go-i18n middleware: resolves language from cookie, query parameter, and Accept-Language; stores `*i18n.Localizer` in request context.

## Example

```go
log := slog.Default()
errHandler := httputil.ErrorHandler{Logger: log}

r.Use(middleware.Recoverer(log))
r.Use(middleware.RequestID())
r.Use(middleware.SecurityHeaders(
	middleware.WithHSTS(63072000*time.Second, true, true),
))
r.Use(middleware.ContextTimeout(3 * time.Second))
r.Use(metrics.Middleware(nil, func(r *http.Request) string {
	return "/items"
}, metrics.WithNamespace("myapp")))
r.Use(localization.Middleware(bundle))

r.With(middleware.RequireJSON(1 << 20)).Post("/items", func(w http.ResponseWriter, r *http.Request) {
	var req createItemRequest
	if !httputil.DecodeAndValidate(w, r, &req, validator) {
		return
	}
	item, err := store.Create(r.Context(), req)
	if errHandler.Handle(w, r, err, "create item") {
		return
	}
	httputil.RenderCreated(w, r, item)
})

r.Get("/items", func(w http.ResponseWriter, r *http.Request) {
	field, dir, ok := httputil.ParseSortQuery(r, []string{"name", "score"})
	if !ok {
		field, dir = "name", "asc"
	}
	items, err := store.List(r.Context(), field, dir)
	if errHandler.Handle(w, r, err, "list items") {
		return
	}
	httputil.RenderOK(w, r, items)
})

r.Get("/health", httputil.HealthHandler(map[string]httputil.Checker{
	"db":    dbPinger,
	"redis": redisPinger,
}, httputil.HealthTimeout(2*time.Second)))
```
