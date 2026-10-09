package charts

import (
	"fmt"
	"html/template"
	"math"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"
)

// Text inks. Text never wears a series color: it inherits the host page's
// text color (currentColor) at a few fixed emphasis levels.
const (
	inkPrimary   = `fill="currentColor"`
	inkSecondary = `fill="currentColor" fill-opacity="0.66"`
	inkMuted     = `fill="currentColor" fill-opacity="0.5"`
	gridStroke   = `stroke="currentColor" stroke-opacity="0.1"`
	axisStroke   = `stroke="currentColor" stroke-opacity="0.24"`
	// surfaceStroke draws the 2px "surface ring" around markers. The host page
	// may theme it through the --chart-surface custom property.
	surfaceStroke = `style="stroke:var(--chart-surface,#fff)" stroke-width="2"`
	// Default text color on badges.
	badgePositiveText = "#15803d"
	badgeNegativeText = "#b91c1c"
	badgeNeutralText  = "#475569"
)

// clampAbs bounds magnitudes so that differences never overflow to ±Inf.
const clampAbs = 1e300

var idSeq atomic.Uint64

// newID returns a document-unique id prefix for gradients and clip paths, so
// several charts can be inlined in the same HTML page without collisions.
func newID(kind string) string {
	return "sgc-" + kind + "-" + strconv.FormatUint(idSeq.Add(1), 36)
}

// canvas accumulates SVG markup.
type canvas struct{ b strings.Builder }

func (c *canvas) printf(format string, a ...any) { fmt.Fprintf(&c.b, format, a...) }
func (c *canvas) raw(s string)                   { c.b.WriteString(s) }
func (c *canvas) html() template.HTML            { return template.HTML(c.b.String()) }

// open writes the root <svg> element and its accessible <title>.
func (c *canvas) open(kind string, w, h float64, title string) { c.openCapped(kind, w, h, title, 0) }

// openCapped is open with a CSS max-width (px) for intrinsically small charts
// (gauge, donut) that would look oversized when stretched to a wide container.
func (c *canvas) openCapped(kind string, w, h float64, title string, maxW float64) {
	c.printf(`<svg xmlns="http://www.w3.org/2000/svg" class="sgc sgc-%s" viewBox="0 0 %s %s" width="100%%" preserveAspectRatio="xMidYMid meet" role="img"`, kind, n(w), n(h))
	if title != "" {
		c.printf(` aria-label="%s"`, esc(title))
	}
	if maxW > 0 {
		c.printf(` font-family="inherit" style="display:block;height:auto;max-width:min(100%%,%spx);margin:0 auto">`, n(maxW))
	} else {
		c.raw(` font-family="inherit" style="display:block;height:auto;max-width:100%">`)
	}
	if title != "" {
		c.printf(`<title>%s</title>`, esc(title))
	}
}

func (c *canvas) close() { c.raw(`</svg>`) }

// text writes a single <text> element. attrs is inserted verbatim (trusted).
func (c *canvas) text(x, y float64, s string, size float64, anchor, attrs string) {
	c.printf(`<text x="%s" y="%s" font-size="%s"`, n(x), n(y), n(size))
	if anchor != "" && anchor != "start" {
		c.printf(` text-anchor="%s"`, anchor)
	}
	if attrs != "" {
		c.raw(" " + attrs)
	}
	c.printf(`>%s</text>`, esc(s))
}

// n formats a coordinate compactly (max 2 decimals). Non-finite values are
// rendered as 0 so the SVG never contains NaN or Inf.
func n(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "0"
	}
	v = math.Round(v*100) / 100
	if v == 0 {
		return "0" // also normalizes -0
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// clampVal bounds a finite value to ±clampAbs.
func clampVal(v float64) float64 {
	if v > clampAbs {
		return clampAbs
	}
	if v < -clampAbs {
		return -clampAbs
	}
	return v
}

func def(v, d float64) float64 {
	if !finite(v) || v <= 0 {
		return d
	}
	return v
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// esc XML-escapes s for use in text content and attribute values. Invalid
// UTF-8 and characters that are not allowed in XML 1.0 are dropped.
func esc(s string) string {
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "�")
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	for _, r := range s {
		switch {
		case r == '&':
			b.WriteString("&amp;")
		case r == '<':
			b.WriteString("&lt;")
		case r == '>':
			b.WriteString("&gt;")
		case r == '"':
			b.WriteString("&quot;")
		case r == '\'':
			b.WriteString("&#39;")
		case r == '\t' || r == '\n' || r == '\r':
			b.WriteByte(' ')
		case r < 0x20, r >= 0xD800 && r <= 0xDFFF, r == 0xFFFE, r == 0xFFFF:
			// not allowed in XML: drop
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// runeWidth approximates the advance width of r in em for a typical
// sans-serif UI font. Good enough to avoid collisions and decide truncation.
func runeWidth(r rune) float64 {
	switch {
	case r == ' ':
		return 0.28
	case strings.ContainsRune("il|!.,:;'`ıìíîï", r):
		return 0.27
	case strings.ContainsRune("fjrtI()[]{}/\\-", r):
		return 0.36
	case strings.ContainsRune("mwMW@%", r):
		return 0.86
	case r >= '0' && r <= '9':
		return 0.57
	case r >= 0x1100 && (unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) ||
		unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || r >= 0x1F300):
		return 1.0
	case unicode.IsUpper(r):
		return 0.66
	default:
		return 0.53
	}
}

// textWidth estimates the rendered width of s at the given font size.
func textWidth(s string, size float64) float64 {
	w := 0.0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w * size
}

// truncate shortens s with an ellipsis so it fits in maxW at the given size.
// It reports whether s was shortened.
func truncate(s string, maxW, size float64) (string, bool) {
	if textWidth(s, size) <= maxW {
		return s, false
	}
	budget := maxW - textWidth("…", size)
	if budget <= 0 {
		return "…", true
	}
	w := 0.0
	for i, r := range s {
		w += runeWidth(r) * size
		if w > budget {
			return strings.TrimRight(s[:i], " ") + "…", true
		}
	}
	return s, false
}

// labelText writes a possibly truncated label; when it was shortened the full
// label is kept in a <title> child so it is available as a tooltip.
func (c *canvas) labelText(x, y float64, s string, maxW, size float64, anchor, attrs string) {
	t, cut := truncate(s, maxW, size)
	if !cut {
		c.text(x, y, t, size, anchor, attrs)
		return
	}
	c.printf(`<text x="%s" y="%s" font-size="%s"`, n(x), n(y), n(size))
	if anchor != "" && anchor != "start" {
		c.printf(` text-anchor="%s"`, anchor)
	}
	if attrs != "" {
		c.raw(" " + attrs)
	}
	c.printf(`><title>%s</title>%s</text>`, esc(s), esc(t))
}

// niceNum returns a "nice" number (1, 2, 5 × 10^k) approximately equal to x.
func niceNum(x float64, round bool) float64 {
	if !finite(x) || x <= 0 {
		return 1
	}
	exp := math.Floor(math.Log10(x))
	p := math.Pow(10, exp)
	f := x / p
	var nf float64
	if round {
		switch {
		case f < 1.5:
			nf = 1
		case f < 3:
			nf = 2
		case f < 7:
			nf = 5
		default:
			nf = 10
		}
	} else {
		switch {
		case f <= 1:
			nf = 1
		case f <= 2:
			nf = 2
		case f <= 5:
			nf = 5
		default:
			nf = 10
		}
	}
	return nf * p
}

// niceScale expands [lo, hi] to round bounds with a 1-2-5 step.
func niceScale(lo, hi float64, maxTicks int) (float64, float64, float64) {
	if !finite(lo) || !finite(hi) {
		return 0, 1, 0.25
	}
	if hi < lo {
		lo, hi = hi, lo
	}
	if hi == lo {
		if hi == 0 {
			hi = 1
		} else {
			d := math.Abs(hi) * 0.1
			lo, hi = lo-d, hi+d
		}
	}
	if maxTicks < 2 {
		maxTicks = 2
	}
	rng := niceNum(hi-lo, false)
	step := niceNum(rng/float64(maxTicks-1), true)
	nlo := math.Floor(lo/step) * step
	nhi := math.Ceil(hi/step) * step
	if !finite(nlo) || !finite(nhi) || nhi <= nlo {
		return lo, hi, hi - lo
	}
	return nlo, nhi, step
}

// roundStep removes floating point noise from v given the tick step.
func roundStep(v, step float64) float64 {
	if step <= 0 || !finite(step) {
		return v
	}
	d := -math.Floor(math.Log10(step)) + 2
	if d < 0 {
		d = 0
	}
	if d > 12 {
		d = 12
	}
	p := math.Pow(10, d)
	r := math.Round(v*p) / p
	if !finite(r) {
		return v
	}
	if r == 0 {
		return 0
	}
	return r
}

// buildTicks returns lo, every multiple of step in (lo, hi), and hi. Multiples
// too close to lo are dropped so labels never collide.
func buildTicks(lo, hi, step float64) []float64 {
	if !(step > 0) || !finite(step) || hi <= lo {
		return []float64{lo}
	}
	out := []float64{roundStep(lo, step)}
	first := math.Ceil(lo/step) * step
	for i := 0; i < 60; i++ {
		v := roundStep(first+float64(i)*step, step)
		if v > hi+step*1e-9 {
			break
		}
		if v-lo < step*0.5 {
			continue
		}
		out = append(out, v)
	}
	return out
}

// shortLabel shortens ISO dates for axes: 2024-03-12 / 20240312 -> "12/03",
// 2024-03 -> "03/2024". Other labels are returned unchanged.
func shortLabel(s string) string {
	if t, ok := parseDay(s); ok {
		return t.Format("02/01")
	}
	if len(s) == 7 && s[4] == '-' {
		if t, err := time.Parse("2006-01", s); err == nil {
			return t.Format("01/2006")
		}
	}
	return s
}

// longLabel formats ISO dates fully for tooltips: "12/03/2024".
func longLabel(s string) string {
	if t, ok := parseDay(s); ok {
		return t.Format("02/01/2006")
	}
	return shortLabel(s)
}

func parseDay(s string) (time.Time, bool) {
	switch len(s) {
	case 10:
		if s[4] == '-' && s[7] == '-' {
			if t, err := time.Parse("2006-01-02", s); err == nil {
				return t, true
			}
		}
	case 8:
		for _, r := range s {
			if r < '0' || r > '9' {
				return time.Time{}, false
			}
		}
		if t, err := time.Parse("20060102", s); err == nil && t.Year() > 1900 {
			return t, true
		}
	}
	return time.Time{}, false
}

// seriesColor returns c, or the palette color for index i when c is empty.
func seriesColor(c string, i int) string {
	if strings.TrimSpace(c) != "" {
		return c
	}
	if i < 0 {
		i = 0
	}
	return Palette[i%len(Palette)]
}

// isLight reports whether a hex color is light enough to need dark text.
func isLight(c string) bool {
	c = strings.TrimPrefix(strings.TrimSpace(c), "#")
	if len(c) == 3 {
		c = string([]byte{c[0], c[0], c[1], c[1], c[2], c[2]})
	}
	if len(c) != 6 {
		return false
	}
	v, err := strconv.ParseUint(c, 16, 32)
	if err != nil {
		return false
	}
	lin := func(x uint64) float64 {
		f := float64(x) / 255
		if f <= 0.03928 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	l := 0.2126*lin(v>>16&0xff) + 0.7152*lin(v>>8&0xff) + 0.0722*lin(v&0xff)
	// Contrast with white vs black; pick the better one.
	return (l+0.05)/0.05 > 1.05/(l+0.05)
}

// hBarPath draws a horizontal bar from x0 (baseline, square) to x1 (data
// end, rounded corners of radius r).
func hBarPath(x0, x1, y, h, r float64) string {
	w := math.Abs(x1 - x0)
	if w < 0.01 {
		return ""
	}
	r = math.Min(r, math.Min(w, h/2))
	if x1 >= x0 {
		return fmt.Sprintf("M%s %sH%sQ%s %s %s %sV%sQ%s %s %s %sH%sZ",
			n(x0), n(y), n(x1-r), n(x1), n(y), n(x1), n(y+r), n(y+h-r), n(x1), n(y+h), n(x1-r), n(y+h), n(x0))
	}
	return fmt.Sprintf("M%s %sH%sQ%s %s %s %sV%sQ%s %s %s %sH%sZ",
		n(x0), n(y), n(x1+r), n(x1), n(y), n(x1), n(y+r), n(y+h-r), n(x1), n(y+h), n(x1+r), n(y+h), n(x0))
}

// vBarPath draws a vertical column from y0 (baseline, square) to y1 (data
// end, rounded corners of radius r).
func vBarPath(x, w, y0, y1, r float64) string {
	h := math.Abs(y1 - y0)
	if h < 0.01 {
		return ""
	}
	r = math.Min(r, math.Min(h, w/2))
	if y1 <= y0 {
		return fmt.Sprintf("M%s %sV%sQ%s %s %s %sH%sQ%s %s %s %sV%sZ",
			n(x), n(y0), n(y1+r), n(x), n(y1), n(x+r), n(y1), n(x+w-r), n(x+w), n(y1), n(x+w), n(y1+r), n(y0))
	}
	return fmt.Sprintf("M%s %sV%sQ%s %s %s %sH%sQ%s %s %s %sV%sZ",
		n(x), n(y0), n(y1-r), n(x), n(y1), n(x+r), n(y1), n(x+w-r), n(x+w), n(y1), n(x+w), n(y1-r), n(y0))
}

// pickAxisLabels selects which of count evenly spaced positions (dx apart)
// get a label of width labelW so that labels never collide. The first and the
// last positions are always labelled.
func pickAxisLabels(count int, dx, labelW float64) []int {
	if count <= 0 {
		return nil
	}
	if count == 1 {
		return []int{0}
	}
	need := labelW + 8
	step := 1
	if dx > 0 {
		step = int(math.Ceil(need / dx))
	} else {
		step = count
	}
	if step < 1 {
		step = 1
	}
	var out []int
	for i := 0; i < count; i += step {
		out = append(out, i)
	}
	last := count - 1
	if out[len(out)-1] != last {
		if float64(last-out[len(out)-1])*dx >= need {
			out = append(out, last)
		} else if len(out) > 1 {
			out[len(out)-1] = last
		}
	}
	return out
}

// EmptyState returns a neutral placeholder SVG of the given viewBox size with
// an optional (localized) message. Charts use it, without message, when they
// have nothing to draw; callers can use it directly with their own wording.
func EmptyState(width, height float64, message string) template.HTML {
	return emptySVG("empty", def(width, 720), def(height, 200), "", message)
}

func emptySVG(kind string, w, h float64, title, message string) template.HTML {
	if h < 48 {
		h = 48
	}
	var c canvas
	c.open(kind+" sgc-empty", w, h, title)
	c.printf(`<rect x="1" y="1" width="%s" height="%s" rx="8" fill="currentColor" fill-opacity="0.03" stroke="currentColor" stroke-opacity="0.18" stroke-dasharray="4 4"/>`, n(w-2), n(h-2))
	if message == "" {
		message = "—"
	}
	size := 13.0
	c.labelText(w/2, h/2+size*0.35, message, w-24, size, "middle", inkMuted)
	c.close()
	return c.html()
}
