package render

import (
	"fmt"
	"html/template"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/flocom/SEO-GEO-Report/internal/charts"
	"github.com/flocom/SEO-GEO-Report/internal/i18n"
	"github.com/flocom/SEO-GEO-Report/internal/insights"
	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// builder carries everything needed to build the sections of one report.
type builder struct {
	r       *model.Report
	lang    string
	opt     model.Options
	an      insights.Analysis
	maxRows int
	hints   bool
	auto    bool
	accent  string
}

func (b *builder) t(key string) string               { return i18n.T(b.lang, key) }
func (b *builder) tf(key string, a ...any) string    { return i18n.Tf(b.lang, key, a...) }
func (b *builder) num(v float64) string              { return i18n.Number(b.lang, v) }
func (b *builder) dec(v float64, d int) string       { return i18n.Decimal(b.lang, v, d) }
func (b *builder) pct(v float64) string              { return i18n.Percent(b.lang, v, 1) }
func (b *builder) pct0(v float64) string             { return i18n.Percent(b.lang, v, 0) }
func (b *builder) pos(v float64) string              { return i18n.Decimal(b.lang, v, 1) }
func (b *builder) dur(v float64) string              { return i18n.Duration(b.lang, v) }
func (b *builder) money(v float64) string            { return i18n.Currency(b.lang, v, b.r.Meta.Currency, -1) }
func (b *builder) date(s string) string              { return i18n.Date(b.lang, s) }
func (b *builder) plural(n float64, k string) string { return i18n.Plural(b.lang, n, k) }

// Chart formatters.
func (b *builder) fmtCompact() charts.Formatter {
	return func(v float64) string { return i18n.Compact(b.lang, v) }
}
func (b *builder) fmtNum() charts.Formatter { return func(v float64) string { return b.num(v) } }
func (b *builder) fmtPct() charts.Formatter { return func(v float64) string { return b.pct(v) } }
func (b *builder) fmtPos() charts.Formatter { return func(v float64) string { return b.pos(v) } }
func (b *builder) fmtMoney() charts.Formatter {
	return func(v float64) string { return b.money(v) }
}

// fmtShare formats a share given in percent for chart legends:
// "6,6 %" / "6.6%" below 10, "42 %" / "42%" above.
func (b *builder) fmtShare() charts.Formatter {
	return func(v float64) string {
		digits := 0
		if math.Abs(v) < 10 {
			digits = 1
		}
		return i18n.Percent(b.lang, v, digits)
	}
}

// fmtDelta formats the relative change shown in chart delta badges:
// "+144 %" / "+144%", "−4,5 %" / "−4.5%"; beyond ±999 % it is capped.
func (b *builder) fmtDelta() charts.Formatter {
	return func(v float64) string {
		switch {
		case v > 999:
			return ">" + i18n.SignedPercent(b.lang, 999, 0)
		case v < -999:
			return "<" + i18n.SignedPercent(b.lang, -999, 0)
		}
		digits := 0
		if math.Abs(v) < 10 {
			digits = 1
		}
		if math.Abs(v) < 0.05 {
			return i18n.Percent(b.lang, 0, 0)
		}
		return i18n.SignedPercent(b.lang, v, digits)
	}
}

// fmtDateTick and fmtDateTip format ISO dates on chart axes ("1 juil.") and
// in chart tooltips ("1 juil. 2026"); other labels are kept as they are.
func (b *builder) fmtDateTick(s string) string { return i18n.ShortDate(b.lang, s) }
func (b *builder) fmtDateTip(s string) string  { return i18n.Date(b.lang, s) }

// hint returns the beginner explanation for key when hints are enabled.
func (b *builder) hint(key string) string {
	if !b.hints {
		return ""
	}
	return b.t("hint." + key)
}

// ---------------------------------------------------------------------------
// Deltas

const flatThreshold = 0.5 // |Δ%| below this is shown as stable

func arrowFor(dir, tone string) template.HTML {
	if a := charts.DeltaArrow(dir, toneName(tone)); a != "" {
		return a
	}
	switch dir {
	case "up":
		return arrowUp
	case "down":
		return arrowDown
	}
	return arrowFlat
}

func toneName(t string) string {
	switch t {
	case "pos":
		return "positive"
	case "neg":
		return "negative"
	}
	return "neutral"
}

func toneFromInsights(t insights.Tone) string {
	switch t {
	case insights.Positive:
		return "pos"
	case insights.Negative:
		return "neg"
	}
	return "neu"
}

// direction returns up/down/flat and the tone of a change.
func direction(change, rel float64, lowerBetter bool) (string, string) {
	if math.Abs(rel) < flatThreshold || change == 0 {
		return "flat", "neu"
	}
	dir := "up"
	if change < 0 {
		dir = "down"
	}
	good := change > 0
	if lowerBetter {
		good = !good
	}
	if good {
		return dir, "pos"
	}
	return dir, "neg"
}

// deltaRel shows a relative change ("+12,3 %") with the absolute change as
// secondary text.
func (b *builder) deltaRel(m model.Metric, lowerBetter bool, abs func(float64) string) *deltaView {
	d, ok := m.Delta()
	if !ok {
		return nil
	}
	rel, okRel := m.DeltaPct()
	if !okRel {
		if d == 0 {
			return nil
		}
		dir, tone := direction(d, 100, lowerBetter)
		return &deltaView{Text: b.t("delta.new"), Sub: signed(abs(d), d), Tone: tone, Dir: dir, Arrow: arrowFor(dir, tone)}
	}
	dir, tone := direction(d, rel, lowerBetter)
	digits := 1
	if math.Abs(rel) >= 100 {
		digits = 0
	}
	dv := &deltaView{Text: i18n.SignedPercent(b.lang, rel, digits), Tone: tone, Dir: dir, Arrow: arrowFor(dir, tone)}
	if abs != nil {
		dv.Sub = signed(abs(d), d)
	}
	return dv
}

// deltaPts shows the change of a percentage in points.
func (b *builder) deltaPts(m model.Metric) *deltaView {
	d, ok := m.Delta()
	if !ok {
		return nil
	}
	// Small rates (CTR) need two decimals to show a meaningful change.
	digits := 1
	if math.Abs(m.Current) < 10 && math.Abs(d) < 1 {
		digits = 2
	}
	rel := 100.0
	if m.Previous != nil && *m.Previous != 0 {
		rel = d / math.Abs(*m.Previous) * 100
	}
	if math.Abs(d) < 0.5*math.Pow(10, -float64(digits)) {
		rel, d = 0, 0
	}
	dir, tone := direction(d, rel, false)
	return &deltaView{Text: i18n.Points(b.lang, d, digits), Tone: tone, Dir: dir, Arrow: arrowFor(dir, tone)}
}

// deltaPosition shows the change of an average position: going down is good.
func (b *builder) deltaPosition(m model.Metric) *deltaView {
	d, ok := m.Delta()
	if !ok {
		return nil
	}
	rel := 100.0
	if math.Abs(d) < 0.05 {
		rel = 0
	}
	dir, tone := direction(d, rel, true)
	return &deltaView{Text: i18n.SignedDecimal(b.lang, d, 1), Tone: tone, Dir: dir, Arrow: arrowFor(dir, tone)}
}

func signed(s string, v float64) string {
	if v > 0 && !strings.HasPrefix(s, "+") {
		return "+" + s
	}
	return s
}

func metricOf(cur float64, prev *float64) model.Metric {
	return model.Metric{Current: cur, Previous: prev}
}

// ---------------------------------------------------------------------------
// Misc helpers

func barStyle(v, max float64) template.CSS {
	if max <= 0 || v <= 0 {
		return template.CSS("width:0")
	}
	p := v / max * 100
	if p < 1.5 {
		p = 1.5
	}
	if p > 100 {
		p = 100
	}
	return template.CSS("width:" + strconv.FormatFloat(p, 'f', 1, 64) + "%")
}

func maxOf(vals ...float64) float64 {
	m := 0.0
	for _, v := range vals {
		if v > m {
			m = v
		}
	}
	return m
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// shortURL turns "https://www.example.com/blog/post/" into "/blog/post/".
func shortURL(s string) string {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil || u.Host == "" {
		return s
	}
	p := u.EscapedPath()
	if dec, err := url.PathUnescape(p); err == nil {
		p = dec
	}
	if p == "" {
		p = "/"
	}
	if u.RawQuery != "" {
		p += "?" + u.RawQuery
	}
	return p
}

var hexColor = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// DefaultAccent is used when meta.brand_color is missing or invalid.
const DefaultAccent = "#2563eb"

// accentColor validates a hex color and expands it to #rrggbb.
func accentColor(c string) string {
	c = strings.TrimSpace(c)
	if c != "" && !strings.HasPrefix(c, "#") {
		c = "#" + c
	}
	if !hexColor.MatchString(c) {
		return DefaultAccent
	}
	if len(c) == 4 {
		c = "#" + strings.Repeat(c[1:2], 2) + strings.Repeat(c[2:3], 2) + strings.Repeat(c[3:4], 2)
	}
	return strings.ToLower(c)
}

func rgb(hex string) (int, int, int) {
	v, _ := strconv.ParseUint(hex[1:], 16, 32)
	return int(v >> 16 & 0xff), int(v >> 8 & 0xff), int(v & 0xff)
}

// themeCSS derives the accent variables from a validated #rrggbb color.
func themeCSS(accent string) template.CSS {
	r, g, bl := rgb(accent)
	// Relative luminance decides the ink color on accent backgrounds and
	// darkens very light brand colors used as text.
	lum := (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(bl)) / 255
	on := "#ffffff"
	if lum > 0.62 {
		on = "#0f172a"
	}
	ink := accent
	if lum > 0.55 {
		f := 0.55 / lum
		ink = fmt.Sprintf("#%02x%02x%02x", int(float64(r)*f), int(float64(g)*f), int(float64(bl)*f))
	}
	return template.CSS(fmt.Sprintf(
		":root{--accent:%s;--accent-ink:%s;--accent-on:%s;--accent-rgb:%d,%d,%d;"+
			"--accent-soft:rgba(%d,%d,%d,.09);--accent-line:rgba(%d,%d,%d,.28)}",
		accent, ink, on, r, g, bl, r, g, bl, r, g, bl))
}

var dataImage = regexp.MustCompile(`^data:image/(png|jpe?g|gif|webp|svg\+xml);base64,[A-Za-z0-9+/=\s]+$`)

// logoURL accepts https URLs and base64 data: image URIs.
func logoURL(s string) template.URL {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if dataImage.MatchString(s) {
		return template.URL(s) //nolint:gosec // validated base64 image
	}
	if u, err := url.Parse(s); err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" {
		return template.URL(u.String()) //nolint:gosec // http(s) only
	}
	return ""
}

var dataFavicon = regexp.MustCompile(`^data:image/(png|x-icon|vnd\.microsoft\.icon|svg\+xml|gif|jpeg|webp);base64,[A-Za-z0-9+/=\s]+$`)

// faviconURL accepts https URLs and base64 data: image URIs; "none" disables.
func faviconURL(s string) template.URL {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "none") {
		return ""
	}
	if dataFavicon.MatchString(s) {
		return template.URL(s) //nolint:gosec // validated base64 image
	}
	if u, err := url.Parse(s); err == nil && u.Scheme == "https" && u.Host != "" {
		return template.URL(u.String()) //nolint:gosec // https only
	}
	return ""
}

func siteHref(s string) template.URL {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	if u, err := url.Parse(s); err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" {
		return template.URL(u.String()) //nolint:gosec // http(s) only
	}
	return ""
}

func initials(name string) string {
	var out []rune
	for _, w := range strings.Fields(name) {
		r := []rune(w)
		if len(r) > 0 {
			out = append(out, r[0])
		}
		if len(out) == 2 {
			break
		}
	}
	if len(out) == 0 {
		return "•"
	}
	return strings.ToUpper(string(out))
}

// sortedCopy returns rows sorted by key desc without touching the input.
func sortedCopy[T any](rows []T, key func(T) float64) []T {
	out := append([]T(nil), rows...)
	sort.SliceStable(out, func(i, j int) bool { return key(out[i]) > key(out[j]) })
	return out
}

func topN[T any](rows []T, n int) []T {
	if len(rows) > n {
		return rows[:n]
	}
	return rows
}

// limitRows truncates a table to maxRows and records the truncation note.
func (b *builder) limit(t *table, total int) *table {
	if len(t.Rows) > b.maxRows {
		t.Rows = t.Rows[:b.maxRows]
	}
	if total > len(t.Rows) {
		t.Truncated = b.tf("table.truncated", b.num(float64(len(t.Rows))), b.num(float64(total)))
	}
	return t
}

// series helpers for charts.
func alignPrev(cur int, prev []float64) []float64 {
	if len(prev) == 0 {
		return nil
	}
	out := make([]float64, cur)
	for i := range out {
		if i < len(prev) {
			out[i] = prev[i]
		} else {
			out[i] = math.NaN()
		}
	}
	return out
}

func nonZero(vals []float64) bool {
	for _, v := range vals {
		if v != 0 && !math.IsNaN(v) {
			return true
		}
	}
	return false
}

// colorAI identifies AI-assistant traffic in multi-series charts.
const colorAI = "#f59e0b"

func pickColor(i int) string { return charts.Palette[i%len(charts.Palette)] }

// seriesColor is the brand color for the first series, then the palette.
func (b *builder) seriesColor(i int) string {
	if i == 0 {
		return b.accent
	}
	return pickColor(i)
}

// columns renders charts.Columns in the brand color with localized deltas.
func (b *builder) columns(o charts.ColumnsOpts) template.HTML {
	if o.Color == "" {
		o.Color = b.accent
	}
	if o.DeltaFormat == nil {
		o.DeltaFormat = b.fmtDelta()
	}
	return charts.Columns(o)
}

func boolPtr(b *bool) bool { return b != nil && *b }

// Chart wrappers that apply the report locale to the numbers, percentages and
// dates the charts print themselves.
func (b *builder) bars(o charts.BarsOpts) template.HTML {
	if o.DeltaFormat == nil {
		o.DeltaFormat = b.fmtDelta()
	}
	return charts.Bars(o)
}

func (b *builder) donut(o charts.DonutOpts) template.HTML {
	if o.PctFormat == nil {
		o.PctFormat = b.fmtShare()
	}
	return charts.Donut(o)
}

func (b *builder) stacked(o charts.StackedOpts) template.HTML {
	if o.PctFormat == nil {
		o.PctFormat = b.fmtShare()
	}
	return charts.Stacked(o)
}

func (b *builder) gauge(o charts.GaugeOpts) template.HTML {
	if o.Format == nil {
		o.Format = b.fmtCompact()
	}
	return charts.Gauge(o)
}
