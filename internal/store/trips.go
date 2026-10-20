package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"saarthi/internal/domain"
)

// TripPhoto is one timestamped proof photo.
type TripPhoto struct {
	ID, BookingID, Phase, Angle, Path string
	TakenAt                           time.Time
}

// URL is the access-checked address of the photo.
func (p TripPhoto) URL() string { return "/photos/" + p.ID }

// TripPhotos lists a booking's proof photos.
func (s *Store) TripPhotos(ctx context.Context, bookingID string) ([]TripPhoto, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, booking_id, phase, angle, path, taken_at FROM trip_photos
		WHERE booking_id = ? ORDER BY phase, CASE angle WHEN 'front' THEN 0 WHEN 'back' THEN 1 WHEN 'left' THEN 2 WHEN 'right' THEN 3 ELSE 4 END`, bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TripPhoto
	for rows.Next() {
		var p TripPhoto
		var at string
		if err := rows.Scan(&p.ID, &p.BookingID, &p.Phase, &p.Angle, &p.Path, &at); err != nil {
			return nil, err
		}
		p.TakenAt, _ = parseTS(at)
		out = append(out, p)
	}
	return out, rows.Err()
}

// SaveTripPhoto records (or replaces) the photo for one angle and returns
// the replaced file's path, if any, so the caller can delete it.
func (s *Store) SaveTripPhoto(ctx context.Context, bookingID, phase, angle, path string) (TripPhoto, string, error) {
	p := TripPhoto{ID: NewID(), BookingID: bookingID, Phase: phase, Angle: angle, Path: path, TakenAt: s.Now().UTC().Truncate(time.Second)}
	var old string
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `SELECT path FROM trip_photos WHERE booking_id = ? AND phase = ? AND angle = ?`,
			bookingID, phase, angle).Scan(&old)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM trip_photos WHERE booking_id = ? AND phase = ? AND angle = ?`, bookingID, phase, angle); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO trip_photos (id, booking_id, phase, angle, path, taken_at) VALUES (?,?,?,?,?,?)`,
			p.ID, bookingID, phase, angle, path, ts(p.TakenAt))
		return err
	})
	return p, old, err
}

// TripPhotoAccess is a photo plus who may see it.
type TripPhotoAccess struct {
	TripPhoto
	RenterID, ShopOwnerID string
}

// TripPhotoByID loads a photo with its booking's renter and shop owner.
func (s *Store) TripPhotoByID(ctx context.Context, id string) (TripPhotoAccess, error) {
	var p TripPhotoAccess
	var at string
	err := s.DB.QueryRowContext(ctx, `SELECT tp.id, tp.booking_id, tp.phase, tp.angle, tp.path, tp.taken_at, b.renter_id, sh.owner_id
		FROM trip_photos tp JOIN bookings b ON b.id = tp.booking_id JOIN shops sh ON sh.id = b.shop_id WHERE tp.id = ?`, id).
		Scan(&p.ID, &p.BookingID, &p.Phase, &p.Angle, &p.Path, &at, &p.RenterID, &p.ShopOwnerID)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	p.TakenAt, _ = parseTS(at)
	return p, err
}

// --- reviews -----------------------------------------------------------------

// Review is a renter's rating of a closed trip.
type Review struct {
	Rating    int
	Body      string
	Author    string
	CreatedAt time.Time
}

// AddReview stores a 1–5 star review for a closed trip and updates the
// vehicle's average.
func (s *Store) AddReview(ctx context.Context, renterID, code string, rating int, body string) error {
	if rating < 1 || rating > 5 {
		return ErrBadTransition
	}
	b, err := s.BookingByCode(ctx, code)
	if err != nil {
		return err
	}
	if b.RenterID != renterID {
		return ErrNotFound
	}
	if b.Status != domain.StatusClosed || b.Reviewed {
		return ErrBadTransition
	}
	body = strings.TrimSpace(body)
	if r := []rune(body); len(r) > 600 {
		body = string(r[:600])
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO reviews (id, booking_id, author_id, vehicle_id, shop_id, rating, body, created_at)
			VALUES (?,?,?,?,?,?,?,?)`, NewID(), b.ID, renterID, b.VehicleID, b.ShopID, rating, body, ts(s.Now())); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE vehicles SET rating_avg = (rating_avg * rating_count + ?) / (rating_count + 1),
			rating_count = rating_count + 1 WHERE id = ?`, rating, b.VehicleID)
		return err
	})
}

// ReviewFor returns the review of a booking, if any.
func (s *Store) ReviewFor(ctx context.Context, bookingID string) (*Review, error) {
	var r Review
	var at string
	err := s.DB.QueryRowContext(ctx, `SELECT r.rating, r.body, u.name, r.created_at FROM reviews r JOIN users u ON u.id = r.author_id
		WHERE r.booking_id = ?`, bookingID).Scan(&r.Rating, &r.Body, &r.Author, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.CreatedAt, _ = parseTS(at)
	r.Author = firstName(r.Author)
	return &r, nil
}

// VehicleReviews lists the latest reviews for a vehicle.
func (s *Store) VehicleReviews(ctx context.Context, vehicleID string, limit int) ([]Review, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT r.rating, r.body, u.name, r.created_at FROM reviews r JOIN users u ON u.id = r.author_id
		WHERE r.vehicle_id = ? ORDER BY r.created_at DESC LIMIT ?`, vehicleID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Review
	for rows.Next() {
		var r Review
		var at string
		if err := rows.Scan(&r.Rating, &r.Body, &r.Author, &at); err != nil {
			return nil, err
		}
		r.CreatedAt, _ = parseTS(at)
		r.Author = firstName(r.Author)
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- admin -------------------------------------------------------------------

// KPIs are the admin overview numbers.
type KPIs struct {
	Bookings     int
	Revenue      domain.Paise
	ActiveTrips  int
	AvgRelease   time.Duration
	Released     int
	PendingShops int
	Renters      int
}

// AdminKPIs computes the overview.
func (s *Store) AdminKPIs(ctx context.Context) (KPIs, error) {
	if err := s.ExpireStale(ctx); err != nil {
		return KPIs{}, err
	}
	var k KPIs
	var avgHours sql.NullFloat64
	err := s.DB.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM bookings WHERE status NOT IN ('pending_payment','expired')),
		(SELECT COALESCE(SUM(total_paise), 0) FROM bookings WHERE status IN ('awaiting_shop','confirmed','ready','active','returned','closed')),
		(SELECT COUNT(*) FROM bookings WHERE status = 'active'),
		(SELECT AVG((julianday(d.released_at) - julianday(b.returned_at)) * 24) FROM deposits d JOIN bookings b ON b.id = d.booking_id
			WHERE d.released_at IS NOT NULL AND b.returned_at IS NOT NULL),
		(SELECT COUNT(*) FROM deposits d JOIN bookings b ON b.id = d.booking_id WHERE d.released_at IS NOT NULL AND b.returned_at IS NOT NULL),
		(SELECT COUNT(*) FROM shops WHERE status = 'pending'),
		(SELECT COUNT(*) FROM users WHERE role = 'renter')`).
		Scan(&k.Bookings, &k.Revenue, &k.ActiveTrips, &avgHours, &k.Released, &k.PendingShops, &k.Renters)
	if avgHours.Valid {
		k.AvgRelease = time.Duration(avgHours.Float64 * float64(time.Hour))
	}
	return k, err
}
