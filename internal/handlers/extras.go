package handlers

import (
	"errors"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"

	"saarthi/internal/store"
)

// --- shop profile ------------------------------------------------------------

// ShopData feeds /s/{slug}.
type ShopData struct {
	S           store.Shop
	Rating      float64
	RatingCount int
	Cards       []CardView
	Reviews     []store.ShopReview
	WhatsApp    string
}

// ShopPage is a shop's public profile: header, vehicles, reviews.
func (h *Handlers) ShopPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s, err := h.Store.ShopBySlug(ctx, r.PathValue("slug"))
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w, r)
		return
	}
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	u := UserFrom(ctx)
	if s.Status != "verified" && !u.IsAdmin() && !(u != nil && u.ID == s.OwnerID) {
		h.notFound(w, r)
		return
	}
	avg, n, err := h.Store.ShopRating(ctx, s.ID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	vs, err := h.Store.ShopLiveVehicles(ctx, s.ID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	reviews, err := h.Store.ShopReviews(ctx, s.ID, 8)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	p := h.page(w, r, s.Name+" · "+s.Place()+" · Saarthi", "explore", nil)
	p.Data = ShopData{S: s, Rating: avg, RatingCount: n, Cards: h.cards(vs, time.Time{}, time.Time{}, p.InCompare), Reviews: reviews,
		WhatsApp: "https://wa.me/" + s.WhatsApp + "?text=" + url.QueryEscape("Hi "+s.Name+", I found you on Saarthi.")}
	h.Views.Page(w, r, http.StatusOK, "shop", p)
}

// --- command palette ---------------------------------------------------------

// PaletteData is what the Ctrl K palette lists.
type PaletteData struct {
	Q    string
	Hits []store.PaletteHit
}

var providerPages = []store.PaletteHit{
	{Kind: "page", Title: "Today", Sub: "Requests, pickups and returns", URL: "/p"},
	{Kind: "page", Title: "Fleet", Sub: "All vehicles", URL: "/p/fleet"},
	{Kind: "page", Title: "Calendar", Sub: "Bookings and blocks", URL: "/p/calendar"},
	{Kind: "page", Title: "Add a vehicle", Sub: "Shortcut N", URL: "/p/vehicles/new"},
	{Kind: "page", Title: "Account", Sub: "Profile and theme", URL: "/account"},
	{Kind: "page", Title: "Renter site", Sub: "See what renters see", URL: "/"},
}

var adminPages = []store.PaletteHit{
	{Kind: "page", Title: "Overview", Sub: "KPIs, charts, shops", URL: "/admin"},
	{Kind: "page", Title: "Bookings", Sub: "Every booking", URL: "/admin/bookings"},
	{Kind: "page", Title: "Bookings on trip", Sub: "Status: on trip", URL: "/admin/bookings?status=active"},
	{Kind: "page", Title: "Bookings being inspected", Sub: "Status: returned", URL: "/admin/bookings?status=returned"},
	{Kind: "page", Title: "Styleguide", Sub: "Design system", URL: "/dev/styleguide"},
	{Kind: "page", Title: "Renter site", Sub: "Explore", URL: "/"},
}

func filterPages(pages []store.PaletteHit, q string) []store.PaletteHit {
	q = strings.ToLower(strings.TrimSpace(q))
	var out []store.PaletteHit
	for _, p := range pages {
		if q == "" || strings.Contains(strings.ToLower(p.Title+" "+p.Sub), q) {
			out = append(out, p)
		}
	}
	return out
}

// Palette answers the provider (shop-scoped) or admin command palette.
func (h *Handlers) Palette(admin bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if rs := []rune(q); len(rs) > 40 {
			q = string(rs[:40])
		}
		pages, shopID := adminPages, ""
		if !admin {
			shop, ok := h.providerShop(w, r)
			if !ok {
				return
			}
			pages, shopID = providerPages, shop.ID
		}
		hits := filterPages(pages, q)
		if q != "" {
			found, err := h.Store.Palette(r.Context(), shopID, q, 6)
			if err != nil {
				h.serverError(w, r, err)
				return
			}
			hits = append(hits, found...)
		}
		h.Views.Fragment(w, http.StatusOK, "palette-results", PaletteData{Q: q, Hits: hits})
	}
}

// --- offline, service worker, manifest ---------------------------------------

// Offline is what the service worker shows when there's no network.
func (h *Handlers) Offline(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, "offline", "Offline · Saarthi", "", nil)
}

// ServiceWorker serves /sw.js from the site root (so it can cover /trips),
// with the versioned app-shell URLs filled in.
func (h *Handlers) ServiceWorker(w http.ResponseWriter, r *http.Request) {
	src, err := fs.ReadFile(h.Views.fsys, "static/sw.js")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	shell := []string{"/offline"}
	for _, p := range []string{"/static/css/tokens.css", "/static/css/app.css", "/static/vendor/htmx.min.js", "/static/js/app.js",
		"/static/img/icons.svg", "/static/img/favicon.svg", "/static/fonts/inter-latin.woff2", "/static/fonts/fraunces-latin.woff2",
		"/static/fonts/inter-latin-ext.woff2", "/static/fonts/fraunces-latin-ext.woff2"} {
		shell = append(shell, h.Views.asset(p))
	}
	ver := h.Views.asset("/static/css/app.css") + h.Views.asset("/static/js/app.js")
	body := strings.NewReplacer(`"__SHELL__"`, toJSON(shell), "__VERSION__", shortHash(ver)).Replace(string(src))
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Service-Worker-Allowed", "/")
	w.Write([]byte(body))
}

func shortHash(s string) string {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h = (h ^ uint32(s[i])) * 16777619
	}
	const hex = "0123456789abcdef"
	out := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		out[i] = hex[h&15]
		h >>= 4
	}
	return string(out)
}

// Manifest makes Saarthi installable on Android.
func (h *Handlers) Manifest(w http.ResponseWriter, r *http.Request) {
	m := map[string]any{
		"name": "Saarthi", "short_name": "Saarthi",
		"description": "Rent scooters, bikes, cars, EVs and cycles from local shops.",
		"start_url":   "/trips", "scope": "/", "display": "standalone",
		"background_color": "#FAF8F3", "theme_color": "#FAF8F3",
		"icons": []map[string]string{
			{"src": h.Views.asset("/static/img/icon-192.png"), "sizes": "192x192", "type": "image/png"},
			{"src": h.Views.asset("/static/img/icon-512.png"), "sizes": "512x512", "type": "image/png"},
			{"src": h.Views.asset("/static/img/icon-512.png"), "sizes": "512x512", "type": "image/png", "purpose": "maskable"},
			{"src": h.Views.asset("/static/img/favicon.svg"), "sizes": "any", "type": "image/svg+xml"},
		},
	}
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write([]byte(toJSON(m)))
}

// --- errors ------------------------------------------------------------------

// ErrorPage renders the designed 500 page ("Your bookings are safe").
func (h *Handlers) ErrorPage(w http.ResponseWriter, r *http.Request) {
	if IsHTMX(r) || r.Header.Get("X-Requested-With") == "fetch" {
		http.Error(w, "Something went wrong on our side. Please try again.", http.StatusInternalServerError)
		return
	}
	h.render(w, r, http.StatusInternalServerError, "error", "Something broke · Saarthi", "", nil)
}
