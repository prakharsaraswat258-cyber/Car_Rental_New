package store

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Upload limits: at most 8 MB, and the file type is checked by its magic bytes.
const (
	MaxUploadBytes = 8 << 20
	maxPixels      = 50_000_000 // refuse decompression bombs
	maxSide        = 1600       // stored photos are resized to fit
)

var (
	ErrTooBig   = errors.New("photo is over 8 MB")
	ErrNotImage = errors.New("that file isn't a JPEG or PNG photo")
)

// SaveImage validates an uploaded photo, fixes its rotation, shrinks it to
// fit 1600 px and re-encodes it as JPEG (which drops EXIF, including GPS).
// rel is the path inside the uploads folder, e.g. "trips/<id>/x.jpg".
func (s *Store) SaveImage(r io.Reader, rel string) error {
	data, err := io.ReadAll(io.LimitReader(r, MaxUploadBytes+1))
	if err != nil {
		return err
	}
	if len(data) > MaxUploadBytes {
		return ErrTooBig
	}
	out, err := ProcessImage(data)
	if err != nil {
		return err
	}
	if strings.Contains(rel, "..") {
		return fmt.Errorf("bad upload path")
	}
	full := filepath.Join(s.UploadDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, out, 0o644)
}

// RemoveUpload deletes a stored upload (best effort).
func (s *Store) RemoveUpload(rel string) {
	if rel == "" || strings.Contains(rel, "..") {
		return
	}
	os.Remove(filepath.Join(s.UploadDir, filepath.FromSlash(rel)))
}

// UploadPath is the file behind a stored relative path.
func (s *Store) UploadPath(rel string) (string, error) {
	if rel == "" || strings.Contains(rel, "..") {
		return "", ErrNotFound
	}
	return filepath.Join(s.UploadDir, filepath.FromSlash(rel)), nil
}

// ProcessImage is SaveImage without the disk: sniff, decode, orient,
// resize, encode.
func ProcessImage(data []byte) ([]byte, error) {
	var decode func(io.Reader) (image.Image, error)
	var config func(io.Reader) (image.Config, error)
	switch {
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		decode, config = jpeg.Decode, jpeg.DecodeConfig
	case bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}):
		decode, config = png.Decode, png.DecodeConfig
	default:
		return nil, ErrNotImage
	}
	cfg, err := config(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, ErrNotImage
	}
	if cfg.Width*cfg.Height > maxPixels {
		return nil, ErrTooBig
	}
	img, err := decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrNotImage
	}
	rgba := toRGBA(img)
	if bytes.HasPrefix(data, []byte{0xFF, 0xD8}) {
		rgba = orient(rgba, exifOrientation(data))
	}
	rgba = shrink(rgba, maxSide)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, rgba, &jpeg.Options{Quality: 82}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func toRGBA(img image.Image) *image.RGBA {
	if r, ok := img.(*image.RGBA); ok && r.Rect.Min == (image.Point{}) {
		return r
	}
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
	return out
}

// exifOrientation reads tag 0x0112 from a JPEG's APP1 Exif block (1 if absent).
func exifOrientation(data []byte) int {
	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			return 1
		}
		marker := data[i+1]
		if marker == 0xD9 || marker == 0xDA { // end / start of scan
			return 1
		}
		size := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		if size < 2 || i+2+size > len(data) {
			return 1
		}
		seg := data[i+4 : i+2+size]
		if marker == 0xE1 && len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
			return tiffOrientation(seg[6:])
		}
		i += 2 + size
	}
	return 1
}

func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	off := int(bo.Uint32(t[4:8]))
	if off+2 > len(t) {
		return 1
	}
	n := int(bo.Uint16(t[off : off+2]))
	for k := 0; k < n; k++ {
		e := off + 2 + k*12
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:e+2]) == 0x0112 {
			v := int(bo.Uint16(t[e+8 : e+10]))
			if v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// orient applies an EXIF orientation so the stored photo is upright.
func orient(src *image.RGBA, o int) *image.RGBA {
	if o <= 1 || o > 8 {
		return src
	}
	w, h := src.Rect.Dx(), src.Rect.Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch o {
			case 2:
				dx, dy = w-1-x, y
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dx, dy = x, h-1-y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			si := src.PixOffset(x, y)
			di := dst.PixOffset(dx, dy)
			copy(dst.Pix[di:di+4], src.Pix[si:si+4])
		}
	}
	return dst
}

// shrink box-filters src down so its longer side is at most max.
func shrink(src *image.RGBA, max int) *image.RGBA {
	w, h := src.Rect.Dx(), src.Rect.Dy()
	if w <= max && h <= max {
		return src
	}
	dw, dh := max, h*max/w
	if h > w {
		dw, dh = w*max/h, max
	}
	if dw < 1 {
		dw = 1
	}
	if dh < 1 {
		dh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		y0, y1 := y*h/dh, (y+1)*h/dh
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < dw; x++ {
			x0, x1 := x*w/dw, (x+1)*w/dw
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var r, g, b, a, n uint64
			for sy := y0; sy < y1; sy++ {
				i := src.PixOffset(x0, sy)
				for sx := x0; sx < x1; sx++ {
					r += uint64(src.Pix[i])
					g += uint64(src.Pix[i+1])
					b += uint64(src.Pix[i+2])
					a += uint64(src.Pix[i+3])
					n++
					i += 4
				}
			}
			di := dst.PixOffset(x, y)
			dst.Pix[di], dst.Pix[di+1], dst.Pix[di+2], dst.Pix[di+3] = uint8(r/n), uint8(g/n), uint8(b/n), uint8(a/n)
		}
	}
	return dst
}
