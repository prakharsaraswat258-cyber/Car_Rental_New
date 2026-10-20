// Package app wires Saarthi together: routes, middleware and static files.
package app

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
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

	// Public
	mux.HandleFunc("GET /{$}", h.Home)
	mux.HandleFunc("POST /theme", h.SetTheme)

	// Everything else
	mux.HandleFunc("/", h.NotFound)

	var root http.Handler = mux
	root = session(cfg.Store, root)
	root = theme(root)
	root = sameOrigin(root)
	root = securityHeaders(root)
	root = logRequests(cfg.Log, root)
	root = recoverPanics(cfg.Log, root)
	return root, nil
}

// staticFiles serves web/static (D-027). Versioned URLs (?v=hash) cache
// forever; everything else (ES module imports, fonts named in CSS) must
// revalidate, which is cheap because every file gets a content-hash ETag.
func staticFiles(fsys fs.FS, dev bool) http.Handler {
	files := http.FileServerFS(fsys)
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
		if tag := etag(strings.TrimPrefix(r.URL.Path, "/")); tag != "" {
			w.Header().Set("ETag", tag) // FileServer answers If-None-Match with 304
		}
		files.ServeHTTP(w, r)
	})
}
