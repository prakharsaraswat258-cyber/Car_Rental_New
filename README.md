# Saarthi

[![CI](https://github.com/prakharsaraswat258-cyber/Car_Rental_New/actions/workflows/ci.yml/badge.svg)](https://github.com/prakharsaraswat258-cyber/Car_Rental_New/actions/workflows/ci.yml)

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
6. Open the pass again, turn the network off (DevTools → Network → Offline) and reload: it still opens.
7. Go to **Trips**. Open the *ready for pickup* Activa → **Start check-in** → **Start the walk-around**: a full-screen camera with an outline per angle (too dark or blurry asks for a retake; without a camera each tile opens the photo picker). Two-wheelers need 4 angles + the odometer, cars 8 angles + odometer + fuel. Swipe to start. Then **Return the vehicle** the same way.
8. Open the *closed* Activa trip: drag the before / after slider on each angle, and watch the Deposit Tracker coin journey. The Goa Classic 350 has an itemised extra-km deduction and a review to leave.
9. From any vehicle, tap the shop name for its profile page (`/s/goa-wheels`): rating, reviews, live fleet.

**Provider (Today → accept → calendar → inspect)**
1. Log in with **Try as Provider**. *Today* shows requests, pickups and returns.
2. **Accept** a Thar request; **Mark ready** a pickup.
3. Under *Waiting for inspection*, tap **Inspect and release deposit**. Extra km and the late fee are pre-filled; add a fuel line and watch the settlement update; **Release deposit**.
4. **Calendar**: drag across days on a row to block them for maintenance (on a phone, tap the first day, then the last; with a keyboard, arrows + Shift, then Enter). Undo is in the toast. Try blocking over a teal booking bar: it shakes, "Already booked". Tap a hatched block to remove it.
5. **Fleet**: toggle Live/Hidden. Open the *XPulse* draft: Publish stays disabled until it has a photo. Upload one, then Publish. **Add vehicle** walks Basics → Photos → Pricing with a live card preview.
6. Press **Ctrl K** (Cmd K on a Mac) to jump to any booking, vehicle or page. `?` lists the shortcuts: `N` new vehicle, `B` block days, `/` search.

**Admin (numbers → verify)**
1. **Try as Admin**. KPI tiles: bookings, revenue, active trips, average deposit release time. Below them, 30-day charts of bookings and revenue per day (hover or touch for the day's number; a table view sits under each).
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

GitHub Actions runs gofmt, `go vet` and `go test` on every push to `main` and every pull request. On Windows with Smart App Control, a test binary can be refused ("An Application Control policy has blocked this file"); `go test -trimpath -ldflags="-s -w" ./...` gets past it.

Domain rules (pricing, Ask Saarthi parser, booking states, deposit settlement) have table tests; the store tests cover the double-booking guard, payment, the full trip lifecycle and image processing; the app tests render every page for every role. Browser (end-to-end) tests are milestone L8 and wait for your go-ahead.

## Layout

```
cmd/saarthi/          main (run this locally); flags and startup live in internal/app/run.go
cmd/server/           same main, at the path Vercel's Go preset expects
internal/app/         routes, middleware (session, role guard, CSP, same-origin, logging, recover)
internal/handlers/    HTTP handlers, view models, template renderer, vehicle line-art
internal/domain/      pure rules + tests: money, pricing, booking states, deposit, askparser/
internal/store/       SQLite schema, demo seed, queries, image uploads
web/templates/        layouts/, partials/, pages/{renter,provider,admin}
web/static/           css/ (tokens.css, app.css), js/ (one module per component), vendor/, fonts/, img/
docs/                 PROGRESS.md (milestones), DECISIONS.md (why things are the way they are)
```

Third-party front-end files are vendored in `web/static/vendor` and `web/static/fonts`: htmx 2.0.11 (0BSD), Motion 12.43.0 (MIT), Leaflet 1.9.4 (BSD-2), Lucide icons 1.60.0 (ISC), Inter and Fraunces (SIL OFL). Full licence texts: [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md). Map tiles come from CARTO / OpenStreetMap at runtime (internet needed for the map only).

## Hosting

GitHub keeps the code; it can't run a Go server (GitHub Pages is static files only).

**Vercel** (set up in `vercel.json`): import the repo, and every push to `main` deploys. Vercel's Go preset builds `./cmd/server` (the same app as `./cmd/saarthi`, at the path Vercel looks for) and runs it as a server on `$PORT`. Vercel's disk is read-only except `/tmp`, so each new server instance seeds fresh demo data there. Bookings you make last while that instance is warm (minutes of idle time), then the demo resets. If Vercel runs two instances at once, they don't share data, so you may be logged out. That's fine for a demo; for lasting data, use a host with a disk.

**Anywhere else**: run `go run ./cmd/saarthi` (or a built binary) on a host that runs Go and gives it a disk for `data/`. `$PORT` sets the port, `$SAARTHI_DATA` the data folder.

It's a demo either way: data is seeded, and payments, SMS and email are simulated.
