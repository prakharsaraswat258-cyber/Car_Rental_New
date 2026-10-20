package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"saarthi/internal/domain"
)

// DemoPassword is the password of every seeded account.
const DemoPassword = "demo1234"

// Demo accounts used by the "Try as…" buttons.
const (
	DemoRenterEmail   = "renter@saarthi.test"
	DemoProviderEmail = "provider@saarthi.test"
	DemoAdminEmail    = "admin@saarthi.test"
)

// SeedIfEmpty fills an empty database with demo data and reports whether
// it did. Dates are relative to now (D-016).
func (s *Store) SeedIfEmpty(ctx context.Context) (bool, error) {
	var n int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(DemoPassword), bcrypt.DefaultCost)
	if err != nil {
		return false, err
	}
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		sd := &seeder{
			ctx: ctx, tx: tx, now: s.Now().UTC().Truncate(time.Second), uploads: s.UploadDir,
			hash: string(hash), ids: map[string]string{}, vehicles: map[string]seedVehicle{},
			addons: map[string][]seedAddon{},
		}
		sd.run()
		return sd.err
	})
	if err != nil {
		return false, fmt.Errorf("seed demo data: %w", err)
	}
	return true, nil
}

type seeder struct {
	ctx     context.Context
	tx      *sql.Tx
	now     time.Time
	uploads string
	hash    string
	err     error

	ids      map[string]string // seed key → row id
	vehicles map[string]seedVehicle
	addons   map[string][]seedAddon // shop key → add-ons
}

func (sd *seeder) exec(q string, args ...any) {
	if sd.err != nil {
		return
	}
	if _, err := sd.tx.ExecContext(sd.ctx, q, args...); err != nil {
		sd.err = fmt.Errorf("%w\n  in: %s", err, strings.TrimSpace(strings.SplitN(q, "(", 2)[0]))
	}
}

func (sd *seeder) id(key string) string {
	id, ok := sd.ids[key]
	if !ok {
		id = NewID()
		sd.ids[key] = id
	}
	return id
}

// day is offset days from today (IST) at hh:mm IST.
func (sd *seeder) day(offset, hh, mm int) time.Time {
	n := sd.now.In(domain.IST)
	return time.Date(n.Year(), n.Month(), n.Day()+offset, hh, mm, 0, 0, domain.IST)
}

// hour is the start of the current IST hour plus offset hours.
func (sd *seeder) hour(offset int) time.Time {
	n := sd.now.In(domain.IST)
	return time.Date(n.Year(), n.Month(), n.Day(), n.Hour()+offset, 0, 0, 0, domain.IST)
}

// past clamps t so seeded history never sits in the future.
func (sd *seeder) past(t time.Time) time.Time {
	if t.After(sd.now) {
		return sd.now
	}
	return t
}

func (sd *seeder) run() {
	sd.seedUsers()
	sd.seedPlaces()
	sd.seedShops()
	sd.seedVehicles()
	sd.seedAddons()
	sd.seedBlocks()
	sd.seedBookings()
}

// --- users -------------------------------------------------------------------

func (sd *seeder) seedUsers() {
	users := []struct {
		key, role, name, email, phone, licence string
		ageDays                                int
	}{
		{"riya", RoleRenter, "Riya Sharma", DemoRenterEmail, "+91 90000 10001", "verified", 120},
		{"kabir", RoleRenter, "Kabir Rao", "renter2@saarthi.test", "+91 90000 10002", "verified", 60},
		{"mahesh", RoleProvider, "Mahesh Purohit", DemoProviderEmail, "+91 90000 20001", "verified", 400},
		{"anita", RoleProvider, "Anita D'Souza", "provider2@saarthi.test", "+91 90000 20002", "verified", 300},
		{"joel", RoleProvider, "Joel Fernandes", "provider3@saarthi.test", "+91 90000 20003", "pending", 2},
		{"admin", RoleAdmin, "Saarthi Admin", DemoAdminEmail, "+91 90000 30001", "verified", 500},
	}
	for _, u := range users {
		sd.exec(`INSERT INTO users (id, role, name, email, password_hash, phone, licence_status, created_at)
			VALUES (?,?,?,?,?,?,?,?)`,
			sd.id(u.key), u.role, u.name, u.email, sd.hash, u.phone, u.licence, ts(sd.now.AddDate(0, 0, -u.ageDays)))
	}
}

// --- cities & areas ----------------------------------------------------------

func (sd *seeder) seedPlaces() {
	cities := []struct {
		key, name, slug, aliases string
		lat, lng                 float64
	}{
		{"abu", "Mount Abu", "mount-abu", "mount abu,mt abu,mt. abu,abu", 24.5926, 72.7156},
		{"goa", "Goa", "goa", "goa", 15.4989, 73.8278},
	}
	for _, c := range cities {
		sd.exec(`INSERT INTO cities (id, name, slug, aliases, lat, lng) VALUES (?,?,?,?,?,?)`,
			sd.id("city:"+c.key), c.name, c.slug, c.aliases, c.lat, c.lng)
	}
	areas := []struct {
		key, city, name, aliases string
		lat, lng                 float64
	}{
		{"nakki", "abu", "Nakki Lake", "nakki lake,nakki", 24.5936, 72.7076},
		{"polo", "abu", "Polo Ground", "polo ground,polo", 24.5986, 72.7148},
		{"abu-road", "abu", "Abu Road", "abu road,abu rd", 24.4797, 72.7825},
		{"panaji", "goa", "Panaji", "panaji,panjim", 15.4909, 73.8278},
		{"calangute", "goa", "Calangute", "calangute", 15.5439, 73.7553},
		{"anjuna", "goa", "Anjuna", "anjuna", 15.5733, 73.7407},
		{"margao", "goa", "Margao", "margao,madgaon", 15.2832, 73.9862},
	}
	for _, a := range areas {
		sd.exec(`INSERT INTO areas (id, city_id, name, aliases, lat, lng) VALUES (?,?,?,?,?,?)`,
			sd.id("area:"+a.key), sd.id("city:"+a.city), a.name, a.aliases, a.lat, a.lng)
	}
}

// --- shops -------------------------------------------------------------------

func (sd *seeder) seedShops() {
	shops := []struct {
		key, owner, name, slug, phone, whatsapp, address, city, area, status string
		lat, lng                                                             float64
		ageDays                                                              int
	}{
		{"abu", "mahesh", "Abu Riders", "abu-riders", "+91 90000 20001", "919000020001",
			"Shop 4, Nakki Lake Road, near Boat Point, Mount Abu 307501", "abu", "nakki", "verified", 24.5941, 72.7089, 400},
		{"goa", "anita", "Goa Wheels", "goa-wheels", "+91 90000 20002", "919000020002",
			"Opp. Municipal Garden, 18th June Road, Panaji, Goa 403001", "goa", "panaji", "verified", 15.4972, 73.8263, 300},
		{"anj", "joel", "Anjuna Bike Point", "anjuna-bike-point", "+91 90000 20003", "919000020003",
			"Flea Market Road, Anjuna, Goa 403509", "goa", "anjuna", "pending", 15.5746, 73.7416, 2},
	}
	for _, s := range shops {
		sd.exec(`INSERT INTO shops (id, owner_id, name, slug, phone, whatsapp, address, city_id, area_id, lat, lng, status, created_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			sd.id("shop:"+s.key), sd.id(s.owner), s.name, s.slug, s.phone, s.whatsapp, s.address,
			sd.id("city:"+s.city), sd.id("area:"+s.area), s.lat, s.lng, s.status, ts(sd.now.AddDate(0, 0, -s.ageDays)))
	}
}

// --- vehicles ----------------------------------------------------------------

type seedVehicle struct {
	key, shop, typ, make, model string
	year                        int
	reg, fuel, trans            string
	seats                       int
	features, desc              string
	hourly, daily, deposit      int64 // rupees
	kmDay                       int
	extraKm, lateFee            int64 // rupees
	minHours                    int
	instant                     bool
	status                      string
	odometer                    int
	rating                      float64
	ratingCount                 int
}

func (v seedVehicle) rates() domain.Rates {
	return domain.Rates{
		Hourly: domain.Rupees(v.hourly), Daily: domain.Rupees(v.daily), Deposit: domain.Rupees(v.deposit),
		KmLimitPerDay: v.kmDay, MinHours: v.minHours,
	}
}

var seedVehicles = []seedVehicle{
	// Abu Riders, Mount Abu
	{"abu-activa-1", "abu", "scooter", "Honda", "Activa 6G", 2024, "RJ 38 SA 4412", "petrol", "automatic", 2,
		"Helmet included,Under-seat storage,Phone charger",
		"Light and easy for the Nakki Lake loop and Sunset Point. Gearless, so anyone who rides a scooter will feel at home.",
		60, 450, 1500, 100, 3, 100, 2, true, "live", 8420, 4.8, 64},
	{"abu-activa-2", "abu", "scooter", "Honda", "Activa 6G", 2023, "RJ 38 SA 2207", "petrol", "automatic", 2,
		"Helmet included,Under-seat storage",
		"Our second Activa, same easy ride. Good for two people and a day bag.",
		60, 450, 1500, 100, 3, 100, 2, true, "live", 15210, 4.6, 41},
	{"abu-jupiter", "abu", "scooter", "TVS", "Jupiter", 2023, "RJ 38 SB 1180", "petrol", "automatic", 2,
		"Helmet included,Big boot,Front pocket",
		"Comfortable seat and a roomy boot for a full day of sightseeing.",
		60, 450, 1500, 100, 3, 100, 2, true, "live", 12260, 4.7, 38},
	{"abu-access", "abu", "scooter", "Suzuki", "Access 125", 2024, "RJ 38 SC 0921", "petrol", "automatic", 2,
		"Helmet included,Phone charger",
		"Smooth 125 cc that takes the climb to Guru Shikhar without fuss.",
		65, 480, 1500, 100, 3, 100, 2, true, "live", 5310, 4.5, 22},
	{"abu-classic", "abu", "bike", "Royal Enfield", "Classic 350", 2023, "RJ 38 BA 7731", "petrol", "manual", 2,
		"Helmet included,Saddle bags,USB charger",
		"The classic thump for the hill roads. Comfortable for two with luggage.",
		90, 900, 3000, 150, 5, 150, 2, true, "live", 21450, 4.9, 57},
	{"abu-hunter", "abu", "bike", "Royal Enfield", "Hunter 350", 2024, "RJ 38 BB 3140", "petrol", "manual", 2,
		"Helmet included,USB charger",
		"A lighter Enfield that handles the hairpins on the ghat road with ease.",
		85, 850, 3000, 150, 5, 150, 2, true, "live", 9870, 4.6, 19},
	{"abu-pulsar", "abu", "bike", "Bajaj", "Pulsar 150", 2022, "RJ 38 BC 5532", "petrol", "manual", 2,
		"Helmet included",
		"Dependable 150 cc for riders who want gears without the weight.",
		70, 650, 2000, 150, 4, 120, 2, true, "live", 28700, 4.4, 27},
	{"abu-swift", "abu", "car", "Maruti Suzuki", "Swift", 2023, "RJ 38 CA 6620", "petrol", "manual", 5,
		"AC,Bluetooth audio,Room for 2 bags",
		"Easy hatchback for the drive up from Abu Road station.",
		200, 2200, 5000, 250, 10, 300, 4, true, "live", 33100, 4.7, 33},
	{"abu-thar", "abu", "car", "Mahindra", "Thar", 2024, "RJ 38 CB 0099", "diesel", "manual", 4,
		"4x4,AC,Convertible top",
		"Open-top 4x4 for the Achalgarh and Trevor's Tank side roads. The shop confirms every Thar booking itself.",
		350, 3500, 8000, 250, 14, 400, 4, false, "live", 14200, 4.8, 12},
	{"abu-ather", "abu", "ev", "Ather", "450X", 2024, "RJ 38 EA 0410", "electric", "automatic", 2,
		"Fast charging,Map on dash,Helmet included",
		"Quiet electric scooter with about 100 km of range. Charging point at the shop.",
		80, 600, 2000, 90, 4, 120, 2, true, "live", 4120, 4.7, 18},
	{"abu-cycle", "abu", "cycle", "Hero", "Sprint Pro 21-speed", 2023, "", "none", "none", 1,
		"Helmet included,Lock,Bottle holder",
		"A geared cycle for slow mornings around the lake.",
		25, 200, 500, 0, 0, 50, 1, true, "live", 0, 4.5, 9},
	{"abu-dio", "abu", "scooter", "Honda", "Dio", 2021, "RJ 38 SA 1001", "petrol", "automatic", 2,
		"Helmet included",
		"Sporty little scooter. Off the road this week for a service.",
		55, 400, 1500, 100, 3, 100, 2, true, "hidden", 30400, 4.3, 46},
	{"abu-xpulse", "abu", "bike", "Hero", "XPulse 200 4V", 2024, "RJ 38 BD 2024", "petrol", "manual", 2,
		"Helmet included,Off-road tyres",
		"",
		100, 1000, 3000, 150, 5, 150, 2, true, "draft", 1200, 0, 0},

	// Goa Wheels, Panaji
	{"goa-activa", "goa", "scooter", "Honda", "Activa 6G", 2024, "GA 07 K 4410", "petrol", "automatic", 2,
		"Helmet included,Under-seat storage",
		"The easy way to hop between Panaji, Miramar and Dona Paula.",
		60, 450, 1500, 100, 3, 100, 2, true, "live", 6710, 4.7, 88},
	{"goa-ntorq", "goa", "scooter", "TVS", "Ntorq 125", 2023, "GA 07 K 2291", "petrol", "automatic", 2,
		"Helmet included,Phone charger,Digital console",
		"Peppy 125 cc scooter for the coastal road to the north beaches.",
		65, 500, 1500, 100, 3, 100, 2, true, "live", 11980, 4.6, 40},
	{"goa-classic", "goa", "bike", "Royal Enfield", "Classic 350", 2023, "GA 03 R 7781", "petrol", "manual", 2,
		"Helmet included,Saddle bags",
		"The beach-road classic. Steady, comfortable and easy to ride two-up.",
		90, 900, 3000, 150, 5, 150, 2, true, "live", 30610, 4.8, 71},
	{"goa-himalayan", "goa", "bike", "Royal Enfield", "Himalayan 450", 2024, "GA 03 R 1450", "petrol", "manual", 2,
		"Helmet included,Tank bag,Knuckle guards",
		"Adventure bike for the Western Ghats loop via Chorla.",
		140, 1400, 5000, 200, 6, 200, 2, true, "live", 7300, 4.9, 14},
	{"goa-swift", "goa", "car", "Maruti Suzuki", "Swift", 2024, "GA 07 F 6631", "petrol", "manual", 5,
		"AC,Bluetooth audio,Room for 2 bags",
		"Compact and easy to park in Panaji's narrow lanes.",
		200, 2200, 5000, 250, 10, 300, 4, true, "live", 18800, 4.6, 52},
	{"goa-baleno", "goa", "car", "Maruti Suzuki", "Baleno", 2023, "GA 07 F 3304", "petrol", "automatic", 5,
		"AC,Automatic,Apple CarPlay,Rear camera",
		"Automatic hatchback with a big boot for airport runs.",
		220, 2400, 5000, 250, 10, 300, 4, true, "live", 24100, 4.5, 21},
	{"goa-creta", "goa", "car", "Hyundai", "Creta", 2024, "GA 07 G 1188", "diesel", "automatic", 5,
		"AC,Automatic,Sunroof,Cruise control",
		"Roomy SUV for families heading south to Palolem.",
		300, 3200, 7000, 250, 12, 400, 4, true, "live", 15500, 4.8, 17},
	{"goa-nexon", "goa", "ev", "Tata", "Nexon EV", 2024, "GA 07 E 0042", "electric", "automatic", 5,
		"AC,Automatic,Fast charging,Connected car app",
		"Silent electric SUV with about 300 km of real range. Charging at the shop and around Panaji.",
		260, 2800, 6000, 250, 8, 350, 4, true, "live", 9900, 4.8, 26},
	{"goa-ola", "goa", "ev", "Ola", "S1 Pro", 2024, "GA 07 E 1170", "electric", "automatic", 2,
		"Helmet included,Big boot,Map on dash",
		"Electric scooter with about 150 km of range. Home charger included.",
		75, 550, 2000, 100, 4, 120, 2, true, "live", 6040, 4.3, 31},
	{"goa-cycle", "goa", "cycle", "Hercules", "Roadeo", 2023, "", "none", "none", 1,
		"Helmet included,Lock,Basket",
		"Easy cycle for the Fontainhas lanes and the Mandovi promenade.",
		25, 200, 500, 0, 0, 50, 1, true, "live", 0, 4.6, 15},

	// Anjuna Bike Point (pending verification — hidden from search until the admin verifies, D-017)
	{"anj-activa", "anj", "scooter", "Honda", "Activa 5G", 2021, "GA 08 K 1200", "petrol", "automatic", 2,
		"Helmet included",
		"Simple scooter for the Anjuna–Vagator stretch.",
		55, 400, 1000, 100, 3, 100, 2, true, "live", 24500, 0, 0},
	{"anj-pulsar", "anj", "bike", "Bajaj", "Pulsar 220F", 2022, "GA 08 B 0220", "petrol", "manual", 2,
		"Helmet included",
		"Sporty 220 cc for longer rides up the coast.",
		80, 750, 2500, 150, 5, 150, 2, true, "live", 19800, 0, 0},
}

func (sd *seeder) seedVehicles() {
	for i, v := range seedVehicles {
		sd.vehicles[v.key] = v
		sd.exec(`INSERT INTO vehicles (id, shop_id, type, make, model, year, reg_number, fuel, transmission, seats,
				features, description, hourly_paise, daily_paise, deposit_paise, km_limit_per_day, extra_km_paise,
				late_fee_per_hour_paise, min_hours, instant_book, status, odometer_km, rating_avg, rating_count, created_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			sd.id("veh:"+v.key), sd.id("shop:"+v.shop), v.typ, v.make, v.model, v.year, v.reg, v.fuel, v.trans, v.seats,
			v.features, v.desc, domain.Rupees(v.hourly), domain.Rupees(v.daily), domain.Rupees(v.deposit), v.kmDay,
			domain.Rupees(v.extraKm), domain.Rupees(v.lateFee), v.minHours, boolInt(v.instant), v.status, v.odometer,
			v.rating, v.ratingCount, ts(sd.now.AddDate(0, 0, -(200-i*5))))
	}
}

// --- add-ons -----------------------------------------------------------------

type seedAddon struct {
	key  string
	a    domain.Addon
	fits string
}

func (sd *seeder) seedAddons() {
	for _, shop := range []string{"abu", "goa", "anj"} {
		list := []seedAddon{
			{"helmet", domain.Addon{Name: "Extra helmet", Price: domain.Rupees(50), Per: domain.PerTrip}, domain.ClassTwoWheeler},
			{"phone", domain.Addon{Name: "Phone holder", Price: domain.Rupees(30), Per: domain.PerTrip}, ""},
			{"seat", domain.Addon{Name: "Child seat", Price: domain.Rupees(150), Per: domain.PerDay}, domain.ClassFourWheeler},
		}
		for i := range list {
			list[i].a.ID = sd.id("addon:" + shop + ":" + list[i].key)
			sd.exec(`INSERT INTO addons (id, shop_id, name, price_paise, per, fits) VALUES (?,?,?,?,?,?)`,
				list[i].a.ID, sd.id("shop:"+shop), list[i].a.Name, list[i].a.Price, string(list[i].a.Per), list[i].fits)
		}
		sd.addons[shop] = list
	}
}

// --- blocks ------------------------------------------------------------------

func (sd *seeder) seedBlocks() {
	blocks := []struct {
		vehicle    string
		start, end time.Time
		note       string
	}{
		{"abu-swift", sd.day(4, 0, 0), sd.day(6, 0, 0), "Service at the workshop"},
		{"abu-classic", sd.day(8, 0, 0), sd.day(9, 0, 0), "Kept for a regular customer"},
		{"goa-baleno", sd.day(2, 0, 0), sd.day(3, 0, 0), "Tyre change"},
	}
	for _, b := range blocks {
		sd.exec(`INSERT INTO blocks (id, vehicle_id, start_at, end_at, note) VALUES (?,?,?,?,?)`,
			NewID(), sd.id("veh:"+b.vehicle), ts(b.start), ts(b.end), b.note)
	}
}

// --- bookings ----------------------------------------------------------------

type seedBooking struct {
	key, renter, vehicle string
	start, end           time.Time
	status               domain.Status
	addons               []string // add-on keys
	created              time.Time
	odoStart, odoEnd     int
	returned             time.Time
	released             time.Time // deposit released (closed bookings)
	deductions           func(q domain.PriceQuote, v seedVehicle) []domain.Deduction
	review               *seedReview
	photos               []string // phases with proof photos
}

type seedReview struct {
	rating int
	body   string
}

func (sd *seeder) seedBookings() {
	list := []seedBooking{
		{ // Past, closed, reviewed
			key: "b1", renter: "riya", vehicle: "abu-activa-1",
			start: sd.day(-12, 9, 0), end: sd.day(-10, 9, 0), status: domain.StatusClosed,
			addons: []string{"helmet"}, created: sd.day(-14, 20, 15),
			odoStart: 7980, odoEnd: 8170, returned: sd.day(-10, 8, 35), released: sd.day(-10, 10, 40),
			review: &seedReview{5, "Spotless scooter and a quick deposit check. The shop pointed us to a quiet road up to Sunset Point."},
			photos: []string{"checkin", "return"},
		},
		{ // Past, closed with an extra-km deduction, waiting for a review
			key: "b2", renter: "riya", vehicle: "goa-classic",
			start: sd.day(-25, 10, 0), end: sd.day(-22, 10, 0), status: domain.StatusClosed,
			created:  sd.day(-30, 11, 0),
			odoStart: 30100, odoEnd: 30610, returned: sd.day(-22, 10, 20), released: sd.day(-22, 13, 5),
			deductions: func(q domain.PriceQuote, v seedVehicle) []domain.Deduction {
				return []domain.Deduction{domain.ExtraKm(30100, 30610, q.KmIncluded, domain.Rupees(v.extraKm))}
			},
			photos: []string{"checkin", "return"},
		},
		{ // Active right now
			key: "b3", renter: "riya", vehicle: "abu-classic",
			start: sd.day(-1, 10, 0), end: sd.day(1, 10, 0), status: domain.StatusActive,
			addons: []string{"helmet"}, created: sd.day(-4, 18, 40),
			odoStart: 21450, photos: []string{"checkin"},
		},
		{ // Returned 95 min late with 60 km over — waiting for the shop's inspection
			key: "b4", renter: "riya", vehicle: "abu-jupiter",
			start: sd.hour(-50), end: sd.hour(-2), status: domain.StatusReturned,
			created:  sd.day(-6, 12, 10),
			odoStart: 12000, odoEnd: 12260, returned: sd.hour(-2).Add(95 * time.Minute),
			photos: []string{"checkin", "return"},
		},
		{ // Ready for pickup in two hours — the renter can check in now
			key: "b5", renter: "riya", vehicle: "abu-activa-2",
			start: sd.hour(2), end: sd.hour(26), status: domain.StatusReady,
			addons: []string{"helmet"}, created: sd.now.Add(-46 * time.Hour),
		},
		{ // Upcoming, confirmed
			key: "b6", renter: "riya", vehicle: "goa-nexon",
			start: sd.day(6, 9, 0), end: sd.day(8, 9, 0), status: domain.StatusConfirmed,
			addons: []string{"seat"}, created: sd.now.Add(-26 * time.Hour),
		},
		{ // Paid, waiting for the shop (Thar has instant book off)
			key: "b7", renter: "riya", vehicle: "abu-thar",
			start: sd.day(3, 8, 0), end: sd.day(5, 20, 0), status: domain.StatusAwaitingShop,
			addons: []string{"phone"}, created: sd.now.Add(-10 * time.Minute),
		},
		{ // Cancelled long ago
			key: "b8", renter: "riya", vehicle: "goa-swift",
			start: sd.day(-40, 9, 0), end: sd.day(-38, 9, 0), status: domain.StatusCancelled,
			created: sd.day(-45, 10, 0),
		},
		// Another renter, so the provider's Today screen is busy.
		{ // Pickup later today
			key: "k1", renter: "kabir", vehicle: "abu-swift",
			start: sd.hour(3), end: sd.hour(51), status: domain.StatusConfirmed,
			addons: []string{"seat"}, created: sd.day(-3, 15, 0),
		},
		{ // On trip, due back today
			key: "k2", renter: "kabir", vehicle: "abu-hunter",
			start: sd.hour(-30), end: sd.hour(4), status: domain.StatusActive,
			created: sd.day(-5, 10, 0), odoStart: 9870, photos: []string{"checkin"},
		},
		{ // Second request waiting for the shop
			key: "k3", renter: "kabir", vehicle: "abu-thar",
			start: sd.day(9, 9, 0), end: sd.day(10, 9, 0), status: domain.StatusAwaitingShop,
			created: sd.now.Add(-35 * time.Minute),
		},
	}
	for _, b := range list {
		sd.booking(b)
	}
}

func (sd *seeder) booking(b seedBooking) {
	if sd.err != nil {
		return
	}
	v := sd.vehicles[b.vehicle]
	var addons []domain.Addon
	for _, key := range b.addons {
		for _, a := range sd.addons[v.shop] {
			if a.key == key {
				addons = append(addons, a.a)
			}
		}
	}
	q, err := domain.Quote(v.rates(), b.start, b.end, addons, domain.DefaultTaxBPS)
	if err != nil {
		sd.err = fmt.Errorf("booking %s: %w", b.key, err)
		return
	}
	lines, err := json.Marshal(q.Lines)
	if err != nil {
		sd.err = err
		return
	}

	id := sd.id("booking:" + b.key)
	events := sd.events(b, v, q)
	updated := events[len(events)-1].at

	sd.exec(`INSERT INTO bookings (id, code, renter_id, vehicle_id, shop_id, start_at, end_at, status,
			rent_paise, addons_paise, tax_paise, total_paise, deposit_paise, km_included, price_lines,
			odometer_start, odometer_end, returned_at, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, NewBookingCode(), sd.id(b.renter), sd.id("veh:"+b.vehicle), sd.id("shop:"+v.shop),
		ts(b.start), ts(b.end), string(b.status),
		q.Rent, q.Addons, q.Tax, q.Total, q.Deposit, q.KmIncluded, string(lines),
		nullInt(b.odoStart), nullInt(b.odoEnd), nullTime(b.returned), ts(b.created), ts(updated))

	for _, e := range events {
		sd.exec(`INSERT INTO booking_events (id, booking_id, status, note, created_at) VALUES (?,?,?,?,?)`,
			NewID(), id, string(e.status), e.note, ts(e.at))
	}

	// Deposit: held from payment until inspection; none for cancelled.
	var deductions []domain.Deduction
	if b.deductions != nil {
		deductions = b.deductions(q, v)
	}
	settle := domain.Settle(q.Deposit, deductions)
	switch b.status {
	case domain.StatusCancelled, domain.StatusDeclined, domain.StatusExpired, domain.StatusPendingPayment:
	case domain.StatusClosed:
		sd.exec(`INSERT INTO deposits (booking_id, held_paise, deducted_paise, status, released_at, credited_at) VALUES (?,?,?,?,?,?)`,
			id, q.Deposit, settle.Deducted, string(domain.DepositCredited), ts(b.released), ts(b.released.Add(domain.CreditDelay)))
	case domain.StatusReturned:
		sd.exec(`INSERT INTO deposits (booking_id, held_paise, status) VALUES (?,?,?)`, id, q.Deposit, string(domain.DepositInspection))
	default:
		sd.exec(`INSERT INTO deposits (booking_id, held_paise, status) VALUES (?,?,?)`, id, q.Deposit, string(domain.DepositHeld))
	}
	for _, d := range settle.Items {
		sd.exec(`INSERT INTO deductions (id, booking_id, kind, amount_paise, note) VALUES (?,?,?,?,?)`,
			NewID(), id, string(d.Kind), d.Amount, d.Note)
	}

	if b.review != nil {
		sd.exec(`INSERT INTO reviews (id, booking_id, author_id, vehicle_id, shop_id, rating, body, created_at) VALUES (?,?,?,?,?,?,?,?)`,
			NewID(), id, sd.id(b.renter), sd.id("veh:"+b.vehicle), sd.id("shop:"+v.shop), b.review.rating, b.review.body,
			ts(b.released.Add(3*time.Hour)))
	}

	for _, phase := range b.photos {
		sd.tripPhotos(id, phase, b, v)
	}
}

type seedEvent struct {
	status domain.Status
	at     time.Time
	note   string
}

// events rebuilds a believable history up to the booking's status.
func (sd *seeder) events(b seedBooking, v seedVehicle, q domain.PriceQuote) []seedEvent {
	paid := fmt.Sprintf("Paid %s (demo) · %s refundable deposit held", q.PayNow, q.Deposit)
	ev := []seedEvent{{domain.StatusPendingPayment, b.created, "Booking started"}}
	add := func(s domain.Status, at time.Time, note string) {
		ev = append(ev, seedEvent{s, sd.past(at), note})
	}
	if b.status == domain.StatusCancelled {
		add(domain.StatusConfirmed, b.created.Add(time.Minute), paid)
		add(domain.StatusCancelled, b.created.Add(48*time.Hour), "Cancelled by you · full refund")
		return ev
	}
	if !v.instant {
		add(domain.StatusAwaitingShop, b.created.Add(time.Minute), paid+" · waiting for the shop")
		if b.status == domain.StatusAwaitingShop {
			return ev
		}
		add(domain.StatusConfirmed, b.created.Add(12*time.Minute), "The shop accepted your request")
	} else {
		add(domain.StatusConfirmed, b.created.Add(time.Minute), paid)
	}
	order := []domain.Status{domain.StatusReady, domain.StatusActive, domain.StatusReturned, domain.StatusClosed}
	for _, s := range order {
		if !reached(b.status, s) {
			break
		}
		switch s {
		case domain.StatusReady:
			at := b.start.Add(-time.Hour)
			if at.After(sd.now.Add(-20 * time.Minute)) {
				at = sd.now.Add(-20 * time.Minute)
			}
			add(s, at, "Ready for pickup")
		case domain.StatusActive:
			add(s, b.start.Add(5*time.Minute), fmt.Sprintf("Trip started · odometer %s km", domain.Grouped(int64(b.odoStart))))
		case domain.StatusReturned:
			add(s, b.returned, fmt.Sprintf("Returned · odometer %s km", domain.Grouped(int64(b.odoEnd))))
		case domain.StatusClosed:
			var deds []domain.Deduction
			if b.deductions != nil {
				deds = b.deductions(q, v)
			}
			st := domain.Settle(q.Deposit, deds)
			add(s, b.released, fmt.Sprintf("Inspection done · %s refunded", st.Refund))
		}
	}
	return ev
}

// reached reports whether a booking now at cur has passed through s.
func reached(cur, s domain.Status) bool {
	rank := map[domain.Status]int{
		domain.StatusConfirmed: 1, domain.StatusReady: 2, domain.StatusActive: 3,
		domain.StatusReturned: 4, domain.StatusClosed: 5,
	}
	return rank[cur] >= rank[s] && rank[s] > 0
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullInt(n int) any {
	if n == 0 {
		return nil
	}
	return n
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return ts(t)
}
