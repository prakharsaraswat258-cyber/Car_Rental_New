package domain

import "math"

// Proof photo shots. Cars walk around 8 angles plus the odometer and the
// fuel (or charge) gauge; two-wheelers need 4 angles and the odometer;
// cycles just the 4 angles (UI_UX_UPGRADE §1).
const (
	ShotFront      = "front"
	ShotFrontLeft  = "front_left"
	ShotLeft       = "left"
	ShotRearLeft   = "rear_left"
	ShotRear       = "rear"
	ShotRearRight  = "rear_right"
	ShotRight      = "right"
	ShotFrontRight = "front_right"
	ShotOdometer   = "odometer"
	ShotFuel       = "fuel"
)

// AllShots is every shot the database accepts, in walk-around order.
var AllShots = []string{ShotFront, ShotFrontLeft, ShotLeft, ShotRearLeft, ShotRear, ShotRearRight, ShotRight, ShotFrontRight, ShotOdometer, ShotFuel}

// ProofShots lists the required shots for a vehicle class, in the order
// the camera walks through them.
func ProofShots(class string) []string {
	switch class {
	case ClassFourWheeler:
		return []string{ShotFront, ShotFrontLeft, ShotLeft, ShotRearLeft, ShotRear, ShotRearRight, ShotRight, ShotFrontRight, ShotOdometer, ShotFuel}
	case ClassCycle:
		return []string{ShotFront, ShotLeft, ShotRear, ShotRight}
	}
	return []string{ShotFront, ShotLeft, ShotRear, ShotRight, ShotOdometer}
}

// ShotsComplete reports whether every required shot is in have.
func ShotsComplete(class string, have map[string]bool) bool {
	for _, s := range ProofShots(class) {
		if !have[s] {
			return false
		}
	}
	return true
}

// MissingShots lists required shots not in have, in walk-around order.
func MissingShots(class string, have map[string]bool) []string {
	var out []string
	for _, s := range ProofShots(class) {
		if !have[s] {
			out = append(out, s)
		}
	}
	return out
}

// ShotLabel is the human name of a shot: "Front left", "Fuel gauge".
func ShotLabel(shot string, electric bool) string {
	switch shot {
	case ShotFront:
		return "Front"
	case ShotFrontLeft:
		return "Front left"
	case ShotLeft:
		return "Left side"
	case ShotRearLeft:
		return "Rear left"
	case ShotRear:
		return "Rear"
	case ShotRearRight:
		return "Rear right"
	case ShotRight:
		return "Right side"
	case ShotFrontRight:
		return "Front right"
	case ShotOdometer:
		return "Odometer"
	case ShotFuel:
		if electric {
			return "Charge level"
		}
		return "Fuel gauge"
	}
	return shot
}

// ProofPhrase says what the walk-around asks for, for copy like "Take
// eight quick photos, then the odometer and fuel, at pickup and return."
func ProofPhrase(class string) string {
	switch class {
	case ClassFourWheeler:
		return "eight quick photos, then the odometer and fuel"
	case ClassCycle:
		return "four quick photos"
	}
	return "four quick photos and the odometer"
}

// IsAngle reports whether a shot is of the body (not a gauge).
func IsAngle(shot string) bool { return shot != ShotOdometer && shot != ShotFuel }

// HaversineKm is the great-circle distance between two points in km.
func HaversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	const r = 6371.0
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLng := (lng2 - lng1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * r * math.Asin(math.Min(1, math.Sqrt(a)))
}
