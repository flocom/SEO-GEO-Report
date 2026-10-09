package render

import (
	"math"
	"sort"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// Accessors preferring the insights analysis, with light fallbacks on the raw
// model so that the report stays complete even when the analysis is empty.

func (b *builder) aiReferrals() []model.AIReferralRow {
	if len(b.an.AIReferrals) > 0 {
		return b.an.AIReferrals
	}
	if g := b.r.GEO; g != nil && len(g.AIReferrals) > 0 {
		return sortedCopy(g.AIReferrals, func(r model.AIReferralRow) float64 { return r.Sessions })
	}
	return nil
}

func (b *builder) aiSessions() *model.Metric {
	if b.an.AISessions != nil {
		return b.an.AISessions
	}
	rows := b.aiReferrals()
	if len(rows) == 0 {
		return nil
	}
	m := model.Metric{}
	prev, allPrev := 0.0, true
	for _, r := range rows {
		m.Current += r.Sessions
		if r.PrevSessions == nil {
			allPrev = false
		} else {
			prev += *r.PrevSessions
		}
	}
	if allPrev {
		m.Previous = model.F(prev)
	}
	return &m
}

func (b *builder) aiShare() *float64 {
	if b.an.AIShare != nil {
		return b.an.AIShare
	}
	ai := b.aiSessions()
	if ai == nil || b.r.Analytics == nil || b.r.Analytics.Totals.Sessions == nil || b.r.Analytics.Totals.Sessions.Current <= 0 {
		return nil
	}
	return model.F(ai.Current / b.r.Analytics.Totals.Sessions.Current * 100)
}

func (b *builder) aiDaily() (labels []string, values []float64) {
	if g := b.r.GEO; g != nil && len(g.AIDaily) > 0 {
		for _, p := range g.AIDaily {
			labels = append(labels, p.Date)
			values = append(values, p.Value)
		}
		return labels, values
	}
	if a := b.r.Analytics; a != nil {
		for _, p := range a.Daily {
			labels = append(labels, p.Date)
			values = append(values, p.AISessions)
		}
		if nonZero(values) {
			return labels, values
		}
	}
	return nil, nil
}

func (b *builder) citationRate() *model.Metric {
	if b.an.CitationRate != nil {
		return b.an.CitationRate
	}
	g := b.r.GEO
	if g == nil || len(g.Citations) == 0 {
		return nil
	}
	cited, prevN, prevCited := 0, 0, 0
	for _, c := range g.Citations {
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
	m := model.Metric{Current: float64(cited) / float64(len(g.Citations)) * 100}
	if prevN > 0 {
		m.Previous = model.F(float64(prevCited) / float64(prevN) * 100)
	}
	return &m
}

func (b *builder) positions() *model.PositionDistribution {
	if b.an.Positions != nil {
		return b.an.Positions
	}
	if sc := b.r.SearchConsole; sc != nil {
		return sc.PositionDistribution
	}
	return nil
}

func posDistTotal(p *model.PositionDistribution) float64 {
	return p.Top3.Current + p.Top10.Current + p.Top20.Current + p.Beyond20.Current
}

// winnersLosers returns the queries with the largest click gains and losses.
func (b *builder) winnersLosers() (win, lose []model.GSCRow) {
	if len(b.an.Winners) > 0 || len(b.an.Losers) > 0 {
		return b.an.Winners, b.an.Losers
	}
	sc := b.r.SearchConsole
	if sc == nil {
		return nil, nil
	}
	var withPrev []model.GSCRow
	for _, q := range sc.Queries {
		if q.PrevClicks != nil {
			withPrev = append(withPrev, q)
		}
	}
	delta := func(q model.GSCRow) float64 { return q.Clicks - *q.PrevClicks }
	sort.SliceStable(withPrev, func(i, j int) bool { return delta(withPrev[i]) > delta(withPrev[j]) })
	for _, q := range withPrev {
		if delta(q) > 0 && len(win) < 5 {
			win = append(win, q)
		}
	}
	for i := len(withPrev) - 1; i >= 0 && len(lose) < 5; i-- {
		if delta(withPrev[i]) < 0 {
			lose = append(lose, withPrev[i])
		}
	}
	return win, lose
}

// ctrByPosition aggregates queries into position buckets (CTR vs position).
func ctrByPosition(rows []model.GSCRow) (labels []int, ctr []float64, n int) {
	type agg struct{ clicks, impr float64 }
	buckets := make([]agg, 4)
	for _, q := range rows {
		if q.Position <= 0 || q.Impressions <= 0 {
			continue
		}
		i := 3
		switch {
		case q.Position < 3.5:
			i = 0
		case q.Position < 10.5:
			i = 1
		case q.Position < 20.5:
			i = 2
		}
		buckets[i].clicks += q.Clicks
		buckets[i].impr += q.Impressions
		n++
	}
	for i, a := range buckets {
		if a.impr > 0 {
			labels = append(labels, i)
			ctr = append(ctr, math.Round(a.clicks/a.impr*1000)/10)
		}
	}
	return labels, ctr, n
}
