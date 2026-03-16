// Package httputil provides HTTP helpers for JSON APIs and chi-based servers.
//
// # Error handling and responses
//
// HandleError writes a JSON error response; if err implements HTTPStatus and GetCode (e.g. httperr.HTTPError),
// that status and code are used, otherwise 500 INTERNAL_ERROR. ErrorHandler wraps logging (Info for 4xx, Error for 5xx)
// and HandleError. ErrorResponse and ValidationErrorResponse are the JSON shapes. ValidationHTTPError combines *HTTPError with a slice of field errors. RenderJSON, RenderOK, RenderCreated,
// RenderAccepted, RenderNoContent, RenderError, RenderErrorWithCode, RenderInvalidID, and RenderText send typed responses.
// RenderText writes the body as-is with no escaping; use Content-Type text/plain and do not pass user-controlled HTML to avoid XSS.
// For 5xx, RenderError and RenderErrorWithCode replace the message with "Internal server error".
//
// # Request decoding and validation
//
// DecodeAndValidate reads JSON from the body (limit MaxRequestBodySize), disallows unknown fields and trailing data,
// then validates with Validator; on failure it writes the response and returns false. DecodeAndValidateE returns an
// error instead of writing. DecodeJSON decodes without validation. ParseMultipartFormLimit parses multipart form with
// body and memory limits.
//
// # Pagination
//
// ClampPage, ClampPerPage, ClampLimit, ParseIntQuery, TotalPages, NewPaginationMeta, and PaginationMeta support
// page/per-page handling. Paginated[T] holds a page of items and metadata; NewPaginated builds it; FetchPage runs
// fetch and count in parallel and returns Paginated. MaxPage is the maximum allowed page number.
//
// # Query parsing
//
// ParseBoolQuery, ParseEnumQuery, ParseSortQuery, and ParseTimeQuery parse and validate query parameters.
//
// # Client IP
//
// ParseTrustedProxyCIDRs parses CIDR strings for trusted proxies. GetClientIPWithNets returns the client IP using
// X-Real-IP and X-Forwarded-For when the connection is from a trusted net. Restrict trustedNets to your actual proxy/load-balancer CIDRs; overly broad ranges (e.g. 0.0.0.0/0) allow clients to spoof X-Forwarded-For. GetClientIPE parses CIDRs from strings and returns an error on invalid input. GetClientIP is deprecated; use GetClientIPE.
//
// # Path and context
//
// ChiPathFromRequest returns the chi route pattern. UserIDKey is the context key for the authenticated user ID.
// GetUserID, ParseUUID, ParseUUIDField, and ParseAuthUserID read or parse IDs and optionally write error responses.
//
// # Search and download
//
// ValidateSearchQ checks length and control characters. EscapeILIKE and SanitizeSearchQ escape strings for PostgreSQL
// ILIKE. RenderJSONAttachment, RenderStream, RenderStreamLimited, and RenderBytes send file downloads with
// sanitized Content-Disposition filenames. RenderStreamLimited returns ErrStreamTruncated when maxBytes > 0 and the source exceeds the limit (response is already committed). RenderStream and RenderStreamLimited return ErrInvalidContentType for disallowed content types.
//
// # SSE and health
//
// NewSSEWriter and NewSSEWriterWithLimit return an SSEWriter for Server-Sent Events; Send and SendJSON are concurrent-safe. Close marks the writer done. NewSSEWriterWithLimit accepts MaxEventBytes(n) to cap event payload size (default 64KB); larger payloads return ErrSSEPayloadTooLarge.
// HealthHandler runs Checker implementations in parallel with a timeout and returns JSON status and checks. Use HealthOnEncodeError to log or handle encode failures; HealthHideDetails omits check details from the response.
package httputil
