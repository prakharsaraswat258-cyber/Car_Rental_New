package handlers

import (
	"net/http"
	"net/url"
	"time"

	"saarthi/internal/store"
)

// Home is the explore landing page. In L0 it also shows a data check.
func (h *Handlers) Home(w http.ResponseWriter, r *http.Request) {
	counts, err := h.Store.Counts(r.Context())
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	h.Views.Page(w, r, http.StatusOK, "home", h.page(r, "Saarthi · Rent a ride from local shops", struct {
		Counts     store.Counts
		DemoEmails []string
		DemoPass   string
	}{counts, []string{store.DemoRenterEmail, store.DemoProviderEmail, store.DemoAdminEmail}, store.DemoPassword}))
}

// NotFound renders the 404 page for any unknown path.
func (h *Handlers) NotFound(w http.ResponseWriter, r *http.Request) {
	h.Views.Page(w, r, http.StatusNotFound, "notfound", h.page(r, "Not found · Saarthi", nil))
}

// Forbidden renders the 403 page.
func (h *Handlers) Forbidden(w http.ResponseWriter, r *http.Request) {
	h.Views.Page(w, r, http.StatusForbidden, "forbidden", h.page(r, "Not allowed · Saarthi", nil))
}

// SetTheme saves light / dark / system in a cookie. The page has already
// switched client-side; htmx/fetch callers get 204, forms a redirect back.
func (h *Handlers) SetTheme(w http.ResponseWriter, r *http.Request) {
	theme := r.FormValue("theme")
	if !validTheme(theme) {
		http.Error(w, "unknown theme", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: ThemeCookie, Value: theme, Path: "/",
		MaxAge: int((365 * 24 * time.Hour).Seconds()), HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: r.TLS != nil,
	})
	if r.Header.Get("HX-Request") == "true" || r.Header.Get("X-Requested-With") == "fetch" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, safeBack(r), http.StatusSeeOther)
}

// safeBack returns the same-site path the request came from, or "/".
func safeBack(r *http.Request) string {
	ref, err := url.Parse(r.Referer())
	if err != nil || ref.Host != r.Host || ref.Path == "" {
		return "/"
	}
	if ref.RawQuery != "" {
		return ref.Path + "?" + ref.RawQuery
	}
	return ref.Path
}

func (h *Handlers) serverError(w http.ResponseWriter, r *http.Request, err error) {
	h.Log.Error("handler", "path", r.URL.Path, "err", err)
	msg := "Something went wrong on our side. Please try again."
	if h.Dev {
		msg = err.Error()
	}
	http.Error(w, msg, http.StatusInternalServerError)
}
