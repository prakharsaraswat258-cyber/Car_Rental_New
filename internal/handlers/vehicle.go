package handlers

import (
	"errors"
	"net/http"
	"time"

	"saarthi/internal/domain"
	"saarthi/internal/store"
)

// VehicleData feeds the vehicle detail page.
type VehicleData struct {
	V         store.Vehicle
	Reviews   []store.Review
	Box       QuoteView
	Cal       CalView
	Map       string
	InCompare bool
	Next      string // where login sends you back to
	Rules     []Rule
	Included  []string
	Gallery   []GalleryItem
}

// GalleryItem is a photo, or a line-art view when the shop hasn't
// uploaded photos yet.
type GalleryItem struct {
	Photo, Art, Label string
	Mirror, Turn      bool
}

func gallery(v store.Vehicle) []GalleryItem {
	var out []GalleryItem
	for i, p := range v.Photos {
		if i == 5 {
			break
		}
		out = append(out, GalleryItem{Photo: p.URL(), Label: v.Name() + " photo " + domain.Grouped(int64(i+1))})
	}
	if len(out) > 0 {
		return out
	}
	a := v.Art()
	return []GalleryItem{
		{Art: a, Label: "Side view"},
		{Art: frontArt(a), Label: "Front"},
		{Art: a, Label: "Other side", Mirror: true},
		{Art: a, Label: "Three-quarter view", Turn: true},
		{Art: frontArt(a), Label: "Rear"},
	}
}

// Rule is one plain-words rental rule.
type Rule struct{ Icon, Text, Tip string }

func (h *Handlers) loadVehicle(w http.ResponseWriter, r *http.Request) (store.Vehicle, bool) {
	v, err := h.Store.VehicleByID(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		h.notFound(w, r)
		return v, false
	}
	if err != nil {
		h.serverError(w, r, err)
		return v, false
	}
	u := UserFrom(r.Context())
	owner := u.IsProvider() && v.ShopStatus != "" && h.ownsShop(r, v.ShopID)
	if !v.Searchable() && !owner && !u.IsAdmin() {
		h.notFound(w, r)
		return v, false
	}
	return v, true
}

func (h *Handlers) ownsShop(r *http.Request, shopID string) bool {
	u := UserFrom(r.Context())
	if u == nil {
		return false
	}
	s, err := h.Store.ShopByOwner(r.Context(), u.ID)
	return err == nil && s.ID == shopID
}

// Vehicle is the detail page.
func (h *Handlers) Vehicle(w http.ResponseWriter, r *http.Request) {
	v, ok := h.loadVehicle(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	start, end, _ := parseRange(r)
	box, err := h.quoteView(ctx, v, start, end, r.URL.Query()["addon"], UserFrom(ctx))
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	reviews, err := h.Store.VehicleReviews(ctx, v.ID, 3)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	now := h.Store.Now()
	busy, err := h.Store.Busy(ctx, []string{v.ID}, domain.StartOfDay(now).AddDate(0, 0, -7), domain.StartOfDay(now).AddDate(0, 2, 0))
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	month := now
	if !start.IsZero() {
		month = start
	}
	p := h.page(w, r, v.Name()+" in "+v.CityName+" · Saarthi", "explore", nil)
	p.Body = "has-bookbar"
	p.Data = VehicleData{
		V: v, Reviews: reviews, Box: box,
		Cal:       calendarMonth(v.ID, month, now, busy[v.ID]),
		Map:       toJSON([]Pin{{ID: v.ID, Lat: v.ShopLat, Lng: v.ShopLng, Label: v.ShopName}}),
		InCompare: p.InCompare(v.ID),
		Gallery:   gallery(v),
		Next:      r.URL.RequestURI(),
		Rules:     rules(v),
		Included:  included(v),
	}
	h.Views.Page(w, r, http.StatusOK, "vehicle", p)
}

func rules(v store.Vehicle) []Rule {
	var out []Rule
	out = append(out, Rule{Icon: "shield-check", Text: "Refundable deposit of " + v.Deposit.String() + ", returned after inspection.",
		Tip: "Held during the trip and returned after inspection"})
	if v.KmPerDay > 0 {
		out = append(out, Rule{Icon: "gauge", Text: domain.Grouped(int64(v.KmPerDay)) + " km a day included. Extra km cost " + v.ExtraKm.String() + " each.",
			Tip: "Free kilometres included; extra km cost " + v.ExtraKm.String() + " each"})
	} else {
		out = append(out, Rule{Icon: "gauge", Text: "Ride as far as you like. No km limit."})
	}
	out = append(out, Rule{Icon: "clock", Text: "Late returns cost " + v.LateFee.String() + " an hour, after 30 minutes of grace.",
		Tip: "Charged per hour after your return time, after 30 min grace"})
	if v.MinHours > 1 {
		out = append(out, Rule{Icon: "hourglass", Text: "Minimum booking is " + domain.Grouped(int64(v.MinHours)) + " hours."})
	}
	if v.Type != "cycle" {
		out = append(out, Rule{Icon: "key-round", Text: "Bring your driving licence and one photo ID. The shop does the licence check at pickup."})
	}
	out = append(out, Rule{Icon: "camera", Text: "Take " + domain.ProofPhrase(v.Class()) + " at pickup and return.", Tip: "Photos are timestamped so nobody can blame you later"})
	return out
}

func included(v store.Vehicle) []string {
	out := append([]string(nil), v.Features...)
	if v.KmPerDay > 0 {
		out = append(out, domain.Grouped(int64(v.KmPerDay))+" km a day")
	} else {
		out = append(out, "Unlimited km")
	}
	out = append(out, "GST included in the total", "Roadside help from the shop")
	return out
}

// Quote re-renders the price box when dates or add-ons change.
func (h *Handlers) Quote(w http.ResponseWriter, r *http.Request) {
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
	h.Views.Fragment(w, http.StatusOK, "price-box", box)
}

// Calendar renders one month of availability.
func (h *Handlers) Calendar(w http.ResponseWriter, r *http.Request) {
	v, ok := h.loadVehicle(w, r)
	if !ok {
		return
	}
	now := h.Store.Now()
	month, err := time.ParseInLocation("2006-01", r.URL.Query().Get("month"), domain.IST)
	if err != nil {
		month = now
	}
	from := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, domain.IST).AddDate(0, 0, -7)
	busy, err := h.Store.Busy(r.Context(), []string{v.ID}, from, from.AddDate(0, 0, 50))
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	h.Views.Fragment(w, http.StatusOK, "cal-month", calendarMonth(v.ID, month, now, busy[v.ID]))
}

// --- compare -----------------------------------------------------------------

// CompareRow is one line of the compare table.
type CompareRow struct {
	Label string
	Cells []CompareCell
}

// CompareCell is one value; Best marks the best value in the row.
type CompareCell struct {
	Text string
	Best bool
}

// CompareData feeds the compare sheet.
type CompareData struct {
	Cards    []CardView
	Rows     []CompareRow
	HasDates bool
	Query    string
}

// CompareAdd puts a vehicle in the tray (max 3).
func (h *Handlers) CompareAdd(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ids := compareIDs(r)
	if !contains(ids, id) {
		if old := r.FormValue("replace"); old != "" {
			var kept []string
			for _, x := range ids {
				if x != old {
					kept = append(kept, x)
				}
			}
			ids = kept
		}
		if len(ids) >= CompareMax {
			vs, _ := h.Store.VehiclesByIDs(r.Context(), ids)
			nv, err := h.Store.VehicleByID(r.Context(), id)
			if err != nil {
				h.notFound(w, r)
				return
			}
			if r.Header.Get("X-Requested-With") == "fetch" {
				w.Header().Set("X-Compare-Full", "1")
				h.Views.Fragment(w, http.StatusConflict, "compare-full", map[string]any{"Current": vs, "New": nv})
				return
			}
			setFlash(w, Toast{Message: "Compare holds 3. Remove one first.", Tone: "warn"})
			http.Redirect(w, r, safeBack(r), http.StatusSeeOther)
			return
		}
		if _, err := h.Store.VehicleByID(r.Context(), id); err != nil {
			h.notFound(w, r)
			return
		}
		ids = append(ids, id)
	}
	setCompareIDs(w, ids)
	h.compareReply(w, r, ids)
}

// CompareRemove takes a vehicle out of the tray.
func (h *Handlers) CompareRemove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var ids []string
	for _, x := range compareIDs(r) {
		if x != id {
			ids = append(ids, x)
		}
	}
	setCompareIDs(w, ids)
	h.compareReply(w, r, ids)
}

func (h *Handlers) compareReply(w http.ResponseWriter, r *http.Request, ids []string) {
	if r.Header.Get("X-Requested-With") != "fetch" && r.Header.Get("HX-Request") != "true" {
		http.Redirect(w, r, safeBack(r), http.StatusSeeOther)
		return
	}
	vs, err := h.Store.VehiclesByIDs(r.Context(), ids)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	w.Header().Set("X-Compare-IDs", toJSON(ids))
	h.Views.Fragment(w, http.StatusOK, "compare-tray", Page{Compare: vs})
}

// Compare shows the side-by-side sheet (or page without JS).
func (h *Handlers) Compare(w http.ResponseWriter, r *http.Request) {
	ids := compareIDs(r)
	vs, err := h.Store.VehiclesByIDs(r.Context(), ids)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	start, end, hasDates := parseRange(r)
	if !hasDates {
		start, end = time.Time{}, time.Time{}
	}
	cards := h.cards(vs, start, end, func(string) bool { return true })
	data := CompareData{Cards: cards, Rows: compareRows(cards), HasDates: hasDates, Query: r.URL.RawQuery}
	if IsHTMX(r) {
		h.Views.Fragment(w, http.StatusOK, "compare-sheet", data)
		return
	}
	h.render(w, r, http.StatusOK, "compare", "Compare · Saarthi", "explore", data)
}

func compareRows(cards []CardView) []CompareRow {
	type val struct {
		text string
		n    float64
	}
	row := func(label string, better string, get func(CardView) val) CompareRow {
		cr := CompareRow{Label: label}
		vals := make([]val, len(cards))
		best := -1.0
		for i, c := range cards {
			vals[i] = get(c)
			if better == "" {
				continue
			}
			if best < 0 || (better == "low" && vals[i].n < best) || (better == "high" && vals[i].n > best) {
				best = vals[i].n
			}
		}
		for _, v := range vals {
			cr.Cells = append(cr.Cells, CompareCell{Text: v.text, Best: better != "" && len(cards) > 1 && v.n == best})
		}
		return cr
	}
	var rows []CompareRow
	rows = append(rows, row("Total", "low", func(c CardView) val {
		if c.Quote != nil {
			return val{c.Quote.Total.String(), float64(c.Quote.Total)}
		}
		return val{c.PerDay.String() + " a day", float64(c.PerDay)}
	}))
	rows = append(rows,
		row("Refundable deposit", "low", func(c CardView) val { return val{c.V.Deposit.String(), float64(c.V.Deposit)} }),
		row("Free km a day", "high", func(c CardView) val {
			if c.V.KmPerDay == 0 {
				return val{"Unlimited", 1e9}
			}
			return val{domain.Grouped(int64(c.V.KmPerDay)) + " km", float64(c.V.KmPerDay)}
		}),
		row("Extra km", "low", func(c CardView) val { return val{c.V.ExtraKm.String() + "/km", float64(c.V.ExtraKm)} }),
		row("Late fee", "low", func(c CardView) val { return val{c.V.LateFee.String() + "/h", float64(c.V.LateFee)} }),
		row("Rating", "high", func(c CardView) val {
			return val{c.V.RatingText(), c.V.RatingAvg}
		}),
		row("Type", "", func(c CardView) val { return val{text: domain.TypeLabel(c.V.Type)} }),
		row("Fuel", "", func(c CardView) val { return val{text: titleCase(c.V.Fuel)} }),
		row("Gears", "", func(c CardView) val { return val{text: titleCase(c.V.Trans)} }),
		row("Seats", "high", func(c CardView) val { return val{domain.Grouped(int64(c.V.Seats)), float64(c.V.Seats)} }),
		row("Shop", "", func(c CardView) val { return val{text: c.V.ShopName + ", " + c.V.Place()} }),
	)
	return rows
}

func titleCase(s string) string {
	switch s {
	case "", "none":
		return "—"
	}
	return string(s[0]-32) + s[1:]
}
