package importer

import (
	"errors"
	"sort"
	"strings"

	"github.com/flocom/SEO-GEO-Report/internal/insights"
	"github.com/flocom/SEO-GEO-Report/internal/model"
)

type gaDim int

const (
	dimNone gaDim = iota
	dimDate
	dimDateRange
	dimChannel
	dimSourceMedium
	dimSource
	dimMedium
	dimLanding
)

type gaMetric int

const (
	metNone gaMetric = iota
	metSessions
	metUsers
	metEngRate
	metKeyEvents
	metRevenue
)

func classifyGA4Metric(h string) gaMetric {
	switch {
	case containsAny(h, "engagement rate", "taux d'engagement"):
		return metEngRate
	case containsAny(h, "key event", "evenements cles", "evenement cle", "conversion") && !containsAny(h, "rate", "taux", "per ", "par "):
		return metKeyEvents
	case containsAny(h, "revenue", "revenu", "chiffre d'affaires") && !containsAny(h, "average", "moyen", "per ", "par "):
		return metRevenue
	case containsAny(h, "users", "utilisateurs") && !containsAny(h, "new", "nouveaux", "returning", "per ", "par ", "rate", "taux"):
		return metUsers
	case strings.Contains(h, "sessions") && !containsAny(h, "engag", "per ", "par ", "rate", "taux", "duration", "duree"):
		return metSessions
	}
	return metNone
}

func classifyGA4Dim(h string) gaDim {
	switch {
	case containsAny(h, "date range", "plage de dates", "periode", "period"):
		return dimDateRange
	case h == "date" || h == "jour" || h == "day" || strings.HasPrefix(h, "date "):
		return dimDate
	case containsAny(h, "channel group", "groupe de canaux", "canal") || h == "channel":
		return dimChannel
	case containsAny(h, "landing page", "page de destination"):
		return dimLanding
	case strings.Contains(h, "source") && containsAny(h, "medium", "support"):
		return dimSourceMedium
	case strings.Contains(h, "source"):
		return dimSource
	case containsAny(h, "medium", "support"):
		return dimMedium
	}
	return dimNone
}

type ga4Cols struct {
	dims       map[gaDim]int
	cur, prev  map[gaMetric]int
	prevHeader string
}

func mapGA4Columns(headers []string) ga4Cols {
	c := ga4Cols{dims: map[gaDim]int{}, cur: map[gaMetric]int{}, prev: map[gaMetric]int{}}
	for i, h := range headers {
		if m := classifyGA4Metric(h); m != metNone {
			if isPreviousHeader(h) {
				c.prev[m] = i
				c.prevHeader = h
			} else if _, ok := c.cur[m]; !ok {
				c.cur[m] = i
			} else if _, ok := c.prev[m]; !ok {
				c.prev[m] = i
			}
			continue
		}
		if d := classifyGA4Dim(h); d != dimNone {
			if _, ok := c.dims[d]; !ok {
				c.dims[d] = i
			}
		}
	}
	return c
}

// kindDim returns the dimension column required by a GA4 table kind.
func (c ga4Cols) kindDim(kind GA4Kind) (int, bool) {
	var d []gaDim
	switch kind {
	case GA4Channels:
		d = []gaDim{dimChannel}
	case GA4Sources:
		d = []gaDim{dimSourceMedium, dimSource}
	case GA4LandingPages:
		d = []gaDim{dimLanding}
	case GA4Daily:
		d = []gaDim{dimDate}
	}
	for _, x := range d {
		if i, ok := c.dims[x]; ok {
			return i, true
		}
	}
	return -1, false
}

func (c ga4Cols) guessKind() GA4Kind {
	for _, k := range []GA4Kind{GA4Daily, GA4LandingPages, GA4Sources, GA4Channels} {
		if _, ok := c.kindDim(k); ok {
			return k
		}
	}
	return ""
}

// GuessGA4Kind returns the kind of a GA4 CSV from its first table header
// ("" when it is not recognized).
func GuessGA4Kind(data []byte) GA4Kind {
	for _, blk := range splitBlocks(decodeText(data)) {
		recs, err := readRecords(blk, detectDelimiter(blk))
		if err != nil || len(recs) == 0 {
			continue
		}
		c := mapGA4Columns(foldAll(recs[0]))
		if len(c.cur) > 0 {
			if k := c.guessKind(); k != "" {
				return k
			}
		}
	}
	return ""
}

// normalizeGA4Kind accepts the GA4Kind constants and loose aliases
// ("channel", "source / medium", "landing-pages", "date"...). Unknown values
// give "" (guess from the header).
func normalizeGA4Kind(k GA4Kind) GA4Kind {
	v := fold(string(k))
	switch {
	case v == "":
		return ""
	case containsAny(v, "landing", "destination", "page"):
		return GA4LandingPages
	case containsAny(v, "source", "medium", "support"):
		return GA4Sources
	case containsAny(v, "channel", "canal", "canaux"):
		return GA4Channels
	case containsAny(v, "daily", "date", "day", "jour"):
		return GA4Daily
	}
	return ""
}

func foldAll(rec []string) []string {
	out := make([]string, len(rec))
	for i, h := range rec {
		out[i] = fold(h)
	}
	return out
}

// splitBlocks removes '#' comment lines and splits the text into blocks
// separated by blank lines.
func splitBlocks(text string) []string {
	var blocks []string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			blocks = append(blocks, strings.Join(cur, "\n"))
			cur = nil
		}
	}
	for _, line := range strings.Split(text, "\n") {
		tl := strings.TrimSpace(line)
		if strings.HasPrefix(tl, "#") {
			continue
		}
		if strings.Trim(tl, ",;\t\" ") == "" {
			flush()
			continue
		}
		cur = append(cur, line)
	}
	flush()
	return blocks
}

func isTotalRow(key string) bool {
	k := fold(key)
	switch k {
	case "total", "totals", "totaux", "grand total", "total general", "somme", "sum":
		return true
	}
	return strings.HasPrefix(k, "total ") || strings.HasPrefix(k, "grand total")
}

// gaVals accumulates the metrics of one row group.
type gaVals struct {
	sessions, users, keyEvents, revenue float64
	engW, engSum, engN                  float64 // sessions-weighted and plain sums of engagement rates
	present                             bool
}

func (v *gaVals) add(o gaVals) {
	v.sessions += o.sessions
	v.users += o.users
	v.keyEvents += o.keyEvents
	v.revenue += o.revenue
	v.engW += o.engW
	v.engSum += o.engSum
	v.engN += o.engN
	v.present = v.present || o.present
}

func (v gaVals) engagementRate() float64 {
	if v.sessions > 0 && v.engW > 0 {
		return round2(v.engW / v.sessions)
	}
	if v.engN > 0 {
		return round2(v.engSum / v.engN)
	}
	return 0
}

type gaGroup struct {
	key                       string
	cur, prev                 gaVals
	organic, ai               float64
	prevOrganic, prevAI       float64
	prevFromRange, alignedPrv bool
}

// ParseGA4CSV parses a GA4 "Export > Download CSV" file (comment lines starting
// with '#' are skipped, French or English headers) and merges the rows into a
// (possibly nil) Analytics. kind may be empty: it is guessed from the header.
//
// Only the first data table is used; "Total"/"Grand total" rows are skipped
// (their values fill missing totals for channel and daily tables). Engagement
// rates given as ratios (0.62) are converted to percent. Rows of a "Date
// range" column other than the first range, and "Previous ..." comparison
// columns, fill the prev_* fields and previous_daily. The parsed table
// replaces the matching list (channels, sources, landing_pages or daily);
// missing totals are completed from channel and daily tables.
func ParseGA4CSV(data []byte, kind GA4Kind, into *model.Analytics) (*model.Analytics, error) {
	kind = normalizeGA4Kind(kind)
	text := decodeText(data)
	for _, blk := range splitBlocks(text) {
		delim := detectDelimiter(blk)
		recs, err := readRecords(blk, delim)
		if err != nil || len(recs) < 1 {
			continue
		}
		headers := foldAll(recs[0])
		cols := mapGA4Columns(headers)
		if len(cols.cur) == 0 {
			continue
		}
		k := kind
		if k == "" {
			k = cols.guessKind()
		}
		keyIdx, ok := cols.kindDim(k)
		if !ok {
			continue
		}
		decimalComma := delim == ';' || looksFrench(headers)
		return applyGA4(into, k, cols, keyIdx, recs[1:], decimalComma), nil
	}
	if kind != "" {
		return into, errors.New("GA4 CSV: no table with a " + string(kind) + " dimension and a known metric (sessions, users, key events...) found")
	}
	return into, errors.New("GA4 CSV: no table with a known dimension (channel group, source / medium, landing page, date) and metric (sessions, users...) found")
}

func applyGA4(a *model.Analytics, kind GA4Kind, cols ga4Cols, keyIdx int, rows [][]string, decimalComma bool) *model.Analytics {
	if a == nil {
		a = &model.Analytics{}
	}
	values := func(rec []string, prev bool) gaVals {
		idx := cols.cur
		if prev {
			idx = cols.prev
		}
		var v gaVals
		get := func(m gaMetric) (float64, bool, bool) {
			i, ok := idx[m]
			if !ok {
				return 0, false, false
			}
			return parseNumber(cell(rec, i), decimalComma)
		}
		if x, _, ok := get(metSessions); ok {
			v.sessions, v.present = x, true
		}
		if x, _, ok := get(metUsers); ok {
			v.users, v.present = x, true
		}
		if x, _, ok := get(metKeyEvents); ok {
			v.keyEvents, v.present = x, true
		}
		if x, _, ok := get(metRevenue); ok {
			v.revenue, v.present = x, true
		}
		if x, pct, ok := get(metEngRate); ok {
			if !pct && x <= 1 {
				x *= 100
			}
			v.engW, v.engSum, v.engN, v.present = x*v.sessions, x, 1, true
		}
		return v
	}

	rangeIdx, hasRange := cols.dims[dimDateRange]
	firstRange := ""
	groups := map[string]*gaGroup{}
	prevRangeDaily := map[string]*gaGroup{}
	var order, prevOrder []string
	var total, prevTotal gaVals
	hasTotal := false

	for _, rec := range rows {
		raw := cell(rec, keyIdx)
		isPrevRange := false
		if hasRange {
			label := cell(rec, rangeIdx)
			if firstRange == "" {
				firstRange = label
			}
			isPrevRange = label != firstRange
		}
		if isTotalRow(raw) {
			if isPrevRange {
				prevTotal.add(values(rec, false))
			} else {
				total.add(values(rec, false))
				prevTotal.add(values(rec, true))
			}
			hasTotal = true
			continue
		}
		key := raw
		switch kind {
		case GA4Daily:
			d, ok := parseDate(raw, decimalComma)
			if !ok {
				continue
			}
			key = d
		case GA4Sources:
			if _, ok := cols.dims[dimSourceMedium]; !ok {
				if mi, ok := cols.dims[dimMedium]; ok && cell(rec, mi) != "" {
					key = raw + " / " + cell(rec, mi)
				}
			}
		}
		if key == "" {
			continue
		}

		target, ord := groups, &order
		if isPrevRange && kind == GA4Daily {
			target, ord = prevRangeDaily, &prevOrder
		}
		g, ok := target[key]
		if !ok {
			g = &gaGroup{key: key}
			target[key] = g
			*ord = append(*ord, key)
		}
		cur := values(rec, false)
		organic, ai := 0.0, 0.0
		if kind == GA4Daily {
			if ci, ok := cols.dims[dimChannel]; ok {
				if insights.IsOrganicSearchChannel(cell(rec, ci)) {
					organic = cur.sessions
				}
			} else if src := sourceOf(rec, cols); src != "" && strings.HasSuffix(fold(src), "/ organic") {
				organic = cur.sessions
			}
			if src := sourceOf(rec, cols); src != "" && insights.DetectAIPlatform(src) != "" {
				ai = cur.sessions
			}
		}
		if isPrevRange {
			if kind == GA4Daily {
				g.cur.add(cur) // stored in the previous-range map with its own date
				g.organic += organic
				g.ai += ai
			} else {
				g.prev.add(cur)
				g.prevFromRange = true
			}
			continue
		}
		g.cur.add(cur)
		g.organic += organic
		g.ai += ai
		if len(cols.prev) > 0 {
			p := values(rec, true)
			if p.present {
				g.prev.add(p)
				g.alignedPrv = true
				// Organic / AI split of the comparison columns.
				if organic > 0 {
					g.prevOrganic += p.sessions
				}
				if ai > 0 {
					g.prevAI += p.sessions
				}
			}
		}
	}

	hasPrev := func(g *gaGroup) bool { return g.prevFromRange || g.alignedPrv }
	_, hasUsers := cols.cur[metUsers]
	_, hasEng := cols.cur[metEngRate]
	_, hasKey := cols.cur[metKeyEvents]
	_, hasRev := cols.cur[metRevenue]
	anyPrev := len(cols.prev) > 0 || (hasRange && (len(prevRangeDaily) > 0 || anyGroup(groups, hasPrev)))
	prevOf := func(g *gaGroup) *float64 {
		if hasPrev(g) {
			return model.F(g.prev.sessions)
		}
		if anyPrev && kind != GA4Daily {
			return model.F(0) // absent from the comparison period
		}
		return nil
	}

	switch kind {
	case GA4Channels:
		a.Channels = nil
		var sum, psum gaVals
		for _, k := range order {
			g := groups[k]
			a.Channels = append(a.Channels, model.ChannelRow{
				Channel: g.key, Sessions: g.cur.sessions, PrevSessions: prevOf(g), Users: g.cur.users,
				EngagementRate: g.cur.engagementRate(), KeyEvents: g.cur.keyEvents, Revenue: g.cur.revenue,
			})
			sum.add(g.cur)
			psum.add(g.prev)
			if insights.IsOrganicSearchChannel(g.key) && a.OrganicTotals == nil {
				ot := &model.GA4Totals{}
				p := (*gaVals)(nil)
				if hasPrev(g) {
					p = &g.prev
				}
				fillTotals(ot, g.cur, p, true, hasUsers, hasEng, hasKey, hasRev)
				a.OrganicTotals = ot
			}
		}
		if hasTotal {
			sum, psum = total, prevTotal
		}
		var pp *gaVals
		if anyPrev {
			pp = &psum
		}
		fillTotals(&a.Totals, sum, pp, true, hasTotal && hasUsers, hasEng, hasKey, hasRev)

	case GA4Sources:
		a.Sources = nil
		for _, k := range order {
			g := groups[k]
			src, med := splitSourceMedium(g.key)
			a.Sources = append(a.Sources, model.SourceRow{
				Source: src, Medium: med, Sessions: g.cur.sessions, PrevSessions: prevOf(g), Users: g.cur.users,
				EngagementRate: g.cur.engagementRate(), KeyEvents: g.cur.keyEvents, Revenue: g.cur.revenue,
			})
		}

	case GA4LandingPages:
		a.LandingPages = nil
		for _, k := range order {
			g := groups[k]
			a.LandingPages = append(a.LandingPages, model.LandingPageRow{
				Page: g.key, Sessions: g.cur.sessions, PrevSessions: prevOf(g),
				EngagementRate: g.cur.engagementRate(), KeyEvents: g.cur.keyEvents, Revenue: g.cur.revenue,
			})
		}

	case GA4Daily:
		a.Daily, a.PreviousDaily = nil, nil
		var sum, psum gaVals
		for _, k := range order {
			g := groups[k]
			a.Daily = append(a.Daily, model.GA4DailyPoint{Date: g.key, Sessions: g.cur.sessions, OrganicSessions: g.organic, AISessions: g.ai})
			sum.add(g.cur)
		}
		for _, k := range prevOrder {
			g := prevRangeDaily[k]
			a.PreviousDaily = append(a.PreviousDaily, model.GA4DailyPoint{Date: g.key, Sessions: g.cur.sessions, OrganicSessions: g.organic, AISessions: g.ai})
			psum.add(g.cur)
		}
		if len(cols.prev) > 0 {
			shift := shiftDates(order, isYearComparison(cols.prevHeader))
			for _, k := range order {
				g := groups[k]
				if !g.alignedPrv {
					continue
				}
				a.PreviousDaily = append(a.PreviousDaily, model.GA4DailyPoint{Date: shift(g.key), Sessions: g.prev.sessions, OrganicSessions: g.prevOrganic, AISessions: g.prevAI})
				psum.add(g.prev)
			}
		}
		sort.SliceStable(a.Daily, func(i, j int) bool { return a.Daily[i].Date < a.Daily[j].Date })
		sort.SliceStable(a.PreviousDaily, func(i, j int) bool { return a.PreviousDaily[i].Date < a.PreviousDaily[j].Date })
		if hasTotal {
			sum = total
			if prevTotal.present {
				psum = prevTotal
			}
		}
		var pp *gaVals
		if len(a.PreviousDaily) > 0 {
			pp = &psum
		}
		fillTotals(&a.Totals, sum, pp, true, hasTotal && hasUsers, hasEng, hasKey, hasRev)
	}
	return a
}

func anyGroup(groups map[string]*gaGroup, f func(*gaGroup) bool) bool {
	for _, g := range groups {
		if f(g) {
			return true
		}
	}
	return false
}

func sourceOf(rec []string, cols ga4Cols) string {
	if i, ok := cols.dims[dimSourceMedium]; ok {
		return cell(rec, i)
	}
	if i, ok := cols.dims[dimSource]; ok {
		s := cell(rec, i)
		if mi, ok := cols.dims[dimMedium]; ok && cell(rec, mi) != "" {
			s += " / " + cell(rec, mi)
		}
		return s
	}
	return ""
}

// splitSourceMedium splits "google / organic" into ("google", "organic").
func splitSourceMedium(s string) (string, string) {
	if src, med, ok := strings.Cut(s, " / "); ok {
		return strings.TrimSpace(src), strings.TrimSpace(med)
	}
	if strings.Count(s, "/") == 1 {
		src, med, _ := strings.Cut(s, "/")
		if !strings.Contains(src, ".") || !strings.Contains(med, ".") { // avoid splitting host/path
			return strings.TrimSpace(src), strings.TrimSpace(med)
		}
	}
	return strings.TrimSpace(s), ""
}

// fillTotals sets the metrics of t that are still nil.
func fillTotals(t *model.GA4Totals, cur gaVals, prev *gaVals, hasSessions, hasUsers, hasEng, hasKey, hasRev bool) {
	set := func(dst **model.Metric, c float64, p func(gaVals) float64) {
		if *dst != nil {
			return
		}
		m := &model.Metric{Current: c}
		if prev != nil && prev.present {
			m.Previous = model.F(p(*prev))
		}
		*dst = m
	}
	if hasSessions && cur.present {
		set(&t.Sessions, cur.sessions, func(v gaVals) float64 { return v.sessions })
	}
	if hasUsers {
		set(&t.Users, cur.users, func(v gaVals) float64 { return v.users })
	}
	if hasEng {
		set(&t.EngagementRate, cur.engagementRate(), func(v gaVals) float64 { return v.engagementRate() })
	}
	if hasKey {
		set(&t.KeyEvents, cur.keyEvents, func(v gaVals) float64 { return v.keyEvents })
	}
	if hasRev {
		set(&t.Revenue, round2(cur.revenue), func(v gaVals) float64 { return round2(v.revenue) })
	}
}
