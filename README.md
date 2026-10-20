# Saarthi

Rent scooters, bikes, cars, EVs and cycles from local shops. A calm, fast web app in Go: server-rendered HTML, htmx, a little vanilla JS, SQLite. Demo data only — payments, SMS and email are simulated.

## Run it (3 commands)

```bash
winget install GoLang.Go
go run ./cmd/saarthi
start http://localhost:8080
```

Reopen the terminal after installing Go so `go` is on your PATH. The first run downloads two Go modules, creates `data/saarthi.db` and fills it with demo data.

| Flag | What it does |
|---|---|
| `-reset` | Wipe `data/` and reseed fresh demo data (demo dates are relative to the first run) |
| `-dev` | Read templates and static files from `./web` on every request: edit, then refresh |
| `-addr :9000` | Listen somewhere else |
| `-data path` | Keep the database and uploads in another folder |

## Demo accounts

Password for all: `demo1234`

| Role | Email | What's there |
|---|---|---|
| Renter | `renter@saarthi.test` | Trips in every state: closed + reviewed, active, returned awaiting inspection, ready for pickup, confirmed, waiting for the shop, cancelled |
| Provider | `provider@saarthi.test` | Abu Riders, Mount Abu: 13 vehicles, requests to accept, pickups and returns today, one inspection to do |
| Provider | `provider2@saarthi.test` | Goa Wheels, Panaji |
| Admin | `admin@saarthi.test` | Anjuna Bike Point is waiting for verification |

## Tests

```bash
go test ./...
```

```bash
go vet ./...
```

## Layout

```
cmd/saarthi/          main: flags, server, graceful shutdown
internal/app/         routes, middleware (session, role guard, CSP, same-origin, logging, recover)
internal/handlers/    HTTP handlers + template renderer
internal/domain/      pure rules + tests: money, pricing, booking states, deposit settlement
internal/store/       SQLite schema, demo seed, queries
web/templates/        layouts/, partials/, pages/
web/static/           css/ (tokens.css, app.css), js/, vendor/ (htmx, Motion, Leaflet), fonts/, img/
docs/                 PROGRESS.md (milestones), DECISIONS.md (why things are the way they are)
```

Third-party front-end files are vendored in `web/static/vendor` and `web/static/fonts`: htmx 2.0.11 (BSD-0), Motion 12.43.0 (MIT), Leaflet 1.9.4 (BSD-2), Lucide icons 1.60.0 (ISC), Inter and Fraunces (SIL OFL). Map tiles come from CARTO / OpenStreetMap at runtime.
