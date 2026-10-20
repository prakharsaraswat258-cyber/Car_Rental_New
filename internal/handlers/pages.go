package handlers

import (
	"net/http"
	"net/url"
	"time"

	"saarthi/internal/domain"
	"saarthi/internal/store"
)

// Examples cycle through the composer placeholder every 4 s.
var askExamples = []string{
	"scooter in Mount Abu tomorrow 9am for 2 days",
	"bullet in Goa this weekend under 1k",
	"car from Panjim 15 nov to 18 nov",
	"EV near Calangute for 3 hrs",
	"cycle at Nakki Lake tomorrow morning",
}

// HomeData feeds the Explore landing page.
type HomeData struct {
	Examples []string
	Popular  []CardView
	Cities   []store.City
	Search   SearchForm
	Ask      *AskReply // non-JS fallback after POST /ask
}

// Home is the explore landing page with Ask Saarthi.
func (h *Handlers) Home(w http.ResponseWriter, r *http.Request) {
	h.home(w, r, nil)
}

func (h *Handlers) home(w http.ResponseWriter, r *http.Request, ask *AskReply) {
	ctx := r.Context()
	popular, err := h.Store.PopularVehicles(ctx, 6)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	cities, err := h.Store.Cities(ctx)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	p := h.page(w, r, "Saarthi · Rent a ride from local shops", "explore", nil)
	p.Data = HomeData{
		Examples: askExamples,
		Popular:  h.cards(popular, time.Time{}, time.Time{}, p.InCompare),
		Cities:   cities,
		Search:   SearchForm{Cities: cities, DRP: newDRP(time.Time{}, time.Time{}, h.Store.Now(), nil)},
		Ask:      ask,
	}
	h.Views.Page(w, r, http.StatusOK, "home", p)
}

// NotFound renders the 404 page for any unknown path.
func (h *Handlers) NotFound(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusNotFound, "notfound", "Not found · Saarthi", "", nil)
}

// Forbidden renders the 403 page.
func (h *Handlers) Forbidden(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusForbidden, "forbidden", "Not allowed · Saarthi", "", nil)
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

// Account shows who is logged in, the licence check and the theme.
func (h *Handlers) Account(w http.ResponseWriter, r *http.Request) {
	u := UserFrom(r.Context())
	var shop *store.Shop
	if u.IsProvider() {
		if s, err := h.Store.ShopByOwner(r.Context(), u.ID); err == nil {
			shop = &s
		}
	}
	h.render(w, r, http.StatusOK, "account", "Account · Saarthi", "account", shop)
}

// StyleguideData feeds /dev/styleguide.
type StyleguideData struct {
	Card     CardView
	Card2    CardView
	Quote    domain.PriceQuote
	DRP      DRPView
	Cal      CalView
	Deposit  DepositView
	Examples []string
	Pins     string
	Swatches []string
}

// Styleguide shows every component and animation on demo data.
func (h *Handlers) Styleguide(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := h.Store.Now()
	pop, err := h.Store.PopularVehicles(ctx, 2)
	if err != nil || len(pop) < 2 {
		h.serverError(w, r, err)
		return
	}
	start := time.Date(now.In(domain.IST).Year(), now.In(domain.IST).Month(), now.In(domain.IST).Day()+1, 9, 0, 0, 0, domain.IST)
	end := start.Add(48 * time.Hour)
	cards := h.cards(pop, start, end, nil)
	q, _ := domain.Quote(pop[0].Rates(), start, end,
		[]domain.Addon{{Name: "Extra helmet", Price: domain.Rupees(50), Per: domain.PerTrip}}, domain.DefaultTaxBPS)
	busy := []store.Busy{{Start: start.AddDate(0, 0, 6), End: start.AddDate(0, 0, 8), Kind: "booking"}}
	h.render(w, r, http.StatusOK, "styleguide", "Styleguide · Saarthi", "", StyleguideData{
		Card: cards[0], Card2: cards[1], Quote: q,
		DRP: newDRP(start, end, now, busyDays(busy)),
		Cal: calendarMonth("demo", now, now, busy),
		Deposit: DepositView{Code: "SAR-DEMO", Stop: 3, Held: domain.Rupees(1500), Deducted: domain.Rupees(180), Refund: domain.Rupees(1320),
			Items:  []domain.Deduction{{Kind: domain.DeductExtraKm, Amount: domain.Rupees(180), Note: "60 km over · ₹3 × 60 km"}},
			Labels: [4]string{"Held", "Inspection", "Released", "Credited"}, Note: "₹1,320 is back in your account.",
			HeldAt: now.Add(-50 * time.Hour), InspAt: now.Add(-2 * time.Hour), RelAt: now.Add(-time.Hour), CredAt: now.Add(-59 * time.Minute)},
		Examples: askExamples,
		Pins:     pins(cards),
		Swatches: []string{"paper", "paper-2", "surface", "ink", "ink-2", "line", "accent", "road", "good", "warn", "bad"},
	})
}

func (h *Handlers) serverError(w http.ResponseWriter, r *http.Request, err error) {
	h.Log.Error("handler", "path", r.URL.Path, "err", err)
	msg := "Something went wrong on our side. Please try again."
	if h.Dev && err != nil {
		msg = err.Error()
	}
	http.Error(w, msg, http.StatusInternalServerError)
}
