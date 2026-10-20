package store

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"time"

	"saarthi/internal/domain"
)

// Shop is a rental business.
type Shop struct {
	ID, OwnerID, Name, Slug, Phone, WhatsApp, Address string
	CityID, CityName, AreaID, AreaName                string
	Lat, Lng                                          float64
	Status                                            string
	CreatedAt                                         time.Time
	OwnerName, OwnerEmail                             string
	VehicleCount, LiveCount                           int
}

// Place is "Nakki Lake, Mount Abu".
func (s Shop) Place() string {
	if s.AreaName != "" {
		return s.AreaName + ", " + s.CityName
	}
	return s.CityName
}

const shopSelect = `SELECT sh.id, sh.owner_id, sh.name, sh.slug, sh.phone, sh.whatsapp, sh.address,
	sh.city_id, c.name, COALESCE(sh.area_id, ''), COALESCE(a.name, ''), sh.lat, sh.lng, sh.status, sh.created_at,
	u.name, u.email,
	(SELECT COUNT(*) FROM vehicles v WHERE v.shop_id = sh.id),
	(SELECT COUNT(*) FROM vehicles v WHERE v.shop_id = sh.id AND v.status = 'live')
FROM shops sh
JOIN cities c ON c.id = sh.city_id
LEFT JOIN areas a ON a.id = sh.area_id
JOIN users u ON u.id = sh.owner_id`

func scanShop(row scanner) (Shop, error) {
	var s Shop
	var created string
	err := row.Scan(&s.ID, &s.OwnerID, &s.Name, &s.Slug, &s.Phone, &s.WhatsApp, &s.Address,
		&s.CityID, &s.CityName, &s.AreaID, &s.AreaName, &s.Lat, &s.Lng, &s.Status, &created,
		&s.OwnerName, &s.OwnerEmail, &s.VehicleCount, &s.LiveCount)
	if errors.Is(err, sql.ErrNoRows) {
		return s, ErrNotFound
	}
	if err != nil {
		return s, err
	}
	s.CreatedAt, err = parseTS(created)
	return s, err
}

// ShopByOwner returns the shop a provider runs.
func (s *Store) ShopByOwner(ctx context.Context, ownerID string) (Shop, error) {
	return scanShop(s.DB.QueryRowContext(ctx, shopSelect+" WHERE sh.owner_id = ? ORDER BY sh.created_at LIMIT 1", ownerID))
}

// Shops lists every shop for admin: pending first.
func (s *Store) Shops(ctx context.Context) ([]Shop, error) {
	rows, err := s.DB.QueryContext(ctx, shopSelect+` ORDER BY CASE sh.status WHEN 'pending' THEN 0 WHEN 'verified' THEN 1 ELSE 2 END, sh.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Shop
	for rows.Next() {
		sh, err := scanShop(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sh)
	}
	return out, rows.Err()
}

// ShopByID loads one shop.
func (s *Store) ShopByID(ctx context.Context, id string) (Shop, error) {
	return scanShop(s.DB.QueryRowContext(ctx, shopSelect+" WHERE sh.id = ?", id))
}

// SetShopStatus verifies or suspends a shop.
func (s *Store) SetShopStatus(ctx context.Context, id, status string) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE shops SET status = ? WHERE id = ?`, status, id)
	return mustAffect(res, err)
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// CreateShop opens a pending shop for a new provider, with the standard add-ons.
func (s *Store) CreateShop(ctx context.Context, ownerID, name, cityID, areaID, address, phone string) (Shop, error) {
	var lat, lng float64
	if areaID != "" {
		if err := s.DB.QueryRowContext(ctx, `SELECT lat, lng FROM areas WHERE id = ? AND city_id = ?`, areaID, cityID).Scan(&lat, &lng); err != nil {
			areaID = ""
		}
	}
	if areaID == "" {
		if err := s.DB.QueryRowContext(ctx, `SELECT lat, lng FROM cities WHERE id = ?`, cityID).Scan(&lat, &lng); err != nil {
			return Shop{}, ErrNotFound
		}
	}
	slug := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if slug == "" {
		slug = "shop"
	}
	slug += "-" + NewID()[:6]
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone)
	if len(digits) == 10 {
		digits = "91" + digits
	}
	id := NewID()
	var area any
	if areaID != "" {
		area = areaID
	}
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO shops (id, owner_id, name, slug, phone, whatsapp, address, city_id, area_id, lat, lng, status, created_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,'pending',?)`,
			id, ownerID, name, slug, phone, digits, address, cityID, area, lat, lng, ts(s.Now())); err != nil {
			return err
		}
		for _, a := range defaultAddons {
			if _, err := tx.ExecContext(ctx, `INSERT INTO addons (id, shop_id, name, price_paise, per, fits) VALUES (?,?,?,?,?,?)`,
				NewID(), id, a.Name, a.Price, string(a.Per), a.Fits); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Shop{}, err
	}
	return s.ShopByID(ctx, id)
}

// --- add-ons -----------------------------------------------------------------

// ShopAddon is a shop's optional extra with the vehicle classes it fits.
type ShopAddon struct {
	domain.Addon
	Fits string
}

var defaultAddons = []ShopAddon{
	{domain.Addon{Name: "Extra helmet", Price: domain.Rupees(50), Per: domain.PerTrip}, domain.ClassTwoWheeler},
	{domain.Addon{Name: "Phone holder", Price: domain.Rupees(30), Per: domain.PerTrip}, ""},
	{domain.Addon{Name: "Child seat", Price: domain.Rupees(150), Per: domain.PerDay}, domain.ClassFourWheeler},
}

func shopAddons(ctx context.Context, q querier, shopID string) ([]ShopAddon, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, name, price_paise, per, fits FROM addons WHERE shop_id = ? ORDER BY price_paise, name`, shopID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ShopAddon
	for rows.Next() {
		var a ShopAddon
		var per string
		if err := rows.Scan(&a.ID, &a.Name, &a.Price, &per, &a.Fits); err != nil {
			return nil, err
		}
		a.Per = domain.AddonPer(per)
		out = append(out, a)
	}
	return out, rows.Err()
}

// AddonsFor lists the add-ons a shop offers for a vehicle.
func (s *Store) AddonsFor(ctx context.Context, v Vehicle) ([]ShopAddon, error) {
	all, err := shopAddons(ctx, s.DB, v.ShopID)
	if err != nil {
		return nil, err
	}
	var out []ShopAddon
	for _, a := range all {
		if domain.AddonFits(a.Fits, v.Class()) {
			out = append(out, a)
		}
	}
	return out, nil
}

// --- blocks ------------------------------------------------------------------

// Block is a stretch a shop keeps a vehicle off the market.
type Block struct {
	ID, VehicleID string
	Start, End    time.Time
	Note          string
}

// CreateBlock blocks a shop's vehicle unless a booking is in the way.
func (s *Store) CreateBlock(ctx context.Context, shopID, vehicleID string, start, end time.Time, note string) (Block, error) {
	if !end.After(start) {
		return Block{}, ErrBadTransition
	}
	b := Block{ID: NewID(), VehicleID: vehicleID, Start: start, End: end, Note: note}
	now := s.Now()
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		if err := expireStale(ctx, tx, now); err != nil {
			return err
		}
		var owner string
		if err := tx.QueryRowContext(ctx, `SELECT shop_id FROM vehicles WHERE id = ?`, vehicleID).Scan(&owner); err != nil || owner != shopID {
			return ErrNotFound
		}
		taken, err := busy(ctx, tx, now, []string{vehicleID}, start, end, true)
		if err != nil {
			return err
		}
		if len(taken) > 0 {
			return &SlotTakenError{Busy: taken[0]}
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO blocks (id, vehicle_id, start_at, end_at, note) VALUES (?,?,?,?,?)`,
			b.ID, vehicleID, ts(start), ts(end), note)
		return err
	})
	return b, err
}

// DeleteBlock removes a block of shopID's vehicle and returns it (for Undo).
func (s *Store) DeleteBlock(ctx context.Context, shopID, blockID string) (Block, error) {
	var b Block
	var st, en string
	err := s.DB.QueryRowContext(ctx, `SELECT bl.id, bl.vehicle_id, bl.start_at, bl.end_at, bl.note FROM blocks bl
		JOIN vehicles v ON v.id = bl.vehicle_id WHERE bl.id = ? AND v.shop_id = ?`, blockID, shopID).
		Scan(&b.ID, &b.VehicleID, &st, &en, &b.Note)
	if errors.Is(err, sql.ErrNoRows) {
		return b, ErrNotFound
	}
	if err != nil {
		return b, err
	}
	b.Start, _ = parseTS(st)
	b.End, _ = parseTS(en)
	_, err = s.DB.ExecContext(ctx, `DELETE FROM blocks WHERE id = ?`, blockID)
	return b, err
}

// ShopBlocks lists a shop's blocks overlapping [from, to).
func (s *Store) ShopBlocks(ctx context.Context, shopID string, from, to time.Time) ([]Block, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT bl.id, bl.vehicle_id, bl.start_at, bl.end_at, bl.note FROM blocks bl
		JOIN vehicles v ON v.id = bl.vehicle_id WHERE v.shop_id = ? AND bl.start_at < ? AND bl.end_at > ? ORDER BY bl.start_at`,
		shopID, ts(to), ts(from))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Block
	for rows.Next() {
		var b Block
		var st, en string
		if err := rows.Scan(&b.ID, &b.VehicleID, &st, &en, &b.Note); err != nil {
			return nil, err
		}
		b.Start, _ = parseTS(st)
		b.End, _ = parseTS(en)
		out = append(out, b)
	}
	return out, rows.Err()
}
