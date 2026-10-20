package app

import (
	"context"
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
	h, err := New(Config{Store: st, Web: web.FS, Log: log})
	if err != nil {
		t.Fatal(err)
	}
	views, err := handlers.NewViews(web.FS, false, log)
	if err != nil {
		t.Fatal(err)
	}
	return &env{st: st, h: h, handler: &handlers.Handlers{Store: st, Views: views, Log: log}}
}

func (e *env) do(r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	return w
}

func TestHomeRenders(t *testing.T) {
	e := newEnv(t)
	w := e.do(httptest.NewRequest("GET", "/", nil))
	if w.Code != 200 {
		t.Fatalf("GET / = %d: %s", w.Code, w.Body)
	}
	body := w.Body.String()
	for _, want := range []string{"<!doctype html>", "Where are you headed?", "vehicles you can book", store.DemoRenterEmail} {
		if !strings.Contains(body, want) {
			t.Errorf("home page is missing %q", want)
		}
	}
	if csp := w.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") {
		t.Errorf("CSP = %q", csp)
	}
}

func TestHTMXGetsFragment(t *testing.T) {
	e := newEnv(t)
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("HX-Request", "true")
	w := e.do(r)
	if w.Code != 200 || strings.Contains(w.Body.String(), "<!doctype html>") {
		t.Fatalf("htmx request got a full page (code %d)", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Where are you headed?") {
		t.Errorf("fragment is missing content")
	}
}

func TestNotFound(t *testing.T) {
	e := newEnv(t)
	w := e.do(httptest.NewRequest("GET", "/no/such/road", nil))
	if w.Code != 404 || !strings.Contains(w.Body.String(), "doesn't go anywhere") {
		t.Fatalf("GET unknown = %d", w.Code)
	}
}

func TestThemeCookieRoundTrip(t *testing.T) {
	e := newEnv(t)
	r := httptest.NewRequest("POST", "/theme", strings.NewReader("theme=dark"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "http://example.com")
	r.Host = "example.com"
	w := e.do(r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("POST /theme = %d", w.Code)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != handlers.ThemeCookie || cookies[0].Value != "dark" || !cookies[0].HttpOnly {
		t.Fatalf("cookies = %+v", cookies)
	}

	r = httptest.NewRequest("GET", "/", nil)
	r.AddCookie(cookies[0])
	if body := e.do(r).Body.String(); !strings.Contains(body, `data-theme="dark"`) {
		t.Errorf("dark theme not applied server-side")
	}

	// system → no data-theme attribute, CSS follows the OS
	r = httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: handlers.ThemeCookie, Value: "system"})
	if body := e.do(r).Body.String(); strings.Contains(body, `data-theme="`) {
		t.Errorf("system theme should not set data-theme")
	}

	// junk is rejected
	r = httptest.NewRequest("POST", "/theme", strings.NewReader("theme=neon"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if w := e.do(r); w.Code != http.StatusBadRequest {
		t.Errorf("bad theme = %d, want 400", w.Code)
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
	r = httptest.NewRequest("POST", "/theme", strings.NewReader("theme=dark"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	if w := e.do(r); w.Code != http.StatusForbidden {
		t.Fatalf("cross-site POST = %d, want 403", w.Code)
	}
}

func TestStaticFiles(t *testing.T) {
	e := newEnv(t)
	w := e.do(httptest.NewRequest("GET", "/static/css/app.css", nil))
	if w.Code != 200 {
		t.Fatalf("GET app.css = %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Errorf("content-type = %q", ct)
	}
	tag := w.Header().Get("ETag")
	if tag == "" {
		t.Fatal("no ETag")
	}
	r := httptest.NewRequest("GET", "/static/css/app.css", nil)
	r.Header.Set("If-None-Match", tag)
	if w := e.do(r); w.Code != http.StatusNotModified {
		t.Errorf("revalidation = %d, want 304", w.Code)
	}
	if w := e.do(httptest.NewRequest("GET", "/static/js/app.js", nil)); !strings.HasPrefix(w.Header().Get("Content-Type"), "text/javascript") {
		t.Errorf("js content-type = %q", w.Header().Get("Content-Type"))
	}
	if w := e.do(httptest.NewRequest("GET", "/static/css/", nil)); w.Code != 404 {
		t.Errorf("directory listing = %d, want 404", w.Code)
	}
}

func TestRequireRole(t *testing.T) {
	e := newEnv(t)
	ok := func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "shop console") }
	guarded := session(e.st, requireRole(e.handler, ok, store.RoleProvider))

	login := func(email string) *http.Cookie {
		u, _, err := e.st.UserByEmail(context.Background(), email)
		if err != nil {
			t.Fatal(err)
		}
		tok, _, err := e.st.CreateSession(context.Background(), u.ID)
		if err != nil {
			t.Fatal(err)
		}
		return &http.Cookie{Name: SessionCookie, Value: tok}
	}

	// logged out → login with next=
	w := httptest.NewRecorder()
	guarded.ServeHTTP(w, httptest.NewRequest("GET", "/p/fleet?x=1", nil))
	if w.Code != http.StatusSeeOther {
		t.Fatalf("logged out = %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login?next="+url.QueryEscape("/p/fleet?x=1") {
		t.Errorf("redirect = %q", loc)
	}

	// renter → 403
	r := httptest.NewRequest("GET", "/p/fleet", nil)
	r.AddCookie(login(store.DemoRenterEmail))
	w = httptest.NewRecorder()
	guarded.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("renter on provider route = %d, want 403", w.Code)
	}

	// provider → through
	r = httptest.NewRequest("GET", "/p/fleet", nil)
	r.AddCookie(login(store.DemoProviderEmail))
	w = httptest.NewRecorder()
	guarded.ServeHTTP(w, r)
	if w.Code != 200 || w.Body.String() != "shop console" {
		t.Errorf("provider = %d %q", w.Code, w.Body)
	}

	// stale cookie is cleared and treated as logged out
	r = httptest.NewRequest("GET", "/p/fleet", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookie, Value: "deadbeef"})
	w = httptest.NewRecorder()
	guarded.ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther || !strings.Contains(w.Header().Get("Set-Cookie"), SessionCookie+"=;") {
		t.Errorf("stale cookie: code %d set-cookie %q", w.Code, w.Header().Get("Set-Cookie"))
	}
}
