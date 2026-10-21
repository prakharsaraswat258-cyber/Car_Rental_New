# UI/UX UPGRADE — addendum to BUILD_PROMPT.md (LITE)

This file **adds to** `BUILD_PROMPT.md`; it does not replace it. Where they conflict on frontend/UX, **this file wins**.
LITE constraints still apply: Go + `html/template` + hand-written CSS + vanilla JS + htmx + Motion + Leaflet only.
No new dependencies, no Node, no frameworks, no Docker. Everything stays on drive D:.

**Prime directive:** the frontend must feel like a top-tier product: as calm and refined as Claude, as clear as Airbnb, as alive as Uber's trip card,
as crisp as Linear. If a choice trades backend completeness for frontend quality, choose frontend quality.

---

## 1. Restore these frontend features (cut from LITE, now back in scope)

| Feature | Milestone | Spec |
|---|---|---|
| Full-screen check-in camera | L5 | `getUserMedia` (rear camera) with an original SVG silhouette overlay per angle; fallback to `<input type=file accept=image/* capture=environment>` when camera is denied/unavailable. **8 angles** (front, front-left, left, rear-left, rear, rear-right, right, front-right) + odometer photo + fuel photo. Two-wheelers use 4 angles + odometer. |
| Too-dark / blurry check | L5 | Client-side: sample the frame on a small canvas; if mean brightness < 60/255 or Laplacian variance too low → "Too dark to see scratches. Try again near light." with a retake button. |
| Angle-matched photo compare | L5, L7 | Trip detail and admin both show check-in vs return photos **side by side per angle**, with timestamps, and a slider to wipe between before/after. |
| Drag to block + hatched maintenance | L6 | Pointer-drag across day cells creates a ghost bar that snaps to whole days; maintenance blocks render with a diagonal hatch pattern; conflicts shake 3× and snap back. Keyboard fallback: select cells with arrows + Shift, Enter to block. |
| Command palette + shortcuts | L6 | Ctrl/Cmd K on provider and admin: jump to any page, find a booking by code, a vehicle by reg number. Shortcuts: `N` new vehicle, `B` block dates, `/` focus search, `?` shows a shortcuts sheet. Vanilla JS, results via htmx partial. |
| Status chips | L1 | Every status chip = colour + icon + word (never colour alone). |
| Shop profile page | L3 | `GET /s/{slug}`: header with shop name, area, rating, WhatsApp; grid of its vehicles; reviews. |
| Rating filter + distance sort | L2 | Add "4★ & up" filter and "Nearest" sort (haversine from area centre). |
| Admin charts | L7 | 30-day bookings and revenue as server-rendered inline SVG line charts (no chart library), with hover tooltips in vanilla JS. |
| Help popovers | L1 → all | A reusable "?" popover component; use it on every term in the tooltip table of BUILD_PROMPT §6. |
| "How pickup works" card | L4 | Dismissible 3-step card on the first Pickup Pass (dismissal saved in a cookie). |
| Offline Pickup Pass | L7 | Tiny hand-written service worker (`web/static/sw.js`) caching the app shell and visited `/trips/{code}/pass` pages, so the pass opens with no network. Web app manifest + icons so it can be installed on Android. |
| Add to calendar | L4 | Already in spec — keep the `.ics` download polished with the pass details. |

---

## 2. Raise the craft (apply to every screen)

**Layout & composition**
- 4 px spacing grid, strictly. Vertical rhythm from the type scale. Max content width 1200 px; reading width ≤ 68 ch.
- Home is a calm, centred composition like Claude's new-chat screen: generous whitespace, Fraunces greeting, the composer as the single hero. Below the fold: a horizontally scrollable category strip (Airbnb-style) and "Popular near you" cards.
- Sticky headers use a subtle translucent paper background with `backdrop-filter: blur(12px)` and a 1 px bottom border that appears only after scrolling.
- Vehicle detail gallery: 1 large + 4 small grid on desktop (Airbnb pattern), swipeable full-bleed carousel with dot indicator on mobile, tap to open a full-screen lightbox with pinch/zoom-friendly sizing.
- Pickup Pass styled like a wallet pass: rounded card, perforated divider, big booking code in tabular numerals, vehicle silhouette, QR-style pattern (decorative, generated SVG from the code).

**Illustrations & imagery**
- One original line-art illustration set (single stroke, `--ink-2`, 1.5 px) for: each vehicle type, every empty state, 404, 500, offline, and the onboarding checklist. Consistent stroke and corner style.
- Designed error pages: 404 "This road doesn't go anywhere" with a road-ends illustration and a Search button; 500 "Something broke on our side. Your bookings are safe." with Retry.
- Images always have width/height (no layout shift), `object-fit: cover`, a tinted placeholder background while loading, `loading="lazy"` below the fold, and a fade-in on load.

**Forms**
- Inline validation on blur, friendly copy under the field, never only red borders. Error summary at top for screen readers.
- Inputs have floating or top labels (never placeholder-only), clear buttons on search fields, correct `inputmode` / `autocomplete` (email, tel, numeric for odometer).
- Primary buttons show the wheel loader inside while submitting and are disabled to prevent double submits.

**Loading behaviour**
- Nothing shows for requests under 300 ms. If a skeleton appears, keep it at least 400 ms to avoid flicker.
- Skeletons match the real layout exactly (same card sizes).
- Optimistic UI for compare add/remove, theme switch, filter chip removal — update instantly, reconcile with htmx response.
- Preserve scroll position on back navigation from detail to results.

**Dark mode is designed, not inverted**
- Photos dimmed slightly (`filter: brightness(.92)`) in dark mode; borders carry depth instead of shadows; accent lightened (token already set); map tiles get a subtle dark filter.
- Check every screen in both themes before ticking a milestone.

**Touch & gestures (mobile)**
- Swipe gallery, drag-to-dismiss bottom sheets, swipe-to-confirm, pull-down to close the lightbox. All with keyboard/button alternatives.
- Tap targets ≥ 44 × 44 px; no hover-only affordances.
- Respect safe-area insets (`env(safe-area-inset-bottom)`) for the bottom tab bar and sticky price bar.

---

## 3. Accessibility (WCAG 2.2 AA — restore the full list)
- Contrast ≥ 4.5:1 text, ≥ 3:1 UI components. Never `--accent` for small text on `--paper`.
- Skip-to-content link; landmarks (`header`, `nav`, `main`, `footer`); `lang="en-IN"`.
- Visible 2 px focus ring in `--road` on everything focusable; logical tab order.
- Dialogs and sheets trap focus, close on Escape, return focus to the trigger.
- Composer announces parsed chips via an `aria-live="polite"` region; toasts announced politely, errors assertively.
- Date picker fully operable by arrow keys, PageUp/PageDown for months, Enter to select.
- Swipe-to-confirm has a button alternative ("Hold Enter to start" + a plain "Start trip" button for screen readers).
- Photos have alt text (angle + phase, e.g. "Return photo, front left").
- Everything works at 200% zoom and with `prefers-reduced-motion`.

---

## 4. Performance budget (check at the end of every milestone)
- Own JS ≤ 60 KB gzipped total, loaded as ES modules, deferred; only load Leaflet on pages with a map, Motion only where springs are used.
- CSS ≤ 50 KB gzipped. Preload the two main font files; `font-display: swap`; subset already done.
- Locally: LCP < 1.5 s, CLS < 0.05, no long tasks > 50 ms during animations.
- Animate only `transform` and `opacity`; use `will-change` only during an animation, remove after.
- Gzip responses in Go (`compress/gzip` middleware) and long cache headers for hashed or ETagged static files.

---

## 5. Design QA checklist (every screen, before ticking a milestone)
1. One primary action in `--accent`; nothing else competes.
2. Spacing on the 4 px grid; alignment clean at 360, 390, 768, 1024, 1440 px.
3. Empty, loading, error and success states all designed, with copy from the spec.
4. Copy is plain, numbers first, matches the voice rules; prices show the total.
5. Light and dark both checked.
6. Keyboard-only path works; focus visible; screen-reader labels present.
7. Motion: purposeful, correct tokens, smooth, and correct under reduced motion.
8. No layout shift on load; images sized.
9. Matches the component library (no one-off styles).
10. Would this screen look at home next to Claude, Airbnb or Linear? If not, iterate.

Record the checklist result per screen in `docs/PROGRESS.md`.

---

## 6. Review loop with the user (replaces automated visual testing until L8)
Browser automation stays banned until L8. So after each milestone from L1 onward:
1. Give the user a numbered click-path for every new screen and animation, including the exact URL and which demo account to use.
2. Ask the user to send screenshots: phone width and desktop, light and dark.
3. Critique the screenshots against the Design QA checklist, list concrete fixes, apply them, and only then move to the next milestone.

---

## 7. Pattern references (learn the pattern, never copy assets, colours or text)
| Pattern | Learn from | Use in Saarthi |
|---|---|---|
| Single calm composer, streaming feel, quiet sidebar | Claude | Home / Ask Saarthi, thinking-wheel loader |
| Search pill, category strip, gallery grid, total price | Airbnb | Results, detail |
| Live trip card always saying "what's next" | Uber | Trip detail, My trips active card |
| Wallet pass | Apple Wallet | Pickup Pass |
| Guided photo check-in | Turo | Check-in camera |
| Sidebar, Ctrl K, keyboard-first speed | Linear | Provider and admin |
| Itemised, trustworthy payment summary | Stripe Checkout | Checkout bill |

---

## 8. How to apply this
1. Read this file fully.
2. Update `docs/PROGRESS.md`: add the restored features to their milestones, add the Design QA checklist to every milestone from L1, add the performance budget check.
3. Log in `docs/DECISIONS.md` that `UI_UX_UPGRADE.md` overrides BUILD_PROMPT on frontend matters.
4. If L1 is already done, do a polish pass on it against §2–§5 before continuing.
5. Continue with the current milestone.
