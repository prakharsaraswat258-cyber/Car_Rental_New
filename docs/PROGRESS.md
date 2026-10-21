# PROGRESS — Saarthi LITE

Spec: `BUILD_PROMPT.md` · Frontend addendum (wins on frontend): `UI_UX_UPGRADE.md` · Rules: `CLAUDE.md` · Decisions: `docs/DECISIONS.md`

Every milestone ends with: `go test ./...` green · `go vet ./...` green · Design QA checklist (UI_UX_UPGRADE §5) · performance budget (§4) · this file ticked · `git commit` · "how to see it" click-path + ask for screenshots (phone + desktop, light + dark).

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
- [x] Check-in / return pages: 4 angles + odometer, `capture="environment"`, angle fill, missing-photo copy (upgraded: shots per vehicle class, see U below)
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

## U — UI/UX upgrade (`UI_UX_UPGRADE.md`, applied across L1–L7)
Restored features (§1), by milestone:
- [x] L1 · Status chips = colour + icon + word (`statusIcon`), everywhere a status shows
- [x] L1 · "?" help popovers (`tip` partial, `help.js`): tap/Enter toggles, Esc / outside tap closes, focus returns
- [x] L1 · Illustration set at 1.5 px `--ink-2`: 6 side views, 6 front views, gauge, road-end (404), broken (500), offline, calendar, compare, quiet, checklist
- [x] L1 · Designed 404 / 500 / offline pages (`notfound`, `error`, `offline`); recover middleware renders the 500 page
- [x] L2 · Claude-like centred home, category strip with live counts, sticky header border only after scroll
- [x] L2 · "4★ & up" filter and "Nearest" sort (haversine from the area / city centre, distance on cards)
- [x] L2 · Skeleton only after 300 ms, shown ≥ 400 ms; optimistic chip removal; results scroll restored on Back
- [x] L3 · Gallery: 1 + 4 grid on desktop, swipe carousel + dots on phones, lightbox (arrows, swipe, pull down to close); line-art views when a vehicle has no photos
- [x] L3 · Shop profile `/s/{slug}`: rating, reviews, live fleet (verified shops only, unless owner / admin)
- [x] L3 · Compare toggle is optimistic, reverts on error
- [x] L4 · Wallet-style Pickup Pass with a QR-like pattern (decorative, the code is the source of truth)
- [x] L4 · "How pickup works" card, dismissal remembered in a cookie
- [x] L4 · Offline Pickup Pass: service worker (`/sw.js`), manifest, 192 / 512 icons; logging out clears saved passes
- [x] L4 · `.ics` with GEO, URL, STATUS, TRANSP
- [x] L4 · Date picker keyboard: arrows, PageUp / PageDown, Home / End, Enter; roving tabindex
- [x] L4 · Inline validation on blur + focused error summary (signup, vehicle editor); double-submit guard
- [x] L5 · Full-screen check-in camera (`getUserMedia`, rear camera) with a silhouette per angle; file picker fallback
- [x] L5 · Shots per class: cars 8 angles + odometer + fuel/charge; bikes & scooters 4 angles + odometer; cycles 4 angles (`domain.ProofShots`, tested)
- [x] L5 · Too dark (mean < 60) / blurry (Laplacian variance < 35) check, Retake or Use anyway — camera and picker both
- [x] L5 · Angle-matched before / after compare with a wipe slider (trip detail, provider booking, inspection, admin booking)
- [x] L6 · Calendar: drag across days (ghost bar snaps to days), hatched maintenance blocks, touch tap-tap, keyboard (arrows, Shift + arrows, Enter, Esc), Undo
- [x] L6 · Ctrl / Cmd K command palette (bookings, vehicles, pages; shops for admin) + shortcuts N, B, /, ?
- [x] L7 · Admin 30-day inline-SVG charts (bookings per day, revenue per day; one series each, one axis) with crosshair tooltip + table view; admin booking detail
- [x] L7 · Gzip middleware, lazy Leaflet (map pages only) and lazy Motion (springs only); dark-mode image dimming; tap targets ≥ 44 px; sized lazy images fade in
- [x] Demo history: 30 days of closed trips + reviews so charts and shop pages have data; `SchemaVersion = 2` asks for `-reset` on an old DB

Performance budget (§4), measured 2026-10-21:
- [x] Own JS 41.4 KB gzip (≤ 60) as deferred ES modules · CSS 19.4 KB gzip (≤ 50)
- [x] Leaflet (41.7 KB gz) loads only when a `[data-map]` exists; Motion (45.3 KB gz) only on the first spring; htmx 16.5 KB gz everywhere
- [x] Two main fonts preloaded, `font-display: swap`; gzip + weak ETags in Go; hashed static URLs cache forever
- [ ] LCP < 1.5 s, CLS < 0.05, long tasks: needs a browser (yours, DevTools → Lighthouse) until L8

Design QA checklist (§5), per screen. "Code" = checked by reading the markup/CSS and by `go test` rendering every page; "Eyes" = needs your screenshots (phone + desktop, light + dark).

| Screen | 1 one accent action | 3 states | 4 copy | 6 keyboard / labels | 7 motion + reduced | 8 no shift | 2, 5, 9, 10 visual |
|---|---|---|---|---|---|---|---|
| Home `/` | code ✓ | code ✓ | code ✓ | code ✓ | code ✓ | code ✓ | Eyes |
| Results `/search` | code ✓ | code ✓ | code ✓ | code ✓ | code ✓ | code ✓ | Eyes |
| Vehicle `/v/{id}` + lightbox | code ✓ | code ✓ | code ✓ | code ✓ | code ✓ | code ✓ | Eyes |
| Shop `/s/{slug}` | code ✓ | code ✓ | code ✓ | code ✓ | code ✓ | code ✓ | Eyes |
| Checkout + Pay sheet | code ✓ | code ✓ | code ✓ | code ✓ | code ✓ | code ✓ | Eyes |
| Pickup Pass | code ✓ | code ✓ | code ✓ | code ✓ | code ✓ | code ✓ | Eyes |
| My trips / trip detail | code ✓ | code ✓ | code ✓ | code ✓ | code ✓ | code ✓ | Eyes |
| Check-in camera / return | code ✓ | code ✓ | code ✓ | code ✓ | code ✓ | code ✓ | Eyes |
| Provider today / fleet / editor | code ✓ | code ✓ | code ✓ | code ✓ | code ✓ | code ✓ | Eyes |
| Provider calendar | code ✓ | code ✓ | code ✓ | code ✓ | code ✓ | code ✓ | Eyes |
| Inspection | code ✓ | code ✓ | code ✓ | code ✓ | code ✓ | code ✓ | Eyes |
| Admin overview + charts | code ✓ | code ✓ | code ✓ | code ✓ | code ✓ | code ✓ | Eyes |
| 404 / 500 / offline | code ✓ | n/a | code ✓ | code ✓ | code ✓ | code ✓ | Eyes |

- [ ] Review loop (§6): your screenshots → my critique against §5 → fixes. Waiting on you.

## L8 — End-to-end tests (ONLY after L0–L7 done AND user says go)
- [ ] Ask user first (browser download ~300–500 MB)
- [ ] playwright-go: Ask → book → pay → Pickup Pass; pay fail; provider accept; check-in → return → inspection → credited; block hides dates; admin verifies shop
- [ ] Commit `L8`

---

## Log
- 2026-10-20 — LITE spec adopted. Docs rewritten. L0 code written; waiting on Go install to build and test.
- 2026-10-20 — Go 1.27.0 installed on D:. L0 built, vetted and tested first try (gofmt only). L0 committed. Next: L1 design system.
- 2026-10-20 — User asked for everything in one go. L1–L7 built together and committed as one change: design system + styleguide, Ask Saarthi parser (33 cases), search/map/compare, vehicle detail + live quote, auth + checkout + demo pay, trips + pass + check-in/return + deposit tracker + reviews, provider console (today, fleet editor, calendar blocks, inspection), admin. `go test ./...` renders every page for every role. Browser checks left to the user per CLAUDE.md. L8 waits for the user's go-ahead.
- 2026-10-21 — `UI_UX_UPGRADE.md` adopted (overrides BUILD_PROMPT on frontend). Built all restored features + polish / a11y / performance pass in one go (section U). Schema v2 → start once with `-reset`. Design QA self-checked in code; visual review waits on the user's screenshots.
- 2026-10-21 — Hosted on Vercel (D-049, D-050): go preset via `vercel.json` + `cmd/server`, demo db in `/tmp`, identical seed on every instance, signed sessions, CDN-cached static files, browser-side photo shrink. Live: https://carrental-lac-three.vercel.app. Checked over HTTP: every role's pages, 60 parallel requests with one login, 35 parallel trip photos, a check-in upload, 3 full book → pay → pass flows. GitHub Actions CI added and green.
