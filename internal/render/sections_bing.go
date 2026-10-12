package render

import (
	"html/template"
	"math"
	"strings"

	"github.com/flocom/SEO-GEO-Report/internal/charts"
	"github.com/flocom/SEO-GEO-Report/internal/i18n"
	"github.com/flocom/SEO-GEO-Report/internal/insights"
	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// ---------------------------------------------------------------------------
// Bing Webmaster Tools

func (b *builder) bingKPIs() []kpiCard {
	bg := b.r.Bing
	if bg == nil {
		return nil
	}
	return b.searchKPIs("bing.", "bing_", bg.Totals, bg.Daily)
}

func (b *builder) bing() *section {
	bg := b.r.Bing
	if bg == nil {
		return nil
	}
	s := &section{ID: "bing"}
	s.Blocks = append(s.Blocks, b.sectionInsights("bing")...)
	if cards := b.bingKPIs(); len(cards) > 0 {
		for i := range cards {
			cards[i].Explain, cards[i].Spark = "", ""
		}
		s.Blocks = append(s.Blocks, block{Kind: "kpis", Span: 12, Tone: "compact", KPIs: cards})
	}

	dates := gscDates(bg.Daily)
	clicks := func(p model.GSCDailyPoint) float64 { return p.Clicks }
	impr := func(p model.GSCDailyPoint) float64 { return p.Impressions }
	pos := func(p model.GSCDailyPoint) float64 { return p.Position }
	s.Blocks = add(s.Blocks, b.trend(b.t("chart.bing_clicks_daily"), dates, gscSeries(bg.Daily, clicks), gscSeries(bg.PreviousDaily, clicks), b.fmtCompact(), false, true), 12, b.hint("bing_clicks_daily"))
	s.Blocks = add(s.Blocks, b.trend(b.t("chart.impressions_daily"), dates, gscSeries(bg.Daily, impr), gscSeries(bg.PreviousDaily, impr), b.fmtCompact(), false, true), 6, b.hint("bing_impressions_daily"))
	s.Blocks = add(s.Blocks, b.trend(b.t("chart.position_daily"), dates, gscSeries(bg.Daily, pos), gscSeries(bg.PreviousDaily, pos), b.fmtPos(), true, false), 6, b.hint("bing_position_daily"))
	s.Blocks = add(s.Blocks, b.enginesBlock(), 6, b.hint("engines"))
	s.Blocks = add(s.Blocks, b.gscBars(bg.Queries, 10, b.t("chart.bing_top_queries"), nil), 6, b.hint("bing_top_queries"))
	balance(s.Blocks)

	if len(bg.Queries) > 0 {
		s.Blocks = add(s.Blocks, block{Kind: "table", Title: b.t("table.bing_queries"), Table: b.gscTable(bg.Queries, b.t("col.query"), func(k string) string { return truncate(k, 90) })}, 12, b.hint("table_bing"))
	}
	if len(bg.Pages) > 0 {
		s.Blocks = add(s.Blocks, block{Kind: "table", Title: b.t("table.bing_pages"), Table: b.gscTable(bg.Pages, b.t("col.page"), func(k string) string { return truncate(shortURL(k), 90) })}, 12, "")
	}
	if ai := bg.AIPerformance; ai != nil {
		s.Blocks = append(s.Blocks, b.bingAIBlocks(ai)...)
	}
	if len(s.Blocks) == 0 {
		return nil
	}
	return s
}

// enginesBlock compares Google and Bing clicks.
func (b *builder) enginesBlock() block {
	share := insights.BingShare(b.r)
	if share == nil {
		return block{}
	}
	g, bc := b.r.SearchConsole.Totals.Clicks, b.r.Bing.Totals.Clicks
	items := []charts.BarItem{
		{Label: "Google", Value: g.Current, Previous: g.Previous, Color: b.seriesColor(1)},
		{Label: "Bing", Value: bc.Current, Previous: bc.Previous, Color: b.accent},
	}
	title := b.t("chart.engines")
	bl := block{Kind: "chart", Title: title, Sub: b.t("chart.engines.sub"), mk: func(w, h float64) template.HTML {
		return b.bars(charts.BarsOpts{Width: w, Height: h * 0.6, Items: items, Format: b.fmtNum(), ShowDelta: true, Title: title})
	}}
	st := stat{Label: b.t("stat.bing_share"), Value: i18nPct2(b, share.Current)}
	if d := b.deltaPts(*share); share.Previous != nil && d != nil {
		// A share moves when either engine moves: neither good nor bad news.
		d.Tone = "neu"
		d.Arrow = arrowFor(d.Dir, d.Tone)
		st.Delta = d
	}
	bl.Stats = []stat{st, {Label: b.t("kpi.bing_clicks.label"), Value: b.num(bc.Current), Delta: b.deltaRel(bc, false, b.num)}}
	return bl
}

// bingAIBlocks renders the AI Performance report: citations of the site in
// Copilot and Bing AI answers.
func (b *builder) bingAIBlocks(ai *model.BingAIPerformance) []block {
	var blocks []block
	var dates []string
	var cur, prev []float64
	for _, p := range ai.Daily {
		dates = append(dates, p.Date)
		cur = append(cur, p.Citations)
	}
	for _, p := range ai.PreviousDaily {
		prev = append(prev, p.Citations)
	}

	var cards []kpiCard
	if ai.Citations.Current > 0 || ai.Citations.Previous != nil {
		c := b.card(insights.KeyBingAICitations, "bing_ai_citations", b.num(ai.Citations.Current), &ai.Citations, b.num, b.deltaRel(ai.Citations, false, b.num))
		cards = append(cards, b.withSpark(c, dates, cur, false, b.num))
	}
	if m := ai.CitedPages; m != nil && m.Current > 0 {
		cards = append(cards, b.card("bing.ai_cited_pages", "bing_cited_pages", b.dec(m.Current, 1), m, func(v float64) string { return b.dec(v, 1) }, b.deltaRel(*m, false, nil)))
	}
	if len(cards) > 0 {
		blocks = append(blocks, block{Kind: "kpis", Span: 12, Title: b.t("bing.ai.title"), KPIs: cards})
	}

	if len(cur) >= 2 && nonZero(cur) {
		series := []charts.Series{{Name: b.t("label.current"), Values: cur, Color: colorAI, Area: true}}
		if p := alignPrev(len(cur), prev); p != nil && nonZero(p) {
			series = append(series, charts.Series{Name: b.t("label.previous"), Values: p, Color: charts.ColorPrevious, Dashed: true})
		}
		title := b.t("chart.bing_ai_daily")
		blocks = add(blocks, block{Kind: "chart", Title: title, mk: func(w, h float64) template.HTML {
			return charts.Line(charts.LineOpts{Width: w, Height: h, Labels: dates, XFormat: b.fmtDateTick, TooltipXFormat: b.fmtDateTip, Series: series, YFormat: b.fmtCompact(), Title: title})
		}}, 12, b.hint("bing_ai_daily"))
	}

	queries := sortedCopy(ai.GroundingQueries, func(q model.BingGroundingQuery) float64 { return q.Citations })
	pages := sortedCopy(ai.Pages, func(p model.BingCitedPage) float64 { return p.Citations })
	if top := topN(queries, 10); len(top) >= 2 {
		var items []charts.BarItem
		for _, q := range top {
			items = append(items, charts.BarItem{Label: truncate(q.Query, 60), Value: q.Citations, Previous: q.PrevCitations, Color: colorAI})
		}
		title := b.t("chart.bing_grounding")
		blocks = add(blocks, block{Kind: "chart", Title: title, mk: func(w, h float64) template.HTML {
			return b.bars(charts.BarsOpts{Width: w, Items: items, Format: b.fmtNum(), ShowDelta: true, Title: title})
		}}, 6, b.hint("bing_grounding"))
	}
	if top := topN(pages, 10); len(top) >= 2 {
		var items []charts.BarItem
		for _, p := range top {
			items = append(items, charts.BarItem{Label: truncate(shortURL(p.URL), 60), Value: p.Citations, Previous: p.PrevCitations, Color: colorAI})
		}
		title := b.t("chart.bing_cited_pages")
		blocks = add(blocks, block{Kind: "chart", Title: title, mk: func(w, h float64) template.HTML {
			return b.bars(charts.BarsOpts{Width: w, Items: items, Format: b.fmtNum(), ShowDelta: true, Title: title})
		}}, 6, b.hint("bing_cited_pages"))
	}
	blocks = add(blocks, b.intentsBlock(queries), 6, b.hint("bing_intents"))
	balance(blocks)

	if len(queries) > 0 {
		blocks = add(blocks, block{Kind: "table", Title: b.t("table.bing_grounding"), Table: b.groundingTable(queries)}, 12, b.hint("table_bing_grounding"))
	}
	if len(pages) > 0 {
		blocks = add(blocks, block{Kind: "table", Title: b.t("table.bing_cited_pages"), Table: b.citedPagesTable(pages)}, 12, "")
	}
	return blocks
}

// intentsBlock splits the citations by query intent, when Bing gives it.
func (b *builder) intentsBlock(queries []model.BingGroundingQuery) block {
	byIntent := map[string]float64{}
	var order []string
	total := 0.0
	for _, q := range queries {
		in := strings.TrimSpace(q.Intent)
		if in == "" || q.Citations <= 0 {
			continue
		}
		if _, ok := byIntent[in]; !ok {
			order = append(order, in)
		}
		byIntent[in] += q.Citations
		total += q.Citations
	}
	if len(order) < 2 {
		return block{}
	}
	type kv struct {
		k string
		v float64
	}
	var rows []kv
	for _, k := range order {
		rows = append(rows, kv{k, byIntent[k]})
	}
	rows = sortedCopy(rows, func(r kv) float64 { return r.v })
	var slices []charts.Slice
	for i, r := range rows {
		slices = append(slices, charts.Slice{Label: r.k, Value: r.v, Color: b.seriesColor(i)})
	}
	title := b.t("chart.bing_intents")
	return block{Kind: "chart", Title: title, Sub: b.t("chart.bing_intents.sub"), mk: func(w, h float64) template.HTML {
		return b.donut(charts.DonutOpts{Slices: slices, CenterValue: i18n.Compact(b.lang, total), CenterLabel: b.t("col.citations"), Format: b.fmtNum(), Title: title})
	}}
}

func (b *builder) groundingTable(rows []model.BingGroundingQuery) *table {
	hasPrev, hasShare, hasPages, hasIntent, hasTopic := false, false, false, false, false
	maxC := 0.0
	for i, r := range rows {
		hasPrev = hasPrev || r.PrevCitations != nil
		hasShare = hasShare || r.CitationShare > 0
		hasPages = hasPages || r.CitedPages > 0
		hasIntent = hasIntent || r.Intent != ""
		hasTopic = hasTopic || r.Topic != ""
		if i < b.maxRows {
			maxC = math.Max(maxC, r.Citations)
		}
	}
	t := &table{Columns: []column{{Label: "#", Num: true, Width: "rank"}, {Label: b.t("col.grounding_query"), Width: "key"}, {Label: b.t("col.citations"), Num: true, Width: "bar"}}}
	if hasPrev {
		t.Columns = append(t.Columns, column{Label: b.t("col.delta"), Num: true})
	}
	if hasShare {
		t.Columns = append(t.Columns, column{Label: b.t("col.citation_share"), Num: true})
	}
	if hasPages {
		t.Columns = append(t.Columns, column{Label: b.t("col.cited_pages"), Num: true})
	}
	if hasIntent {
		t.Columns = append(t.Columns, column{Label: b.t("col.intent")})
	}
	if hasTopic {
		t.Columns = append(t.Columns, column{Label: b.t("col.topic")})
	}
	for i, r := range rows {
		if i >= b.maxRows {
			break
		}
		cells := []cell{
			{Text: itoa(i + 1), Num: true, Muted: true},
			{Text: truncate(r.Query, 90), Title: r.Query, Key: true, Wrap: true},
			{Text: b.num(r.Citations), Sub: b.prevText(r.PrevCitations, b.num), Num: true, Bar: barStyle(r.Citations, maxC), BarTone: "warn"},
		}
		if hasPrev {
			cells = append(cells, cell{Num: true, Delta: b.deltaRel(metricOf(r.Citations, r.PrevCitations), false, b.num)})
		}
		if hasShare {
			c := cell{Text: "–", Num: true, Muted: true}
			if r.CitationShare > 0 {
				c = cell{Text: i18nPct2(b, r.CitationShare), Num: true}
			}
			cells = append(cells, c)
		}
		if hasPages {
			c := cell{Text: "–", Num: true, Muted: true}
			if r.CitedPages > 0 {
				c = cell{Text: b.dec(r.CitedPages, 1), Num: true}
			}
			cells = append(cells, c)
		}
		if hasIntent {
			cells = append(cells, cell{Text: r.Intent, Muted: r.Intent == ""})
		}
		if hasTopic {
			cells = append(cells, cell{Text: r.Topic, Muted: r.Topic == "", Wrap: true})
		}
		t.Rows = append(t.Rows, row{Cells: cells})
	}
	return b.limit(t, len(rows))
}

func (b *builder) citedPagesTable(rows []model.BingCitedPage) *table {
	hasPrev := false
	maxC := 0.0
	for i, r := range rows {
		hasPrev = hasPrev || r.PrevCitations != nil
		if i < b.maxRows {
			maxC = math.Max(maxC, r.Citations)
		}
	}
	t := &table{Columns: []column{{Label: "#", Num: true, Width: "rank"}, {Label: b.t("col.page"), Width: "key"}, {Label: b.t("col.citations"), Num: true, Width: "bar"}}}
	if hasPrev {
		t.Columns = append(t.Columns, column{Label: b.t("col.delta"), Num: true})
	}
	for i, r := range rows {
		if i >= b.maxRows {
			break
		}
		key := truncate(shortURL(r.URL), 90)
		title := ""
		if key != r.URL {
			title = r.URL
		}
		cells := []cell{
			{Text: itoa(i + 1), Num: true, Muted: true},
			{Text: key, Title: title, Key: true, Wrap: true},
			{Text: b.num(r.Citations), Sub: b.prevText(r.PrevCitations, b.num), Num: true, Bar: barStyle(r.Citations, maxC), BarTone: "warn"},
		}
		if hasPrev {
			cells = append(cells, cell{Num: true, Delta: b.deltaRel(metricOf(r.Citations, r.PrevCitations), false, b.num)})
		}
		t.Rows = append(t.Rows, row{Cells: cells})
	}
	return b.limit(t, len(rows))
}
