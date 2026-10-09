package charts

import (
	"math"
	"strconv"
	"strings"
)

// Number formatters. They are locale-neutral: "." as decimal separator,
// a narrow no-break space (U+202F) as thousands separator and the true minus
// sign (U+2212). Callers needing locale-specific output pass their own
// Formatter in the chart options.

const (
	minus     = "−"
	thinSpace = " "
	noValue   = "–"
)

// FormatCompact formats v compactly: 950, 12.3k, 4.5M, 1.2B. Small
// fractional values keep up to two significant decimals (0.03, 4.2). It is
// the default formatter of every chart.
func FormatCompact(v float64) string {
	if s, ok := special(v); ok {
		return s
	}
	sign := ""
	if v < 0 {
		sign = minus
		v = -v
	}
	units := []struct {
		div float64
		suf string
	}{{1, ""}, {1e3, "k"}, {1e6, "M"}, {1e9, "B"}, {1e12, "T"}}
	if dec := smallDecimals(v); roundTo(v, dec) < 1000 {
		return sign + trimNum(roundTo(v, dec), dec)
	}
	for i := 1; i < len(units); i++ {
		s := v / units[i].div
		dec := 1
		if s >= 100 {
			dec = 0
		}
		r := roundTo(s, dec)
		if r >= 1000 && i < len(units)-1 {
			continue
		}
		return sign + trimNum(r, dec) + units[i].suf
	}
	return sign + strconv.FormatFloat(v, 'g', 3, 64)
}

// FormatInt formats v rounded to an integer with grouped thousands:
// 12 345 (narrow no-break space).
func FormatInt(v float64) string {
	if s, ok := special(v); ok {
		return s
	}
	v = math.Round(v)
	if math.Abs(v) >= 1e18 {
		return FormatCompact(v)
	}
	sign := ""
	if v < 0 {
		sign = minus
		v = -v
	}
	return sign + group(strconv.FormatInt(int64(v), 10))
}

// FormatDecimal formats v with one decimal (e.g. average position "4.2"),
// grouping thousands.
func FormatDecimal(v float64) string {
	if s, ok := special(v); ok {
		return s
	}
	sign := ""
	if v < 0 {
		sign = minus
		v = -v
	}
	if v >= 1e15 {
		return sign + FormatCompact(v)
	}
	s := strconv.FormatFloat(roundTo(v, 1), 'f', 1, 64)
	ip, fp, _ := strings.Cut(s, ".")
	if s == "0.0" {
		sign = ""
	}
	return sign + group(ip) + "." + fp
}

// FormatPct formats v, already expressed in percent, as "12%" or "4.5%"
// (one decimal below 10).
func FormatPct(v float64) string {
	if s, ok := special(v); ok {
		return s
	}
	dec := 0
	if math.Abs(v) < 10 {
		dec = 1
	}
	sign := ""
	if v < 0 {
		sign = minus
		v = -v
	}
	r := roundTo(v, dec)
	if r == 0 {
		sign = ""
	}
	if r >= 1e6 {
		return sign + FormatCompact(r) + "%"
	}
	return sign + trimNum(r, dec) + "%"
}

// FormatRatio formats a ratio (0.123) as a percentage ("12%"), e.g. CTR.
func FormatRatio(v float64) string { return FormatPct(v * 100) }

// FormatDelta formats a relative change in percent with an explicit sign:
// "+12%", "−4.5%", "0%". Values beyond ±999% are capped (">+999%").
func FormatDelta(pct float64) string {
	if s, ok := special(pct); ok {
		return s
	}
	switch {
	case pct > 999:
		return ">+999%"
	case pct < -999:
		return "<" + minus + "999%"
	}
	s := FormatPct(pct)
	if s == "0%" || s == "0.0%" {
		return "0%"
	}
	if pct > 0 {
		return "+" + s
	}
	return s
}

func special(v float64) (string, bool) {
	switch {
	case math.IsNaN(v):
		return noValue, true
	case math.IsInf(v, 1):
		return "∞", true
	case math.IsInf(v, -1):
		return minus + "∞", true
	}
	return "", false
}

func smallDecimals(v float64) int {
	switch {
	case v == math.Trunc(v):
		return 0
	case v < 1:
		return 2
	case v < 100:
		return 1
	default:
		return 0
	}
}

func roundTo(v float64, dec int) float64 {
	p := math.Pow(10, float64(dec))
	return math.Round(v*p) / p
}

// trimNum formats v with dec decimals and trims trailing zeros.
func trimNum(v float64, dec int) string {
	s := strconv.FormatFloat(v, 'f', dec, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	if s == "-0" {
		s = "0"
	}
	return s
}

func group(digits string) string {
	if len(digits) <= 3 {
		return digits
	}
	var b strings.Builder
	pre := len(digits) % 3
	if pre > 0 {
		b.WriteString(digits[:pre])
	}
	for i := pre; i < len(digits); i += 3 {
		if b.Len() > 0 {
			b.WriteString(thinSpace)
		}
		b.WriteString(digits[i : i+3])
	}
	return b.String()
}
