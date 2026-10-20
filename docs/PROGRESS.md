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
- [ ] Vendor: htmx 2, Motion (vanilla), Leaflet, Lucide icons → sprite, Fraunces + Inter woff2 (latin + latin-ext for ₹)
- [ ] `tokens.css`: colours (light/dark), type scale, spacing (4 px), radii, shadow, motion tokens
- [ ] Theme toggle light / dark / system, 200 ms cross-fade
- [ ] Brand mark (spoked wheel SVG), vehicle silhouettes (scooter, bike, car, ev, cycle)
- [ ] Components: Button (4 variants + loading), Input, Select, Chip (filter/status/removable), VehicleCard, PriceBox, ItemisedBill, BottomSheet (drag to dismiss), Dialog, Toast (+Undo), Skeleton, EmptyState, StatusLine, DepositTracker, PhotoGrid, Tabs, Table, CalendarGrid, DateRangePicker, SwipeConfirm, PriceOdometer, WheelLoader, Tooltip, MapView
- [ ] Layouts: renter (top bar desktop / bottom tabs mobile), console (sidebar + top bar)
- [ ] `/dev/styleguide` shows every component + animation
- [ ] `prefers-reduced-motion` → 120 ms fades, no rolling/flying
- [ ] Commit `L1`

## L2 — Explore
- [ ] `askparser.Parse(text, now, gazetteer)` — places/aliases, types, dates, times, durations, budget, assumed duration, Missing list, spans
- [ ] ≥ 25 table tests (now = Tue 20 Oct 2026 15:00 IST), incl. the 5 spec cases
- [ ] Home: greeting, composer (cycling placeholder 4 s), quick chips, "Popular near you"
- [ ] `POST /ask` → chips; composer → chips FLIP animation; missing-info prompt with quick chips
- [ ] Classic Where / When / What fallback form
- [ ] `GET /search`: filters (type, fuel, transmission, max price), sort (price, rating), totals for dates
- [ ] Map with price pins (Leaflet), map/list toggle on mobile
- [ ] Skeletons (6 cards), empty state with "free a day later" suggestions
- [ ] Commit `L2`

## L3 — Vehicle & compare
- [ ] `GET /v/{id}`: gallery, specs, what's included, rules, shop card + WhatsApp, availability calendar
- [ ] `GET /v/{id}/quote` live price box (htmx) + price odometer
- [ ] Conflict copy "Booked 9 to 11 Nov. Pick 12 Nov?"
- [ ] Compare tray (≤ 3, replace prompt), compare sheet (best value bold), tray arc animation
- [ ] Card → detail View Transition
- [ ] Commit `L3`

## L4 — Book
- [ ] Login / signup (renter or provider), 3 "Try as…" demo buttons, login sheet on Book
- [ ] Checkout: dates, add-on toggles, itemised bill, cancellation line
- [ ] `POST /bookings` with overlap guard in one IMMEDIATE tx + test (touching ranges OK, blocks, unexpired pending)
- [ ] Pay (demo) sheet: Success / Fail; fail copy; 15-min lazy expiry
- [ ] Pickup Pass + reveal animation + `.ics`
- [ ] My trips tabs (Upcoming / Active / Past), live card, 6-step status line, awaiting-shop amber state
- [ ] Cancel
- [ ] Handler tests: overlap, auth/role guard
- [ ] Commit `L4`

## L5 — Trip
- [ ] Check-in / return pages: 4 angles + odometer, `capture="environment"`, angle fill, missing-photo copy
- [ ] Upload validation (8 MB, magic bytes), re-encode JPEG (strip EXIF), `GET /photos/{id}` access check
- [ ] SwipeConfirm start / end (+ hold Enter 1 s)
- [ ] Deposit Tracker + coin journey, deductions itemised, lazy `credited` after 1 min
- [ ] Review 1–5 stars after close, vehicle rating update
- [ ] Commit `L5`

## L6 — Provider
- [ ] `/p` Today: requests, pickups today, returns today, onboarding checklist, empty state
- [ ] Accept / Decline / Mark ready
- [ ] Fleet grid with status chips; 3-step editor (Basics, Photos, Pricing) + live card preview; publish gate
- [ ] Photo upload for vehicles; Live / Hidden toggle
- [ ] Calendar: vehicles × 14 days, booking bars, blocks (click start + end), conflict shake + toast, delete block
- [ ] Inspection: odometer end, auto extra-km + late fee, add deduction rows, cap with warning, release deposit
- [ ] Settlement tests (already in L0, extend if needed)
- [ ] Commit `L6`

## L7 — Admin & polish
- [ ] `/admin`: KPI tiles, shops Verify / Suspend, bookings table
- [ ] Responsive pass 360 / 768 / 1200
- [ ] Dark mode pass, contrast check (AA)
- [ ] Keyboard + focus pass
- [ ] Every empty / loading / error state present
- [ ] README demo walkthrough per role
- [ ] Commit `L7`

## L8 — End-to-end tests (ONLY after L0–L7 done AND user says go)
- [ ] Ask user first (browser download ~300–500 MB)
- [ ] playwright-go: Ask → book → pay → Pickup Pass; pay fail; provider accept; check-in → return → inspection → credited; block hides dates; admin verifies shop
- [ ] Commit `L8`

---

## Log
- 2026-10-20 — LITE spec adopted. Docs rewritten. L0 code written; waiting on Go install to build and test.
- 2026-10-20 — Go 1.27.0 installed on D:. L0 built, vetted and tested first try (gofmt only). L0 committed. Next: L1 design system.
