package app

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"saarthi/internal/domain"
	"saarthi/internal/handlers"
	"saarthi/internal/store"
	"saarthi/web"
)

type env struct {
	t       *testing.T
	st      *store.Store
	h       http.Handler
	handler *handlers.Handlers
}

func newEnv(t *testing.T) *env {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	st.Now = func() time.Time { return time.Date(2026, 10, 20, 15, 0, 0, 0, domain.IST) }
	if _, err := st.SeedIfEmpty(context.Background()); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := New(Config{Store: st, Web: web.FS, Log: log, Dev: true})
	if err != nil {
		t.Fatal(err)
	}
	views, err := handlers.NewViews(web.FS, false, log)
	if err != nil {
		t.Fatal(err)
	}
	return &env{t: t, st: st, h: h, handler: &handlers.Handlers{Store: st, Views: views, Log: log}}
}

func (e *env) do(r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	return w
}

func (e *env) get(path string, c *http.Cookie, hdr ...string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", path, nil)
	if c != nil {
		r.AddCookie(c)
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		r.Header.Set(hdr[i], hdr[i+1])
	}
	return e.do(r)
}

func (e *env) post(path string, form url.Values, c *http.Cookie, hdr ...string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if c != nil {
		r.AddCookie(c)
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		r.Header.Set(hdr[i], hdr[i+1])
	}
	return e.do(r)
}

// login uses the one-tap demo buttons and returns the session cookie.
func (e *env) login(role string) *http.Cookie {
	e.t.Helper()
	w := e.post("/login/demo/"+role, url.Values{}, nil)
	if w.Code != http.StatusSeeOther {
		e.t.Fatalf("demo login %s = %d: %s", role, w.Code, w.Body)
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == SessionCookie {
			return c
		}
	}
	e.t.Fatalf("no session cookie for %s", role)
	return nil
}

func (e *env) id(q string, args ...any) string {
	e.t.Helper()
	var id string
	if err := e.st.DB.QueryRow(q, args...).Scan(&id); err != nil {
		e.t.Fatalf("%s: %v", q, err)
	}
	return id
}

// ok asserts a 200 page that rendered cleanly.
func ok(t *testing.T, path string, w *httptest.ResponseRecorder, wants ...string) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s = %d\n%s", path, w.Code, truncate(w.Body.String()))
	}
	body := w.Body.String()
	if i := strings.Index(body, "ZgotmplZ"); i >= 0 {
		lo, hi := max(0, i-160), min(len(body), i+40)
		t.Errorf("%s: html/template blocked an unsafe value (ZgotmplZ) near:\n%s", path, body[lo:hi])
	}
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("%s: missing %q", path, want)
		}
	}
}

func truncate(s string) string {
	if len(s) > 600 {
		return s[:600] + "…"
	}
	return s
}

func TestPublicPages(t *testing.T) {
	e := newEnv(t)
	activa := e.id(`SELECT id FROM vehicles WHERE reg_number = 'RJ 38 SA 4412'`)
	nakki := e.id(`SELECT id FROM areas WHERE name = 'Nakki Lake'`)
	pages := []struct {
		path  string
		wants []string
	}{
		{"/", []string{"Where are you headed?", "Popular near you", "data-composer"}},
		{"/search", []string{"vehicles"}},
		{"/search?city=goa&type=bike&start=2026-10-24T09:00&end=2026-10-25T20:00", []string{"Goa", "Bike", "total"}},
		{"/search?city=mount-abu&area=" + nakki + "&type=scooter&max=800&sort=price&start=2026-10-21T09:00&end=2026-10-23T09:00", []string{"Nakki Lake", "Up to ₹800 a day", "Lowest price first"}},
		{"/search?city=goa&type=cycle&start=2026-10-21T09:00&end=2026-10-21T12:00&max=1", nil},
		{"/v/" + activa, []string{"Honda Activa 6G", "a day, all-in", "What's included", "Availability"}},
		{"/v/" + activa + "?start=2026-11-02T09:00&end=2026-11-04T09:00", []string{"Book for", "Refundable deposit"}},
		{"/v/" + activa + "/quote?start=2026-11-02T09:00&end=2026-11-04T09:00", []string{"price-box", "total"}},
		{"/v/" + activa + "/calendar?month=2026-11", []string{"November 2026"}},
		{"/login", []string{"Try as Renter", "Try as Provider", "Try as Admin"}},
		{"/signup", []string{"Create your account"}},
		{"/signup?role=provider", []string{"Shop name"}},
		{"/compare", []string{"Nothing to compare yet"}},
		{"/dev/styleguide", []string{"Saarthi styleguide", "data-swipe", "data-deposit"}},
	}
	for _, p := range pages {
		ok(t, p.path, e.get(p.path, nil), p.wants...)
	}
	if w := e.get("/no/such/road", nil); w.Code != 404 || !strings.Contains(w.Body.String(), "doesn't go anywhere") {
		t.Errorf("404 page = %d", w.Code)
	}
}

func TestConflictSuggestsNextFreeDay(t *testing.T) {
	e := newEnv(t)
	// the Thar is booked day+3 08:00 → day+5 20:00 by the demo renter
	thar := e.id(`SELECT id FROM vehicles WHERE reg_number = 'RJ 38 CB 0099'`)
	w := e.get("/v/"+thar+"?start=2026-10-24T09:00&end=2026-10-25T09:00", nil)
	ok(t, "thar", w, "Booked 23 to 25 Oct.", "Pick 26 Oct?")
}

func TestAskJSON(t *testing.T) {
	e := newEnv(t)
	w := e.post("/ask", url.Values{"q": {"scooter in mount abu tomorrow 9am for 2 days under 800"}}, nil, "Accept", "application/json")
	var reply struct {
		Chips   []struct{ Kind, Label string }
		Missing []string
		URL     string
	}
	if err := json.Unmarshal(w.Body.Bytes(), &reply); err != nil {
		t.Fatalf("ask: %v %s", err, w.Body)
	}
	if len(reply.Chips) != 4 || len(reply.Missing) != 0 || !strings.Contains(reply.URL, "city=mount-abu") {
		t.Errorf("reply = %+v", reply)
	}
	// no-JS: missing dates re-renders home with the prompt
	w = e.post("/ask", url.Values{"q": {"bike in goa"}}, nil)
	ok(t, "/ask no-js", w, "catch the dates. When do you need it?", "This weekend")
	// complete text redirects to results
	w = e.post("/ask", url.Values{"q": {"bike in goa tomorrow"}}, nil)
	if w.Code != http.StatusSeeOther || !strings.HasPrefix(w.Header().Get("Location"), "/search?") {
		t.Errorf("ask redirect = %d %s", w.Code, w.Header().Get("Location"))
	}
}

func TestRenterFlow(t *testing.T) {
	e := newEnv(t)
	c := e.login(store.RoleRenter)

	ok(t, "/trips", e.get("/trips", c), "My trips", "On trip now")
	for _, tab := range []string{"upcoming", "active", "past"} {
		ok(t, "/trips?tab="+tab, e.get("/trips?tab="+tab, c))
	}
	ok(t, "/account", e.get("/account", c), "Licence check")

	rows, err := e.st.DB.Query(`SELECT b.code, b.status FROM bookings b JOIN users u ON u.id = b.renter_id WHERE u.email = ?`, store.DemoRenterEmail)
	if err != nil {
		t.Fatal(err)
	}
	type bk struct{ code, status string }
	var list []bk
	for rows.Next() {
		var b bk
		rows.Scan(&b.code, &b.status)
		list = append(list, b)
	}
	rows.Close()
	for _, b := range list {
		ok(t, "trip "+b.status, e.get("/trips/"+b.code, c), b.code)
		switch b.status {
		case "confirmed", "awaiting_shop", "ready":
			ok(t, "pass "+b.status, e.get("/trips/"+b.code+"/pass", c), "Pickup Pass", "Add to calendar")
			ics := e.get("/trips/"+b.code+"/pass.ics", c)
			if !strings.Contains(ics.Body.String(), "BEGIN:VEVENT") {
				t.Errorf("ics for %s: %s", b.code, ics.Body)
			}
		}
		switch b.status {
		case "ready":
			ok(t, "checkin", e.get("/trips/"+b.code+"/checkin", c), "Swipe to start trip", "data-silhouette")
		case "active":
			ok(t, "return", e.get("/trips/"+b.code+"/return", c), "Swipe to end trip")
		case "closed", "returned":
			ok(t, "deposit fragment", e.get("/trips/"+b.code+"/deposit", c), "Refundable deposit")
		}
	}

	// book → pay fail → pay success → Pickup Pass
	nexon := e.id(`SELECT id FROM vehicles WHERE reg_number = 'GA 07 E 1170'`)
	ok(t, "checkout", e.get("/checkout/"+nexon+"?start=2026-11-02T09:00&end=2026-11-03T09:00", c), "Review and pay", "Pay ₹")
	ok(t, "checkout htmx", e.get("/checkout/"+nexon+"?start=2026-11-02T09:00&end=2026-11-04T09:00", c, "HX-Request", "true"), "checkout-bill")
	w := e.post("/bookings", url.Values{"vehicle": {nexon}, "start": {"2026-11-02T09:00"}, "end": {"2026-11-03T09:00"}}, c, "HX-Request", "true")
	ok(t, "pay sheet", w, "Pay (demo)", "Success · pay")
	var code string
	e.st.DB.QueryRow(`SELECT code FROM bookings WHERE vehicle_id = ? AND status = 'pending_payment'`, nexon).Scan(&code)
	if code == "" {
		t.Fatal("no pending booking created")
	}
	w = e.post("/bookings/"+code+"/pay", url.Values{"result": {"fail"}}, c, "HX-Request", "true")
	ok(t, "pay fail", w, "Your bank declined it. Nothing was charged. Try again.")
	w = e.post("/bookings/"+code+"/pay", url.Values{"result": {"success"}}, c, "HX-Request", "true")
	if got := w.Header().Get("HX-Redirect"); got != "/trips/"+code+"/pass?new=1" {
		t.Fatalf("pay success redirect = %q (%d)", got, w.Code)
	}
	ok(t, "pass reveal", e.get("/trips/"+code+"/pass?new=1", c), "data-pass-reveal", code)

	// same slot again is refused
	w = e.post("/bookings", url.Values{"vehicle": {nexon}, "start": {"2026-11-02T12:00"}, "end": {"2026-11-02T18:00"}}, c, "HX-Request", "true")
	ok(t, "double booking", w, "Someone just booked those dates.")

	// compare tray
	w = e.post("/compare/"+nexon, url.Values{}, c, "X-Requested-With", "fetch")
	ok(t, "compare add", w, "compare-tray")
	var cc *http.Cookie
	for _, k := range w.Result().Cookies() {
		if k.Name == "compare" {
			cc = k
		}
	}
	if cc == nil {
		t.Fatal("no compare cookie")
	}
	r := httptest.NewRequest("GET", "/compare", nil)
	r.AddCookie(c)
	r.AddCookie(cc)
	r.Header.Set("HX-Request", "true")
	ok(t, "compare sheet", e.do(r), "Side by side", "Ola S1 Pro")
}

func TestProviderPages(t *testing.T) {
	e := newEnv(t)
	c := e.login(store.RoleProvider)
	ok(t, "/p", e.get("/p", c), "Today", "Requests waiting", "Accept")
	ok(t, "/p/fleet", e.get("/p/fleet", c), "Fleet", "Live")
	ok(t, "/p/calendar", e.get("/p/calendar", c), "data-pcal", "pcal-bar")
	ok(t, "/p/vehicles/new", e.get("/p/vehicles/new", c), "Add a vehicle")
	shop := e.id(`SELECT s.id FROM shops s JOIN users u ON u.id = s.owner_id WHERE u.email = ?`, store.DemoProviderEmail)
	v := e.id(`SELECT id FROM vehicles WHERE shop_id = ? AND status = 'draft'`, shop)
	ok(t, "edit", e.get("/p/vehicles/"+v+"/edit?step=photos", c), "Publish", "photo-manage")

	returned := e.id(`SELECT id FROM bookings WHERE shop_id = ? AND status = 'returned'`, shop)
	ok(t, "booking", e.get("/p/bookings/"+returned, c), "Timeline")
	ok(t, "inspect", e.get("/p/bookings/"+returned+"/inspect", c), "Inspect the return", "Extra kilometres", "Late return")
	w := e.post("/p/bookings/"+returned+"/inspect", url.Values{"odo_end": {"12260"}, "kind": {"fuel"}, "amount": {"100"}, "note": {"Quarter tank short"}}, c, "HX-Request", "true")
	ok(t, "inspect preview", w, "Kept", "Quarter tank short")
	w = e.post("/p/bookings/"+returned+"/inspect", url.Values{"odo_end": {"12260"}}, c)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("release deposit = %d %s", w.Code, truncate(w.Body.String()))
	}
	var st string
	e.st.DB.QueryRow(`SELECT status FROM bookings WHERE id = ?`, returned).Scan(&st)
	if st != "closed" {
		t.Errorf("after inspection status = %s", st)
	}

	req := e.id(`SELECT id FROM bookings WHERE shop_id = ? AND status = 'awaiting_shop' LIMIT 1`, shop)
	w = e.post("/p/bookings/"+req+"/accept", url.Values{}, c, "HX-Request", "true")
	ok(t, "accept", w, "Confirmed")
	if !strings.Contains(w.Header().Get("HX-Trigger"), "toast") {
		t.Errorf("accept should toast")
	}

	// block + conflict
	vid := e.id(`SELECT id FROM vehicles WHERE reg_number = 'RJ 38 SC 0921'`)
	w = e.post("/p/blocks", url.Values{"vehicle": {vid}, "start": {"2026-10-28"}, "end": {"2026-10-29"}}, c)
	if w.Code != 200 {
		t.Fatalf("block = %d %s", w.Code, w.Body)
	}
	thar := e.id(`SELECT id FROM vehicles WHERE reg_number = 'RJ 38 CB 0099'`)
	w = e.post("/p/blocks", url.Values{"vehicle": {thar}, "start": {"2026-10-23"}, "end": {"2026-10-24"}}, c)
	if w.Code != http.StatusConflict {
		t.Errorf("block over a booking = %d, want 409", w.Code)
	}
}

func TestAdminPages(t *testing.T) {
	e := newEnv(t)
	c := e.login(store.RoleAdmin)
	ok(t, "/admin", e.get("/admin", c), "Overview", "Anjuna Bike Point", "Verify")
	ok(t, "/admin/bookings", e.get("/admin/bookings", c), "Bookings")
	ok(t, "/admin/bookings?status=closed", e.get("/admin/bookings?status=closed", c))
	shop := e.id(`SELECT id FROM shops WHERE status = 'pending'`)
	w := e.post("/admin/shops/"+shop+"/verify", url.Values{}, c, "HX-Request", "true")
	ok(t, "verify", w, "Verified")
	// its vehicles now show in search
	ok(t, "search anjuna", e.get("/search?city=goa&area="+e.id(`SELECT id FROM areas WHERE name = 'Anjuna'`), nil), "Activa 5G")
}

func TestRoleGuards(t *testing.T) {
	e := newEnv(t)
	if w := e.get("/trips", nil); w.Code != http.StatusSeeOther || !strings.HasPrefix(w.Header().Get("Location"), "/login?next=") {
		t.Errorf("anon /trips = %d %s", w.Code, w.Header().Get("Location"))
	}
	renter := e.login(store.RoleRenter)
	for _, p := range []string{"/p", "/p/fleet", "/admin"} {
		if w := e.get(p, renter); w.Code != http.StatusForbidden {
			t.Errorf("renter on %s = %d, want 403", p, w.Code)
		}
	}
	provider := e.login(store.RoleProvider)
	if w := e.get("/trips", provider); w.Code != http.StatusForbidden {
		t.Errorf("provider on /trips = %d", w.Code)
	}
	// a provider cannot touch another shop's booking
	other := e.id(`SELECT b.id FROM bookings b JOIN shops s ON s.id = b.shop_id WHERE s.slug = 'goa-wheels' LIMIT 1`)
	if w := e.get("/p/bookings/"+other, provider); w.Code != http.StatusNotFound {
		t.Errorf("foreign booking = %d, want 404", w.Code)
	}
	// renters only see their own trips
	var kabirCode string
	e.st.DB.QueryRow(`SELECT b.code FROM bookings b JOIN users u ON u.id = b.renter_id WHERE u.email = 'renter2@saarthi.test' LIMIT 1`).Scan(&kabirCode)
	if w := e.get("/trips/"+kabirCode, renter); w.Code != http.StatusNotFound {
		t.Errorf("someone else's trip = %d, want 404", w.Code)
	}
	// proof photos are private
	var photo string
	e.st.DB.QueryRow(`SELECT tp.id FROM trip_photos tp JOIN bookings b ON b.id = tp.booking_id JOIN users u ON u.id = b.renter_id WHERE u.email = 'renter2@saarthi.test' LIMIT 1`).Scan(&photo)
	if w := e.get("/photos/"+photo, renter); w.Code != http.StatusNotFound {
		t.Errorf("someone else's photo = %d", w.Code)
	}
	if w := e.get("/photos/"+photo, provider); w.Code != http.StatusOK {
		t.Errorf("shop's own trip photo = %d", w.Code)
	}
}

func TestThemeCookieRoundTrip(t *testing.T) {
	e := newEnv(t)
	r := httptest.NewRequest("POST", "/theme", strings.NewReader("theme=dark"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "http://example.com")
	w := e.do(r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("POST /theme = %d", w.Code)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != handlers.ThemeCookie || cookies[0].Value != "dark" {
		t.Fatalf("cookies = %+v", cookies)
	}
	if body := e.get("/", cookies[0]).Body.String(); !strings.Contains(body, `data-theme="dark"`) {
		t.Errorf("dark theme not applied server-side")
	}
	if body := e.get("/", &http.Cookie{Name: handlers.ThemeCookie, Value: "system"}).Body.String(); strings.Contains(body, `data-theme="`) {
		t.Errorf("system theme should not set data-theme")
	}
}

func TestCrossOriginPostBlocked(t *testing.T) {
	e := newEnv(t)
	r := httptest.NewRequest("POST", "/theme", strings.NewReader("theme=dark"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "https://evil.example")
	if w := e.do(r); w.Code != http.StatusForbidden {
		t.Fatalf("cross-origin POST = %d, want 403", w.Code)
	}
}

func TestStaticFiles(t *testing.T) {
	e := newEnv(t)
	w := e.get("/static/css/app.css", nil)
	if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/css") || w.Header().Get("ETag") == "" {
		t.Fatalf("app.css = %d %q %q", w.Code, w.Header().Get("Content-Type"), w.Header().Get("ETag"))
	}
	if w := e.get("/static/js/app.js", nil); !strings.HasPrefix(w.Header().Get("Content-Type"), "text/javascript") {
		t.Errorf("js content-type = %q", w.Header().Get("Content-Type"))
	}
	if w := e.get("/static/css/", nil); w.Code != 404 {
		t.Errorf("directory listing = %d, want 404", w.Code)
	}
}
