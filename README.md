# Saarthi — Vehicle Rental Platform

A web app for renting scooters, bikes, cars, EVs and cycles from local rental shops (demo cities: Mount Abu and Goa), with separate dashboards for renters, shop owners and an admin.

All data is demo data. Payments are simulated with a "Pay (demo)" screen; no real money, SMS or email is involved.

## Features

### Renter
- **Natural-language search ("Ask Saarthi")**: type e.g. `scooter in mount abu tomorrow 9am for 2 days under 800` and it fills in city, type, dates and budget.
- Filter by vehicle type, price and features; results also shown as price pins on a map.
- Compare two vehicles side by side.
- Vehicle page with live price breakdown (hourly rate, daily cap, add-ons, GST, refundable deposit).
- Book and pay (demo success / fail), then get a **Pickup Pass** with a booking code that also works offline and can be added to a calendar (.ics).
- **Check-in and return photos**: a guided camera walk-around of the vehicle (4 angles for two-wheelers, 8 for cars, plus odometer and fuel).
- My Trips: every booking with its status, before/after photo comparison, deposit tracker and reviews.

### Provider (shop owner)
- **Today** screen: new requests to accept, pickups and returns due today.
- **Fleet**: add and edit vehicles in three steps (Basics → Photos → Pricing), publish or hide them.
- **Calendar**: see bookings per vehicle and block days for maintenance (drag across days).
- **Inspection**: after a return, compare photos, add charges (extra km, late fee, fuel) and release the rest of the deposit.
- Quick search with Ctrl K and keyboard shortcuts.

### Admin
- Overview with KPI tiles (bookings, revenue, active trips, deposit release time) and 30-day charts.
- Verify new shops; unverified shops are hidden from search.
- List of all bookings, filterable by status.

### General
- Accounts with login/sign-up and three roles; each role only sees its own data.
- Light and dark theme, responsive layout for phone and desktop, respects "reduced motion".
- Double-booking protection: two renters can never book the same vehicle for overlapping times.

## Tech stack

| Part | Technology |
|---|---|
| Backend | **Go** (standard library `net/http` router, `html/template`) |
| Frontend | **HTML** templates, **CSS** (hand-written, CSS variables for the light/dark theme) and **JavaScript** (plain ES modules) |
| Frontend libraries | htmx (partial page updates), Motion (animations), Leaflet (maps) — included as files in `web/static/vendor/`, no npm or build step |
| Database | **SQLite** (`modernc.org/sqlite`, pure Go — no C compiler needed) |
| Security | bcrypt password hashing, HttpOnly session cookies, role checks, Content-Security-Policy |

**How they connect:** the Go server reads data from SQLite, fills in the HTML templates and sends finished pages along with the CSS and JavaScript files. In the browser, htmx and the JavaScript modules send requests back to Go routes, which answer with small HTML fragments that are swapped into the page without a full reload.

## Folder structure

```
cmd/
  saarthi/            main program (run this)
  server/             same program, entry point used for hosting on Vercel
internal/
  app/                server setup, routes, middleware (sessions, role checks, security headers, logging)
  handlers/           HTTP handlers for renter, provider, admin and auth pages
  domain/             business rules: money, pricing, booking status, deposit, search-text parser
  store/              SQLite schema, demo data, database queries, image uploads
web/
  templates/          HTML templates: layouts/, partials/, pages/ (renter, provider/, admin/)
  static/css/         tokens.css (colours, spacing, fonts) and app.css (components)
  static/js/          one JavaScript module per component
  static/vendor/      htmx, Motion, Leaflet
  static/fonts/       Inter and Fraunces
  static/img/         icons and favicon
.github/workflows/    automatic checks (gofmt, vet, tests) on every push
```

## How to run

You only need Go installed (version 1.26 or newer).

```bash
winget install GoLang.Go
go run ./cmd/saarthi
start http://localhost:8080
```

Reopen the terminal after installing Go. On the first run the app creates `data/saarthi.db` and fills it with demo data.

Useful flags:

| Flag | What it does |
|---|---|
| `-reset` | Delete `data/` and create fresh demo data (demo dates are relative to the day they were created) |
| `-addr :9000` | Use another port |
| `-dev` | Read templates and CSS/JS from disk on every request, so edits show after a refresh |

## Demo accounts

Password for all accounts: `demo1234`. The login page also has one-click **Try as Renter / Provider / Admin** buttons.

| Role | Email | What to look at |
|---|---|---|
| Renter | `renter@saarthi.test` | Trips in every state (upcoming, on trip, returned, closed, cancelled) |
| Provider | `provider@saarthi.test` | Abu Riders, Mount Abu: 13 vehicles, requests to accept, an inspection to do |
| Provider | `provider2@saarthi.test` | Goa Wheels, Panaji |
| Admin | `admin@saarthi.test` | Anjuna Bike Point waiting for verification |

## Screenshots

> Add screenshots here.

| Home / search | Vehicle page | Pickup Pass |
|---|---|---|
| _screenshot_ | _screenshot_ | _screenshot_ |

| Provider – Today | Provider – Calendar | Admin – Overview |
|---|---|---|
| _screenshot_ | _screenshot_ | _screenshot_ |

## Tests

```bash
go test ./...
```

```bash
go vet ./...
```

- `internal/domain`: table-driven unit tests for pricing, the search-text parser, booking status changes and deposit settlement.
- `internal/store`: double-booking guard, payment, the full trip lifecycle, image upload processing.
- `internal/app`: renders every page for every role and checks that role restrictions work.

On Windows with Smart App Control, a test binary can be blocked ("An Application Control policy has blocked this file"); `go test -trimpath -ldflags="-s -w" ./...` works around it.

## Credits

Vendored libraries and fonts keep their own licences: htmx (0BSD), Motion (MIT), Leaflet (BSD-2), Lucide icons (ISC), Inter and Fraunces (SIL OFL). Full texts are in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md). Map tiles © OpenStreetMap contributors, © CARTO. Vehicle drawings and illustrations are original to this project.
