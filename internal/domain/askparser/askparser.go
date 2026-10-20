// Package askparser turns a trip described like a text message
// ("scooter in mount abu tomorrow 9am for 2 days under 800") into search
// filters plus chips for the UI. It is rule-based: deterministic, offline,
// and covered by table tests.
package askparser

import (
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"saarthi/internal/domain"
)

// Place is a city, or an area inside a city, with lower-case aliases.
type Place struct {
	CitySlug string
	CityName string
	AreaID   string // empty for a whole city
	AreaName string
	Aliases  []string
}

// Gazetteer is every place the parser can recognise.
type Gazetteer []Place

// Span marks matched text, in UTF-16 offsets (what JavaScript indexes),
// so the UI can lift those exact words into the chips row.
type Span struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Chip kinds.
const (
	ChipPlace  = "place"
	ChipType   = "type"
	ChipDates  = "dates"
	ChipBudget = "budget"
	ChipSort   = "sort"
)

// Chip is one understood piece of the request.
type Chip struct {
	Kind    string `json:"kind"`
	Label   string `json:"label"`
	Spans   []Span `json:"spans"`
	Assumed bool   `json:"assumed,omitempty"`
}

// Missing values tell the UI what to ask next.
const (
	MissingPlace = "place"
	MissingDates = "dates"
)

// Result is everything Parse understood.
type Result struct {
	Text            string
	CitySlug        string
	CityName        string
	AreaID          string
	AreaName        string
	Type            string // scooter | bike | car | ev | cycle
	Start, End      time.Time
	DurationAssumed bool  // no duration given: 1 day assumed
	MaxPerDay       int64 // rupees, 0 = no budget
	SortByPrice     bool
	Chips           []Chip
	Missing         []string
}

// HasDates reports whether a trip window was understood.
func (r Result) HasDates() bool { return !r.Start.IsZero() }

// Query encodes the result as /search parameters.
func (r Result) Query() url.Values {
	v := url.Values{}
	if strings.TrimSpace(r.Text) != "" {
		v.Set("q", strings.TrimSpace(r.Text))
	}
	if r.CitySlug != "" {
		v.Set("city", r.CitySlug)
	}
	if r.AreaID != "" {
		v.Set("area", r.AreaID)
	}
	if r.Type != "" {
		v.Set("type", r.Type)
	}
	if r.HasDates() {
		v.Set("start", domain.FormatLocal(r.Start))
		v.Set("end", domain.FormatLocal(r.End))
		if r.DurationAssumed {
			v.Set("assumed", "1")
		}
	}
	if r.MaxPerDay > 0 {
		v.Set("max", strconv.FormatInt(r.MaxPerDay, 10))
	}
	if r.SortByPrice {
		v.Set("sort", "price")
	}
	return v
}

// Parse reads text relative to now (any zone; IST is used for calendar
// days) and the known places.
func Parse(text string, now time.Time, g Gazetteer) Result {
	p := &parser{text: text, low: asciiLower(text), now: now.In(domain.IST), spans: map[string][][2]int{}}
	p.used = make([]bool, len(p.low))
	r := Result{Text: text}

	p.budget(&r)
	p.dates(&r)
	p.sorting(&r)
	p.vehicleType(&r)
	p.place(&r, g)
	p.chips(&r)
	return r
}

// --- machinery ---------------------------------------------------------------

type parser struct {
	text, low string
	used      []bool
	now       time.Time
	spans     map[string][][2]int // chip kind → byte ranges
}

func (p *parser) free(a, b int) bool {
	for i := a; i < b; i++ {
		if p.used[i] {
			return false
		}
	}
	return true
}

func (p *parser) mark(a, b int, kind string) {
	for i := a; i < b; i++ {
		p.used[i] = true
	}
	p.spans[kind] = append(p.spans[kind], [2]int{a, b})
}

// take returns the first match of re over text not used yet, and marks it.
func (p *parser) take(re *regexp.Regexp, kind string) []int {
	for _, m := range re.FindAllStringSubmatchIndex(p.low, -1) {
		if p.free(m[0], m[1]) {
			p.mark(m[0], m[1], kind)
			return m
		}
	}
	return nil
}

func (p *parser) group(m []int, i int) string {
	if m == nil || 2*i+1 >= len(m) || m[2*i] < 0 {
		return ""
	}
	return p.low[m[2*i]:m[2*i+1]]
}

func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

func isWordByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= '0' && c <= '9'
}

// --- budget ------------------------------------------------------------------

const num = `(\d+(?:\.\d+)?)`

var (
	reBudgetKw = regexp.MustCompile(`\b(?:under|below|max|maximum|upto|up to|within|less than|budget(?: of)?)\s*(?:rs\.?|inr|₹)?\s*` +
		num + `\s*(k)?\b(?:\s*(?:rs\b\.?|rupees\b|inr\b|/-))?`)
	reBudgetCur    = regexp.MustCompile(`(?:₹|\brs\.?|\binr)\s*` + num + `\s*(k)?\b`)
	reBudgetSuffix = regexp.MustCompile(`\b` + num + `\s*(k)?\s*(?:rs\b\.?|rupees\b|inr\b|/-)`)
	reBudgetK      = regexp.MustCompile(`\b` + num + `(k)\b`)
)

func (p *parser) budget(r *Result) {
	for _, re := range []*regexp.Regexp{reBudgetKw, reBudgetCur, reBudgetSuffix, reBudgetK} {
		if m := p.take(re, ChipBudget); m != nil {
			v, err := strconv.ParseFloat(p.group(m, 1), 64)
			if err != nil || v <= 0 {
				continue
			}
			if p.group(m, 2) == "k" {
				v *= 1000
			}
			r.MaxPerDay = int64(v + 0.5)
			return
		}
	}
}

// --- dates, times, durations -------------------------------------------------

const (
	months   = `(jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|june?|july?|aug(?:ust)?|sep(?:t(?:ember)?)?|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)`
	dayNum   = `(\d{1,2})(?:st|nd|rd|th)?`
	weekdays = `(monday|mon|tuesday|tues|tue|wednesday|wed|thursday|thurs|thur|thu|friday|fri|saturday|sat|sunday|sun)`
	time12   = `(\d{1,2})(?:[:.](\d{2}))?\s*(am|pm)\b`
	time24   = `([01]?\d|2[0-3]):([0-5]\d)\b`
)

var (
	reDayRange   = regexp.MustCompile(`\b(?:from\s+)?` + dayNum + `\s*(?:-|to|till|until)\s*` + dayNum + `\s*` + months + `\b`)
	reEndTime12  = regexp.MustCompile(`(?:\b(?:to|till|until)\s*|-\s*)` + time12)
	reEndTime24  = regexp.MustCompile(`(?:\b(?:to|till|until)\s*|-\s*)` + time24)
	reEndDM      = regexp.MustCompile(`\b(?:to|till|until)\s+` + dayNum + `\s*` + months + `\b`)
	reEndMD      = regexp.MustCompile(`\b(?:to|till|until)\s+` + months + `\s*` + dayNum + `\b`)
	reEndSlash   = regexp.MustCompile(`\b(?:to|till|until)\s+(\d{1,2})/(\d{1,2})\b`)
	reEndWeekday = regexp.MustCompile(`\b(?:to|till|until)\s+(?:next\s+)?` + weekdays + `\b`)
	reRelative   = regexp.MustCompile(`\b(day after tomorrow|day after tmrw|tomorrow|tmrw|tmr|today|tonight|now)\b`)
	reWeekend    = regexp.MustCompile(`\b(?:(this|coming|next)\s+)?weekend\b`)
	reWeekday    = regexp.MustCompile(`\b(?:(next|this|coming|on)\s+)?` + weekdays + `\b`)
	reDM         = regexp.MustCompile(`\b(?:from\s+|on\s+)?` + dayNum + `\s*` + months + `\b`)
	reMD         = regexp.MustCompile(`\b(?:from\s+|on\s+)?` + months + `\s*` + dayNum + `\b`)
	reSlash      = regexp.MustCompile(`\b(?:from\s+|on\s+)?(\d{1,2})/(\d{1,2})(?:/(\d{2,4}))?\b`)
	reTime12     = regexp.MustCompile(`\b(?:at\s+|from\s+|by\s+)?` + time12)
	reTime24     = regexp.MustCompile(`\b(?:at\s+|from\s+)?` + time24)
	reTimeWord   = regexp.MustCompile(`\b(?:in the\s+|this\s+|at\s+)?(morning|afternoon|evening|noon|night)\b`)
	reDuration   = regexp.MustCompile(`\b(?:for\s+)?(\d+(?:\.\d+)?|half an?|half|an?|one|two|three|four|five|six|seven|eight|nine|ten)\s*(hours?|hrs?|h|days?|weeks?|wks?|nights?)\b`)
	reFullDay    = regexp.MustCompile(`\b(?:for\s+)?(?:a\s+)?(?:full|whole)\s+day\b`)
)

type civil struct {
	y int
	m time.Month
	d int
}

func civilOf(t time.Time) civil { t = t.In(domain.IST); return civil{t.Year(), t.Month(), t.Day()} }

func (c civil) at(h, m int) time.Time { return time.Date(c.y, c.m, c.d, h, m, 0, 0, domain.IST) }

func (c civil) addDays(n int) civil { return civilOf(c.at(12, 0).AddDate(0, 0, n)) }

func (c civil) weekday() time.Weekday { return c.at(12, 0).Weekday() }

func (c civil) before(o civil) bool { return c.at(0, 0).Before(o.at(0, 0)) }

type clock struct{ h, m int }

var monthByPrefix = map[string]time.Month{
	"jan": time.January, "feb": time.February, "mar": time.March, "apr": time.April, "may": time.May, "jun": time.June,
	"jul": time.July, "aug": time.August, "sep": time.September, "oct": time.October, "nov": time.November, "dec": time.December,
}

func monthOf(s string) time.Month { return monthByPrefix[s[:3]] }

var weekdayByPrefix = map[string]time.Weekday{
	"mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday, "thu": time.Thursday,
	"fri": time.Friday, "sat": time.Saturday, "sun": time.Sunday,
}

func weekdayOf(s string) time.Weekday { return weekdayByPrefix[s[:3]] }

// calendarDate resolves day+month to the next time it happens (this year,
// or next year if it already passed). Invalid dates (31 nov) return false.
func (p *parser) calendarDate(d int, m time.Month, notBefore civil) (civil, bool) {
	if m == 0 || d < 1 || d > 31 {
		return civil{}, false
	}
	t := time.Date(notBefore.y, m, d, 12, 0, 0, 0, domain.IST)
	if t.Month() != m {
		return civil{}, false
	}
	c := civilOf(t)
	if c.before(notBefore) {
		c = civilOf(t.AddDate(1, 0, 0))
	}
	return c, true
}

// weekdayFrom is the first wd on or after from (strictly after if strict).
func weekdayFrom(from civil, wd time.Weekday, strict bool) civil {
	n := (int(wd) - int(from.weekday()) + 7) % 7
	if n == 0 && strict {
		n = 7
	}
	return from.addDays(n)
}

func parseClock12(hs, ms, ampm string) (clock, bool) {
	h, _ := strconv.Atoi(hs)
	m := 0
	if ms != "" {
		m, _ = strconv.Atoi(ms)
	}
	if h < 1 || h > 12 || m > 59 {
		return clock{}, false
	}
	if ampm == "pm" && h != 12 {
		h += 12
	}
	if ampm == "am" && h == 12 {
		h = 0
	}
	return clock{h, m}, true
}

var numberWords = map[string]float64{
	"a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
	"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "half": 0.5, "half a": 0.5, "half an": 0.5,
}

// HalfDay is what "half day" means.
const HalfDay = 6 * time.Hour

func (p *parser) dates(r *Result) {
	today := civilOf(p.now)
	var (
		sDate, eDate     *civil
		sClock, eClock   *clock
		endWeekday       = time.Weekday(-1)
		dur              time.Duration
		weekend, tonight bool
		weekendNext      bool
	)

	// "15 to 18 nov"
	if m := p.take(reDayRange, ChipDates); m != nil {
		d1, _ := strconv.Atoi(p.group(m, 1))
		d2, _ := strconv.Atoi(p.group(m, 2))
		mon := monthOf(p.group(m, 3))
		if s, ok := p.calendarDate(d1, mon, today); ok {
			sDate = &s
			if e, ok := p.calendarDate(d2, mon, s); ok {
				eDate = &e
			}
		}
	}

	// end time: "10am to 6pm"
	if m := p.take(reEndTime12, ChipDates); m != nil {
		if c, ok := parseClock12(p.group(m, 1), p.group(m, 2), p.group(m, 3)); ok {
			eClock = &c
		}
	} else if m := p.take(reEndTime24, ChipDates); m != nil {
		h, _ := strconv.Atoi(p.group(m, 1))
		mi, _ := strconv.Atoi(p.group(m, 2))
		eClock = &clock{h, mi}
	}

	// end date: "to 18 nov", "till sunday", "to 18/11"
	if eDate == nil {
		if m := p.take(reEndDM, ChipDates); m != nil {
			d, _ := strconv.Atoi(p.group(m, 1))
			if e, ok := p.calendarDate(d, monthOf(p.group(m, 2)), today); ok {
				eDate = &e
			}
		} else if m := p.take(reEndMD, ChipDates); m != nil {
			d, _ := strconv.Atoi(p.group(m, 2))
			if e, ok := p.calendarDate(d, monthOf(p.group(m, 1)), today); ok {
				eDate = &e
			}
		} else if m := p.take(reEndSlash, ChipDates); m != nil {
			d, _ := strconv.Atoi(p.group(m, 1))
			mo, _ := strconv.Atoi(p.group(m, 2))
			if e, ok := p.calendarDate(d, time.Month(mo), today); ok && mo >= 1 && mo <= 12 {
				eDate = &e
			}
		} else if m := p.take(reEndWeekday, ChipDates); m != nil {
			endWeekday = weekdayOf(p.group(m, 1))
		}
	}

	// start date
	if sDate == nil {
		if m := p.take(reRelative, ChipDates); m != nil {
			switch p.group(m, 1) {
			case "today", "now":
				sDate = &today
			case "tonight":
				sDate, tonight = &today, true
			case "tomorrow", "tmrw", "tmr":
				t := today.addDays(1)
				sDate = &t
			default: // day after tomorrow
				t := today.addDays(2)
				sDate = &t
			}
		} else if m := p.take(reWeekend, ChipDates); m != nil {
			weekend = true
			weekendNext = p.group(m, 1) == "next"
		} else if m := p.take(reWeekday, ChipDates); m != nil {
			t := weekdayFrom(today, weekdayOf(p.group(m, 2)), p.group(m, 1) == "next")
			sDate = &t
		} else if m := p.take(reDM, ChipDates); m != nil {
			d, _ := strconv.Atoi(p.group(m, 1))
			if s, ok := p.calendarDate(d, monthOf(p.group(m, 2)), today); ok {
				sDate = &s
			}
		} else if m := p.take(reMD, ChipDates); m != nil {
			d, _ := strconv.Atoi(p.group(m, 2))
			if s, ok := p.calendarDate(d, monthOf(p.group(m, 1)), today); ok {
				sDate = &s
			}
		} else if m := p.take(reSlash, ChipDates); m != nil {
			d, _ := strconv.Atoi(p.group(m, 1))
			mo, _ := strconv.Atoi(p.group(m, 2))
			if mo >= 1 && mo <= 12 {
				if s, ok := p.calendarDate(d, time.Month(mo), today); ok {
					sDate = &s
				}
			}
		}
	}

	// start time
	if m := p.take(reTime12, ChipDates); m != nil {
		if c, ok := parseClock12(p.group(m, 1), p.group(m, 2), p.group(m, 3)); ok {
			sClock = &c
		}
	} else if m := p.take(reTime24, ChipDates); m != nil {
		h, _ := strconv.Atoi(p.group(m, 1))
		mi, _ := strconv.Atoi(p.group(m, 2))
		sClock = &clock{h, mi}
	} else if m := p.take(reTimeWord, ChipDates); m != nil {
		c := map[string]clock{"morning": {9, 0}, "noon": {12, 0}, "afternoon": {14, 0}, "evening": {18, 0}, "night": {20, 0}}[p.group(m, 1)]
		sClock = &c
	}

	// duration
	if m := p.take(reDuration, ChipDates); m != nil {
		amount := numberWords[p.group(m, 1)]
		if amount == 0 {
			amount, _ = strconv.ParseFloat(p.group(m, 1), 64)
		}
		unit := p.group(m, 2)
		switch {
		case strings.HasPrefix(unit, "h"):
			dur = time.Duration(amount * float64(time.Hour))
		case strings.HasPrefix(unit, "w"):
			dur = time.Duration(amount * 7 * 24 * float64(time.Hour))
		default: // days, nights
			if amount == 0.5 {
				dur = HalfDay
			} else {
				dur = time.Duration(amount * 24 * float64(time.Hour))
			}
		}
	} else if p.take(reFullDay, ChipDates) != nil {
		dur = 24 * time.Hour
	}

	if sDate == nil && sClock == nil && !weekend && dur == 0 && eClock == nil && eDate == nil && endWeekday < 0 {
		return // no dates at all
	}

	// resolve start
	if weekend && sDate == nil {
		var sat civil
		if today.weekday() == time.Sunday {
			sat = today
		} else {
			sat = weekdayFrom(today, time.Saturday, false)
		}
		if weekendNext {
			sat = sat.addDays(7)
		}
		sDate = &sat
	}
	if tonight && sClock == nil {
		sClock = &clock{20, 0}
	}
	var start time.Time
	switch {
	case sDate == nil && sClock != nil:
		start = today.at(sClock.h, sClock.m)
		if !start.After(p.now) {
			start = start.AddDate(0, 0, 1)
		}
	case sDate == nil:
		start = domain.NextHour(p.now)
	case sClock != nil:
		start = sDate.at(sClock.h, sClock.m)
	case *sDate == today:
		start = domain.NextHour(p.now)
	default:
		start = sDate.at(9, 0)
	}
	if start.Before(p.now) {
		start = domain.NextHour(p.now)
	}

	// resolve end
	var end time.Time
	sc := civilOf(start)
	startClock := clock{start.Hour(), start.Minute()}
	switch {
	case eDate != nil || endWeekday >= 0:
		e := civil{}
		if eDate != nil {
			e = *eDate
		} else {
			e = weekdayFrom(sc, endWeekday, true)
		}
		c := startClock
		if eClock != nil {
			c = *eClock
		}
		end = e.at(c.h, c.m)
		if !end.After(start) {
			end = time.Time{}
		}
	case eClock != nil:
		end = sc.at(eClock.h, eClock.m)
		if !end.After(start) {
			end = end.AddDate(0, 0, 1)
		}
	case dur > 0:
		end = start.Add(dur)
	case weekend:
		sunday := sc
		if sc.weekday() == time.Saturday {
			sunday = sc.addDays(1)
		}
		end = sunday.at(20, 0)
		if !end.After(start) {
			end = time.Time{}
		}
	}
	if end.IsZero() {
		end = start.Add(24 * time.Hour)
		r.DurationAssumed = true
	}
	if end.Sub(start) > domain.MaxTripDays*24*time.Hour {
		end = start.Add(domain.MaxTripDays * 24 * time.Hour)
	}
	r.Start, r.End = start, end
}

// --- sort, type, place -------------------------------------------------------

var reCheap = regexp.MustCompile(`\b(cheap|cheapest|cheaper|affordable|low cost|lowest price|budget)\b`)

func (p *parser) sorting(r *Result) {
	if p.take(reCheap, ChipSort) != nil {
		r.SortByPrice = true
	}
}

var typePatterns = []struct {
	re  *regexp.Regexp
	typ string
}{
	{regexp.MustCompile(`\b(?:electric scooters?|e-scooters?|electric bikes?|e-bikes?|electric cars?|electric|evs?|ather|ola s1|nexon ev)\b`), "ev"},
	{regexp.MustCompile(`\b(?:royal enfield|enfield|bullets?|motorcycles?|motorbikes?|bikes?|classic 350|himalayan|pulsar)\b`), "bike"},
	{regexp.MustCompile(`\b(?:scooters?|scooty|scootie|scooties|activa|jupiter|ntorq|dio|access 125)\b`), "scooter"},
	{regexp.MustCompile(`\b(?:cars?|suvs?|sedans?|hatchbacks?|swift|thar|creta|baleno|jeep)\b`), "car"},
	{regexp.MustCompile(`\b(?:bicycles?|cycles?|cycling|mtb)\b`), "cycle"},
}

func (p *parser) vehicleType(r *Result) {
	best, bestAt := -1, len(p.low)+1
	var bestM []int
	for i, tp := range typePatterns {
		for _, m := range tp.re.FindAllStringIndex(p.low, -1) {
			if p.free(m[0], m[1]) && m[0] < bestAt {
				best, bestAt, bestM = i, m[0], m
				break
			}
		}
	}
	if best >= 0 {
		p.mark(bestM[0], bestM[1], ChipType)
		r.Type = typePatterns[best].typ
	}
}

// findWord finds alias as a whole word in unused text.
func (p *parser) findWord(alias string) (int, bool) {
	from := 0
	for {
		i := strings.Index(p.low[from:], alias)
		if i < 0 {
			return 0, false
		}
		i += from
		j := i + len(alias)
		okL := i == 0 || !isWordByte(p.low[i-1])
		okR := j == len(p.low) || !isWordByte(p.low[j])
		if okL && okR && p.free(i, j) {
			return i, true
		}
		from = i + 1
	}
}

func (p *parser) place(r *Result, g Gazetteer) {
	type cand struct {
		alias string
		pl    Place
	}
	var cands []cand
	for _, pl := range g {
		for _, a := range pl.Aliases {
			if a = strings.TrimSpace(strings.ToLower(a)); a != "" {
				cands = append(cands, cand{a, pl})
			}
		}
	}
	sort.SliceStable(cands, func(i, j int) bool { return len(cands[i].alias) > len(cands[j].alias) })
	for _, c := range cands {
		if i, ok := p.findWord(c.alias); ok {
			p.mark(i, i+len(c.alias), ChipPlace)
			r.CitySlug, r.CityName, r.AreaID, r.AreaName = c.pl.CitySlug, c.pl.CityName, c.pl.AreaID, c.pl.AreaName
			// "calangute goa": the city name joins the same chip
			for _, o := range cands {
				if o.pl.CitySlug == c.pl.CitySlug && o.pl.AreaID == "" {
					if k, ok := p.findWord(o.alias); ok {
						p.mark(k, k+len(o.alias), ChipPlace)
					}
				}
			}
			return
		}
	}
}

// --- chips -------------------------------------------------------------------

func (p *parser) chips(r *Result) {
	add := func(kind, label string, assumed bool) {
		r.Chips = append(r.Chips, Chip{Kind: kind, Label: label, Spans: p.spanList(kind), Assumed: assumed})
	}
	if r.CitySlug != "" {
		label := r.CityName
		if r.AreaName != "" {
			label = r.AreaName + ", " + r.CityName
		}
		add(ChipPlace, label, false)
	} else {
		r.Missing = append(r.Missing, MissingPlace)
	}
	if r.HasDates() {
		add(ChipDates, domain.RangeText(r.Start, r.End), r.DurationAssumed)
	} else {
		r.Missing = append(r.Missing, MissingDates)
	}
	if r.Type != "" {
		add(ChipType, domain.TypeLabel(r.Type), false)
	}
	if r.MaxPerDay > 0 {
		add(ChipBudget, BudgetLabel(r.MaxPerDay, r.HasDates() && r.End.Sub(r.Start) > 24*time.Hour), false)
	}
	if r.SortByPrice {
		add(ChipSort, "Lowest price first", false)
	}
}

// BudgetLabel renders "Up to ₹800" or "Up to ₹800 a day".
func BudgetLabel(rupees int64, perDay bool) string {
	s := "Up to " + domain.Rupees(rupees).String()
	if perDay {
		s += " a day"
	}
	return s
}

func (p *parser) spanList(kind string) []Span {
	raw := append([][2]int(nil), p.spans[kind]...)
	sort.Slice(raw, func(i, j int) bool { return raw[i][0] < raw[j][0] })
	out := make([]Span, 0, len(raw))
	for _, s := range raw {
		out = append(out, Span{Start: utf16Len(p.text[:s[0]]), End: utf16Len(p.text[:s[1]])})
	}
	return out
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}
