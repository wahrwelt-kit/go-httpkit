package httputil

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/TakuyaYagam1/go-httpkit/httperr"
)

type contextKey string

// UserIDKey is the context key for the authenticated user ID (string, e.g. UUID). Set by auth middleware; read with GetUserID.
const UserIDKey contextKey = "user_id"

// GetUserID returns the authenticated user ID from context, or "" if not set.
func GetUserID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		return userID
	}
	return ""
}

// ParseUUID parses id as a UUID. On failure writes an error response and returns (uuid.Nil, false).
func ParseUUID(w http.ResponseWriter, r *http.Request, id string) (uuid.UUID, bool) {
	if id == "" {
		HandleError(w, r, httperr.ErrInvalidID())
		return uuid.Nil, false
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		HandleError(w, r, httperr.ErrInvalidID())
		return uuid.Nil, false
	}
	return parsed, true
}

// ParseUUIDField parses value as a UUID. On failure writes a validation error for the given field name and returns (uuid.Nil, false).
// field is used in the error message sent to the client; it must be a constant or sanitized (e.g. alphanumeric, underscore). Do not pass user-controlled input as field.
func ParseUUIDField(w http.ResponseWriter, r *http.Request, value, field string) (uuid.UUID, bool) {
	parsed, err := uuid.Parse(value)
	if err != nil {
		HandleError(w, r, httperr.NewValidationErrorf("invalid %s", sanitizeValidationFieldName(field)))
		return uuid.Nil, false
	}
	return parsed, true
}

func sanitizeValidationFieldName(s string) string {
	const maxLen = 64
	var sb strings.Builder
	for i, r := range s {
		if i >= maxLen {
			break
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			sb.WriteRune(r)
		}
	}
	if sb.Len() == 0 {
		return "field"
	}
	return sb.String()
}

// ParseAuthUserID returns the authenticated user's UUID from context. Writes 401 and returns (uuid.Nil, false) if not authenticated or invalid.
func ParseAuthUserID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	userID := GetUserID(r.Context())
	if userID == "" {
		HandleError(w, r, httperr.ErrNotAuthenticated())
		return uuid.Nil, false
	}
	return ParseUUID(w, r, userID)
}
