package store

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"saarthi/internal/domain"
)

// Photo is an uploaded vehicle picture.
type Photo struct {
	ID       string
	Path     string
	Position int
}

// URL is where the browser loads the photo from.
func (p Photo) URL() string { return "/vphotos/" + p.ID }

// Vehicle is a rentable vehicle with its shop and place joined in.
type Vehicle struct {
	ID, ShopID, Type, Make, Model string
	Year                          int
	Reg, Fuel, Trans              string
	Seats                         int
	Features                      []string
	Description                   string
	Hourly, Daily, Deposit        domain.Paise
	KmPerDay                      int
	ExtraKm, LateFee              domain.Paise
	MinHours                      int
	InstantBook                   bool
	Status                        string
	Odometer                      int
	RatingAvg                     float64
	RatingCount                   int
	CreatedAt                     time.Time

	ShopName, ShopSlug, ShopStatus, ShopAddress, ShopPhone, ShopWhatsApp string
	ShopLat, ShopLng                                                     float64
	CityName, CitySlug, AreaName, AreaID                                 string

	Photos []Photo
}

// Name is "Honda Activa 6G".
func (v Vehicle) Name() string { return strings.TrimSpace(v.Make + " " + v.Model) }

// Rates is what pricing needs.
func (v Vehicle) Rates() domain.Rates {
	return domain.Rates{Hourly: v.Hourly, Daily: v.Daily, Deposit: v.Deposit, KmLimitPerDay: v.KmPerDay, MinHours: v.MinHours}
}

// Class is two_wheeler, four_wheeler or cycle.
func (v Vehicle) Class() string { return domain.ClassOf(v.Type, v.Seats) }

// ProofPhrase is the check-in ask for this vehicle ("four quick photos and
// the odometer").
func (v Vehicle) ProofPhrase() string { return domain.ProofPhrase(v.Class()) }

// Art names the line-art silhouette used when there is no photo.
func (v Vehicle) Art() string { return ArtFor(v.Type, v.Seats) }

// ArtFor picks scooter, bike, car, ev-car, ev-scooter or cycle.
func ArtFor(typ string, seats int) string {
	if typ == "ev" {
		if domain.ClassOf(typ, seats) == domain.ClassFourWheeler {
			return "ev-car"
		}
		return "ev-scooter"
	}
	switch typ {
	case "scooter", "bike", "car", "cycle":
		return typ
	}
	return "scooter"
}

// Cover is the first photo's URL, or "" to use the line-art.
func (v Vehicle) Cover() string {
	if len(v.Photos) > 0 {
		return v.Photos[0].URL()
	}
	return ""
}

// Place is "Nakki Lake, Mount Abu".
func (v Vehicle) Place() string {
	if v.AreaName != "" {
		return v.AreaName + ", " + v.CityName
	}
	return v.CityName
}

// Searchable reports whether renters can find and book it.
func (v Vehicle) Searchable() bool { return v.Status == "live" && v.ShopStatus == "verified" }

// RatingText is "4.8" or "New".
func (v Vehicle) RatingText() string {
	if v.RatingCount == 0 {
		return "New"
	}
	return strconv.FormatFloat(v.RatingAvg, 'f', 1, 64)
}

const vehicleSelect = `SELECT v.id, v.shop_id, v.type, v.make, v.model, v.year, v.reg_number, v.fuel, v.transmission, v.seats,
	v.features, v.description, v.hourly_paise, v.daily_paise, v.deposit_paise, v.km_limit_per_day, v.extra_km_paise,
	v.late_fee_per_hour_paise, v.min_hours, v.instant_book, v.status, v.odometer_km, v.rating_avg, v.rating_count, v.created_at,
	s.name, s.slug, s.status, s.address, s.phone, s.whatsapp, s.lat, s.lng, c.name, c.slug, COALESCE(a.name, ''), COALESCE(a.id, '')
FROM vehicles v
JOIN shops s ON s.id = v.shop_id
JOIN cities c ON c.id = s.city_id
LEFT JOIN areas a ON a.id = s.area_id`

type scanner interface{ Scan(...any) error }

func scanVehicle(row scanner) (Vehicle, error) {
	var v Vehicle
	var features, created string
	var instant int
	err := row.Scan(&v.ID, &v.ShopID, &v.Type, &v.Make, &v.Model, &v.Year, &v.Reg, &v.Fuel, &v.Trans, &v.Seats,
		&features, &v.Description, &v.Hourly, &v.Daily, &v.Deposit, &v.KmPerDay, &v.ExtraKm,
		&v.LateFee, &v.MinHours, &instant, &v.Status, &v.Odometer, &v.RatingAvg, &v.RatingCount, &created,
		&v.ShopName, &v.ShopSlug, &v.ShopStatus, &v.ShopAddress, &v.ShopPhone, &v.ShopWhatsApp, &v.ShopLat, &v.ShopLng,
		&v.CityName, &v.CitySlug, &v.AreaName, &v.AreaID)
	if errors.Is(err, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	if err != nil {
		return v, err
	}
	v.InstantBook = instant == 1
	v.Features = splitList(features)
	v.CreatedAt, err = parseTS(created)
	return v, err
}

func splitList(csv string) []string {
	var out []string
	for _, f := range strings.Split(csv, ",") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

func (s *Store) queryVehicles(ctx context.Context, where string, args ...any) ([]Vehicle, error) {
	rows, err := s.DB.QueryContext(ctx, vehicleSelect+" "+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Vehicle
	for rows.Next() {
		v, err := scanVehicle(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, s.loadPhotos(ctx, out)
}

func (s *Store) loadPhotos(ctx context.Context, vs []Vehicle) error {
	if len(vs) == 0 {
		return nil
	}
	idx := map[string]int{}
	ids := make([]any, len(vs))
	for i, v := range vs {
		idx[v.ID] = i
		ids[i] = v.ID
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT id, vehicle_id, path, position FROM vehicle_photos
		WHERE vehicle_id IN (`+placeholders(len(ids))+`) ORDER BY position, id`, ids...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var p Photo
		var vid string
		if err := rows.Scan(&p.ID, &vid, &p.Path, &p.Position); err != nil {
			return err
		}
		vs[idx[vid]].Photos = append(vs[idx[vid]].Photos, p)
	}
	return rows.Err()
}

func placeholders(n int) string {
	if n <= 0 {
		return "NULL"
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// VehicleByID loads one vehicle (any status).
func (s *Store) VehicleByID(ctx context.Context, id string) (Vehicle, error) {
	vs, err := s.queryVehicles(ctx, "WHERE v.id = ?", id)
	if err != nil {
		return Vehicle{}, err
	}
	if len(vs) == 0 {
		return Vehicle{}, ErrNotFound
	}
	return vs[0], nil
}

// VehiclesByIDs loads vehicles keeping the order of ids.
func (s *Store) VehiclesByIDs(ctx context.Context, ids []string) ([]Vehicle, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	vs, err := s.queryVehicles(ctx, "WHERE v.id IN ("+placeholders(len(ids))+")", args...)
	if err != nil {
		return nil, err
	}
	pos := map[string]int{}
	for i, id := range ids {
		pos[id] = i
	}
	sort.Slice(vs, func(i, j int) bool { return pos[vs[i].ID] < pos[vs[j].ID] })
	return vs, nil
}

// ShopVehicles lists a shop's whole fleet, newest first.
func (s *Store) ShopVehicles(ctx context.Context, shopID string) ([]Vehicle, error) {
	return s.queryVehicles(ctx, "WHERE v.shop_id = ? ORDER BY CASE v.status WHEN 'live' THEN 0 WHEN 'draft' THEN 1 ELSE 2 END, v.type, v.make, v.model", shopID)
}

// PopularVehicles are the best-rated searchable vehicles.
func (s *Store) PopularVehicles(ctx context.Context, n int) ([]Vehicle, error) {
	return s.queryVehicles(ctx, `WHERE v.status = 'live' AND s.status = 'verified'
		ORDER BY v.rating_avg * MIN(v.rating_count, 50) DESC, v.rating_count DESC LIMIT ?`, n)
}

// SearchFilter narrows /search.
type SearchFilter struct {
	CitySlug, AreaID    string
	Types, Fuels, Trans []string
}

// CandidateVehicles returns searchable vehicles matching place/type/fuel/
// transmission. Availability and price are applied by the caller.
func (s *Store) CandidateVehicles(ctx context.Context, f SearchFilter) ([]Vehicle, error) {
	where := []string{"v.status = 'live'", "s.status = 'verified'"}
	var args []any
	if f.CitySlug != "" {
		where = append(where, "c.slug = ?")
		args = append(args, f.CitySlug)
	}
	if f.AreaID != "" {
		where = append(where, "s.area_id = ?")
		args = append(args, f.AreaID)
	}
	in := func(col string, vals []string) {
		if len(vals) == 0 {
			return
		}
		where = append(where, col+" IN ("+placeholders(len(vals))+")")
		for _, v := range vals {
			args = append(args, v)
		}
	}
	in("v.type", f.Types)
	in("v.fuel", f.Fuels)
	in("v.transmission", f.Trans)
	return s.queryVehicles(ctx, "WHERE "+strings.Join(where, " AND ")+" ORDER BY v.daily_paise, v.id", args...)
}

// VehicleInput is what the provider editor saves.
type VehicleInput struct {
	Type, Make, Model string
	Year              int
	Reg, Fuel, Trans  string
	Seats             int
	Features          []string
	Description       string
	Hourly, Daily     domain.Paise
	Deposit           domain.Paise
	KmPerDay          int
	ExtraKm, LateFee  domain.Paise
	MinHours          int
	InstantBook       bool
}

// CreateVehicle adds a draft vehicle to a shop.
func (s *Store) CreateVehicle(ctx context.Context, shopID string, in VehicleInput) (string, error) {
	id := NewID()
	_, err := s.DB.ExecContext(ctx, `INSERT INTO vehicles (id, shop_id, type, make, model, year, reg_number, fuel, transmission, seats,
			features, description, hourly_paise, daily_paise, deposit_paise, km_limit_per_day, extra_km_paise,
			late_fee_per_hour_paise, min_hours, instant_book, status, odometer_km, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,'draft',0,?)`,
		id, shopID, in.Type, in.Make, in.Model, in.Year, in.Reg, in.Fuel, in.Trans, in.Seats,
		strings.Join(in.Features, ","), in.Description, in.Hourly, in.Daily, in.Deposit, in.KmPerDay, in.ExtraKm,
		in.LateFee, in.MinHours, boolInt(in.InstantBook), ts(s.Now()))
	return id, err
}

// UpdateVehicle saves editor fields for a vehicle of shopID.
func (s *Store) UpdateVehicle(ctx context.Context, shopID, id string, in VehicleInput) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE vehicles SET type=?, make=?, model=?, year=?, reg_number=?, fuel=?, transmission=?, seats=?,
			features=?, description=?, hourly_paise=?, daily_paise=?, deposit_paise=?, km_limit_per_day=?, extra_km_paise=?,
			late_fee_per_hour_paise=?, min_hours=?, instant_book=?
		WHERE id = ? AND shop_id = ?`,
		in.Type, in.Make, in.Model, in.Year, in.Reg, in.Fuel, in.Trans, in.Seats,
		strings.Join(in.Features, ","), in.Description, in.Hourly, in.Daily, in.Deposit, in.KmPerDay, in.ExtraKm,
		in.LateFee, in.MinHours, boolInt(in.InstantBook), id, shopID)
	return mustAffect(res, err)
}

// SetVehicleStatus moves a vehicle of shopID to draft / live / hidden.
func (s *Store) SetVehicleStatus(ctx context.Context, shopID, id, status string) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE vehicles SET status = ? WHERE id = ? AND shop_id = ?`, status, id, shopID)
	return mustAffect(res, err)
}

// AddVehiclePhoto records an uploaded photo at the end of the gallery.
func (s *Store) AddVehiclePhoto(ctx context.Context, vehicleID, id, path string) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO vehicle_photos (id, vehicle_id, path, position)
		VALUES (?, ?, ?, (SELECT COALESCE(MAX(position), -1) + 1 FROM vehicle_photos WHERE vehicle_id = ?))`,
		id, vehicleID, path, vehicleID)
	return err
}

// VehiclePhoto finds a photo by id.
func (s *Store) VehiclePhoto(ctx context.Context, id string) (Photo, error) {
	var p Photo
	err := s.DB.QueryRowContext(ctx, `SELECT id, path, position FROM vehicle_photos WHERE id = ?`, id).Scan(&p.ID, &p.Path, &p.Position)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

// DeleteVehiclePhoto removes a photo of a vehicle in shopID and returns its path.
func (s *Store) DeleteVehiclePhoto(ctx context.Context, shopID, vehicleID, photoID string) (string, error) {
	var path string
	err := s.DB.QueryRowContext(ctx, `SELECT p.path FROM vehicle_photos p JOIN vehicles v ON v.id = p.vehicle_id
		WHERE p.id = ? AND p.vehicle_id = ? AND v.shop_id = ?`, photoID, vehicleID, shopID).Scan(&path)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	_, err = s.DB.ExecContext(ctx, `DELETE FROM vehicle_photos WHERE id = ?`, photoID)
	return path, err
}

func mustAffect(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
