package render

import (
	"math"

	"github.com/flocom/SEO-GEO-Report/internal/i18n"
)

// Weekly aggregation of daily series.
//
// Daily charts of tiny volumes (0 to 10 clicks a day) are mostly noise: a
// zig-zag a beginner reads as instability. Such series, and long periods in
// part 1, are summed per week instead.

// Aggregation thresholds.
const (
	lowDailyAverage = 5.0 // below this average per day, daily points are noise
	longPeriodDays  = 60  // part 1 shows weekly points beyond this length
)

// aggMode tells how daily values combine into a weekly point.
type aggMode int

const (
	aggSum  aggMode = iota // counts (clicks, sessions): weekly total
	aggMean                // rates and positions: average of the days
)

// weekBucket is one weekly point: the indices of its days.
type weekBucket struct {
	from, to int // inclusive day indices
}

// weekBuckets splits n consecutive days into weeks of 7 days from the first
// one. A trailing remainder of 4 days or more is its own (partial) week;
// shorter remainders join the last full week, so that no point stands on 1
// to 3 days.
func weekBuckets(n int) []weekBucket {
	if n <= 0 {
		return nil
	}
	var out []weekBucket
	for i := 0; i < n; i += 7 {
		out = append(out, weekBucket{from: i, to: min(i+6, n-1)})
	}
	if l := len(out); l > 1 {
		if last := out[l-1]; last.to-last.from+1 < 4 {
			out[l-2].to = last.to
			out = out[:l-1]
		}
	}
	return out
}

// aggregateWeekly turns a daily series into weekly points. Sums are scaled
// to 7 days for weeks with another number of known days (a 10-day last week
// counts as 7/10 of its total, a comparison period shorter than the current
// one is extrapolated), so that every point compares with the others. NaN
// days are skipped; a week without any value is NaN.
func aggregateWeekly(vals []float64, buckets []weekBucket, mode aggMode) []float64 {
	out := make([]float64, len(buckets))
	for k, bk := range buckets {
		sum, n := 0.0, 0
		for i := bk.from; i <= bk.to; i++ {
			if i >= len(vals) || math.IsNaN(vals[i]) {
				continue
			}
			sum += vals[i]
			n++
		}
		switch {
		case n == 0:
			out[k] = math.NaN()
		case mode == aggMean:
			out[k] = sum / float64(n)
		default:
			out[k] = sum * 7 / float64(n)
		}
	}
	return out
}

// dailyAverage is the mean of the non-NaN values.
func dailyAverage(vals []float64) float64 {
	sum, n := 0.0, 0
	for _, v := range vals {
		if !math.IsNaN(v) {
			sum += v
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// lowVolume reports whether a daily count series is too small to be read
// day by day.
func lowVolume(vals []float64) bool { return len(vals) > 0 && dailyAverage(vals) < lowDailyAverage }

// wantWeekly decides whether a daily count series is shown per week: tiny
// volumes always, long periods in part 1 (glance).
func wantWeekly(vals []float64, glance bool) bool {
	if len(vals) < 14 {
		return false // under two weeks, weekly points say nothing
	}
	return lowVolume(vals) || (glance && len(vals) > longPeriodDays)
}

// weekly is a daily series turned into weekly points, with the labels (first
// day of each week, ISO) and the tooltip text of each week ("1 – 7 oct. 2026").
type weekly struct {
	labels []string
	ranges map[string]string
	cur    []float64
	prev   []float64
}

// toWeekly aggregates the current and previous series on the same weeks
// (the previous period is aligned day by day on the current one).
func (b *builder) toWeekly(dates []string, cur, prev []float64, mode aggMode) weekly {
	bk := weekBuckets(len(cur))
	w := weekly{ranges: map[string]string{}, cur: aggregateWeekly(cur, bk, mode)}
	if len(prev) > 0 {
		w.prev = aggregateWeekly(alignPrev(len(cur), prev), bk, mode)
	}
	for _, k := range bk {
		if k.to >= len(dates) {
			break
		}
		start, end := dates[k.from], dates[k.to]
		w.labels = append(w.labels, start)
		w.ranges[start] = i18n.DateRange(b.lang, start, end)
	}
	return w
}

// tip formats a weekly label for tooltips (the week range).
func (w weekly) tip(fallback func(string) string) func(string) string {
	return func(s string) string {
		if r, ok := w.ranges[s]; ok {
			return r
		}
		return fallback(s)
	}
}
