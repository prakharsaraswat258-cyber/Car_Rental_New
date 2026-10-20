// Package handlers holds Saarthi's HTTP handlers. They stay thin: parse the
// request, call store/domain, render a template.
package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"saarthi/internal/store"
)

// Handlers carries what every handler needs.
type Handlers struct {
	Store *store.Store
	Views *Views
	Log   *slog.Logger
	Dev   bool
}

// Page is the data every full page template gets.
type Page struct {
	Title string
	User  *store.User
	Theme string // light | dark | system
	Path  string
	Dev   bool
	Data  any
}

func (h *Handlers) page(r *http.Request, title string, data any) Page {
	return Page{
		Title: title,
		User:  UserFrom(r.Context()),
		Theme: ThemeFrom(r.Context()),
		Path:  r.URL.Path,
		Dev:   h.Dev,
		Data:  data,
	}
}

// --- request context ---------------------------------------------------------

type ctxKey int

const (
	userKey ctxKey = iota
	themeKey
)

// WithUser stores the logged-in user on the request context.
func WithUser(ctx context.Context, u *store.User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

// UserFrom returns the logged-in user, or nil.
func UserFrom(ctx context.Context) *store.User {
	u, _ := ctx.Value(userKey).(*store.User)
	return u
}

// Themes a visitor can pick.
const (
	ThemeLight  = "light"
	ThemeDark   = "dark"
	ThemeSystem = "system"
)

// ThemeCookie is the cookie holding the visitor's theme choice (D-008).
const ThemeCookie = "theme"

// WithTheme stores the theme choice on the request context.
func WithTheme(ctx context.Context, theme string) context.Context {
	return context.WithValue(ctx, themeKey, theme)
}

// ThemeFrom returns the theme choice, defaulting to system.
func ThemeFrom(ctx context.Context) string {
	if t, ok := ctx.Value(themeKey).(string); ok && validTheme(t) {
		return t
	}
	return ThemeSystem
}

func validTheme(t string) bool { return t == ThemeLight || t == ThemeDark || t == ThemeSystem }
