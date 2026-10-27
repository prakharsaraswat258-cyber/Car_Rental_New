package app

import (
	"compress/gzip"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"saarthi/internal/handlers"
	"saarthi/internal/store"
)

// SessionCookie holds the login token.
const SessionCookie = handlers.SessionCookie

// recoverPanics turns a panic into the designed 500 page and a log line
// with the stack.
func recoverPanics(log *slog.Logger, page http.HandlerFunc, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				log.Error("panic", "path", r.URL.Path, "err", v, "stack", string(debug.Stack()))
				func() {
					defer func() {
						if recover() != nil {
							http.Error(w, "Something broke on our side. Your bookings are safe.", http.StatusInternalServerError)
						}
					}()
					page(w, r)
				}()
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// --- gzip --------------------------------------------------------------------

var gzipPool = sync.Pool{New: func() any { w, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression); return w }}

var compressible = []string{"text/", "application/json", "application/javascript", "application/manifest+json", "image/svg+xml"}

type gzipWriter struct {
	http.ResponseWriter
	gz      *gzip.Writer
	decided bool
	on      bool
}

func (g *gzipWriter) decide() {
	if g.decided {
		return
	}
	g.decided = true
	h := g.Header()
	if h.Get("Content-Encoding") != "" {
		return
	}
	ct := h.Get("Content-Type")
	for _, p := range compressible {
		if strings.HasPrefix(ct, p) {
			g.on = true
			break
		}
	}
	if !g.on {
		return
	}
	h.Del("Content-Length")
	h.Set("Content-Encoding", "gzip")
	h.Add("Vary", "Accept-Encoding")
	if tag := h.Get("ETag"); tag != "" && !strings.HasPrefix(tag, "W/") {
		h.Set("ETag", "W/"+tag) // the bytes differ from the uncompressed file
	}
	g.gz = gzipPool.Get().(*gzip.Writer)
	g.gz.Reset(g.ResponseWriter)
}

func (g *gzipWriter) WriteHeader(code int) {
	if code < 200 || code == http.StatusNoContent || code == http.StatusNotModified {
		g.decided = true
	} else {
		g.decide()
	}
	g.ResponseWriter.WriteHeader(code)
}

func (g *gzipWriter) Write(b []byte) (int, error) {
	if !g.decided {
		if g.Header().Get("Content-Type") == "" {
			g.Header().Set("Content-Type", http.DetectContentType(b))
		}
		g.decide()
	}
	if g.on {
		return g.gz.Write(b)
	}
	return g.ResponseWriter.Write(b)
}

func (g *gzipWriter) Unwrap() http.ResponseWriter { return g.ResponseWriter }

func (g *gzipWriter) close() {
	if g.on && g.gz != nil {
		g.gz.Close()
		gzipPool.Put(g.gz)
	}
}

// gzipResponses compresses text responses (HTML, CSS, JS, JSON, SVG) when
// the browser accepts gzip. Range requests are left alone.
func gzipResponses(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") || r.Header.Get("Range") != "" || r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		g := &gzipWriter{ResponseWriter: w}
		defer g.close()
		next.ServeHTTP(g, r)
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

// securityHeaders sets a strict-but-workable Content-Security-Policy and related headers.
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
// second line of defence.
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

// theme puts the visitor's theme choice on the context.
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
