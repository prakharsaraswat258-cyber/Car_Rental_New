package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"saarthi/internal/domain"
	"saarthi/internal/store"
)

// CheckoutData feeds the checkout page.
type CheckoutData struct {
	Box   QuoteView
	Error string
}

// Checkout shows dates, add-ons and the itemised bill.
func (h *Handlers) Checkout(w http.ResponseWriter, r *http.Request) {
	r.SetPathValue("id", r.PathValue("vehicleID"))
	v, ok := h.loadVehicle(w, r)
	if !ok {
		return
	}
	start, end, _ := parseRange(r)
	box, err := h.quoteView(r.Context(), v, start, end, r.Form["addon"], UserFrom(r.Context()))
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	box.Action = "/checkout/" + v.ID
	if IsHTMX(r) {
		h.Views.Fragment(w, http.StatusOK, "checkout-bill", CheckoutData{Box: box})
		return
	}
	h.render(w, r, http.StatusOK, "checkout", "Checkout · Saarthi", "explore", CheckoutData{Box: box})
}

// PayData feeds the demo payment sheet.
type PayData struct {
	B     store.Booking
	Error string
}

// CreateBooking holds the slot (pending_payment) and opens the pay sheet.
func (h *Handlers) CreateBooking(w http.ResponseWriter, r *http.Request) {
	u := UserFrom(r.Context())
	start, end, ok := parseRange(r)
	if !ok {
		h.checkoutError(w, r, "Pick pickup and return times first.")
		return
	}
	b, err := h.Store.CreateBooking(r.Context(), store.NewBooking{
		RenterID: u.ID, VehicleID: r.FormValue("vehicle"), Start: start, End: end, AddonIDs: r.Form["addon"],
	})
	if err != nil {
		var taken *store.SlotTakenError
		if !errors.As(err, &taken) && !errors.Is(err, store.ErrInPast) && !errors.Is(err, store.ErrNotBookable) &&
			!errors.Is(err, domain.ErrEndBeforeStart) && !errors.Is(err, domain.ErrTripTooLong) && !errors.Is(err, store.ErrNotFound) {
			h.serverError(w, r, err)
			return
		}
		h.checkoutError(w, r, friendly(err))
		return
	}
	if !IsHTMX(r) {
		http.Redirect(w, r, "/trips/"+b.Code+"?pay=1", http.StatusSeeOther)
		return
	}
	h.Views.Fragment(w, http.StatusOK, "pay-sheet", PayData{B: b})
}

func (h *Handlers) checkoutError(w http.ResponseWriter, r *http.Request, msg string) {
	if IsHTMX(r) {
		w.Header().Set("HX-Retarget", "#checkout-error")
		w.Header().Set("HX-Reswap", "innerHTML")
		h.Views.Fragment(w, http.StatusOK, "form-error", msg)
		return
	}
	setFlash(w, Toast{Message: msg, Tone: "bad"})
	http.Redirect(w, r, safeBack(r), http.StatusSeeOther)
}

// Pay runs the demo payment: Success or Fail.
func (h *Handlers) Pay(w http.ResponseWriter, r *http.Request) {
	u := UserFrom(r.Context())
	code := r.PathValue("code")
	b, err := h.Store.Pay(r.Context(), u.ID, code, r.FormValue("result") == "success")
	switch {
	case err == nil:
		setFlash(w, Toast{Message: fmt.Sprintf("Paid %s (demo). Booking %s.", b.PayNow(), b.Code), Tone: "good"})
		redirect(w, r, "/trips/"+b.Code+"/pass?new=1")
	case errors.Is(err, store.ErrNotFound):
		h.notFound(w, r)
	case errors.Is(err, store.ErrPaymentDeclined), errors.Is(err, store.ErrExpired), errors.Is(err, store.ErrBadTransition):
		if !IsHTMX(r) {
			setFlash(w, Toast{Message: friendly(err), Tone: "bad"})
			http.Redirect(w, r, "/trips/"+code, http.StatusSeeOther)
			return
		}
		b, _ = h.Store.BookingByCode(r.Context(), code)
		h.Views.Fragment(w, http.StatusOK, "pay-body", PayData{B: b, Error: friendly(err)})
	default:
		h.serverError(w, r, err)
	}
}

// --- trips -------------------------------------------------------------------

// TripsData feeds My trips.
type TripsData struct {
	Tab                    string
	Live                   *store.Booking
	Trips                  []store.Booking
	Upcoming, Active, Past int
}

// Trips lists the renter's trips by tab.
func (h *Handlers) Trips(w http.ResponseWriter, r *http.Request) {
	u := UserFrom(r.Context())
	all, err := h.Store.RenterBookings(r.Context(), u.ID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	d := TripsData{Tab: r.URL.Query().Get("tab")}
	var lists = map[string][]store.Booking{}
	for i := range all {
		b := all[i]
		lists[b.Tab()] = append(lists[b.Tab()], b)
		if b.Status == domain.StatusActive && d.Live == nil {
			d.Live = &all[i]
		}
	}
	// past newest first
	past := lists["past"]
	for i, j := 0, len(past)-1; i < j; i, j = i+1, j-1 {
		past[i], past[j] = past[j], past[i]
	}
	d.Upcoming, d.Active, d.Past = len(lists["upcoming"]), len(lists["active"]), len(past)
	if d.Tab == "" {
		switch {
		case d.Active > 0:
			d.Tab = "active"
		case d.Upcoming > 0:
			d.Tab = "upcoming"
		default:
			d.Tab = "past"
		}
		if len(all) == 0 {
			d.Tab = "upcoming"
		}
	}
	d.Trips = lists[d.Tab]
	h.render(w, r, http.StatusOK, "trips", "My trips · Saarthi", "trips", d)
}

// TripData feeds the trip detail page.
type TripData struct {
	B         store.Booking
	Events    []store.Event
	Checkin   []store.TripPhoto
	Return    []store.TripPhoto
	Pairs     []PhotoPair
	Deposit   DepositView
	Next      NextStep
	Review    *store.Review
	Steps     [6]string
	CanCancel bool
	OpenPay   bool
}

func (h *Handlers) renterBooking(w http.ResponseWriter, r *http.Request) (store.Booking, bool) {
	b, err := h.Store.BookingByCode(r.Context(), r.PathValue("code"))
	u := UserFrom(r.Context())
	if errors.Is(err, store.ErrNotFound) || (err == nil && b.RenterID != u.ID) {
		h.notFound(w, r)
		return b, false
	}
	if err != nil {
		h.serverError(w, r, err)
		return b, false
	}
	return b, true
}

// Trip is the trip detail page: status line, next step, photos, deposit.
func (h *Handlers) Trip(w http.ResponseWriter, r *http.Request) {
	b, ok := h.renterBooking(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	events, err := h.Store.BookingEvents(ctx, b.ID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	photos, err := h.Store.TripPhotos(ctx, b.ID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	dep, err := h.depositView(ctx, b)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	rev, err := h.Store.ReviewFor(ctx, b.ID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	d := TripData{B: b, Events: events, Deposit: dep, Next: nextStep(b, h.Store.Now()), Review: rev, Steps: domain.TripSteps,
		CanCancel: b.CanCancel(h.Store.Now()), OpenPay: r.URL.Query().Get("pay") == "1" && b.Status == domain.StatusPendingPayment,
		Pairs: photoPairs(photos, b.Electric())}
	for _, p := range photos {
		if p.Phase == "checkin" {
			d.Checkin = append(d.Checkin, p)
		} else {
			d.Return = append(d.Return, p)
		}
	}
	h.render(w, r, http.StatusOK, "trip", b.Code+" · "+b.VehicleName()+" · Saarthi", "trips", d)
}

// DepositFragment re-renders the Deposit Tracker (polled while released).
func (h *Handlers) DepositFragment(w http.ResponseWriter, r *http.Request) {
	b, ok := h.renterBooking(w, r)
	if !ok {
		return
	}
	dep, err := h.depositView(r.Context(), b)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	h.Views.Fragment(w, http.StatusOK, "deposit-tracker", dep)
}

// PassData feeds the Pickup Pass.
type PassData struct {
	B        store.Booking
	Reveal   bool
	ShowTips bool // "How pickup works", until dismissed (cookie)
}

// TipsCookie remembers that the "How pickup works" card was dismissed.
const TipsCookie = "pickup_tips"

// Pass is the Pickup Pass.
func (h *Handlers) Pass(w http.ResponseWriter, r *http.Request) {
	b, ok := h.renterBooking(w, r)
	if !ok {
		return
	}
	switch b.Status {
	case domain.StatusPendingPayment:
		http.Redirect(w, r, "/trips/"+b.Code+"?pay=1", http.StatusSeeOther)
		return
	case domain.StatusCancelled, domain.StatusDeclined, domain.StatusExpired:
		http.Redirect(w, r, "/trips/"+b.Code, http.StatusSeeOther)
		return
	}
	_, dismissed := r.Cookie(TipsCookie)
	h.render(w, r, http.StatusOK, "pass", "Pickup Pass "+b.Code+" · Saarthi", "trips",
		PassData{B: b, Reveal: r.URL.Query().Get("new") == "1", ShowTips: dismissed != nil})
}

// PassICS is the Add to calendar file.
func (h *Handlers) PassICS(w http.ResponseWriter, r *http.Request) {
	b, ok := h.renterBooking(w, r)
	if !ok {
		return
	}
	esc := func(s string) string {
		return strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\n", `\n`).Replace(s)
	}
	const f = "20060102T150405Z"
	lines := []string{
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//Saarthi//Demo//EN", "CALSCALE:GREGORIAN", "METHOD:PUBLISH",
		"BEGIN:VEVENT",
		"UID:" + b.Code + "@saarthi.test",
		"DTSTAMP:" + h.Store.Now().UTC().Format(f),
		"DTSTART:" + b.Start.UTC().Format(f),
		"DTEND:" + b.End.UTC().Format(f),
		"SUMMARY:" + esc(b.VehicleName()+" from "+b.ShopName+" · "+b.Code),
		"LOCATION:" + esc(b.ShopName+", "+b.ShopAddress),
		fmt.Sprintf("GEO:%.5f;%.5f", b.ShopLat, b.ShopLng),
		"URL:" + scheme(r) + "://" + r.Host + "/trips/" + b.Code + "/pass",
		"STATUS:CONFIRMED", "TRANSP:OPAQUE",
		"DESCRIPTION:" + esc(fmt.Sprintf("Pickup Pass %s.\nBring your driving licence and one photo ID.\nTotal paid %s. Refundable deposit %s.\nShop: %s",
			b.Code, b.Total, b.Deposit, b.ShopPhone)),
		"BEGIN:VALARM", "TRIGGER:-PT1H", "ACTION:DISPLAY", "DESCRIPTION:" + esc("Pickup in an hour: "+b.VehicleName()), "END:VALARM",
		"END:VEVENT", "END:VCALENDAR",
	}
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="saarthi-`+b.Code+`.ics"`)
	w.Write([]byte(strings.Join(lines, "\r\n") + "\r\n"))
}

func scheme(r *http.Request) string {
	// behind a hosting proxy (Vercel) TLS ends before us
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		return "https"
	}
	return "http"
}

// CancelTrip cancels a booking (free until 24 h before pickup).
func (h *Handlers) CancelTrip(w http.ResponseWriter, r *http.Request) {
	u := UserFrom(r.Context())
	b, err := h.Store.Cancel(r.Context(), u.ID, r.PathValue("code"))
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w, r)
		return
	}
	if err != nil {
		setFlash(w, Toast{Message: "Pickup is less than 24 h away, so it can't be cancelled here. Message the shop on WhatsApp.", Tone: "warn"})
		redirect(w, r, "/trips/"+r.PathValue("code"))
		return
	}
	setFlash(w, Toast{Message: "Cancelled. Nothing was kept.", Tone: "good"})
	redirect(w, r, "/trips/"+b.Code)
}
