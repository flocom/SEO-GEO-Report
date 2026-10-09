package render

import (
	"html/template"
	"strings"

	"github.com/flocom/SEO-GEO-Report/internal/charts"
	"github.com/flocom/SEO-GEO-Report/internal/insights"
	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// ---------------------------------------------------------------------------
// Analytics (GA4 overview)

func (b *builder) analytics() *section {
	a := b.r.Analytics
	if a == nil {
		return nil
	}
	s := &section{ID: "analytics"}
	s.Blocks = append(s.Blocks, b.sectionInsights("analytics")...)

	// Engagement tiles.
	t := a.Totals
	var cards []kpiCard
	addCard := func(key string, m *model.Metric, val func(float64) string, d func(model.Metric) *deltaView) {
		if m == nil {
			return
		}
		cards = append(cards, b.card("ga4."+key, key, val(m.Current), m, val, d(*m)))
	}
	rel := func(m model.Metric) *deltaView { return b.deltaRel(m, false, b.num) }
	addCard("sessions", t.Sessions, b.num, rel)
	addCard("users", t.Users, b.num, rel)
	addCard("new_users", t.NewUsers, b.num, rel)
	addCard("engaged_sessions", t.EngagedSessions, b.num, rel)
	addCard("engagement_rate", t.EngagementRate, b.pct, b.deltaPts)
	addCard("avg_engagement_time", t.AvgEngagementTime, b.dur, func(m model.Metric) *deltaView { return b.deltaRel(m, false, nil) })
	addCard("page_views", t.PageViews, b.num, rel)
	addCard("key_events", t.KeyEvents, b.num, rel)
	addCard("revenue", t.Revenue, b.money, func(m model.Metric) *deltaView { return b.deltaRel(m, false, b.money) })
	for i := range cards {
		cards[i].Explain = ""
	}
	if len(cards) > 0 {
		s.Blocks = append(s.Blocks, block{Kind: "kpis", Span: 12, Tone: "compact", KPIs: cards})
	}

	// Sessions over time.
	if len(a.Daily) >= 2 {
		var dates []string
		for _, p := range a.Daily {
			dates = append(dates, p.Date)
		}
		sess := ga4Series(a.Daily, func(p model.GA4DailyPoint) float64 { return p.Sessions })
		series := []charts.Series{{Name: b.t("lbl.all_sessions"), Values: sess, Color: b.accent, Area: true}}
		if org := ga4Series(a.Daily, func(p model.GA4DailyPoint) float64 { return p.OrganicSessions }); nonZero(org) {
			series = append(series, charts.Series{Name: b.t("lbl.organic_sessions"), Values: org, Color: "#10b981"})
		}
		if ai := ga4Series(a.Daily, func(p model.GA4DailyPoint) float64 { return p.AISessions }); nonZero(ai) {
			series = append(series, charts.Series{Name: b.t("lbl.ai_sessions"), Values: ai, Color: colorAI})
		}
		if prev := alignPrev(len(sess), ga4Series(a.PreviousDaily, func(p model.GA4DailyPoint) float64 { return p.Sessions })); prev != nil && nonZero(prev) {
			series = append(series, charts.Series{Name: b.t("lbl.prev_sessions"), Values: prev, Color: charts.ColorPrevious, Dashed: true})
		}
		if nonZero(sess) {
			svg := func(w, h float64) template.HTML {
				return charts.Line(charts.LineOpts{Width: w, Height: h, Labels: dates, XFormat: b.fmtDateTick, TooltipXFormat: b.fmtDateTip, Series: series, YFormat: b.fmtCompact(), Title: b.t("chart.sessions_daily")})
			}
			s.Blocks = add(s.Blocks, block{Kind: "chart", Title: b.t("chart.sessions_daily"), mk: svg}, 12, b.hint("sessions_daily"))
		}
	}

	// Organic vs whole site.
	if o := a.OrganicTotals; o != nil {
		tb := &table{Columns: []column{{Label: b.t("col.metric"), Width: "key"}, {Label: b.t("col.all"), Num: true}, {Label: b.t("col.organic"), Num: true}, {Label: b.t("col.delta"), Num: true}, {Label: b.t("col.organic_share"), Num: true}}}
		addRow := func(key string, all, org *model.Metric, f func(float64) string, share bool, d *deltaView) {
			if org == nil {
				return
			}
			allTxt := "–"
			if all != nil {
				allTxt = f(all.Current)
			}
			sh := cell{Text: "–", Num: true, Muted: true}
			if share && all != nil && all.Current > 0 {
				p := org.Current / all.Current * 100
				sh = cell{Text: b.pct0(p), Num: true, Bar: barStyle(p, 100)}
			}
			tb.Rows = append(tb.Rows, row{Cells: []cell{
				{Text: b.t("kpi." + key + ".label"), Key: true},
				{Text: allTxt, Num: true, Muted: true},
				{Text: f(org.Current), Sub: b.prevText(org.Previous, f), Num: true},
				{Num: true, Delta: d},
				sh,
			}})
		}
		relp := func(m *model.Metric, f func(float64) string) *deltaView {
			if m == nil {
				return nil
			}
			return b.deltaRel(*m, false, f)
		}
		addRow("sessions", t.Sessions, o.Sessions, b.num, true, relp(o.Sessions, b.num))
		addRow("users", t.Users, o.Users, b.num, true, relp(o.Users, b.num))
		addRow("new_users", t.NewUsers, o.NewUsers, b.num, true, relp(o.NewUsers, b.num))
		addRow("engaged_sessions", t.EngagedSessions, o.EngagedSessions, b.num, true, relp(o.EngagedSessions, b.num))
		if o.EngagementRate != nil {
			addRow("engagement_rate", t.EngagementRate, o.EngagementRate, b.pct, false, b.deltaPts(*o.EngagementRate))
		}
		addRow("avg_engagement_time", t.AvgEngagementTime, o.AvgEngagementTime, b.dur, false, relp(o.AvgEngagementTime, nil))
		addRow("page_views", t.PageViews, o.PageViews, b.num, true, relp(o.PageViews, b.num))
		addRow("key_events", t.KeyEvents, o.KeyEvents, b.num, true, relp(o.KeyEvents, b.num))
		addRow("revenue", t.Revenue, o.Revenue, b.money, true, relp(o.Revenue, b.money))
		if len(tb.Rows) > 0 {
			s.Blocks = add(s.Blocks, block{Kind: "table", Title: b.t("table.organic_vs_all"), Table: tb}, 12, b.hint("organic_vs_all"))
		}
	}
	if len(s.Blocks) == 0 {
		return nil
	}
	return s
}

// ---------------------------------------------------------------------------
// Channels & sources

func (b *builder) channels() *section {
	a := b.r.Analytics
	if a == nil || (len(a.Channels) == 0 && len(a.Sources) == 0) {
		return nil
	}
	s := &section{ID: "channels"}
	s.Blocks = append(s.Blocks, b.sectionInsights("channels")...)
	if len(a.Channels) > 0 {
		rows := sortedCopy(a.Channels, func(r model.ChannelRow) float64 { return r.Sessions })
		var items []charts.BarItem
		for i, c := range rows {
			items = append(items, charts.BarItem{Label: c.Channel, Value: c.Sessions, Previous: c.PrevSessions, Color: b.channelColor(i, c.Channel)})
		}
		s.Blocks = add(s.Blocks, b.channelDonut(a.Channels), 6, b.hint("channels"))
		bars := func(w, h float64) template.HTML {
			return b.bars(charts.BarsOpts{Width: w, Items: topN(items, 8), Format: b.fmtNum(), ShowDelta: true, Title: b.t("chart.channels_delta")})
		}
		s.Blocks = add(s.Blocks, block{Kind: "chart", Title: b.t("chart.channels_delta"), mk: bars}, 6, b.hint("bars_delta"))
		balance(s.Blocks)
		s.Blocks = add(s.Blocks, block{Kind: "table", Title: b.t("table.channels"), Table: b.channelTable(a.Channels)}, 12, "")
	}
	if len(a.Sources) > 0 {
		s.Blocks = add(s.Blocks, block{Kind: "table", Title: b.t("table.sources"), Sub: b.t("table.sources.sub"), Table: b.sourceTable(a.Sources)}, 12, "")
	}
	return s
}

func (b *builder) channelColor(i int, name string) string {
	if isOrganic(name) {
		return b.accent
	}
	return pickColor(i + 1)
}

// channelDonut shows the split of sessions by channel (7 slices + others).
func (b *builder) channelDonut(ch []model.ChannelRow) block {
	rows := sortedCopy(ch, func(r model.ChannelRow) float64 { return r.Sessions })
	var slices []charts.Slice
	total := 0.0
	for i, c := range rows {
		switch {
		case i < 7:
			slices = append(slices, charts.Slice{Label: c.Channel, Value: c.Sessions, Color: b.channelColor(i, c.Channel)})
		case len(slices) == 7:
			slices = append(slices, charts.Slice{Label: b.t("lbl.others"), Value: c.Sessions, Color: charts.ColorPrevious})
		default:
			slices[7].Value += c.Sessions
		}
		total += c.Sessions
	}
	if total <= 0 {
		return block{}
	}
	donut := func(w, h float64) template.HTML {
		return b.donut(charts.DonutOpts{Slices: slices, CenterValue: compact(b, total), CenterLabel: b.t("col.sessions"), Format: b.fmtNum(), Title: b.t("chart.channels")})
	}
	return block{Kind: "chart", Title: b.t("chart.channels"), Sub: b.t("chart.channels.sub"), mk: donut}
}

func compact(b *builder, v float64) string {
	if v >= 10000 {
		return b.fmtCompact()(v)
	}
	return b.num(v)
}

func isOrganic(ch string) bool {
	c := strings.ToLower(ch)
	return strings.Contains(c, "organic search") || strings.Contains(c, "recherche naturelle") || strings.Contains(c, "recherche organique")
}

// trafficTable is shared by channels, sources, landing pages and AI referrals.
type trafficRow struct {
	key      string
	title    string
	sub      string
	badges   []badge
	sessions float64
	prev     *float64
	users    float64
	eng      float64
	events   float64
	revenue  float64
	hl       bool
}

func (b *builder) trafficTable(keyLabel string, rows []trafficRow, withUsers bool) *table {
	hasPrev, hasUsers, hasEng, hasEv, hasRev := false, false, false, false, false
	maxS := 0.0
	for i, r := range rows {
		hasPrev = hasPrev || r.prev != nil
		hasUsers = hasUsers || r.users > 0
		hasEng = hasEng || r.eng > 0
		hasEv = hasEv || r.events > 0
		hasRev = hasRev || r.revenue > 0
		if i < b.maxRows && r.sessions > maxS {
			maxS = r.sessions
		}
	}
	hasUsers = hasUsers && withUsers
	t := &table{Columns: []column{{Label: "#", Num: true, Width: "rank"}, {Label: keyLabel, Width: "key"}, {Label: b.t("col.sessions"), Num: true, Width: "bar"}}}
	if hasPrev {
		t.Columns = append(t.Columns, column{Label: b.t("col.delta"), Num: true})
	}
	if hasUsers {
		t.Columns = append(t.Columns, column{Label: b.t("col.users"), Num: true})
	}
	if hasEng {
		t.Columns = append(t.Columns, column{Label: b.t("col.engagement"), Num: true})
	}
	if hasEv {
		t.Columns = append(t.Columns, column{Label: b.t("col.key_events"), Num: true})
	}
	if hasRev {
		t.Columns = append(t.Columns, column{Label: b.t("col.revenue"), Num: true})
	}
	opt := func(v float64, f func(float64) string) cell {
		if v == 0 {
			return cell{Text: "–", Num: true, Muted: true}
		}
		return cell{Text: f(v), Num: true}
	}
	for i, r := range rows {
		if i >= b.maxRows {
			break
		}
		cells := []cell{
			{Text: itoa(i + 1), Num: true, Muted: true},
			{Text: r.key, Title: r.title, Sub: r.sub, Key: true, Wrap: true, Badges: r.badges},
			{Text: b.num(r.sessions), Sub: b.prevText(r.prev, b.num), Num: true, Bar: barStyle(r.sessions, maxS)},
		}
		if hasPrev {
			cells = append(cells, cell{Num: true, Delta: b.deltaRel(metricOf(r.sessions, r.prev), false, b.num)})
		}
		if hasUsers {
			cells = append(cells, opt(r.users, b.num))
		}
		if hasEng {
			cells = append(cells, opt(r.eng, b.pct))
		}
		if hasEv {
			cells = append(cells, opt(r.events, b.num))
		}
		if hasRev {
			cells = append(cells, opt(r.revenue, b.money))
		}
		t.Rows = append(t.Rows, row{Cells: cells, Highlight: r.hl})
	}
	return b.limit(t, len(rows))
}

func (b *builder) channelTable(ch []model.ChannelRow) *table {
	var rows []trafficRow
	for _, c := range ch {
		rows = append(rows, trafficRow{key: c.Channel, sessions: c.Sessions, prev: c.PrevSessions, users: c.Users, eng: c.EngagementRate, events: c.KeyEvents, revenue: c.Revenue, hl: isOrganic(c.Channel)})
	}
	return b.trafficTable(b.t("col.channel"), rows, true)
}

func (b *builder) isAISource(src string) string {
	if p := insights.DetectAIPlatform(src); p != "" {
		return p
	}
	for _, r := range b.aiReferrals() {
		if r.Source != "" && strings.EqualFold(r.Source, src) {
			return r.Platform
		}
	}
	return ""
}

func (b *builder) sourceTable(src []model.SourceRow) *table {
	var rows []trafficRow
	for _, s := range src {
		k := s.Source
		if s.Medium != "" {
			k += " / " + s.Medium
		}
		r := trafficRow{key: k, sessions: s.Sessions, prev: s.PrevSessions, users: s.Users, eng: s.EngagementRate, events: s.KeyEvents, revenue: s.Revenue}
		if p := b.isAISource(s.Source); p != "" {
			r.badges = []badge{{Text: b.t("badge.ai") + " · " + p, Tone: "accent", Icon: icoAI}}
			r.hl = true
		}
		rows = append(rows, r)
	}
	return b.trafficTable(b.t("col.source"), rows, true)
}

// ---------------------------------------------------------------------------
// Landing pages

func (b *builder) landingPages() *section {
	a := b.r.Analytics
	if a == nil || len(a.LandingPages) == 0 {
		return nil
	}
	s := &section{ID: "landing_pages"}
	s.Blocks = append(s.Blocks, b.sectionInsights("landing_pages")...)
	top := topN(sortedCopy(a.LandingPages, func(r model.LandingPageRow) float64 { return r.Sessions }), 10)
	if len(top) >= 2 {
		var items []charts.BarItem
		for _, p := range top {
			items = append(items, charts.BarItem{Label: truncate(shortURL(p.Page), 60), Value: p.Sessions, Previous: p.PrevSessions, Color: b.accent})
		}
		svg := func(w, h float64) template.HTML {
			return b.bars(charts.BarsOpts{Width: w, Items: items, Format: b.fmtNum(), ShowDelta: true, Title: b.t("chart.landing")})
		}
		s.Blocks = add(s.Blocks, block{Kind: "chart", Title: b.t("chart.landing"), mk: svg}, 12, b.hint("bars_delta"))
	}
	var rows []trafficRow
	for _, p := range a.LandingPages {
		k := truncate(shortURL(p.Page), 90)
		title := ""
		if k != p.Page {
			title = p.Page
		}
		rows = append(rows, trafficRow{key: k, title: title, sessions: p.Sessions, prev: p.PrevSessions, eng: p.EngagementRate, events: p.KeyEvents, revenue: p.Revenue})
	}
	s.Blocks = add(s.Blocks, block{Kind: "table", Title: b.t("table.landing"), Table: b.trafficTable(b.t("col.page"), rows, false)}, 12, b.hint("table_landing"))
	return s
}
