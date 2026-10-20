package domain

import "testing"

func TestPaiseString(t *testing.T) {
	tests := []struct {
		in   Paise
		want string
	}{
		{0, "₹0"},
		{Rupees(5), "₹5"},
		{Rupees(450), "₹450"},
		{Rupees(1500), "₹1,500"},
		{Rupees(12345), "₹12,345"},
		{Rupees(123456), "₹1,23,456"},
		{Rupees(12345678), "₹1,23,45,678"},
		{14160, "₹141.60"},
		{100005, "₹1,000.05"},
		{-Rupees(250), "−₹250"},
	}
	for _, tt := range tests {
		if got := tt.in.String(); got != tt.want {
			t.Errorf("Paise(%d).String() = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestPaiseWholeRupees(t *testing.T) {
	tests := []struct {
		in   Paise
		want int64
	}{
		{0, 0}, {49, 0}, {50, 1}, {14160, 142}, {14149, 141}, {-150, -2},
	}
	for _, tt := range tests {
		if got := tt.in.WholeRupees(); got != tt.want {
			t.Errorf("Paise(%d).WholeRupees() = %d, want %d", tt.in, got, tt.want)
		}
	}
}
