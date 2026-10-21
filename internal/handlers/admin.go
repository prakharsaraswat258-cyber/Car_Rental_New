package handlers

import (
	"net/http"

	"saarthi/internal/domain"
	"saarthi/internal/store"
)

// AdminData feeds the admin overview.
type AdminData struct {
	K        store.KPIs
	Shops    []store.Shop
	Recent   []store.Booking
	Status   string
	Bookings []store.Booking
	Statuses []domain.Status
	Charts   []Chart
}

// Admin is the overview: KPI tiles, shops, latest bookings.
func (h *Handlers) Admin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	k, err := h.Store.AdminKPIs(ctx)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	shops, err := h.Store.Shops(ctx)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	recent, err := h.Store.AllBookings(ctx, "", 6)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	series, err := h.Store.DailySeries(ctx, 30)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	h.render(w, r, http.StatusOK, "admin/overview", "Admin · Saarthi", "admin",
		AdminData{K: k, Shops: shops, Recent: recent, Charts: adminCharts(series)})
}

// AdminBooking shows one booking with angle-matched proof photos.
func (h *Handlers) AdminBooking(w http.ResponseWriter, r *http.Request) {
	b, err := h.Store.BookingByID(r.Context(), r.PathValue("id"))
	if err != nil {
		h.notFound(w, r)
		return
	}
	events, _ := h.Store.BookingEvents(r.Context(), b.ID)
	photos, _ := h.Store.TripPhotos(r.Context(), b.ID)
	dep, _ := h.depositView(r.Context(), b)
	h.render(w, r, http.StatusOK, "admin/booking", b.Code+" · Admin · Saarthi", "admin-bookings",
		TripData{B: b, Events: events, Deposit: dep, Steps: domain.TripSteps, Pairs: photoPairs(photos, b.Electric())})
}

// AdminShopStatus verifies or suspends a shop.
func (h *Handlers) AdminShopStatus(status string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h.Store.SetShopStatus(r.Context(), r.PathValue("id"), status); err != nil {
			h.notFound(w, r)
			return
		}
		shop, err := h.Store.ShopByID(r.Context(), r.PathValue("id"))
		if err != nil {
			h.serverError(w, r, err)
			return
		}
		msg := shop.Name + " is verified. Its live vehicles now show in search."
		if status == "suspended" {
			msg = shop.Name + " is suspended and hidden from search."
		}
		if IsHTMX(r) {
			toastHeader(w, Toast{Message: msg, Tone: "good"})
			h.Views.Fragment(w, http.StatusOK, "shop-row", shop)
			return
		}
		setFlash(w, Toast{Message: msg, Tone: "good"})
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	}
}

// AdminBookings lists every booking, filterable by status.
func (h *Handlers) AdminBookings(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	statuses := []domain.Status{domain.StatusPendingPayment, domain.StatusAwaitingShop, domain.StatusConfirmed, domain.StatusReady,
		domain.StatusActive, domain.StatusReturned, domain.StatusClosed, domain.StatusCancelled, domain.StatusDeclined, domain.StatusExpired}
	valid := false
	for _, s := range statuses {
		valid = valid || string(s) == status
	}
	if !valid {
		status = ""
	}
	list, err := h.Store.AllBookings(r.Context(), status, 300)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	h.render(w, r, http.StatusOK, "admin/bookings", "Bookings · Admin · Saarthi", "admin-bookings",
		AdminData{Status: status, Bookings: list, Statuses: statuses})
}
