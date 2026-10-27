package handlers

import (
	"fmt"
	"math"
	"strings"

	"saarthi/internal/domain"
	"saarthi/internal/store"
)

// Chart is a server-rendered single-series line chart (inline SVG, no
// library). One series per chart, so no legend; the title names it.
type Chart struct {
	ID, Title, Sub string
	W, H           float64
	Left, Right    float64
	Top, Bottom    float64
	PlotRight      float64 // x where the plot ends
	PlotH          float64
	BaseY          float64 // y of the baseline
	TickX, LabelY  float64 // y-tick text x, x-tick text y
	Line, Area     string
	Points         []ChartPoint
	YTicks, XTicks []ChartTick
	Last           ChartPoint
	Total          string
}

// ChartPoint is one day.
type ChartPoint struct {
	X, Y       float64
	HitX, HitW float64 // hover band, wider than the mark
	Day, Value string
}

// ChartTick is an axis label.
type ChartTick struct {
	Pos   float64
	Label string
}

func niceMax(v float64) float64 {
	if v <= 0 {
		return 1
	}
	exp := math.Pow(10, math.Floor(math.Log10(v)))
	for _, m := range []float64{1, 2, 2.5, 5, 10} {
		if v <= m*exp {
			return m * exp
		}
	}
	return 10 * exp
}

// compact renders 1,200 as 1.2K and 1,50,000 as 1.5L (Indian lakh).
func compact(v float64, rupee bool) string {
	p := ""
	if rupee {
		p = "₹"
	}
	switch {
	case v >= 100000:
		return p + strings.TrimSuffix(fmt.Sprintf("%.1f", v/100000), ".0") + "L"
	case v >= 1000:
		return p + strings.TrimSuffix(fmt.Sprintf("%.1f", v/1000), ".0") + "K"
	}
	return p + fmt.Sprintf("%.0f", v)
}

func lineChart(id, title string, series []store.DayPoint, value func(store.DayPoint) float64, label func(float64) string, rupee bool) Chart {
	c := Chart{ID: id, Title: title, W: 640, H: 220, Left: 48, Right: 20, Top: 16, Bottom: 30}
	if len(series) == 0 {
		return c
	}
	pw, ph := c.W-c.Left-c.Right, c.H-c.Top-c.Bottom
	c.PlotRight, c.PlotH, c.BaseY, c.TickX, c.LabelY = c.W-c.Right, ph, c.Top+ph, c.Left-8, c.H-8
	peak, total := 0.0, 0.0
	for _, p := range series {
		v := value(p)
		total += v
		peak = math.Max(peak, v)
	}
	top := niceMax(peak)
	n := float64(len(series) - 1)
	if n == 0 {
		n = 1
	}
	var line, area strings.Builder
	for i, p := range series {
		v := value(p)
		x := c.Left + float64(i)/n*pw
		y := c.Top + ph - v/top*ph
		band := pw / n
		pt := ChartPoint{X: x, Y: y, HitX: x - band/2, HitW: band, Day: domain.DayText(p.Day), Value: label(v)}
		c.Points = append(c.Points, pt)
		if i == 0 {
			fmt.Fprintf(&line, "M%.1f %.1f", x, y)
			fmt.Fprintf(&area, "M%.1f %.1f L%.1f %.1f", x, c.Top+ph, x, y)
		} else {
			fmt.Fprintf(&line, " L%.1f %.1f", x, y)
			fmt.Fprintf(&area, " L%.1f %.1f", x, y)
		}
		if i%7 == 0 || i == len(series)-1 {
			c.XTicks = append(c.XTicks, ChartTick{Pos: x, Label: p.Day.In(domain.IST).Format("2 Jan")})
		}
	}
	fmt.Fprintf(&area, " L%.1f %.1f Z", c.Points[len(c.Points)-1].X, c.Top+ph)
	c.Line, c.Area = line.String(), area.String()
	for _, f := range []float64{0, 0.5, 1} {
		c.YTicks = append(c.YTicks, ChartTick{Pos: c.Top + ph - f*ph, Label: compact(top*f, rupee)})
	}
	c.Last = c.Points[len(c.Points)-1]
	c.Total = label(total)
	return c
}

func adminCharts(series []store.DayPoint) []Chart {
	bookings := lineChart("chart-bookings", "Bookings per day", series,
		func(p store.DayPoint) float64 { return float64(p.Bookings) },
		func(v float64) string {
			if v == 1 {
				return "1 booking"
			}
			return fmt.Sprintf("%.0f bookings", v)
		}, false)
	bookings.Sub = "Last 30 days · " + bookings.Total
	revenue := lineChart("chart-revenue", "Revenue per day", series,
		func(p store.DayPoint) float64 { return float64(p.Revenue) / 100 },
		func(v float64) string { return domain.Paise(int64(v*100 + 0.5)).String() }, true)
	revenue.Sub = "Last 30 days · " + revenue.Total + " · rent, add-ons and GST"
	return []Chart{bookings, revenue}
}
