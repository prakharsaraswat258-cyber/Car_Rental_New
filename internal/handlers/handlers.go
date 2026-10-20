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
	Title   string
	User    *store.User
	Theme   string // light | dark | system
	Path    string
	Nav     string // active nav item
	Dev     bool
	Flash   string // JSON toast to show on load
	Compare []store.Vehicle
	Shop    *store.Shop // console pages
	Body    string      // extra body classes
	Data    any
}

// InCompare reports whether a vehicle is in the compare tray.
func (p Page) InCompare(id string) bool {
	for _, v := range p.Compare {
		if v.ID == id {
			return true
		}
	}
	return false
}

func (h *Handlers) page(w http.ResponseWriter, r *http.Request, title, nav string, data any) Page {
	p := Page{
		Title: title,
		User:  UserFrom(r.Context()),
		Theme: ThemeFrom(r.Context()),
		Path:  r.URL.Path,
		Nav:   nav,
		Dev:   h.Dev,
		Flash: takeFlash(w, r),
		Data:  data,
	}
	if ids := compareIDs(r); len(ids) > 0 {
		if vs, err := h.Store.VehiclesByIDs(r.Context(), ids); err == nil {
			p.Compare = vs
		}
	}
	return p
}

// render is Views.Page with the common page data filled in.
func (h *Handlers) render(w http.ResponseWriter, r *http.Request, status int, tmpl, title, nav string, data any) {
	h.Views.Page(w, r, status, tmpl, h.page(w, r, title, nav, data))
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
