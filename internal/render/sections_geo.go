package render

import (
	"html/template"
	"sort"
	"strconv"
	"strings"

	"github.com/flocom/SEO-GEO-Report/internal/charts"
	"github.com/flocom/SEO-GEO-Report/internal/model"
)

func (b *builder) geo() *section {
	g := b.r.GEO
	refs := b.aiReferrals()
	if g == nil && len(refs) == 0 {
		return nil
	}
	if g == nil {
		g = &model.GEO{}
	}
	s := &section{ID: "geo"}
	s.Blocks = append(s.Blocks, b.sectionInsights("geo")...)

	if cards := b.geoKPIs(); len(cards) > 0 {
		s.Blocks = append(s.Blocks, block{Kind: "kpis", Span: 12, KPIs: cards})
	}

	// AI traffic over time.
	if dates, vals := b.aiDaily(); len(vals) >= 2 && nonZero(vals) {
		svg := func(w, h float64) template.HTML {
			return charts.Line(charts.LineOpts{Width: w, Height: h, Labels: dates, XFormat: b.fmtDateTick, TooltipXFormat: b.fmtDateTip, Series: []charts.Series{{Name: b.t("lbl.ai_sessions"), Values: vals, Color: colorAI, Area: true}}, YFormat: b.fmtCompact(), Title: b.t("chart.ai_daily")})
		}
		s.Blocks = add(s.Blocks, block{Kind: "chart", Title: b.t("chart.ai_daily"), mk: svg}, 12, b.hint("ai_daily"))
	}

	// AI referrals by platform.
	if len(refs) > 0 {
		var slices []charts.Slice
		var items []charts.BarItem
		total := 0.0
		for i, r := range refs {
			col := b.seriesColor(i)
			slices = append(slices, charts.Slice{Label: r.Platform, Value: r.Sessions, Color: col})
			items = append(items, charts.BarItem{Label: r.Platform, Value: r.Sessions, Previous: r.PrevSessions, Color: col})
			total += r.Sessions
		}
		if len(refs) > 1 {
			donut := func(w, h float64) template.HTML {
				return b.donut(charts.DonutOpts{Slices: slices, CenterValue: compact(b, total), CenterLabel: b.t("lbl.ai_sessions"), Format: b.fmtNum(), Title: b.t("chart.ai_platforms")})
			}
			s.Blocks = add(s.Blocks, block{Kind: "chart", Title: b.t("chart.ai_platforms"), mk: donut}, 6, b.hint("ai_platforms"))
		}
		bars := func(w, h float64) template.HTML {
			return b.bars(charts.BarsOpts{Width: w, Items: items, Format: b.fmtNum(), ShowDelta: true, Title: b.t("chart.ai_platforms_delta")})
		}
		s.Blocks = add(s.Blocks, block{Kind: "chart", Title: b.t("chart.ai_platforms_delta"), mk: bars}, 6, b.hint("bars_delta"))
		balance(s.Blocks)
		var rows []trafficRow
		for _, r := range refs {
			rows = append(rows, trafficRow{key: r.Platform, sub: r.Source, sessions: r.Sessions, prev: r.PrevSessions, users: r.Users, eng: r.EngagementRate, events: r.KeyEvents, revenue: r.Revenue})
		}
		s.Blocks = add(s.Blocks, block{Kind: "table", Title: b.t("table.ai_referrals"), Table: b.trafficTable(b.t("col.platform"), rows, true)}, 12, "")
	}

	// Citation checks.
	if len(g.Citations) > 0 {
		s.Blocks = append(s.Blocks, b.citationBlocks(g.Citations)...)
	}

	// Share of voice.
	if len(g.ShareOfVoice) > 0 {
		rows := sortedCopy(g.ShareOfVoice, func(r model.ShareOfVoice) float64 { return r.Share })
		var items []charts.BarItem
		for _, r := range rows {
			col := charts.ColorPrevious
			label := r.Brand
			if r.IsSelf {
				col = b.accent
				label += " (" + b.t("badge.you") + ")"
			}
			items = append(items, charts.BarItem{Label: label, Value: r.Share, Previous: r.Previous, Color: col})
		}
		svg := func(w, h float64) template.HTML {
			return b.bars(charts.BarsOpts{Width: w, Items: items, Format: b.fmtPct(), ShowDelta: true, Title: b.t("chart.sov")})
		}
		bl := block{Kind: "chart", Title: b.t("chart.sov"), Sub: b.t("chart.sov.sub"), mk: svg}
		s.Blocks = add(s.Blocks, bl, 6, b.hint("sov"))
	}

	// AI Overviews.
	if len(g.AIOverviews) > 0 {
		s.Blocks = append(s.Blocks, b.aioBlocks(g.AIOverviews)...)
	}
	balance(s.Blocks)

	// AI crawlers.
	if len(g.AICrawlers) > 0 {
		rows := sortedCopy(g.AICrawlers, func(r model.CrawlerRow) float64 { return r.Hits })
		var items []charts.BarItem
		for _, r := range rows {
			items = append(items, charts.BarItem{Label: r.Bot, Value: r.Hits, Previous: r.PrevHits, Color: b.accent})
		}
		svg := func(w, h float64) template.HTML {
			return b.bars(charts.BarsOpts{Width: w, Items: items, Format: b.fmtNum(), ShowDelta: true, Title: b.t("chart.crawlers")})
		}
		s.Blocks = add(s.Blocks, block{Kind: "chart", Title: b.t("chart.crawlers"), Sub: b.t("chart.crawlers.sub"), mk: svg}, 6, b.hint("crawlers"))
		t := &table{Compact: true, Columns: []column{{Label: b.t("col.bot"), Width: "key"}, {Label: b.t("col.hits"), Num: true, Width: "bar"}, {Label: b.t("col.delta"), Num: true}}}
		max := 0.0
		for _, r := range rows {
			max = maxOf(max, r.Hits)
		}
		for _, r := range rows {
			t.Rows = append(t.Rows, row{Cells: []cell{
				{Text: r.Bot, Key: true},
				{Text: b.num(r.Hits), Sub: b.prevText(r.PrevHits, b.num), Num: true, Bar: barStyle(r.Hits, max)},
				{Num: true, Delta: b.deltaRel(metricOf(r.Hits, r.PrevHits), false, b.num)},
			}})
		}
		s.Blocks = add(s.Blocks, block{Kind: "table", Title: b.t("table.crawlers"), Table: b.limit(t, len(rows))}, 6, "")
	}
	if len(s.Blocks) == 0 {
		return nil
	}
	return s
}

func (b *builder) sovTable(rows []model.ShareOfVoice) *table {
	t := &table{Compact: true, Columns: []column{{Label: b.t("col.brand"), Width: "key"}, {Label: b.t("col.share"), Num: true, Width: "bar"}, {Label: b.t("col.delta"), Num: true}}}
	for _, r := range rows {
		m := metricOf(r.Share, r.Previous)
		c := cell{Text: r.Brand, Key: true}
		if r.IsSelf {
			c.Badges = []badge{{Text: b.t("badge.you"), Tone: "accent"}}
		}
		t.Rows = append(t.Rows, row{Highlight: r.IsSelf, Cells: []cell{c, {Text: b.pct(r.Share), Num: true, Bar: barStyle(r.Share, 100)}, {Num: true, Delta: b.deltaPts(m)}}})
	}
	return t
}

func (b *builder) citationBlocks(cs []model.CitationCheck) []block {
	var out []block
	// Rate by engine.
	type agg struct{ n, cited int }
	byEngine := map[string]*agg{}
	var engines []string
	cited, mentioned, gained, lost := 0, 0, 0, 0
	for _, c := range cs {
		e := strings.TrimSpace(c.Engine)
		if e == "" {
			e = "–"
		}
		if byEngine[e] == nil {
			byEngine[e] = &agg{}
			engines = append(engines, e)
		}
		byEngine[e].n++
		if c.Cited {
			byEngine[e].cited++
			cited++
		} else if c.Mentioned {
			mentioned++
		}
		if c.PreviouslyCited != nil {
			if c.Cited && !*c.PreviouslyCited {
				gained++
			}
			if !c.Cited && *c.PreviouslyCited {
				lost++
			}
		}
	}
	sort.SliceStable(engines, func(i, j int) bool {
		ai, aj := byEngine[engines[i]], byEngine[engines[j]]
		return float64(ai.cited)/float64(ai.n) > float64(aj.cited)/float64(aj.n)
	})
	var items []charts.BarItem
	for _, e := range engines {
		a := byEngine[e]
		items = append(items, charts.BarItem{Label: e + " (" + strconv.Itoa(a.cited) + "/" + strconv.Itoa(a.n) + ")", Value: float64(a.cited) / float64(a.n) * 100, Color: b.accent})
	}
	stats := []stat{
		{Label: b.t("stat.prompts"), Value: b.num(float64(len(cs)))},
		{Label: b.t("stat.cited"), Value: b.num(float64(cited)), Sub: b.pct0(float64(cited) / float64(len(cs)) * 100), Tone: "pos"},
		{Label: b.t("stat.mentioned"), Value: b.num(float64(mentioned)), Tone: "neu"},
	}
	if gained+lost > 0 {
		stats = append(stats, stat{Label: b.t("stat.gained"), Value: "+" + b.num(float64(gained)), Tone: "pos"}, stat{Label: b.t("stat.lost"), Value: b.num(float64(lost)), Tone: "neg"})
	}
	svg := func(w, h float64) template.HTML {
		return b.bars(charts.BarsOpts{Width: w, Items: items, Format: b.fmtPct(), Title: b.t("chart.citations_engine")})
	}
	out = add(out, block{Kind: "chart", Title: b.t("chart.citations_engine"), mk: svg, Stats: stats}, 12, b.hint("citations"))

	// Detail table.
	hasPos, hasComp, hasPrev, hasNotes, hasMention := false, false, false, false, false
	for _, c := range cs {
		hasPos = hasPos || c.Position != nil
		hasComp = hasComp || len(c.CompetitorsCited) > 0
		hasPrev = hasPrev || c.PreviouslyCited != nil
		hasNotes = hasNotes || c.Notes != ""
		hasMention = hasMention || c.Mentioned
	}
	t := &table{Columns: []column{{Label: b.t("col.prompt"), Width: "wide"}, {Label: b.t("col.engine")}, {Label: b.t("col.cited")}}}
	if hasMention {
		t.Columns = append(t.Columns, column{Label: b.t("col.mentioned")})
	}
	if hasPos {
		t.Columns = append(t.Columns, column{Label: b.t("col.rank"), Num: true})
	}
	if hasPrev {
		t.Columns = append(t.Columns, column{Label: b.t("col.change")})
	}
	if hasComp {
		t.Columns = append(t.Columns, column{Label: b.t("col.competitors")})
	}
	if hasNotes {
		t.Columns = append(t.Columns, column{Label: b.t("col.notes")})
	}
	for _, c := range cs {
		citedB := badge{Text: b.t("badge.no"), Tone: "neg", Icon: icoX}
		if c.Cited {
			citedB = badge{Text: b.t("badge.yes"), Tone: "pos", Icon: icoCheck}
		}
		sub := ""
		if c.Date != "" {
			sub = b.date(c.Date)
		}
		cells := []cell{{Text: c.Prompt, Sub: sub, Key: true, Wrap: true}, {Text: c.Engine}, {Badges: []badge{citedB}}}
		if hasMention {
			mc := cell{Text: "–", Muted: true}
			if c.Mentioned || c.Cited {
				mc = cell{Badges: []badge{{Text: b.t("badge.yes"), Tone: "neu"}}}
			}
			cells = append(cells, mc)
		}
		if hasPos {
			pc := cell{Text: "–", Num: true, Muted: true}
			if c.Position != nil {
				pc = cell{Text: "#" + strconv.Itoa(*c.Position), Num: true}
			}
			cells = append(cells, pc)
		}
		if hasPrev {
			ch := cell{Text: "–", Muted: true}
			if c.PreviouslyCited != nil {
				switch {
				case c.Cited && !*c.PreviouslyCited:
					ch = cell{Badges: []badge{{Text: b.t("badge.new_citation"), Tone: "pos", Icon: icoTrendUp}}}
				case !c.Cited && *c.PreviouslyCited:
					ch = cell{Badges: []badge{{Text: b.t("badge.lost"), Tone: "neg", Icon: icoTrendDown}}}
				case c.Cited:
					ch = cell{Badges: []badge{{Text: b.t("badge.kept"), Tone: "neu"}}}
				default:
					ch = cell{Text: b.t("badge.still_missing"), Muted: true}
				}
			}
			cells = append(cells, ch)
		}
		if hasComp {
			cells = append(cells, cell{Text: strings.Join(c.CompetitorsCited, ", "), Wrap: true, Muted: len(c.CompetitorsCited) == 0})
		}
		if hasNotes {
			cells = append(cells, cell{Text: c.Notes, Wrap: true})
		}
		t.Rows = append(t.Rows, row{Cells: cells, Highlight: c.Cited})
	}
	out = add(out, block{Kind: "table", Title: b.t("table.citations"), Table: b.limit(t, len(cs))}, 12, b.hint("table_citations"))
	return out
}

func (b *builder) aioBlocks(rows []model.AIOverviewRow) []block {
	present, cited := 0, 0
	for _, o := range rows {
		if o.OverviewPresent {
			present++
			if o.Cited {
				cited++
			}
		}
	}
	none := len(rows) - present
	stats := []stat{
		{Label: b.t("stat.aio_checked"), Value: b.num(float64(len(rows)))},
		{Label: b.t("stat.aio_present"), Value: b.num(float64(present)), Sub: b.pct0(float64(present) / float64(len(rows)) * 100)},
		{Label: b.t("stat.aio_cited"), Value: b.num(float64(cited)), Tone: "pos"},
	}
	if present > 0 {
		stats = append(stats, stat{Label: b.t("stat.aio_rate"), Value: b.pct0(float64(cited) / float64(present) * 100), Tone: "pos"})
	}
	svg := func(w, h float64) template.HTML {
		return b.stacked(charts.StackedOpts{Width: w, Segments: []charts.StackSegment{
			{Label: b.t("lbl.aio_cited"), Value: float64(cited), Color: charts.ColorPositive},
			{Label: b.t("lbl.aio_not_cited"), Value: float64(present - cited), Color: charts.ColorWarning},
			{Label: b.t("lbl.aio_none"), Value: float64(none), Color: "#d6d9de"},
		}, Format: b.fmtNum(), Title: b.t("chart.aio")})
	}
	var out []block
	out = add(out, block{Kind: "chart", Title: b.t("chart.aio"), mk: svg, Stats: stats}, 6, b.hint("aio"))
	hasNotes := false
	for _, o := range rows {
		hasNotes = hasNotes || o.Notes != ""
	}
	t := &table{Columns: []column{{Label: b.t("col.query"), Width: "wide"}, {Label: b.t("col.overview")}, {Label: b.t("col.cited")}}}
	if hasNotes {
		t.Columns = append(t.Columns, column{Label: b.t("col.notes")})
	}
	for _, o := range rows {
		ov := cell{Text: b.t("badge.no"), Muted: true}
		if o.OverviewPresent {
			ov = cell{Badges: []badge{{Text: b.t("badge.yes"), Tone: "info"}}}
		}
		ci := cell{Text: "–", Muted: true}
		if o.OverviewPresent {
			ci = cell{Badges: []badge{{Text: b.t("badge.no"), Tone: "neg", Icon: icoX}}}
			if o.Cited {
				ci = cell{Badges: []badge{{Text: b.t("badge.yes"), Tone: "pos", Icon: icoCheck}}}
			}
		}
		cells := []cell{{Text: o.Query, Key: true, Wrap: true}, ov, ci}
		if hasNotes {
			cells = append(cells, cell{Text: o.Notes, Wrap: true})
		}
		t.Rows = append(t.Rows, row{Cells: cells, Highlight: o.Cited})
	}
	out = add(out, block{Kind: "table", Title: b.t("table.aio"), Table: b.limit(t, len(rows))}, 12, "")
	return out
}
