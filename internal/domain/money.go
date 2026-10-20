// Package domain holds Saarthi's pure business rules: money, pricing,
// booking status transitions and deposit settlement. Nothing here touches
// the database or HTTP, so every rule is unit-testable.
package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// Paise is an amount of Indian money in paise (100 paise = 1 rupee).
// Money is never a float anywhere in Saarthi.
type Paise int64

// Rupees converts whole rupees to Paise.
func Rupees(r int64) Paise { return Paise(r * 100) }

// String formats with the rupee sign and Indian digit grouping:
// ₹450, ₹1,500, ₹1,23,456, ₹141.60. Paise appear only when non-zero.
func (p Paise) String() string {
	if p < 0 {
		return "−₹" + (-p).Plain()
	}
	return "₹" + p.Plain()
}

// Plain is String without the rupee sign: "1,23,456" or "141.60".
func (p Paise) Plain() string {
	if p < 0 {
		return "−" + (-p).Plain()
	}
	s := groupIndian(int64(p) / 100)
	if rem := int64(p) % 100; rem != 0 {
		s += fmt.Sprintf(".%02d", rem)
	}
	return s
}

// WholeRupees returns the amount rounded half-up to whole rupees.
func (p Paise) WholeRupees() int64 {
	if p < 0 {
		return -(-p).WholeRupees()
	}
	return (int64(p) + 50) / 100
}

// Grouped formats a plain number with Indian grouping: 21450 → "21,450".
func Grouped(n int64) string {
	if n < 0 {
		return "−" + groupIndian(-n)
	}
	return groupIndian(n)
}

// groupIndian groups digits the Indian way: last three, then pairs.
func groupIndian(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	head, tail := s[:len(s)-3], s[len(s)-3:]
	var parts []string
	for len(head) > 2 {
		parts = append(parts, head[len(head)-2:])
		head = head[:len(head)-2]
	}
	if head != "" {
		parts = append(parts, head)
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, ",") + "," + tail
}
