package insights

import (
	"math"
	"strings"
)

// Human-readable changes.
//
// A percentage is unreadable on small or multiplied figures: "+500 %" for a
// move from 1 to 6 visits, "+279 %" for impressions multiplied by 3.8.
// ChangeKind is the single rule used by the automatic insights and by the
// report (KPI cards, scorecard, tiles, table and chart badges):
//
//   - "new": previous = 0 and current > 0;
//   - "ratio": multiplied by 3 or more from a base of at least 5 ("×3,8");
//   - "abs": small base, previous < 20 ("+5");
//   - "ratio" again for larger bases multiplied by 3 or more;
//   - "pct" otherwise, always between −100 % and +200 %.
//
// "" means there is no change to show (missing or invalid values, 0 → 0).
func ChangeKind(cur, prev float64) string {
	if math.IsNaN(cur) || math.IsNaN(prev) || math.IsInf(cur, 0) || math.IsInf(prev, 0) {
		return ""
	}
	switch {
	case prev == 0 && cur == 0:
		return ""
	case prev == 0 && cur > 0:
		return "new"
	case prev == 0:
		return "abs"
	case prev < 0:
		return "pct"
	}
	ratio := cur / prev
	switch {
	case ratio >= RatioThreshold && prev >= MinRatioBase:
		return "ratio"
	case prev < SmallBase:
		return "abs"
	case ratio >= RatioThreshold:
		return "ratio"
	}
	return "pct"
}

// Thresholds of ChangeKind.
const (
	SmallBase      = 20.0 // below, the absolute change is clearer than a percentage
	MinRatioBase   = 5.0  // a multiplier needs a base of at least 5
	RatioThreshold = 3.0  // from ×3, a multiplier replaces the percentage
)

// FormatRatio formats a multiplier: "×3,8" / "×3.8" below 10, "×47" above.
func FormatRatio(r float64, lang string) string {
	if r >= 9.95 {
		return "×" + FormatNumber(r, 0, lang)
	}
	s := FormatNumber(r, 1, lang)
	s = strings.TrimSuffix(strings.TrimSuffix(s, ",0"), ".0")
	return "×" + s
}

// spc is a change in parentheses or after a figure: "+58 %", "×3,7", "+5".
func (w *writer) spc(pct, cur, prev float64) string {
	switch ChangeKind(cur, prev) {
	case "ratio":
		return FormatRatio(cur/prev, w.lang)
	case "abs":
		return w.sn(cur - prev)
	}
	return w.sp(pct)
}

// by completes "progressent …" / "grew …": "de 58 %" / "by 58%",
// "fortement (×3,7)" / "sharply (×3.7)", "de 5" / "by 5".
func (w *writer) by(pct, cur, prev float64) string {
	switch ChangeKind(cur, prev) {
	case "ratio":
		if w.lang == "en" {
			return "sharply (" + FormatRatio(cur/prev, w.lang) + ")"
		}
		return "fortement (" + FormatRatio(cur/prev, w.lang) + ")"
	case "abs":
		if w.lang == "en" {
			return "by " + w.n(math.Abs(cur-prev))
		}
		return "de " + w.n(math.Abs(cur-prev))
	}
	if w.lang == "en" {
		return "by " + w.p(pct)
	}
	return "de " + w.p(pct)
}
