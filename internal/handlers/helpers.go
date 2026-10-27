package handlers

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"saarthi/internal/domain"
	"saarthi/internal/store"
)

// SessionCookie holds the login token.
const SessionCookie = "saarthi_session"

// Toast is a message the front-end shows bottom-centre.
type Toast struct {
	Message string `json:"message"`
	Tone    string `json:"tone,omitempty"` // good | bad | warn | info
	Undo    *Undo  `json:"undo,omitempty"`
}

// Undo is the request the toast's Undo button sends.
type Undo struct {
	URL    string            `json:"url"`
	Method string            `json:"method"`
	Body   map[string]string `json:"body,omitempty"`
}

// asciiJSON marshals v with every non-ASCII rune escaped, so it is safe in
// HTTP headers and cookies (₹ would otherwise arrive as mojibake).
func asciiJSON(v any) string {
	b, _ := json.Marshal(v)
	var sb strings.Builder
	for _, r := range string(b) {
		if r < utf8.RuneSelf {
			sb.WriteRune(r)
			continue
		}
		if r > 0xFFFF {
			r1, r2 := 0xD800+((r-0x10000)>>10), 0xDC00+((r-0x10000)&0x3FF)
			fmt.Fprintf(&sb, `\u%04x\u%04x`, r1, r2)
			continue
		}
		fmt.Fprintf(&sb, `\u%04x`, r)
	}
	return sb.String()
}

// toastHeader asks htmx to fire a "toast" event after the swap.
func toastHeader(w http.ResponseWriter, t Toast) {
	w.Header().Set("HX-Trigger", asciiJSON(map[string]any{"toast": t}))
}

const flashCookie = "flash"

// setFlash queues a toast for the next full page load (after a redirect).
func setFlash(w http.ResponseWriter, t Toast) {
	http.SetCookie(w, &http.Cookie{Name: flashCookie, Value: base64.RawURLEncoding.EncodeToString([]byte(asciiJSON(t))),
		Path: "/", MaxAge: 60, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

func takeFlash(w http.ResponseWriter, r *http.Request) string {
	c, err := r.Cookie(flashCookie)
	if err != nil || c.Value == "" {
		return ""
	}
	http.SetCookie(w, &http.Cookie{Name: flashCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	b, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil || !json.Valid(b) {
		return ""
	}
	return string(b)
}

// redirect sends the browser elsewhere; htmx requests get HX-Redirect.
func redirect(w http.ResponseWriter, r *http.Request, to string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", to)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, to, http.StatusSeeOther)
}

// safeNext keeps post-login redirects on this site.
func safeNext(next string) string {
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.ContainsAny(next, "\\\r\n") {
		return ""
	}
	return next
}

// parseRange reads start/end (IST wall-clock "2006-01-02T15:04") from a form or query.
func parseRange(r *http.Request) (start, end time.Time, ok bool) {
	s, err1 := domain.ParseLocal(r.FormValue("start"))
	e, err2 := domain.ParseLocal(r.FormValue("end"))
	if err1 != nil || err2 != nil {
		return time.Time{}, time.Time{}, false
	}
	return s, e, true
}

func formInt(r *http.Request, name string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(strings.ReplaceAll(r.FormValue(name), ",", "")))
	return n
}

// formRupees reads a rupee amount ("1,500" or "1500.50") into paise.
func formRupees(r *http.Request, name string) domain.Paise {
	s := strings.TrimSpace(strings.ReplaceAll(strings.TrimPrefix(r.FormValue(name), "₹"), ",", ""))
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 {
		return 0
	}
	return domain.Paise(f*100 + 0.5)
}

// --- compare tray cookie ---------------------------------------------------

const compareCookie = "compare"

// CompareMax is how many vehicles the tray holds.
const CompareMax = 3

func compareIDs(r *http.Request) []string {
	c, err := r.Cookie(compareCookie)
	if err != nil {
		return nil
	}
	var out []string
	for _, id := range strings.Split(c.Value, ".") {
		if len(id) == 32 && isHex(id) && !contains(out, id) {
			out = append(out, id)
		}
	}
	if len(out) > CompareMax {
		out = out[:CompareMax]
	}
	return out
}

func setCompareIDs(w http.ResponseWriter, ids []string) {
	http.SetCookie(w, &http.Cookie{Name: compareCookie, Value: strings.Join(ids, "."), Path: "/",
		MaxAge: int((7 * 24 * time.Hour).Seconds()), HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

func isHex(s string) bool {
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// --- small helpers -----------------------------------------------------------

// vehicleURL links to a vehicle keeping the chosen dates.
func vehicleURL(id string, start, end time.Time) string {
	if start.IsZero() {
		return "/v/" + id
	}
	q := url.Values{"start": {domain.FormatLocal(start)}, "end": {domain.FormatLocal(end)}}
	return "/v/" + id + "?" + q.Encode()
}

// friendly turns store errors into renter-facing copy.
func friendly(err error) string {
	var taken *store.SlotTakenError
	switch {
	case errors.As(err, &taken):
		return "Someone just booked those dates. " + busyText(taken.Busy) + "."
	case errors.Is(err, store.ErrExpired):
		return "The 15-minute hold ran out, so nothing was charged. Pick the dates again to book."
	case errors.Is(err, store.ErrPaymentDeclined):
		return "Your bank declined it. Nothing was charged. Try again."
	case errors.Is(err, store.ErrNotBookable):
		return "This vehicle can't be booked right now."
	case errors.Is(err, store.ErrInPast):
		return "That pickup time has already passed. Pick a later time."
	case errors.Is(err, store.ErrPhotosMissing):
		return "Add all four angle photos first."
	case errors.Is(err, store.ErrOdometer):
		return "That odometer reading doesn't look right. Check the number on the dial."
	case errors.Is(err, domain.ErrEndBeforeStart):
		return "Return time must be after pickup time."
	case errors.Is(err, domain.ErrTripTooLong):
		return "Trips can be at most 30 days."
	case errors.Is(err, store.ErrBadTransition):
		return "That step isn't possible right now. Refresh to see the latest."
	case errors.Is(err, store.ErrNotFound):
		return "We couldn't find that."
	}
	return "Something went wrong on our side. Please try again."
}

// busyText is "Booked 9 to 11 Nov" for a busy range.
func busyText(b store.Busy) string {
	verb := "Booked"
	if b.Kind == "block" {
		verb = "Unavailable"
	}
	s, e := b.Start.In(domain.IST), b.End.Add(-time.Minute).In(domain.IST)
	switch {
	case domain.SameDay(s, e):
		return fmt.Sprintf("%s on %d %s", verb, s.Day(), s.Format("Jan"))
	case s.Month() == e.Month():
		return fmt.Sprintf("%s %d to %d %s", verb, s.Day(), e.Day(), e.Format("Jan"))
	}
	return fmt.Sprintf("%s %d %s to %d %s", verb, s.Day(), s.Format("Jan"), e.Day(), e.Format("Jan"))
}

func (h *Handlers) notFound(w http.ResponseWriter, r *http.Request) { h.NotFound(w, r) }

func badRequest(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusUnprocessableEntity)
	w.Write([]byte(msg))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func wantsJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json")
}

// toJSON is for data-* attributes (html/template escapes the quotes).
func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
