package app

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"

	"saarthi/internal/handlers"
	"saarthi/internal/store"
)

// SessionCookie holds the login token (D-007).
const SessionCookie = "saarthi_session"

// recoverPanics turns a panic into a 500 and a log line with the stack.
func recoverPanics(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				log.Error("panic", "path", r.URL.Path, "err", v, "stack", string(debug.Stack()))
				http.Error(w, "Something went wrong on our side. Please try again.", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// logRequests logs one line per non-static request.
func logRequests(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		if strings.HasPrefix(r.URL.Path, "/static/") && sw.status < 400 {
			return
		}
		attrs := []any{"method", r.Method, "path", r.URL.Path, "status", sw.status, "took", time.Since(start).Round(time.Microsecond)}
		if r.Header.Get("HX-Request") == "true" {
			attrs = append(attrs, "htmx", true)
		}
		log.Info("http", attrs...)
	})
}

// securityHeaders sets a strict-but-workable CSP and friends (D-025).
func securityHeaders(next http.Handler) http.Handler {
	csp := strings.Join([]string{
		"default-src 'self'",
		"script-src 'self'",
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data: blob: https://*.basemaps.cartocdn.com",
		"font-src 'self'",
		"connect-src 'self'",
		"frame-ancestors 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"object-src 'none'",
	}, "; ")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Permissions-Policy", "camera=(self), geolocation=(), microphone=()")
		next.ServeHTTP(w, r)
	})
}

// sameOrigin rejects state-changing requests from other sites. SameSite=Lax
// already keeps the session cookie off cross-site POSTs; this is the
// belt to that brace (D-007).
func sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Host != r.Host {
				http.Error(w, "Cross-site request blocked.", http.StatusForbidden)
				return
			}
		} else if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			http.Error(w, "Cross-site request blocked.", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// theme puts the visitor's theme choice on the context (D-008).
func theme(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(handlers.ThemeCookie); err == nil {
			r = r.WithContext(handlers.WithTheme(r.Context(), c.Value))
		}
		next.ServeHTTP(w, r)
	})
}

// session loads the logged-in user, if any, onto the context.
func session(st *store.Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/static/") {
			next.ServeHTTP(w, r)
			return
		}
		c, err := r.Cookie(SessionCookie)
		if err == nil && c.Value != "" {
			u, err := st.UserBySession(r.Context(), c.Value)
			switch {
			case err == nil:
				r = r.WithContext(handlers.WithUser(r.Context(), &u))
			case errors.Is(err, store.ErrNotFound):
				// stale cookie: clear it
				http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
			default:
				http.Error(w, "Something went wrong on our side. Please try again.", http.StatusInternalServerError)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// requireRole lets only logged-in users with one of roles through. Logged
// out visitors go to /login?next=…; wrong roles get 403.
func requireRole(h *handlers.Handlers, next http.HandlerFunc, roles ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := handlers.UserFrom(r.Context())
		if u == nil {
			to := "/login?next=" + url.QueryEscape(r.URL.RequestURI())
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", to)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, to, http.StatusSeeOther)
			return
		}
		for _, role := range roles {
			if u.Role == role {
				next(w, r)
				return
			}
		}
		h.Forbidden(w, r)
	}
}
