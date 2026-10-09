package charts

import (
	"html/template"
	"math"
	"strings"
)

type pt struct{ x, y float64 }

func line(o LineOpts) template.HTML {
	W := math.Max(def(o.Width, 720), 160)
	H := math.Max(def(o.Height, 260), 100)
	yf := o.YFormat
	if yf == nil {
		yf = FormatCompact
	}
	xf, tf := o.XFormat, o.TooltipXFormat
	if xf == nil {
		xf = shortLabel
	}
	if tf == nil {
		tf = longLabel
	}
	title := o.Title
	if title == "" {
		var names []string
		for _, s := range o.Series {
			if s.Name != "" {
				names = append(names, s.Name)
			}
		}
		title = strings.Join(names, " · ")
	}

	// Domain.
	count := len(o.Labels)
	dmin, dmax := math.Inf(1), math.Inf(-1)
	for _, s := range o.Series {
		if len(s.Values) > count {
			count = len(s.Values)
		}
		for _, v := range s.Values {
			if finite(v) {
				v = clampVal(v)
				dmin = math.Min(dmin, v)
				dmax = math.Max(dmax, v)
			}
		}
	}
	if count == 0 || dmin > dmax {
		return emptySVG("line", W, H, title, "")
	}

	lo, hi := dmin, dmax
	hasYMin := o.YMin != nil && finite(*o.YMin)
	switch {
	case hasYMin:
		lo = math.Min(clampVal(*o.YMin), dmin)
	case !o.InvertY && lo > 0:
		lo = 0
	}
	if hi == lo {
		if hi == 0 {
			hi = 1
		} else {
			d := math.Abs(hi) * 0.1
			lo, hi = lo-d, hi+d
			if dmin >= 0 && lo < 0 && !o.InvertY {
				lo = 0
			}
		}
	}
	nlo, nhi, step := niceScale(lo, hi, 5)
	if hasYMin && dmin >= *o.YMin && nlo < *o.YMin {
		nlo = *o.YMin
	}
	if o.InvertY && !hasYMin && dmin >= 1 && nlo < 1 {
		nlo = 1 // average position: 1 is the best possible rank
	}
	ticks := buildTicks(nlo, nhi, step)
	span := nhi - nlo
	if !(span > 0) || !finite(span) {
		span = 1
	}

	// Legend (two or more series).
	var c canvas
	c.open("line", W, H, title)
	top := 10.0
	if len(o.Series) >= 2 {
		top = legend(&c, o.Series, W, false) + 14
	}

	// Layout.
	const tickSize = 11.0
	left := 0.0
	for _, t := range ticks {
		left = math.Max(left, textWidth(yf(t), tickSize))
	}
	left = math.Min(math.Max(left+10, 28), W*0.25)
	right := 14.0
	bottom := H - 24
	plotW := W - left - right
	plotH := bottom - top
	if plotH < 20 {
		plotH = 20
		bottom = top + plotH
	}
	ymap := func(v float64) float64 {
		f := (clampVal(v) - nlo) / span
		if o.InvertY {
			return top + f*plotH
		}
		return bottom - f*plotH
	}
	xmap := func(i int) float64 {
		if count == 1 {
			return left + plotW/2
		}
		return left + float64(i)*plotW/float64(count-1)
	}

	// Grid and y ticks.
	for _, t := range ticks {
		y := ymap(t)
		c.printf(`<line x1="%s" y1="%s" x2="%s" y2="%s" %s/>`, n(left), n(y), n(left+plotW), n(y), gridStroke)
		c.text(left-8, y+tickSize*0.35, yf(t), tickSize, "end", inkMuted+` style="font-variant-numeric:tabular-nums"`)
	}
	c.printf(`<line x1="%s" y1="%s" x2="%s" y2="%s" %s/>`, n(left), n(bottom), n(left+plotW), n(bottom), axisStroke)

	// X labels, thinned.
	maxW := 0.0
	for _, l := range o.Labels {
		maxW = math.Max(maxW, textWidth(xf(l), tickSize))
	}
	labelCount := min(len(o.Labels), count)
	if labelCount > 0 {
		dx := plotW
		if count > 1 {
			dx = plotW / float64(count-1)
		}
		lw := math.Min(maxW, 120)
		idx := pickAxisLabels(labelCount, dx, lw)
		for _, i := range idx {
			// Keep labels inside the viewBox (centered, clamped at the edges).
			lab, _ := truncate(xf(o.Labels[i]), 120, tickSize)
			w := textWidth(lab, tickSize)
			x := clampF(xmap(i), w/2+1, W-w/2-1)
			c.labelText(x, H-6, xf(o.Labels[i]), 120, tickSize, "middle", inkMuted)
		}
	}

	// Baseline for areas.
	base := bottom
	if !o.InvertY && nlo <= 0 && nhi >= 0 {
		base = ymap(0)
	}

	// Segments split on NaN gaps.
	segsOf := func(s Series) [][]pt {
		var segs [][]pt
		var cur []pt
		for i, v := range s.Values {
			if !finite(v) {
				if len(cur) > 0 {
					segs = append(segs, cur)
					cur = nil
				}
				continue
			}
			cur = append(cur, pt{xmap(i), ymap(v)})
		}
		if len(cur) > 0 {
			segs = append(segs, cur)
		}
		return segs
	}

	// Draw order: areas, then comparison (dashed) lines, then solid lines.
	for i, s := range o.Series {
		if !s.Area {
			continue
		}
		col := seriesColor(s.Color, i)
		id := newID("area")
		aTop, aBot := 0.22, 0.02
		if s.Dashed {
			aTop, aBot = 0.1, 0.01
		}
		c.printf(`<defs><linearGradient id="%s" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="%s" stop-opacity="%s"/><stop offset="1" stop-color="%s" stop-opacity="%s"/></linearGradient></defs>`,
			id, esc(col), n(aTop), esc(col), n(aBot))
		for _, seg := range segsOf(s) {
			if len(seg) < 2 {
				continue
			}
			var b strings.Builder
			b.WriteString("M" + n(seg[0].x) + " " + n(base))
			for _, p := range seg {
				b.WriteString("L" + n(p.x) + " " + n(p.y))
			}
			b.WriteString("L" + n(seg[len(seg)-1].x) + " " + n(base) + "Z")
			c.printf(`<path d="%s" fill="url(#%s)" stroke="none"/>`, b.String(), id)
		}
	}
	for pass := 0; pass < 2; pass++ {
		for i, s := range o.Series {
			if s.Dashed != (pass == 0) {
				continue
			}
			col := esc(seriesColor(s.Color, i))
			dash := ""
			if s.Dashed {
				dash = ` stroke-dasharray="5 4" stroke-opacity="0.9"`
			}
			segs := segsOf(s)
			for _, seg := range segs {
				if len(seg) == 1 {
					c.printf(`<circle cx="%s" cy="%s" r="2.5" fill="%s"/>`, n(seg[0].x), n(seg[0].y), col)
					continue
				}
				var b strings.Builder
				for k, p := range seg {
					if k == 0 {
						b.WriteString("M")
					} else {
						b.WriteString("L")
					}
					b.WriteString(n(p.x) + " " + n(p.y))
				}
				c.printf(`<path d="%s" fill="none" stroke="%s" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"%s/>`, b.String(), col, dash)
			}
			// End dot on the last point of solid series.
			if !s.Dashed && len(segs) > 0 {
				last := segs[len(segs)-1]
				p := last[len(last)-1]
				if count > 1 || len(last) == 1 {
					c.printf(`<circle cx="%s" cy="%s" r="4" fill="%s" %s/>`, n(p.x), n(p.y), col, surfaceStroke)
				}
			}
		}
	}

	// Native tooltips: one invisible band per x position.
	if count <= 500 {
		bw := plotW
		if count > 1 {
			bw = plotW / float64(count-1)
		}
		for i := 0; i < count; i++ {
			x0 := clampF(xmap(i)-bw/2, left, left+plotW)
			x1 := clampF(xmap(i)+bw/2, left, left+plotW)
			var tip []string
			if i < len(o.Labels) && o.Labels[i] != "" {
				tip = append(tip, tf(o.Labels[i]))
			}
			for _, s := range o.Series {
				v := math.NaN()
				if i < len(s.Values) {
					v = s.Values[i]
				}
				if s.Name != "" {
					tip = append(tip, s.Name+": "+yf(v))
				} else {
					tip = append(tip, yf(v))
				}
			}
			c.printf(`<rect x="%s" y="%s" width="%s" height="%s" fill="#000" fill-opacity="0"><title>%s</title></rect>`,
				n(x0), n(top), n(math.Max(x1-x0, 1)), n(plotH), esc(strings.Join(tip, " · ")))
		}
	}
	c.close()
	return c.html()
}

// legend draws a wrapping horizontal legend at the top and returns its height.
// Keys are short line segments (dashed for comparison series), or rounded
// squares when swatch is set (column charts).
func legend(c *canvas, series []Series, W float64, swatch bool) float64 {
	const size = 12.0
	const rowH = 18.0
	x, y := 0.0, 12.0
	for i, s := range series {
		name := s.Name
		if name == "" {
			name = "—"
		}
		name, _ = truncate(name, W*0.45, size)
		w := 22 + textWidth(name, size) + 18
		if x > 0 && x+w > W {
			x = 0
			y += rowH
		}
		col := esc(seriesColor(s.Color, i))
		dash := ""
		if s.Dashed {
			dash = ` stroke-dasharray="4 3"`
		}
		tx := x + 22
		if swatch {
			c.printf(`<rect x="%s" y="%s" width="10" height="10" rx="3" fill="%s"/>`, n(x), n(y-9), col)
			tx = x + 16
			w -= 6
		} else {
			c.printf(`<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="2.5" stroke-linecap="round"%s/>`, n(x+1), n(y-4), n(x+16), n(y-4), col, dash)
		}
		c.text(tx, y, name, size, "", inkSecondary)
		x += w
	}
	return y + 4
}
