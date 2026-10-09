package charts

import (
	"encoding/xml"
	"fmt"
	"html/template"
	"io"
	"math"
	"os"
	"strings"
	"testing"
)

var nan = math.NaN()

func fp(v float64) *float64 { return &v }

// wellFormed parses s as XML and fails the test on error. It also checks the
// output never contains NaN/Inf coordinates.
func wellFormed(t *testing.T, name string, s template.HTML) {
	t.Helper()
	str := string(s)
	if str == "" {
		t.Fatalf("%s: empty output", name)
	}
	if !strings.HasPrefix(str, "<svg") {
		t.Fatalf("%s: does not start with <svg: %.60s", name, str)
	}
	d := xml.NewDecoder(strings.NewReader(str))
	d.Strict = true
	depth, roots := 0, 0
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("%s: invalid XML: %v\n%s", name, err, str)
		}
		switch tok.(type) {
		case xml.StartElement:
			if depth == 0 {
				roots++
			}
			depth++
		case xml.EndElement:
			depth--
		}
	}
	if roots != 1 || depth != 0 {
		t.Fatalf("%s: expected a single root element, got %d (depth %d)", name, roots, depth)
	}
	for _, bad := range []string{"NaN", "Inf", "%!"} {
		if strings.Contains(str, bad) {
			t.Fatalf("%s: output contains %q\n%s", name, bad, str)
		}
	}
	if !strings.Contains(str, `role="img"`) {
		t.Fatalf("%s: missing role=img", name)
	}
}

func seq(n int, f func(i int) float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = f(i)
	}
	return out
}

func dates(n int) []string {
	out := make([]string, n)
	for i := range out {
		d := 1 + i%28
		m := 1 + (i/28)%12
		out[i] = "2024-" + two(m) + "-" + two(d)
	}
	return out
}

func two(v int) string {
	if v < 10 {
		return "0" + string(rune('0'+v))
	}
	return string(rune('0'+v/10)) + string(rune('0'+v%10))
}

const evil = `<script>alert("x")</script> & 'q' ]]> ` + "\x00\x1b\xff\ufffe end"

func TestLine(t *testing.T) {
	cases := []struct {
		name string
		o    LineOpts
	}{
		{"empty", LineOpts{}},
		{"no-values", LineOpts{Labels: dates(5), Series: []Series{{Name: "a"}}}},
		{"all-nan", LineOpts{Labels: dates(3), Series: []Series{{Values: []float64{nan, nan, nan}}}}},
		{"single-point", LineOpts{Labels: []string{"2024-03-12"}, Series: []Series{{Name: "Clicks", Values: []float64{42}, Area: true}}}},
		{"all-zeros", LineOpts{Labels: dates(10), Series: []Series{{Values: make([]float64, 10), Area: true}}}},
		{"constant", LineOpts{Labels: dates(10), Series: []Series{{Values: seq(10, func(int) float64 { return 7 })}}}},
		{"negative", LineOpts{Labels: dates(20), Series: []Series{{Values: seq(20, func(i int) float64 { return float64(i) - 12 }), Area: true}}}},
		{"huge", LineOpts{Labels: dates(5), Series: []Series{{Values: []float64{1e300, -1e300, math.MaxFloat64, -math.MaxFloat64, 0}}}}},
		{"inf", LineOpts{Labels: dates(4), Series: []Series{{Values: []float64{math.Inf(1), 1, math.Inf(-1), 2}}}}},
		{"nan-gaps", LineOpts{Labels: dates(9), Series: []Series{{Name: "a", Values: []float64{1, 2, nan, 4, nan, nan, 6, 7, nan}, Area: true}, {Name: "b", Dashed: true, Values: []float64{nan, 1, 2, 3}}}}},
		{"escaping", LineOpts{Title: evil, Labels: []string{evil, evil}, Series: []Series{{Name: evil, Values: []float64{1, 2}, Color: `red" onload="x`}, {Name: evil, Values: []float64{2, 1}}}}},
		{"invert", LineOpts{InvertY: true, Labels: dates(30), Series: []Series{{Name: "Position", Values: seq(30, func(i int) float64 { return 3 + float64(i%7)*1.3 })}}}},
		{"ymin", LineOpts{YMin: fp(50), Labels: dates(5), Series: []Series{{Values: []float64{60, 70, 80, 75, 90}}}}},
		{"ymin-nan", LineOpts{YMin: fp(nan), Labels: dates(3), Series: []Series{{Values: []float64{1, 2, 3}}}}},
		{"labels-only-longer", LineOpts{Labels: dates(90), Series: []Series{{Values: []float64{1, 2}}}}},
		{"values-longer", LineOpts{Labels: dates(2), Series: []Series{{Values: seq(50, func(i int) float64 { return float64(i * i) })}}}},
		{"tiny-size", LineOpts{Width: 1, Height: 1, Labels: dates(3), Series: []Series{{Values: []float64{1, 2, 3}}}}},
		{"negative-size", LineOpts{Width: -5, Height: nan, Labels: dates(3), Series: []Series{{Values: []float64{1, 2, 3}}}}},
		{"many-series", LineOpts{Labels: dates(5), Series: func() []Series {
			var s []Series
			for i := 0; i < 12; i++ {
				s = append(s, Series{Name: "series with a rather long name", Values: []float64{float64(i), 2, 3, 4, 5}})
			}
			return s
		}()}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := Line(tc.o)
			wellFormed(t, tc.name, out)
		})
	}
}

func TestLineEscaping(t *testing.T) {
	out := string(Line(LineOpts{Title: evil, Labels: []string{evil, "x"}, Series: []Series{{Name: evil, Values: []float64{1, 2}}, {Name: "b", Values: []float64{1, 2}}}}))
	if strings.Contains(out, "<script>") || strings.Contains(out, `"x")`) {
		t.Fatalf("unescaped user data in output")
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Fatalf("expected escaped title")
	}
}

func TestLineInvertY(t *testing.T) {
	// Position 1 (best) must be drawn above position 10.
	out := string(Line(LineOpts{InvertY: true, Labels: []string{"a", "b"}, Series: []Series{{Values: []float64{1, 10}}}}))
	i := strings.Index(out, `stroke-width="2" stroke-linejoin`)
	if i < 0 {
		t.Fatal("line path not found")
	}
	start := strings.LastIndex(out[:i], `d="M`)
	var x0, y0, x1, y1 float64
	if _, err := sscanPath(out[start+4:], &x0, &y0, &x1, &y1); err != nil {
		t.Fatalf("parse path: %v", err)
	}
	if !(y0 < y1) {
		t.Fatalf("InvertY: expected y(1)=%v above y(10)=%v", y0, y1)
	}
	// Top tick label is 1.
	if !strings.Contains(out, `>1</text>`) {
		t.Fatalf("expected a tick labelled 1")
	}
	// Non inverted: the opposite.
	out = string(Line(LineOpts{Labels: []string{"a", "b"}, Series: []Series{{Values: []float64{1, 10}}}}))
	i = strings.Index(out, `stroke-width="2" stroke-linejoin`)
	start = strings.LastIndex(out[:i], `d="M`)
	if _, err := sscanPath(out[start+4:], &x0, &y0, &x1, &y1); err != nil {
		t.Fatalf("parse path: %v", err)
	}
	if !(y0 > y1) {
		t.Fatalf("normal: expected y(1)=%v below y(10)=%v", y0, y1)
	}
}

func sscanPath(s string, x0, y0, x1, y1 *float64) (int, error) {
	s = strings.NewReplacer("L", " ", `"`, " ").Replace(s)
	return fmt.Sscan(s, x0, y0, x1, y1)
}

func TestBars(t *testing.T) {
	long := strings.Repeat("https://example.com/very/long/path/segment/", 6)
	cases := []struct {
		name string
		o    BarsOpts
	}{
		{"empty", BarsOpts{}},
		{"single", BarsOpts{Items: []BarItem{{Label: "a", Value: 10}}}},
		{"zeros", BarsOpts{Items: []BarItem{{Label: "a"}, {Label: "b"}}}},
		{"negative", BarsOpts{Items: []BarItem{{Label: "a", Value: -5}, {Label: "b", Value: 10}}}},
		{"nan", BarsOpts{Items: []BarItem{{Label: "a", Value: nan, Previous: fp(nan)}, {Label: "b", Value: math.Inf(1)}}, ShowDelta: true}},
		{"huge", BarsOpts{Items: []BarItem{{Label: "a", Value: math.MaxFloat64, Previous: fp(-math.MaxFloat64)}}, ShowDelta: true}},
		{"delta", BarsOpts{ShowDelta: true, Items: []BarItem{{Label: "up", Value: 120, Previous: fp(100)}, {Label: "down", Value: 80, Previous: fp(100)}, {Label: "zero-prev", Value: 5, Previous: fp(0)}, {Label: "flat", Value: 5, Previous: fp(5)}}}},
		{"long-labels", BarsOpts{Items: []BarItem{{Label: long, Value: 10}, {Label: "short", Value: 3}}}},
		{"escaping", BarsOpts{Title: evil, Items: []BarItem{{Label: evil, Value: 1, Color: `"><x`}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { wellFormed(t, tc.name, Bars(tc.o)) })
	}
	out := string(Bars(BarsOpts{Items: []BarItem{{Label: long, Value: 10}}}))
	if !strings.Contains(out, "…") || !strings.Contains(out, "<title>"+long+"</title>") {
		t.Fatalf("long label should be truncated with full text in <title>")
	}
	out = string(Bars(BarsOpts{ShowDelta: true, Items: []BarItem{{Label: "a", Value: 120, Previous: fp(100)}, {Label: "b", Value: 50, Previous: fp(100)}}}))
	if !strings.Contains(out, "+20%") || !strings.Contains(out, "−50%") {
		t.Fatalf("expected delta badges, got %s", out)
	}
	if !strings.Contains(out, ColorPositive) || !strings.Contains(out, ColorNegative) {
		t.Fatalf("expected tone colors on badges")
	}
}

func TestColumns(t *testing.T) {
	cases := []struct {
		name string
		o    ColumnsOpts
	}{
		{"empty", ColumnsOpts{}},
		{"labels-no-values", ColumnsOpts{Labels: []string{"a", "b"}}},
		{"single", ColumnsOpts{Labels: []string{"a"}, Current: []float64{3}}},
		{"with-prev", ColumnsOpts{Labels: []string{"Jan", "Feb", "Mar"}, Current: []float64{3, 5, 2}, Previous: []float64{2, 6, 2}, CurrentName: "2024", PreviousName: "2023"}},
		{"prev-shorter", ColumnsOpts{Labels: []string{"a", "b", "c"}, Current: []float64{3, 5, 2}, Previous: []float64{2}}},
		{"negative", ColumnsOpts{Labels: []string{"a", "b"}, Current: []float64{-3, 5}, Previous: []float64{2, -6}}},
		{"nan", ColumnsOpts{Labels: []string{"a", "b"}, Current: []float64{nan, nan}}},
		{"zeros", ColumnsOpts{Labels: []string{"a", "b"}, Current: []float64{0, 0}}},
		{"huge", ColumnsOpts{Labels: []string{"a", "b"}, Current: []float64{math.MaxFloat64, -math.MaxFloat64}}},
		{"many", ColumnsOpts{Labels: dates(60), Current: seq(60, func(i int) float64 { return float64(i) })}},
		{"escaping", ColumnsOpts{Title: evil, Labels: []string{evil}, Current: []float64{1}, Previous: []float64{2}, CurrentName: evil, PreviousName: evil}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { wellFormed(t, tc.name, Columns(tc.o)) })
	}
}

func TestDonut(t *testing.T) {
	cases := []struct {
		name string
		o    DonutOpts
	}{
		{"empty", DonutOpts{}},
		{"all-zero", DonutOpts{Slices: []Slice{{Label: "a"}, {Label: "b"}}}},
		{"negative-nan", DonutOpts{Slices: []Slice{{Label: "a", Value: -3}, {Label: "b", Value: nan}, {Label: "c", Value: 2}}}},
		{"single", DonutOpts{Slices: []Slice{{Label: "a", Value: 5}}, CenterValue: "5", CenterLabel: "sessions"}},
		{"many-tiny", DonutOpts{Slices: func() []Slice {
			s := []Slice{{Label: "Google", Value: 9000}, {Label: "Bing", Value: 400}}
			for i := 0; i < 10; i++ {
				s = append(s, Slice{Label: "tiny" + string(rune('a'+i)), Value: 3})
			}
			return s
		}()}},
		{"huge", DonutOpts{Slices: []Slice{{Label: "a", Value: math.MaxFloat64}, {Label: "b", Value: math.MaxFloat64}}}},
		{"escaping", DonutOpts{Title: evil, CenterValue: evil, CenterLabel: evil, Slices: []Slice{{Label: evil, Value: 1, Color: `"/>`}, {Label: "b", Value: 2}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { wellFormed(t, tc.name, Donut(tc.o)) })
	}
	out := string(Donut(DonutOpts{Slices: []Slice{{Label: "a", Value: 1}, {Label: "b", Value: 3}}}))
	if !strings.Contains(out, "25%") || !strings.Contains(out, "75%") {
		t.Fatalf("expected percentages in legend")
	}
	slices, _ := prepareSlices([]Slice{{Label: "big", Value: 1000}, {Label: "x", Value: 1}, {Label: "y", Value: 1}})
	if len(slices) != 2 || slices[1].Label != "x, y" {
		t.Fatalf("tiny slices should merge, got %+v", slices)
	}
}

func TestGauge(t *testing.T) {
	cases := []struct {
		name string
		o    GaugeOpts
	}{
		{"zero-value", GaugeOpts{}},
		{"typical", GaugeOpts{Value: 72, Label: "Score", Display: "72/100"}},
		{"over", GaugeOpts{Value: 250, Max: 100}},
		{"under", GaugeOpts{Value: -50}},
		{"nan", GaugeOpts{Value: nan, Min: nan, Max: math.Inf(1)}},
		{"min-eq-max", GaugeOpts{Value: 3, Min: 3, Max: 3}},
		{"reversed", GaugeOpts{Value: 3, Min: 10, Max: 0}},
		{"bands", GaugeOpts{Value: 2.1, Min: 0, Max: 4, Bands: []GaugeBand{{Upto: 4, Color: "#0f0"}, {Upto: 2.5, Color: "#f00"}, {Upto: nan}}}},
		{"huge", GaugeOpts{Value: math.MaxFloat64, Min: -math.MaxFloat64, Max: math.MaxFloat64}},
		{"escaping", GaugeOpts{Value: 1, Label: evil, Display: evil, Title: evil, Bands: []GaugeBand{{Upto: 50, Color: `"<`}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { wellFormed(t, tc.name, Gauge(tc.o)) })
	}
}

func TestStacked(t *testing.T) {
	cases := []struct {
		name string
		o    StackedOpts
	}{
		{"empty", StackedOpts{}},
		{"zeros", StackedOpts{Segments: []StackSegment{{Label: "a"}, {Label: "b"}}}},
		{"single", StackedOpts{Segments: []StackSegment{{Label: "Good", Value: 1}}}},
		{"typical", StackedOpts{Segments: []StackSegment{{Label: "Good", Value: 70, Color: ColorPositive}, {Label: "Needs improvement", Value: 20, Color: "#f59e0b"}, {Label: "Poor", Value: 10, Color: ColorNegative}}}},
		{"nan-negative", StackedOpts{Segments: []StackSegment{{Label: "a", Value: nan}, {Label: "b", Value: -1}, {Label: "c", Value: 3}}}},
		{"huge", StackedOpts{Segments: []StackSegment{{Label: "a", Value: math.MaxFloat64}, {Label: "b", Value: math.MaxFloat64}}}},
		{"escaping", StackedOpts{Title: evil, Segments: []StackSegment{{Label: evil, Value: 1, Color: `"<`}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { wellFormed(t, tc.name, Stacked(tc.o)) })
	}
}

func TestSparkline(t *testing.T) {
	cases := []struct {
		name string
		o    SparkOpts
	}{
		{"empty", SparkOpts{}},
		{"single", SparkOpts{Values: []float64{3}}},
		{"constant", SparkOpts{Values: []float64{3, 3, 3}}},
		{"nan", SparkOpts{Values: []float64{nan, 1, nan, 2, 3, nan}}},
		{"all-nan", SparkOpts{Values: []float64{nan, nan}}},
		{"invert", SparkOpts{Values: []float64{5, 3, 1}, InvertY: true}},
		{"huge", SparkOpts{Values: []float64{math.MaxFloat64, -math.MaxFloat64, math.Inf(1)}}},
		{"color", SparkOpts{Values: []float64{1, 2}, Color: `"<x`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { wellFormed(t, tc.name, Sparkline(tc.o)) })
	}
}

func TestDeltaArrow(t *testing.T) {
	for _, d := range []string{"up", "down", "flat", "", "weird"} {
		for _, tone := range []string{"positive", "negative", "neutral", ""} {
			out := DeltaArrow(d, tone)
			wellFormed(t, d+"/"+tone, out)
		}
	}
	if !strings.Contains(string(DeltaArrow("up", "positive")), ColorPositive) {
		t.Fatal("positive tone color missing")
	}
	if !strings.Contains(string(DeltaArrow("down", "negative")), ColorNegative) {
		t.Fatal("negative tone color missing")
	}
}

func TestEmptyState(t *testing.T) {
	wellFormed(t, "empty", EmptyState(0, 0, ""))
	wellFormed(t, "empty-msg", EmptyState(300, 100, evil))
}

func TestUniqueIDs(t *testing.T) {
	a := string(Line(LineOpts{Labels: []string{"a", "b"}, Series: []Series{{Values: []float64{1, 2}, Area: true}}}))
	b := string(Line(LineOpts{Labels: []string{"a", "b"}, Series: []Series{{Values: []float64{1, 2}, Area: true}}}))
	id := func(s string) string {
		i := strings.Index(s, `id="`)
		j := strings.Index(s[i+4:], `"`)
		return s[i+4 : i+4+j]
	}
	if id(a) == id(b) {
		t.Fatalf("gradient ids must be unique across charts: %s", id(a))
	}
}

func TestFormatters(t *testing.T) {
	cases := []struct {
		f    Formatter
		in   float64
		want string
	}{
		{FormatCompact, 0, "0"},
		{FormatCompact, 950, "950"},
		{FormatCompact, 999.6, "1k"},
		{FormatCompact, 99.94, "99.9"},
		{FormatCompact, 1000, "1k"},
		{FormatCompact, 12345, "12.3k"},
		{FormatCompact, 123456, "123k"},
		{FormatCompact, 999950, "1M"},
		{FormatCompact, 4.5e6, "4.5M"},
		{FormatCompact, 1.2e9, "1.2B"},
		{FormatCompact, -2500, "−2.5k"},
		{FormatCompact, 0.034, "0.03"},
		{FormatCompact, 4.25, "4.3"},
		{FormatCompact, nan, "–"},
		{FormatCompact, math.Inf(1), "∞"},
		{FormatCompact, math.MaxFloat64, FormatCompact(math.MaxFloat64)},
		{FormatInt, 1234567, "1 234 567"},
		{FormatInt, -999, "−999"},
		{FormatInt, 0.4, "0"},
		{FormatDecimal, 4.25, "4.3"},
		{FormatDecimal, 12345.67, "12 345.7"},
		{FormatDecimal, -0.01, "0.0"},
		{FormatPct, 12.34, "12%"},
		{FormatPct, 4.56, "4.6%"},
		{FormatPct, 4, "4%"},
		{FormatRatio, 0.0345, "3.5%"},
		{FormatDelta, 12.3, "+12%"},
		{FormatDelta, -4.5, "−4.5%"},
		{FormatDelta, 0.01, "0%"},
		{FormatDelta, 5000, ">+999%"},
	}
	for _, tc := range cases {
		if got := tc.f(tc.in); got != tc.want {
			t.Errorf("format(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestShortLabel(t *testing.T) {
	cases := map[string]string{
		"2024-03-12": "12/03",
		"20240312":   "12/03",
		"2024-03":    "03/2024",
		"query":      "query",
		"2024-13-45": "2024-13-45",
		"":           "",
	}
	for in, want := range cases {
		if got := shortLabel(in); got != want {
			t.Errorf("shortLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNiceTicks(t *testing.T) {
	cases := []struct{ lo, hi float64 }{
		{0, 1}, {0, 97}, {0, 1234}, {-5, 5}, {3.2, 8.7}, {0, 1e-9}, {1e12, 1e12 + 5}, {-1e300, 1e300}, {7, 7}, {0, 0},
	}
	for _, tc := range cases {
		lo, hi, step := niceScale(tc.lo, tc.hi, 5)
		if !finite(lo) || !finite(hi) || !finite(step) || step <= 0 || lo > tc.lo || (hi < tc.hi && tc.hi != tc.lo) {
			t.Errorf("niceScale(%v,%v) = %v,%v,%v", tc.lo, tc.hi, lo, hi, step)
		}
		ticks := buildTicks(lo, hi, step)
		if len(ticks) < 1 || len(ticks) > 61 {
			t.Errorf("ticks(%v,%v) = %v", tc.lo, tc.hi, ticks)
		}
	}
}

func TestTruncate(t *testing.T) {
	s, cut := truncate("hello", 1000, 12)
	if cut || s != "hello" {
		t.Fatal("should not truncate")
	}
	s, cut = truncate(strings.Repeat("abcdef ", 20), 80, 12)
	if !cut || !strings.HasSuffix(s, "…") || textWidth(s, 12) > 80 {
		t.Fatalf("bad truncation %q (%v)", s, textWidth(s, 12))
	}
	if got, _ := truncate("abc", 0, 12); got != "…" {
		t.Fatalf("zero width: %q", got)
	}
}

// TestPreview writes an HTML page with every chart when CHARTS_PREVIEW is set
// to an output path: CHARTS_PREVIEW=/tmp/charts.html go test -run Preview .
func TestPreview(t *testing.T) {
	path := os.Getenv("CHARTS_PREVIEW")
	if path == "" {
		t.Skip("set CHARTS_PREVIEW=/path/out.html to write a preview page")
	}
	cur := seq(90, func(i int) float64 {
		return 800 + 300*math.Sin(float64(i)/9) + float64(i)*6 + float64((i*37)%50)
	})
	prev := seq(90, func(i int) float64 { return 700 + 200*math.Sin(float64(i)/8+1) + float64((i*53)%60) })
	pos := seq(90, func(i int) float64 { return 9 - float64(i)/18 + math.Sin(float64(i)/5) })
	gaps := append([]float64(nil), cur[:30]...)
	gaps[8], gaps[9], gaps[20] = nan, nan, nan

	blocks := []struct {
		title string
		html  template.HTML
		wide  bool
	}{
		{"Line — clicks vs previous period", Line(LineOpts{Width: 1100, Height: 300, Labels: dates(90), Series: []Series{
			{Name: "Clicks (last 90 days)", Values: cur, Area: true},
			{Name: "Previous period", Values: prev, Dashed: true, Color: ColorPrevious},
		}, YFormat: FormatCompact}), true},
		{"Line — average position (InvertY)", Line(LineOpts{Width: 540, Labels: dates(90), InvertY: true, YFormat: FormatDecimal, Series: []Series{{Name: "Position", Values: pos}}}), false},
		{"Line — NaN gaps, single series", Line(LineOpts{Width: 540, Labels: dates(30), Series: []Series{{Name: "Sessions", Values: gaps, Area: true, Color: Palette[2]}}}), false},
		{"Line — single point", Line(LineOpts{Width: 540, Labels: []string{"2024-03-12"}, Series: []Series{{Name: "x", Values: []float64{42}}}}), false},
		{"Line — empty", Line(LineOpts{Width: 540}), false},
		{"Bars — top queries with delta", Bars(BarsOpts{Width: 540, ShowDelta: true, Format: FormatInt, Items: []BarItem{
			{Label: "chaussures running homme", Value: 4210, Previous: fp(3600)},
			{Label: "meilleure montre gps 2024", Value: 3120, Previous: fp(3500)},
			{Label: "avis <marque> & co", Value: 1980, Previous: fp(1100)},
			{Label: "trail", Value: 1200, Previous: fp(1200)},
			{Label: "nouvelle requête", Value: 640},
		}}), false},
		{"Bars — long URLs (label above)", Bars(BarsOpts{Width: 540, Format: FormatInt, ShowDelta: true, Items: []BarItem{
			{Label: "https://www.example.com/blog/2024/03/comment-choisir-ses-chaussures-de-trail-pour-debutant", Value: 1520, Previous: fp(1300)},
			{Label: "https://www.example.com/produits/montres/gps/forerunner-265", Value: 980, Previous: fp(1210)},
			{Label: "https://www.example.com/", Value: 450, Previous: fp(400)},
		}}), false},
		{"Columns — sessions by month", Columns(ColumnsOpts{Width: 540, Labels: []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun"}, Current: []float64{1200, 1500, 1320, 1810, 2100, 1950}, Previous: []float64{1000, 1100, 1400, 1500, 1600, 1700}, CurrentName: "2024", PreviousName: "2023"}), false},
		{"Columns — single series, 30 days", Columns(ColumnsOpts{Width: 540, Labels: dates(30), Current: seq(30, func(i int) float64 { return float64(50 + (i*17)%40) })}), false},
		{"Donut — traffic channels", Donut(DonutOpts{CenterLabel: "sessions", Format: FormatInt, Slices: []Slice{
			{Label: "Organic Search", Value: 5400}, {Label: "Direct", Value: 2100}, {Label: "Referral", Value: 900},
			{Label: "Organic Social", Value: 600}, {Label: "Email", Value: 120}, {Label: "Paid", Value: 40}, {Label: "Display", Value: 30},
		}}), false},
		{"Donut — AI engines", Donut(DonutOpts{CenterValue: "38%", CenterLabel: "share of voice", Slices: []Slice{
			{Label: "ChatGPT", Value: 46}, {Label: "Perplexity", Value: 28}, {Label: "Gemini", Value: 18}, {Label: "Copilot", Value: 8},
		}}), false},
		{"Donut — empty", Donut(DonutOpts{CenterLabel: "sessions"}), false},
		{"Gauge — GEO score", Gauge(GaugeOpts{Value: 72, Label: "AI visibility score"}), false},
		{"Gauge — LCP", Gauge(GaugeOpts{Value: 3.1, Max: 6, Display: "3.1 s", Label: "LCP (p75)", Bands: []GaugeBand{{Upto: 2.5, Color: ColorPositive}, {Upto: 4, Color: "#f59e0b"}, {Upto: 6, Color: ColorNegative}}}), false},
		{"Stacked — Core Web Vitals", Stacked(StackedOpts{Width: 540, Format: FormatInt, Segments: []StackSegment{
			{Label: "Good", Value: 312, Color: ColorPositive}, {Label: "Needs improvement", Value: 88, Color: "#f59e0b"}, {Label: "Poor", Value: 21, Color: ColorNegative},
		}}), false},
		{"Stacked — position buckets", Stacked(StackedOpts{Width: 540, Segments: []StackSegment{
			{Label: "Top 3", Value: 120}, {Label: "4–10", Value: 340}, {Label: "11–20", Value: 410}, {Label: "21–50", Value: 600}, {Label: "51+", Value: 25},
		}}), false},
	}

	var b strings.Builder
	b.WriteString(`<!doctype html><html><head><meta charset="utf-8"><title>charts preview</title><style>
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Inter,Roboto,sans-serif;background:#f6f7f9;color:#0f172a;margin:0;padding:32px}
.grid{display:grid;grid-template-columns:repeat(2,1fr);gap:20px;max-width:1180px;margin:auto}
.card{background:#fff;border:1px solid #e5e7eb;border-radius:12px;padding:18px 20px}
.wide{grid-column:1/-1}
h3{font-size:13px;font-weight:600;color:#475569;margin:0 0 14px}
.kpis{display:flex;gap:16px}.kpi{flex:1;background:#fff;border:1px solid #e5e7eb;border-radius:12px;padding:14px 16px}
.kpi .v{font-size:26px;font-weight:650}.kpi .d{font-size:13px;color:#16a34a;margin:2px 0 8px}
.dark{background:#0f172a;color:#e2e8f0;--chart-surface:#0f172a;border-color:#1e293b}
.dark h3{color:#94a3b8}
@media print{body{background:#fff;padding:0}.card{break-inside:avoid}}
</style></head><body><div class="grid">`)
	b.WriteString(`<div class="wide kpis">`)
	for i, k := range []struct {
		v, d, dir, tone string
		vals            []float64
		inv             bool
	}{
		{"12.4k", "+18%", "up", "positive", cur[60:], false},
		{"3.2%", "−4.1%", "down", "negative", prev[60:], false},
		{"7.4", "+1.3", "up", "positive", pos[60:], true},
		{"58%", "0%", "flat", "neutral", gaps, false},
	} {
		b.WriteString(`<div class="kpi"><div class="v">` + k.v + `</div><div class="d">` + string(DeltaArrow(k.dir, k.tone)) + " " + k.d + `</div>` +
			string(Sparkline(SparkOpts{Values: k.vals, InvertY: k.inv, Color: Palette[i]})) + `</div>`)
	}
	b.WriteString(`</div>`)
	for _, bl := range blocks {
		cls := "card"
		if bl.wide {
			cls += " wide"
		}
		b.WriteString(`<div class="` + cls + `"><h3>` + template.HTMLEscapeString(bl.title) + `</h3>` + string(bl.html) + `</div>`)
	}
	b.WriteString(`<div class="card dark wide"><h3>Dark host (currentColor theming)</h3>` + string(blocks[0].html) + `</div>`)
	b.WriteString(`</div></body></html>`)
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s", path)
}

func TestLocaleHooks(t *testing.T) {
	frPct := func(v float64) string {
		return strings.Replace(strings.TrimSuffix(FormatPct(v), "%"), ".", ",", 1) + "\u202f%"
	}
	frDelta := func(v float64) string { return "Δ" + FormatPct(v) }

	out := string(Line(LineOpts{
		Labels:         []string{"2024-03-12", "2024-03-13"},
		Series:         []Series{{Name: "a", Values: []float64{1, 2}}},
		XFormat:        func(s string) string { return "X" + s[8:] },
		TooltipXFormat: func(s string) string { return "T" + s },
	}))
	wellFormed(t, "line-hooks", template.HTML(out))
	if !strings.Contains(out, ">X12<") || !strings.Contains(out, "T2024-03-12") || strings.Contains(out, "12/03") {
		t.Fatalf("XFormat/TooltipXFormat not applied:\n%s", out)
	}

	out = string(Bars(BarsOpts{ShowDelta: true, DeltaFormat: frDelta, Items: []BarItem{{Label: "a", Value: 120, Previous: fp(100)}}}))
	wellFormed(t, "bars-hooks", template.HTML(out))
	if !strings.Contains(out, "Δ20%") || strings.Contains(out, "+20%") {
		t.Fatalf("DeltaFormat not applied")
	}

	out = string(Columns(ColumnsOpts{Labels: []string{"a"}, Current: []float64{2}, Previous: []float64{1}, CurrentName: "c", PreviousName: "p",
		Color: "#123456", PreviousColor: "#abcdef", DeltaFormat: frDelta}))
	wellFormed(t, "columns-hooks", template.HTML(out))
	if !strings.Contains(out, "#123456") || !strings.Contains(out, "#abcdef") || strings.Contains(out, Palette[0]) || strings.Contains(out, ColorPrevious) || !strings.Contains(out, "Δ100%") {
		t.Fatalf("Columns colors/DeltaFormat not applied:\n%s", out)
	}

	out = string(Donut(DonutOpts{PctFormat: frPct, Slices: []Slice{{Label: "a", Value: 1}, {Label: "b", Value: 14}}}))
	wellFormed(t, "donut-hooks", template.HTML(out))
	if !strings.Contains(out, "6,7 %") || strings.Contains(out, "6.7%") {
		t.Fatalf("Donut PctFormat not applied")
	}

	out = string(Stacked(StackedOpts{PctFormat: frPct, Segments: []StackSegment{{Label: "a", Value: 1}, {Label: "b", Value: 14}}}))
	wellFormed(t, "stacked-hooks", template.HTML(out))
	if !strings.Contains(out, "6,7 %") || strings.Contains(out, "6.7%") {
		t.Fatalf("Stacked PctFormat not applied")
	}

	out = string(Gauge(GaugeOpts{Value: 2.5, Max: 4, Format: func(v float64) string { return "g" + FormatDecimal(v) }}))
	wellFormed(t, "gauge-hooks", template.HTML(out))
	if !strings.Contains(out, ">g2.5<") || !strings.Contains(out, ">g4.0<") {
		t.Fatalf("Gauge Format not applied")
	}

	out = string(Sparkline(SparkOpts{Values: []float64{1, 2}, Format: func(v float64) string { return "s" + FormatInt(v) }}))
	wellFormed(t, "spark-hooks", template.HTML(out))
	if !strings.Contains(out, "s1 → s2") {
		t.Fatalf("Sparkline Format not applied")
	}
}

func TestBarsHeight(t *testing.T) {
	items := []BarItem{{Label: "a", Value: 1}, {Label: "b", Value: 2}}
	compact := string(Bars(BarsOpts{Width: 400, Items: items}))
	filled := string(Bars(BarsOpts{Width: 400, Items: items, Height: 300}))
	wellFormed(t, "bars-height", template.HTML(filled))
	if !strings.Contains(compact, `viewBox="0 0 400 68"`) {
		t.Fatalf("compact height changed: %.200s", compact)
	}
	if !strings.Contains(filled, `viewBox="0 0 400 104"`) { // rows capped at 1.6x
		t.Fatalf("Height not applied: %.200s", filled)
	}
}
