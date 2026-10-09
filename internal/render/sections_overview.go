package render

import (
	"math"
	"strconv"

	"github.com/flocom/SEO-GEO-Report/internal/charts"
	"github.com/flocom/SEO-GEO-Report/internal/insights"
	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// ---------------------------------------------------------------------------
// KPI cards

func (b *builder) card(key, labelKey string, value string, m *model.Metric, prevFmt func(float64) string, d *deltaView) kpiCard {
	c := kpiCard{Key: key, Label: b.t("kpi." + labelKey + ".label"), Value: value, Delta: d}
	if b.hints {
		c.Explain = b.t("kpi." + labelKey + ".explain")
	}
	if m != nil && m.Previous != nil && prevFmt != nil {
		c.Prev = b.tf("kpi.prev", prevFmt(*m.Previous))
	}
	if st, ok := b.an.KPIs[key]; ok && st.HasDelta && c.Delta != nil {
		c.Delta.Tone = toneFromInsights(st.Tone)
		c.Delta.Arrow = arrowFor(c.Delta.Dir, c.Delta.Tone)
	}
	if c.Delta != nil {
		c.Tone = c.Delta.Tone
	}
	return c
}

func (b *builder) sparkline(dates []string, vals []float64, invert bool, f func(float64) string) (h charts.SparkOpts) {
	if f == nil {
		f = b.fmtCompact()
	}
	return charts.SparkOpts{Values: vals, Labels: dates, LabelFormat: b.fmtDateTip, Color: b.accent, InvertY: invert, Format: f}
}

func gscDates(pts []model.GSCDailyPoint) []string {
	out := make([]string, len(pts))
	for i, p := range pts {
		out[i] = p.Date
	}
	return out
}

func ga4Dates(pts []model.GA4DailyPoint) []string {
	out := make([]string, len(pts))
	for i, p := range pts {
		out[i] = p.Date
	}
	return out
}

func gscSeries(pts []model.GSCDailyPoint, f func(model.GSCDailyPoint) float64) []float64 {
	out := make([]float64, 0, len(pts))
	for _, p := range pts {
		out = append(out, f(p))
	}
	return out
}

func ga4Series(pts []model.GA4DailyPoint, f func(model.GA4DailyPoint) float64) []float64 {
	out := make([]float64, 0, len(pts))
	for _, p := range pts {
		out = append(out, f(p))
	}
	return out
}

func (b *builder) withSpark(c kpiCard, dates []string, vals []float64, invert bool, f func(float64) string) kpiCard {
	if len(vals) >= 3 && nonZero(vals) {
		c.Spark = charts.Sparkline(b.sparkline(dates, vals, invert, f))
	}
	return c
}

func (b *builder) gscKPIs() []kpiCard {
	sc := b.r.SearchConsole
	if sc == nil {
		return nil
	}
	t := sc.Totals
	if t.Clicks.Current == 0 && t.Impressions.Current == 0 && len(sc.Daily) == 0 {
		return nil
	}
	cards := []kpiCard{
		b.withSpark(b.card("gsc.clicks", "clicks", b.num(t.Clicks.Current), &t.Clicks, b.num, b.deltaRel(t.Clicks, false, b.num)),
			gscDates(sc.Daily), gscSeries(sc.Daily, func(p model.GSCDailyPoint) float64 { return p.Clicks }), false, b.num),
		b.withSpark(b.card("gsc.impressions", "impressions", b.num(t.Impressions.Current), &t.Impressions, b.num, b.deltaRel(t.Impressions, false, b.num)),
			gscDates(sc.Daily), gscSeries(sc.Daily, func(p model.GSCDailyPoint) float64 { return p.Impressions }), false, b.num),
	}
	if t.CTR != nil {
		cards = append(cards, b.withSpark(b.card("gsc.ctr", "ctr", i18nPct2(b, t.CTR.Current), t.CTR, func(v float64) string { return i18nPct2(b, v) }, b.deltaPts(*t.CTR)),
			gscDates(sc.Daily), gscSeries(sc.Daily, func(p model.GSCDailyPoint) float64 { return p.CTR }), false, func(v float64) string { return i18nPct2(b, v) }))
	}
	if t.Position != nil && t.Position.Current > 0 {
		cards = append(cards, b.withSpark(b.card("gsc.position", "position", b.pos(t.Position.Current), t.Position, b.pos, b.deltaPosition(*t.Position)),
			gscDates(sc.Daily), gscSeries(sc.Daily, func(p model.GSCDailyPoint) float64 { return p.Position }), true, b.pos))
	}
	return cards
}

func i18nPct2(b *builder, v float64) string {
	if v < 10 {
		return b.dec(v, 2) + pctSuffix(b.lang)
	}
	return b.pct(v)
}

func pctSuffix(lang string) string {
	if lang == "en" {
		return "%"
	}
	return " %"
}

func (b *builder) ga4KPIs() []kpiCard {
	a := b.r.Analytics
	if a == nil {
		return nil
	}
	var cards []kpiCard
	t := a.Totals
	if t.Sessions != nil {
		cards = append(cards, b.withSpark(b.card("ga4.sessions", "sessions", b.num(t.Sessions.Current), t.Sessions, b.num, b.deltaRel(*t.Sessions, false, b.num)),
			ga4Dates(a.Daily), ga4Series(a.Daily, func(p model.GA4DailyPoint) float64 { return p.Sessions }), false, b.num))
	}
	if a.OrganicTotals != nil && a.OrganicTotals.Sessions != nil {
		m := a.OrganicTotals.Sessions
		cards = append(cards, b.withSpark(b.card("ga4.organic_sessions", "organic_sessions", b.num(m.Current), m, b.num, b.deltaRel(*m, false, b.num)),
			ga4Dates(a.Daily), ga4Series(a.Daily, func(p model.GA4DailyPoint) float64 { return p.OrganicSessions }), false, b.num))
	}
	if t.Users != nil {
		cards = append(cards, b.card("ga4.users", "users", b.num(t.Users.Current), t.Users, b.num, b.deltaRel(*t.Users, false, b.num)))
	}
	if t.EngagementRate != nil {
		cards = append(cards, b.card("ga4.engagement_rate", "engagement_rate", b.pct(t.EngagementRate.Current), t.EngagementRate, b.pct, b.deltaPts(*t.EngagementRate)))
	}
	if t.KeyEvents != nil {
		cards = append(cards, b.card("ga4.key_events", "key_events", b.num(t.KeyEvents.Current), t.KeyEvents, b.num, b.deltaRel(*t.KeyEvents, false, b.num)))
	}
	if t.Revenue != nil {
		cards = append(cards, b.card("ga4.revenue", "revenue", b.money(t.Revenue.Current), t.Revenue, b.money, b.deltaRel(*t.Revenue, false, b.money)))
	}
	return cards
}

func (b *builder) geoKPIs() []kpiCard {
	var cards []kpiCard
	if ai := b.aiSessions(); ai != nil {
		dates, vals := b.aiDaily()
		cards = append(cards, b.withSpark(b.card("geo.ai_sessions", "ai_sessions", b.num(ai.Current), ai, b.num, b.deltaRel(*ai, false, b.num)), dates, vals, false, b.num))
	}
	if s := b.aiShare(); s != nil {
		cards = append(cards, b.card("geo.ai_share", "ai_share", i18nPct2(b, *s), nil, nil, nil))
	}
	if cr := b.citationRate(); cr != nil {
		cards = append(cards, b.card("geo.citation_rate", "citation_rate", b.pct0(cr.Current), cr, b.pct0, b.deltaPts(*cr)))
	}
	if g := b.r.GEO; g != nil {
		for _, s := range g.ShareOfVoice {
			if s.IsSelf {
				m := metricOf(s.Share, s.Previous)
				cards = append(cards, b.card("geo.share_of_voice", "share_of_voice", b.pct(s.Share), &m, b.pct, b.deltaPts(m)))
				break
			}
		}
		if len(g.AIOverviews) > 0 {
			present, cited := 0, 0
			for _, o := range g.AIOverviews {
				if o.OverviewPresent {
					present++
					if o.Cited {
						cited++
					}
				}
			}
			if present > 0 {
				c := b.card("geo.aio_cited", "aio_cited", strconv.Itoa(cited)+" / "+strconv.Itoa(present), nil, nil, nil)
				c.Prev = b.tf("kpi.aio_cited.sub", b.pct0(float64(cited)/float64(present)*100))
				cards = append(cards, c)
			}
		}
	}
	return cards
}

// ---------------------------------------------------------------------------
// Summary ("At a glance")

func (b *builder) progressVerdict(p float64) (key, tone string) {
	switch {
	case p >= 65:
		return "strong_up", "pos"
	case p >= 55:
		return "up", "pos"
	case p > 45:
		return "stable", "neu"
	case p > 35:
		return "down", "neg"
	}
	return "strong_down", "neg"
}

func (b *builder) summary() *section {
	s := &section{ID: "summary"}
	n := b.r.Narrative

	// Hero band: gauge + verdict + headline numbers.
	h := &hero{Period: b.periodLine()}
	all := map[string]kpiCard{}
	for _, c := range append(append(b.gscKPIs(), b.ga4KPIs()...), b.geoKPIs()...) {
		all[c.Key] = c
	}
	// The headline numbers duplicate the KPI cards that follow; they are only
	// shown when the KPI section is hidden.
	for _, k := range []string{"gsc.clicks", "gsc.position", "ga4.sessions", "ga4.organic_sessions", "ga4.key_events", "geo.ai_sessions", "geo.citation_rate", "gsc.impressions"} {
		if c, ok := all[k]; ok && len(h.Stats) < 4 && b.r.SectionHidden("kpis") {
			c.Spark, c.Explain, c.Prev = "", "", ""
			h.Stats = append(h.Stats, c)
		}
	}
	if p := b.an.Progress; p != nil {
		v := math.Max(0, math.Min(100, *p))
		key, tone := b.progressVerdict(v)
		h.Score = b.num(v)
		h.Verdict = b.t("hero.verdict." + key)
		h.Desc = b.t("hero.verdict." + key + ".desc")
		h.Tone = tone
		h.Scale = b.t("hero.scale")
		h.Gauge = b.gauge(charts.GaugeOpts{
			Value: v, Min: 0, Max: 100, Display: h.Score, Label: b.t("hero.progress"),
			Bands: []charts.GaugeBand{{Upto: 35, Color: charts.ColorNegative}, {Upto: 45, Color: "#d98a3a"}, {Upto: 55, Color: charts.ColorPrevious}, {Upto: 65, Color: "#82b896"}, {Upto: 100, Color: charts.ColorPositive}},
			Title: b.t("hero.progress"),
		})
	} else if len(all) > 0 {
		h.NoCompare = b.t("hero.no_comparison")
	}
	if h.Score != "" || len(h.Stats) > 0 || h.NoCompare != "" {
		s.Blocks = append(s.Blocks, block{Kind: "hero", Span: 12, Hero: h, Hint: b.hint("progress"), HintKind: "chart"})
	}

	if n != nil && n.ExecutiveSummary != "" {
		s.Blocks = append(s.Blocks, block{Kind: "markdown", Span: 12, Title: b.t("summary.exec"), HTML: Markdown(n.ExecutiveSummary)})
	}
	if n != nil && (len(n.Highlights) > 0 || len(n.Concerns) > 0) {
		span := 6
		if len(n.Highlights) == 0 || len(n.Concerns) == 0 {
			span = 12
		}
		if len(n.Highlights) > 0 {
			s.Blocks = append(s.Blocks, block{Kind: "list", Span: span, Tone: "pos", Title: b.t("summary.highlights"), Items: mdItems(n.Highlights)})
		}
		if len(n.Concerns) > 0 {
			s.Blocks = append(s.Blocks, block{Kind: "list", Span: span, Tone: "neg", Title: b.t("summary.concerns"), Items: mdItems(n.Concerns)})
		}
	}
	if b.auto && len(b.an.Insights) > 0 {
		var cs []callout
		for _, in := range topN(b.an.Insights, 4) {
			cs = append(cs, b.insightCallout(in))
		}
		s.Blocks = append(s.Blocks, block{Kind: "callouts", Span: 12, Title: b.t("insights.title"), Sub: b.t("insights.auto"), Callouts: cs})
	}
	if len(s.Blocks) == 0 {
		if b.r.SearchConsole == nil && b.r.Analytics == nil && b.r.GEO == nil {
			s.Blocks = append(s.Blocks, block{Kind: "callouts", Span: 12, Callouts: []callout{{Tone: "info", Icon: icoInfo, Title: b.t("empty.title"), Text: b.t("empty.text")}}})
			return s
		}
		return nil
	}
	return s
}

func mdItems(items []string) []listItem {
	var out []listItem
	for _, it := range items {
		if it == "" {
			continue
		}
		out = append(out, listItem{HTML: inlineMarkdown(it)})
	}
	return out
}

func (b *builder) insightCallout(in insights.Insight) callout {
	tone := toneFromInsights(in.Tone)
	c := callout{Tone: tone, Text: in.Text}
	switch tone {
	case "pos":
		c.Icon = icoTrendUp
	case "neg":
		c.Icon = icoTrendDown
	default:
		c.Icon = icoInfo
	}
	return c
}

// sectionInsights returns the automatic insights attached to a section.
func (b *builder) sectionInsights(id string) []block {
	if !b.auto {
		return nil
	}
	var cs []callout
	for _, in := range b.an.Insights {
		if in.Section == id && in.Text != "" {
			cs = append(cs, b.insightCallout(in))
		}
	}
	if len(cs) == 0 {
		return nil
	}
	return []block{{Kind: "callouts", Span: 12, Callouts: cs}}
}

func (b *builder) periodLine() string {
	m := b.r.Meta
	p := b.dateRange(m.Period)
	if m.ComparisonPeriod != nil {
		return p + "  ·  " + b.tf("hero.vs", b.dateRange(*m.ComparisonPeriod))
	}
	return p
}

// ---------------------------------------------------------------------------
// KPIs

func (b *builder) kpis() *section {
	s := &section{ID: "kpis"}
	all := map[string]kpiCard{}
	var order []string
	for _, c := range append(append(b.gscKPIs(), b.ga4KPIs()...), b.geoKPIs()...) {
		all[c.Key] = c
		order = append(order, c.Key)
	}
	// The beginner view keeps a handful of cards: the preferred ones first,
	// then whatever else exists, up to six.
	preferred := []string{"gsc.clicks", "gsc.impressions", "ga4.organic_sessions", "geo.ai_sessions", "ga4.key_events", "gsc.position"}
	if _, ok := all["ga4.organic_sessions"]; !ok {
		preferred[2] = "ga4.sessions"
	}
	var cards []kpiCard
	used := map[string]bool{}
	for _, k := range append(preferred, order...) {
		if c, ok := all[k]; ok && !used[k] && len(cards) < 6 {
			cards = append(cards, c)
			used[k] = true
		}
	}
	if len(cards) == 0 {
		return nil
	}
	s.Blocks = append(s.Blocks, block{Kind: "kpis", Span: 12, Tone: "hero", KPIs: cards})

	// One or two big, simple charts.
	if sc := b.r.SearchConsole; sc != nil && len(sc.Daily) >= 2 {
		var dates []string
		for _, p := range sc.Daily {
			dates = append(dates, p.Date)
		}
		clicks := func(p model.GSCDailyPoint) float64 { return p.Clicks }
		s.Blocks = add(s.Blocks, b.trend(b.t("chart.clicks_daily"), dates, gscSeries(sc.Daily, clicks), gscSeries(sc.PreviousDaily, clicks), b.fmtCompact(), false, true), 6, b.hint("clicks_daily"))
	} else if a := b.r.Analytics; a != nil && len(a.Daily) >= 2 {
		var dates []string
		for _, p := range a.Daily {
			dates = append(dates, p.Date)
		}
		sess := func(p model.GA4DailyPoint) float64 { return p.Sessions }
		s.Blocks = add(s.Blocks, b.trend(b.t("chart.sessions_daily"), dates, ga4Series(a.Daily, sess), ga4Series(a.PreviousDaily, sess), b.fmtCompact(), false, true), 6, b.hint("sessions_glance"))
	}
	if a := b.r.Analytics; a != nil && len(a.Channels) > 1 {
		s.Blocks = add(s.Blocks, b.channelDonut(a.Channels), 6, b.hint("channels"))
	}
	if b.hints {
		s.Blocks = append(s.Blocks, block{Kind: "callouts", Span: 12, Callouts: []callout{{Tone: "info", Icon: icoBulb, Title: b.t("hint.title_kpi"), Text: b.t("hint.kpis")}}})
	}
	return s
}
