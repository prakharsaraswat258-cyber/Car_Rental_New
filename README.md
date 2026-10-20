# Saarthi

Rent scooters, bikes, cars, EVs and cycles from local shops in Mount Abu and Goa. A calm, fast web app in Go: server-rendered HTML, htmx, a little vanilla JS, SQLite. Demo data only. Payments, SMS and email are simulated.

## Run it (3 commands)

```bash
winget install GoLang.Go
go run ./cmd/saarthi
start http://localhost:8080
```

Reopen the terminal after installing Go so `go` is on your PATH. The first run downloads the Go modules, creates `data/saarthi.db` and fills it with demo data.

| Flag | What it does |
|---|---|
| `-reset` | Wipe `data/` and reseed fresh demo data. Demo dates are relative to when the data was seeded, so run this if "today" looks stale |
| `-dev` | Read templates and static files from `./web` on every request: edit, then refresh |
| `-addr :9000` | Listen somewhere else |
| `-data path` | Keep the database and uploads in another folder |

## Demo accounts

Every login page has one-tap **Try as Renter / Provider / Admin** buttons. Password for all: `demo1234`.

| Role | Email | What's there |
|---|---|---|
| Renter | `renter@saarthi.test` | Trips in every state: closed + reviewed, closed with a deduction, on trip, returned (being inspected), ready for pickup, confirmed, waiting for the shop, cancelled |
| Provider | `provider@saarthi.test` | Abu Riders, Mount Abu: 13 vehicles, 2 requests to accept, pickups and returns today, one inspection to do |
| Provider | `provider2@saarthi.test` | Goa Wheels, Panaji |
| Admin | `admin@saarthi.test` | Anjuna Bike Point is waiting for verification |

## 2-minute demo click-paths

**Renter (Explore → book → pay → pass)**
1. On the home page, type `scooter in mount abu tomorrow 9am for 2 days under 800` and press Search. Watch the words fly into chips, then the results slide up.
2. Try the type chips and *More* filters; on desktop the map shows price pins (hover a card to light its pin). On a phone, tap **Map**.
3. Tap the compare button on two cards: the picture arcs into the tray. Tap **Compare** for the side-by-side sheet.
4. Open a vehicle. Change the dates (the total rolls like an odometer) and tick an add-on. Tap **Book**, then **Try as Renter** in the login sheet.
5. On checkout tap **Pay**, then **Fail** (calm decline copy), then **Success**. The Pickup Pass rises and flips. Add it to your calendar (.ics).
6. Go to **Trips**. Open the *ready for pickup* Activa → **Start check-in** → add 4 photos (each fills its side of the silhouette) + odometer → swipe to start. Then **Return the vehicle** the same way.
7. Open the *closed* Activa trip to see the Deposit Tracker coin journey, or the Goa Classic 350 to see an itemised extra-km deduction and leave a review.

**Provider (Today → accept → calendar → inspect)**
1. Log in with **Try as Provider**. *Today* shows requests, pickups and returns.
2. **Accept** a Thar request; **Mark ready** a pickup.
3. Under *Waiting for inspection*, tap **Inspect and release deposit**. Extra km and the late fee are pre-filled; add a fuel line and watch the settlement update; **Release deposit**.
4. **Calendar**: tap a start day, then an end day on a row to block it (Undo in the toast). Try blocking over a teal booking bar: it shakes, "Already booked". Tap a grey block to remove it.
5. **Fleet**: toggle Live/Hidden. Open the *XPulse* draft: Publish stays disabled until it has a photo. Upload one, then Publish. **Add vehicle** walks Basics → Photos → Pricing with a live card preview.

**Admin (numbers → verify)**
1. **Try as Admin**. KPI tiles: bookings, revenue, active trips, average deposit release time.
2. **Verify** Anjuna Bike Point. Its scooter and bike now appear in a Goa search.
3. **Bookings** lists everything, filterable by status.

**Design system**: http://localhost:8080/dev/styleguide shows every component and animation with buttons to replay them. Use the theme switch (sun / moon / monitor) anywhere; set your OS to reduced motion to see everything turn into short fades.

## Tests

```bash
go test ./...
```

```bash
go vet ./...
```

Domain rules (pricing, Ask Saarthi parser, booking states, deposit settlement) have table tests; the store tests cover the double-booking guard, payment, the full trip lifecycle and image processing; the app tests render every page for every role. Browser (end-to-end) tests are milestone L8 and wait for your go-ahead.

## Layout

```
cmd/saarthi/          main: flags, server, graceful shutdown
internal/app/         routes, middleware (session, role guard, CSP, same-origin, logging, recover)
internal/handlers/    HTTP handlers, view models, template renderer, vehicle line-art
internal/domain/      pure rules + tests: money, pricing, booking states, deposit, askparser/
internal/store/       SQLite schema, demo seed, queries, image uploads
web/templates/        layouts/, partials/, pages/{renter,provider,admin}
web/static/           css/ (tokens.css, app.css), js/ (one module per component), vendor/, fonts/, img/
docs/                 PROGRESS.md (milestones), DECISIONS.md (why things are the way they are)
```

Third-party front-end files are vendored in `web/static/vendor` and `web/static/fonts`: htmx 2.0.11 (BSD-0), Motion 12.43.0 (MIT), Leaflet 1.9.4 (BSD-2), Lucide icons 1.60.0 (ISC), Inter and Fraunces (SIL OFL). Map tiles come from CARTO / OpenStreetMap at runtime (internet needed for the map only).
