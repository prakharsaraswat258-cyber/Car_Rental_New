package domain

import (
	"testing"
	"time"
)

func TestCanTransitionAllowed(t *testing.T) {
	allowed := []struct {
		from, to Status
		by       Actor
	}{
		{StatusPendingPayment, StatusConfirmed, ActorRenter},
		{StatusPendingPayment, StatusAwaitingShop, ActorRenter},
		{StatusPendingPayment, StatusExpired, ActorSystem},
		{StatusPendingPayment, StatusCancelled, ActorRenter},
		{StatusAwaitingShop, StatusConfirmed, ActorProvider},
		{StatusAwaitingShop, StatusDeclined, ActorProvider},
		{StatusAwaitingShop, StatusCancelled, ActorRenter},
		{StatusConfirmed, StatusReady, ActorProvider},
		{StatusConfirmed, StatusCancelled, ActorRenter},
		{StatusReady, StatusActive, ActorRenter},
		{StatusReady, StatusCancelled, ActorRenter},
		{StatusActive, StatusReturned, ActorRenter},
		{StatusReturned, StatusClosed, ActorProvider},
	}
	for _, tt := range allowed {
		if !CanTransition(tt.from, tt.to, tt.by) {
			t.Errorf("%s → %s by %s should be allowed", tt.from, tt.to, tt.by)
		}
	}
}

func TestCanTransitionForbidden(t *testing.T) {
	forbidden := []struct {
		name     string
		from, to Status
		by       Actor
	}{
		{"renter cannot accept own request", StatusAwaitingShop, StatusConfirmed, ActorRenter},
		{"provider cannot pay", StatusPendingPayment, StatusConfirmed, ActorProvider},
		{"provider cannot start trip", StatusReady, StatusActive, ActorProvider},
		{"renter cannot mark ready", StatusConfirmed, StatusReady, ActorRenter},
		{"renter cannot close (inspection is the shop's)", StatusReturned, StatusClosed, ActorRenter},
		{"no skipping ready", StatusConfirmed, StatusActive, ActorRenter},
		{"no cancelling an active trip", StatusActive, StatusCancelled, ActorRenter},
		{"no cancelling after return", StatusReturned, StatusCancelled, ActorRenter},
		{"closed is final", StatusClosed, StatusActive, ActorProvider},
		{"cancelled is final", StatusCancelled, StatusConfirmed, ActorProvider},
		{"declined is final", StatusDeclined, StatusConfirmed, ActorProvider},
		{"expired is final", StatusExpired, StatusConfirmed, ActorRenter},
		{"system cannot confirm", StatusPendingPayment, StatusConfirmed, ActorSystem},
		{"only pending can expire", StatusConfirmed, StatusExpired, ActorSystem},
		{"no going backwards", StatusActive, StatusReady, ActorProvider},
		{"self loop", StatusConfirmed, StatusConfirmed, ActorProvider},
	}
	for _, tt := range forbidden {
		t.Run(tt.name, func(t *testing.T) {
			if CanTransition(tt.from, tt.to, tt.by) {
				t.Errorf("%s → %s by %s should be forbidden", tt.from, tt.to, tt.by)
			}
		})
	}
}

func TestTerminalStatusesHaveNoExit(t *testing.T) {
	all := []Status{StatusPendingPayment, StatusAwaitingShop, StatusConfirmed, StatusReady, StatusActive,
		StatusReturned, StatusClosed, StatusCancelled, StatusDeclined, StatusExpired}
	actors := []Actor{ActorRenter, ActorProvider, ActorSystem}
	for _, from := range all {
		if !from.Terminal() {
			continue
		}
		for _, to := range all {
			for _, a := range actors {
				if CanTransition(from, to, a) {
					t.Errorf("terminal %s has exit → %s by %s", from, to, a)
				}
			}
		}
	}
}

func TestOverlaps(t *testing.T) {
	at := func(h int) time.Time {
		return time.Date(2026, 11, 9, 0, 0, 0, 0, IST).Add(time.Duration(h) * time.Hour)
	}
	tests := []struct {
		name       string
		a1, a2     int
		b1, b2     int
		wantResult bool
	}{
		{"same range", 0, 24, 0, 24, true},
		{"b inside a", 0, 48, 10, 12, true},
		{"a inside b", 10, 12, 0, 48, true},
		{"b starts inside a", 0, 24, 23, 30, true},
		{"b ends inside a", 10, 24, 0, 11, true},
		{"touching end→start is fine", 0, 24, 24, 48, false},
		{"touching start←end is fine", 24, 48, 0, 24, false},
		{"far apart", 0, 10, 20, 30, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Overlaps(at(tt.a1), at(tt.a2), at(tt.b1), at(tt.b2)); got != tt.wantResult {
				t.Errorf("Overlaps = %v, want %v", got, tt.wantResult)
			}
		})
	}
}

func TestTripStep(t *testing.T) {
	tests := []struct {
		s         Status
		checkedIn bool
		want      int
	}{
		{StatusAwaitingShop, false, 0},
		{StatusPendingPayment, false, 0},
		{StatusConfirmed, false, 1},
		{StatusReady, false, 2},
		{StatusReady, true, 3},
		{StatusActive, true, 4},
		{StatusReturned, true, 5},
		{StatusClosed, true, 6},
		{StatusCancelled, false, -1},
		{StatusDeclined, false, -1},
		{StatusExpired, false, -1},
	}
	for _, tt := range tests {
		if got := TripStep(tt.s, tt.checkedIn); got != tt.want {
			t.Errorf("TripStep(%s, %v) = %d, want %d", tt.s, tt.checkedIn, got, tt.want)
		}
	}
}

func TestHoldsSlotMatchesSlotStatuses(t *testing.T) {
	in := map[Status]bool{}
	for _, s := range SlotStatuses() {
		in[s] = true
		if !s.HoldsSlot() {
			t.Errorf("%s in SlotStatuses but HoldsSlot is false", s)
		}
	}
	for _, s := range []Status{StatusReturned, StatusClosed, StatusCancelled, StatusDeclined, StatusExpired} {
		if s.HoldsSlot() || in[s] {
			t.Errorf("%s should not hold a slot", s)
		}
	}
}
