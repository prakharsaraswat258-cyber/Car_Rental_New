package domain

import "testing"

func TestClassOf(t *testing.T) {
	tests := []struct {
		typ   string
		seats int
		want  string
	}{
		{"scooter", 2, ClassTwoWheeler},
		{"bike", 2, ClassTwoWheeler},
		{"car", 5, ClassFourWheeler},
		{"car", 2, ClassFourWheeler}, // two-seater car is still a car
		{"ev", 2, ClassTwoWheeler},   // electric scooter
		{"ev", 5, ClassFourWheeler},  // electric car
		{"cycle", 1, ClassCycle},
	}
	for _, tt := range tests {
		if got := ClassOf(tt.typ, tt.seats); got != tt.want {
			t.Errorf("ClassOf(%s, %d) = %s, want %s", tt.typ, tt.seats, got, tt.want)
		}
	}
}

func TestAddonFits(t *testing.T) {
	tests := []struct {
		fits, class string
		want        bool
	}{
		{"", ClassCycle, true},
		{"two_wheeler", ClassTwoWheeler, true},
		{"two_wheeler", ClassFourWheeler, false},
		{"two_wheeler, cycle", ClassCycle, true},
		{"four_wheeler", ClassTwoWheeler, false},
	}
	for _, tt := range tests {
		if got := AddonFits(tt.fits, tt.class); got != tt.want {
			t.Errorf("AddonFits(%q, %s) = %v, want %v", tt.fits, tt.class, got, tt.want)
		}
	}
}
