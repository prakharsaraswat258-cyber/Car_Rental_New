// Package app wires Saarthi together: routes, middleware and static files.
package app

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"strings"
	"sync"

	"saarthi/internal/handlers"
	"saarthi/internal/store"
)

// Config is everything New needs.
type Config struct {
	Store *store.Store
	Web   fs.FS // contains templates/ and static/
	Dev   bool
	Log   *slog.Logger
}

func init() {
	// Windows can map .js to text/plain via the registry; pin the types
	// browsers are strict about.
	for ext, typ := range map[string]string{
		".js": "text/javascript; charset=utf-8", ".mjs": "text/javascript; charset=utf-8",
		".css": "text/css; charset=utf-8", ".svg": "image/svg+xml", ".woff2": "font/woff2",
		".json": "application/json", ".ics": "text/calendar; charset=utf-8", ".webmanifest": "application/manifest+json",
	} {
		mime.AddExtensionType(ext, typ)
	}
}

// New builds the root handler.
func New(cfg Config) (http.Handler, error) {
	views, err := handlers.NewViews(cfg.Web, cfg.Dev, cfg.Log)
	if err != nil {
		return nil, err
	}
	h := &handlers.Handlers{Store: cfg.Store, Views: views, Log: cfg.Log, Dev: cfg.Dev}

	static, err := fs.Sub(cfg.Web, "static")
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", staticFiles(static, cfg.Dev)))
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/static/img/favicon.svg", http.StatusMovedPermanently)
	})

	renter := func(f http.HandlerFunc) http.HandlerFunc { return requireRole(h, f, store.RoleRenter) }
	provider := func(f http.HandlerFunc) http.HandlerFunc { return requireRole(h, f, store.RoleProvider) }
	admin := func(f http.HandlerFunc) http.HandlerFunc { return requireRole(h, f, store.RoleAdmin) }
	anyone := func(f http.HandlerFunc) http.HandlerFunc {
		return requireRole(h, f, store.RoleRenter, store.RoleProvider, store.RoleAdmin)
	}

	// Public
	mux.HandleFunc("GET /{$}", h.Home)
	mux.HandleFunc("POST /ask", h.Ask)
	mux.HandleFunc("GET /search", h.Search)
	mux.HandleFunc("GET /v/{id}", h.Vehicle)
	mux.HandleFunc("GET /v/{id}/quote", h.Quote)
	mux.HandleFunc("GET /v/{id}/calendar", h.Calendar)
	mux.HandleFunc("GET /compare", h.Compare)
	mux.HandleFunc("POST /compare/{id}", h.CompareAdd)
	mux.HandleFunc("DELETE /compare/{id}", h.CompareRemove)
	mux.HandleFunc("POST /compare/{id}/remove", h.CompareRemove)
	mux.HandleFunc("GET /login", h.LoginPage)
	mux.HandleFunc("POST /login", h.Login)
	mux.HandleFunc("POST /login/demo/{role}", h.DemoLogin)
	mux.HandleFunc("GET /signup", h.SignupPage)
	mux.HandleFunc("POST /signup", h.Signup)
	mux.HandleFunc("POST /logout", h.Logout)
	mux.HandleFunc("GET /dev/styleguide", h.Styleguide)
	mux.HandleFunc("POST /theme", h.SetTheme)
	mux.HandleFunc("GET /vphotos/{id}", h.VehiclePhoto)
	mux.HandleFunc("GET /photos/{id}", h.TripPhoto) // owner / shop / admin checked inside
	mux.HandleFunc("GET /account", anyone(h.Account))
	mux.HandleFunc("GET /s/{slug}", h.ShopPage)
	mux.HandleFunc("GET /offline", h.Offline)
	mux.HandleFunc("GET /sw.js", h.ServiceWorker)
	mux.HandleFunc("GET /manifest.webmanifest", h.Manifest)

	// Renter
	mux.HandleFunc("GET /checkout/{vehicleID}", renter(h.Checkout))
	mux.HandleFunc("POST /bookings", renter(h.CreateBooking))
	mux.HandleFunc("POST /bookings/{code}/pay", renter(h.Pay))
	mux.HandleFunc("GET /trips", renter(h.Trips))
	mux.HandleFunc("GET /trips/{code}", renter(h.Trip))
	mux.HandleFunc("GET /trips/{code}/pass", renter(h.Pass))
	mux.HandleFunc("GET /trips/{code}/pass.ics", renter(h.PassICS))
	mux.HandleFunc("GET /trips/{code}/deposit", renter(h.DepositFragment))
	mux.HandleFunc("POST /trips/{code}/cancel", renter(h.CancelTrip))
	mux.HandleFunc("GET /trips/{code}/checkin", renter(h.CheckinPage("checkin")))
	mux.HandleFunc("GET /trips/{code}/return", renter(h.CheckinPage("return")))
	mux.HandleFunc("POST /trips/{code}/photos", renter(h.UploadTripPhoto))
	mux.HandleFunc("POST /trips/{code}/start", renter(h.StartTrip))
	mux.HandleFunc("POST /trips/{code}/end", renter(h.EndTrip))
	mux.HandleFunc("POST /trips/{code}/review", renter(h.Review))

	// Provider
	mux.HandleFunc("GET /p", provider(h.ProviderToday))
	mux.HandleFunc("GET /p/bookings/{id}", provider(h.ProviderBooking))
	mux.HandleFunc("POST /p/bookings/{id}/accept", provider(h.ProviderMove(store.ShopAccept)))
	mux.HandleFunc("POST /p/bookings/{id}/decline", provider(h.ProviderMove(store.ShopDecline)))
	mux.HandleFunc("POST /p/bookings/{id}/ready", provider(h.ProviderMove(store.ShopReady)))
	mux.HandleFunc("GET /p/bookings/{id}/inspect", provider(h.InspectPage))
	mux.HandleFunc("POST /p/bookings/{id}/inspect", provider(h.Inspect))
	mux.HandleFunc("GET /p/fleet", provider(h.ProviderFleet))
	mux.HandleFunc("GET /p/vehicles/new", provider(h.VehicleNew))
	mux.HandleFunc("POST /p/vehicles/new", provider(h.VehicleCreate))
	mux.HandleFunc("GET /p/vehicles/{id}/edit", provider(h.VehicleEdit))
	mux.HandleFunc("POST /p/vehicles/{id}/edit", provider(h.VehicleUpdate))
	mux.HandleFunc("POST /p/vehicles/{id}/photos", provider(h.VehiclePhotoUpload))
	mux.HandleFunc("POST /p/vehicles/{id}/photos/{pid}/delete", provider(h.VehiclePhotoDelete))
	mux.HandleFunc("POST /p/vehicles/{id}/status", provider(h.VehicleStatus))
	mux.HandleFunc("GET /p/calendar", provider(h.ProviderCalendar))
	mux.HandleFunc("POST /p/blocks", provider(h.CreateBlock))
	mux.HandleFunc("DELETE /p/blocks/{id}", provider(h.DeleteBlock))
	mux.HandleFunc("GET /p/palette", provider(h.Palette(false)))

	// Admin
	mux.HandleFunc("GET /admin", admin(h.Admin))
	mux.HandleFunc("POST /admin/shops/{id}/verify", admin(h.AdminShopStatus("verified")))
	mux.HandleFunc("POST /admin/shops/{id}/suspend", admin(h.AdminShopStatus("suspended")))
	mux.HandleFunc("GET /admin/bookings", admin(h.AdminBookings))
	mux.HandleFunc("GET /admin/bookings/{id}", admin(h.AdminBooking))
	mux.HandleFunc("GET /admin/palette", admin(h.Palette(true)))

	// Everything else
	mux.HandleFunc("/", h.NotFound)

	var root http.Handler = mux
	root = session(cfg.Store, root)
	root = theme(root)
	root = sameOrigin(root)
	root = securityHeaders(root)
	root = gzipResponses(root)
	root = logRequests(cfg.Log, root)
	root = recoverPanics(cfg.Log, h.ErrorPage, root)
	return root, nil
}

// staticFiles serves web/static (D-027). Versioned URLs (?v=hash) cache
// forever; everything else (ES module imports, fonts named in CSS) must
// revalidate, which is cheap because every file gets a content-hash ETag.
func staticFiles(fsys fs.FS, dev bool) http.Handler {
	files := http.FileServerFS(fsys)
	hosted := os.Getenv("VERCEL") != ""
	var mu sync.Mutex
	etags := map[string]string{}
	etag := func(name string) string {
		mu.Lock()
		defer mu.Unlock()
		if tag, ok := etags[name]; ok && !dev {
			return tag
		}
		b, err := fs.ReadFile(fsys, name)
		if err != nil {
			return ""
		}
		sum := sha256.Sum256(b)
		tag := `"` + hex.EncodeToString(sum[:12]) + `"`
		etags[name] = tag
		return tag
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "" || strings.HasSuffix(r.URL.Path, "/") { // no directory listings
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Has("v") && !dev {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		if hosted && !dev {
			// Vercel's CDN keeps a copy per deployment, so page loads don't
			// wake extra server instances (each has its own demo db)
			w.Header().Set("CDN-Cache-Control", "public, max-age=31536000")
		}
		if tag := etag(strings.TrimPrefix(r.URL.Path, "/")); tag != "" {
			w.Header().Set("ETag", tag) // FileServer answers If-None-Match with 304
		}
		files.ServeHTTP(w, r)
	})
}
