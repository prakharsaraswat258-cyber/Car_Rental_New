# BUILD PROMPT — Saarthi (LITE): fabulous frontend, basic features working, Go only

> **Message to paste into Claude Code:**
> "Scrap the previous plan. I replaced `CLAUDE.md` and `BUILD_PROMPT.md` with a LITE spec. Read both fully.
> Answers to your questions: (1) No Docker, no WSL, no Postgres, no gcc, no make. I will install only Go with `winget install GoLang.Go`; tell me when to run it. SQLite via `modernc.org/sqlite` (pure Go). (2) D-014: yes, an unpaid booking holds its slot for 15 minutes. (3) Yes, download only: htmx 2, Motion (vanilla), Leaflet, Lucide SVG icons, Fraunces and Inter woff2, plus the Go modules listed in CLAUDE.md. No Tailwind binary, no sqlc. (4) Yes, `git init` and commit after each milestone.
> Rewrite `docs/PROGRESS.md` (milestones L0–L8) and `docs/DECISIONS.md` for the LITE spec, give me a 5-line summary, then start L0."

You are a senior Go developer and a top-tier product designer. Build **Saarthi**, a vehicle rental web app where **the frontend is the star**: it must feel as calm and polished as Claude, as clear as Airbnb, as alive as Uber's trip card. The backend only needs to make the basic features truly work on demo data.

---

## 1. Product in short

Rent cars, bikes, scooters, EVs and cycles from local shops. Three roles:
- **Renter**: search, compare, book, pay (demo), track the trip, upload pickup/return photos, watch the deposit come back, review.
- **Provider** (shop): manage fleet, accept bookings, see a calendar, block dates, inspect returns.
- **Admin**: see numbers, verify shops, see all bookings.

**Signature ideas (make these shine):**
1. **Ask Saarthi** — one big composer: "scooter in Mount Abu tomorrow 9am for 2 days under ₹800" turns into filter chips.
2. **Total price everywhere** — rent + add-ons + tax. Deposit shown separately as refundable.
3. **Proof photos** — guided angle photos at pickup and return, timestamped.
4. **Deposit Tracker** — Held → Inspection → Released → Credited, with any deduction itemised.
5. **Price odometer** — totals roll digit by digit like a car odometer.

---

## 2. Features (basic but working)

| # | Feature | Role | Must work |
|---|---|---|---|
| 1 | Demo login | All | Email + password login and signup (renter or provider). Plus 3 **"Try as Renter / Provider / Admin"** buttons on the login page for instant demo login |
| 2 | Ask Saarthi | Renter | Parse text into city, area, dates, vehicle type, budget → chips; chips editable/removable; classic Where / When / What form as fallback |
| 3 | Search results | Renter | Cards with photo, name, total for chosen dates, deposit, km limit, rating; filters (type, fuel, transmission, max price); sort (price, rating); map with price pins (Leaflet) |
| 4 | Vehicle detail | Renter | Gallery, specs, rules, shop card with WhatsApp link, availability calendar, sticky price box that recalculates live (htmx) |
| 5 | Compare | Renter | Tray holds up to 3; compare sheet side by side |
| 6 | Checkout + demo pay | Renter | Dates, add-on toggles, itemised bill, "Pay (demo)" sheet with Success / Fail; success → booking confirmed (or "Waiting for the shop" if instant book is off) |
| 7 | Pickup Pass | Renter | Code, vehicle, address, time, what to bring, total paid, deposit held |
| 8 | My trips + status line | Renter | Upcoming / Active / Past; 6-step status line |
| 9 | Proof check-in & return | Renter | Upload 4 angle photos (front, back, left, right) + odometer reading; swipe to start / swipe to end |
| 10 | Deposit Tracker + review | Renter | Shows deposit states and deductions; 1–5 star review after close |
| 11 | Provider Today + requests | Provider | Pickups/returns today, pending requests with Accept / Decline, "Mark ready for pickup" |
| 12 | Fleet manager | Provider | Add / edit / hide vehicles, upload photos, set prices, deposit, km limit, instant book on/off |
| 13 | Calendar + blocks | Provider | Vehicles × days grid, bookings as bars, click-drag (or click start + end) to block dates |
| 14 | Inspection | Provider | After return: auto extra-km and late fee, add fuel/damage deduction with note → releases deposit |
| 15 | Admin lite | Admin | KPI tiles, shops list with Verify / Suspend, all bookings table |

**Not building:** real payments, OTP/SMS, email, Google login, GPS, chat (WhatsApp link instead), payouts, CSV, offline mode, background job queue, Docker, Postgres.

**Demo mode rule:** no waiting on real time. Renter can start check-in any time after "Ready for pickup"; provider inspection can happen right after return. Pending payments expire after 15 min (checked lazily on read, no background worker). An unpaid booking holds its slot during those 15 min.

---

## 3. Data (SQLite, `internal/store/schema.sql`, created on startup)

```sql
users(id TEXT PK, role TEXT CHECK(role IN('renter','provider','admin')), name, email UNIQUE, password_hash, phone, licence_status TEXT DEFAULT 'pending', created_at)
sessions(token TEXT PK, user_id, expires_at)
cities(id, name, slug, lat, lng)
areas(id, city_id, name, aliases TEXT /* comma list */, lat, lng)
shops(id, owner_id, name, slug, phone, whatsapp, address, city_id, area_id, lat, lng, status TEXT DEFAULT 'pending' /* pending|verified|suspended */, created_at)
vehicles(id, shop_id, type /* scooter|bike|car|ev|cycle */, make, model, year, reg_number, fuel, transmission, seats,
         features TEXT /* comma list */, description, hourly_paise INT, daily_paise INT, deposit_paise INT,
         km_limit_per_day INT, extra_km_paise INT, late_fee_per_hour_paise INT, min_hours INT DEFAULT 1,
         instant_book INT DEFAULT 1, status TEXT DEFAULT 'live' /* draft|live|hidden */, odometer_km INT,
         rating_avg REAL DEFAULT 0, rating_count INT DEFAULT 0, created_at)
vehicle_photos(id, vehicle_id, path, position)
addons(id, shop_id, name, price_paise, per /* trip|day */)
blocks(id, vehicle_id, start_at, end_at, note)
bookings(id, code UNIQUE, renter_id, vehicle_id, shop_id, start_at, end_at, status,
         rent_paise, addons_paise, tax_paise, total_paise, deposit_paise, km_included,
         price_lines TEXT /* JSON itemised lines */, odometer_start, odometer_end,
         returned_at, created_at, updated_at)
booking_events(id, booking_id, status, note, created_at)
trip_photos(id, booking_id, phase /* checkin|return */, angle /* front|back|left|right|odometer */, path, taken_at)
deposits(booking_id PK, held_paise, deducted_paise DEFAULT 0, status /* held|inspection|released|credited */, released_at, credited_at)
deductions(id, booking_id, kind /* extra_km|late|fuel|damage|cleaning */, amount_paise, note)
reviews(id, booking_id UNIQUE, author_id, vehicle_id, shop_id, rating INT, body, created_at)
```
IDs: random 16-byte hex. Booking code: `SAR-` + 5 chars from `23456789ABCDEFGHJKMNPQRSTVWXYZ`.

**No double booking:** inside one transaction (`BEGIN IMMEDIATE`), check overlap against bookings in active statuses (including unexpired `pending_payment`) and against blocks, then insert. Overlap rule: `existing.start < new.end AND new.start < existing.end` (touching ranges are fine). Write a test for this.

**Demo "Credited":** when the provider releases the deposit, status goes `released`; it flips to `credited` automatically 1 minute later (checked lazily on read) so the coin journey can be seen end to end.

---

## 4. Domain logic (pure Go + unit tests)

### Pricing — `Quote(vehicle, start, end, addons, taxBPS=1800)`
1. Reject `end <= start` or > 30 days.
2. `hours = ceil(minutes/60)`, at least `min_hours`.
3. `days = hours/24`, `rem = hours%24`, `rent = days*daily + min(rem*hourly, daily)`.
4. `billableDays = max(1, ceil(hours/24))`; `kmIncluded = km_limit_per_day * billableDays`.
5. Add-ons: per trip once, per day × billableDays.
6. `tax = round((rent+addons) * 18%)`; `total = rent + addons + tax`; `payNow = total + deposit`.
7. Return itemised lines used by every price box and bill so all screens match.

Tests (hourly ₹60, daily ₹500, min_hours 2): 1 h → ₹120 · 5 h → ₹300 · 10 h → ₹500 · 24 h → ₹500 · 26 h 10 min → ₹680 · 47 h → ₹1,000 · 72 h → ₹1,500. Plus tax rounding and an add-on per day over 3 days.

### Return settlement
- Late: grace 30 min, then `ceil(lateMinutes/60) × late_fee_per_hour`.
- Extra km: `max(0, odometer_end − odometer_start − km_included) × extra_km_paise`.
- Provider adds fuel/damage/cleaning with a note. Total deductions capped at deposit. Refund = deposit − deductions.

### Booking statuses
`pending_payment → awaiting_shop (instant book off) → confirmed → ready → active → returned → closed`, plus `cancelled`, `declined`, `expired`.
Status line for renters (6 steps): **Confirmed · Ready for pickup · Checked in · On trip · Returned · Closed**. `awaiting_shop` shows an amber "Waiting for the shop" state before step 1. Implement transitions as a table with who may fire each; test allowed and forbidden moves.

### Ask Saarthi parser — `askparser.Parse(text, now, gazetteer)`
Rule-based (no AI). Recognise:
- Places from cities/areas + aliases ("abu" → Mount Abu, "panjim" → Panaji, Goa).
- Types: scooter/scooty/activa/jupiter → scooter; bike/bullet/royal enfield/motorcycle → bike; car/suv/sedan/hatchback → car; ev/electric → ev; cycle/bicycle → cycle.
- Dates: today, tomorrow, day after tomorrow, tonight, this weekend, next <weekday>, "15 nov", "nov 15", "15/11".
- Times: 9am, 9:30 pm, 21:00, morning (9:00), afternoon (14:00), evening (18:00).
- Durations: "for 2 days", "3 hrs", "a week", "half day", "to 18 nov", "till sunday".
- Budget: under/below/max/upto ₹800, 800 rs, 1k, 1.5k.
- Missing duration → 1 day, chip marked "assumed". Missing place/dates → `Missing` list so UI asks "Where to?" / "When do you need it?".
- Return chips with the matched text span (for the fly-into-chips animation).

Tests with `now = Tue 20 Oct 2026 15:00 IST`, at least 25 cases, including:
| Input | Expected |
|---|---|
| `scooter in mount abu tomorrow 9am for 2 days under 800` | Mount Abu · scooter · 21 Oct 09:00 → 23 Oct 09:00 · max ₹800 |
| `bullet goa this weekend` | Goa · bike · Sat 24 Oct 09:00 → Sun 25 Oct 20:00 |
| `car panjim 15 nov to 18 nov` | Panaji, Goa · car · 15 Nov 09:00 → 18 Nov 09:00 |
| `ev for 3 hrs` | ev · 16:00 → 19:00 today · missing place |
| `cheap bike` | bike · missing place and dates |

---

## 5. Brand DNA and design system (this is where to spend effort)

**Personality:** the calm co-driver — warm, honest, unhurried. Inspired by Claude's warm paper canvas, serif + sans pairing and calm motion; Airbnb's search pill and total-price clarity; Uber's live trip card; Turo's photo check-in; Linear's crisp consoles. Learn patterns only; never copy their logos, assets or text.

**Principles:** the price is the whole price · proof over promises · one primary action per screen · calm motion, no confetti · money always visible.

### Tokens (`tokens.css`)
| Token | Light | Dark | Use |
|---|---|---|---|
| `--paper` | #FAF8F3 | #1B1A18 | Page background |
| `--surface` | #FFFFFF | #252420 | Cards, sheets, composer |
| `--ink` | #1F1D1A | #F2EFE7 | Text |
| `--ink-2` | #6B675F | #A8A397 | Secondary text |
| `--line` | #E7E3D8 | #3A3833 | Borders |
| `--accent` Saffron Clay | #C2552F | #E2774F | Primary button, selection, one highlight per screen |
| `--road` Highway Teal | #2E6B66 | #5FA39C | Maps, available, progress, focus ring |
| `--good` | #2F7D4F | #5DB37E | Confirmed, released |
| `--warn` | #B7791F | #E0A84B | Pending |
| `--bad` | #B3261E | #EF6B5E | Errors, damage |

Theme: follows system, with a toggle (light / dark / system) saved in a cookie and applied server-side as `data-theme` on `<html>` (no flash).

**Type:** Fraunces for page titles, home greeting and big totals; Inter for UI with `tabular-nums` on prices and times. Scale 12 / 14 / 16 / 20 / 24 / 32 / 48 px. Body ≥ 16 px on mobile.
**Shape:** 4 px grid; radius 8 inputs, 16 cards, 24 sheets, pill chips and composer. Borders over shadows; one soft shadow for floating things only.
**Icons:** Lucide SVG sprite. **Brand mark:** an original spoked-wheel SVG. **Vehicle images:** original single-line SVG silhouettes per type on a tinted card (seed data uses these; providers can upload real photos).

**Components** (template partials + CSS + small JS): Button (primary/secondary/ghost/danger, loading), Input, Select, Chip (filter/status/removable), VehicleCard, PriceBox, ItemisedBill, BottomSheet, Dialog, Toast (with Undo), Skeleton, EmptyState, StatusLine, DepositTracker, PhotoGrid, Tabs, Table, CalendarGrid, DateRangePicker, SwipeConfirm, PriceOdometer, WheelLoader, Tooltip, MapView. Show all of them on `/dev/styleguide`.

**Layouts:** Renter — top bar on desktop, bottom tabs on mobile (Explore, Trips, Account). Provider and admin — left sidebar + top bar. Mobile-first at 360 px; breakpoints 768 and 1200; tap targets ≥ 44 px.

---

## 6. Screens and copy

| Screen | Layout | Primary action | States |
|---|---|---|---|
| Home | Fraunces greeting "Where are you headed?"; big pill composer, placeholder cycles examples every 4 s; chips Scooter, Bike, Car, EV; "Popular near you" row | Search | Missing info: "I didn't catch the dates. When do you need it?" with quick chips (Today, Tomorrow, This weekend) |
| Results | Chips row; sort; cards; map right on desktop, toggle on mobile | Open a vehicle | Empty: "Nothing free for those dates. Here are 3 that are free a day later." · Loading: 6 skeleton cards |
| Vehicle detail | Gallery, specs grid, "What's included", rules in plain words, shop card, calendar (booked days struck through), sticky price box (bottom bar on mobile) | Book for ₹X | Conflict: "Booked 9 to 11 Nov. Pick 12 Nov?" |
| Compare | Floating tray (≤ 3 thumbs) → sheet with table; best value per row bold | Book this one | 4th add: "Compare holds 3. Replace one?" |
| Login | Card with email/password + 3 "Try as…" demo buttons; opens as a sheet when tapping Book while logged out | Continue | Wrong password: "That didn't match. Try again or use a demo account." |
| Checkout | Dates, add-on toggles, itemised bill (rent, add-ons, tax, total, refundable deposit, pay now), one-line cancellation rule | Pay ₹X | Fail: "Your bank declined it. Nothing was charged. Try again." |
| Pickup Pass | Card: code, vehicle, address + map link, time, bring licence + ID, total paid, deposit held | Add to calendar (.ics) | Awaiting shop: amber "Waiting for the shop, usually under 15 min" |
| My trips | Tabs; active trip pinned as live card | Open trip | Empty: line-art scooter, "No trips yet. Where to first?" |
| Trip detail | Status line; next-step card; photos; Deposit Tracker; WhatsApp the shop | The one next step | — |
| Check-in / return | Angle checklist with silhouette per angle, file input with `capture="environment"`, odometer field | Swipe to start / end | Photo missing: "Take the left side too, so old scratches are on record." |
| Provider Today | Cards: requests waiting, pickups today, returns today; onboarding checklist for new shops | Handle next | Empty: "Quiet day. Add photos to boost your listings." |
| Provider fleet / editor | Grid with status chips; editor in 3 steps (Basics, Photos, Pricing) with live card preview | Publish | Publish disabled until name, prices and 1 photo |
| Provider calendar | Vehicles × 14 days; teal booking bars; grey blocks; click start + end day to block | Block dates | Conflict: bar shakes, toast "Already booked" |
| Provider inspection | Odometer end, auto deductions pre-filled, add deduction rows | Release deposit | Over deposit: capped with warning |
| Admin | KPI tiles (bookings, revenue, active trips, avg deposit release time), shops table with Verify/Suspend, bookings table | Verify | — |

**Voice:** plain, numbers first. "₹2,340 total. Your ₹2,000 deposit returns after inspection." Use "Refundable deposit" and "Licence check", never "security deposit" or "KYC".

**Tooltips:** Composer — "Describe the trip like a text: place, dates, vehicle, budget" · Total — "Rent, fees and taxes for your dates. Deposit shown separately" · Deposit — "Held during the trip and returned after inspection" · km limit — "Free kilometres included; extra km cost ₹X each" · Late fee — "Charged per hour after your return time, after 30 min grace" · Proof photos — "Photos are timestamped so nobody can blame you later" · Hidden chip — "Not shown to renters. Toggle Live to publish".

---

## 7. Motion and animation (make it feel premium, not flashy)

Animate only `transform` and `opacity`. Respect `prefers-reduced-motion` (swap to 120 ms fades, no rolling or flying).

**Tokens:** fast 180 ms · base 260 ms · slow 400 ms · hero 600 ms · ease `cubic-bezier(0.2,0,0,1)` · enter `cubic-bezier(0.05,0.7,0.1,1)` · spring stiffness 260 / damping 30 · list stagger 30 ms (max 8).

| Animation | Where | Behaviour |
|---|---|---|
| Composer → chips | Home | On send, matched words lift out of the text and fly (FLIP) into the chips row; results slide up |
| Wheel loader | Waits > 300 ms | Spoked wheel turns with a dashed road line scrolling under it (replaces spinners; hook htmx events) |
| Price odometer | Detail, checkout | Changed digits roll vertically like a car odometer, 240 ms per digit, 20 ms stagger |
| Card → detail | Results → vehicle | View Transitions API: card photo grows into the gallery hero (htmx `transition:true`) |
| Into the tray | Compare | Thumbnail shrinks and arcs into the tray; counter bumps (spring) |
| Range fill | Date picker | End date fills the range as one sliding pill |
| Pickup Pass reveal | After pay | Pass rises and flips once; wheel makes one turn; `navigator.vibrate(15)`; no confetti |
| Road progress | Status line | Line between steps draws like road paint; current step pulses softly |
| Angle fill | Check-in | Each uploaded angle fills its part of the silhouette in teal; all filled unlocks the swipe |
| Swipe to confirm | Start / end trip | Thumb follows finger, track fills, springs back if released early; keyboard: hold Enter 1 s |
| Coin journey | Deposit Tracker | Coin travels Held → Inspection → Released → Credited; amount counts down for deductions |
| Calendar conflict | Provider calendar | Bar shakes 3 times and snaps back |

**Micro-interactions:** buttons scale 0.98 on press; cards lift 2 px on hover (desktop); skeletons pulse; toasts slide up with Undo; sheets drag to dismiss; theme switch cross-fades 200 ms; result cards stagger in.

---

## 8. Routes

Public: `GET /` · `POST /ask` · `GET /search` · `GET /v/{id}` · `GET /v/{id}/quote` (price partial) · `GET /v/{id}/calendar` · `POST|DELETE /compare/{id}` · `GET /compare` · `GET|POST /login` · `POST /login/demo/{role}` · `GET|POST /signup` · `POST /logout` · `GET /dev/styleguide` · `POST /theme`
Renter: `GET /checkout/{vehicleID}` · `POST /bookings` · `POST /bookings/{code}/pay` (demo success/fail) · `GET /trips` · `GET /trips/{code}` · `GET /trips/{code}/pass` · `GET /trips/{code}/pass.ics` · `POST /trips/{code}/cancel` · `GET /trips/{code}/{checkin|return}` · `POST /trips/{code}/photos` · `POST /trips/{code}/{start|end}` · `POST /trips/{code}/review` · `GET /photos/{id}` (owner/shop/admin only)
Provider (`/p`): `GET /p` · `POST /p/bookings/{id}/{accept|decline|ready}` · `GET /p/fleet` · `GET|POST /p/vehicles/new` · `GET|POST /p/vehicles/{id}/edit` · `POST /p/vehicles/{id}/photos` · `POST /p/vehicles/{id}/status` · `GET /p/calendar` · `POST /p/blocks` · `DELETE /p/blocks/{id}` · `GET|POST /p/bookings/{id}/inspect`
Admin (`/admin`): `GET /admin` · `POST /admin/shops/{id}/{verify|suspend}` · `GET /admin/bookings`

---

## 9. Seed data (auto on first run)
- Cities: Mount Abu (areas: Nakki Lake, Polo Ground, Abu Road) and Goa (Panaji/Panjim, Calangute, Anjuna, Margao).
- Users (password `demo1234`): `renter@saarthi.test`, `provider@saarthi.test` (Abu Riders, Mount Abu), `provider2@saarthi.test` (Goa Wheels), `admin@saarthi.test`. One extra pending shop for the admin to verify.
- ~24 vehicles with realistic prices: Activa ₹60/h · ₹450/day · deposit ₹1,500 · Classic 350 ₹90/h · ₹900/day · ₹3,000 · Swift ₹200/h · ₹2,200/day · ₹5,000 · Nexon EV ₹260/h · ₹2,800/day · ₹6,000 · cycle ₹25/h · ₹200/day · ₹500.
- Add-ons: extra helmet ₹50/trip, phone holder ₹30/trip, child seat ₹150/day.
- Bookings in every state for the demo renter (closed with review, active, returned awaiting inspection, awaiting shop, upcoming confirmed) so every screen has something to show.

---

## 10. Milestones (each ends with `go test ./...` + `go vet ./...` green, PROGRESS.md ticked, a git commit, and "how to see it" steps for the user)

- **L0 — Setup & skeleton.** Check Go is installed (if not, tell the user to run `winget install GoLang.Go` and reopen the terminal). `go mod init`, folder layout, server on :8080, SQLite schema + seed, base layout, README (3 commands max to run).
- **L1 — Design system.** Tokens, fonts, light/dark, all components and animations on `/dev/styleguide`, reduced-motion support. *Spend real effort here: it should look like a top product.*
- **L2 — Explore.** Home with Ask Saarthi (parser + tests + chips animation), results with filters, sort, map, skeletons, empty states.
- **L3 — Vehicle & compare.** Detail page, calendar, live quote with price odometer (pricing + tests), compare tray and sheet, card → detail transition.
- **L4 — Book.** Login/signup + demo buttons, checkout, demo pay sheet, double-booking guard (+ test), Pickup Pass reveal, My trips, status line.
- **L5 — Trip.** Check-in and return photos with angle fill, swipe to start/end, provider confirm, Deposit Tracker with coin journey, review.
- **L6 — Provider.** Today, requests accept/decline/ready, fleet + 3-step editor with live preview, calendar with blocks, inspection with auto deductions (+ tests).
- **L7 — Admin & polish.** Admin page; full responsive pass at 360 px / tablet / desktop; dark mode pass; keyboard and focus pass; contrast check; every empty/loading/error state present; README with demo walkthrough.
- **L8 — End-to-end tests (ONLY after L0–L7 are done AND the user says go).** Ask the user first, since it downloads a browser (~300–500 MB). Then playwright-go tests for: Ask Saarthi → book → pay → Pickup Pass; pay fail; provider accept; check-in → return → inspection → deposit credited; provider block hides dates; admin verifies shop.

## 11. Done means
All 15 features work on seed data in light and dark, on phone and desktop width; animations smooth and reduced-motion safe; `go test ./...` passes; README shows how to run in 3 commands and a 2-minute demo click-path for each role.
