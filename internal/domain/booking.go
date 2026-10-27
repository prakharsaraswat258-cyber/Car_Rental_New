package domain

import "time"

// Status is where a booking is in its life.
type Status string

const (
	StatusPendingPayment Status = "pending_payment"
	StatusAwaitingShop   Status = "awaiting_shop"
	StatusConfirmed      Status = "confirmed"
	StatusReady          Status = "ready"
	StatusActive         Status = "active"
	StatusReturned       Status = "returned"
	StatusClosed         Status = "closed"
	StatusCancelled      Status = "cancelled"
	StatusDeclined       Status = "declined"
	StatusExpired        Status = "expired"
)

// PaymentHold is how long an unpaid booking holds its slot.
const PaymentHold = 15 * time.Minute

// Actor is who fires a transition.
type Actor string

const (
	ActorRenter   Actor = "renter"
	ActorProvider Actor = "provider"
	ActorSystem   Actor = "system"
)

type transition struct {
	from, to Status
	by       Actor
}

// transitions is the whole booking state machine. Anything not listed
// here is forbidden.
var transitions = []transition{
	{StatusPendingPayment, StatusConfirmed, ActorRenter},    // demo pay success, instant book on
	{StatusPendingPayment, StatusAwaitingShop, ActorRenter}, // demo pay success, instant book off
	{StatusPendingPayment, StatusCancelled, ActorRenter},
	{StatusPendingPayment, StatusExpired, ActorSystem}, // 15 min passed, checked lazily

	{StatusAwaitingShop, StatusConfirmed, ActorProvider}, // accept
	{StatusAwaitingShop, StatusDeclined, ActorProvider},  // decline
	{StatusAwaitingShop, StatusCancelled, ActorRenter},

	{StatusConfirmed, StatusReady, ActorProvider}, // mark ready for pickup
	{StatusConfirmed, StatusCancelled, ActorRenter},

	{StatusReady, StatusActive, ActorRenter}, // swipe to start, after check-in photos
	{StatusReady, StatusCancelled, ActorRenter},

	{StatusActive, StatusReturned, ActorRenter}, // swipe to end, after return photos

	{StatusReturned, StatusClosed, ActorProvider}, // inspection done, deposit released
}

// CanTransition reports whether actor may move a booking from → to.
func CanTransition(from, to Status, by Actor) bool {
	for _, t := range transitions {
		if t.from == from && t.to == to && t.by == by {
			return true
		}
	}
	return false
}

// Terminal statuses never change again.
func (s Status) Terminal() bool {
	switch s {
	case StatusClosed, StatusCancelled, StatusDeclined, StatusExpired:
		return true
	}
	return false
}

// HoldsSlot reports whether a booking in this status keeps its vehicle
// unavailable for its dates. pending_payment holds only while unexpired;
// callers expire stale rows first (see PaymentHold).
func (s Status) HoldsSlot() bool {
	switch s {
	case StatusPendingPayment, StatusAwaitingShop, StatusConfirmed, StatusReady, StatusActive:
		return true
	}
	return false
}

// SlotStatuses lists every status for which HoldsSlot is true, for SQL IN (...).
func SlotStatuses() []Status {
	return []Status{StatusPendingPayment, StatusAwaitingShop, StatusConfirmed, StatusReady, StatusActive}
}

// Overlaps is the half-open range rule: touching ranges do not overlap.
func Overlaps(aStart, aEnd, bStart, bEnd time.Time) bool {
	return aStart.Before(bEnd) && bStart.Before(aEnd)
}

// TripSteps are the renter's six-step status line.
var TripSteps = [6]string{"Confirmed", "Ready for pickup", "Checked in", "On trip", "Returned", "Closed"}

// TripStep returns how many steps of TripSteps are reached (1..6), 0 for a
// booking not yet confirmed (awaiting shop / unpaid), or -1 for one that
// left the line (cancelled, declined, expired). checkedIn is true once all
// check-in photos are in; there is no separate status for it.
func TripStep(s Status, checkedIn bool) int {
	switch s {
	case StatusPendingPayment, StatusAwaitingShop:
		return 0
	case StatusConfirmed:
		return 1
	case StatusReady:
		if checkedIn {
			return 3
		}
		return 2
	case StatusActive:
		return 4
	case StatusReturned:
		return 5
	case StatusClosed:
		return 6
	}
	return -1
}

// StatusLabel is the renter-facing name of a status.
func (s Status) Label() string {
	switch s {
	case StatusPendingPayment:
		return "Waiting for payment"
	case StatusAwaitingShop:
		return "Waiting for the shop"
	case StatusConfirmed:
		return "Confirmed"
	case StatusReady:
		return "Ready for pickup"
	case StatusActive:
		return "On trip"
	case StatusReturned:
		return "Returned"
	case StatusClosed:
		return "Closed"
	case StatusCancelled:
		return "Cancelled"
	case StatusDeclined:
		return "Declined by shop"
	case StatusExpired:
		return "Payment timed out"
	}
	return string(s)
}

// Tone is the colour family a status chip uses: good, warn, bad, road or muted.
func (s Status) Tone() string {
	switch s {
	case StatusConfirmed, StatusClosed:
		return "good"
	case StatusPendingPayment, StatusAwaitingShop, StatusReturned:
		return "warn"
	case StatusReady, StatusActive:
		return "road"
	case StatusDeclined:
		return "bad"
	}
	return "muted"
}
