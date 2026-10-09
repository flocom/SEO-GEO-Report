package render

import (
	"fmt"
	"html/template"
	"strings"
)

// Balanced grids.
//
// Every grid of tiles (KPI cards, mini statistics, hero figures...) is laid
// out server-side so that it never leaves a hole or a lone card on its last
// row: n items are split into rows of at most max columns, as evenly as
// possible (larger rows first: 7 items in 3 columns give 3 + 2 + 2, never
// 3 + 3 + 1). The CSS grid then uses as many tracks as the least common
// multiple of the row sizes, and each item spans its share of a row.
//
// One layout is computed per breakpoint (desktop, tablet, phone, print) and
// handed to the stylesheet through CSS custom properties, so the same markup
// stays balanced at every width without JavaScript.

// balancedRows splits n items into rows of at most max items.
func balancedRows(n, max int) []int {
	if n <= 0 {
		return nil
	}
	if max < 1 {
		max = 1
	}
	for {
		rows := (n + max - 1) / max
		base, extra := n/rows, n%rows
		// A lone item next to fuller rows (e.g. 3 items in 2 columns: 2 + 1)
		// is avoided by using fewer columns: 3 rows of 1.
		if base == 1 && extra > 0 && max > 1 {
			max--
			continue
		}
		out := make([]int, rows)
		for i := range out {
			out[i] = base
			if i < extra {
				out[i]++
			}
		}
		return out
	}
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// gridLayout returns the number of grid tracks and the span of each item for
// n items in rows of at most max items.
func gridLayout(n, max int) (int, []int) {
	rows := balancedRows(n, max)
	if len(rows) == 0 {
		return 1, nil
	}
	tracks := 1
	for _, r := range rows {
		tracks = tracks / gcd(tracks, r) * r
	}
	spans := make([]int, 0, n)
	for _, r := range rows {
		for i := 0; i < r; i++ {
			spans = append(spans, tracks/r)
		}
	}
	return tracks, spans
}

// balGrid is the layout of one grid at the four breakpoints.
type balGrid struct {
	tracks [4]int
	spans  [4][]int
}

// Breakpoint order in balGrid.
const (
	bpDesk = iota
	bpTab
	bpPhone
	bpPrint
)

var bpVars = [4]string{"d", "t", "m", "p"}

// newBalGrid lays out n items with at most desk / tab / phone / print
// columns.
func newBalGrid(n, desk, tab, phone, print int) balGrid {
	var g balGrid
	for i, max := range [4]int{desk, tab, phone, print} {
		g.tracks[i], g.spans[i] = gridLayout(n, max)
	}
	return g
}

// Style is the container style: the number of tracks per breakpoint.
func (g balGrid) Style() template.CSS {
	var b strings.Builder
	for i, v := range bpVars {
		fmt.Fprintf(&b, "--b%s:%d;", v, g.tracks[i])
	}
	return template.CSS(b.String()) //nolint:gosec // integers only
}

// At is the style of item i: its span per breakpoint.
func (g balGrid) At(i int) template.CSS {
	var b strings.Builder
	for k, v := range bpVars {
		s := 1
		if i >= 0 && i < len(g.spans[k]) {
			s = g.spans[k][i]
		}
		fmt.Fprintf(&b, "--s%s:%d;", v, s)
	}
	return template.CSS(b.String()) //nolint:gosec // integers only
}

// Grid presets: maximum columns per breakpoint for each kind of grid.
var gridPresets = map[string][4]int{
	"kpi-hero":    {6, 3, 2, 3}, // Part 1 key indicators
	"kpi":         {5, 3, 2, 4}, // section KPI tiles
	"kpi-compact": {6, 3, 2, 4}, // small metric tiles
	"stats":       {6, 4, 2, 5}, // mini statistics in a full-width card
	"stats-half":  {4, 3, 2, 4}, // mini statistics in a half-width card
	"hero-stats":  {4, 4, 2, 4},
	"meta":        {4, 3, 1, 3}, // cover facts
}

// balFor lays out n items with a named preset (template helper).
func balFor(preset string, n int) balGrid {
	p, ok := gridPresets[preset]
	if !ok {
		p = [4]int{3, 2, 1, 3}
	}
	return newBalGrid(n, p[0], p[1], p[2], p[3])
}
