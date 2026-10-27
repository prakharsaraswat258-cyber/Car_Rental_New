// Command saarthi runs the Saarthi vehicle rental web app.
//
//	go run ./cmd/saarthi          # http://localhost:8080
//	go run ./cmd/saarthi -reset   # wipe data/ and reseed demo data
//	go run ./cmd/saarthi -dev     # read ./web from disk (edit → refresh)
//
// Flags and hosting env vars are handled by app.Main.
package main

import (
	"os"

	"saarthi/internal/app"
)

func main() { os.Exit(app.Main(os.Args[1:])) }
