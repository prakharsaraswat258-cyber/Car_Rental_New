package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"saarthi/internal/domain"
)

// Errors callers turn into friendly copy.
var (
	ErrSlotTaken       = errors.New("those dates are taken")
	ErrBadTransition   = errors.New("that step isn't possible right now")
	ErrExpired         = errors.New("the 15-minute payment window closed")
	ErrPaymentDeclined = errors.New("payment declined")
	ErrNotBookable     = errors.New("this vehicle can't be booked right now")
	ErrInPast          = errors.New("pickup time is in the past")
	ErrPhotosMissing   = errors.New("proof photos missing")
	ErrOdometer        = errors.New("odometer reading looks wrong")
)

// SlotTakenError says what is in the way.
type SlotTakenError struct{ Busy Busy }

func (e *SlotTakenError) Error() string { return ErrSlotTaken.Error() }
func (e *SlotTakenError) Unwrap() error { return ErrSlotTaken }

// Busy is a stretch of time a vehicle is not free.
type Busy struct {
	VehicleID  string
	Start, End time.Time
	Kind       string // booking | block
	ID         string
	Label      string // booking code or block note
}

type querier interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
	ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error)
}

// --- lazy expiry & credit ----------------------------------------------------

// ExpireStale moves unpaid bookings older than 15 minutes to expired and
// released deposits older than a minute to credited (D-009, D-011). It is
// called before reads instead of running a background worker.
func (s *Store) ExpireStale(ctx context.Context) error {
	now := s.Now()
	var stale int
	err := s.DB.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM bookings WHERE status = 'pending_payment' AND created_at <= ?) +
		(SELECT COUNT(*) FROM deposits WHERE status = 'released' AND released_at <= ?)`,
		ts(now.Add(-domain.PaymentHold)), ts(now.Add(-domain.CreditDelay))).Scan(&stale)
	if err != nil || stale == 0 {
		return err
	}
	return s.inTx(ctx, func(tx *sql.Tx) error { return expireStale(ctx, tx, now) })
}

func expireStale(ctx context.Context, tx *sql.Tx, now time.Time) error {
	ids, err := returningIDs(ctx, tx, `UPDATE bookings SET status = 'expired', updated_at = ?
		WHERE status = 'pending_payment' AND created_at <= ? RETURNING id`, ts(now), ts(now.Add(-domain.PaymentHold)))
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := addEvent(ctx, tx, id, domain.StatusExpired, "The 15-minute payment window closed. Nothing was charged.", now); err != nil {
			return err
		}
	}
	rows, err := tx.QueryContext(ctx, `UPDATE deposits SET status = 'credited',
			credited_at = strftime('%Y-%m-%dT%H:%M:%SZ', released_at, '+60 seconds')
		WHERE status = 'released' AND released_at <= ? RETURNING booking_id, credited_at, held_paise - deducted_paise`,
		ts(now.Add(-domain.CreditDelay)))
	if err != nil {
		return err
	}
	type credit struct {
		id, at string
		amt    domain.Paise
	}
	var credits []credit
	for rows.Next() {
		var c credit
		if err := rows.Scan(&c.id, &c.at, &c.amt); err != nil {
			rows.Close()
			return err
		}
		credits = append(credits, c)
	}
	rows.Close()
	for _, c := range credits {
		at, _ := parseTS(c.at)
		var st string
		tx.QueryRowContext(ctx, `SELECT status FROM bookings WHERE id = ?`, c.id).Scan(&st)
		if err := addEvent(ctx, tx, c.id, domain.Status(st), fmt.Sprintf("%s credited back to your account", c.amt), at); err != nil {
			return err
		}
	}
	return nil
}

func returningIDs(ctx context.Context, q querier, query string, args ...any) ([]string, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func addEvent(ctx context.Context, q querier, bookingID string, st domain.Status, note string, at time.Time) error {
	_, err := q.ExecContext(ctx, `INSERT INTO booking_events (id, booking_id, status, note, created_at) VALUES (?,?,?,?,?)`,
		NewID(), bookingID, string(st), note, ts(at))
	return err
}

// --- availability ------------------------------------------------------------

// busy lists bookings holding a slot and blocks that overlap [from, to)
// (half-open, D-010). skipBlocks ignores blocks (used when creating one).
func busy(ctx context.Context, q querier, now time.Time, vehicleIDs []string, from, to time.Time, skipBlocks bool) ([]Busy, error) {
	if len(vehicleIDs) == 0 {
		return nil, nil
	}
	args := []any{}
	for _, id := range vehicleIDs {
		args = append(args, id)
	}
	bargs := append(append([]any{}, args...), ts(to), ts(from), ts(now.Add(-domain.PaymentHold)))
	rows, err := q.QueryContext(ctx, `SELECT vehicle_id, start_at, end_at, id, code FROM bookings
		WHERE vehicle_id IN (`+placeholders(len(vehicleIDs))+`) AND start_at < ? AND end_at > ?
		AND (status IN ('awaiting_shop','confirmed','ready','active') OR (status = 'pending_payment' AND created_at > ?))
		ORDER BY start_at`, bargs...)
	if err != nil {
		return nil, err
	}
	out, err := scanBusy(rows, "booking")
	if err != nil || skipBlocks {
		return out, err
	}
	rows, err = q.QueryContext(ctx, `SELECT vehicle_id, start_at, end_at, id, note FROM blocks
		WHERE vehicle_id IN (`+placeholders(len(vehicleIDs))+`) AND start_at < ? AND end_at > ? ORDER BY start_at`,
		append(append([]any{}, args...), ts(to), ts(from))...)
	if err != nil {
		return nil, err
	}
	blocks, err := scanBusy(rows, "block")
	return append(out, blocks...), err
}

func scanBusy(rows *sql.Rows, kind string) ([]Busy, error) {
	defer rows.Close()
	var out []Busy
	for rows.Next() {
		var b Busy
		var s, e string
		if err := rows.Scan(&b.VehicleID, &s, &e, &b.ID, &b.Label); err != nil {
			return nil, err
		}
		b.Kind = kind
		b.Start, _ = parseTS(s)
		b.End, _ = parseTS(e)
		out = append(out, b)
	}
	return out, rows.Err()
}

// Busy returns, per vehicle, everything that overlaps [from, to).
func (s *Store) Busy(ctx context.Context, vehicleIDs []string, from, to time.Time) (map[string][]Busy, error) {
	if err := s.ExpireStale(ctx); err != nil {
		return nil, err
	}
	list, err := busy(ctx, s.DB, s.Now(), vehicleIDs, from, to, false)
	if err != nil {
		return nil, err
	}
	out := map[string][]Busy{}
	for _, b := range list {
		out[b.VehicleID] = append(out[b.VehicleID], b)
	}
	return out, nil
}

// --- bookings ----------------------------------------------------------------

// Booking is a reservation with what every screen needs joined in.
type Booking struct {
	ID, Code, RenterID, VehicleID, ShopID string
	Start, End                            time.Time
	Status                                domain.Status
	Rent, Addons, Tax, Total, Deposit     domain.Paise
	KmIncluded                            int
	Lines                                 []domain.Line
	OdoStart, OdoEnd                      int
	ReturnedAt, CreatedAt, UpdatedAt      time.Time

	VehicleMake, VehicleModel, VehicleType, VehicleReg string
	VehicleSeats                                       int
	ExtraKmRate, LateFeeRate                           domain.Paise
	InstantBook                                        bool
	CoverID                                            string

	RenterName, RenterEmail, RenterPhone string

	ShopName, ShopAddress, ShopPhone, ShopWhatsApp, ShopOwnerID string
	ShopLat, ShopLng                                            float64

	DepositStatus     domain.DepositStatus // "" when no deposit row
	DepositHeld       domain.Paise
	DepositDeducted   domain.Paise
	DepositReleasedAt time.Time
	DepositCreditedAt time.Time

	VehicleFuel               string
	CheckinShots, ReturnShots map[string]bool // proof shots uploaded per phase
	Reviewed                  bool
}

// Class is two_wheeler, four_wheeler or cycle.
func (b Booking) Class() string { return domain.ClassOf(b.VehicleType, b.VehicleSeats) }

// Electric reports an EV (the fuel shot becomes a charge-level shot).
func (b Booking) Electric() bool { return b.VehicleFuel == "electric" }

// VehicleName is "Honda Activa 6G".
func (b Booking) VehicleName() string { return strings.TrimSpace(b.VehicleMake + " " + b.VehicleModel) }

// Art is the vehicle's line-art name.
func (b Booking) Art() string { return ArtFor(b.VehicleType, b.VehicleSeats) }

// Cover is the vehicle's first photo URL or "".
func (b Booking) Cover() string {
	if b.CoverID == "" {
		return ""
	}
	return "/vphotos/" + b.CoverID
}

// CheckedIn is true once every check-in shot for the vehicle class is in.
func (b Booking) CheckedIn() bool { return domain.ShotsComplete(b.Class(), b.CheckinShots) }

// ReturnPhotosDone is true once every return shot is in.
func (b Booking) ReturnPhotosDone() bool { return domain.ShotsComplete(b.Class(), b.ReturnShots) }

// Step is the renter status-line position (domain.TripStep).
func (b Booking) Step() int { return domain.TripStep(b.Status, b.CheckedIn()) }

// PayNow is total + deposit.
func (b Booking) PayNow() domain.Paise { return b.Total + b.Deposit }

// PayBy is when the unpaid hold ends.
func (b Booking) PayBy() time.Time { return b.CreatedAt.Add(domain.PaymentHold) }

// Tab is upcoming, active or past for My trips.
func (b Booking) Tab() string {
	switch b.Status {
	case domain.StatusActive, domain.StatusReturned:
		return "active"
	case domain.StatusPendingPayment, domain.StatusAwaitingShop, domain.StatusConfirmed, domain.StatusReady:
		return "upcoming"
	}
	return "past"
}

// CancelWindow is how long before pickup free cancellation ends.
const CancelWindow = 24 * time.Hour

// CanCancel reports whether the renter can cancel now.
func (b Booking) CanCancel(now time.Time) bool {
	switch b.Status {
	case domain.StatusPendingPayment, domain.StatusAwaitingShop:
		return true
	case domain.StatusConfirmed, domain.StatusReady:
		return b.Start.Sub(now) >= CancelWindow
	}
	return false
}

// DepositState is the deposit status with the lazy credit rule applied.
func (b Booking) DepositState(now time.Time) domain.DepositStatus {
	return domain.EffectiveDepositStatus(b.DepositStatus, b.DepositReleasedAt, now)
}

// Refund is what comes back from the deposit.
func (b Booking) Refund() domain.Paise { return b.DepositHeld - b.DepositDeducted }

// WhatsApp links to a chat with the shop, with a prefilled message.
func (b Booking) WhatsApp() string {
	msg := fmt.Sprintf("Hi %s, this is %s about booking %s (%s).", b.ShopName, firstName(b.RenterName), b.Code, b.VehicleName())
	return "https://wa.me/" + b.ShopWhatsApp + "?text=" + url.QueryEscape(msg)
}

// MapLink opens the shop on OpenStreetMap.
func (b Booking) MapLink() string {
	return fmt.Sprintf("https://www.openstreetmap.org/?mlat=%.5f&mlon=%.5f#map=17/%.5f/%.5f", b.ShopLat, b.ShopLng, b.ShopLat, b.ShopLng)
}

// LateBy is how late the vehicle came back (0 if on time).
func (b Booking) LateBy() time.Duration {
	if b.ReturnedAt.IsZero() || !b.ReturnedAt.After(b.End) {
		return 0
	}
	return b.ReturnedAt.Sub(b.End)
}

func firstName(name string) string {
	if i := strings.IndexByte(name, ' '); i > 0 {
		return name[:i]
	}
	return name
}

const bookingSelect = `SELECT b.id, b.code, b.renter_id, b.vehicle_id, b.shop_id, b.start_at, b.end_at, b.status,
	b.rent_paise, b.addons_paise, b.tax_paise, b.total_paise, b.deposit_paise, b.km_included, b.price_lines,
	COALESCE(b.odometer_start, 0), COALESCE(b.odometer_end, 0), COALESCE(b.returned_at, ''), b.created_at, b.updated_at,
	v.make, v.model, v.type, v.reg_number, v.seats, v.extra_km_paise, v.late_fee_per_hour_paise, v.instant_book,
	COALESCE((SELECT p.id FROM vehicle_photos p WHERE p.vehicle_id = v.id ORDER BY p.position LIMIT 1), ''),
	u.name, u.email, u.phone,
	s.name, s.address, s.phone, s.whatsapp, s.owner_id, s.lat, s.lng,
	COALESCE(d.status, ''), COALESCE(d.held_paise, 0), COALESCE(d.deducted_paise, 0), COALESCE(d.released_at, ''), COALESCE(d.credited_at, ''),
	v.fuel,
	COALESCE((SELECT GROUP_CONCAT(tp.angle) FROM trip_photos tp WHERE tp.booking_id = b.id AND tp.phase = 'checkin'), ''),
	COALESCE((SELECT GROUP_CONCAT(tp.angle) FROM trip_photos tp WHERE tp.booking_id = b.id AND tp.phase = 'return'), ''),
	EXISTS (SELECT 1 FROM reviews r WHERE r.booking_id = b.id)
FROM bookings b
JOIN vehicles v ON v.id = b.vehicle_id
JOIN users u ON u.id = b.renter_id
JOIN shops s ON s.id = b.shop_id
LEFT JOIN deposits d ON d.booking_id = b.id`

func scanBooking(row scanner) (Booking, error) {
	var b Booking
	var start, end, returned, created, updated, lines, status, depStatus, relAt, credAt, inShots, outShots string
	var instant, reviewed int
	err := row.Scan(&b.ID, &b.Code, &b.RenterID, &b.VehicleID, &b.ShopID, &start, &end, &status,
		&b.Rent, &b.Addons, &b.Tax, &b.Total, &b.Deposit, &b.KmIncluded, &lines,
		&b.OdoStart, &b.OdoEnd, &returned, &created, &updated,
		&b.VehicleMake, &b.VehicleModel, &b.VehicleType, &b.VehicleReg, &b.VehicleSeats, &b.ExtraKmRate, &b.LateFeeRate, &instant,
		&b.CoverID,
		&b.RenterName, &b.RenterEmail, &b.RenterPhone,
		&b.ShopName, &b.ShopAddress, &b.ShopPhone, &b.ShopWhatsApp, &b.ShopOwnerID, &b.ShopLat, &b.ShopLng,
		&depStatus, &b.DepositHeld, &b.DepositDeducted, &relAt, &credAt,
		&b.VehicleFuel, &inShots, &outShots, &reviewed)
	if errors.Is(err, sql.ErrNoRows) {
		return b, ErrNotFound
	}
	if err != nil {
		return b, err
	}
	b.CheckinShots, b.ReturnShots = shotSet(inShots), shotSet(outShots)
	b.Status = domain.Status(status)
	b.DepositStatus = domain.DepositStatus(depStatus)
	b.InstantBook = instant == 1
	b.Reviewed = reviewed == 1
	for _, p := range []struct {
		dst *time.Time
		src string
	}{{&b.Start, start}, {&b.End, end}, {&b.ReturnedAt, returned}, {&b.CreatedAt, created}, {&b.UpdatedAt, updated},
		{&b.DepositReleasedAt, relAt}, {&b.DepositCreditedAt, credAt}} {
		if *p.dst, err = parseTS(p.src); err != nil {
			return b, err
		}
	}
	if err := json.Unmarshal([]byte(lines), &b.Lines); err != nil {
		return b, fmt.Errorf("price lines: %w", err)
	}
	return b, nil
}

func shotSet(csv string) map[string]bool {
	m := map[string]bool{}
	for _, s := range strings.Split(csv, ",") {
		if s != "" {
			m[s] = true
		}
	}
	return m
}

func (s *Store) queryBookings(ctx context.Context, where string, args ...any) ([]Booking, error) {
	if err := s.ExpireStale(ctx); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, bookingSelect+" "+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Booking
	for rows.Next() {
		b, err := scanBooking(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) oneBooking(ctx context.Context, where string, args ...any) (Booking, error) {
	bs, err := s.queryBookings(ctx, where+" LIMIT 1", args...)
	if err != nil {
		return Booking{}, err
	}
	if len(bs) == 0 {
		return Booking{}, ErrNotFound
	}
	return bs[0], nil
}

// BookingByCode loads a booking by its SAR- code.
func (s *Store) BookingByCode(ctx context.Context, code string) (Booking, error) {
	return s.oneBooking(ctx, "WHERE b.code = ?", strings.ToUpper(code))
}

// BookingByID loads a booking by id.
func (s *Store) BookingByID(ctx context.Context, id string) (Booking, error) {
	return s.oneBooking(ctx, "WHERE b.id = ?", id)
}

// RenterBookings lists a renter's trips, soonest first.
func (s *Store) RenterBookings(ctx context.Context, renterID string) ([]Booking, error) {
	return s.queryBookings(ctx, "WHERE b.renter_id = ? ORDER BY b.start_at", renterID)
}

// ShopBookings lists a shop's bookings in a window, excluding dead ones.
func (s *Store) ShopBookings(ctx context.Context, shopID string, from, to time.Time) ([]Booking, error) {
	return s.queryBookings(ctx, `WHERE b.shop_id = ? AND b.start_at < ? AND b.end_at > ?
		AND b.status NOT IN ('cancelled','declined','expired') ORDER BY b.start_at`, shopID, ts(to), ts(from))
}

// ShopBookingsByStatus lists a shop's bookings in the given statuses.
func (s *Store) ShopBookingsByStatus(ctx context.Context, shopID string, statuses ...domain.Status) ([]Booking, error) {
	args := []any{shopID}
	for _, st := range statuses {
		args = append(args, string(st))
	}
	return s.queryBookings(ctx, "WHERE b.shop_id = ? AND b.status IN ("+placeholders(len(statuses))+") ORDER BY b.start_at", args...)
}

// AllBookings lists bookings for admin, newest first, optionally by status.
func (s *Store) AllBookings(ctx context.Context, status string, limit int) ([]Booking, error) {
	if status != "" {
		return s.queryBookings(ctx, "WHERE b.status = ? ORDER BY b.created_at DESC LIMIT ?", status, limit)
	}
	return s.queryBookings(ctx, "ORDER BY b.created_at DESC LIMIT ?", limit)
}

// NewBooking is a renter's request to book.
type NewBooking struct {
	RenterID, VehicleID string
	Start, End          time.Time
	AddonIDs            []string
}

// CreateBooking checks availability and inserts a pending_payment booking
// in one BEGIN IMMEDIATE transaction, so two renters can never get the
// same slot (spec §3). The unpaid booking holds its slot for 15 minutes.
func (s *Store) CreateBooking(ctx context.Context, nb NewBooking) (Booking, error) {
	now := s.Now()
	var code string
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		if err := expireStale(ctx, tx, now); err != nil {
			return err
		}
		v, err := scanVehicle(tx.QueryRowContext(ctx, vehicleSelect+" WHERE v.id = ?", nb.VehicleID))
		if err != nil {
			return err
		}
		if !v.Searchable() {
			return ErrNotBookable
		}
		if nb.Start.Before(now.Add(-10 * time.Minute)) {
			return ErrInPast
		}
		taken, err := busy(ctx, tx, now, []string{v.ID}, nb.Start, nb.End, false)
		if err != nil {
			return err
		}
		if len(taken) > 0 {
			return &SlotTakenError{Busy: taken[0]}
		}
		addons, err := shopAddons(ctx, tx, v.ShopID)
		if err != nil {
			return err
		}
		var chosen []domain.Addon
		for _, a := range addons {
			if contains(nb.AddonIDs, a.ID) && domain.AddonFits(a.Fits, v.Class()) {
				chosen = append(chosen, a.Addon)
			}
		}
		q, err := domain.Quote(v.Rates(), nb.Start, nb.End, chosen, domain.DefaultTaxBPS)
		if err != nil {
			return err
		}
		lines, _ := json.Marshal(q.Lines)
		for {
			code = NewBookingCode()
			var n int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM bookings WHERE code = ?`, code).Scan(&n); err != nil {
				return err
			}
			if n == 0 {
				break
			}
		}
		id := NewID()
		if _, err := tx.ExecContext(ctx, `INSERT INTO bookings (id, code, renter_id, vehicle_id, shop_id, start_at, end_at, status,
				rent_paise, addons_paise, tax_paise, total_paise, deposit_paise, km_included, price_lines, created_at, updated_at)
			VALUES (?,?,?,?,?,?,?,'pending_payment',?,?,?,?,?,?,?,?,?)`,
			id, code, nb.RenterID, v.ID, v.ShopID, ts(nb.Start), ts(nb.End),
			q.Rent, q.Addons, q.Tax, q.Total, q.Deposit, q.KmIncluded, string(lines), ts(now), ts(now)); err != nil {
			return err
		}
		return addEvent(ctx, tx, id, domain.StatusPendingPayment, "Booking started · slot held for 15 minutes", now)
	})
	if err != nil {
		return Booking{}, err
	}
	return s.BookingByCode(ctx, code)
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// move changes a booking's status if the state machine allows it.
func move(ctx context.Context, tx *sql.Tx, id string, to domain.Status, by domain.Actor, note string, now time.Time) error {
	var cur string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM bookings WHERE id = ?`, id).Scan(&cur); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if !domain.CanTransition(domain.Status(cur), to, by) {
		return ErrBadTransition
	}
	if _, err := tx.ExecContext(ctx, `UPDATE bookings SET status = ?, updated_at = ? WHERE id = ? AND status = ?`,
		string(to), ts(now), id, cur); err != nil {
		return err
	}
	return addEvent(ctx, tx, id, to, note, now)
}

// Pay runs the demo payment. success=false records a decline and keeps the
// booking pending (the slot stays held until the 15 minutes run out).
func (s *Store) Pay(ctx context.Context, renterID, code string, success bool) (Booking, error) {
	b, err := s.BookingByCode(ctx, code)
	if err != nil {
		return b, err
	}
	if b.RenterID != renterID {
		return b, ErrNotFound
	}
	now := s.Now()
	declined := false
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		if err := expireStale(ctx, tx, now); err != nil {
			return err
		}
		var cur string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM bookings WHERE id = ?`, b.ID).Scan(&cur); err != nil {
			return err
		}
		switch domain.Status(cur) {
		case domain.StatusExpired:
			return ErrExpired
		case domain.StatusPendingPayment:
		default:
			return ErrBadTransition
		}
		if !success {
			declined = true // commit the event, then report the decline
			return addEvent(ctx, tx, b.ID, domain.StatusPendingPayment, "Payment declined by the bank (demo). Nothing was charged.", now)
		}
		to, note := domain.StatusConfirmed, fmt.Sprintf("Paid %s (demo) · %s refundable deposit held", b.PayNow(), b.Deposit)
		if !b.InstantBook {
			to, note = domain.StatusAwaitingShop, note+" · waiting for the shop"
		}
		if err := move(ctx, tx, b.ID, to, domain.ActorRenter, note, now); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO deposits (booking_id, held_paise, status) VALUES (?, ?, 'held')`, b.ID, b.Deposit)
		return err
	})
	if err != nil {
		return b, err
	}
	if declined {
		return b, ErrPaymentDeclined
	}
	return s.BookingByCode(ctx, code)
}

func releaseDeposit(ctx context.Context, tx *sql.Tx, bookingID string, deducted domain.Paise, now time.Time) error {
	_, err := tx.ExecContext(ctx, `UPDATE deposits SET status = 'released', deducted_paise = ?, released_at = ?
		WHERE booking_id = ? AND status IN ('held','inspection')`, deducted, ts(now), bookingID)
	return err
}

// Cancel cancels a renter's booking; any deposit is refunded in full.
func (s *Store) Cancel(ctx context.Context, renterID, code string) (Booking, error) {
	b, err := s.BookingByCode(ctx, code)
	if err != nil {
		return b, err
	}
	now := s.Now()
	if b.RenterID != renterID {
		return b, ErrNotFound
	}
	if !b.CanCancel(now) {
		return b, ErrBadTransition
	}
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		note := "Cancelled by you · nothing was charged"
		if b.DepositStatus != "" {
			note = fmt.Sprintf("Cancelled by you · %s refunded in full", b.PayNow())
		}
		if err := move(ctx, tx, b.ID, domain.StatusCancelled, domain.ActorRenter, note, now); err != nil {
			return err
		}
		return releaseDeposit(ctx, tx, b.ID, 0, now)
	})
	if err != nil {
		return b, err
	}
	return s.BookingByCode(ctx, code)
}

// ShopAction is accept, decline or ready.
type ShopAction string

const (
	ShopAccept  ShopAction = "accept"
	ShopDecline ShopAction = "decline"
	ShopReady   ShopAction = "ready"
)

// ShopMove runs a provider action on a booking of shopID.
func (s *Store) ShopMove(ctx context.Context, shopID, bookingID string, act ShopAction) (Booking, error) {
	b, err := s.BookingByID(ctx, bookingID)
	if err != nil {
		return b, err
	}
	if b.ShopID != shopID {
		return b, ErrNotFound
	}
	now := s.Now()
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		switch act {
		case ShopAccept:
			return move(ctx, tx, b.ID, domain.StatusConfirmed, domain.ActorProvider, "The shop accepted your request", now)
		case ShopDecline:
			if err := move(ctx, tx, b.ID, domain.StatusDeclined, domain.ActorProvider,
				fmt.Sprintf("The shop can't take this booking · %s refunded in full", b.PayNow()), now); err != nil {
				return err
			}
			return releaseDeposit(ctx, tx, b.ID, 0, now)
		case ShopReady:
			return move(ctx, tx, b.ID, domain.StatusReady, domain.ActorProvider, "Ready for pickup at "+b.ShopName, now)
		}
		return ErrBadTransition
	})
	if err != nil {
		return b, err
	}
	return s.BookingByID(ctx, bookingID)
}

// StartTrip moves ready → active once the four check-in angles are in.
func (s *Store) StartTrip(ctx context.Context, renterID, code string, odometer int) (Booking, error) {
	b, err := s.BookingByCode(ctx, code)
	if err != nil {
		return b, err
	}
	if b.RenterID != renterID {
		return b, ErrNotFound
	}
	if !b.CheckedIn() {
		return b, ErrPhotosMissing
	}
	if odometer < 0 || (b.VehicleType != "cycle" && odometer == 0) {
		return b, ErrOdometer
	}
	now := s.Now()
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		if err := move(ctx, tx, b.ID, domain.StatusActive, domain.ActorRenter,
			fmt.Sprintf("Trip started · odometer %s km", domain.Grouped(int64(odometer))), now); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE bookings SET odometer_start = ? WHERE id = ?`, nullInt(odometer), b.ID)
		return err
	})
	if err != nil {
		return b, err
	}
	return s.BookingByCode(ctx, code)
}

// EndTrip moves active → returned once the four return angles are in. The
// deposit goes to inspection.
func (s *Store) EndTrip(ctx context.Context, renterID, code string, odometer int) (Booking, error) {
	b, err := s.BookingByCode(ctx, code)
	if err != nil {
		return b, err
	}
	if b.RenterID != renterID {
		return b, ErrNotFound
	}
	if !b.ReturnPhotosDone() {
		return b, ErrPhotosMissing
	}
	if b.VehicleType != "cycle" && (odometer <= 0 || odometer < b.OdoStart) {
		return b, ErrOdometer
	}
	now := s.Now()
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		if err := move(ctx, tx, b.ID, domain.StatusReturned, domain.ActorRenter,
			fmt.Sprintf("Returned · odometer %s km · the shop is inspecting", domain.Grouped(int64(odometer))), now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE bookings SET odometer_end = ?, returned_at = ? WHERE id = ?`,
			nullInt(odometer), ts(now), b.ID); err != nil {
			return err
		}
		if odometer > 0 {
			if _, err := tx.ExecContext(ctx, `UPDATE vehicles SET odometer_km = ? WHERE id = ?`, odometer, b.VehicleID); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `UPDATE deposits SET status = 'inspection' WHERE booking_id = ? AND status = 'held'`, b.ID)
		return err
	})
	if err != nil {
		return b, err
	}
	return s.BookingByCode(ctx, code)
}

// AutoDeductions works out extra-km and late fees for a returned booking.
func AutoDeductions(b Booking, odoEnd int) []domain.Deduction {
	var out []domain.Deduction
	if d := domain.ExtraKm(b.OdoStart, odoEnd, b.KmIncluded, b.ExtraKmRate); d.Amount > 0 {
		out = append(out, d)
	}
	if d := domain.LateFee(b.End, b.ReturnedAt, b.LateFeeRate); d.Amount > 0 {
		out = append(out, d)
	}
	return out
}

// Inspect closes a returned booking: records deductions (capped at the
// deposit), releases the rest and closes the trip.
func (s *Store) Inspect(ctx context.Context, shopID, bookingID string, odoEnd int, deductions []domain.Deduction) (domain.Settlement, error) {
	b, err := s.BookingByID(ctx, bookingID)
	if err != nil {
		return domain.Settlement{}, err
	}
	if b.ShopID != shopID {
		return domain.Settlement{}, ErrNotFound
	}
	settle := domain.Settle(b.DepositHeld, deductions)
	now := s.Now()
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		if err := move(ctx, tx, b.ID, domain.StatusClosed, domain.ActorProvider,
			fmt.Sprintf("Inspection done · %s refunded", settle.Refund), now); err != nil {
			return err
		}
		if odoEnd > 0 {
			if _, err := tx.ExecContext(ctx, `UPDATE bookings SET odometer_end = ? WHERE id = ?`, odoEnd, b.ID); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM deductions WHERE booking_id = ?`, b.ID); err != nil {
			return err
		}
		// Store what is actually kept: scale nothing, but trim the last
		// items so the stored rows add up to the capped total.
		left := settle.Deducted
		for _, d := range settle.Items {
			amt := d.Amount
			if amt > left {
				amt = left
			}
			left -= amt
			if _, err := tx.ExecContext(ctx, `INSERT INTO deductions (id, booking_id, kind, amount_paise, note) VALUES (?,?,?,?,?)`,
				NewID(), b.ID, string(d.Kind), amt, d.Note); err != nil {
				return err
			}
		}
		return releaseDeposit(ctx, tx, b.ID, settle.Deducted, now)
	})
	return settle, err
}

// Event is one line in a booking's timeline.
type Event struct {
	Status domain.Status
	Note   string
	At     time.Time
}

// BookingEvents lists a booking's history, oldest first.
func (s *Store) BookingEvents(ctx context.Context, bookingID string) ([]Event, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT status, note, created_at FROM booking_events WHERE booking_id = ? ORDER BY created_at, rowid`, bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		var st, at string
		if err := rows.Scan(&st, &e.Note, &at); err != nil {
			return nil, err
		}
		e.Status = domain.Status(st)
		e.At, _ = parseTS(at)
		out = append(out, e)
	}
	return out, rows.Err()
}

// Deductions lists what was kept from a booking's deposit.
func (s *Store) Deductions(ctx context.Context, bookingID string) ([]domain.Deduction, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT kind, amount_paise, note FROM deductions WHERE booking_id = ? ORDER BY rowid`, bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Deduction
	for rows.Next() {
		var d domain.Deduction
		var kind string
		if err := rows.Scan(&kind, &d.Amount, &d.Note); err != nil {
			return nil, err
		}
		d.Kind = domain.DeductionKind(kind)
		out = append(out, d)
	}
	return out, rows.Err()
}
