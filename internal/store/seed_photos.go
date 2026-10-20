package store

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"os"
	"path/filepath"
	"time"
)

// TripAngles are the proof-photo angles, in checklist order.
var TripAngles = []string{"front", "back", "left", "right", "odometer"}

// tripPhotos writes generated placeholder JPEGs for one phase of a seeded
// booking and records them (D-026).
func (sd *seeder) tripPhotos(bookingID, phase string, b seedBooking, v seedVehicle) {
	if sd.err != nil {
		return
	}
	base := b.start.Add(-10 * time.Minute)
	if phase == "return" {
		base = b.returned.Add(-6 * time.Minute)
	}
	body := bodyColours[len(v.key)%len(bodyColours)]
	for i, angle := range TripAngles {
		rel := filepath.ToSlash(filepath.Join("trips", bookingID, phase+"-"+angle+".jpg"))
		if err := writePlaceholderPhoto(filepath.Join(sd.uploads, rel), angle, body); err != nil {
			sd.err = fmt.Errorf("seed photo: %w", err)
			return
		}
		sd.exec(`INSERT INTO trip_photos (id, booking_id, phase, angle, path, taken_at) VALUES (?,?,?,?,?,?)`,
			NewID(), bookingID, phase, angle, rel, ts(sd.past(base.Add(time.Duration(i)*40*time.Second))))
	}
}

var bodyColours = []color.RGBA{
	{0x2E, 0x6B, 0x66, 0xFF}, // highway teal
	{0xC2, 0x55, 0x2F, 0xFF}, // saffron clay
	{0x3A, 0x38, 0x33, 0xFF}, // graphite
	{0xB8, 0xB2, 0xA4, 0xFF}, // silver
}

// writePlaceholderPhoto paints a soft, abstract "vehicle on a road" scene
// (or a dial for the odometer) and saves it as a JPEG with no metadata.
func writePlaceholderPhoto(path, angle string, body color.RGBA) error {
	const w, h = 640, 480
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	if angle == "odometer" {
		paintDial(img)
	} else {
		paintVehicle(img, angle == "left" || angle == "right", body)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 72}); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

type rgb [3]float64

func mix(a, b rgb, t float64) rgb {
	return rgb{a[0] + (b[0]-a[0])*t, a[1] + (b[1]-a[1])*t, a[2] + (b[2]-a[2])*t}
}

func (c rgb) scale(k float64) rgb { return rgb{c[0] * k, c[1] * k, c[2] * k} }

func (c rgb) rgba() color.RGBA {
	cl := func(v float64) uint8 { return uint8(math.Max(0, math.Min(255, v))) }
	return color.RGBA{cl(c[0]), cl(c[1]), cl(c[2]), 0xFF}
}

func fromRGBA(c color.RGBA) rgb { return rgb{float64(c.R), float64(c.G), float64(c.B)} }

func paintVehicle(img *image.RGBA, side bool, bodyCol color.RGBA) {
	b := img.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	sky0, sky1 := rgb{0xF1, 0xEC, 0xE2}, rgb{0xDC, 0xD3, 0xC2}
	ground0, ground1 := rgb{0x8E, 0x88, 0x7B}, rgb{0x66, 0x61, 0x57}
	body := fromRGBA(bodyCol)
	wheel := rgb{0x1F, 0x1D, 0x1A}

	horizon := h * 0.62
	cx, cy := w/2, h*0.56
	rx, ry := w*0.31, h*0.12
	wheelR := h * 0.075
	wheels := [][2]float64{{cx - rx*0.62, cy + ry*0.95}, {cx + rx*0.62, cy + ry*0.95}}
	if !side {
		rx, ry = w*0.17, h*0.15
		wheels = [][2]float64{{cx, cy + ry*1.0}}
	}

	for y := 0; y < b.Dy(); y++ {
		fy := float64(y)
		for x := 0; x < b.Dx(); x++ {
			fx := float64(x)
			var c rgb
			if fy < horizon {
				c = mix(sky0, sky1, fy/horizon)
			} else {
				c = mix(ground0, ground1, (fy-horizon)/(h-horizon))
				// dashed road line
				if math.Abs(fy-h*0.86) < 3 && int(fx/48)%2 == 0 {
					c = rgb{0xE8, 0xE2, 0xD3}
				}
			}
			// shadow
			if dx, dy := (fx-cx)/(rx*1.1), (fy-(cy+ry*1.55))/(ry*0.35); dx*dx+dy*dy < 1 {
				c = c.scale(0.78)
			}
			// body, lit from the top
			if dx, dy := (fx-cx)/rx, (fy-cy)/ry; dx*dx+dy*dy < 1 {
				c = body.scale(1.08 - 0.22*(dy+1)/2)
			}
			// wheels with a lighter hub
			for _, wp := range wheels {
				d := math.Hypot(fx-wp[0], fy-wp[1])
				if d < wheelR {
					c = wheel
					if d < wheelR*0.4 {
						c = rgb{0x6B, 0x67, 0x5F}
					}
				}
			}
			// vignette
			vx, vy := (fx-w/2)/(w/2), (fy-h/2)/(h/2)
			c = c.scale(1 - 0.18*(vx*vx+vy*vy))
			img.SetRGBA(x, y, c.rgba())
		}
	}
}

func paintDial(img *image.RGBA) {
	b := img.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	cx, cy, r := w/2, h/2, h*0.32
	bg0, bg1 := rgb{0x2A, 0x28, 0x24}, rgb{0x17, 0x16, 0x14}
	ring := rgb{0xF2, 0xEF, 0xE7}
	accent := rgb{0xE2, 0x77, 0x4F}
	for y := 0; y < b.Dy(); y++ {
		fy := float64(y)
		for x := 0; x < b.Dx(); x++ {
			fx := float64(x)
			d := math.Hypot(fx-cx, fy-cy)
			c := mix(bg0, bg1, math.Min(1, d/(w*0.6)))
			switch {
			case math.Abs(d-r) < 3:
				c = ring
			case d < r && math.Abs(fy-(cy+r*0.45)) < 14 && math.Abs(fx-cx) < r*0.55:
				c = rgb{0x0E, 0x0D, 0x0C} // odometer window
			}
			// needle from centre towards upper right
			ang := math.Atan2(cy-fy, fx-cx)
			if d < r*0.85 && math.Abs(ang-0.7) < 0.03 {
				c = accent
			}
			img.SetRGBA(x, y, c.rgba())
		}
	}
}
