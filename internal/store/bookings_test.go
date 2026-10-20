package store

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"saarthi/internal/domain"
)

func vehicleByReg(t *testing.T, s *Store, reg string) Vehicle {
	t.Helper()
	var id string
	if err := s.DB.QueryRow(`SELECT id FROM vehicles WHERE reg_number = ?`, reg).Scan(&id); err != nil {
		t.Fatalf("vehicle %s: %v", reg, err)
	}
	v, err := s.VehicleByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func userID(t *testing.T, s *Store, email string) string {
	t.Helper()
	u, _, err := s.UserByEmail(context.Background(), email)
	if err != nil {
		t.Fatal(err)
	}
	return u.ID
}

func day(n, h int) time.Time {
	return time.Date(2026, 10, 20+n, h, 0, 0, 0, domain.IST)
}

func TestNoDoubleBooking(t *testing.T) {
	s := seeded(t)
	ctx := context.Background()
	v := vehicleByReg(t, s, "GA 07 K 4410") // Goa Wheels Activa, nothing seeded on it
	renter := userID(t, s, DemoRenterEmail)
	book := func(start, end time.Time) (Booking, error) {
		return s.CreateBooking(ctx, NewBooking{RenterID: renter, VehicleID: v.ID, Start: start, End: end})
	}

	first, err := book(day(10, 9), day(12, 9))
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != domain.StatusPendingPayment || first.Code[:4] != "SAR-" {
		t.Fatalf("first = %s %s", first.Status, first.Code)
	}
	if first.Total != first.Rent+first.Addons+first.Tax {
		t.Errorf("stored bill does not add up")
	}

	tests := []struct {
		name       string
		start, end time.Time
		taken      bool
	}{
		{"same dates", day(10, 9), day(12, 9), true},
		{"starts inside", day(11, 9), day(13, 9), true},
		{"wraps around", day(9, 9), day(13, 9), true},
		{"inside", day(10, 12), day(10, 18), true},
		{"touching after is fine", day(12, 9), day(13, 9), false},
		{"touching before is fine", day(9, 9), day(10, 9), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := book(tt.start, tt.end)
			if tt.taken {
				var st *SlotTakenError
				if !errors.As(err, &st) || !errors.Is(err, ErrSlotTaken) {
					t.Fatalf("err = %v, want SlotTakenError", err)
				}
				if st.Busy.Label != first.Code {
					t.Errorf("conflict label = %q, want %q", st.Busy.Label, first.Code)
				}
			} else if err != nil {
				t.Fatalf("err = %v, want ok", err)
			}
		})
	}

	// An unpaid booking holds the slot for 15 minutes, then frees it.
	s.Now = func() time.Time { return seedNow.Add(14 * time.Minute) }
	if _, err := book(day(10, 9), day(12, 9)); !errors.Is(err, ErrSlotTaken) {
		t.Fatalf("at 14 min: err = %v, want taken", err)
	}
	s.Now = func() time.Time { return seedNow.Add(16 * time.Minute) }
	again, err := book(day(10, 9), day(12, 9))
	if err != nil {
		t.Fatalf("at 16 min: %v", err)
	}
	old, _ := s.BookingByCode(ctx, first.Code)
	if old.Status != domain.StatusExpired {
		t.Errorf("first booking = %s, want expired", old.Status)
	}
	if again.Code == first.Code {
		t.Error("expected a new booking")
	}
}

func TestBlocksAndBookingsCollide(t *testing.T) {
	s := seeded(t)
	ctx := context.Background()
	v := vehicleByReg(t, s, "GA 07 K 2291") // Ntorq, free
	shop := v.ShopID
	renter := userID(t, s, DemoRenterEmail)

	if _, err := s.CreateBlock(ctx, shop, v.ID, day(20, 0), day(22, 0), "service"); err != nil {
		t.Fatal(err)
	}
	_, err := s.CreateBooking(ctx, NewBooking{RenterID: renter, VehicleID: v.ID, Start: day(21, 9), End: day(23, 9)})
	var st *SlotTakenError
	if !errors.As(err, &st) || st.Busy.Kind != "block" {
		t.Fatalf("booking over a block: %v", err)
	}

	b, err := s.CreateBooking(ctx, NewBooking{RenterID: renter, VehicleID: v.ID, Start: day(25, 9), End: day(26, 9)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateBlock(ctx, shop, v.ID, day(25, 0), day(27, 0), ""); !errors.Is(err, ErrSlotTaken) {
		t.Errorf("block over a booking: err = %v, want taken", err)
	}
	// another shop cannot block this vehicle
	other := vehicleByReg(t, s, "RJ 38 SA 4412").ShopID
	if _, err := s.CreateBlock(ctx, other, v.ID, day(28, 0), day(29, 0), ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("foreign shop block: err = %v, want not found", err)
	}
	_ = b
}

func TestPayFlow(t *testing.T) {
	s := seeded(t)
	ctx := context.Background()
	renter := userID(t, s, DemoRenterEmail)
	instant := vehicleByReg(t, s, "GA 07 F 6631")  // Goa Swift
	request := vehicleByReg(t, s, "RJ 38 CB 0099") // Thar, instant book off

	b, err := s.CreateBooking(ctx, NewBooking{RenterID: renter, VehicleID: instant.ID, Start: day(12, 9), End: day(13, 9)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Pay(ctx, renter, b.Code, false); !errors.Is(err, ErrPaymentDeclined) {
		t.Fatalf("fail button: err = %v", err)
	}
	b, _ = s.BookingByCode(ctx, b.Code)
	if b.Status != domain.StatusPendingPayment || b.DepositStatus != "" {
		t.Fatalf("after decline: %s deposit %q", b.Status, b.DepositStatus)
	}
	if other := userID(t, s, "renter2@saarthi.test"); true {
		if _, err := s.Pay(ctx, other, b.Code, true); !errors.Is(err, ErrNotFound) {
			t.Errorf("someone else paying: err = %v", err)
		}
	}
	b, err = s.Pay(ctx, renter, b.Code, true)
	if err != nil {
		t.Fatal(err)
	}
	if b.Status != domain.StatusConfirmed || b.DepositStatus != domain.DepositHeld || b.DepositHeld != instant.Deposit {
		t.Errorf("after pay: %s deposit %s %s", b.Status, b.DepositStatus, b.DepositHeld)
	}
	if _, err := s.Pay(ctx, renter, b.Code, true); !errors.Is(err, ErrBadTransition) {
		t.Errorf("paying twice: err = %v", err)
	}

	r, err := s.CreateBooking(ctx, NewBooking{RenterID: renter, VehicleID: request.ID, Start: day(14, 9), End: day(15, 9)})
	if err != nil {
		t.Fatal(err)
	}
	r, err = s.Pay(ctx, renter, r.Code, true)
	if err != nil || r.Status != domain.StatusAwaitingShop {
		t.Fatalf("request booking: %s %v", r.Status, err)
	}
	r, err = s.ShopMove(ctx, request.ShopID, r.ID, ShopAccept)
	if err != nil || r.Status != domain.StatusConfirmed {
		t.Fatalf("accept: %s %v", r.Status, err)
	}

	late, err := s.CreateBooking(ctx, NewBooking{RenterID: renter, VehicleID: instant.ID, Start: day(16, 9), End: day(17, 9)})
	if err != nil {
		t.Fatal(err)
	}
	s.Now = func() time.Time { return seedNow.Add(20 * time.Minute) }
	if _, err := s.Pay(ctx, renter, late.Code, true); !errors.Is(err, ErrExpired) {
		t.Errorf("paying after 15 min: err = %v", err)
	}
}

func TestTripLifecycle(t *testing.T) {
	s := seeded(t)
	ctx := context.Background()
	renter := userID(t, s, DemoRenterEmail)
	v := vehicleByReg(t, s, "GA 03 R 1450") // Himalayan, ₹5,000 deposit

	b, err := s.CreateBooking(ctx, NewBooking{RenterID: renter, VehicleID: v.ID, Start: day(1, 9), End: day(2, 9)})
	if err != nil {
		t.Fatal(err)
	}
	if b, err = s.Pay(ctx, renter, b.Code, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ShopMove(ctx, v.ShopID, b.ID, ShopReady); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartTrip(ctx, renter, b.Code, 7300); !errors.Is(err, ErrPhotosMissing) {
		t.Fatalf("start without photos: %v", err)
	}
	for _, a := range []string{"front", "back", "left", "right"} {
		if _, _, err := s.SaveTripPhoto(ctx, b.ID, "checkin", a, "x.jpg"); err != nil {
			t.Fatal(err)
		}
	}
	if b, err = s.StartTrip(ctx, renter, b.Code, 7300); err != nil || b.Status != domain.StatusActive {
		t.Fatalf("start: %v %s", err, b.Status)
	}
	for _, a := range []string{"front", "back", "left", "right"} {
		s.SaveTripPhoto(ctx, b.ID, "return", a, "y.jpg")
	}
	if _, err := s.EndTrip(ctx, renter, b.Code, 7000); !errors.Is(err, ErrOdometer) {
		t.Errorf("odometer going backwards: %v", err)
	}
	s.Now = func() time.Time { return day(2, 11) } // 2 h late
	if b, err = s.EndTrip(ctx, renter, b.Code, 7600); err != nil || b.Status != domain.StatusReturned {
		t.Fatalf("end: %v %s", err, b.Status)
	}
	if b.DepositStatus != domain.DepositInspection {
		t.Errorf("deposit = %s, want inspection", b.DepositStatus)
	}

	auto := AutoDeductions(b, 7600) // 300 km, 200 included → 100 × ₹6; late 2 h × ₹200
	var total domain.Paise
	for _, d := range auto {
		total += d.Amount
	}
	if total != domain.Rupees(600+400) {
		t.Errorf("auto deductions = %s, want ₹1,000 (%+v)", total, auto)
	}
	deds := append(auto, domain.Deduction{Kind: domain.DeductDamage, Amount: domain.Rupees(4500), Note: "Bent footpeg"})
	settle, err := s.Inspect(ctx, v.ShopID, b.ID, 7600, deds)
	if err != nil {
		t.Fatal(err)
	}
	if !settle.Capped || settle.Deducted != domain.Rupees(5000) || settle.Refund != 0 {
		t.Errorf("settlement = %+v", settle)
	}
	stored, _ := s.Deductions(ctx, b.ID)
	var sum domain.Paise
	for _, d := range stored {
		sum += d.Amount
	}
	if sum != domain.Rupees(5000) {
		t.Errorf("stored deductions add to %s, want the capped ₹5,000", sum)
	}
	b, _ = s.BookingByCode(ctx, b.Code)
	if b.Status != domain.StatusClosed || b.DepositStatus != domain.DepositReleased {
		t.Errorf("after inspection: %s %s", b.Status, b.DepositStatus)
	}

	s.Now = func() time.Time { return day(2, 11).Add(2 * time.Minute) }
	b, _ = s.BookingByCode(ctx, b.Code)
	if b.DepositStatus != domain.DepositCredited {
		t.Errorf("a minute after release: %s, want credited", b.DepositStatus)
	}

	if err := s.AddReview(ctx, renter, b.Code, 4, "Good bike"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddReview(ctx, renter, b.Code, 5, "again"); !errors.Is(err, ErrBadTransition) {
		t.Errorf("second review: %v", err)
	}
	after, _ := s.VehicleByID(ctx, v.ID)
	if after.RatingCount != v.RatingCount+1 {
		t.Errorf("rating count %d → %d", v.RatingCount, after.RatingCount)
	}
}

func TestCancelRefunds(t *testing.T) {
	s := seeded(t)
	ctx := context.Background()
	renter := userID(t, s, DemoRenterEmail)
	v := vehicleByReg(t, s, "GA 07 E 1170") // Ola S1
	b, _ := s.CreateBooking(ctx, NewBooking{RenterID: renter, VehicleID: v.ID, Start: day(5, 9), End: day(6, 9)})
	s.Pay(ctx, renter, b.Code, true)
	b, err := s.Cancel(ctx, renter, b.Code)
	if err != nil || b.Status != domain.StatusCancelled || b.DepositStatus != domain.DepositReleased || b.DepositDeducted != 0 {
		t.Fatalf("cancel: %v %s %s", err, b.Status, b.DepositStatus)
	}
	// inside 24 h free cancellation is over
	c, _ := s.CreateBooking(ctx, NewBooking{RenterID: renter, VehicleID: v.ID, Start: seedNow.Add(5 * time.Hour), End: seedNow.Add(9 * time.Hour)})
	s.Pay(ctx, renter, c.Code, true)
	if _, err := s.Cancel(ctx, renter, c.Code); !errors.Is(err, ErrBadTransition) {
		t.Errorf("late cancel: %v", err)
	}
}

func TestProcessImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3200, 1000))
	for x := 0; x < 3200; x++ {
		img.Set(x, 10, color.RGBA{200, 80, 40, 255})
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	out, err := ProcessImage(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(out, []byte{0xFF, 0xD8, 0xFF}) {
		t.Fatal("output is not a JPEG")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil || cfg.Width != 1600 || cfg.Height != 500 {
		t.Errorf("resized to %dx%d (%v), want 1600x500", cfg.Width, cfg.Height, err)
	}
	if _, err := ProcessImage([]byte("GIF89a not a photo")); !errors.Is(err, ErrNotImage) {
		t.Errorf("text file: %v", err)
	}
}

func TestExifOrientation(t *testing.T) {
	// SOI, APP1 Exif with a big-endian TIFF holding Orientation = 6.
	tiff := []byte{'M', 'M', 0, 42, 0, 0, 0, 8, 0, 1, 0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, 6, 0, 0, 0, 0, 0, 0}
	seg := append([]byte("Exif\x00\x00"), tiff...)
	data := []byte{0xFF, 0xD8, 0xFF, 0xE1, byte((len(seg) + 2) >> 8), byte(len(seg) + 2)}
	data = append(data, seg...)
	data = append(data, 0xFF, 0xD9)
	if got := exifOrientation(data); got != 6 {
		t.Errorf("orientation = %d, want 6", got)
	}
	src := image.NewRGBA(image.Rect(0, 0, 4, 2))
	src.Set(0, 0, color.RGBA{255, 0, 0, 255})
	r := orient(src, 6) // 90° clockwise: top-left goes to top-right
	if r.Rect.Dx() != 2 || r.Rect.Dy() != 4 || r.RGBAAt(1, 0).R != 255 {
		t.Errorf("rotate 6: size %v, (1,0)=%v", r.Rect, r.RGBAAt(1, 0))
	}
}
