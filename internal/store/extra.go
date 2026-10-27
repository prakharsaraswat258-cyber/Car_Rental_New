package store

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"saarthi/internal/domain"
)

// ShopBySlug loads a shop for its public profile.
func (s *Store) ShopBySlug(ctx context.Context, slug string) (Shop, error) {
	return scanShop(s.DB.QueryRowContext(ctx, shopSelect+" WHERE sh.slug = ?", slug))
}

// ShopRating is the review-weighted average across a shop's vehicles.
func (s *Store) ShopRating(ctx context.Context, shopID string) (avg float64, count int, err error) {
	var a sql.NullFloat64
	err = s.DB.QueryRowContext(ctx, `SELECT SUM(rating_avg * rating_count) / NULLIF(SUM(rating_count), 0), COALESCE(SUM(rating_count), 0)
		FROM vehicles WHERE shop_id = ? AND status = 'live'`, shopID).Scan(&a, &count)
	return a.Float64, count, err
}

// ShopReview is a review with the vehicle it was for.
type ShopReview struct {
	Review
	Vehicle string
}

// ShopReviews lists the latest reviews for a shop.
func (s *Store) ShopReviews(ctx context.Context, shopID string, limit int) ([]ShopReview, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT r.rating, r.body, u.name, r.created_at, v.make || ' ' || v.model
		FROM reviews r JOIN users u ON u.id = r.author_id JOIN vehicles v ON v.id = r.vehicle_id
		WHERE r.shop_id = ? ORDER BY r.created_at DESC LIMIT ?`, shopID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ShopReview
	for rows.Next() {
		var r ShopReview
		var at string
		if err := rows.Scan(&r.Rating, &r.Body, &r.Author, &at, &r.Vehicle); err != nil {
			return nil, err
		}
		r.CreatedAt, _ = parseTS(at)
		r.Author = firstName(r.Author)
		out = append(out, r)
	}
	return out, rows.Err()
}

// ShopLiveVehicles lists the vehicles renters can book from a shop.
func (s *Store) ShopLiveVehicles(ctx context.Context, shopID string) ([]Vehicle, error) {
	return s.queryVehicles(ctx, "WHERE v.shop_id = ? AND v.status = 'live' AND s.status = 'verified' ORDER BY v.rating_avg DESC, v.daily_paise", shopID)
}

// DayPoint is one day of the admin charts.
type DayPoint struct {
	Day      time.Time
	Bookings int
	Revenue  domain.Paise
}

// DailySeries returns bookings and revenue per IST day for the last n days
// (today included), zero-filled.
func (s *Store) DailySeries(ctx context.Context, n int) ([]DayPoint, error) {
	today := domain.StartOfDay(s.Now())
	first := today.AddDate(0, 0, -(n - 1))
	rows, err := s.DB.QueryContext(ctx, `SELECT date(created_at, '+5 hours', '+30 minutes') AS d, COUNT(*), COALESCE(SUM(total_paise), 0)
		FROM bookings WHERE status NOT IN ('pending_payment','expired') AND created_at >= ?
		GROUP BY d`, ts(first))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byDay := map[string]DayPoint{}
	for rows.Next() {
		var d string
		var p DayPoint
		if err := rows.Scan(&d, &p.Bookings, &p.Revenue); err != nil {
			return nil, err
		}
		byDay[d] = p
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]DayPoint, n)
	for i := range out {
		day := first.AddDate(0, 0, i)
		p := byDay[day.Format("2006-01-02")]
		p.Day = day
		out[i] = p
	}
	return out, nil
}

// PaletteHit is one command-palette result.
type PaletteHit struct {
	Kind, Title, Sub, URL string
}

// Palette finds bookings by code or renter, vehicles by reg or name, and
// (for admin, shopID == "") shops by name.
func (s *Store) Palette(ctx context.Context, shopID, q string, limit int) ([]PaletteHit, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, nil
	}
	like := "%" + strings.ReplaceAll(strings.ReplaceAll(q, "%", ""), "_", "") + "%"
	plain := "%" + strings.ReplaceAll(strings.ToUpper(strings.ReplaceAll(q, " ", "")), "%", "") + "%"
	var out []PaletteHit
	scope, args := "", []any{like, like, like}
	if shopID != "" {
		scope, args = " AND b.shop_id = ?", append(args, shopID)
	}
	args = append(args, limit)
	rows, err := s.DB.QueryContext(ctx, `SELECT b.id, b.code, u.name, v.make || ' ' || v.model, b.status FROM bookings b
		JOIN users u ON u.id = b.renter_id JOIN vehicles v ON v.id = b.vehicle_id
		WHERE (b.code LIKE ? OR u.name LIKE ? OR u.email LIKE ?)`+scope+` ORDER BY b.start_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, code, name, veh, st string
		if err := rows.Scan(&id, &code, &name, &veh, &st); err != nil {
			rows.Close()
			return nil, err
		}
		url := "/p/bookings/" + id
		if shopID == "" {
			url = "/admin/bookings/" + id
		}
		out = append(out, PaletteHit{Kind: "booking", Title: code + " · " + name, Sub: veh + " · " + domain.Status(st).Label(), URL: url})
	}
	rows.Close()

	vargs := []any{plain, like}
	vscope := ""
	if shopID != "" {
		vscope, vargs = " AND v.shop_id = ?", append(vargs, shopID)
	}
	vargs = append(vargs, limit)
	rows, err = s.DB.QueryContext(ctx, `SELECT v.id, v.make || ' ' || v.model, v.reg_number, s.name, v.shop_id FROM vehicles v JOIN shops s ON s.id = v.shop_id
		WHERE (REPLACE(UPPER(v.reg_number), ' ', '') LIKE ? OR (v.make || ' ' || v.model) LIKE ?)`+vscope+` ORDER BY v.make LIMIT ?`, vargs...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, name, reg, shop, sid string
		if err := rows.Scan(&id, &name, &reg, &shop, &sid); err != nil {
			rows.Close()
			return nil, err
		}
		url := "/p/vehicles/" + id + "/edit"
		if shopID == "" {
			url = "/v/" + id
		}
		sub := shop
		if reg != "" {
			sub = reg + " · " + shop
		}
		out = append(out, PaletteHit{Kind: "vehicle", Title: name, Sub: sub, URL: url})
	}
	rows.Close()

	if shopID == "" {
		rows, err = s.DB.QueryContext(ctx, `SELECT slug, name, status FROM shops WHERE name LIKE ? ORDER BY name LIMIT ?`, like, limit)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var slug, name, st string
			if err := rows.Scan(&slug, &name, &st); err != nil {
				rows.Close()
				return nil, err
			}
			out = append(out, PaletteHit{Kind: "shop", Title: name, Sub: "Shop · " + st, URL: "/s/" + slug})
		}
		rows.Close()
	}
	return out, nil
}
