// Package store is Saarthi's SQLite layer: schema, demo seed and queries,
// written with plain database/sql on the pure-Go modernc.org/sqlite driver.
package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	mrand "math/rand/v2"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" driver (no CGO)
)

//go:embed schema.sql
var schemaSQL string

// TimeLayout is the one format every timestamp is stored in (UTC). Because
// it is fixed-width, comparing the text in SQL compares the times.
const TimeLayout = "2006-01-02T15:04:05Z"

// ErrNotFound is returned when a row does not exist (or is not visible).
var ErrNotFound = errors.New("not found")

// Store wraps the database and the uploads folder.
type Store struct {
	DB        *sql.DB
	Dir       string
	UploadDir string
	Now       func() time.Time // swapped in tests
}

const dbFile = "saarthi.db"

// Open opens (creating if needed) data/saarthi.db and applies the schema.
func Open(dir string) (*Store, error) {
	uploads := filepath.Join(dir, "uploads")
	if err := os.MkdirAll(uploads, 0o755); err != nil {
		return nil, fmt.Errorf("create data folder: %w", err)
	}
	db, err := sql.Open("sqlite", dsn(filepath.Join(dir, dbFile)))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(8)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("open database: %w", err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}
	if err := checkVersion(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{DB: db, Dir: dir, UploadDir: uploads, Now: time.Now}, nil
}

// SchemaVersion goes up whenever schema.sql changes in a way CREATE IF NOT
// EXISTS can't apply. Demo data is reseedable, so old databases are reset
// rather than migrated.
const SchemaVersion = 2

func checkVersion(db *sql.DB) error {
	var v, users int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		return err
	}
	if v == SchemaVersion {
		return nil
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users); err != nil {
		return err
	}
	if users > 0 && v < SchemaVersion {
		return fmt.Errorf("the demo database is from an older Saarthi build; start once with: go run ./cmd/saarthi -reset")
	}
	_, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, SchemaVersion))
	return err
}

// dsn turns on WAL, foreign keys and a busy timeout, and makes every
// transaction BEGIN IMMEDIATE so check-then-insert is atomic.
func dsn(path string) string {
	return "file:" + filepath.ToSlash(path) +
		"?_pragma=foreign_keys(1)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=synchronous(NORMAL)" +
		"&_txlock=immediate"
}

// Close closes the database.
func (s *Store) Close() error { return s.DB.Close() }

// Wipe deletes the database files and uploads inside dir. Only Saarthi's
// own files are touched.
func Wipe(dir string) error {
	for _, name := range []string{dbFile, dbFile + "-wal", dbFile + "-shm"} {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("wipe %s: %w", name, err)
		}
	}
	if err := os.RemoveAll(filepath.Join(dir, "uploads")); err != nil {
		return fmt.Errorf("wipe uploads: %w", err)
	}
	return nil
}

// --- small helpers ---------------------------------------------------------

// Random bytes for ids, tokens and booking codes. While demo data is being
// seeded they come from a fixed stream instead, so every server instance
// (Vercel runs several, each with its own /tmp database) builds identical
// demo rows: the same vehicle ids, booking codes and photo ids.
var (
	randMu  sync.Mutex
	randSrc io.Reader = rand.Reader
	seeding int
)

func randomBytes(b []byte) {
	randMu.Lock()
	defer randMu.Unlock()
	if _, err := io.ReadFull(randSrc, b); err != nil {
		panic(err) // the OS random source failing is not recoverable
	}
}

// fixedRandom switches to the seed stream until the returned func runs.
func fixedRandom() (restore func()) {
	randMu.Lock()
	defer randMu.Unlock()
	if seeding == 0 {
		randSrc = mrand.NewChaCha8(sha256.Sum256([]byte("saarthi demo seed v1")))
	}
	seeding++
	return func() {
		randMu.Lock()
		defer randMu.Unlock()
		if seeding--; seeding == 0 {
			randSrc = rand.Reader
		}
	}
}

// NewID returns 16 random bytes as hex.
func NewID() string {
	b := make([]byte, 16)
	randomBytes(b)
	return hex.EncodeToString(b)
}

// NewToken returns 32 random bytes as hex, for session cookies.
func NewToken() string {
	b := make([]byte, 32)
	randomBytes(b)
	return hex.EncodeToString(b)
}

const codeAlphabet = "23456789ABCDEFGHJKMNPQRSTVWXYZ"

// NewBookingCode returns "SAR-" + 5 characters with no look-alikes.
func NewBookingCode() string {
	out := []byte("SAR-")
	buf := make([]byte, 1)
	limit := byte(256 - 256%len(codeAlphabet)) // reject to avoid modulo bias
	for len(out) < 9 {
		randomBytes(buf)
		if buf[0] >= limit {
			continue
		}
		out = append(out, codeAlphabet[int(buf[0])%len(codeAlphabet)])
	}
	return string(out)
}

// ts formats a time for storage.
func ts(t time.Time) string { return t.UTC().Format(TimeLayout) }

// parseTS parses a stored time; empty text is the zero time.
func parseTS(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(TimeLayout, s)
}

// nullTS parses a nullable stored time.
func nullTS(ns sql.NullString) (time.Time, error) {
	if !ns.Valid {
		return time.Time{}, nil
	}
	return parseTS(ns.String)
}

// inTx runs fn inside one transaction (BEGIN IMMEDIATE via the DSN).
func (s *Store) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}
