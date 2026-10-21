package handlers

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"sort"
	"time"

	"saarthi/internal/domain"
	"saarthi/internal/store"
)

// CardView is a vehicle card in a list.
type CardView struct {
	V         store.Vehicle
	Quote     *domain.PriceQuote // nil when no dates are chosen
	PerDay    domain.Paise       // all-in price for a day (no dates)
	Href      string
	InCompare bool
	FreeFrom  string // "Free from Thu 12 Nov" for suggestions

	Distance     float64 // km from DistanceFrom (search with a place)
	DistanceFrom string
}

// DistanceText is "1.2 km from Nakki Lake" (empty without a place).
func (c CardView) DistanceText() string {
	if c.DistanceFrom == "" {
		return ""
	}
	if c.Distance < 0.95 {
		return fmt.Sprintf("%.0f m from %s", math.Max(50, math.Round(c.Distance*20)*50), c.DistanceFrom)
	}
	return fmt.Sprintf("%.1f km from %s", c.Distance, c.DistanceFrom)
}

// dayQuote prices 24 hours from now, for "from ₹531 a day, all-in".
func dayQuote(v store.Vehicle, now time.Time) domain.Paise {
	start := domain.NextHour(now)
	q, err := domain.Quote(v.Rates(), start, start.Add(24*time.Hour), nil, domain.DefaultTaxBPS)
	if err != nil {
		return v.Daily
	}
	return q.Total
}

func (h *Handlers) cards(vs []store.Vehicle, start, end time.Time, inCompare func(string) bool) []CardView {
	now := h.Store.Now()
	out := make([]CardView, 0, len(vs))
	for _, v := range vs {
		c := CardView{V: v, Href: vehicleURL(v.ID, start, end), PerDay: dayQuote(v, now)}
		if inCompare != nil {
			c.InCompare = inCompare(v.ID)
		}
		if !start.IsZero() {
			if q, err := domain.Quote(v.Rates(), start, end, nil, domain.DefaultTaxBPS); err == nil {
				c.Quote = &q
			}
		}
		out = append(out, c)
	}
	return out
}

// Pin is a map marker.
type Pin struct {
	ID    string  `json:"id"`
	Lat   float64 `json:"lat"`
	Lng   float64 `json:"lng"`
	Label string  `json:"label"`
	Href  string  `json:"href"`
}

// pins spreads vehicles of the same shop in a small spiral so each price
// pin can be seen and tapped.
func pins(cards []CardView) string {
	seen := map[string]int{}
	out := make([]Pin, 0, len(cards))
	for _, c := range cards {
		key := fmt.Sprintf("%.4f,%.4f", c.V.ShopLat, c.V.ShopLng)
		n := seen[key]
		seen[key]++
		lat, lng := c.V.ShopLat, c.V.ShopLng
		if n > 0 {
			a := float64(n) * 2.4
			r := 0.0011 * math.Sqrt(float64(n))
			lat += r * math.Sin(a)
			lng += r * math.Cos(a) * 1.1
		}
		label := c.PerDay.String() + "/day"
		if c.Quote != nil {
			label = c.Quote.Total.String()
		}
		out = append(out, Pin{ID: c.V.ID, Lat: lat, Lng: lng, Label: label, Href: c.Href})
	}
	return toJSON(out)
}

// --- date range picker -------------------------------------------------------

// DRPView feeds the date range picker partial.
type DRPView struct {
	Start, End           string // LocalLayout values ("" when unset)
	StartLabel, EndLabel string
	Busy                 string // JSON list of busy IST dates
	Today                string
	Compact              bool
}

func newDRP(start, end time.Time, now time.Time, busyDays []string) DRPView {
	d := DRPView{Today: now.In(domain.IST).Format("2006-01-02"), Busy: toJSON(busyDays), StartLabel: "Add date", EndLabel: "Add date"}
	if busyDays == nil {
		d.Busy = "[]"
	}
	if !start.IsZero() {
		d.Start, d.End = domain.FormatLocal(start), domain.FormatLocal(end)
		d.StartLabel = domain.DayText(start) + ", " + domain.Clock(start)
		d.EndLabel = domain.DayText(end) + ", " + domain.Clock(end)
	}
	return d
}

// busyDays lists IST dates touched by busy ranges.
func busyDays(list []store.Busy) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, b := range list {
		for d := domain.StartOfDay(b.Start); d.Before(b.End); d = d.AddDate(0, 0, 1) {
			k := d.Format("2006-01-02")
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	sort.Strings(out)
	return out
}

// --- price box ---------------------------------------------------------------

// AddonView is one add-on toggle.
type AddonView struct {
	ID, Name, Detail string
	Price            domain.Paise
	Checked          bool
}

// ConflictView explains why the dates don't work and offers new ones.
type ConflictView struct {
	Text       string
	SuggestURL string
	Suggest    string
}

// QuoteView is the price box / checkout bill.
type QuoteView struct {
	V        store.Vehicle
	DRP      DRPView
	Quote    *domain.PriceQuote
	Err      string
	Conflict *ConflictView
	Addons   []AddonView
	BookURL  string
	DayTotal domain.Paise // all-in price of one day, shown before dates are picked
	Action   string       // where the dates form refreshes ("/v/{id}/quote")
	LoggedIn bool
	IsRenter bool
}

// AddonQuery is the add-on part of a URL.
func (q QuoteView) AddonQuery() string {
	v := url.Values{}
	for _, a := range q.Addons {
		if a.Checked {
			v.Add("addon", a.ID)
		}
	}
	return v.Encode()
}

func (h *Handlers) quoteView(ctx context.Context, v store.Vehicle, start, end time.Time, picked []string, u *store.User) (QuoteView, error) {
	now := h.Store.Now()
	qv := QuoteView{V: v, Action: "/v/" + v.ID + "/quote", LoggedIn: u != nil, IsRenter: u.IsRenter(), DayTotal: dayQuote(v, now)}

	addons, err := h.Store.AddonsFor(ctx, v)
	if err != nil {
		return qv, err
	}
	var chosen []domain.Addon
	for _, a := range addons {
		av := AddonView{ID: a.ID, Name: a.Name, Price: a.Price, Checked: contains(picked, a.ID), Detail: "once per trip"}
		if a.Per == domain.PerDay {
			av.Detail = "per day"
		}
		if av.Checked {
			chosen = append(chosen, a.Addon)
		}
		qv.Addons = append(qv.Addons, av)
	}

	// busy days for the next ~4 months feed the picker's struck-through days
	horizon := domain.StartOfDay(now)
	busyMap, err := h.Store.Busy(ctx, []string{v.ID}, horizon, horizon.AddDate(0, 4, 0))
	if err != nil {
		return qv, err
	}
	busyList := busyMap[v.ID]
	qv.DRP = newDRP(start, end, now, busyDays(busyList))

	if start.IsZero() {
		return qv, nil
	}
	q, err := domain.Quote(v.Rates(), start, end, chosen, domain.DefaultTaxBPS)
	if err != nil {
		qv.Err = friendly(err)
		return qv, nil
	}
	if start.Before(now.Add(-10 * time.Minute)) {
		qv.Err = friendly(store.ErrInPast)
		return qv, nil
	}
	qv.Quote = &q
	for _, b := range busyList {
		if domain.Overlaps(b.Start, b.End, start, end) {
			qv.Conflict = conflict(b, busyList, v.ID, start, end)
			break
		}
	}
	bq := url.Values{"start": {domain.FormatLocal(start)}, "end": {domain.FormatLocal(end)}}
	for _, a := range qv.Addons {
		if a.Checked {
			bq.Add("addon", a.ID)
		}
	}
	qv.BookURL = "/checkout/" + v.ID + "?" + bq.Encode()
	return qv, nil
}

// conflict suggests the first start (same clock, same length) after the
// clash that is free: "Booked 9 to 11 Nov. Pick 12 Nov?"
func conflict(b store.Busy, all []store.Busy, vehicleID string, start, end time.Time) *ConflictView {
	dur := end.Sub(start)
	clock := start.In(domain.IST)
	cand := b.End
	for i := 0; i < 20; i++ {
		day := domain.StartOfDay(cand)
		s := time.Date(day.Year(), day.Month(), day.Day(), clock.Hour(), clock.Minute(), 0, 0, domain.IST)
		if s.Before(cand) {
			s = s.AddDate(0, 0, 1)
		}
		e := s.Add(dur)
		clash := false
		for _, o := range all {
			if domain.Overlaps(o.Start, o.End, s, e) {
				clash, cand = true, o.End
				break
			}
		}
		if !clash {
			return &ConflictView{
				Text:       busyText(b) + ".",
				Suggest:    fmt.Sprintf("Pick %d %s?", s.Day(), s.Format("Jan")),
				SuggestURL: vehicleURL(vehicleID, s, e),
			}
		}
	}
	return &ConflictView{Text: busyText(b) + ". Try other dates."}
}

// --- availability month -------------------------------------------------------

// CalDay is one cell of the availability calendar.
type CalDay struct {
	Day                                 int
	InMonth, Past, Today, Busy, Partial bool
}

// CalView is a month of availability.
type CalView struct {
	VehicleID, Title string
	PrevURL, NextURL string
	Weeks            [][]CalDay
}

func calendarMonth(vehicleID string, month time.Time, now time.Time, busyList []store.Busy) CalView {
	first := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, domain.IST)
	cv := CalView{VehicleID: vehicleID, Title: first.Format("January 2006")}
	thisMonth := time.Date(now.In(domain.IST).Year(), now.In(domain.IST).Month(), 1, 0, 0, 0, 0, domain.IST)
	if first.After(thisMonth) {
		cv.PrevURL = fmt.Sprintf("/v/%s/calendar?month=%s", vehicleID, first.AddDate(0, -1, 0).Format("2006-01"))
	}
	if first.Before(thisMonth.AddDate(0, 6, 0)) {
		cv.NextURL = fmt.Sprintf("/v/%s/calendar?month=%s", vehicleID, first.AddDate(0, 1, 0).Format("2006-01"))
	}
	offset := (int(first.Weekday()) + 6) % 7 // Monday first
	day := first.AddDate(0, 0, -offset)
	today := domain.StartOfDay(now)
	for w := 0; w < 6; w++ {
		var week []CalDay
		for i := 0; i < 7; i++ {
			c := CalDay{Day: day.Day(), InMonth: day.Month() == first.Month(), Past: day.Before(today), Today: day.Equal(today)}
			dayEnd := day.AddDate(0, 0, 1)
			var covered time.Duration
			for _, b := range busyList {
				s, e := b.Start, b.End
				if s.Before(day) {
					s = day
				}
				if e.After(dayEnd) {
					e = dayEnd
				}
				if e.After(s) {
					covered += e.Sub(s)
				}
			}
			c.Busy = covered >= 12*time.Hour
			c.Partial = !c.Busy && covered > 0
			week = append(week, c)
			day = dayEnd
		}
		cv.Weeks = append(cv.Weeks, week)
		if day.Month() != first.Month() && w >= 3 {
			break
		}
	}
	return cv
}

// --- deposit tracker -------------------------------------------------------

// DepositView feeds the Deposit Tracker.
type DepositView struct {
	Code                   string
	Stop                   int // 0 held, 1 inspection, 2 released, 3 credited; -1 none
	Held, Deducted, Refund domain.Paise
	Items                  []domain.Deduction
	HeldAt, InspAt, RelAt  time.Time
	CredAt                 time.Time
	Live                   bool // poll until credited
	Note                   string
	Labels                 [4]string
}

func (h *Handlers) depositView(ctx context.Context, b store.Booking) (DepositView, error) {
	now := h.Store.Now()
	dv := DepositView{Code: b.Code, Stop: -1, Labels: [4]string{"Held", "Inspection", "Released", "Credited"}}
	if b.DepositStatus == "" {
		return dv, nil
	}
	dv.Held, dv.Deducted, dv.Refund = b.DepositHeld, b.DepositDeducted, b.Refund()
	switch b.DepositState(now) {
	case domain.DepositHeld:
		dv.Stop = 0
		dv.Note = fmt.Sprintf("Your %s is held during the trip and returned after inspection.", b.DepositHeld)
	case domain.DepositInspection:
		dv.Stop = 1
		dv.Note = "The shop is checking the vehicle against your proof photos."
	case domain.DepositReleased:
		dv.Stop, dv.Live = 2, true
		dv.Note = fmt.Sprintf("%s is on its way back to you.", dv.Refund)
	case domain.DepositCredited:
		dv.Stop = 3
		dv.Note = fmt.Sprintf("%s is back in your account.", dv.Refund)
	}
	if b.Status == domain.StatusCancelled || b.Status == domain.StatusDeclined {
		dv.Note = fmt.Sprintf("Refunded in full: %s.", b.PayNow())
	}
	dv.HeldAt, dv.InspAt, dv.RelAt = b.CreatedAt, b.ReturnedAt, b.DepositReleasedAt
	dv.CredAt = b.DepositCreditedAt
	if dv.CredAt.IsZero() && dv.Stop == 3 {
		dv.CredAt = b.DepositReleasedAt.Add(domain.CreditDelay)
	}
	if dv.Deducted > 0 {
		items, err := h.Store.Deductions(ctx, b.ID)
		if err != nil {
			return dv, err
		}
		dv.Items = items
	}
	return dv, nil
}

// --- next step ---------------------------------------------------------------

// NextStep is the one thing the renter should do now.
type NextStep struct {
	Title, Text string
	Label, URL  string
	Tone        string
	Sheet       string // id of a sheet the button opens instead of a link
}

func nextStep(b store.Booking, now time.Time) NextStep {
	switch b.Status {
	case domain.StatusPendingPayment:
		return NextStep{Title: "Finish payment", Tone: "warn",
			Text:  fmt.Sprintf("Your slot is held until %s. After that it goes back on the market.", domain.Clock(b.PayBy())),
			Label: "Pay " + b.PayNow().String(), Sheet: "pay-sheet"}
	case domain.StatusAwaitingShop:
		return NextStep{Title: "Waiting for the shop", Tone: "warn",
			Text:  b.ShopName + " confirms this one personally, usually in under 15 minutes. Your money comes back in full if they can't.",
			Label: "Message the shop", URL: b.WhatsApp()}
	case domain.StatusConfirmed:
		return NextStep{Title: "You're booked", Tone: "good",
			Text:  fmt.Sprintf("Pickup %s, %s at %s. Bring your driving licence and one photo ID.", domain.DayText(b.Start), domain.Clock(b.Start), b.ShopName),
			Label: "Show Pickup Pass", URL: "/trips/" + b.Code + "/pass"}
	case domain.StatusReady:
		if b.CheckedIn() {
			return NextStep{Title: "Photos are in. Ready to ride.", Tone: "road",
				Text: "Swipe to start the trip once you have the keys.", Label: "Start the trip", URL: "/trips/" + b.Code + "/checkin"}
		}
		return NextStep{Title: "Your ride is ready", Tone: "road",
			Text:  "Take four quick photos before you leave, so old scratches are on record.",
			Label: "Start check-in", URL: "/trips/" + b.Code + "/checkin"}
	case domain.StatusActive:
		left := b.End.Sub(now)
		text := "Due back " + domain.DayText(b.End) + ", " + domain.Clock(b.End) + "."
		if left < 0 {
			text = "You're " + domain.DurationWords(-left) + " past the return time. Late fee starts after 30 min grace."
		} else if left < 6*time.Hour {
			text = "Due back in " + domain.DurationWords(left) + ", at " + domain.Clock(b.End) + "."
		}
		return NextStep{Title: "On the road", Tone: "road", Text: text, Label: "Return the vehicle", URL: "/trips/" + b.Code + "/return"}
	case domain.StatusReturned:
		return NextStep{Title: "The shop is inspecting", Tone: "warn",
			Text: "They compare the vehicle with your photos. Your deposit is released right after."}
	case domain.StatusClosed:
		if !b.Reviewed {
			return NextStep{Title: "How was the ride?", Tone: "good", Text: "A quick rating helps the next renter and the shop.",
				Label: "Rate this trip", Sheet: "review-sheet"}
		}
		return NextStep{Title: "Trip closed", Tone: "good", Text: "Thanks for riding with " + b.ShopName + "."}
	case domain.StatusCancelled:
		return NextStep{Title: "Cancelled", Tone: "muted", Text: "Nothing was kept. Plans change.", Label: "Find another ride", URL: "/"}
	case domain.StatusDeclined:
		return NextStep{Title: "The shop couldn't take it", Tone: "bad", Text: "Your money is refunded in full.", Label: "Find another ride", URL: "/"}
	case domain.StatusExpired:
		return NextStep{Title: "Payment timed out", Tone: "muted", Text: "The 15-minute hold ran out. Nothing was charged.", Label: "Try again", URL: "/v/" + b.VehicleID}
	}
	return NextStep{}
}
