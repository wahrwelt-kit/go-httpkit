package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

// newTestBundle creates a bundle with English (default) and French translations
// Uses the flat JSON format supported by go-i18n v2: {"MessageID": "translation"}
func newTestBundle(t *testing.T) *i18n.Bundle {
	t.Helper()
	b := i18n.NewBundle(language.English)
	b.RegisterUnmarshalFunc("json", json.Unmarshal)
	_, err := b.ParseMessageFileBytes(
		[]byte(`{"greeting":"Hello","farewell":"Goodbye"}`),
		"en.json",
	)
	require.NoError(t, err)
	_, err = b.ParseMessageFileBytes(
		[]byte(`{"greeting":"Bonjour","farewell":"Au revoir"}`),
		"fr.json",
	)
	require.NoError(t, err)
	return b
}

func TestI18n_AcceptLanguageHeader(t *testing.T) {
	t.Parallel()
	bundle := newTestBundle(t)
	var got string
	handler := I18n(bundle)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = Localize(r.Context(), &i18n.LocalizeConfig{MessageID: "greeting"})
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "fr")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	assert.Equal(t, "Bonjour", got)
}

func TestI18n_AcceptLanguageWithQValues(t *testing.T) {
	t.Parallel()
	bundle := newTestBundle(t)
	var got string
	handler := I18n(bundle)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = Localize(r.Context(), &i18n.LocalizeConfig{MessageID: "greeting"})
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,fr;q=0.8") // en wins
	handler.ServeHTTP(httptest.NewRecorder(), req)
	assert.Equal(t, "Hello", got)
}

func TestI18n_FallsBackToDefaultBundleLanguage(t *testing.T) {
	t.Parallel()
	bundle := newTestBundle(t)
	var got string
	handler := I18n(bundle)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = Localize(r.Context(), &i18n.LocalizeConfig{MessageID: "greeting"})
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "de") // not in bundle -> falls back to English
	handler.ServeHTTP(httptest.NewRecorder(), req)
	assert.Equal(t, "Hello", got)
}

func TestI18n_QueryParamOverridesHeader(t *testing.T) {
	t.Parallel()
	bundle := newTestBundle(t)
	var got string
	handler := I18n(bundle, WithLanguageQueryParam("lang"))(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = Localize(r.Context(), &i18n.LocalizeConfig{MessageID: "greeting"})
	}))
	req := httptest.NewRequest(http.MethodGet, "/?lang=fr", nil)
	req.Header.Set("Accept-Language", "en")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	assert.Equal(t, "Bonjour", got)
}

func TestI18n_CookieOverridesQueryAndHeader(t *testing.T) {
	t.Parallel()
	bundle := newTestBundle(t)
	var got string
	handler := I18n(bundle, WithLanguageCookie("lang"), WithLanguageQueryParam("lang"))(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = Localize(r.Context(), &i18n.LocalizeConfig{MessageID: "greeting"})
	}))
	req := httptest.NewRequest(http.MethodGet, "/?lang=en", nil)
	req.Header.Set("Accept-Language", "en")
	req.AddCookie(&http.Cookie{Name: "lang", Value: "fr"})
	handler.ServeHTTP(httptest.NewRecorder(), req)
	assert.Equal(t, "Bonjour", got)
}

func TestI18n_QueryParamIgnoredWhenNotConfigured(t *testing.T) {
	t.Parallel()
	bundle := newTestBundle(t)
	var got string
	handler := I18n(bundle)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = Localize(r.Context(), &i18n.LocalizeConfig{MessageID: "greeting"})
	}))
	req := httptest.NewRequest(http.MethodGet, "/?lang=fr", nil)
	req.Header.Set("Accept-Language", "en")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	assert.Equal(t, "Hello", got) // query param ignored, uses Accept-Language
}

func TestI18n_CookieIgnoredWhenNotConfigured(t *testing.T) {
	t.Parallel()
	bundle := newTestBundle(t)
	var got string
	handler := I18n(bundle)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = Localize(r.Context(), &i18n.LocalizeConfig{MessageID: "greeting"})
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "en")
	req.AddCookie(&http.Cookie{Name: "lang", Value: "fr"})
	handler.ServeHTTP(httptest.NewRecorder(), req)
	assert.Equal(t, "Hello", got) // cookie ignored
}

func TestI18n_LocalizerStoredInContext(t *testing.T) {
	t.Parallel()
	bundle := newTestBundle(t)
	var l *i18n.Localizer
	handler := I18n(bundle)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		l = GetLocalizer(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)
	require.NotNil(t, l)
}

func TestI18n_NilBundlePanics(t *testing.T) {
	t.Parallel()
	require.Panics(t, func() { I18n(nil) })
}

func TestGetLocalizer_NotSet(t *testing.T) {
	t.Parallel()
	assert.Nil(t, GetLocalizer(context.Background()))
}

func TestLocalize_NoLocalizerInContext_ReturnsDefault(t *testing.T) {
	t.Parallel()
	got := Localize(context.Background(), &i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{ID: "greeting", Other: "Hello"},
	})
	assert.Equal(t, "Hello", got)
}

func TestLocalize_NoLocalizerNoDefault_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	got := Localize(context.Background(), &i18n.LocalizeConfig{MessageID: "greeting"})
	assert.Empty(t, got)
}

func TestLocalize_UnknownMessageID_ReturnsDefault(t *testing.T) {
	t.Parallel()
	bundle := newTestBundle(t)
	var got string
	handler := I18n(bundle)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = Localize(r.Context(), &i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{ID: "no_such_key", Other: "fallback"},
		})
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)
	assert.Equal(t, "fallback", got)
}
