# PROGRESS — Saarthi LITE

Spec: `BUILD_PROMPT.md` · Rules: `CLAUDE.md` · Decisions: `docs/DECISIONS.md`

Every milestone ends with: `go test ./...` green · `go vet ./...` green · this file ticked · `git commit` · "how to see it" steps.

Legend: `[x]` done · `[ ]` todo · `[~]` partly done / pulled forward

---

## L0 — Setup & skeleton
- [x] Go 1.27.0 installed at `D:\Go`; GOPATH / GOCACHE / GOMODCACHE / GOTMPDIR on D: (D-028)
- [x] `git init`, `.gitignore` (data/, binaries)
- [x] `go.mod` (module `saarthi`), deps: `modernc.org/sqlite`, `golang.org/x/crypto`
- [x] Folder layout per CLAUDE.md (`cmd/`, `internal/{app,handlers,domain,store}`, `web/{templates,static}`)
- [x] `cmd/saarthi/main.go` with `-addr`, `-reset`, `-data`, `-dev`; graceful shutdown
- [x] SQLite schema (`internal/store/schema.sql`, embedded, `CREATE IF NOT EXISTS`), WAL, FKs, `_txlock=immediate`
- [x] Seed: 2 cities + 7 areas, 4 demo users + extras, 3 shops (1 pending), ~24 vehicles, add-ons, bookings in every state, deposits, events, review, placeholder trip photos
- [x] Pulled forward (seed needs them): `domain` money, pricing `Quote`, status transitions, return settlement + table tests
- [x] Base layout, theme cookie → `data-theme` (no flash), security headers, recover + logging + session middleware
- [x] Home placeholder showing live DB counts
- [x] README (3 commands to run)
- [x] `go mod tidy` (modernc.org/sqlite v1.60.1, x/crypto v0.58.0), `go vet ./...`, `go test ./...` green
- [x] Smoke run: `/` 200 with live counts, static MIME types, 404 page
- [x] Commit `L0`

## L1 — Design system
- [x] Vendor: htmx 2, Motion (vanilla), Leaflet, Lucide icons → sprite, Fraunces + Inter woff2 (latin + latin-ext for ₹)
- [x] `tokens.css`: colours (light/dark), type scale, spacing (4 px), radii, shadow, motion tokens
- [x] Theme toggle light / dark / system, 200 ms cross-fade
- [x] Brand mark (spoked wheel SVG), vehicle silhouettes (scooter, bike, car, ev, cycle)
- [x] Components: Button (4 variants + loading), Input, Select, Chip (filter/status/removable), VehicleCard, PriceBox, ItemisedBill, BottomSheet (drag to dismiss), Dialog, Toast (+Undo), Skeleton, EmptyState, StatusLine, DepositTracker, PhotoGrid, Tabs, Table, CalendarGrid, DateRangePicker, SwipeConfirm, PriceOdometer, WheelLoader, Tooltip, MapView
- [x] Layouts: renter (top bar desktop / bottom tabs mobile), console (sidebar + top bar)
- [x] `/dev/styleguide` shows every component + animation
- [x] `prefers-reduced-motion` → 120 ms fades, no rolling/flying
- [x] Commit (L1–L7 landed together, see log)

## L2 — Explore
- [x] `askparser.Parse(text, now, gazetteer)` — places/aliases, types, dates, times, durations, budget, assumed duration, Missing list, spans
- [x] ≥ 25 table tests (now = Tue 20 Oct 2026 15:00 IST), incl. the 5 spec cases
- [x] Home: greeting, composer (cycling placeholder 4 s), quick chips, "Popular near you"
- [x] `POST /ask` → chips; composer → chips FLIP animation; missing-info prompt with quick chips
- [x] Classic Where / When / What fallback form
- [x] `GET /search`: filters (type, fuel, transmission, max price), sort (price, rating), totals for dates
- [x] Map with price pins (Leaflet), map/list toggle on mobile
- [x] Skeletons (6 cards), empty state with "free a day later" suggestions
- [x] Commit (with L1)

## L3 — Vehicle & compare
- [x] `GET /v/{id}`: gallery, specs, what's included, rules, shop card + WhatsApp, availability calendar
- [x] `GET /v/{id}/quote` live price box (htmx) + price odometer
- [x] Conflict copy "Booked 9 to 11 Nov. Pick 12 Nov?"
- [x] Compare tray (≤ 3, replace prompt), compare sheet (best value bold), tray arc animation
- [x] Card → detail View Transition
- [x] Commit (with L1)

## L4 — Book
- [x] Login / signup (renter or provider), 3 "Try as…" demo buttons, login sheet on Book
- [x] Checkout: dates, add-on toggles, itemised bill, cancellation line
- [x] `POST /bookings` with overlap guard in one IMMEDIATE tx + test (touching ranges OK, blocks, unexpired pending)
- [x] Pay (demo) sheet: Success / Fail; fail copy; 15-min lazy expiry
- [x] Pickup Pass + reveal animation + `.ics`
- [x] My trips tabs (Upcoming / Active / Past), live card, 6-step status line, awaiting-shop amber state
- [x] Cancel
- [x] Handler tests: overlap, auth/role guard
- [x] Commit (with L1)

## L5 — Trip
- [x] Check-in / return pages: 4 angles + odometer, `capture="environment"`, angle fill, missing-photo copy
- [x] Upload validation (8 MB, magic bytes), re-encode JPEG (strip EXIF), `GET /photos/{id}` access check
- [x] SwipeConfirm start / end (+ hold Enter 1 s)
- [x] Deposit Tracker + coin journey, deductions itemised, lazy `credited` after 1 min
- [x] Review 1–5 stars after close, vehicle rating update
- [x] Commit (with L1)

## L6 — Provider
- [x] `/p` Today: requests, pickups today, returns today, onboarding checklist, empty state
- [x] Accept / Decline / Mark ready
- [x] Fleet grid with status chips; 3-step editor (Basics, Photos, Pricing) + live card preview; publish gate
- [x] Photo upload for vehicles; Live / Hidden toggle
- [x] Calendar: vehicles × 14 days, booking bars, blocks (click start + end), conflict shake + toast, delete block
- [x] Inspection: odometer end, auto extra-km + late fee, add deduction rows, cap with warning, release deposit
- [x] Settlement tests (already in L0, extend if needed)
- [x] Commit (with L1)

## L7 — Admin & polish
- [x] `/admin`: KPI tiles, shops Verify / Suspend, bookings table
- [~] Responsive pass 360 / 768 / 1200 (built mobile-first; needs your manual check in a browser)
- [~] Dark mode pass, contrast check (AA) (tokens checked on paper: accent 4.5:1, ink-2 5.3:1, warn text uses --warn-ink; visual pass is yours)
- [~] Keyboard + focus pass (focus rings, skip link, native dialogs, hold-Enter swipe; manual check is yours)
- [x] Every empty / loading / error state present
- [x] README demo walkthrough per role
- [x] Commit (with L1)

## L8 — End-to-end tests (ONLY after L0–L7 done AND user says go)
- [ ] Ask user first (browser download ~300–500 MB)
- [ ] playwright-go: Ask → book → pay → Pickup Pass; pay fail; provider accept; check-in → return → inspection → credited; block hides dates; admin verifies shop
- [ ] Commit `L8`

---

## Log
- 2026-10-20 — LITE spec adopted. Docs rewritten. L0 code written; waiting on Go install to build and test.
- 2026-10-20 — Go 1.27.0 installed on D:. L0 built, vetted and tested first try (gofmt only). L0 committed. Next: L1 design system.
- 2026-10-20 — User asked for everything in one go. L1–L7 built together and committed as one change: design system + styleguide, Ask Saarthi parser (33 cases), search/map/compare, vehicle detail + live quote, auth + checkout + demo pay, trips + pass + check-in/return + deposit tracker + reviews, provider console (today, fleet editor, calendar blocks, inspection), admin. `go test ./...` renders every page for every role. Browser checks left to the user per CLAUDE.md. L8 waits for the user's go-ahead.
