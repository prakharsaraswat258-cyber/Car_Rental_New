package domain

import (
	"math"
	"strings"
	"testing"
)

func TestProofShots(t *testing.T) {
	tests := []struct {
		class string
		want  string
	}{
		{ClassFourWheeler, "front,front_left,left,rear_left,rear,rear_right,right,front_right,odometer,fuel"},
		{ClassTwoWheeler, "front,left,rear,right,odometer"},
		{ClassCycle, "front,left,rear,right"},
	}
	for _, tt := range tests {
		if got := strings.Join(ProofShots(tt.class), ","); got != tt.want {
			t.Errorf("ProofShots(%s) = %s, want %s", tt.class, got, tt.want)
		}
		for _, s := range ProofShots(tt.class) {
			found := false
			for _, a := range AllShots {
				found = found || a == s
			}
			if !found {
				t.Errorf("%s is not in AllShots", s)
			}
		}
	}
}

func TestShotsComplete(t *testing.T) {
	have := map[string]bool{"front": true, "left": true, "rear": true, "right": true}
	if ShotsComplete(ClassTwoWheeler, have) {
		t.Error("two-wheeler without odometer should be incomplete")
	}
	if got := MissingShots(ClassTwoWheeler, have); len(got) != 1 || got[0] != ShotOdometer {
		t.Errorf("missing = %v", got)
	}
	if !ShotsComplete(ClassCycle, have) {
		t.Error("cycle with 4 angles should be complete")
	}
	have["odometer"] = true
	if !ShotsComplete(ClassTwoWheeler, have) {
		t.Error("two-wheeler with 4 angles + odometer should be complete")
	}
	if got := MissingShots(ClassFourWheeler, have); len(got) != 5 || got[0] != ShotFrontLeft || got[4] != ShotFuel {
		t.Errorf("car missing = %v", got)
	}
}

func TestShotLabel(t *testing.T) {
	if ShotLabel(ShotFuel, true) != "Charge level" || ShotLabel(ShotFuel, false) != "Fuel gauge" || ShotLabel(ShotRearRight, false) != "Rear right" {
		t.Error("labels wrong")
	}
	if IsAngle(ShotOdometer) || !IsAngle(ShotFrontLeft) {
		t.Error("IsAngle wrong")
	}
}

func TestHaversineKm(t *testing.T) {
	tests := []struct {
		name                   string
		lat1, lng1, lat2, lng2 float64
		want, tol              float64
	}{
		{"same point", 24.59, 72.71, 24.59, 72.71, 0, 0.001},
		{"Nakki Lake to Abu Road station", 24.5936, 72.7076, 24.4797, 72.7825, 14.8, 0.5},
		{"Panaji to Margao", 15.4909, 73.8278, 15.2832, 73.9862, 28.6, 0.8},
	}
	for _, tt := range tests {
		if got := HaversineKm(tt.lat1, tt.lng1, tt.lat2, tt.lng2); math.Abs(got-tt.want) > tt.tol {
			t.Errorf("%s: %.2f km, want %.1f", tt.name, got, tt.want)
		}
	}
}
