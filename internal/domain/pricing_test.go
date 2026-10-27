package domain

import (
	"errors"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 21, 9, 0, 0, 0, IST)

// sample rates: ₹60/h, ₹500/day, min 2 h
var sampleRates = Rates{Hourly: Rupees(60), Daily: Rupees(500), Deposit: Rupees(1500), KmLimitPerDay: 100, MinHours: 2}

func TestQuoteRent(t *testing.T) {
	tests := []struct {
		name     string
		dur      time.Duration
		wantRent Paise
		hours    int
		billable int
	}{
		{"1 h bumps to min 2 h", time.Hour, Rupees(120), 2, 1},
		{"5 h", 5 * time.Hour, Rupees(300), 5, 1},
		{"10 h capped at a day", 10 * time.Hour, Rupees(500), 10, 1},
		{"24 h is one day", 24 * time.Hour, Rupees(500), 24, 1},
		{"26 h 10 min rounds to 27 h", 26*time.Hour + 10*time.Minute, Rupees(680), 27, 2},
		{"47 h is a day plus a capped day", 47 * time.Hour, Rupees(1000), 47, 2},
		{"72 h is three days", 72 * time.Hour, Rupees(1500), 72, 3},
		{"1 minute still bills min hours", time.Minute, Rupees(120), 2, 1},
		{"61 minutes is 2 h", 61 * time.Minute, Rupees(120), 2, 1},
		{"exactly 30 days allowed", 30 * 24 * time.Hour, Rupees(15000), 720, 30},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := Quote(sampleRates, t0, t0.Add(tt.dur), nil, DefaultTaxBPS)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if q.Rent != tt.wantRent {
				t.Errorf("rent = %s, want %s", q.Rent, tt.wantRent)
			}
			if q.Hours != tt.hours {
				t.Errorf("hours = %d, want %d", q.Hours, tt.hours)
			}
			if q.BillableDays != tt.billable {
				t.Errorf("billable days = %d, want %d", q.BillableDays, tt.billable)
			}
			if q.KmIncluded != 100*tt.billable {
				t.Errorf("km included = %d, want %d", q.KmIncluded, 100*tt.billable)
			}
			if q.Total != q.Rent+q.Addons+q.Tax {
				t.Errorf("total %s != rent+addons+tax", q.Total)
			}
			if q.PayNow != q.Total+q.Deposit {
				t.Errorf("pay now %s != total+deposit", q.PayNow)
			}
		})
	}
}

func TestQuoteRejects(t *testing.T) {
	tests := []struct {
		name    string
		end     time.Time
		wantErr error
	}{
		{"end equals start", t0, ErrEndBeforeStart},
		{"end before start", t0.Add(-time.Hour), ErrEndBeforeStart},
		{"over 30 days", t0.Add(30*24*time.Hour + time.Minute), ErrTripTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Quote(sampleRates, t0, tt.end, nil, DefaultTaxBPS)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestQuoteTax(t *testing.T) {
	tests := []struct {
		name    string
		hourly  Paise
		hours   time.Duration
		wantTax Paise
	}{
		{"₹120 → ₹21.60", Rupees(60), 2 * time.Hour, 2160},
		{"₹33.33 → ₹6.00 (5.9994 rounds up)", 3333, time.Hour, 600},
		{"₹25.25 → ₹4.55 (half rounds up)", 2525, time.Hour, 455},
		{"₹25.24 → ₹4.54", 2524, time.Hour, 454},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Rates{Hourly: tt.hourly, Daily: Rupees(10000), MinHours: 1}
			q, err := Quote(r, t0, t0.Add(tt.hours), nil, DefaultTaxBPS)
			if err != nil {
				t.Fatal(err)
			}
			if q.Tax != tt.wantTax {
				t.Errorf("tax = %d paise, want %d", q.Tax, tt.wantTax)
			}
		})
	}
}

func TestQuoteAddons(t *testing.T) {
	helmet := Addon{ID: "h", Name: "Extra helmet", Price: Rupees(50), Per: PerTrip}
	seat := Addon{ID: "s", Name: "Child seat", Price: Rupees(150), Per: PerDay}

	q, err := Quote(sampleRates, t0, t0.Add(72*time.Hour), []Addon{helmet, seat}, DefaultTaxBPS)
	if err != nil {
		t.Fatal(err)
	}
	// helmet once (₹50) + child seat ₹150 × 3 days (₹450)
	if q.Addons != Rupees(500) {
		t.Errorf("addons = %s, want ₹500", q.Addons)
	}
	// (₹1,500 + ₹500) × 18% = ₹360
	if q.Tax != Rupees(360) {
		t.Errorf("tax = %s, want ₹360", q.Tax)
	}
	if q.Total != Rupees(2360) {
		t.Errorf("total = %s, want ₹2,360", q.Total)
	}
	if q.PayNow != Rupees(3860) {
		t.Errorf("pay now = %s, want ₹3,860", q.PayNow)
	}

	// Lines: rent, 2 add-ons, tax, total, deposit, pay now — in that order.
	wantKinds := []LineKind{LineRent, LineAddon, LineAddon, LineTax, LineTotal, LineDeposit, LinePayNow}
	if len(q.Lines) != len(wantKinds) {
		t.Fatalf("got %d lines, want %d", len(q.Lines), len(wantKinds))
	}
	for i, k := range wantKinds {
		if q.Lines[i].Kind != k {
			t.Errorf("line %d kind = %s, want %s", i, q.Lines[i].Kind, k)
		}
	}
	if q.Lines[2].Amount != Rupees(450) || q.Lines[2].Detail != "₹150 × 3 days" {
		t.Errorf("child seat line = %+v", q.Lines[2])
	}
}

func TestQuoteUnlimitedKm(t *testing.T) {
	r := Rates{Hourly: Rupees(25), Daily: Rupees(200), MinHours: 1}
	q, err := Quote(r, t0, t0.Add(48*time.Hour), nil, DefaultTaxBPS)
	if err != nil {
		t.Fatal(err)
	}
	if q.KmIncluded != 0 {
		t.Errorf("km included = %d, want 0 (unlimited)", q.KmIncluded)
	}
}

func TestDurationText(t *testing.T) {
	tests := []struct {
		d, h int
		want string
	}{
		{0, 5, "5 h"}, {1, 0, "1 day"}, {3, 0, "3 days"}, {1, 3, "1 day 3 h"}, {2, 23, "2 days 23 h"},
	}
	for _, tt := range tests {
		if got := DurationText(tt.d, tt.h); got != tt.want {
			t.Errorf("DurationText(%d,%d) = %q, want %q", tt.d, tt.h, got, tt.want)
		}
	}
}
