package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"saarthi/internal/domain"
)

// seedNow is a fixed clock: Tue 20 Oct 2026 15:00 IST.
var seedNow = time.Date(2026, 10, 20, 15, 0, 0, 0, domain.IST)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	s.Now = func() time.Time { return seedNow }
	return s
}

func seeded(t *testing.T) *Store {
	t.Helper()
	s := newTestStore(t)
	ok, err := s.SeedIfEmpty(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("fresh database was not seeded")
	}
	return s
}

func TestSeedFillsEveryScreen(t *testing.T) {
	s := seeded(t)
	ctx := context.Background()

	again, err := s.SeedIfEmpty(ctx)
	if err != nil || again {
		t.Fatalf("second seed: again=%v err=%v, want false nil", again, err)
	}

	c, err := s.Counts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if c.Cities != 2 || c.Areas != 7 || c.Shops != 3 {
		t.Errorf("places/shops = %+v", c)
	}
	if c.Vehicles < 24 {
		t.Errorf("vehicles = %d, want ~24", c.Vehicles)
	}
	// hidden, draft and pending-shop vehicles are not searchable
	if c.LiveVehicles != c.Vehicles-4 {
		t.Errorf("live searchable = %d, want %d", c.LiveVehicles, c.Vehicles-4)
	}

	// Demo renter has a booking in every state the spec asks for.
	rows, err := s.DB.QueryContext(ctx, `SELECT b.status, COUNT(*) FROM bookings b
		JOIN users u ON u.id = b.renter_id WHERE u.email = ? GROUP BY b.status`, DemoRenterEmail)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			t.Fatal(err)
		}
		got[st] = n
	}
	rows.Close()
	for _, st := range []domain.Status{domain.StatusClosed, domain.StatusActive, domain.StatusReturned,
		domain.StatusAwaitingShop, domain.StatusConfirmed, domain.StatusReady} {
		if got[string(st)] == 0 {
			t.Errorf("demo renter has no %s booking (got %v)", st, got)
		}
	}

	// Every stored bill adds up.
	var bad int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM bookings
		WHERE total_paise != rent_paise + addons_paise + tax_paise`).Scan(&bad); err != nil {
		t.Fatal(err)
	}
	if bad != 0 {
		t.Errorf("%d bookings with total != rent + addons + tax", bad)
	}

	// One review, one deduction, deposits for every paid booking.
	var reviews, deductions, credited, inspection int
	s.DB.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM reviews), (SELECT COUNT(*) FROM deductions),
		(SELECT COUNT(*) FROM deposits WHERE status = 'credited'),
		(SELECT COUNT(*) FROM deposits WHERE status = 'inspection')`).Scan(&reviews, &deductions, &credited, &inspection)
	// the month of history adds more reviews, deductions and credited deposits
	if reviews < 10 || deductions < 1 || credited < 20 || inspection != 1 {
		t.Errorf("reviews=%d deductions=%d credited=%d inspection=%d", reviews, deductions, credited, inspection)
	}

	// Seeded proof photos exist on disk.
	var path string
	if err := s.DB.QueryRowContext(ctx, `SELECT path FROM trip_photos LIMIT 1`).Scan(&path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.UploadDir, filepath.FromSlash(path))); err != nil {
		t.Errorf("seed photo missing: %v", err)
	}

	// No seeded event sits in the future.
	var future int
	s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM booking_events WHERE created_at > ?`, ts(seedNow)).Scan(&future)
	if future != 0 {
		t.Errorf("%d booking events are in the future", future)
	}
}

func TestSessions(t *testing.T) {
	s := seeded(t)
	ctx := context.Background()

	u, hash, err := s.UserByEmail(ctx, "RENTER@saarthi.test") // email match ignores case
	if err != nil {
		t.Fatal(err)
	}
	if hash == "" || u.Role != RoleRenter {
		t.Fatalf("user = %+v", u)
	}

	tok, _, err := s.CreateSession(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.UserBySession(ctx, tok)
	if err != nil || got.ID != u.ID {
		t.Fatalf("UserBySession = %+v, %v", got, err)
	}

	// expired sessions are invisible
	s.Now = func() time.Time { return seedNow.Add(SessionTTL + time.Hour) }
	if _, err := s.UserBySession(ctx, tok); err != ErrNotFound {
		t.Errorf("expired session: err = %v, want ErrNotFound", err)
	}

	s.Now = func() time.Time { return seedNow }
	if err := s.DeleteSession(ctx, tok); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserBySession(ctx, tok); err != ErrNotFound {
		t.Errorf("deleted session: err = %v, want ErrNotFound", err)
	}
}

// Two instances seeded apart build the same demo rows (D-050).
func TestSeedIsIdenticalAcrossInstances(t *testing.T) {
	a, b := seeded(t), newTestStore(t)
	b.Now = func() time.Time { return seedNow.Add(7 * time.Minute) }
	if _, err := b.SeedIfEmpty(context.Background()); err != nil {
		t.Fatal(err)
	}
	rows := func(s *Store, q string) string {
		t.Helper()
		r, err := s.DB.Query(q)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		var out []string
		for r.Next() {
			var v string
			if err := r.Scan(&v); err != nil {
				t.Fatal(err)
			}
			out = append(out, v)
		}
		return strings.Join(out, ",")
	}
	for _, q := range []string{
		`SELECT id FROM users ORDER BY id`,
		`SELECT id FROM vehicles ORDER BY id`,
		`SELECT code FROM bookings ORDER BY code`,
		`SELECT id FROM trip_photos ORDER BY id`,
	} {
		if x, y := rows(a, q), rows(b, q); x == "" || x != y {
			t.Errorf("%s differs between instances", q)
		}
	}
	// after seeding, ids are random again
	if NewID() == NewID() {
		t.Error("ids repeat after seeding")
	}
}

// A login made on one instance works on another; logging out there sticks;
// a tampered token never does (D-050).
func TestSessionsAcrossInstances(t *testing.T) {
	a, b := seeded(t), seeded(t)
	ctx := context.Background()
	u, _, err := a.UserByEmail(ctx, DemoRenterEmail)
	if err != nil {
		t.Fatal(err)
	}
	tok, _, err := a.CreateSession(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := b.UserBySession(ctx, tok); err != nil || got.ID != u.ID {
		t.Fatalf("other instance: %+v, %v", got, err)
	}
	if _, err := b.UserBySession(ctx, tok[:len(tok)-1]+"0"); err != ErrNotFound {
		t.Errorf("tampered token: err = %v, want ErrNotFound", err)
	}
	forged := strings.Replace(tok, u.ID, strings.Repeat("a", len(u.ID)), 1)
	if _, err := b.UserBySession(ctx, forged); err != ErrNotFound {
		t.Errorf("forged user: err = %v, want ErrNotFound", err)
	}
	if err := b.DeleteSession(ctx, tok); err != nil {
		t.Fatal(err)
	}
	if _, err := b.UserBySession(ctx, tok); err != ErrNotFound {
		t.Errorf("after logout: err = %v, want ErrNotFound", err)
	}
	// a token this instance has never seen, logged out first, stays out
	tok2, _, _ := a.CreateSession(ctx, u.ID)
	c := seeded(t)
	if err := c.DeleteSession(ctx, tok2); err != nil {
		t.Fatal(err)
	}
	if _, err := c.UserBySession(ctx, tok2); err != ErrNotFound {
		t.Errorf("unseen token logged out: err = %v, want ErrNotFound", err)
	}
}

func TestBookingCode(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 500; i++ {
		c := NewBookingCode()
		if len(c) != 9 || c[:4] != "SAR-" {
			t.Fatalf("bad code %q", c)
		}
		for _, r := range c[4:] {
			if !containsRune(codeAlphabet, r) {
				t.Fatalf("code %q has %q outside the alphabet", c, r)
			}
		}
		seen[c] = true
	}
	if len(seen) < 495 {
		t.Errorf("too many collisions: %d unique of 500", len(seen))
	}
}

func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}

func TestWipe(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("x"), 0o644)
	if err := Wipe(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, dbFile)); !os.IsNotExist(err) {
		t.Errorf("db still there: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "keep.txt")); err != nil {
		t.Errorf("wipe touched an unrelated file: %v", err)
	}
}
