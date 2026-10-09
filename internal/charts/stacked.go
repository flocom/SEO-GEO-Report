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

	var c, hv canvas
	c.open("stacked", W, H, o.Title)
	clip := newID("clip")
	c.printf(`<defs><clipPath id="%s"><rect x="0" y="%s" width="%s" height="%s" rx="4"/></clipPath></defs>`, clip, n(barY), n(W), n(barH))
	c.printf(`<g clip-path="url(#%s)">`, clip)
	usable := W - gap*float64(len(segs)-1)
	x := 0.0
	type span struct{ x, w float64 }
	spans := make([]span, len(segs))
	for k, s := range segs {
		w := s.Value / total * usable
		spans[k] = span{x, w}
		pct := sharePct(s.Value, total, o.PctFormat)
		c.printf(`<rect class="sgc-mark" x="%s" y="%s" width="%s" height="%s" fill="%s"/>`, n(x), n(barY), n(math.Max(w, 0.5)), n(barH), esc(s.color))
		if w >= textWidth(pct, 11.5)+14 {
			ink := `fill="#ffffff"`
			if isLight(s.color) {
				ink = `fill="#0f172a"`
			}
			c.text(x+w/2, barY+barH/2+11.5*0.36, pct, 11.5, "middle", ink+` font-weight="600"`)
		}
		x += w + gap
	}
	c.raw(`</g>`)

	for k, it := range items {
		s := segs[k]
		c.printf(`<rect x="%s" y="%s" width="10" height="10" rx="2" fill="%s"/>`, n(it.x), n(it.y-9), esc(s.color))
		c.printf(`<text x="%s" y="%s" font-size="%s" %s>%s<tspan dx="6" %s>%s</tspan></text>`,
			n(it.x+16), n(it.y), n(size), inkPrimary, esc(it.label), inkMuted, esc(it.val))

		// Hover zone: the segment and its legend item.
		sp := spans[k]
		hw := math.Max(sp.w, 4)
		lw := 16 + (textWidth(it.label, size)+textWidth(it.val, size))*1.08 + 6
		hv.printf(`<g class="sgc-hz"><rect class="sgc-hit" x="%s" y="%s" width="%s" height="%s"/>`, n(sp.x+sp.w/2-hw/2), n(barY), n(hw), n(barH))
		hv.printf(`<rect class="sgc-hit" x="%s" y="%s" width="%s" height="18"/><g class="sgc-tip">`, n(it.x-3), n(it.y-13), n(lw))
		hv.printf(`<rect class="sgc-hl" x="%s" y="%s" width="%s" height="18" rx="3"/>`, n(it.x-3), n(it.y-13), n(lw))
		hv.printf(`<rect x="%s" y="%s" width="%s" height="%s" fill="%s" clip-path="url(#%s)"/>`, n(sp.x), n(barY), n(math.Max(sp.w, 0.5)), n(barH), esc(s.color), clip)
		tooltip(&hv, sp.x+sp.w/2, barY+barH/2, W, H, s.Label, []tipRow{{key: s.color, value: f(s.Value) + " · " + sharePct(s.Value, total, o.PctFormat)}}, 12)
		hv.raw(`</g></g>`)
	}
	c.closeHover(&hv)
	return c.html()
}
