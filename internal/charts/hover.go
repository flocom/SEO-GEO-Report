package charts

import (
	"html/template"
	"math"
	"strconv"
	"strings"
)

// Hover layer.
//
// Interactivity is pure CSS: each chart ends with a <style> element followed
// by a <g class="sgc-hover"> layer holding one hover zone per data point.
// A zone (<g class="sgc-hz">) contains an invisible hit area and a hidden
// tooltip (<g class="sgc-tip">) revealed by :hover. No JavaScript is needed,
// so charts stay interactive in a downloaded standalone HTML file, and the
// layer is hidden when printing.
//
// The CSS is identical in every chart (class selectors only), so repeating
// it in each inline SVG is harmless. Tooltip colors can be themed by the host
// page through --chart-tooltip-bg and --chart-tooltip-fg.
const hoverCSS = `.sgc-hit{fill:#000;fill-opacity:0}` +
	`.sgc-hits{fill:none;stroke:#000;stroke-opacity:0;pointer-events:stroke}` +
	`.sgc-tip{opacity:0;pointer-events:none;transition:opacity .12s ease-out}` +
	`.sgc-hz:hover .sgc-tip{opacity:1}` +
	`.sgc-box{fill:var(--chart-tooltip-bg,#111827);fill-opacity:.96;filter:drop-shadow(0 2px 5px rgba(0,0,0,.22))}` +
	`.sgc-tip .t{fill:var(--chart-tooltip-fg,#f9fafb)}` +
	`.sgc-tip .m{fill:var(--chart-tooltip-fg,#f9fafb);fill-opacity:.72}` +
	`.sgc-tip .b{font-weight:600}.sgc-tip .e{text-anchor:end}` +
	`.sgc-dot{stroke:var(--chart-surface,#fff);stroke-width:2}` +
	`.sgc-guide{stroke:currentColor;stroke-opacity:.35;stroke-width:1}` +
	`.sgc-mark{transition:opacity .12s ease-out}` +
	`.sgc:has(.sgc-hz:hover) .sgc-mark{opacity:.4}` +
	`.sgc-hl{fill:currentColor;fill-opacity:.06}` +
	`@media print{.sgc-hover{display:none}}` +
	`@media (prefers-reduced-motion:reduce){.sgc-tip,.sgc-mark{transition:none}}`

// hoverStart marks the beginning of the interactive part of a chart; Static
// cuts the markup there.
const hoverStart = `<style>`

// closeHover writes the hover layer (if any) and closes the root element.
func (c *canvas) closeHover(hv *canvas) {
	if hv != nil && hv.b.Len() > 0 {
		c.raw(hoverStart + hoverCSS + `</style><g class="sgc-hover">`)
		c.raw(hv.b.String())
		c.raw(`</g>`)
	}
	c.close()
}

// Static returns svg without its hover layer and stylesheet: use it for
// print-only or duplicate variants of a chart to keep documents light.
func Static(svg template.HTML) template.HTML {
	s := string(svg)
	i := strings.Index(s, hoverStart)
	if i < 0 {
		return svg
	}
	return template.HTML(s[:i] + "</svg>")
}

// tipRow is one line of a tooltip: an optional color key, a name and a
// value (right-aligned when both are present).
type tipRow struct {
	key   string // CSS color of the key dot, empty = none
	name  string
	value string
	muted bool
}

// tooltip draws a tooltip box next to the anchor point (ax, ay), flipping to
// the left of the anchor near the right edge and clamping vertically so that
// it always stays inside the [0, W] × [0, H] viewBox.
func tooltip(c *canvas, ax, ay, W, H float64, head string, rows []tipRow, size float64) {
	pad := size * 0.75
	lh := size * 1.45
	keyW := size * 0.95

	headW := 0.0
	if head != "" {
		head, _ = truncate(head, W*0.8-2*pad, size)
		headW = textWidth(head, size) * 1.06 // semibold
	}
	maxW := W*0.8 - 2*pad
	contentW := headW
	for i, r := range rows {
		w := 0.0
		if r.key != "" {
			w += keyW
		}
		nameW := textWidth(r.name, size)
		valW := textWidth(r.value, size) * 1.06
		gap := 0.0
		if r.name != "" && r.value != "" {
			gap = size * 1.2
		}
		if w+nameW+gap+valW > maxW && r.name != "" {
			rows[i].name, _ = truncate(r.name, math.Max(maxW-w-gap-valW, size*3), size)
			nameW = textWidth(rows[i].name, size)
		}
		contentW = math.Max(contentW, w+nameW+gap+valW)
	}
	bw := contentW + 2*pad
	nl := len(rows)
	if head != "" {
		nl++
	}
	bh := float64(nl)*lh + pad*1.2

	off := size * 0.9
	x := ax + off
	if x+bw > W-1 {
		x = ax - off - bw
	}
	x = clampF(x, 1, math.Max(1, W-bw-1))
	y := clampF(ay-bh/2, 1, math.Max(1, H-bh-1))

	c.printf(`<g transform="translate(%s %s)" font-size="%s"><rect class="sgc-box" width="%s" height="%s" rx="%s"/>`, n1(x), n1(y), n1(size), n1(bw), n1(bh), n1(size*0.45))
	ty := pad*0.6 + lh*0.72
	if head != "" {
		c.printf(`<text class="t b" x="%s" y="%s">%s</text>`, n1(pad), n1(ty), esc(head))
		ty += lh
	}
	for _, r := range rows {
		tx := pad
		if r.key != "" {
			c.printf(`<circle cx="%s" cy="%s" r="%s" fill="%s"/>`, n1(tx+size*0.32), n1(ty-size*0.33), n1(size*0.3), esc(r.key))
			tx += keyW
		}
		cls := "t"
		if r.muted {
			cls = "m"
		}
		if r.name != "" {
			nc := "m"
			if r.value == "" {
				nc = cls
			}
			c.printf(`<text class="%s" x="%s" y="%s">%s</text>`, nc, n1(tx), n1(ty), esc(r.name))
		}
		if r.value != "" {
			if r.name != "" {
				c.printf(`<text class="%s b e" x="%s" y="%s">%s</text>`, cls, n1(bw-pad), n1(ty), esc(r.value))
			} else {
				c.printf(`<text class="%s b" x="%s" y="%s">%s</text>`, cls, n1(tx), n1(ty), esc(r.value))
			}
		}
		ty += lh
	}
	c.raw(`</g>`)
}

// n1 formats a hover-layer coordinate with one decimal (lighter markup).
func n1(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "0"
	}
	v = math.Round(v*10) / 10
	if v == 0 {
		return "0"
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}
