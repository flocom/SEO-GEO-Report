package charts

import (
	"html/template"
	"math"
	"strings"
)

func sparkline(o SparkOpts) template.HTML {
	W := math.Max(def(o.Width, 120), 24)
	H := math.Max(def(o.Height, 36), 12)
	col := esc(seriesColor(o.Color, 0))
	const pad = 4.0

	lo, hi := math.Inf(1), math.Inf(-1)
	first, last := math.NaN(), math.NaN()
	for _, v := range o.Values {
		if finite(v) {
			v = clampVal(v)
			lo, hi = math.Min(lo, v), math.Max(hi, v)
			if math.IsNaN(first) {
				first = v
			}
			last = v
		}
	}
	sf := o.Format
	if sf == nil {
		sf = FormatCompact
	}
	lf := o.LabelFormat
	if lf == nil {
		lf = longLabel
	}
	title := ""
	if !math.IsNaN(first) {
		title = sf(first) + " → " + sf(last)
	}

	var c canvas
	c.open("spark", W, H, title)
	if lo > hi {
		c.printf(`<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="currentColor" stroke-opacity="0.25" stroke-width="1.5" stroke-dasharray="3 3"/>`, n(pad), n(H/2), n(W-pad), n(H/2))
		c.close()
		return c.html()
	}
	cnt := len(o.Values)
	xs := func(i int) float64 {
		if cnt <= 1 {
			return W / 2
		}
		return pad + float64(i)*(W-2*pad)/float64(cnt-1)
	}
	ys := func(v float64) float64 {
		if hi == lo {
			return H / 2
		}
		f := (clampVal(v) - lo) / (hi - lo)
		if o.InvertY {
			return pad + f*(H-2*pad)
		}
		return H - pad - f*(H-2*pad)
	}

	var segs [][]pt
	var cur []pt
	for i, v := range o.Values {
		if !finite(v) {
			if len(cur) > 0 {
				segs = append(segs, cur)
				cur = nil
			}
			continue
		}
		cur = append(cur, pt{xs(i), ys(v)})
	}
	if len(cur) > 0 {
		segs = append(segs, cur)
	}

	grad := newID("spark")
	c.printf(`<defs><linearGradient id="%s" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="%s" stop-opacity="0.2"/><stop offset="1" stop-color="%s" stop-opacity="0"/></linearGradient></defs>`, grad, col, col)
	for _, seg := range segs {
		if len(seg) < 2 {
			continue
		}
		var line strings.Builder
		for k, p := range seg {
			if k == 0 {
				line.WriteString("M")
			} else {
				line.WriteString("L")
			}
			line.WriteString(n(p.x) + " " + n(p.y))
		}
		area := line.String() + "L" + n(seg[len(seg)-1].x) + " " + n(H) + "L" + n(seg[0].x) + " " + n(H) + "Z"
		c.printf(`<path d="%s" fill="url(#%s)" stroke="none"/>`, area, grad)
		c.printf(`<path d="%s" fill="none" stroke="%s" stroke-width="1.75" stroke-linejoin="round" stroke-linecap="round"/>`, line.String(), col)
	}
	lastSeg := segs[len(segs)-1]
	p := lastSeg[len(lastSeg)-1]
	c.printf(`<circle cx="%s" cy="%s" r="2.75" fill="%s" %s/>`, n(p.x), n(p.y), col, strings.Replace(surfaceStroke, `stroke-width="2"`, `stroke-width="1.5"`, 1))
	// Hover: one zone per point (every few points on long series, to keep
	// the markup light) with a dot and a one-line tooltip.
	var hv canvas
	if cnt > 1 {
		stride := (cnt + 44) / 45
		bw := (W - 2*pad) / float64(cnt-1) * float64(stride)
		for i := 0; i < cnt; i += stride {
			v := o.Values[i]
			if !finite(v) {
				continue
			}
			x, y := xs(i), ys(v)
			hv.printf(`<g class="sgc-hz"><rect class="sgc-hit" x="%s" y="0" width="%s" height="%s"/><g class="sgc-tip">`,
				n1(clampF(x-bw/2, 0, W)), n1(math.Max(bw, 1)), n1(H))
			hv.printf(`<circle class="sgc-dot" cx="%s" cy="%s" r="2.5" fill="%s" stroke-width="1.5"/>`, n1(x), n1(y), col)
			txt := sf(v)
			if i < len(o.Labels) && o.Labels[i] != "" {
				txt = lf(o.Labels[i]) + " · " + txt
			}
			ay := H * 0.75
			if y > H/2 {
				ay = H * 0.25
			}
			tooltip(&hv, x, ay, W, H, "", []tipRow{{value: txt}}, 8)
			hv.raw(`</g></g>`)
		}
	}
	c.closeHover(&hv)
	return c.html()
}

func deltaArrow(direction, tone string) template.HTML {
	col := ColorNeutral
	switch strings.ToLower(strings.TrimSpace(tone)) {
	case "positive", "good", "up":
		col = ColorPositive
	case "negative", "bad", "down":
		col = ColorNegative
	}
	var shape, label string
	switch strings.ToLower(strings.TrimSpace(direction)) {
	case "up", "+", "increase":
		shape, label = `<path d="M6 1.5 10.5 6.5H7.4V10.5H4.6V6.5H1.5Z" fill="`+col+`"/>`, "↑"
	case "down", "-", "decrease":
		shape, label = `<path d="M6 10.5 10.5 5.5H7.4V1.5H4.6V5.5H1.5Z" fill="`+col+`"/>`, "↓"
	default:
		shape, label = `<rect x="2" y="4.75" width="8" height="2.5" rx="1.25" fill="`+col+`"/>`, "→"
	}
	return template.HTML(`<svg xmlns="http://www.w3.org/2000/svg" class="sgc sgc-arrow" viewBox="0 0 12 12" width="1em" height="1em" role="img" aria-label="` +
		label + `" style="display:inline-block;vertical-align:-0.125em"><title>` + label + `</title>` + shape + `</svg>`)
}
