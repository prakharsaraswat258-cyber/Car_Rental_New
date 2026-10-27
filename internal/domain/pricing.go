package domain

import (
	"errors"
	"fmt"
	"time"
)

// DefaultTaxBPS is GST on rent and add-ons, in basis points (18%).
const DefaultTaxBPS = 1800

// MaxTripDays is the longest trip a renter can book.
const MaxTripDays = 30

var (
	ErrEndBeforeStart = errors.New("return time must be after pickup time")
	ErrTripTooLong    = fmt.Errorf("trips can be at most %d days", MaxTripDays)
)

// Rates is the part of a vehicle its price depends on.
type Rates struct {
	Hourly        Paise
	Daily         Paise
	Deposit       Paise
	KmLimitPerDay int // 0 = unlimited kilometres
	MinHours      int
}

// AddonPer says whether an add-on is charged once or per billable day.
type AddonPer string

const (
	PerTrip AddonPer = "trip"
	PerDay  AddonPer = "day"
)

// Addon is an optional extra a shop offers (helmet, child seat...).
type Addon struct {
	ID    string
	Name  string
	Price Paise
	Per   AddonPer
}

// LineKind tags each row of an itemised bill.
type LineKind string

const (
	LineRent    LineKind = "rent"
	LineAddon   LineKind = "addon"
	LineTax     LineKind = "tax"
	LineTotal   LineKind = "total"
	LineDeposit LineKind = "deposit"
	LinePayNow  LineKind = "pay_now"
)

// Line is one row of an itemised bill. Every price box and bill renders
// these lines, so all screens show the same numbers.
type Line struct {
	Kind   LineKind `json:"kind"`
	Label  string   `json:"label"`
	Detail string   `json:"detail,omitempty"`
	Amount Paise    `json:"amount"`
}

// PriceQuote is the full price of a trip.
type PriceQuote struct {
	Start, End   time.Time
	Hours        int // billed hours, at least MinHours
	Days         int // whole days inside Hours
	RemHours     int // hours left over after whole days
	BillableDays int // days used for km allowance and per-day add-ons
	KmIncluded   int // 0 = unlimited

	Rent    Paise
	Addons  Paise
	Tax     Paise
	Total   Paise // rent + add-ons + tax: "the price is the whole price"
	Deposit Paise // refundable, shown separately
	PayNow  Paise // total + deposit

	Lines []Line
}

// Quote prices a trip from start to end:
//
//	hours = ceil(duration / 1h), at least MinHours
//	rent  = days×daily + min(remHours×hourly, daily)
//	tax   = round((rent + addons) × taxBPS / 10000)
func Quote(r Rates, start, end time.Time, addons []Addon, taxBPS int64) (PriceQuote, error) {
	d := end.Sub(start)
	if d <= 0 {
		return PriceQuote{}, ErrEndBeforeStart
	}
	if d > MaxTripDays*24*time.Hour {
		return PriceQuote{}, ErrTripTooLong
	}

	hours := int((d + time.Hour - 1) / time.Hour)
	if hours < r.MinHours {
		hours = r.MinHours
	}
	days, rem := hours/24, hours%24
	remCost := Paise(rem) * r.Hourly
	if remCost > r.Daily {
		remCost = r.Daily
	}
	rent := Paise(days)*r.Daily + remCost

	billable := (hours + 23) / 24
	if billable < 1 {
		billable = 1
	}

	q := PriceQuote{
		Start: start, End: end,
		Hours: hours, Days: days, RemHours: rem,
		BillableDays: billable,
		KmIncluded:   r.KmLimitPerDay * billable,
		Rent:         rent,
		Deposit:      r.Deposit,
	}
	q.Lines = append(q.Lines, Line{
		Kind:   LineRent,
		Label:  "Rent · " + DurationText(days, rem),
		Detail: rentDetail(r, days, rem, remCost),
		Amount: rent,
	})

	for _, a := range addons {
		amt, detail := a.Price, "once"
		if a.Per == PerDay {
			amt = a.Price * Paise(billable)
			detail = fmt.Sprintf("%s × %d %s", a.Price, billable, plural(billable, "day", "days"))
		}
		q.Addons += amt
		q.Lines = append(q.Lines, Line{Kind: LineAddon, Label: a.Name, Detail: detail, Amount: amt})
	}

	q.Tax = ApplyBPS(q.Rent+q.Addons, taxBPS)
	q.Total = q.Rent + q.Addons + q.Tax
	q.PayNow = q.Total + q.Deposit

	q.Lines = append(q.Lines,
		Line{Kind: LineTax, Label: "GST " + bpsText(taxBPS), Amount: q.Tax},
		Line{Kind: LineTotal, Label: "Total", Amount: q.Total},
		Line{Kind: LineDeposit, Label: "Refundable deposit", Detail: "returned after inspection", Amount: q.Deposit},
		Line{Kind: LinePayNow, Label: "Pay now", Amount: q.PayNow},
	)
	return q, nil
}

// ApplyBPS returns amount × bps / 10000, rounded half-up to the paisa.
func ApplyBPS(amount Paise, bps int64) Paise {
	v := int64(amount) * bps
	if v < 0 {
		return -Paise((-v + 5000) / 10000)
	}
	return Paise((v + 5000) / 10000)
}

// DurationText renders "2 h", "1 day", "1 day 3 h", "3 days".
func DurationText(days, hours int) string {
	switch {
	case days == 0:
		return fmt.Sprintf("%d h", hours)
	case hours == 0:
		return fmt.Sprintf("%d %s", days, plural(days, "day", "days"))
	default:
		return fmt.Sprintf("%d %s %d h", days, plural(days, "day", "days"), hours)
	}
}

func rentDetail(r Rates, days, rem int, remCost Paise) string {
	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%s/day × %d", r.Daily, days))
	}
	if rem > 0 {
		if remCost == r.Daily && Paise(rem)*r.Hourly > r.Daily {
			parts = append(parts, fmt.Sprintf("%d h capped at %s", rem, r.Daily))
		} else {
			parts = append(parts, fmt.Sprintf("%s/h × %d", r.Hourly, rem))
		}
	}
	if len(parts) == 2 {
		return parts[0] + " + " + parts[1]
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return ""
}

func bpsText(bps int64) string {
	if bps%100 == 0 {
		return fmt.Sprintf("%d%%", bps/100)
	}
	return fmt.Sprintf("%.2f%%", float64(bps)/100)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
