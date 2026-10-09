package charts

import (
	"html/template"
	"math"
	"strings"
)

// delta computes the relative change of cur vs prev in percent.
func delta(cur, prev float64) (float64, bool) {
	if !finite(cur) || !finite(prev) || prev == 0 {
		return 0, false
	}
	d := (cur - prev) / math.Abs(prev) * 100
	return d, finite(d)
}

// badge draws a discreet delta label (small arrow + colored text) starting
// at x, vertically centered on cy, and returns its width.
func badge(c *canvas, x, cy, pct float64, df Formatter) float64 {
	const size = 10.5
	txt := df(pct)
	ink, arrow := badgeNeutralText, ""
	switch {
	case math.Abs(pct) < 0.05:
	case pct > 0:
		ink, arrow = badgePositiveText, "▲"
	case pct < 0:
		ink, arrow = badgeNegativeText, "▼"
	}
	if arrow != "" {
		c.printf(`<text x="%s" y="%s" font-size="%s" fill="%s" font-weight="600"><tspan font-size="%s">%s</tspan> %s</text>`,
			n(x), n(cy+size*0.36), n(size), ink, n(size*0.72), arrow, esc(txt))
	} else {
		c.text(x, cy+size*0.36, txt, size, "", `fill="`+ink+`" font-weight="600"`)
	}
	return badgeWidth(pct, df)
}

func badgeWidth(pct float64, df Formatter) float64 { return textWidth(df(pct), 10.5)*1.05 + 12 }

// prevName is the tooltip name of the previous-period value.
func prevName(name string) string {
	if name == "" {
		return "vs"
	}
	return name
}

func bars(o BarsOpts) template.HTML {
	W := math.Max(def(o.Width, 720), 200)
	f := o.Format
	if f == nil {
		f = FormatCompact
	}
	df := o.DeltaFormat
	if df == nil {
		df = FormatDelta
	}
	if len(o.Items) == 0 {
		return emptySVG("bars", W, 120, o.Title, "")
	}
	const labelSize, valueSize = 12.5, 12.0

	// Domain (always includes zero).
	lo, hi := 0.0, 0.0
	maxLabel, maxRight := 0.0, 0.0
	for _, it := range o.Items {
		v := it.Value
		if finite(v) {
			lo, hi = math.Min(lo, clampVal(v)), math.Max(hi, clampVal(v))
		}
		if it.Previous != nil && finite(*it.Previous) {
			p := clampVal(*it.Previous)
			lo, hi = math.Min(lo, p), math.Max(hi, p)
		}
		maxLabel = math.Max(maxLabel, textWidth(it.Label, labelSize))
		r := textWidth(f(v), valueSize) + 8
		if o.ShowDelta && it.Previous != nil {
			if d, ok := delta(v, *it.Previous); ok {
				r += badgeWidth(d, df) + 8
			}
		}
		maxRight = math.Max(maxRight, r)
	}
	if hi == lo {
		hi = lo + 1
	}
	maxRight = math.Min(maxRight+4, W*0.4)

	// Layout: label column on the left when labels are short enough,
	// otherwise label above each bar (long URLs / queries).
	above := maxLabel > W*0.3
	var x0, x1, rowH, barH, labelW, pad float64
	pad = 4
	if above {
		x0, rowH, barH = 0, 42, 12
	} else {
		labelW = math.Min(maxLabel, W*0.3)
		x0, rowH, barH = labelW+14, 30, 14
	}
	if o.Height > 0 && finite(o.Height) && len(o.Items) > 0 {
		limit := rowH * 1.6
		if fill := (o.Height - pad*2) / float64(len(o.Items)); fill > rowH {
			rowH = math.Min(fill, limit)
		}
	}
	x1 = W - maxRight
	if x1-x0 < 40 {
		x1 = x0 + 40
	}
	xs := func(v float64) float64 {
		if !finite(v) {
			v = 0
		}
		return x0 + (clampVal(v)-lo)/(hi-lo)*(x1-x0)
	}
	zero := xs(0)
	H := pad*2 + rowH*float64(len(o.Items))

	var c, hv canvas
	c.open("bars", W, H, o.Title)
	if lo < 0 {
		c.printf(`<line x1="%s" y1="%s" x2="%s" y2="%s" %s/>`, n(zero), n(pad), n(zero), n(H-pad), axisStroke)
	}
	for i, it := range o.Items {
		rowTop := pad + float64(i)*rowH
		var barY float64
		if above {
			barY = rowTop + 22 + (rowH-42)/2
		} else {
			barY = rowTop + (rowH-barH)/2
		}
		cy := barY + barH/2
		col := esc(seriesColor(it.Color, 0))

		var d float64
		hasDelta := false
		if it.Previous != nil {
			if dd, ok := delta(it.Value, *it.Previous); ok {
				d, hasDelta = dd, true
			}
		}

		// Label.
		if above {
			c.labelText(0, barY-8, it.Label, W-maxRight*0.2, labelSize, "", inkPrimary)
		} else {
			c.labelText(0, cy+labelSize*0.35, it.Label, labelW, labelSize, "", inkPrimary)
		}

		end := math.Max(xs(it.Value), zero)
		// Ghost bar for the previous period + end tick.
		if it.Previous != nil && finite(*it.Previous) {
			px := xs(*it.Previous)
			if p := hBarPath(zero, px, barY, barH, 3); p != "" {
				c.printf(`<path class="sgc-mark" d="%s" fill="%s" fill-opacity="0.3"/>`, p, ColorPrevious)
			}
			c.printf(`<rect class="sgc-mark" x="%s" y="%s" width="2" height="%s" rx="1" fill="%s"/>`, n(px-1), n(barY-3), n(barH+6), ColorPrevious)
			end = math.Max(end, px+1)
		}
		bar := hBarPath(zero, xs(it.Value), barY, barH, 3)
		if bar != "" {
			c.printf(`<path class="sgc-mark" d="%s" fill="%s"/>`, bar, col)
		}
		vx := end + 8
		vs := f(it.Value)
		c.text(vx, cy+valueSize*0.35, vs, valueSize, "", inkPrimary+` font-weight="600" style="font-variant-numeric:tabular-nums"`)
		if o.ShowDelta && hasDelta {
			badge(&c, vx+textWidth(vs, valueSize)+8, cy, d, df)
		}

		// Hover zone: highlight the bar and show the details.
		hv.printf(`<g class="sgc-hz"><rect class="sgc-hit" x="0" y="%s" width="%s" height="%s"/><g class="sgc-tip">`, n(rowTop), n(W), n(rowH))
		hv.printf(`<rect class="sgc-hl" x="0" y="%s" width="%s" height="%s" rx="4"/>`, n(rowTop), n(W), n(rowH))
		if bar != "" {
			hv.printf(`<path d="%s" fill="%s"/>`, bar, col)
		}
		rows := []tipRow{{key: seriesColor(it.Color, 0), value: f(it.Value)}}
		if it.Previous != nil {
			rows = append(rows, tipRow{key: ColorPrevious, name: prevName(o.PreviousLabel), value: f(*it.Previous), muted: true})
		}
		if hasDelta {
			rows = append(rows, tipRow{name: "Δ", value: df(d)})
		}
		tooltip(&hv, math.Max(xs(it.Value), zero), cy, W, H, it.Label, rows, 12)
		hv.raw(`</g></g>`)
	}
	c.closeHover(&hv)
	return c.html()
}

func columns(o ColumnsOpts) template.HTML {
	W := math.Max(def(o.Width, 720), 200)
	H := math.Max(def(o.Height, 260), 100)
	f := o.Format
	if f == nil {
		f = FormatCompact
	}
	df := o.DeltaFormat
	if df == nil {
		df = FormatDelta
	}
	curRaw, prevRaw := seriesColor(o.Color, 0), ColorPrevious
	if strings.TrimSpace(o.PreviousColor) != "" {
		prevRaw = o.PreviousColor
	}
	curCol, prevCol := esc(curRaw), esc(prevRaw)
	count := max(len(o.Labels), len(o.Current))
	hasPrev := len(o.Previous) > 0
	if count == 0 || len(o.Current) == 0 {
		return emptySVG("columns", W, H, o.Title, "")
	}
	at := func(s []float64, i int) float64 {
		if i < len(s) {
			return s[i]
		}
		return math.NaN()
	}
	lo, hi := 0.0, 0.0
	seen := false
	for i := 0; i < count; i++ {
		for _, v := range []float64{at(o.Current, i), at(o.Previous, i)} {
			if finite(v) {
				seen = true
				lo, hi = math.Min(lo, clampVal(v)), math.Max(hi, clampVal(v))
			}
		}
	}
	if !seen {
		return emptySVG("columns", W, H, o.Title, "")
	}
	nlo, nhi, step := niceScale(lo, hi, 5)
	ticks := buildTicks(nlo, nhi, step)
	span := nhi - nlo
	if !(span > 0) {
		span = 1
	}

	var c, hv canvas
	c.open("columns", W, H, o.Title)
	top := 12.0
	if hasPrev && (o.CurrentName != "" || o.PreviousName != "") {
		top = legend(&c, []Series{
			{Name: o.CurrentName, Color: curRaw},
			{Name: o.PreviousName, Color: prevRaw},
		}, W, true) + 16
	}
	const tickSize = 11.0
	left := 0.0
	for _, t := range ticks {
		left = math.Max(left, textWidth(f(t), tickSize))
	}
	left = math.Min(math.Max(left+10, 28), W*0.25)
	right := 8.0
	bottom := H - 24
	plotW, plotH := W-left-right, bottom-top
	if plotH < 20 {
		plotH, bottom = 20, top+20
	}
	ys := func(v float64) float64 { return bottom - (clampVal(v)-nlo)/span*plotH }

	for _, t := range ticks {
		y := ys(t)
		c.printf(`<line x1="%s" y1="%s" x2="%s" y2="%s" %s/>`, n(left), n(y), n(left+plotW), n(y), gridStroke)
		c.text(left-8, y+tickSize*0.35, f(t), tickSize, "end", inkMuted+` style="font-variant-numeric:tabular-nums"`)
	}
	base := bottom
	if nlo <= 0 && nhi >= 0 {
		base = ys(0)
	}

	groupW := plotW / float64(count)
	inner := groupW * 0.72
	var colW float64
	if hasPrev {
		colW = math.Min(24, (inner-2)/2)
	} else {
		colW = math.Min(28, inner)
	}
	colW = math.Max(colW, 1)
	showValues := count <= 12 && colW >= 10

	// X labels.
	maxW := 0.0
	for _, l := range o.Labels {
		maxW = math.Max(maxW, textWidth(shortLabel(l), tickSize))
	}
	// Categories: label every column (truncated to its slot) when slots are
	// wide enough; otherwise (e.g. daily dates) thin the labels out.
	labelIdx := map[int]bool{}
	lw := math.Min(maxW, groupW-10)
	if groupW < 44 {
		lw = math.Min(maxW, 72)
	}
	idx := pickAxisLabels(min(len(o.Labels), count), groupW, lw)
	for _, i := range idx {
		labelIdx[i] = true
	}
	labelMax := lw + 1

	for i := 0; i < count; i++ {
		cx := left + groupW*(float64(i)+0.5)
		cur, prev := at(o.Current, i), at(o.Previous, i)
		label := ""
		if i < len(o.Labels) {
			label = o.Labels[i]
		}
		curX := cx - colW/2
		var prevBar, curBar string
		if hasPrev {
			px := cx - colW - 1
			curX = cx + 1
			if finite(prev) {
				if prevBar = vBarPath(px, colW, base, ys(prev), 3); prevBar != "" {
					c.printf(`<path class="sgc-mark" d="%s" fill="%s" fill-opacity="0.55"/>`, prevBar, prevCol)
				}
			}
		}
		ay := base
		if finite(cur) {
			y1 := ys(cur)
			ay = y1
			if curBar = vBarPath(curX, colW, base, y1, 3); curBar != "" {
				c.printf(`<path class="sgc-mark" d="%s" fill="%s"/>`, curBar, curCol)
			}
			if showValues {
				vy := y1 - 5
				if cur < 0 {
					vy = y1 + 13
				}
				c.text(curX+colW/2, vy, f(cur), 10.5, "middle", inkSecondary+` font-weight="600"`)
			}
		}
		if labelIdx[i] && label != "" {
			lab, _ := truncate(shortLabel(label), labelMax, tickSize)
			w := textWidth(lab, tickSize)
			c.labelText(clampF(cx, w/2+1, W-w/2-1), H-6, shortLabel(label), labelMax, tickSize, "middle", inkMuted)
		}

		// Hover zone.
		hv.printf(`<g class="sgc-hz"><rect class="sgc-hit" x="%s" y="%s" width="%s" height="%s"/><g class="sgc-tip">`, n(cx-groupW/2), n(top), n(groupW), n(plotH))
		hv.printf(`<rect class="sgc-hl" x="%s" y="%s" width="%s" height="%s" rx="3"/>`, n(cx-groupW/2), n(top), n(groupW), n(plotH))
		if prevBar != "" {
			hv.printf(`<path d="%s" fill="%s" fill-opacity="0.55"/>`, prevBar, prevCol)
		}
		if curBar != "" {
			hv.printf(`<path d="%s" fill="%s"/>`, curBar, curCol)
		}
		rows := []tipRow{{key: curRaw, name: o.CurrentName, value: f(cur)}}
		if hasPrev {
			rows = append(rows, tipRow{key: prevRaw, name: prevName(o.PreviousName), value: f(prev), muted: true})
			if d, ok := delta(cur, prev); ok {
				rows = append(rows, tipRow{name: "Δ", value: df(d)})
			}
		}
		head := ""
		if label != "" {
			head = longLabel(label)
		}
		tooltip(&hv, cx+colW+2, ay, W, H, head, rows, 12)
		hv.raw(`</g></g>`)
	}
	c.printf(`<line x1="%s" y1="%s" x2="%s" y2="%s" %s/>`, n(left), n(base), n(left+plotW), n(base), axisStroke)
	c.closeHover(&hv)
	return c.html()
}
