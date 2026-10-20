package domain

import "strings"

// VehicleTypes in the order the UI lists them.
var VehicleTypes = []string{"scooter", "bike", "car", "ev", "cycle"}

// TypeLabel is the display name of a vehicle type.
func TypeLabel(t string) string {
	switch t {
	case "scooter":
		return "Scooter"
	case "bike":
		return "Bike"
	case "car":
		return "Car"
	case "ev":
		return "EV"
	case "cycle":
		return "Cycle"
	}
	return t
}

// Vehicle classes decide which add-ons fit (D-014).
const (
	ClassTwoWheeler  = "two_wheeler"
	ClassFourWheeler = "four_wheeler"
	ClassCycle       = "cycle"
)

// ClassOf maps a vehicle type and seat count to a class. EVs can be either
// a scooter (2 seats) or a car (5 seats), so seats decide.
func ClassOf(vehicleType string, seats int) string {
	switch {
	case vehicleType == "cycle":
		return ClassCycle
	case vehicleType == "car" || seats >= 4:
		return ClassFourWheeler
	}
	return ClassTwoWheeler
}

// AddonFits reports whether an add-on with the given fits list (comma
// separated classes, empty = all) applies to a vehicle class.
func AddonFits(fits, class string) bool {
	if strings.TrimSpace(fits) == "" {
		return true
	}
	for _, f := range strings.Split(fits, ",") {
		if strings.TrimSpace(f) == class {
			return true
		}
	}
	return false
}
