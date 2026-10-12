package httputil

import (
	"encoding/json/v2"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRenderJSON(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	RenderJSON(w, r, http.StatusOK, map[string]string{"k": "v"})
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["k"] != "v" {
		t.Errorf("body = %v", body)
	}
}

func TestRenderJSON_ContentTypeAndEscapesHTML(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	RenderJSON(w, r, http.StatusOK, map[string]string{"html": "<script>"})

	if ct := w.Header().Get("Content-Type"); ct != mimeApplicationJSON {
		t.Errorf("Content-Type = %q, want %s", ct, mimeApplicationJSON)
	}
	if body := w.Body.String(); !strings.Contains(body, `\u003cscript\u003e`) {
		t.Errorf("body = %q, want escaped HTML", body)
	}
}

func TestRenderJSON_EscapesJavaScriptSeparators(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	RenderJSON(w, r, http.StatusOK, map[string]string{"separators": "\u2028\u2029"})

	if got, want := w.Body.String(), `{"separators":"\u2028\u2029"}`+"\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestRenderJSON_PreservesJSONWireFormat(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	data := struct {
		Values   []string          `json:"values"`
		Lookup   map[string]string `json:"lookup"`
		Duration time.Duration     `json:"duration"`
	}{Duration: 1234567890}

	RenderJSON(w, httptest.NewRequest(http.MethodGet, "/", http.NoBody), http.StatusOK, data)

	if got, want := w.Body.String(), `{"values":null,"lookup":null,"duration":1234567890}`+"\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestRenderJSON_SortsMapKeys(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	data := map[string]int{"z": 1, "a": 2}

	RenderJSON(w, httptest.NewRequest(http.MethodGet, "/", http.NoBody), http.StatusOK, data)

	if got, want := w.Body.String(), "{\"a\":2,\"z\":1}\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestRenderJSON_OmitEmptyUsesJSONV2Semantics(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	data := struct {
		Count   int  `json:"count,omitempty"`
		Enabled bool `json:"enabled,omitempty"`
	}{}

	RenderJSON(w, httptest.NewRequest(http.MethodGet, "/", http.NoBody), http.StatusOK, data)

	if got, want := w.Body.String(), "{\"count\":0,\"enabled\":false}\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestRenderJSON_EncodeErrorHidesDetails(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	RenderJSON(w, r, http.StatusOK, map[string]float64{"invalid_number": math.Inf(1)})

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != mimeApplicationJSON {
		t.Errorf("Content-Type = %q, want %s", ct, mimeApplicationJSON)
	}
	if strings.Contains(w.Body.String(), "unsupported value") || strings.Contains(w.Body.String(), "+Inf") {
		t.Errorf("body leaked encode error details: %q", w.Body.String())
	}
	var body ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Code != "INTERNAL_ERROR" || body.Message != msgInternalServerError {
		t.Errorf("body = %+v", body)
	}
}

func TestRenderNoContent(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	RenderNoContent(w, r)
	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("body non-empty: %q", w.Body.Bytes())
	}
}

func TestRenderCreated(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	RenderCreated(w, r, map[string]int{"id": 1})
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", w.Code)
	}
	var body map[string]int
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["id"] != 1 {
		t.Errorf("body = %v", body)
	}
}

func TestRenderAccepted(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	RenderAccepted(w, r, map[string]string{"task": "queued"})
	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["task"] != "queued" {
		t.Errorf("body = %v", body)
	}
}

func TestRenderOK(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	RenderOK(w, r, "ok")
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var body string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body != "ok" {
		t.Errorf("body = %q", body)
	}
}

func TestRenderError(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	RenderError(w, r, http.StatusBadRequest, "invalid input")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	var body ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Code != "BAD_REQUEST" {
		t.Errorf("Code = %q", body.Code)
	}
	if body.Message != "invalid input" {
		t.Errorf("Message = %q", body.Message)
	}
}

func TestRenderErrorWithCode(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	RenderErrorWithCode(w, r, http.StatusForbidden, "denied", "CUSTOM_DENIED")
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
	var body ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Code != "CUSTOM_DENIED" {
		t.Errorf("Code = %q", body.Code)
	}
	if body.Message != "denied" {
		t.Errorf("Message = %q", body.Message)
	}
}

func TestRenderInvalidID(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	RenderInvalidID(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	var body ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Code != "INVALID_ID" {
		t.Errorf("Code = %q", body.Code)
	}
	if body.Message != "invalid ID" {
		t.Errorf("Message = %q", body.Message)
	}
}

func TestRenderText(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	RenderText(w, r, http.StatusOK, "text/plain", "hello")
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/plain" {
		t.Errorf("Content-Type = %q", ct)
	}
	if w.Body.String() != "hello" {
		t.Errorf("body = %q", w.Body.String())
	}
}
