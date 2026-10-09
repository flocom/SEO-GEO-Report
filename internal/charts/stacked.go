package charts

import (
	"html/template"
	"math"
)

func stacked(o StackedOpts) template.HTML {
	W := math.Max(def(o.Width, 720), 160)
	f := o.Format
	if f == nil {
		f = FormatCompact
	}
	type seg struct {
		StackSegment
		color string
	}
	var segs []seg
	total := 0.0
	for i, s := range o.Segments {
		if !finite(s.Value) || s.Value <= 0 {
			continue
		}
		s.Value = clampVal(s.Value)
		segs = append(segs, seg{s, seriesColor(s.Color, i)})
		total += s.Value
	}
	if len(segs) == 0 || !(total > 0) || !finite(total) {
		return emptySVG("stacked", W, 64, o.Title, "")
	}

	const barY, barH = 2.0, 28.0
	const gap = 2.0
	const size = 12.0

	// Legend layout (computed first to know the height).
	type item struct {
		x, y       float64
		label, val string
	}
	var items []item
	lx, ly := 0.0, barY+barH+24
	for _, s := range segs {
		label, _ := truncate(s.Label, W*0.5, size)
		val := f(s.Value) + " · " + sharePct(s.Value, total, o.PctFormat)
		w := 16 + (textWidth(label, size)+textWidth(val, size))*1.08 + 12 + 24
		if lx > 0 && lx+w > W {
			lx = 0
			ly += 22
		}
		items = append(items, item{lx, ly, label, val})
		lx += w
	}
	H := ly + 8

	var c canvas
	c.open("stacked", W, H, o.Title)
	clip := newID("clip")
	c.printf(`<defs><clipPath id="%s"><rect x="0" y="%s" width="%s" height="%s" rx="6"/></clipPath></defs>`, clip, n(barY), n(W), n(barH))
	c.printf(`<g clip-path="url(#%s)">`, clip)
	usable := W - gap*float64(len(segs)-1)
	x := 0.0
	for _, s := range segs {
		w := s.Value / total * usable
		pct := sharePct(s.Value, total, o.PctFormat)
		c.printf(`<g><title>%s</title>`, esc(s.Label+": "+f(s.Value)+" ("+pct+")"))
		c.printf(`<rect x="%s" y="%s" width="%s" height="%s" fill="%s"/>`, n(x), n(barY), n(math.Max(w, 0.5)), n(barH), esc(s.color))
		if w >= textWidth(pct, 11.5)+14 {
			ink := `fill="#ffffff"`
			if isLight(s.color) {
				ink = `fill="#0f172a"`
			}
			c.text(x+w/2, barY+barH/2+11.5*0.36, pct, 11.5, "middle", ink+` font-weight="600"`)
		}
		c.raw(`</g>`)
		x += w + gap
	}
	c.raw(`</g>`)

	for i, it := range items {
		s := segs[i]
		c.printf(`<g><title>%s</title>`, esc(s.Label+": "+it.val))
		c.printf(`<rect x="%s" y="%s" width="10" height="10" rx="3" fill="%s"/>`, n(it.x), n(it.y-9), esc(s.color))
		c.printf(`<text x="%s" y="%s" font-size="%s" %s>%s<tspan dx="6" %s>%s</tspan></text>`,
			n(it.x+16), n(it.y), n(size), inkPrimary, esc(it.label), inkMuted, esc(it.val))
		c.raw(`</g>`)
	}
	c.close()
	return c.html()
}
