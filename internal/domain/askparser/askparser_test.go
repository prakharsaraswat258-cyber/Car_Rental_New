package askparser

import (
	"strings"
	"testing"
	"time"

	"saarthi/internal/domain"
)

// now = Tue 20 Oct 2026 15:00 IST (spec §4)
var now = time.Date(2026, 10, 20, 15, 0, 0, 0, domain.IST)

var gaz = Gazetteer{
	{CitySlug: "mount-abu", CityName: "Mount Abu", Aliases: []string{"mount abu", "mt abu", "mt. abu", "abu"}},
	{CitySlug: "mount-abu", CityName: "Mount Abu", AreaID: "nakki", AreaName: "Nakki Lake", Aliases: []string{"nakki lake", "nakki"}},
	{CitySlug: "mount-abu", CityName: "Mount Abu", AreaID: "polo", AreaName: "Polo Ground", Aliases: []string{"polo ground", "polo"}},
	{CitySlug: "mount-abu", CityName: "Mount Abu", AreaID: "abu-road", AreaName: "Abu Road", Aliases: []string{"abu road", "abu rd"}},
	{CitySlug: "goa", CityName: "Goa", Aliases: []string{"goa"}},
	{CitySlug: "goa", CityName: "Goa", AreaID: "panaji", AreaName: "Panaji", Aliases: []string{"panaji", "panjim"}},
	{CitySlug: "goa", CityName: "Goa", AreaID: "calangute", AreaName: "Calangute", Aliases: []string{"calangute"}},
	{CitySlug: "goa", CityName: "Goa", AreaID: "anjuna", AreaName: "Anjuna", Aliases: []string{"anjuna"}},
	{CitySlug: "goa", CityName: "Goa", AreaID: "margao", AreaName: "Margao", Aliases: []string{"margao", "madgaon"}},
}

// at builds an IST time in Oct/Nov 2026 (or any month).
func at(month time.Month, day, hour, min int) time.Time {
	y := 2026
	if month < time.October {
		y = 2027
	}
	return time.Date(y, month, day, hour, min, 0, 0, domain.IST)
}

type want struct {
	city, area, typ string
	start, end      time.Time // zero = no dates
	assumed         bool
	max             int64
	cheap           bool
	missing         string // comma list
}

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		want want
	}{
		// the five from the spec
		{"scooter in mount abu tomorrow 9am for 2 days under 800",
			want{city: "mount-abu", typ: "scooter", start: at(10, 21, 9, 0), end: at(10, 23, 9, 0), max: 800}},
		{"bullet goa this weekend",
			want{city: "goa", typ: "bike", start: at(10, 24, 9, 0), end: at(10, 25, 20, 0)}},
		{"car panjim 15 nov to 18 nov",
			want{city: "goa", area: "panaji", typ: "car", start: at(11, 15, 9, 0), end: at(11, 18, 9, 0)}},
		{"ev for 3 hrs",
			want{typ: "ev", start: at(10, 20, 16, 0), end: at(10, 20, 19, 0), missing: "place"}},
		{"cheap bike",
			want{typ: "bike", cheap: true, missing: "place,dates"}},

		// places and aliases
		{"scooty abu tomorrow",
			want{city: "mount-abu", typ: "scooter", start: at(10, 21, 9, 0), end: at(10, 22, 9, 0), assumed: true}},
		{"activa nakki lake day after tomorrow 10am for 5 hours",
			want{city: "mount-abu", area: "nakki", typ: "scooter", start: at(10, 22, 10, 0), end: at(10, 22, 15, 0)}},
		{"car at abu road 15/11 for a week",
			want{city: "mount-abu", area: "abu-road", typ: "car", start: at(11, 15, 9, 0), end: at(11, 22, 9, 0)}},
		{"Scooter in Mount Abu",
			want{city: "mount-abu", typ: "scooter", missing: "dates"}},
		{"goa",
			want{city: "goa", missing: "dates"}},
		{"",
			want{missing: "place,dates"}},

		// dates and times
		{"royal enfield goa next saturday for 3 days",
			want{city: "goa", typ: "bike", start: at(10, 24, 9, 0), end: at(10, 27, 9, 0)}},
		{"cycle polo ground tonight",
			want{city: "mount-abu", area: "polo", typ: "cycle", start: at(10, 20, 20, 0), end: at(10, 21, 20, 0), assumed: true}},
		{"suv calangute nov 15 evening for 2 days",
			want{city: "goa", area: "calangute", typ: "car", start: at(11, 15, 18, 0), end: at(11, 17, 18, 0)}},
		{"electric scooter margao 9:30 pm today for 2 hrs",
			want{city: "goa", area: "margao", typ: "ev", start: at(10, 20, 21, 30), end: at(10, 20, 23, 30)}},
		{"bike anjuna 21:00 for half day",
			want{city: "goa", area: "anjuna", typ: "bike", start: at(10, 20, 21, 0), end: at(10, 21, 3, 0)}},
		{"motorcycle mt abu friday afternoon for 4 hours",
			want{city: "mount-abu", typ: "bike", start: at(10, 23, 14, 0), end: at(10, 23, 18, 0)}},
		{"hatchback panaji 12 nov till sunday",
			want{city: "goa", area: "panaji", typ: "car", start: at(11, 12, 9, 0), end: at(11, 15, 9, 0)}},
		{"bicycle goa this weekend for 3 hrs",
			want{city: "goa", typ: "cycle", start: at(10, 24, 9, 0), end: at(10, 24, 12, 0)}},
		{"jupiter abu 9am",
			want{city: "mount-abu", typ: "scooter", start: at(10, 21, 9, 0), end: at(10, 22, 9, 0), assumed: true}},
		{"scooter abu 6pm",
			want{city: "mount-abu", typ: "scooter", start: at(10, 20, 18, 0), end: at(10, 21, 18, 0), assumed: true}},
		{"15 to 18 nov car goa",
			want{city: "goa", typ: "car", start: at(11, 15, 9, 0), end: at(11, 18, 9, 0)}},
		{"scooter goa 15 jan",
			want{city: "goa", typ: "scooter", start: at(1, 15, 9, 0), end: at(1, 16, 9, 0), assumed: true}},
		{"car goa today",
			want{city: "goa", typ: "car", start: at(10, 20, 16, 0), end: at(10, 21, 16, 0), assumed: true}},
		{"car goa tomorrow 10am to 6pm",
			want{city: "goa", typ: "car", start: at(10, 21, 10, 0), end: at(10, 21, 18, 0)}},
		{"bike goa for 45 days",
			want{city: "goa", typ: "bike", start: at(10, 20, 16, 0), end: at(11, 19, 16, 0)}},
		{"scooter goa 2nd nov for a full day",
			want{city: "goa", typ: "scooter", start: at(11, 2, 9, 0), end: at(11, 3, 9, 0)}},

		// budgets
		{"scooter goa tomorrow morning under ₹500",
			want{city: "goa", typ: "scooter", start: at(10, 21, 9, 0), end: at(10, 22, 9, 0), assumed: true, max: 500}},
		{"car goa tomorrow 800 rs",
			want{city: "goa", typ: "car", start: at(10, 21, 9, 0), end: at(10, 22, 9, 0), assumed: true, max: 800}},
		{"bike goa tomorrow 1k",
			want{city: "goa", typ: "bike", start: at(10, 21, 9, 0), end: at(10, 22, 9, 0), assumed: true, max: 1000}},
		{"car goa tomorrow upto 1.5k",
			want{city: "goa", typ: "car", start: at(10, 21, 9, 0), end: at(10, 22, 9, 0), assumed: true, max: 1500}},
		{"sedan goa from 15 nov to 18 nov below 3000",
			want{city: "goa", typ: "car", start: at(11, 15, 9, 0), end: at(11, 18, 9, 0), max: 3000}},
		{"budget scooter in goa",
			want{city: "goa", typ: "scooter", cheap: true, missing: "dates"}},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			r := Parse(tt.in, now, gaz)
			w := tt.want
			if r.CitySlug != w.city || r.AreaID != w.area {
				t.Errorf("place = %q/%q, want %q/%q", r.CitySlug, r.AreaID, w.city, w.area)
			}
			if r.Type != w.typ {
				t.Errorf("type = %q, want %q", r.Type, w.typ)
			}
			if !r.Start.Equal(w.start) || !r.End.Equal(w.end) {
				t.Errorf("dates = %s → %s, want %s → %s", fmtT(r.Start), fmtT(r.End), fmtT(w.start), fmtT(w.end))
			}
			if r.DurationAssumed != w.assumed {
				t.Errorf("assumed = %v, want %v", r.DurationAssumed, w.assumed)
			}
			if r.MaxPerDay != w.max {
				t.Errorf("max = %d, want %d", r.MaxPerDay, w.max)
			}
			if r.SortByPrice != w.cheap {
				t.Errorf("cheap = %v, want %v", r.SortByPrice, w.cheap)
			}
			if got := strings.Join(r.Missing, ","); got != w.missing {
				t.Errorf("missing = %q, want %q", got, w.missing)
			}
		})
	}
}

func fmtT(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.In(domain.IST).Format("Mon 2 Jan 15:04")
}

func TestChipsAndSpans(t *testing.T) {
	text := "scooter in mount abu tomorrow 9am for 2 days under 800"
	r := Parse(text, now, gaz)

	byKind := map[string]Chip{}
	for _, c := range r.Chips {
		byKind[c.Kind] = c
	}
	words := func(c Chip) []string {
		var out []string
		for _, s := range c.Spans {
			out = append(out, text[s.Start:s.End]) // ASCII text: UTF-16 offsets == byte offsets
		}
		return out
	}

	if c := byKind[ChipPlace]; c.Label != "Mount Abu" || strings.Join(words(c), "|") != "mount abu" {
		t.Errorf("place chip = %+v %v", c, words(c))
	}
	if c := byKind[ChipType]; c.Label != "Scooter" || strings.Join(words(c), "|") != "scooter" {
		t.Errorf("type chip = %+v", c)
	}
	if c := byKind[ChipDates]; c.Label != "Wed 21 Oct, 9 am → Fri 23 Oct, 9 am" || strings.Join(words(c), "|") != "tomorrow|9am|for 2 days" {
		t.Errorf("dates chip = %+v %v", c, words(c))
	}
	if c := byKind[ChipBudget]; c.Label != "Up to ₹800 a day" || strings.Join(words(c), "|") != "under 800" {
		t.Errorf("budget chip = %+v %v", c, words(c))
	}
	if len(r.Missing) != 0 {
		t.Errorf("missing = %v", r.Missing)
	}
}

func TestAssumedChipAndArea(t *testing.T) {
	r := Parse("car panjim tomorrow", now, gaz)
	var dates, place Chip
	for _, c := range r.Chips {
		switch c.Kind {
		case ChipDates:
			dates = c
		case ChipPlace:
			place = c
		}
	}
	if !dates.Assumed {
		t.Error("1-day duration should be marked assumed")
	}
	if place.Label != "Panaji, Goa" {
		t.Errorf("place label = %q", place.Label)
	}
}

func TestSpansCountUTF16(t *testing.T) {
	// ₹ is one UTF-16 unit but three bytes; spans must count units.
	text := "₹500 scooter goa"
	r := Parse(text, now, gaz)
	for _, c := range r.Chips {
		if c.Kind == ChipType {
			if c.Spans[0].Start != 5 || c.Spans[0].End != 12 {
				t.Errorf("type span = %+v, want 5..12", c.Spans[0])
			}
		}
	}
	if r.MaxPerDay != 500 {
		t.Errorf("max = %d", r.MaxPerDay)
	}
}

func TestQuery(t *testing.T) {
	r := Parse("scooter in mount abu tomorrow 9am for 2 days under 800", now, gaz)
	q := r.Query()
	checks := map[string]string{
		"city": "mount-abu", "type": "scooter", "start": "2026-10-21T09:00", "end": "2026-10-23T09:00", "max": "800",
	}
	for k, v := range checks {
		if q.Get(k) != v {
			t.Errorf("query %s = %q, want %q", k, q.Get(k), v)
		}
	}
	if q.Has("assumed") || q.Has("sort") {
		t.Errorf("unexpected params: %v", q)
	}
}
