package domain

import (
	"fmt"
	"time"
)

// IST is India Standard Time (UTC+05:30). India has no daylight saving,
// so a fixed zone is exact and needs no tzdata on Windows.
var IST = time.FixedZone("IST", 5*60*60+30*60)

// LocalLayout is how date-times travel in URLs and form fields:
// wall-clock IST, minute precision ("2026-10-21T09:00").
const LocalLayout = "2006-01-02T15:04"

// ParseLocal reads a LocalLayout value as IST.
func ParseLocal(s string) (time.Time, error) {
	return time.ParseInLocation(LocalLayout, s, IST)
}

// FormatLocal writes t as a LocalLayout value in IST.
func FormatLocal(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(IST).Format(LocalLayout)
}

// Clock renders a time of day calmly: "9 am", "9:30 pm", "12 pm".
func Clock(t time.Time) string {
	t = t.In(IST)
	h, m := t.Hour(), t.Minute()
	suffix := "am"
	if h >= 12 {
		suffix = "pm"
	}
	h12 := h % 12
	if h12 == 0 {
		h12 = 12
	}
	if m == 0 {
		return fmt.Sprintf("%d %s", h12, suffix)
	}
	return fmt.Sprintf("%d:%02d %s", h12, m, suffix)
}

// DayText renders "Wed 21 Oct".
func DayText(t time.Time) string { return t.In(IST).Format("Mon 2 Jan") }

// RangeText renders a trip: "Wed 21 Oct, 9 am → Fri 23 Oct, 9 am", or
// "Tue 20 Oct, 4 pm → 7 pm" when both ends fall on the same day.
func RangeText(start, end time.Time) string {
	s, e := start.In(IST), end.In(IST)
	if s.Year() == e.Year() && s.YearDay() == e.YearDay() {
		return DayText(s) + ", " + Clock(s) + " → " + Clock(e)
	}
	return DayText(s) + ", " + Clock(s) + " → " + DayText(e) + ", " + Clock(e)
}

// SameDay reports whether a and b fall on the same IST calendar day.
func SameDay(a, b time.Time) bool {
	a, b = a.In(IST), b.In(IST)
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

// StartOfDay is midnight IST of t's day.
func StartOfDay(t time.Time) time.Time {
	t = t.In(IST)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, IST)
}

// NextHour is the start of the IST hour after t.
func NextHour(t time.Time) time.Time {
	t = t.In(IST)
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, IST)
}

// DurationWords renders a duration for people: "45 min", "2 h 25 min", "3 days".
func DurationWords(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	mins := int(d.Round(time.Minute) / time.Minute)
	switch {
	case mins < 60:
		return fmt.Sprintf("%d min", mins)
	case mins < 48*60:
		h, m := mins/60, mins%60
		if m == 0 {
			return fmt.Sprintf("%d h", h)
		}
		return fmt.Sprintf("%d h %d min", h, m)
	default:
		days := mins / (24 * 60)
		return fmt.Sprintf("%d days", days)
	}
}
