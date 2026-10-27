package domain

import (
	"fmt"
	"time"
)

// DepositStatus is a stop on the Deposit Tracker.
type DepositStatus string

const (
	DepositHeld       DepositStatus = "held"
	DepositInspection DepositStatus = "inspection"
	DepositReleased   DepositStatus = "released"
	DepositCredited   DepositStatus = "credited"
)

// DepositStops is the coin journey, in order.
var DepositStops = [4]DepositStatus{DepositHeld, DepositInspection, DepositReleased, DepositCredited}

// CreditDelay is how long after release a demo deposit shows as credited.
const CreditDelay = time.Minute

// LateGrace is how late a return can be before the late fee starts.
const LateGrace = 30 * time.Minute

// EffectiveDepositStatus applies the lazy demo rule: a released deposit
// counts as credited once CreditDelay has passed.
func EffectiveDepositStatus(s DepositStatus, releasedAt, now time.Time) DepositStatus {
	if s == DepositReleased && !releasedAt.IsZero() && !now.Before(releasedAt.Add(CreditDelay)) {
		return DepositCredited
	}
	return s
}

// DeductionKind is the reason money is kept from a deposit.
type DeductionKind string

const (
	DeductExtraKm  DeductionKind = "extra_km"
	DeductLate     DeductionKind = "late"
	DeductFuel     DeductionKind = "fuel"
	DeductDamage   DeductionKind = "damage"
	DeductCleaning DeductionKind = "cleaning"
)

// Label is the renter-facing name of a deduction kind.
func (k DeductionKind) Label() string {
	switch k {
	case DeductExtraKm:
		return "Extra kilometres"
	case DeductLate:
		return "Late return"
	case DeductFuel:
		return "Fuel"
	case DeductDamage:
		return "Damage"
	case DeductCleaning:
		return "Cleaning"
	}
	return string(k)
}

// Deduction is one itemised amount kept from the deposit.
type Deduction struct {
	Kind   DeductionKind
	Amount Paise
	Note   string
}

// LateFee charges each started hour once a return is more than LateGrace
// past the due time: ceil(lateMinutes / 60) × perHour.
func LateFee(due, returned time.Time, perHour Paise) Deduction {
	late := returned.Sub(due)
	if late <= LateGrace {
		return Deduction{Kind: DeductLate}
	}
	hours := int((late + time.Hour - 1) / time.Hour)
	mins := int(late / time.Minute)
	return Deduction{
		Kind:   DeductLate,
		Amount: Paise(hours) * perHour,
		Note:   fmt.Sprintf("Returned %s late · %s/h × %d", minutesText(mins), perHour, hours),
	}
}

// ExtraKm charges kilometres driven over the allowance. kmIncluded ≤ 0
// means unlimited kilometres.
func ExtraKm(odoStart, odoEnd, kmIncluded int, perKm Paise) Deduction {
	if kmIncluded <= 0 || odoEnd <= odoStart {
		return Deduction{Kind: DeductExtraKm}
	}
	over := odoEnd - odoStart - kmIncluded
	if over <= 0 {
		return Deduction{Kind: DeductExtraKm}
	}
	return Deduction{
		Kind:   DeductExtraKm,
		Amount: Paise(over) * perKm,
		Note:   fmt.Sprintf("%d km driven, %d km included · %s × %d km", odoEnd-odoStart, kmIncluded, perKm, over),
	}
}

// Settlement is what happens to a deposit after inspection.
type Settlement struct {
	Deposit   Paise
	Items     []Deduction // only non-zero deductions
	Requested Paise       // sum of items before the cap
	Deducted  Paise       // what is actually kept (≤ deposit)
	Refund    Paise       // deposit − deducted
	Capped    bool        // requested was more than the deposit
}

// Settle totals deductions, caps them at the deposit and works out the refund.
func Settle(deposit Paise, deductions []Deduction) Settlement {
	s := Settlement{Deposit: deposit}
	for _, d := range deductions {
		if d.Amount <= 0 {
			continue
		}
		s.Items = append(s.Items, d)
		s.Requested += d.Amount
	}
	s.Deducted = s.Requested
	if s.Deducted > deposit {
		s.Deducted = deposit
		s.Capped = true
	}
	s.Refund = deposit - s.Deducted
	return s
}

func minutesText(m int) string {
	if m < 60 {
		return fmt.Sprintf("%d min", m)
	}
	if m%60 == 0 {
		return fmt.Sprintf("%d h", m/60)
	}
	return fmt.Sprintf("%d h %d min", m/60, m%60)
}
