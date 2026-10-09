package charts

import (
	"fmt"
	"html/template"
	"math"
	"sort"
	"strings"
)

// donutMaxSlices is the most slices drawn before the smallest ones are merged.
const donutMaxSlices = 7

// tinyShare is the share under which slices are merged (when 2+ are tiny).
const tinyShare = 0.015

// prepareSlices drops empty/invalid slices, assigns colors and merges the
// smallest ones into a single grey slice labelled with their names.
func prepareSlices(in []Slice) ([]Slice, float64) {
	var out []Slice
	total := 0.0
	for i, s := range in {
		if !finite(s.Value) || s.Value <= 0 {
			continue
		}
		s.Value = clampVal(s.Value)
		s.Color = seriesColor(s.Color, i)
		out = append(out, s)
		total += s.Value
	}
	if total <= 0 || !finite(total) {
		return nil, 0
	}
	// Decide which to merge: beyond the top donutMaxSlices, plus tiny ones.
	order := make([]int, len(out))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return out[order[a]].Value > out[order[b]].Value })
	merge := make([]bool, len(out))
	nMerge := 0
	for rank, i := range order {
		if rank >= donutMaxSlices || out[i].Value/total < tinyShare {
			merge[i] = true
			nMerge++
		}
	}
	if nMerge < 2 {
		return out, total
	}
	var kept []Slice
	var names []string
	other := 0.0
	for i, s := range out {
		if merge[i] {
			other += s.Value
			names = append(names, s.Label)
			continue
		}
		kept = append(kept, s)
	}
	kept = append(kept, Slice{Label: strings.Join(names, ", "), Value: other, Color: ColorPrevious})
	return kept, total
}

func polar(cx, cy, r, a float64) (float64, float64) {
	return cx + r*math.Cos(a), cy + r*math.Sin(a)
}

// arcPath returns an SVG arc from angle a0 to a1 (radians, clockwise).
func arcPath(cx, cy, r, a0, a1 float64) string {
	x0, y0 := polar(cx, cy, r, a0)
	x1, y1 := polar(cx, cy, r, a1)
	large := 0
	if a1-a0 > math.Pi {
		large = 1
	}
	return fmt.Sprintf("M%s %sA%s %s 0 %d 1 %s %s", n(x0), n(y0), n(r), n(r), large, n(x1), n(y1))
}

// sharePct formats v as a share of total with pf (nil = FormatPct).
func sharePct(v, total float64, pf Formatter) string {
	if pf == nil {
		pf = FormatPct
	}
	p := v / total * 100
	if p > 0 && p < 0.1 {
		return "<" + pf(0.1)
	}
	return pf(p)
}

func donut(o DonutOpts) template.HTML {
	S := math.Max(def(o.Size, 220), 80)
	f := o.Format
	if f == nil {
		f = FormatCompact
	}
	slices, total := prepareSlices(o.Slices)

	r := S/2 - 4
	thick := math.Max(S*0.14, 8)
	rm := r - thick/2
	holeW := (rm - thick/2) * 2 * 0.86

	center := func(c *canvas, cx, cy float64, value string) {
		size := 26.0 * S / 220
		for size > 12 && textWidth(value, size) > holeW {
			size--
		}
		vy := cy + size*0.35
		if o.CenterLabel != "" {
			vy -= 7 * S / 220
		}
		c.labelText(cx, vy, value, holeW, size, "middle", inkPrimary+` font-weight="650"`)
		if o.CenterLabel != "" {
			ls := math.Max(11*S/220, 9)
			c.labelText(cx, vy+ls+6*S/220, o.CenterLabel, holeW, ls, "middle", inkMuted)
		}
	}

	if len(slices) == 0 {
		var c canvas
		c.openCapped("donut sgc-empty", S, S, o.Title, S*1.25)
		c.printf(`<circle cx="%s" cy="%s" r="%s" fill="none" stroke="currentColor" stroke-opacity="0.08" stroke-width="%s"/>`, n(S/2), n(S/2), n(rm), n(thick))
		cv := o.CenterValue
		if cv == "" {
			cv = "—"
		}
		center(&c, S/2, S/2, cv)
		c.close()
		return c.html()
	}

	// Legend geometry.
	const rowH = 24.0
	const size = 12.5
	legendX := S + 28
	legendW := 300.0
	W := legendX + legendW
	H := math.Max(S, rowH*float64(len(slices))+8)
	cx, cy := S/2, H/2

	var c canvas
	c.openCapped("donut", W, H, o.Title, W*1.4)

	// Ring.
	gap := 0.0
	if len(slices) > 1 {
		gap = 2 / rm
	}
	a := -math.Pi / 2
	for _, s := range slices {
		sweep := s.Value / total * 2 * math.Pi
		tip := s.Label + ": " + f(s.Value) + " (" + sharePct(s.Value, total, o.PctFormat) + ")"
		col := esc(s.Color)
		if len(slices) == 1 || sweep >= 2*math.Pi-1e-9 {
			c.printf(`<circle cx="%s" cy="%s" r="%s" fill="none" stroke="%s" stroke-width="%s"><title>%s</title></circle>`, n(cx), n(cy), n(rm), col, n(thick), esc(tip))
		} else {
			g := math.Min(gap, sweep*0.4)
			c.printf(`<path d="%s" fill="none" stroke="%s" stroke-width="%s"><title>%s</title></path>`,
				arcPath(cx, cy, rm, a+g/2, a+sweep-g/2), col, n(thick), esc(tip))
		}
		a += sweep
	}
	cv := o.CenterValue
	if cv == "" {
		cv = f(total)
	}
	center(&c, cx, cy, cv)

	// Legend: swatch · label ........ value   pct
	pctW, valW := 0.0, 0.0
	for _, s := range slices {
		pctW = math.Max(pctW, textWidth(sharePct(s.Value, total, o.PctFormat), 12))
		valW = math.Max(valW, textWidth(f(s.Value), 12))
	}
	pctX := W
	valX := pctX - pctW - 14
	labelX := legendX + 18
	labelMax := valX - valW - 14 - labelX
	y := cy - rowH*float64(len(slices))/2
	for _, s := range slices {
		my := y + rowH/2
		c.printf(`<g><title>%s</title>`, esc(s.Label+": "+f(s.Value)+" ("+sharePct(s.Value, total, o.PctFormat)+")"))
		c.printf(`<rect x="%s" y="%s" width="%s" height="%s" fill="#000" fill-opacity="0"/>`, n(legendX), n(y), n(legendW), n(rowH))
		c.printf(`<rect x="%s" y="%s" width="10" height="10" rx="3" fill="%s"/>`, n(legendX), n(my-5), esc(s.Color))
		c.labelText(labelX, my+size*0.35, s.Label, labelMax, size, "", inkPrimary)
		c.text(valX, my+12*0.35, f(s.Value), 12, "end", inkMuted+` style="font-variant-numeric:tabular-nums"`)
		c.text(pctX, my+12*0.35, sharePct(s.Value, total, o.PctFormat), 12, "end", inkPrimary+` font-weight="600" style="font-variant-numeric:tabular-nums"`)
		c.raw(`</g>`)
		y += rowH
	}
	c.close()
	return c.html()
}
