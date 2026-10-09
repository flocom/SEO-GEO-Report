package charts

import (
	"fmt"
	"html/template"
	"math"
	"sort"
)

func gauge(o GaugeOpts) template.HTML {
	gf := o.Format
	if gf == nil {
		gf = FormatCompact
	}
	mn, mx := o.Min, o.Max
	if !finite(mn) {
		mn = 0
	}
	if !finite(mx) {
		mx = 0
	}
	mn, mx = clampVal(mn), clampVal(mx)
	if mn == 0 && mx == 0 {
		mx = 100
	}
	if mx < mn {
		mn, mx = mx, mn
	}
	if mx == mn {
		mx = mn + 1
	}
	span := mx - mn

	bands := append([]GaugeBand(nil), o.Bands...)
	if len(bands) == 0 {
		bands = []GaugeBand{
			{Upto: mn + span/3, Color: ColorNegative},
			{Upto: mn + span*2/3, Color: "#f59e0b"},
			{Upto: mx, Color: ColorPositive},
		}
	}
	sort.SliceStable(bands, func(i, j int) bool { return bands[i].Upto < bands[j].Upto })

	hasValue := finite(o.Value)
	v := mn
	if hasValue {
		v = clampF(clampVal(o.Value), mn, mx)
	}
	frac := func(x float64) float64 { return clampF((x-mn)/span, 0, 1) }

	// Color of the band containing the value.
	valColor := ColorNeutral
	for i, b := range bands {
		if v <= b.Upto || i == len(bands)-1 {
			valColor = seriesColor(b.Color, i)
			if v <= b.Upto {
				break
			}
		}
	}

	const W = 240.0
	const r = 92.0
	const t = 18.0
	cx := W / 2
	cy := r + t/2 + 6
	H := cy + 30
	ang := func(fr float64) float64 { return math.Pi + fr*math.Pi } // left → right over the top
	arc := func(f0, f1 float64) string {
		x0, y0 := polar(cx, cy, r, ang(f0))
		x1, y1 := polar(cx, cy, r, ang(f1))
		return fmt.Sprintf("M%s %sA%s %s 0 0 1 %s %s", n(x0), n(y0), n(r), n(r), n(x1), n(y1))
	}

	var c canvas
	c.openCapped("gauge", W, H, o.Title, W*1.5)
	// Track: neutral full arc, then bands as pale washes.
	c.printf(`<path d="%s" fill="none" stroke="currentColor" stroke-opacity="0.08" stroke-width="%s"/>`, arc(0, 1), n(t))
	prev := 0.0
	for i, b := range bands {
		f1 := frac(b.Upto)
		if !finite(b.Upto) {
			continue
		}
		if f1 > prev {
			g := 0.0
			if f1 < 1 {
				g = 0.006
			}
			c.printf(`<path d="%s" fill="none" stroke="%s" stroke-opacity="0.22" stroke-width="%s"/>`, arc(prev, f1-g), esc(seriesColor(b.Color, i)), n(t))
			prev = f1
		}
	}
	// Value arc.
	fv := frac(v)
	if hasValue && fv > 0.002 {
		c.printf(`<path d="%s" fill="none" stroke="%s" stroke-width="%s"/>`, arc(0, fv), esc(valColor), n(t))
	}
	// Knob marker at the value.
	if hasValue {
		kx, ky := polar(cx, cy, r, ang(fv))
		c.printf(`<circle cx="%s" cy="%s" r="%s" fill="%s" %s/>`, n(kx), n(ky), n(t/2+2), esc(valColor), surfaceStroke)
	}

	// Center value.
	disp := o.Display
	if disp == "" {
		if hasValue {
			disp = gf(o.Value)
		} else {
			disp = noValue
		}
	}
	inner := (r - t/2) * 2 * 0.82
	size := 34.0
	for size > 14 && textWidth(disp, size) > inner {
		size--
	}
	c.labelText(cx, cy-8, disp, inner, size, "middle", inkPrimary+` font-weight="650"`)

	// Min / max and label.
	c.text(cx-r, cy+t/2+12, gf(mn), 10, "middle", inkMuted)
	c.text(cx+r, cy+t/2+12, gf(mx), 10, "middle", inkMuted)
	if o.Label != "" {
		c.labelText(cx, cy+18, o.Label, W-2*(r*0.55), 12, "middle", inkSecondary)
	}
	c.close()
	return c.html()
}
