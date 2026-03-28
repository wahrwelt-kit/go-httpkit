package middleware

import (
	"context"
	"net/http"

	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type localizerKey struct{}

// I18nOption configures the I18n middleware
type I18nOption func(*i18nConfig)

type i18nConfig struct {
	queryParam string
	cookieName string
}

// WithLanguageQueryParam sets a URL query parameter name (e.g. "lang") used as language preference,
// checked after cookie but before Accept-Language header
func WithLanguageQueryParam(param string) I18nOption {
	return func(c *i18nConfig) { c.queryParam = param }
}

// WithLanguageCookie sets a cookie name (e.g. "lang") used as language preference, checked first
func WithLanguageCookie(name string) I18nOption {
	return func(c *i18nConfig) { c.cookieName = name }
}

// I18n returns middleware that selects a locale from the request (priority: cookie ->  query param ->  Accept-Language),
// builds an *i18n.Localizer from bundle, and stores it in the context
// Use GetLocalizer or Localize to retrieve translations in handlers
// bundle must not be nil
func I18n(bundle *i18n.Bundle, opts ...I18nOption) func(http.Handler) http.Handler {
	if bundle == nil {
		panic("middleware.I18n: bundle must not be nil")
	}
	cfg := &i18nConfig{}
	for _, o := range opts {
		o(cfg)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			langs := requestLangs(r, cfg)
			localizer := i18n.NewLocalizer(bundle, langs...)
			ctx := context.WithValue(r.Context(), localizerKey{}, localizer)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetLocalizer returns the *i18n.Localizer stored by the I18n middleware, or nil if not set
func GetLocalizer(ctx context.Context) *i18n.Localizer {
	l, _ := ctx.Value(localizerKey{}).(*i18n.Localizer)
	return l
}

// Localize translates cfg using the Localizer from ctx (set by I18n middleware)
// If no localizer is in the context or the message is not found, returns cfg.DefaultMessage.Other,
// or "" if DefaultMessage is nil
func Localize(ctx context.Context, cfg *i18n.LocalizeConfig) string {
	if l := GetLocalizer(ctx); l != nil {
		if s, err := l.Localize(cfg); err == nil {
			return s
		}
	}
	if cfg.DefaultMessage != nil {
		return cfg.DefaultMessage.Other
	}
	return ""
}

// requestLangs builds the language preference list ordered by priority (cookie ->  query param ->  header)
// go-i18n's NewLocalizer calls language.ParseAcceptLanguage on each string, so the full
// Accept-Language header value can be passed as-is
func requestLangs(r *http.Request, cfg *i18nConfig) []string {
	var langs []string
	if cfg.cookieName != "" {
		if c, err := r.Cookie(cfg.cookieName); err == nil && c.Value != "" {
			langs = append(langs, c.Value)
		}
	}
	if cfg.queryParam != "" {
		if q := r.URL.Query().Get(cfg.queryParam); q != "" {
			langs = append(langs, q)
		}
	}
	if accept := r.Header.Get("Accept-Language"); accept != "" {
		langs = append(langs, accept)
	}
	return langs
}
