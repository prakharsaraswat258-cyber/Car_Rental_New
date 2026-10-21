package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"saarthi/internal/store"
	"saarthi/web"
)

// Main is the whole command line, shared by cmd/saarthi (local) and
// cmd/server (the entrypoint Vercel's Go preset looks for). It returns the
// process exit code.
//
// Hosting: $PORT sets the port and $SAARTHI_DATA the data folder. On Vercel
// ($VERCEL set) the only writable folder is /tmp, so each new instance
// starts there with fresh demo data (see vercel.json).
func Main(args []string) int {
	fl := flag.NewFlagSet("saarthi", flag.ContinueOnError)
	addr := fl.String("addr", ":8080", "address to listen on")
	reset := fl.Bool("reset", false, "wipe the database and uploads, then reseed demo data")
	dataDir := fl.String("data", "data", "folder for saarthi.db and uploads")
	dev := fl.Bool("dev", false, "serve templates and static files from ./web on disk, no caching")
	if err := fl.Parse(args); err != nil {
		return 2
	}
	set := map[string]bool{}
	fl.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if port := os.Getenv("PORT"); port != "" && !set["addr"] {
		*addr = ":" + port
	}
	if dir := os.Getenv("SAARTHI_DATA"); dir != "" && !set["data"] {
		*dataDir = dir
	} else if os.Getenv("VERCEL") != "" && !set["data"] {
		*dataDir = filepath.Join(os.TempDir(), "saarthi")
		*reset = true // a new instance never inherits a half-written demo db
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(log, *addr, *dataDir, *reset, *dev); err != nil {
		log.Error("saarthi stopped", "err", err)
		return 1
	}
	return 0
}

func run(log *slog.Logger, addr, dataDir string, reset, dev bool) error {
	if reset {
		if err := store.Wipe(dataDir); err != nil {
			return err
		}
		log.Info("demo data wiped", "folder", dataDir)
	}

	st, err := store.Open(dataDir)
	if err != nil {
		return err
	}
	defer st.Close()

	seeded, err := st.SeedIfEmpty(context.Background())
	if err != nil {
		return err
	}
	if seeded {
		log.Info("demo data seeded", "password", store.DemoPassword,
			"renter", store.DemoRenterEmail, "provider", store.DemoProviderEmail, "admin", store.DemoAdminEmail)
	}

	var webFS fs.FS = web.FS
	if dev {
		if _, err := os.Stat("web/templates"); err != nil {
			return fmt.Errorf("-dev needs to run from the project folder (web/templates not found)")
		}
		webFS = os.DirFS("web")
	}

	handler, err := New(Config{Store: st, Web: webFS, Dev: dev, Log: log})
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second, // photo uploads on slow phones
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("Saarthi is running", "url", displayURL(addr), "dev", dev, "shared_sessions", store.SharedSessions())

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		log.Info("shutting down")
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	}
}

func displayURL(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "http://localhost" + addr
	}
	return "http://" + addr
}
