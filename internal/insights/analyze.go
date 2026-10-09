package insights

import (
	"math"
	"sort"
	"strings"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// Thresholds under which a change is considered "stable" (Neutral).
const (
	neutralPct      = 2.0 // |relative change| below 2 % is neutral
	neutralPosition = 0.3 // |position change| below 0.3 is neutral
	maxInsights     = 15
	topMovers       = 5
)

// KPI keys produced by Analyze.
const (
	KeyGSCClicks         = "gsc.clicks"
	KeyGSCImpressions    = "gsc.impressions"
	KeyGSCCTR            = "gsc.ctr"
	KeyGSCPosition       = "gsc.position"
	KeyGSCIndexed        = "gsc.indexed"
	KeyGA4Sessions       = "ga4.sessions"
	KeyGA4Users          = "ga4.users"
	KeyGA4EngagementRate = "ga4.engagement_rate"
	KeyGA4KeyEvents      = "ga4.key_events"
	KeyGA4Revenue        = "ga4.revenue"
	KeyGA4Organic        = "ga4.organic_sessions"
	KeyGEOAISessions     = "geo.ai_sessions"
	KeyGEOCitationRate   = "geo.citation_rate"
)

func analyze(r *model.Report) Analysis {
	a := Analysis{KPIs: map[string]KPIStatus{}, Insights: []Insight{}, AIReferrals: []model.AIReferralRow{}}
	if r == nil {
		return a
	}
	sc, ga, geo := r.SearchConsole, r.Analytics, r.GEO

	// --- Search Console KPIs
	if sc != nil {
		a.KPIs[KeyGSCClicks] = kpiStatus(KeyGSCClicks, sc.Totals.Clicks, false)
		a.KPIs[KeyGSCImpressions] = kpiStatus(KeyGSCImpressions, sc.Totals.Impressions, false)
		if ctr := gscCTR(sc); ctr != nil {
			a.KPIs[KeyGSCCTR] = kpiStatus(KeyGSCCTR, *ctr, false)
		}
		if sc.Totals.Position != nil && sc.Totals.Position.Current > 0 {
			a.KPIs[KeyGSCPosition] = kpiStatus(KeyGSCPosition, *sc.Totals.Position, true)
		}
		if sc.Indexing != nil {
			a.KPIs[KeyGSCIndexed] = kpiStatus(KeyGSCIndexed, sc.Indexing.Indexed, false)
		}
		a.Positions = positions(sc)
		a.Winners, a.Losers = movers(sc.Queries)
	}

	// --- GA4 KPIs
	if ga != nil {
		t := ga.Totals
		for _, k := range []struct {
			key string
			m   *model.Metric
		}{
			{KeyGA4Sessions, t.Sessions},
			{KeyGA4Users, t.Users},
			{KeyGA4EngagementRate, t.EngagementRate},
			{KeyGA4KeyEvents, t.KeyEvents},
			{KeyGA4Revenue, t.Revenue},
		} {
			if k.m != nil {
				a.KPIs[k.key] = kpiStatus(k.key, *k.m, false)
			}
		}
		if org := organicSessions(ga); org != nil {
			a.KPIs[KeyGA4Organic] = kpiStatus(KeyGA4Organic, *org, false)
		}
	}

	// --- GEO
	a.AIReferrals, a.AISessions = aiReferrals(r)
	if a.AISessions != nil {
		a.KPIs[KeyGEOAISessions] = kpiStatus(KeyGEOAISessions, *a.AISessions, false)
		if total := totalSessions(ga); total > 0 {
			a.AIShare = model.F(round(a.AISessions.Current/total*100, 2))
		}
	}
	if geo != nil {
		a.CitationRate = citationRate(geo.Citations)
		if a.CitationRate != nil {
			a.KPIs[KeyGEOCitationRate] = kpiStatus(KeyGEOCitationRate, *a.CitationRate, false)
		}
	}

	a.Progress = progressIndex(r, &a)
	if *r.Opt().AutoInsights {
		a.Insights = buildInsights(r, &a)
	}
	return a
}

// kpiStatus qualifies a metric. For lowerBetter metrics (average position) a
// decrease is positive and changes smaller than 0.3 positions are neutral;
// otherwise changes smaller than 2 % are neutral.
func kpiStatus(key string, m model.Metric, lowerBetter bool) KPIStatus {
	st := KPIStatus{Key: key, Tone: toneOf(m, lowerBetter)}
	if pct, ok := m.DeltaPct(); ok {
		st.DeltaPct = round(pct, 2)
		st.HasDelta = true
	}
	return st
}

func toneOf(m model.Metric, lowerBetter bool) Tone {
	d, ok := m.Delta()
	if !ok {
		return Neutral
	}
	if lowerBetter {
		switch {
		case math.Abs(d) < neutralPosition:
			return Neutral
		case d < 0:
			return Positive
		default:
			return Negative
		}
	}
	if pct, ok := m.DeltaPct(); ok {
		if math.Abs(pct) < neutralPct {
			return Neutral
		}
	} else if d == 0 {
		return Neutral
	}
	if d > 0 {
		return Positive
	}
	return Negative
}

// gscCTR returns totals.ctr or computes it from clicks and impressions.
func gscCTR(sc *model.SearchConsole) *model.Metric {
	if sc.Totals.CTR != nil {
		return sc.Totals.CTR
	}
	t := sc.Totals
	if t.Impressions.Current <= 0 {
		return nil
	}
	m := model.Metric{Current: t.Clicks.Current / t.Impressions.Current * 100}
	if t.Clicks.Previous != nil && t.Impressions.Previous != nil && *t.Impressions.Previous > 0 {
		m.Previous = model.F(*t.Clicks.Previous / *t.Impressions.Previous * 100)
	}
	return &m
}

// IsOrganicSearchChannel reports whether a GA4 channel name is the Organic
// Search default channel group (English or French label).
func IsOrganicSearchChannel(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	return n == "organic" || strings.Contains(n, "organic search") ||
		strings.Contains(n, "recherche organique") || strings.Contains(n, "recherche naturelle")
}

// organicSessions returns organic_totals.sessions or the Organic Search
// channel row.
func organicSessions(ga *model.Analytics) *model.Metric {
	if ga == nil {
		return nil
	}
	if ga.OrganicTotals != nil && ga.OrganicTotals.Sessions != nil {
		return ga.OrganicTotals.Sessions
	}
	for _, c := range ga.Channels {
		if IsOrganicSearchChannel(c.Channel) {
			return &model.Metric{Current: c.Sessions, Previous: c.PrevSessions}
		}
	}
	return nil
}

// totalSessions returns the site-wide sessions from totals, channels or the
// daily series.
func totalSessions(ga *model.Analytics) float64 {
	if ga == nil {
		return 0
	}
	if ga.Totals.Sessions != nil && ga.Totals.Sessions.Current > 0 {
		return ga.Totals.Sessions.Current
	}
	var s float64
	for _, c := range ga.Channels {
		s += c.Sessions
	}
	if s > 0 {
		return s
	}
	for _, d := range ga.Daily {
		s += d.Sessions
	}
	return s
}

// aiReferrals returns geo.ai_referrals (platform filled when missing) or the
// AI sources detected in analytics.sources aggregated by platform, sorted by
// sessions desc, plus their total.
func aiReferrals(r *model.Report) ([]model.AIReferralRow, *model.Metric) {
	var rows []model.AIReferralRow
	if r.GEO != nil && len(r.GEO.AIReferrals) > 0 {
		for _, row := range r.GEO.AIReferrals {
			if strings.TrimSpace(row.Platform) == "" {
				row.Platform = DetectAIPlatform(row.Source)
				if row.Platform == "" {
					row.Platform = row.Source
				}
			}
			rows = append(rows, row)
		}
	} else if r.Analytics != nil {
		rows = DetectAIReferrals(r.Analytics.Sources)
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Sessions > rows[j].Sessions })

	if len(rows) > 0 {
		m := model.Metric{}
		var prev float64
		hasPrev := false
		for _, row := range rows {
			m.Current += row.Sessions
			if row.PrevSessions != nil {
				prev += *row.PrevSessions
				hasPrev = true
			}
		}
		if hasPrev {
			m.Previous = model.F(prev)
		}
		return rows, &m
	}

	// No per-platform rows: fall back on daily series.
	if r.GEO != nil && len(r.GEO.AIDaily) > 0 {
		m := model.Metric{}
		for _, d := range r.GEO.AIDaily {
			m.Current += d.Value
		}
		return []model.AIReferralRow{}, &m
	}
	if ga := r.Analytics; ga != nil {
		var cur, prev float64
		for _, d := range ga.Daily {
			cur += d.AISessions
		}
		for _, d := range ga.PreviousDaily {
			prev += d.AISessions
		}
		if cur > 0 {
			m := model.Metric{Current: cur}
			if prev > 0 {
				m.Previous = model.F(prev)
			}
			return []model.AIReferralRow{}, &m
		}
	}
	return []model.AIReferralRow{}, nil
}

// DetectAIReferrals extracts AI-assistant traffic from GA4 source rows and
// aggregates it by platform (sessions, users, key events and revenue summed,
// engagement rate weighted by sessions). Source lists the contributing GA4
// sources, comma separated. The result is sorted by sessions desc.
func DetectAIReferrals(sources []model.SourceRow) []model.AIReferralRow {
	type agg struct {
		row     model.AIReferralRow
		engSum  float64
		srcs    []string
		hasPrev bool
		prev    float64
	}
	byPlatform := map[string]*agg{}
	var order []string
	for _, s := range sources {
		p := DetectAIPlatform(s.Source)
		if p == "" && s.Medium != "" {
			p = DetectAIPlatform(s.Source + "/" + s.Medium)
		}
		if p == "" {
			continue
		}
		g, ok := byPlatform[p]
		if !ok {
			g = &agg{row: model.AIReferralRow{Platform: p}}
			byPlatform[p] = g
			order = append(order, p)
		}
		g.row.Sessions += s.Sessions
		g.row.Users += s.Users
		g.row.KeyEvents += s.KeyEvents
		g.row.Revenue += s.Revenue
		g.engSum += s.EngagementRate * s.Sessions
		if s.PrevSessions != nil {
			g.hasPrev = true
			g.prev += *s.PrevSessions
		}
		src := strings.TrimSpace(s.Source)
		dup := false
		for _, x := range g.srcs {
			if x == src {
				dup = true
			}
		}
		if !dup && src != "" {
			g.srcs = append(g.srcs, src)
		}
	}
	out := make([]model.AIReferralRow, 0, len(order))
	for _, p := range order {
		g := byPlatform[p]
		if g.row.Sessions > 0 && g.engSum > 0 {
			g.row.EngagementRate = round(g.engSum/g.row.Sessions, 2)
		}
		if g.hasPrev {
			g.row.PrevSessions = model.F(g.prev)
		}
		g.row.Source = strings.Join(g.srcs, ", ")
		out = append(out, g.row)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Sessions > out[j].Sessions })
	return out
}

// positions returns search_console.position_distribution or computes it from
// the queries (previous buckets from prev_position when available).
func positions(sc *model.SearchConsole) *model.PositionDistribution {
	if sc.PositionDistribution != nil {
		return sc.PositionDistribution
	}
	var cur, prev [4]float64
	found, anyPrev := false, false
	bucket := func(p float64) int {
		switch {
		case p <= 3:
			return 0
		case p <= 10:
			return 1
		case p <= 20:
			return 2
		default:
			return 3
		}
	}
	for _, q := range sc.Queries {
		if q.Position > 0 {
			cur[bucket(q.Position)]++
			found = true
		}
		if q.PrevPosition != nil && *q.PrevPosition > 0 {
			prev[bucket(*q.PrevPosition)]++
			anyPrev = true
		}
	}
	if !found {
		return nil
	}
	mk := func(i int) model.Metric {
		m := model.Metric{Current: cur[i]}
		if anyPrev {
			m.Previous = model.F(prev[i])
		}
		return m
	}
	return &model.PositionDistribution{Top3: mk(0), Top10: mk(1), Top20: mk(2), Beyond20: mk(3)}
}

// citationRate is the percentage of checks where the site is cited; the
// previous rate uses the checks that carry previously_cited.
func citationRate(checks []model.CitationCheck) *model.Metric {
	if len(checks) == 0 {
		return nil
	}
	var cited, prevN, prevCited float64
	for _, c := range checks {
		if c.Cited {
			cited++
		}
		if c.PreviouslyCited != nil {
			prevN++
			if *c.PreviouslyCited {
				prevCited++
			}
		}
	}
	m := model.Metric{Current: round(cited/float64(len(checks))*100, 2)}
	if prevN > 0 {
		m.Previous = model.F(round(prevCited/prevN*100, 2))
	}
	return &m
}

// movers returns the queries with the largest click gains and losses
// (top 5 each), using prev_clicks.
func movers(rows []model.GSCRow) (winners, losers []model.GSCRow) {
	for _, q := range rows {
		if q.PrevClicks == nil {
			continue
		}
		d := q.Clicks - *q.PrevClicks
		if d > 0 {
			winners = append(winners, q)
		} else if d < 0 {
			losers = append(losers, q)
		}
	}
	delta := func(q model.GSCRow) float64 { return q.Clicks - *q.PrevClicks }
	sort.SliceStable(winners, func(i, j int) bool {
		di, dj := delta(winners[i]), delta(winners[j])
		if di != dj {
			return di > dj
		}
		return winners[i].Key < winners[j].Key
	})
	sort.SliceStable(losers, func(i, j int) bool {
		di, dj := delta(losers[i]), delta(losers[j])
		if di != dj {
			return di < dj
		}
		return losers[i].Key < losers[j].Key
	})
	if len(winners) > topMovers {
		winners = winners[:topMovers]
	}
	if len(losers) > topMovers {
		losers = losers[:topMovers]
	}
	return winners, losers
}

// progressIndex computes the composite progress index.
//
// Each available component i yields a relative change d_i in percent:
//
//	clicks, impressions, organic sessions, AI sessions, key events,
//	citation rate: (current - previous) / previous × 100
//	                (+100 when previous is 0 and current > 0)
//	average position: (previous - current) / previous × 100 (lower is better)
//
// The change is squashed with s_i = tanh(d_i / 50), so that +50 % ≈ +0.76 and
// outliers (e.g. +900 % AI sessions) saturate at ±1 instead of dominating.
// With the weights w below (clicks 0.25, impressions 0.10, position 0.15,
// organic sessions 0.15, AI sessions 0.10, citation rate 0.10, key events
// 0.15), renormalised over the available components:
//
//	progress = 50 + 50 × Σ(w_i × s_i) / Σ(w_i)
//
// The result is in [0, 100], 50 meaning stable; nil when no component has
// comparison data.
func progressIndex(r *model.Report, a *Analysis) *float64 {
	type comp struct {
		w           float64
		m           *model.Metric
		lowerBetter bool
	}
	var comps []comp
	if sc := r.SearchConsole; sc != nil {
		comps = append(comps,
			comp{0.25, &sc.Totals.Clicks, false},
			comp{0.10, &sc.Totals.Impressions, false},
			comp{0.15, sc.Totals.Position, true},
		)
	}
	if ga := r.Analytics; ga != nil {
		comps = append(comps, comp{0.15, organicSessions(ga), false}, comp{0.15, ga.Totals.KeyEvents, false})
	}
	comps = append(comps, comp{0.10, a.AISessions, false}, comp{0.10, a.CitationRate, false})

	var sumW, sum float64
	for _, c := range comps {
		d, ok := indexDelta(c.m, c.lowerBetter)
		if !ok {
			continue
		}
		sumW += c.w
		sum += c.w * math.Tanh(d/50)
	}
	if sumW == 0 {
		return nil
	}
	v := round(50+50*sum/sumW, 1)
	return &v
}

func indexDelta(m *model.Metric, lowerBetter bool) (float64, bool) {
	if m == nil || m.Previous == nil {
		return 0, false
	}
	prev, cur := *m.Previous, m.Current
	if lowerBetter {
		if prev <= 0 || cur <= 0 {
			return 0, false
		}
		return (prev - cur) / prev * 100, true
	}
	if prev == 0 {
		if cur > 0 {
			return 100, true
		}
		return 0, true
	}
	return (cur - prev) / math.Abs(prev) * 100, true
}
