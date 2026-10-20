# CLAUDE.md — Saarthi (Vehicle Rental Platform, LITE build)

Read this fully at the start of every session. Full spec: `BUILD_PROMPT.md`. Progress: `docs/PROGRESS.md`.

## Goal
A **fabulous-looking, smooth, responsive web app** for renting cars, bikes, scooters, EVs and cycles, with the
**basic features actually working** end to end on demo data. Frontend quality is the priority. Backend stays
small and simple. This is NOT a production marketplace: no real payments, no SMS, no email, no job queues.

## Machine constraints (Windows, ~15 GB free on C:)
- Only **Go** needs to be installed (`winget install GoLang.Go`). Nothing else.
- **No Docker, no WSL, no Postgres, no gcc/CGO, no make, no Node/npm, no Tailwind binary, no sqlc.**
- Everything must run with: `go run ./cmd/saarthi` → open http://localhost:8080
- Tests run with: `go test ./...` (no `-race`, it needs gcc).

## Stack (fixed)
- **Go** (latest stable, ≥ 1.22) — stdlib `net/http` with 1.22 routing patterns (`GET /v/{id}`), `html/template`, `embed`.
- **SQLite** via `modernc.org/sqlite` (pure Go, no CGO). One file: `data/saarthi.db`. Schema auto-created on startup; demo data auto-seeded when empty. `go run ./cmd/saarthi -reset` wipes and reseeds.
- **Passwords**: `golang.org/x/crypto/bcrypt`. **Sessions**: random token in a `sessions` table + HttpOnly cookie.
- **Frontend**: server-rendered `html/template` + **htmx 2** (partial updates) + **vanilla JS modules** for interactions
  + **Motion** (motion.dev, vanilla build) for springs + **Leaflet** for maps. All vendored as files in `web/static/vendor/`.
- **CSS**: hand-written, no framework. Design tokens as CSS custom properties in `web/static/css/tokens.css`,
  components in `web/static/css/app.css`. Light + dark theme.
- **Fonts**: Fraunces (display) + Inter (UI), self-hosted woff2 in `web/static/fonts/`.
- **Payments**: fake "Pay (demo)" sheet with Success / Fail buttons. No gateway.
- **Uploads**: saved to `data/uploads/`, images re-encoded with stdlib `image/jpeg` (strips EXIF), max 8 MB.

Allowed Go dependencies: `modernc.org/sqlite`, `golang.org/x/crypto`, `golang.org/x/image` (only if needed for WebP/resize). Nothing else without asking.

## Layout
```
cmd/saarthi/main.go         # flags: -addr, -reset
internal/app/               # server setup, routes, middleware (session, auth, role, recover, logging)
internal/handlers/          # renter.go, provider.go, admin.go, auth.go, pages.go
internal/domain/            # PURE logic + unit tests: money, pricing, askparser, booking status, deposit
internal/store/             # SQLite schema.sql (embedded), seed.go, queries (plain database/sql)
web/templates/              # layouts/, partials/, pages/{renter,provider,admin}/
web/static/{css,js,vendor,fonts,img}/
docs/PROGRESS.md docs/DECISIONS.md
README.md
```

## Conventions
- Money = `int64` paise. Never float. Display with Indian grouping: `₹1,23,456`.
- Times stored as UTC (RFC3339 text in SQLite), shown in `Asia/Kolkata`.
- Business rules live in `internal/domain` (pure, testable). Handlers stay thin.
- htmx request (`HX-Request: true`) → return partial template; else full page.
- UI copy from `BUILD_PROMPT.md` §6. No lorem ipsum. No "Amazing deal!!!" copy.
- Keep JS small and organised: one module per component in `web/static/js/`, wired via `data-*` attributes.

## Testing policy (IMPORTANT)
- Write **unit tests** for `internal/domain` as you build (pricing, askparser, status transitions, deposit math). Table-driven. `go test ./...` must pass before each commit.
- A few handler tests with `httptest` are welcome (booking overlap, auth/role guard).
- **NO end-to-end / browser tests** (Playwright, chromedp, Selenium, Puppeteer, Cypress) and no browser tooling installs **until every milestone L0–L7 is done and the user says go**. Until then, browser checks are manual by the user.
- Never delete or weaken a test to make it pass.

## Basic safety (keep, it's cheap)
- bcrypt passwords; HttpOnly + SameSite=Lax session cookie; role checks on every provider/admin route; providers only see their own shop's data; renters only their own trips and photos.
- Validate uploads (size + image magic bytes). Escape everything (`html/template` does this — never use `template.HTML` on user input).
- No secrets needed. Nothing in the repo that looks like a real key.

## Never do
- Never add Docker, Postgres, Redis, queues, Tailwind, Node, React, an ORM, or real payment/SMS/email integrations.
- Never use CGO-requiring packages (e.g. `mattn/go-sqlite3`).
- Never write e2e/browser tests before the user approves (see testing policy).
- Never copy branding, logos, images or copy from other companies. Vehicle images = original SVG line-art.

## How to work
1. Read this file + `docs/PROGRESS.md`.
2. Per milestone: build → unit tests → `go test ./...` → `go vet ./...` → tick PROGRESS.md → `git commit` → next milestone.
3. After each milestone, tell the user exactly what to click to see it working.
4. Ask the user only when truly blocked.
