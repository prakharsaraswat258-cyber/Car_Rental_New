package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"saarthi/internal/domain"
	"saarthi/internal/store"
)

func (h *Handlers) providerShop(w http.ResponseWriter, r *http.Request) (store.Shop, bool) {
	u := UserFrom(r.Context())
	s, err := h.Store.ShopByOwner(r.Context(), u.ID)
	if errors.Is(err, store.ErrNotFound) {
		h.Forbidden(w, r)
		return s, false
	}
	if err != nil {
		h.serverError(w, r, err)
		return s, false
	}
	return s, true
}

func (h *Handlers) console(w http.ResponseWriter, r *http.Request, status int, tmpl, title, nav string, shop *store.Shop, data any) {
	p := h.page(w, r, title, nav, data)
	p.Shop = shop
	h.Views.Page(w, r, status, tmpl, p)
}

// OnboardItem is one step of the new-shop checklist.
type OnboardItem struct {
	Text, URL string
	Done      bool
}

// TodayData feeds the provider Today screen.
type TodayData struct {
	Requests, Pickups, Returns, Inspect, Upcoming []store.Booking
	Onboarding                                    []OnboardItem
	ShowOnboarding                                bool
	Quiet                                         bool
	NoPhotos                                      int
	Today                                         time.Time
}

// ProviderToday shows requests, today's pickups and returns.
func (h *Handlers) ProviderToday(w http.ResponseWriter, r *http.Request) {
	shop, ok := h.providerShop(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	now := h.Store.Now()
	today := domain.StartOfDay(now)
	tomorrow := today.AddDate(0, 0, 1)
	d := TodayData{Today: now}
	var err error
	if d.Requests, err = h.Store.ShopBookingsByStatus(ctx, shop.ID, domain.StatusAwaitingShop); err != nil {
		h.serverError(w, r, err)
		return
	}
	if d.Inspect, err = h.Store.ShopBookingsByStatus(ctx, shop.ID, domain.StatusReturned); err != nil {
		h.serverError(w, r, err)
		return
	}
	week, err := h.Store.ShopBookings(ctx, shop.ID, today, today.AddDate(0, 0, 8))
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	for _, b := range week {
		switch {
		case (b.Status == domain.StatusConfirmed || b.Status == domain.StatusReady) && b.Start.Before(tomorrow):
			d.Pickups = append(d.Pickups, b)
		case b.Status == domain.StatusActive && b.End.Before(tomorrow):
			d.Returns = append(d.Returns, b)
		case (b.Status == domain.StatusConfirmed || b.Status == domain.StatusReady) && !b.Start.Before(tomorrow):
			d.Upcoming = append(d.Upcoming, b)
		}
	}
	fleet, err := h.Store.ShopVehicles(ctx, shop.ID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	priced := shop.VehicleCount > 0
	for _, v := range fleet {
		if len(v.Photos) == 0 && v.Status == "live" {
			d.NoPhotos++
		}
		if v.Daily == 0 {
			priced = false
		}
	}
	d.Onboarding = []OnboardItem{
		{Text: "Shop details sent for verification", Done: true},
		{Text: "Saarthi verifies your shop", Done: shop.Status == "verified"},
		{Text: "Add your first vehicle", URL: "/p/vehicles/new", Done: shop.VehicleCount > 0},
		{Text: "Upload at least one photo", URL: "/p/fleet", Done: shop.VehicleCount > 0 && d.NoPhotos < shop.VehicleCount},
		{Text: "Set prices and publish", URL: "/p/fleet", Done: priced && shop.LiveCount > 0},
	}
	for _, o := range d.Onboarding {
		if !o.Done {
			d.ShowOnboarding = true
		}
	}
	d.Quiet = len(d.Requests)+len(d.Pickups)+len(d.Returns)+len(d.Inspect) == 0
	h.console(w, r, http.StatusOK, "provider/today", "Today · "+shop.Name, "today", &shop, d)
}

// ProviderMove handles accept / decline / ready.
func (h *Handlers) ProviderMove(act store.ShopAction) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		shop, ok := h.providerShop(w, r)
		if !ok {
			return
		}
		b, err := h.Store.ShopMove(r.Context(), shop.ID, r.PathValue("id"), act)
		if errors.Is(err, store.ErrNotFound) {
			h.notFound(w, r)
			return
		}
		msg := map[store.ShopAction]string{
			store.ShopAccept:  "Accepted. " + firstWord(b.RenterName) + " has their Pickup Pass.",
			store.ShopDecline: "Declined. " + firstWord(b.RenterName) + " gets a full refund.",
			store.ShopReady:   "Marked ready. " + firstWord(b.RenterName) + " can check in now.",
		}[act]
		if err != nil {
			msg = friendly(err)
		}
		tone := "good"
		if err != nil {
			tone = "bad"
		}
		if !IsHTMX(r) {
			setFlash(w, Toast{Message: msg, Tone: tone})
			http.Redirect(w, r, safeBack(r), http.StatusSeeOther)
			return
		}
		toastHeader(w, Toast{Message: msg, Tone: tone})
		if err == nil && act == store.ShopDecline {
			w.WriteHeader(http.StatusOK) // empty body removes the card
			return
		}
		h.Views.Fragment(w, http.StatusOK, "req-card", b)
	}
}

func firstWord(s string) string {
	if i := strings.IndexByte(s, ' '); i > 0 {
		return s[:i]
	}
	return s
}

// ProviderBooking is the shop's view of one booking.
func (h *Handlers) ProviderBooking(w http.ResponseWriter, r *http.Request) {
	shop, ok := h.providerShop(w, r)
	if !ok {
		return
	}
	b, err := h.Store.BookingByID(r.Context(), r.PathValue("id"))
	if err != nil || b.ShopID != shop.ID {
		h.notFound(w, r)
		return
	}
	events, _ := h.Store.BookingEvents(r.Context(), b.ID)
	photos, _ := h.Store.TripPhotos(r.Context(), b.ID)
	dep, _ := h.depositView(r.Context(), b)
	d := TripData{B: b, Events: events, Deposit: dep, Steps: domain.TripSteps}
	for _, p := range photos {
		if p.Phase == "checkin" {
			d.Checkin = append(d.Checkin, p)
		} else {
			d.Return = append(d.Return, p)
		}
	}
	h.console(w, r, http.StatusOK, "provider/booking", b.Code+" · "+shop.Name, "today", &shop, d)
}

// --- fleet & editor ----------------------------------------------------------

// ProviderFleet lists the shop's vehicles.
func (h *Handlers) ProviderFleet(w http.ResponseWriter, r *http.Request) {
	shop, ok := h.providerShop(w, r)
	if !ok {
		return
	}
	vs, err := h.Store.ShopVehicles(r.Context(), shop.ID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	h.console(w, r, http.StatusOK, "provider/fleet", "Fleet · "+shop.Name, "fleet", &shop, vs)
}

// GateItem is one publish requirement.
type GateItem struct {
	Text string
	Done bool
}

// EditorData feeds the 3-step vehicle editor.
type EditorData struct {
	IsNew      bool
	V          store.Vehicle
	In         store.VehicleInput
	Features   string
	Errors     map[string]string
	Step       string
	Gate       []GateItem
	CanPublish bool
	Arts       []string
	Action     string
}

func gate(in store.VehicleInput, photos int) ([]GateItem, bool) {
	items := []GateItem{
		{"Name (make and model)", strings.TrimSpace(in.Make) != "" && strings.TrimSpace(in.Model) != ""},
		{"Hourly and daily prices", in.Hourly > 0 && in.Daily > 0},
		{"At least one photo", photos > 0},
	}
	ok := true
	for _, i := range items {
		ok = ok && i.Done
	}
	return items, ok
}

func editorInput(r *http.Request) (store.VehicleInput, map[string]string) {
	in := store.VehicleInput{
		Type: r.FormValue("type"), Make: strings.TrimSpace(r.FormValue("make")), Model: strings.TrimSpace(r.FormValue("model")),
		Year: formInt(r, "year"), Reg: strings.ToUpper(strings.TrimSpace(r.FormValue("reg"))),
		Fuel: r.FormValue("fuel"), Trans: r.FormValue("trans"), Seats: formInt(r, "seats"),
		Description: strings.TrimSpace(r.FormValue("description")),
		Hourly:      formRupees(r, "hourly"), Daily: formRupees(r, "daily"), Deposit: formRupees(r, "deposit"),
		KmPerDay: formInt(r, "km"), ExtraKm: formRupees(r, "extra_km"), LateFee: formRupees(r, "late_fee"),
		MinHours: formInt(r, "min_hours"), InstantBook: r.FormValue("instant") == "on",
	}
	for _, f := range strings.Split(r.FormValue("features"), ",") {
		if f = strings.TrimSpace(f); f != "" && len(in.Features) < 12 {
			in.Features = append(in.Features, f)
		}
	}
	errs := map[string]string{}
	if !contains(domain.VehicleTypes, in.Type) {
		errs["type"] = "Pick a vehicle type."
	}
	if in.Make == "" {
		errs["make"] = "Add the make, like Honda."
	}
	if in.Model == "" {
		errs["model"] = "Add the model, like Activa 6G."
	}
	if !contains([]string{"petrol", "diesel", "electric", "none"}, in.Fuel) {
		in.Fuel = "petrol"
	}
	if !contains([]string{"manual", "automatic", "none"}, in.Trans) {
		in.Trans = "manual"
	}
	if in.Seats < 1 || in.Seats > 9 {
		in.Seats = 2
	}
	if in.MinHours < 1 || in.MinHours > 24 {
		in.MinHours = 1
	}
	if in.KmPerDay < 0 {
		in.KmPerDay = 0
	}
	if in.Daily > 0 && in.Hourly > in.Daily {
		errs["hourly"] = "The hourly price should be less than the daily price."
	}
	return in, errs
}

func (h *Handlers) editor(w http.ResponseWriter, r *http.Request, shop store.Shop, d EditorData, status int) {
	d.Arts = []string{"scooter", "bike", "car", "ev-car", "ev-scooter", "cycle"}
	d.Features = strings.Join(d.In.Features, ", ")
	d.Gate, d.CanPublish = gate(d.In, len(d.V.Photos))
	if d.Step == "" {
		d.Step = "basics"
	}
	title := "Add a vehicle"
	if !d.IsNew {
		title = "Edit " + d.V.Name()
	}
	h.console(w, r, status, "provider/editor", title+" · "+shop.Name, "fleet", &shop, d)
}

// VehicleNew shows an empty editor.
func (h *Handlers) VehicleNew(w http.ResponseWriter, r *http.Request) {
	shop, ok := h.providerShop(w, r)
	if !ok {
		return
	}
	in := store.VehicleInput{Type: "scooter", Fuel: "petrol", Trans: "automatic", Seats: 2, MinHours: 2, KmPerDay: 100,
		InstantBook: true, Year: h.Store.Now().Year()}
	h.editor(w, r, shop, EditorData{IsNew: true, In: in, Errors: map[string]string{}, Action: "/p/vehicles/new"}, http.StatusOK)
}

// VehicleCreate saves a new draft and moves to the Photos step.
func (h *Handlers) VehicleCreate(w http.ResponseWriter, r *http.Request) {
	shop, ok := h.providerShop(w, r)
	if !ok {
		return
	}
	in, errs := editorInput(r)
	if len(errs) > 0 {
		h.editor(w, r, shop, EditorData{IsNew: true, In: in, Errors: errs, Action: "/p/vehicles/new"}, http.StatusUnprocessableEntity)
		return
	}
	id, err := h.Store.CreateVehicle(r.Context(), shop.ID, in)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	setFlash(w, Toast{Message: "Saved as a draft. Add a photo next.", Tone: "good"})
	http.Redirect(w, r, "/p/vehicles/"+id+"/edit?step=photos", http.StatusSeeOther)
}

func (h *Handlers) shopVehicle(w http.ResponseWriter, r *http.Request, shop store.Shop) (store.Vehicle, bool) {
	v, err := h.Store.VehicleByID(r.Context(), r.PathValue("id"))
	if err != nil || v.ShopID != shop.ID {
		h.notFound(w, r)
		return v, false
	}
	return v, true
}

func inputOf(v store.Vehicle) store.VehicleInput {
	return store.VehicleInput{Type: v.Type, Make: v.Make, Model: v.Model, Year: v.Year, Reg: v.Reg, Fuel: v.Fuel, Trans: v.Trans,
		Seats: v.Seats, Features: v.Features, Description: v.Description, Hourly: v.Hourly, Daily: v.Daily, Deposit: v.Deposit,
		KmPerDay: v.KmPerDay, ExtraKm: v.ExtraKm, LateFee: v.LateFee, MinHours: v.MinHours, InstantBook: v.InstantBook}
}

// VehicleEdit shows the editor for an existing vehicle.
func (h *Handlers) VehicleEdit(w http.ResponseWriter, r *http.Request) {
	shop, ok := h.providerShop(w, r)
	if !ok {
		return
	}
	v, ok := h.shopVehicle(w, r, shop)
	if !ok {
		return
	}
	h.editor(w, r, shop, EditorData{V: v, In: inputOf(v), Errors: map[string]string{}, Step: r.URL.Query().Get("step"),
		Action: "/p/vehicles/" + v.ID + "/edit"}, http.StatusOK)
}

// VehicleUpdate saves the editor; action=publish also puts it live.
func (h *Handlers) VehicleUpdate(w http.ResponseWriter, r *http.Request) {
	shop, ok := h.providerShop(w, r)
	if !ok {
		return
	}
	v, ok := h.shopVehicle(w, r, shop)
	if !ok {
		return
	}
	in, errs := editorInput(r)
	d := EditorData{V: v, In: in, Errors: errs, Step: r.FormValue("step"), Action: "/p/vehicles/" + v.ID + "/edit"}
	if len(errs) > 0 {
		h.editor(w, r, shop, d, http.StatusUnprocessableEntity)
		return
	}
	if err := h.Store.UpdateVehicle(r.Context(), shop.ID, v.ID, in); err != nil {
		h.serverError(w, r, err)
		return
	}
	if r.FormValue("action") == "publish" {
		if _, ok := gate(in, len(v.Photos)); !ok {
			d.Errors["publish"] = "Add a name, both prices and at least one photo to publish."
			h.editor(w, r, shop, d, http.StatusUnprocessableEntity)
			return
		}
		if err := h.Store.SetVehicleStatus(r.Context(), shop.ID, v.ID, "live"); err != nil {
			h.serverError(w, r, err)
			return
		}
		msg := in.Make + " " + in.Model + " is live."
		if shop.Status != "verified" {
			msg += " Renters see it once Saarthi verifies your shop."
		}
		setFlash(w, Toast{Message: msg, Tone: "good"})
		http.Redirect(w, r, "/p/fleet", http.StatusSeeOther)
		return
	}
	setFlash(w, Toast{Message: "Saved.", Tone: "good"})
	http.Redirect(w, r, "/p/vehicles/"+v.ID+"/edit?step="+d.Step, http.StatusSeeOther)
}

// VehiclePhotoUpload adds a photo to a vehicle's gallery.
func (h *Handlers) VehiclePhotoUpload(w http.ResponseWriter, r *http.Request) {
	shop, ok := h.providerShop(w, r)
	if !ok {
		return
	}
	v, ok := h.shopVehicle(w, r, shop)
	if !ok {
		return
	}
	if len(v.Photos) >= 8 {
		badRequest(w, "A vehicle can have up to 8 photos.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, store.MaxUploadBytes+1<<20)
	file, _, err := r.FormFile("photo")
	if err != nil {
		badRequest(w, "Choose a photo to upload. Max 8 MB.")
		return
	}
	defer file.Close()
	id := store.NewID()
	rel := path.Join("vehicles", v.ID, id+".jpg")
	if err := h.Store.SaveImage(file, rel); err != nil {
		if errors.Is(err, store.ErrTooBig) || errors.Is(err, store.ErrNotImage) {
			badRequest(w, friendlyUpload(err))
			return
		}
		h.serverError(w, r, err)
		return
	}
	if err := h.Store.AddVehiclePhoto(r.Context(), v.ID, id, rel); err != nil {
		h.serverError(w, r, err)
		return
	}
	h.photoGrid(w, r, v.ID)
}

func friendlyUpload(err error) string {
	if errors.Is(err, store.ErrTooBig) {
		return "That photo is over 8 MB. Try a smaller one."
	}
	return "That file isn't a JPEG or PNG photo."
}

// VehiclePhotoDelete removes a photo.
func (h *Handlers) VehiclePhotoDelete(w http.ResponseWriter, r *http.Request) {
	shop, ok := h.providerShop(w, r)
	if !ok {
		return
	}
	rel, err := h.Store.DeleteVehiclePhoto(r.Context(), shop.ID, r.PathValue("id"), r.PathValue("pid"))
	if err != nil {
		h.notFound(w, r)
		return
	}
	h.Store.RemoveUpload(rel)
	h.photoGrid(w, r, r.PathValue("id"))
}

func (h *Handlers) photoGrid(w http.ResponseWriter, r *http.Request, vehicleID string) {
	v, err := h.Store.VehicleByID(r.Context(), vehicleID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	if r.Header.Get("X-Requested-With") != "fetch" && !IsHTMX(r) {
		http.Redirect(w, r, "/p/vehicles/"+v.ID+"/edit?step=photos", http.StatusSeeOther)
		return
	}
	h.Views.Fragment(w, http.StatusOK, "photo-manage", v)
}

// VehicleStatus toggles Live / Hidden from the fleet grid.
func (h *Handlers) VehicleStatus(w http.ResponseWriter, r *http.Request) {
	shop, ok := h.providerShop(w, r)
	if !ok {
		return
	}
	v, ok := h.shopVehicle(w, r, shop)
	if !ok {
		return
	}
	status := r.FormValue("status")
	if status != "live" && status != "hidden" {
		badRequest(w, "Unknown status.")
		return
	}
	if status == "live" {
		if _, ok := gate(inputOf(v), len(v.Photos)); !ok && v.Status == "draft" {
			msg := "Add a photo and prices in the editor before publishing."
			if IsHTMX(r) {
				toastHeader(w, Toast{Message: msg, Tone: "warn"})
				h.Views.Fragment(w, http.StatusOK, "fleet-card", v)
				return
			}
			setFlash(w, Toast{Message: msg, Tone: "warn"})
			http.Redirect(w, r, "/p/fleet", http.StatusSeeOther)
			return
		}
	}
	if err := h.Store.SetVehicleStatus(r.Context(), shop.ID, v.ID, status); err != nil {
		h.serverError(w, r, err)
		return
	}
	v.Status = status
	msg := v.Name() + " is live."
	if status == "hidden" {
		msg = v.Name() + " is hidden from renters."
	}
	if IsHTMX(r) {
		toastHeader(w, Toast{Message: msg, Tone: "good"})
		h.Views.Fragment(w, http.StatusOK, "fleet-card", v)
		return
	}
	setFlash(w, Toast{Message: msg, Tone: "good"})
	http.Redirect(w, r, "/p/fleet", http.StatusSeeOther)
}

// --- calendar ----------------------------------------------------------------

// CalHead is a day column.
type CalHead struct {
	Date           string
	Dow, Day       string
	Today, Weekend bool
}

// CalBar is a booking or block drawn on the grid.
type CalBar struct {
	Left, Width         float64
	Title, Sub, Kind    string
	Href, Code, BlockID string
}

// CalRow is one vehicle's line.
type CalRow struct {
	V    store.Vehicle
	Bars []CalBar
}

// ProviderCalData feeds the shop calendar.
type ProviderCalData struct {
	Days             []CalHead
	Rows             []CalRow
	From             string
	PrevURL, NextURL string
	Title            string
}

// CalendarDays is how many days the provider calendar shows.
const CalendarDays = 14

func (h *Handlers) providerCalData(r *http.Request, shop store.Shop) (ProviderCalData, error) {
	ctx := r.Context()
	now := h.Store.Now()
	from := domain.StartOfDay(now)
	if t, err := time.ParseInLocation("2006-01-02", r.URL.Query().Get("from"), domain.IST); err == nil {
		from = t
	}
	to := from.AddDate(0, 0, CalendarDays)
	span := to.Sub(from).Hours()
	d := ProviderCalData{From: from.Format("2006-01-02"),
		PrevURL: "/p/calendar?from=" + from.AddDate(0, 0, -7).Format("2006-01-02"),
		NextURL: "/p/calendar?from=" + from.AddDate(0, 0, 7).Format("2006-01-02"),
		Title:   domain.DayText(from) + " – " + domain.DayText(to.AddDate(0, 0, -1)),
	}
	today := domain.StartOfDay(now)
	for i := 0; i < CalendarDays; i++ {
		day := from.AddDate(0, 0, i)
		d.Days = append(d.Days, CalHead{Date: day.Format("2006-01-02"), Dow: day.Format("Mon"), Day: day.Format("2"),
			Today: day.Equal(today), Weekend: day.Weekday() == time.Saturday || day.Weekday() == time.Sunday})
	}
	vs, err := h.Store.ShopVehicles(ctx, shop.ID)
	if err != nil {
		return d, err
	}
	bookings, err := h.Store.ShopBookings(ctx, shop.ID, from, to)
	if err != nil {
		return d, err
	}
	blocks, err := h.Store.ShopBlocks(ctx, shop.ID, from, to)
	if err != nil {
		return d, err
	}
	place := func(s, e time.Time) (float64, float64) {
		if s.Before(from) {
			s = from
		}
		if e.After(to) {
			e = to
		}
		left := s.Sub(from).Hours() / span * 100
		return left, e.Sub(s).Hours() / span * 100
	}
	for _, v := range vs {
		if v.Status == "draft" {
			continue
		}
		row := CalRow{V: v}
		for _, b := range bookings {
			if b.VehicleID != v.ID {
				continue
			}
			l, wd := place(b.Start, b.End)
			kind := ""
			switch b.Status {
			case domain.StatusAwaitingShop, domain.StatusPendingPayment:
				kind = "is-pending"
			case domain.StatusReturned, domain.StatusClosed:
				kind = "is-done"
			}
			row.Bars = append(row.Bars, CalBar{Left: l, Width: wd, Title: firstWord(b.RenterName) + " · " + b.Status.Label(),
				Sub: domain.RangeText(b.Start, b.End), Kind: kind, Href: "/p/bookings/" + b.ID, Code: b.Code})
		}
		for _, bl := range blocks {
			if bl.VehicleID != v.ID {
				continue
			}
			l, wd := place(bl.Start, bl.End)
			note := bl.Note
			if note == "" {
				note = "Blocked"
			}
			row.Bars = append(row.Bars, CalBar{Left: l, Width: wd, Title: note, Sub: "Tap to remove", Kind: "is-block", BlockID: bl.ID})
		}
		d.Rows = append(d.Rows, row)
	}
	return d, nil
}

// ProviderCalendar is the vehicles × 14 days grid.
func (h *Handlers) ProviderCalendar(w http.ResponseWriter, r *http.Request) {
	shop, ok := h.providerShop(w, r)
	if !ok {
		return
	}
	d, err := h.providerCalData(r, shop)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	if IsHTMX(r) && r.Header.Get("HX-Target") == "pcal" {
		h.Views.Fragment(w, http.StatusOK, "pcal", d)
		return
	}
	h.console(w, r, http.StatusOK, "provider/calendar", "Calendar · "+shop.Name, "calendar", &shop, d)
}

// CreateBlock blocks whole days (start..end inclusive) for a vehicle.
func (h *Handlers) CreateBlock(w http.ResponseWriter, r *http.Request) {
	shop, ok := h.providerShop(w, r)
	if !ok {
		return
	}
	s, err1 := time.ParseInLocation("2006-01-02", r.FormValue("start"), domain.IST)
	e, err2 := time.ParseInLocation("2006-01-02", r.FormValue("end"), domain.IST)
	if err1 != nil || err2 != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Pick a start and an end day."})
		return
	}
	if e.Before(s) {
		s, e = e, s
	}
	note := strings.TrimSpace(r.FormValue("note"))
	if rs := []rune(note); len(rs) > 60 {
		note = string(rs[:60])
	}
	b, err := h.Store.CreateBlock(r.Context(), shop.ID, r.FormValue("vehicle"), s, e.AddDate(0, 0, 1), note)
	var taken *store.SlotTakenError
	switch {
	case errors.As(err, &taken):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Already booked", "code": taken.Busy.Label})
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Vehicle not found."})
	case err != nil:
		h.serverError(w, r, err)
	default:
		text := fmt.Sprintf("Blocked %s", rangeDays(s, e))
		writeJSON(w, http.StatusOK, map[string]any{"id": b.ID, "message": text})
	}
}

func rangeDays(s, e time.Time) string {
	if s.Equal(e) {
		return domain.DayText(s)
	}
	return domain.DayText(s) + " to " + domain.DayText(e)
}

// DeleteBlock removes a block; the reply carries what Undo needs.
func (h *Handlers) DeleteBlock(w http.ResponseWriter, r *http.Request) {
	shop, ok := h.providerShop(w, r)
	if !ok {
		return
	}
	b, err := h.Store.DeleteBlock(r.Context(), shop.ID, r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Block not found."})
		return
	}
	last := b.End.AddDate(0, 0, -1)
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "Block removed",
		"undo": Undo{URL: "/p/blocks", Method: "POST", Body: map[string]string{
			"vehicle": b.VehicleID, "start": b.Start.In(domain.IST).Format("2006-01-02"),
			"end": last.In(domain.IST).Format("2006-01-02"), "note": b.Note}},
	})
}

// --- inspection --------------------------------------------------------------

// InspectData feeds the inspection screen.
type InspectData struct {
	B         store.Booking
	Pairs     []PhotoPair
	OdoEnd    int
	Auto      []domain.Deduction
	WaiveKm   bool
	WaiveLate bool
	Manual    []domain.Deduction
	Settle    domain.Settlement
	Errors    map[string]string
	Kinds     []domain.DeductionKind
}

// PhotoPair puts a check-in photo next to the return photo of the same angle.
type PhotoPair struct {
	Angle         string
	Before, After *store.TripPhoto
}

func (h *Handlers) inspectData(r *http.Request, b store.Booking, fromForm bool) (InspectData, error) {
	d := InspectData{B: b, OdoEnd: b.OdoEnd, Errors: map[string]string{},
		Kinds: []domain.DeductionKind{domain.DeductFuel, domain.DeductDamage, domain.DeductCleaning}}
	photos, err := h.Store.TripPhotos(r.Context(), b.ID)
	if err != nil {
		return d, err
	}
	for _, a := range store.TripAngles {
		pp := PhotoPair{Angle: a}
		for i := range photos {
			if photos[i].Angle == a {
				if photos[i].Phase == "checkin" {
					pp.Before = &photos[i]
				} else {
					pp.After = &photos[i]
				}
			}
		}
		if pp.Before != nil || pp.After != nil {
			d.Pairs = append(d.Pairs, pp)
		}
	}
	if fromForm {
		if n := formInt(r, "odo_end"); n > 0 {
			d.OdoEnd = n
		}
		d.WaiveKm, d.WaiveLate = r.FormValue("waive_km") == "on", r.FormValue("waive_late") == "on"
		kinds, amounts, notes := r.Form["kind"], r.Form["amount"], r.Form["note"]
		for i := range kinds {
			k := domain.DeductionKind(kinds[i])
			if k != domain.DeductFuel && k != domain.DeductDamage && k != domain.DeductCleaning {
				continue
			}
			var amt domain.Paise
			if i < len(amounts) {
				r.Form.Set("_amt", amounts[i])
				amt = formRupees(r, "_amt")
			}
			note := ""
			if i < len(notes) {
				note = strings.TrimSpace(notes[i])
			}
			if amt > 0 || note != "" {
				d.Manual = append(d.Manual, domain.Deduction{Kind: k, Amount: amt, Note: note})
			}
		}
	}
	if b.OdoStart > 0 && d.OdoEnd > 0 && d.OdoEnd < b.OdoStart {
		d.Errors["odo_end"] = fmt.Sprintf("The return reading can't be below the pickup reading (%s km).", domain.Grouped(int64(b.OdoStart)))
		d.OdoEnd = b.OdoStart
	}
	for _, a := range store.AutoDeductions(b, d.OdoEnd) {
		if (a.Kind == domain.DeductExtraKm && d.WaiveKm) || (a.Kind == domain.DeductLate && d.WaiveLate) {
			a.Note = "Waived · " + a.Note
			a.Amount = 0
		}
		d.Auto = append(d.Auto, a)
	}
	all := append(append([]domain.Deduction{}, d.Auto...), d.Manual...)
	for _, m := range d.Manual {
		if m.Amount > 0 && m.Note == "" {
			d.Errors["note"] = "Add a short note for each deduction, so the renter knows why."
		}
	}
	d.Settle = domain.Settle(b.DepositHeld, all)
	return d, nil
}

func (h *Handlers) shopBooking(w http.ResponseWriter, r *http.Request) (store.Shop, store.Booking, bool) {
	shop, ok := h.providerShop(w, r)
	if !ok {
		return shop, store.Booking{}, false
	}
	b, err := h.Store.BookingByID(r.Context(), r.PathValue("id"))
	if err != nil || b.ShopID != shop.ID {
		h.notFound(w, r)
		return shop, b, false
	}
	return shop, b, true
}

// InspectPage shows the return inspection.
func (h *Handlers) InspectPage(w http.ResponseWriter, r *http.Request) {
	shop, b, ok := h.shopBooking(w, r)
	if !ok {
		return
	}
	if b.Status != domain.StatusReturned {
		http.Redirect(w, r, "/p/bookings/"+b.ID, http.StatusSeeOther)
		return
	}
	d, err := h.inspectData(r, b, false)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	h.console(w, r, http.StatusOK, "provider/inspect", "Inspect "+b.Code+" · "+shop.Name, "today", &shop, d)
}

// Inspect previews (htmx) or submits the inspection and releases the deposit.
func (h *Handlers) Inspect(w http.ResponseWriter, r *http.Request) {
	shop, b, ok := h.shopBooking(w, r)
	if !ok {
		return
	}
	r.ParseForm()
	d, err := h.inspectData(r, b, true)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	if IsHTMX(r) {
		h.Views.Fragment(w, http.StatusOK, "inspect-summary", d)
		return
	}
	if len(d.Errors) > 0 || b.Status != domain.StatusReturned {
		h.console(w, r, http.StatusUnprocessableEntity, "provider/inspect", "Inspect "+b.Code+" · "+shop.Name, "today", &shop, d)
		return
	}
	all := append(append([]domain.Deduction{}, d.Auto...), d.Manual...)
	settle, err := h.Store.Inspect(r.Context(), shop.ID, b.ID, d.OdoEnd, all)
	if err != nil {
		setFlash(w, Toast{Message: friendly(err), Tone: "bad"})
		http.Redirect(w, r, "/p", http.StatusSeeOther)
		return
	}
	setFlash(w, Toast{Message: fmt.Sprintf("Deposit released: %s back to %s.", settle.Refund, firstWord(b.RenterName)), Tone: "good"})
	http.Redirect(w, r, "/p", http.StatusSeeOther)
}
