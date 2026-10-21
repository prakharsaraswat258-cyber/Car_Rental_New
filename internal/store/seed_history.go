package store

import (
	"fmt"
	"math/rand"
	"time"

	"saarthi/internal/domain"
)

var historyReviews = []struct {
	rating int
	body   string
}{
	{5, "Clean, fuelled and ready on time. Deposit back the same evening."},
	{5, "Smooth ride up the ghat. The shop explained everything in two minutes."},
	{5, "Exactly as listed. Would book again."},
	{4, "Helmet was a bit loose, otherwise a lovely day out."},
	{5, "Easy pickup near the lake and a fair inspection."},
	{4, "Good value for a full day. Pickup took ten minutes longer than planned."},
	{5, "Battery lasted the whole day with charge to spare."},
	{5, "Friendly shop, quick WhatsApp replies, no surprises on the bill."},
	{3, "Fine for the price. A little worn, but it ran well."},
	{5, "The total I saw was the total I paid. That's rare."},
}

// seedHistory adds a month of closed trips by other renters, so the admin
// charts, KPIs and shop reviews have a believable shape. It is
// deterministic (fixed seed) and never touches the demo renter's list.
func (sd *seeder) seedHistory() {
	if sd.err != nil {
		return
	}
	rnd := rand.New(rand.NewSource(20261020))
	renters := []string{"kabir", "meera", "arjun", "sana", "dev"}
	var pool []seedVehicle
	for _, v := range seedVehicles {
		if v.status == "live" && v.shop != "anj" {
			pool = append(pool, v)
		}
	}
	freeAt := map[string]time.Time{}
	n := 0
	for d := -30; d <= -3; d++ {
		trips := rnd.Intn(4) // 0..3 a day
		if d%7 == -2 || d%7 == -3 {
			trips++ // weekends are busier
		}
		for i := 0; i < trips; i++ {
			v := pool[rnd.Intn(len(pool))]
			start := sd.day(d, 8+rnd.Intn(5), 0)
			if start.Before(freeAt[v.key]) {
				continue
			}
			hours := []int{4, 6, 24, 24, 48, 72}[rnd.Intn(6)]
			end := start.Add(time.Duration(hours) * time.Hour)
			if !end.Before(sd.day(-1, 0, 0)) {
				continue
			}
			freeAt[v.key] = end.Add(2 * time.Hour)
			returned := end.Add(time.Duration(rnd.Intn(50)-20) * time.Minute)
			released := returned.Add(time.Duration(60+rnd.Intn(180)) * time.Minute)
			n++
			b := seedBooking{
				key: fmt.Sprintf("h%d", n), renter: renters[rnd.Intn(len(renters))], vehicle: v.key,
				start: start, end: end, status: domain.StatusClosed,
				created:  start.Add(-time.Duration(12+rnd.Intn(80)) * time.Hour),
				odoStart: 10000 + rnd.Intn(20000), returned: returned, released: released,
			}
			if v.kmDay > 0 {
				b.odoEnd = b.odoStart + v.kmDay*((hours+23)/24)*(60+rnd.Intn(55))/100
			}
			if rnd.Intn(5) == 0 && v.kmDay > 0 {
				over := 20 + rnd.Intn(60)
				b.odoEnd = b.odoStart + v.kmDay*((hours+23)/24) + over
				b.deductions = func(q domain.PriceQuote, v seedVehicle) []domain.Deduction {
					return []domain.Deduction{domain.ExtraKm(b.odoStart, b.odoEnd, q.KmIncluded, domain.Rupees(v.extraKm))}
				}
			}
			if rnd.Intn(10) < 6 {
				r := historyReviews[rnd.Intn(len(historyReviews))]
				b.review = &seedReview{r.rating, r.body}
			}
			sd.booking(b)
		}
	}
}
