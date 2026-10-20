// Command saarthi runs the Saarthi vehicle rental web app.
//
//	go run ./cmd/saarthi          # http://localhost:8080
//	go run ./cmd/saarthi -reset   # wipe data/ and reseed demo data
//	go run ./cmd/saarthi -dev     # read ./web from disk (edit → refresh)
package main

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
	"strings"
	"time"

	"saarthi/internal/app"
	"saarthi/internal/store"
	"saarthi/web"
)

func main() {
	addr := flag.String("addr", ":8080", "address to listen on")
	reset := flag.Bool("reset", false, "wipe the database and uploads, then reseed demo data")
	dataDir := flag.String("data", "data", "folder for saarthi.db and uploads")
	dev := flag.Bool("dev", false, "serve templates and static files from ./web on disk, no caching")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(log, *addr, *dataDir, *reset, *dev); err != nil {
		log.Error("saarthi stopped", "err", err)
		os.Exit(1)
	}
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

	handler, err := app.New(app.Config{Store: st, Web: webFS, Dev: dev, Log: log})
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("Saarthi is running", "url", displayURL(addr), "dev", dev)

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
