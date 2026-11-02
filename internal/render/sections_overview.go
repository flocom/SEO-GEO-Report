package render

import (
	"html/template"
	"math"
	"strconv"
	"strings"

	"github.com/flocom/SEO-GEO-Report/internal/charts"
	"github.com/flocom/SEO-GEO-Report/internal/i18n"
	"github.com/flocom/SEO-GEO-Report/internal/insights"
	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// ---------------------------------------------------------------------------
// KPI cards

func (b *builder) card(key, labelKey string, value string, m *model.Metric, prevFmt func(float64) string, d *deltaView) kpiCard {
	c := kpiCard{Key: key, Label: b.t("kpi." + labelKey + ".label"), Value: value, Delta: d, Icon: kpiIcon(key)}
	// Plain-language title first, the technical term as a small subtitle.
	if i18n.Has("kpi." + labelKey + ".plain") {
		c.Term = c.Label
		if i18n.Has("kpi." + labelKey + ".term") {
			c.Term = b.t("kpi." + labelKey + ".term")
		}
		c.Label = b.t("kpi." + labelKey + ".plain")
	}
	if b.hints {
		c.Explain = b.t("kpi." + labelKey + ".explain")
		if isCTRKey(labelKey) && m != nil {
			c.Explain = b.per100(m.Current)
		}
	}
	if m != nil && m.Previous != nil && prevFmt != nil {
		c.Prev = b.tf("kpi.prev", prevFmt(*m.Previous))
	}
	if st, ok := b.an.KPIs[key]; ok && st.HasDelta && c.Delta != nil {
		c.Delta.Tone = toneFromInsights(st.Tone)
		if st.Expected {
			c.Delta.Tone = "exp"
			c.Note = st.Note
		}
		c.Delta.Arrow = arrowFor(c.Delta.Dir, c.Delta.Tone)
	}
	if c.Delta != nil {
		c.Tone = c.Delta.Tone
	}
	// A small before / after visual keeps cards without daily data as full
	// as their neighbours (not for positions: a longer bar would be worse).
	if m != nil && m.Previous != nil && !strings.HasSuffix(labelKey, "position") {
		c.Mini = b.miniBars(m.Current, *m.Previous, c.Tone)
	}
	return c
}

func isCTRKey(k string) bool { return k == "ctr" || strings.HasSuffix(k, "_ctr") }

// per100 explains a click-through rate in words: "Sur 100 affichages,
// environ 3 clics."
func (b *builder) per100(ctr float64) string {
	switch {
	case ctr <= 0:
		return b.t("kpi.ctr.explain")
	case ctr < 1:
		return b.tf("kpi.ctr.per1000", b.num(ctr*10))
	}
	return b.tf("kpi.ctr.per100", b.num(ctr))
}

// miniBars draws two thin bars (before / now) scaled to the larger value.
func (b *builder) miniBars(cur, prev float64, tone string) template.HTML {
	mx := math.Max(cur, prev)
	if mx <= 0 || cur < 0 || prev < 0 {
		return ""
	}
	if tone == "" {
		tone = "neu"
	}
	row := func(cls, label string, v float64) string {
		return `<div class="mb ` + cls + `"><span class="mb-l">` + template.HTMLEscapeString(label) + `</span><span class="mb-t"><i style="` + string(barStyle(v, mx)) + `"></i></span></div>`
	}
	return template.HTML(`<div class="kpi-mini t-` + tone + `" aria-hidden="true">` + //nolint:gosec // escaped labels, numeric styles
		row("mb-prev", b.t("mini.before"), prev) + row("mb-now", b.t("mini.now"), cur) + `</div>`)
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
	if len(vals) < 3 || !nonZero(vals) {
		return c
	}
	// Daily zig-zags of tiny volumes, or of long periods, say little on a
	// small card: weekly points show the trend.
	rate := strings.HasSuffix(c.Key, "ctr") || strings.HasSuffix(c.Key, "position")
	weekly := len(vals) >= 14 && len(vals) > longPeriodDays
	if !rate && wantWeekly(vals, true) {
		weekly = true
	}
	if rate && len(dates) >= 14 {
		weekly = weekly || b.lowSearchVolume(c.Key)
	}
	if weekly && len(dates) == len(vals) {
		mode := aggSum
		if rate {
			mode = aggMean
		}
		w := b.toWeekly(dates, vals, nil, mode)
		o := b.sparkline(w.labels, w.cur, invert, f)
		o.LabelFormat = w.tip(b.fmtDateTip)
		c.Spark = charts.Sparkline(o)
		return c
	}
	c.Spark = charts.Sparkline(b.sparkline(dates, vals, invert, f))
	return c
}

// lowSearchVolume reports whether the daily clicks of the engine of a KPI key
// ("gsc.ctr", "bing.position") are too few to be read day by day.
func (b *builder) lowSearchVolume(key string) bool {
	var daily []model.GSCDailyPoint
	switch {
	case strings.HasPrefix(key, "gsc.") && b.r.SearchConsole != nil:
		daily = b.r.SearchConsole.Daily
	case strings.HasPrefix(key, "bing.") && b.r.Bing != nil:
		daily = b.r.Bing.Daily
	}
	return len(daily) > 0 && lowVolume(gscSeries(daily, func(p model.GSCDailyPoint) float64 { return p.Clicks }))
}

func (b *builder) gscKPIs() []kpiCard {
	sc := b.r.SearchConsole
	if sc == nil {
		return nil
	}
	return b.searchKPIs("gsc.", "", sc.Totals, sc.Daily)
}

// searchKPIs builds the clicks / impressions / CTR / position cards of a
// search engine. Keys are keyPrefix + metric (gsc.clicks) and labels come
// from kpi.<labelPrefix><metric>.label.
func (b *builder) searchKPIs(keyPrefix, labelPrefix string, t model.GSCTotals, daily []model.GSCDailyPoint) []kpiCard {
	if t.Clicks.Current == 0 && t.Impressions.Current == 0 && len(daily) == 0 {
		return nil
	}
	dates := gscDates(daily)
	cards := []kpiCard{
		b.withSpark(b.card(keyPrefix+"clicks", labelPrefix+"clicks", b.num(t.Clicks.Current), &t.Clicks, b.num, b.deltaRel(t.Clicks, false, b.num)),
			dates, gscSeries(daily, func(p model.GSCDailyPoint) float64 { return p.Clicks }), false, b.num),
		b.withSpark(b.card(keyPrefix+"impressions", labelPrefix+"impressions", b.num(t.Impressions.Current), &t.Impressions, b.num, b.deltaRel(t.Impressions, false, b.num)),
			dates, gscSeries(daily, func(p model.GSCDailyPoint) float64 { return p.Impressions }), false, b.num),
	}
	if t.CTR != nil {
		cards = append(cards, b.withSpark(b.card(keyPrefix+"ctr", labelPrefix+"ctr", i18nPct2(b, t.CTR.Current), t.CTR, func(v float64) string { return i18nPct2(b, v) }, b.deltaPts(*t.CTR)),
			dates, gscSeries(daily, func(p model.GSCDailyPoint) float64 { return p.CTR }), false, func(v float64) string { return i18nPct2(b, v) }))
	}
	if t.Position != nil && t.Position.Current > 0 {
		cards = append(cards, b.withSpark(b.card(keyPrefix+"position", labelPrefix+"position", b.pos(t.Position.Current), t.Position, b.pos, b.deltaPosition(*t.Position)),
			dates, gscSeries(daily, func(p model.GSCDailyPoint) float64 { return p.Position }), true, b.pos))
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

// allKPIs returns every KPI card: Google, website traffic, AI, then Bing and
// Merchant Center (last, so that they only fill free slots of the key
// indicators; the Google Shopping clicks card is added on purpose).
func (b *builder) allKPIs() []kpiCard {
	var all []kpiCard
	for _, cards := range [][]kpiCard{b.gscKPIs(), b.ga4KPIs(), b.geoKPIs(), b.bingKPIs(), b.gmcKPIs(), b.gbpKPIs()} {
		all = append(all, cards...)
	}
	return all
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
	for _, c := range b.allKPIs() {
		all[c.Key] = c
	}
	// The headline numbers duplicate the KPI cards that follow; they are only
	// shown when the KPI section is hidden.
	for _, k := range []string{"gsc.clicks", "gsc.position", "ga4.sessions", "ga4.organic_sessions", "ga4.key_events", "geo.ai_sessions", "geo.citation_rate", "gsc.impressions", "gmc.clicks", "gbp.interactions", "gbp.views"} {
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

	// Channel by channel, then the strongest changes as big numbers.
	if sc := b.scorecard(); sc.Kind != "" {
		s.Blocks = append(s.Blocks, sc)
	}
	if tl := b.tiles(); tl.Kind != "" {
		s.Blocks = append(s.Blocks, tl)
	}
	if n != nil && n.ExecutiveSummary != "" {
		s.Blocks = append(s.Blocks, block{Kind: "markdown", Span: 12, Tone: "quote", Title: b.t("summary.exec"), HTML: Markdown(n.ExecutiveSummary)})
	}
	// A few highlights only: the complete lists open part 2.
	s.Blocks = append(s.Blocks, b.glanceLists()...)
	// The color code, once, before everything else.
	if len(s.Blocks) > 0 && b.hasComparison() {
		s.Blocks = append([]block{b.legend()}, s.Blocks...)
	}
	if len(s.Blocks) == 0 {
		if b.r.SearchConsole == nil && b.r.Bing == nil && b.r.Analytics == nil && b.r.MerchantCenter == nil && b.r.BusinessProfile == nil && b.r.GEO == nil {
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
	for _, c := range b.allKPIs() {
		all[c.Key] = c
		order = append(order, c.Key)
	}
	// The beginner view keeps a handful of cards: the preferred ones first,
	// then whatever else exists, up to six. Google Shopping clicks, when the
	// site sells through Merchant Center, are a seventh card.
	preferred := []string{"gsc.clicks", "gsc.impressions", "ga4.organic_sessions", "geo.ai_sessions", "ga4.key_events", "gsc.position"}
	if _, ok := all["ga4.organic_sessions"]; !ok {
		preferred[2] = "ga4.sessions"
	}
	limit := 6
	if _, ok := all["gmc.clicks"]; ok {
		preferred = append(preferred, "gmc.clicks")
		limit = 7
	}
	// Google Business Profile actions (or views) are one more card.
	if k := gbpPart1Key(all); k != "" {
		preferred = append(preferred, k)
		limit++
	}
	var cards []kpiCard
	used := map[string]bool{}
	gbpKept := gbpPart1Keys(all) // at most 3 Business Profile cards in part 1
	for _, k := range append(preferred, order...) {
		if gbpSkipped(k, gbpKept) {
			continue
		}
		if c, ok := all[k]; ok && !used[k] && len(cards) < limit {
			cards = append(cards, c)
			used[k] = true
		}
	}
	if len(cards) == 0 {
		return nil
	}
	s.Blocks = append(s.Blocks, block{Kind: "kpis", Span: 12, Tone: "hero", KPIs: cards, Foot: b.expectedLine()})

	// One or two big, simple charts. Tiny volumes and long periods are shown
	// per week: daily zig-zags would read as instability.
	if sc := b.r.SearchConsole; sc != nil && len(sc.Daily) >= 2 {
		dates := gscDates(sc.Daily)
		clicks := func(p model.GSCDailyPoint) float64 { return p.Clicks }
		cur := gscSeries(sc.Daily, clicks)
		wk := wantWeekly(cur, true)
		hint := b.hint("clicks_glance")
		if wk {
			hint = b.hint("clicks_weekly_glance")
		}
		s.Blocks = add(s.Blocks, b.trendAuto("clicks", dates, cur, gscSeries(sc.PreviousDaily, clicks), b.fmtCompact(), aggSum, false, true, wk), 6, hint)
	} else if a := b.r.Analytics; a != nil && len(a.Daily) >= 2 {
		dates := ga4Dates(a.Daily)
		sess := func(p model.GA4DailyPoint) float64 { return p.Sessions }
		cur := ga4Series(a.Daily, sess)
		wk := wantWeekly(cur, true)
		hint := b.hint("sessions_glance")
		if wk {
			hint = b.hint("sessions_weekly_glance")
		}
		s.Blocks = add(s.Blocks, b.trendAuto("sessions", dates, cur, ga4Series(a.PreviousDaily, sess), b.fmtCompact(), aggSum, false, true, wk), 6, hint)
	}
	if a := b.r.Analytics; a != nil && len(a.Channels) > 1 {
		s.Blocks = add(s.Blocks, b.channelDonut(a.Channels), 6, b.hint("channels_glance"))
	}
	return s
}
