package httputil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserID_Empty(t *testing.T) {
	t.Parallel()
	assert.Empty(t, GetUserID(context.Background()))
}

func TestGetUserID_FromContext(t *testing.T) {
	t.Parallel()
	id := uuid.New().String()
	ctx := context.WithValue(context.Background(), UserIDKey, id)
	assert.Equal(t, id, GetUserID(ctx))
}

func TestParseUUID_Success(t *testing.T) {
	t.Parallel()
	u := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	parsed, ok := ParseUUID(w, req, u.String())
	require.True(t, ok)
	assert.Equal(t, u, parsed)
}

func TestParseUUID_Empty(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	_, ok := ParseUUID(w, req, "")
	require.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestParseUUID_Invalid(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	_, ok := ParseUUID(w, req, "not-a-uuid")
	require.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestParseUUIDField_Valid(t *testing.T) {
	t.Parallel()
	u := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	parsed, ok := ParseUUIDField(w, req, u.String(), "id")
	require.True(t, ok)
	assert.Equal(t, u, parsed)
}

func TestParseAuthUserID_NoUser(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	_, ok := ParseAuthUserID(w, req)
	require.False(t, ok)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestParseAuthUserID_Valid(t *testing.T) {
	t.Parallel()
	u := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserIDKey, u.String()))
	w := httptest.NewRecorder()
	parsed, ok := ParseAuthUserID(w, req)
	require.True(t, ok)
	assert.Equal(t, u, parsed)
}
