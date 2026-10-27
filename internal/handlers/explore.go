package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"saarthi/internal/domain"
	"saarthi/internal/domain/askparser"
	"saarthi/internal/store"
)

// AskOption is a quick chip that completes a request ("Tomorrow").
type AskOption struct {
	Label  string `json:"label"`
	Append string `json:"append"`
}

// AskPrompt asks for the missing piece.
type AskPrompt struct {
	Title   string      `json:"title"`
	Options []AskOption `json:"options"`
}

// AskReply is what POST /ask returns (JSON for the composer animation).
type AskReply struct {
	Text    string           `json:"text"`
	Chips   []askparser.Chip `json:"chips"`
	Missing []string         `json:"missing"`
	URL     string           `json:"url"`
	Prompt  *AskPrompt       `json:"prompt,omitempty"`
}

// Ask parses the composer text into filters.
func (h *Handlers) Ask(w http.ResponseWriter, r *http.Request) {
	text := strings.TrimSpace(r.FormValue("q"))
	if rs := []rune(text); len(rs) > 200 {
		text = string(rs[:200])
	}
	g, err := h.Store.Gazetteer(r.Context())
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	res := askparser.Parse(text, h.Store.Now(), g)
	reply := AskReply{Text: text, Chips: res.Chips, Missing: res.Missing, URL: "/search?" + res.Query().Encode()}
	if reply.Chips == nil {
		reply.Chips = []askparser.Chip{}
	}
	if reply.Missing == nil {
		reply.Missing = []string{}
	}
	cities, _ := h.Store.Cities(r.Context())
	reply.Prompt = askPrompt(res, cities)

	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, reply)
		return
	}
	if reply.Prompt != nil && r.FormValue("anyway") == "" {
		h.home(w, r, &reply) // no-JS: ask for the missing piece on the page
		return
	}
	http.Redirect(w, r, reply.URL, http.StatusSeeOther)
}

func askPrompt(res askparser.Result, cities []store.City) *AskPrompt {
	if strings.TrimSpace(res.Text) == "" {
		return &AskPrompt{Title: "Describe the trip like a text: place, dates, vehicle, budget.",
			Options: []AskOption{{"Scooter in Goa tomorrow", "scooter in goa tomorrow"}, {"Bike in Mount Abu this weekend", "bike in mount abu this weekend"}}}
	}
	for _, m := range res.Missing {
		switch m {
		case askparser.MissingPlace:
			p := &AskPrompt{Title: "Where to?"}
			for _, c := range cities {
				p.Options = append(p.Options, AskOption{c.Name, "in " + strings.ToLower(c.Name)})
			}
			return p
		case askparser.MissingDates:
			return &AskPrompt{Title: "I didn't catch the dates. When do you need it?",
				Options: []AskOption{{"Today", "today"}, {"Tomorrow", "tomorrow"}, {"This weekend", "this weekend"}}}
		}
	}
	return nil
}

// --- search ------------------------------------------------------------------

// SearchForm is the classic Where / When / What form (and edit sheet).
type SearchForm struct {
	Cities     []store.City
	City, Area string
	Types      []string
	DRP        DRPView
	Max        string
	Q          string
}

// ChipView is a chip on the results page.
type ChipView struct {
	Kind, Label, Icon, RemoveURL string
	Assumed                      bool
}

// SearchData feeds the results page and its htmx fragment.
type SearchData struct {
	Q           string
	Query       string // current query string, for links
	Chips       []ChipView
	Cards       []CardView
	Suggestions []CardView
	Widened     string
	HasDates    bool
	Range       string
	Pins        string
	Types       []string
	Fuels       []string
	Trans       []string
	Max         string
	Sort        string
	Form        SearchForm
	MinRating   bool
	HasOrigin   bool
	AllTypes    []string
	AllFuels    []string
	AllTrans    []string
}

// Search lists available vehicles with totals for the chosen dates.
func (h *Handlers) Search(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	now := h.Store.Now()
	// classic form sends place=city:<slug> or place=area:<id>
	if place := q.Get("place"); place != "" || q.Has("place") {
		q.Del("place")
		switch {
		case strings.HasPrefix(place, "city:"):
			q.Set("city", strings.TrimPrefix(place, "city:"))
			q.Del("area")
		case strings.HasPrefix(place, "area:"):
			if a, c, err := h.Store.AreaByID(ctx, strings.TrimPrefix(place, "area:")); err == nil {
				q.Set("city", c.Slug)
				q.Set("area", a.ID)
			}
		default:
			q.Del("city")
			q.Del("area")
		}
		if q.Get("type") == "" {
			q.Del("type")
		}
		if q.Get("start") == "" || q.Get("end") == "" {
			q.Del("start")
			q.Del("end")
		}
		http.Redirect(w, r, "/search?"+q.Encode(), http.StatusSeeOther)
		return
	}
	f := store.SearchFilter{CitySlug: q.Get("city"), AreaID: q.Get("area"), Types: clean(q["type"], domain.VehicleTypes),
		Fuels: clean(q["fuel"], []string{"petrol", "diesel", "electric", "none"}), Trans: clean(q["trans"], []string{"manual", "automatic", "none"})}
	start, end, hasDates := parseRange(r)
	if hasDates && (!end.After(start) || end.Sub(start) > domain.MaxTripDays*24*time.Hour) {
		hasDates = false
	}
	if !hasDates {
		start, end = time.Time{}, time.Time{}
	}
	maxRupees, _ := strconv.ParseInt(q.Get("max"), 10, 64)
	minRating := q.Get("rating") == "4"
	sortBy := q.Get("sort")
	if sortBy != "rating" && sortBy != "nearest" {
		sortBy = "price"
	}
	// "Nearest" measures from the area centre, else the city centre
	var origin *[2]float64
	originName := ""
	if f.AreaID != "" {
		if a, _, err := h.Store.AreaByID(ctx, f.AreaID); err == nil {
			origin, originName = &[2]float64{a.Lat, a.Lng}, a.Name
		}
	} else if f.CitySlug != "" {
		if c, err := h.Store.CityBySlug(ctx, f.CitySlug); err == nil {
			origin, originName = &[2]float64{c.Lat, c.Lng}, c.Name+" centre"
		}
	}

	p := h.page(w, r, "Results · Saarthi", "explore", nil)
	find := func(f store.SearchFilter, start, end time.Time) ([]CardView, error) {
		cands, err := h.Store.CandidateVehicles(ctx, f)
		if err != nil {
			return nil, err
		}
		if !start.IsZero() {
			ids := make([]string, len(cands))
			for i, v := range cands {
				ids[i] = v.ID
			}
			busy, err := h.Store.Busy(ctx, ids, start, end)
			if err != nil {
				return nil, err
			}
			free := cands[:0]
			for _, v := range cands {
				if len(busy[v.ID]) == 0 {
					free = append(free, v)
				}
			}
			cands = free
		}
		cards := h.cards(cands, start, end, p.InCompare)
		if maxRupees > 0 {
			kept := cards[:0]
			for _, c := range cards {
				perDay := c.PerDay
				if c.Quote != nil {
					perDay = c.Quote.Total / domain.Paise(c.Quote.BillableDays)
				}
				if perDay <= domain.Rupees(maxRupees) {
					kept = append(kept, c)
				}
			}
			cards = kept
		}
		if minRating {
			kept := cards[:0]
			for _, c := range cards {
				if c.V.RatingCount > 0 && c.V.RatingAvg >= 4 {
					kept = append(kept, c)
				}
			}
			cards = kept
		}
		if origin != nil {
			for i := range cards {
				cards[i].Distance = domain.HaversineKm(origin[0], origin[1], cards[i].V.ShopLat, cards[i].V.ShopLng)
				cards[i].DistanceFrom = originName
			}
		}
		sort.SliceStable(cards, func(i, j int) bool {
			if sortBy == "nearest" && origin != nil && cards[i].Distance != cards[j].Distance {
				return cards[i].Distance < cards[j].Distance
			}
			if sortBy == "rating" {
				if cards[i].V.RatingAvg != cards[j].V.RatingAvg {
					return cards[i].V.RatingAvg > cards[j].V.RatingAvg
				}
				return cards[i].V.RatingCount > cards[j].V.RatingCount
			}
			return price(cards[i]) < price(cards[j])
		})
		return cards, nil
	}

	cards, err := find(f, start, end)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	data := SearchData{Q: q.Get("q"), Query: q.Encode(), HasDates: hasDates, Types: f.Types, Fuels: f.Fuels, Trans: f.Trans,
		Sort: sortBy, AllTypes: domain.VehicleTypes, AllFuels: []string{"petrol", "diesel", "electric"}, AllTrans: []string{"automatic", "manual"},
		MinRating: minRating, HasOrigin: origin != nil}
	if maxRupees > 0 {
		data.Max = strconv.FormatInt(maxRupees, 10)
	}
	if len(cards) == 0 && f.AreaID != "" {
		if a, c, err := h.Store.AreaByID(ctx, f.AreaID); err == nil {
			wide := f
			wide.AreaID = ""
			if more, err := find(wide, start, end); err == nil && len(more) > 0 {
				cards = more
				data.Widened = fmt.Sprintf("Nothing free in %s, so here is all of %s.", a.Name, c.Name)
			}
		}
	}
	data.Cards = cards
	if len(cards) == 0 && hasDates {
		later, err := find(f, start.Add(24*time.Hour), end.Add(24*time.Hour))
		if err == nil {
			if len(later) > 3 {
				later = later[:3]
			}
			for i := range later {
				later[i].FreeFrom = "Free from " + domain.DayText(start.Add(24*time.Hour))
			}
			data.Suggestions = later
		}
	}
	if hasDates {
		data.Range = domain.RangeText(start, end)
	}
	data.Pins = pins(append(append([]CardView{}, cards...), data.Suggestions...))
	data.Chips = h.searchChips(r, q, f, start, end, maxRupees, sortBy)
	cities, _ := h.Store.Cities(ctx)
	busyNone := []string{}
	data.Form = SearchForm{Cities: cities, City: f.CitySlug, Area: f.AreaID, Types: f.Types, DRP: newDRP(start, end, now, busyNone), Max: data.Max, Q: data.Q}

	p.Data = data
	if IsHTMX(r) && r.Header.Get("HX-Target") == "results" {
		h.Views.Fragment(w, http.StatusOK, "search-results", p)
		return
	}
	h.Views.Page(w, r, http.StatusOK, "search", p)
}

func price(c CardView) domain.Paise {
	if c.Quote != nil {
		return c.Quote.Total
	}
	return c.PerDay
}

func clean(vals, allowed []string) []string {
	var out []string
	for _, v := range vals {
		for _, part := range strings.Split(v, ",") {
			if part = strings.TrimSpace(part); contains(allowed, part) && !contains(out, part) {
				out = append(out, part)
			}
		}
	}
	return out
}

func (h *Handlers) searchChips(r *http.Request, q url.Values, f store.SearchFilter, start, end time.Time, maxRupees int64, sortBy string) []ChipView {
	without := func(keys ...string) string {
		c := url.Values{}
		for k, v := range q {
			c[k] = append([]string(nil), v...)
		}
		for _, k := range keys {
			c.Del(k)
		}
		c.Del("q")
		return "/search?" + c.Encode()
	}
	var chips []ChipView
	ctx := r.Context()
	if f.AreaID != "" {
		if a, c, err := h.Store.AreaByID(ctx, f.AreaID); err == nil {
			chips = append(chips, ChipView{Kind: "place", Icon: "map-pin", Label: a.Name + ", " + c.Name, RemoveURL: without("area", "city")})
		}
	} else if f.CitySlug != "" {
		if c, err := h.Store.CityBySlug(ctx, f.CitySlug); err == nil {
			chips = append(chips, ChipView{Kind: "place", Icon: "map-pin", Label: c.Name, RemoveURL: without("city")})
		}
	}
	if !start.IsZero() {
		chips = append(chips, ChipView{Kind: "dates", Icon: "calendar", Label: domain.RangeText(start, end),
			Assumed: q.Get("assumed") == "1", RemoveURL: without("start", "end", "assumed")})
	}
	for _, t := range f.Types {
		c := url.Values{}
		for k, v := range q {
			c[k] = append([]string(nil), v...)
		}
		c.Del("q")
		var rest []string
		for _, x := range f.Types {
			if x != t {
				rest = append(rest, x)
			}
		}
		c["type"] = rest
		chips = append(chips, ChipView{Kind: "type", Icon: typeIcon(t), Label: domain.TypeLabel(t), RemoveURL: "/search?" + c.Encode()})
	}
	if maxRupees > 0 {
		chips = append(chips, ChipView{Kind: "budget", Icon: "wallet",
			Label: askparser.BudgetLabel(maxRupees, !start.IsZero() && end.Sub(start) > 24*time.Hour), RemoveURL: without("max")})
	}
	if sortBy == "price" && q.Get("sort") == "price" {
		chips = append(chips, ChipView{Kind: "sort", Icon: "arrow-up-down", Label: "Lowest price first", RemoveURL: without("sort")})
	}
	if sortBy == "nearest" {
		chips = append(chips, ChipView{Kind: "sort", Icon: "navigation", Label: "Nearest first", RemoveURL: without("sort")})
	}
	if q.Get("rating") == "4" {
		chips = append(chips, ChipView{Kind: "rating", Icon: "star", Label: "4★ & up", RemoveURL: without("rating")})
	}
	return chips
}

func typeIcon(t string) string {
	switch t {
	case "car":
		return "car-front"
	case "ev":
		return "zap"
	case "cycle", "bike":
		return "bike"
	}
	return "gauge"
}
