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
	CenterValue string // big text in the hole
	CenterLabel string // small text under it
	Size        float64 // default 220 (ring), legend extends width
	Format      Formatter
	Title       string
}

// GaugeOpts configures a semicircular gauge.
type GaugeOpts struct {
	Value    float64 // within [Min, Max]
	Min, Max float64 // default 0..100
	Label    string
	Display  string // text shown instead of the raw value
	Bands    []GaugeBand // colored zones; default red/orange/green thirds
	Title    string
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
}

// SparkOpts configures a tiny trend line for KPI cards.
type SparkOpts struct {
	Values        []float64
	Color         string
	Width, Height float64 // default 120x36
	InvertY       bool
}

// DeltaArrow returns a small inline SVG arrow (up, down or flat) colored by
// tone: "positive", "negative" or "neutral".
func DeltaArrow(direction, tone string) template.HTML { return "" }

// Line renders a multi-series line/area chart with axes, gridlines and legend.
func Line(o LineOpts) template.HTML { return "" }

// Bars renders labelled horizontal bars sorted as given.
func Bars(o BarsOpts) template.HTML { return "" }

// Columns renders vertical grouped columns (current vs previous).
func Columns(o ColumnsOpts) template.HTML { return "" }

// Donut renders a donut chart with legend and percentages.
func Donut(o DonutOpts) template.HTML { return "" }

// Gauge renders a semicircular gauge.
func Gauge(o GaugeOpts) template.HTML { return "" }

// Stacked renders a 100% stacked horizontal bar with legend.
func Stacked(o StackedOpts) template.HTML { return "" }

// Sparkline renders a minimal trend line without axes.
func Sparkline(o SparkOpts) template.HTML { return "" }

// Palette is the default categorical palette (colorblind-friendly order).
var Palette = []string{"#2563eb", "#f59e0b", "#10b981", "#ef4444", "#8b5cf6", "#06b6d4", "#ec4899", "#84cc16"}

// Semantic colors.
const (
	ColorPositive = "#16a34a"
	ColorNegative = "#dc2626"
	ColorNeutral  = "#64748b"
	ColorPrevious = "#94a3b8"
)
