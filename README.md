# go-httpkit

[![CI](https://github.com/wahrwelt-kit/go-httpkit/actions/workflows/ci.yml/badge.svg)](https://github.com/wahrwelt-kit/go-httpkit/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/wahrwelt-kit/go-httpkit.svg)](https://pkg.go.dev/github.com/wahrwelt-kit/go-httpkit)
[![Go Report Card](https://goreportcard.com/badge/github.com/wahrwelt-kit/go-httpkit)](https://goreportcard.com/report/github.com/wahrwelt-kit/go-httpkit)

HTTP helpers for JSON APIs: errors, responses, decoding, validation, pagination, query parsing, SSE, health, and middleware.

## Install

```bash
go get github.com/wahrwelt-kit/go-httpkit
```

```go
import "github.com/wahrwelt-kit/go-httpkit/httperr"
import "github.com/wahrwelt-kit/go-httpkit/httputil"
import "github.com/wahrwelt-kit/go-httpkit/httputil/middleware"
```

## Subpackages

### httperr

HTTP-aware errors: HTTPError (Err, StatusCode, Code, IsExpected), New, CodeFromStatus, IsExpectedClientError. Sentinels: ErrInvalidID, ErrNotAuthenticated, ErrForbidden, ErrNotFound, ErrConflict, ErrGone, ErrUnprocessableEntity, ErrTooManyRequests, ErrServiceUnavailable. NewValidationErrorf for 400 validation errors.

### httputil

- **Responses**: RenderJSON, RenderOK, RenderCreated, RenderAccepted, RenderNoContent, RenderError, RenderErrorWithCode, RenderInvalidID, RenderText
- **Errors**: HandleError, ErrorHandler; ErrorResponse, ValidationErrorResponse
- **Request**: DecodeAndValidate[T], DecodeAndValidateE[T], DecodeJSON[T]
- **Params**: ParseUUID, ParseUUIDField, ParseAuthUserID, GetUserID(ctx)
- **Pagination**: ClampPage, ClampPerPage, ClampLimit, ParseIntQuery, TotalPages, NewPaginationMeta, Ptr[T]
- **Query**: ParseBoolQuery, ParseEnumQuery[T], ParseSortQuery, ParseTimeQuery
- **Search**: EscapeILIKE, ValidateSearchQ, SanitizeSearchQ
- **IP**: GetClientIP(r, trustedProxyCIDRs)
- **Chi**: ChiPathFromRequest(r)
- **Multipart**: ParseMultipartFormLimit
- **Download**: RenderJSONAttachment, RenderStream, RenderBytes
- **SSE**: SSEWriter, NewSSEWriter, Send, SendJSON, Close
- **Health**: Checker, HealthHandler(checkers) → JSON status and checks

### httputil/middleware

- **Metrics**: Prometheus request count and duration; PathFromRequest for route pattern
- **Recoverer(log)**: panic recovery, 500 response, stack log via go-logkit
- **Timeout(d)**: request context timeout, 503 on deadline
- **SecurityHeaders**: X-Content-Type-Options, X-Frame-Options, Referrer-Policy, Permissions-Policy, CSP
- **RequestID**: X-Request-ID from header or new UUID; GetRequestID(ctx)

## Example

```go
r.Get("/items", func(w http.ResponseWriter, r *http.Request) {
    field, dir, ok := httputil.ParseSortQuery(r, []string{"name", "score"})
    if !ok {
        field, dir = "name", "asc"
    }
    // ...
})

r.Get("/health", httputil.HealthHandler(map[string]httputil.Checker{
    "db":    dbPinger,
    "redis": redisPinger,
}))

r.Use(middleware.Recoverer(log))
r.Use(middleware.RequestID())
r.Use(middleware.SecurityHeaders())
```
