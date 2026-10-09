// Package charts renders self-contained, dependency-free SVG charts.
//
// Every function returns inline SVG markup (template.HTML) that scales to its
// container width (viewBox + width="100%"), prints well, and works without any
// JavaScript. Colors default to the package palette; callers may override them.
package charts

import "html/template"

// Formatter formats a numeric value for axis ticks, labels and tooltips.
type Formatter func(float64) string

// Series is one line in a line chart.
type Series struct {
	Name   string
	Values []float64 // NaN = gap
	Color  string    // CSS color, empty = palette
	Dashed bool      // e.g. comparison period
	Area   bool      // fill under the line
}

// LineOpts configures Line.
type LineOpts struct {
	Labels  []string // x labels (typically dates YYYY-MM-DD); thinned automatically
	Series  []Series
	Width   float64 // viewBox width, default 720
	Height  float64 // viewBox height, default 260
	YFormat Formatter
	InvertY bool // lower is better (average position): axis reversed
	YMin    *float64
	Title   string // accessible title (<title>)

	// Optional locale hooks (nil = built-in, locale-neutral behaviour).
	XFormat        func(label string) string // x tick label; default: YYYY-MM-DD -> "DD/MM"
	TooltipXFormat func(label string) string // x label in tooltips; default: "DD/MM/YYYY"
}

// BarItem is one bar.
type BarItem struct {
	Label    string
	Value    float64
	Previous *float64 // optional ghost bar / delta
	Color    string
}

// BarsOpts configures horizontal bars (rankings, channels, sources).
type BarsOpts struct {
	Items     []BarItem
	Width     float64 // default 720
	Format    Formatter
	ShowDelta bool // show +x% badge when Previous is set
	Title     string

	// DeltaFormat formats the delta badge from a relative change in percent
	// (12.3 -> "+12 %"). nil = FormatDelta.
	DeltaFormat Formatter
	// Height is an optional minimum viewBox height: rows are spaced out (up
	// to a comfortable limit) to fill it, e.g. to match a neighbouring card.
	// 0 = compact height from the number of rows.
	Height float64
	// PreviousLabel names the previous-period value in hover tooltips
	// (e.g. "Période précédente"). Empty = "vs".
	PreviousLabel string
}

// ColumnsOpts configures vertical grouped columns comparing two periods.
type ColumnsOpts struct {
	Labels        []string
	Current       []float64
	Previous      []float64 // optional, same length as Current
	CurrentName   string
	PreviousName  string
	Width, Height float64
	Format        Formatter
	Title         string

	// Optional (zero value = defaults).
	Color         string    // current-period columns; default Palette[0]
	PreviousColor string    // previous-period columns; default ColorPrevious
	DeltaFormat   Formatter // relative change in tooltips; nil = FormatDelta
}

// Slice is one donut slice.
type Slice struct {
	Label string
	Value float64
	Color string
}

// DonutOpts configures Donut. The legend is rendered next to the ring.
type DonutOpts struct {
	Slices      []Slice
	CenterValue string  // big text in the hole
	CenterLabel string  // small text under it
	Size        float64 // default 220 (ring), legend extends width
	Format      Formatter
	Title       string

	// PctFormat formats legend shares, given in percent (6.6 -> "6,6 %").
	// nil = FormatPct.
	PctFormat Formatter
}

// GaugeOpts configures a semicircular gauge.
type GaugeOpts struct {
	Value    float64 // within [Min, Max]
	Min, Max float64 // default 0..100
	Label    string
	Display  string      // text shown instead of the raw value
	Bands    []GaugeBand // colored zones; default red/orange/green thirds
	Title    string

	// Format formats the min/max labels and the value when Display is empty.
	// nil = FormatCompact.
	Format Formatter
}

// GaugeBand is a colored zone of a gauge.
type GaugeBand struct {
	Upto  float64
	Color string
}

// StackSegment is one segment of a 100% stacked bar.
type StackSegment struct {
	Label string
	Value float64
	Color string
}

// StackedOpts configures a 100% horizontal stacked bar (CWV, position buckets).
type StackedOpts struct {
	Segments []StackSegment
	Width    float64 // default 720
	Format   Formatter
	Title    string

	// PctFormat formats shares, given in percent (6.6 -> "6,6 %").
	// nil = FormatPct.
	PctFormat Formatter
}

// SparkOpts configures a tiny trend line for KPI cards.
type SparkOpts struct {
	Values        []float64
	Color         string
	Width, Height float64 // default 120x36
	InvertY       bool

	// Format formats the first/last values in the accessible title and the
	// hover tooltip. nil = FormatCompact.
	Format Formatter
	// Labels optionally names each point (typically YYYY-MM-DD dates) for
	// the hover tooltip; LabelFormat formats them (nil = "DD/MM/YYYY").
	Labels      []string
	LabelFormat func(label string) string
}

// DeltaArrow returns a small inline SVG arrow (up, down or flat) colored by
// tone: "positive", "negative" or "neutral".
func DeltaArrow(direction, tone string) template.HTML { return deltaArrow(direction, tone) }

// Line renders a multi-series line/area chart with axes, gridlines and legend.
func Line(o LineOpts) template.HTML { return line(o) }

// Bars renders labelled horizontal bars sorted as given.
func Bars(o BarsOpts) template.HTML { return bars(o) }

// Columns renders vertical grouped columns (current vs previous).
func Columns(o ColumnsOpts) template.HTML { return columns(o) }

// Donut renders a donut chart with legend and percentages.
func Donut(o DonutOpts) template.HTML { return donut(o) }

// Gauge renders a semicircular gauge.
func Gauge(o GaugeOpts) template.HTML { return gauge(o) }

// Stacked renders a 100% stacked horizontal bar with legend.
func Stacked(o StackedOpts) template.HTML { return stacked(o) }

// Sparkline renders a minimal trend line without axes.
func Sparkline(o SparkOpts) template.HTML { return sparkline(o) }

// Palette is the default categorical palette (colorblind-friendly order).
var Palette = []string{"#2f5f96", "#cf8a2a", "#26927a", "#c2533c", "#7a5aa6", "#3b97bd", "#bf5b88", "#87993a"}

// Semantic colors.
const (
	ColorPositive = "#2f8a57"
	ColorNegative = "#c2412d"
	ColorNeutral  = "#6b7280"
	ColorPrevious = "#a3a9b2"
	ColorWarning  = "#cf8a2a"
)
