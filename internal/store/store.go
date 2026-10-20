// Package store is Saarthi's SQLite layer: schema, demo seed and queries,
// written with plain database/sql on the pure-Go modernc.org/sqlite driver.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" driver (no CGO)
)

//go:embed schema.sql
var schemaSQL string

// TimeLayout is the one format every timestamp is stored in (UTC). Because
// it is fixed-width, comparing the text in SQL compares the times (D-006).
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
	return &Store{DB: db, Dir: dir, UploadDir: uploads, Now: time.Now}, nil
}

// dsn turns on WAL, foreign keys and a busy timeout, and makes every
// transaction BEGIN IMMEDIATE so check-then-insert is atomic (D-002).
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

// NewID returns 16 random bytes as hex (D-004).
func NewID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err) // the OS random source failing is not recoverable
	}
	return hex.EncodeToString(b)
}

// NewToken returns 32 random bytes as hex, for session cookies.
func NewToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

const codeAlphabet = "23456789ABCDEFGHJKMNPQRSTVWXYZ"

// NewBookingCode returns "SAR-" + 5 characters with no look-alikes.
func NewBookingCode() string {
	out := []byte("SAR-")
	buf := make([]byte, 1)
	limit := byte(256 - 256%len(codeAlphabet)) // reject to avoid modulo bias
	for len(out) < 9 {
		if _, err := rand.Read(buf); err != nil {
			panic(err)
		}
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
