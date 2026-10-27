package store

import "context"

// Counts is a quick health picture of the data (checked by the seed tests).
type Counts struct {
	Cities, Areas, Shops, Vehicles, LiveVehicles, Bookings, Users int
}

// Counts counts the main tables.
func (s *Store) Counts(ctx context.Context) (Counts, error) {
	var c Counts
	err := s.DB.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM cities),
		(SELECT COUNT(*) FROM areas),
		(SELECT COUNT(*) FROM shops),
		(SELECT COUNT(*) FROM vehicles),
		(SELECT COUNT(*) FROM vehicles v JOIN shops s ON s.id = v.shop_id WHERE v.status = 'live' AND s.status = 'verified'),
		(SELECT COUNT(*) FROM bookings),
		(SELECT COUNT(*) FROM users)`).
		Scan(&c.Cities, &c.Areas, &c.Shops, &c.Vehicles, &c.LiveVehicles, &c.Bookings, &c.Users)
	return c, err
}
