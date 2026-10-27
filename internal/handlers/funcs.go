package handlers

import (
	"crypto/sha256"
	"fmt"
	"html/template"
	"strings"
	"sync/atomic"

	"saarthi/internal/domain"
)

// statusIcon pairs every status with an icon, so chips never rely on
// colour alone (for colour-blind users).
func statusIcon(s any) string {
	switch fmt.Sprint(s) {
	case string(domain.StatusPendingPayment), string(domain.StatusAwaitingShop), "pending":
		return "hourglass"
	case string(domain.StatusConfirmed), "live":
		return "circle-check"
	case string(domain.StatusReady):
		return "key-round"
	case string(domain.StatusActive):
		return "route"
	case string(domain.StatusReturned):
		return "rotate-ccw"
	case string(domain.StatusClosed), "verified":
		return "badge-check"
	case string(domain.StatusCancelled), "suspended":
		return "ban"
	case string(domain.StatusDeclined):
		return "circle-x"
	case string(domain.StatusExpired):
		return "timer"
	case "hidden":
		return "eye-off"
	case "draft":
		return "pencil"
	}
	return "circle-dot"
}

var uidSeq atomic.Uint64

// uid gives each popover / control a unique id on the page.
func uid(prefix string) string { return fmt.Sprintf("%s-%d", prefix, uidSeq.Add(1)) }

// qrPattern draws a decorative, QR-style 25×25 pattern from a booking code
// (not a scannable QR code; the code itself is printed beside it).
func qrPattern(code string) template.HTML {
	const n = 25
	sum := sha256.Sum256([]byte("saarthi:" + code))
	bits := make([]bool, 0, n*n)
	seed := sum[:]
	for len(bits) < n*n {
		for _, b := range seed {
			for i := 0; i < 8; i++ {
				bits = append(bits, b&(1<<i) != 0)
			}
		}
		next := sha256.Sum256(seed)
		seed = next[:]
	}
	finder := func(x, y int) (in, on bool) {
		for _, c := range [][2]int{{0, 0}, {n - 7, 0}, {0, n - 7}} {
			dx, dy := x-c[0], y-c[1]
			if dx >= -1 && dx <= 7 && dy >= -1 && dy <= 7 {
				in = true
				if dx >= 0 && dx < 7 && dy >= 0 && dy < 7 {
					ring := dx == 0 || dx == 6 || dy == 0 || dy == 6
					core := dx >= 2 && dx <= 4 && dy >= 2 && dy <= 4
					on = ring || core
				}
				return
			}
		}
		return false, false
	}
	var d strings.Builder
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			in, on := finder(x, y)
			if (in && on) || (!in && bits[y*n+x]) {
				fmt.Fprintf(&d, "M%d %dh1v1h-1z", x, y)
			}
		}
	}
	return template.HTML(`<svg class="qr" viewBox="-1 -1 27 27" aria-hidden="true" focusable="false" shape-rendering="crispEdges"><path d="` + d.String() + `" fill="currentColor"/></svg>`)
}
