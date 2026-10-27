package domain

import (
	"testing"
	"time"
)

func TestLateFee(t *testing.T) {
	due := time.Date(2026, 10, 22, 18, 0, 0, 0, IST)
	perHour := Rupees(100)
	tests := []struct {
		name string
		late time.Duration
		want Paise
	}{
		{"early return", -2 * time.Hour, 0},
		{"on time", 0, 0},
		{"inside grace", 29 * time.Minute, 0},
		{"exactly 30 min is still grace", 30 * time.Minute, 0},
		{"31 min → 1 h", 31 * time.Minute, Rupees(100)},
		{"60 min → 1 h", time.Hour, Rupees(100)},
		{"61 min → 2 h", 61 * time.Minute, Rupees(200)},
		{"95 min → 2 h", 95 * time.Minute, Rupees(200)},
		{"5 h", 5 * time.Hour, Rupees(500)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := LateFee(due, due.Add(tt.late), perHour)
			if d.Amount != tt.want {
				t.Errorf("late fee = %s, want %s", d.Amount, tt.want)
			}
			if d.Kind != DeductLate {
				t.Errorf("kind = %s", d.Kind)
			}
		})
	}
}

func TestExtraKm(t *testing.T) {
	tests := []struct {
		name             string
		start, end, incl int
		perKm, want      Paise
	}{
		{"under allowance", 12000, 12150, 200, Rupees(3), 0},
		{"exactly allowance", 12000, 12200, 200, Rupees(3), 0},
		{"60 km over", 12000, 12260, 200, Rupees(3), Rupees(180)},
		{"unlimited km", 100, 900, 0, Rupees(3), 0},
		{"odometer went backwards (typo) charges nothing", 12000, 11000, 200, Rupees(3), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtraKm(tt.start, tt.end, tt.incl, tt.perKm).Amount; got != tt.want {
				t.Errorf("extra km = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestSettle(t *testing.T) {
	deposit := Rupees(1500)
	tests := []struct {
		name         string
		items        []Deduction
		wantDeducted Paise
		wantRefund   Paise
		wantCapped   bool
		wantItems    int
	}{
		{"nothing to deduct", nil, 0, Rupees(1500), false, 0},
		{"zero items are dropped", []Deduction{{Kind: DeductLate}, {Kind: DeductExtraKm}}, 0, Rupees(1500), false, 0},
		{"extra km + fuel", []Deduction{
			{Kind: DeductExtraKm, Amount: Rupees(180)},
			{Kind: DeductFuel, Amount: Rupees(200), Note: "Half tank short"},
		}, Rupees(380), Rupees(1120), false, 2},
		{"exactly the deposit", []Deduction{{Kind: DeductDamage, Amount: Rupees(1500)}}, Rupees(1500), 0, false, 1},
		{"over the deposit is capped", []Deduction{
			{Kind: DeductDamage, Amount: Rupees(1400), Note: "Cracked mirror"},
			{Kind: DeductLate, Amount: Rupees(300)},
		}, Rupees(1500), 0, true, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Settle(deposit, tt.items)
			if s.Deducted != tt.wantDeducted || s.Refund != tt.wantRefund || s.Capped != tt.wantCapped {
				t.Errorf("got deducted %s refund %s capped %v; want %s %s %v",
					s.Deducted, s.Refund, s.Capped, tt.wantDeducted, tt.wantRefund, tt.wantCapped)
			}
			if len(s.Items) != tt.wantItems {
				t.Errorf("items = %d, want %d", len(s.Items), tt.wantItems)
			}
			if s.Deducted+s.Refund != deposit {
				t.Errorf("deducted + refund != deposit")
			}
		})
	}
}

func TestEffectiveDepositStatus(t *testing.T) {
	rel := time.Date(2026, 10, 20, 15, 0, 0, 0, IST)
	tests := []struct {
		name string
		s    DepositStatus
		now  time.Time
		want DepositStatus
	}{
		{"held stays held", DepositHeld, rel.Add(time.Hour), DepositHeld},
		{"released just now", DepositReleased, rel.Add(30 * time.Second), DepositReleased},
		{"released a minute ago → credited", DepositReleased, rel.Add(time.Minute), DepositCredited},
		{"credited stays credited", DepositCredited, rel, DepositCredited},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EffectiveDepositStatus(tt.s, rel, tt.now); got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}
