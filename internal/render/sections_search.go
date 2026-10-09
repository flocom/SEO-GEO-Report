package render

import (
	"html/template"
	"math"
	"strings"

	"github.com/flocom/SEO-GEO-Report/internal/charts"
	"github.com/flocom/SEO-GEO-Report/internal/i18n"
	"github.com/flocom/SEO-GEO-Report/internal/model"
)

func (b *builder) dateRange(p model.Period) string { return i18n.DateRange(b.lang, p.Start, p.End) }

// trend renders a daily line chart with an optional dashed comparison series.
func (b *builder) trend(title string, dates []string, cur, prev []float64, f charts.Formatter, invert, area bool) block {
	if len(cur) < 2 || !nonZero(cur) {
		return block{}
	}
	series := []charts.Series{{Name: b.t("label.current"), Values: cur, Color: b.accent, Area: area}}
	if p := alignPrev(len(cur), prev); p != nil && nonZero(p) {
		series = append(series, charts.Series{Name: b.t("label.previous"), Values: p, Color: charts.ColorPrevious, Dashed: true})
	}
	svg := func(w, h float64) template.HTML {
		return charts.Line(charts.LineOpts{Width: w, Height: h, Labels: dates, XFormat: b.fmtDateTick, TooltipXFormat: b.fmtDateTip, Series: series, YFormat: f, InvertY: invert, Title: title})
	}
	return block{Kind: "chart", Title: title, mk: svg}
}

func add(blocks []block, bl block, span int, hint string) []block {
	if bl.Kind == "" || (bl.Kind == "chart" && bl.Chart == "" && bl.mk == nil) {
		return blocks
	}
	bl.Span = span
	if hint != "" && bl.Hint == "" {
		bl.Hint = hint
		if bl.HintKind == "" {
			bl.HintKind = "chart"
			if bl.Kind == "table" {
				bl.HintKind = "table"
			}
		}
	}
	return append(blocks, bl)
}

func (b *builder) prevText(v *float64, f func(float64) string) string {
	if v == nil {
		return ""
	}
	return b.tf("table.prev", f(*v))
}

// ---------------------------------------------------------------------------
// Search Console overview

func (b *builder) searchConsole() *section {
	sc := b.r.SearchConsole
	if sc == nil {
		return nil
	}
	s := &section{ID: "search_console"}
	s.Blocks = append(s.Blocks, b.sectionInsights("search_console")...)
	if cards := b.gscKPIs(); len(cards) > 0 {
		for i := range cards {
			cards[i].Explain, cards[i].Spark = "", ""
		}
		s.Blocks = append(s.Blocks, block{Kind: "kpis", Span: 12, Tone: "compact", KPIs: cards})
	}

	var dates []string
	for _, p := range sc.Daily {
		dates = append(dates, p.Date)
	}
	get := func(pts []model.GSCDailyPoint, f func(model.GSCDailyPoint) float64) []float64 {
		return gscSeries(pts, f)
	}
	clicks := func(p model.GSCDailyPoint) float64 { return p.Clicks }
	impr := func(p model.GSCDailyPoint) float64 { return p.Impressions }
	ctr := func(p model.GSCDailyPoint) float64 { return p.CTR }
	pos := func(p model.GSCDailyPoint) float64 { return p.Position }

	s.Blocks = add(s.Blocks, b.trend(b.t("chart.clicks_daily"), dates, get(sc.Daily, clicks), get(sc.PreviousDaily, clicks), b.fmtCompact(), false, true), 12, b.hint("clicks_daily"))
	s.Blocks = add(s.Blocks, b.trend(b.t("chart.impressions_daily"), dates, get(sc.Daily, impr), get(sc.PreviousDaily, impr), b.fmtCompact(), false, true), 6, b.hint("impressions_daily"))
	s.Blocks = add(s.Blocks, b.trend(b.t("chart.position_daily"), dates, get(sc.Daily, pos), get(sc.PreviousDaily, pos), b.fmtPos(), true, false), 6, b.hint("position_daily"))
	s.Blocks = add(s.Blocks, b.trend(b.t("chart.ctr_daily"), dates, get(sc.Daily, ctr), get(sc.PreviousDaily, ctr), b.fmtPct(), false, false), 6, b.hint("ctr_daily"))

	// Position distribution.
	if pd := b.positions(); pd != nil && posDistTotal(pd) > 0 {
		ms := []model.Metric{pd.Top3, pd.Top10, pd.Top20, pd.Beyond20}
		labels := []string{b.t("lbl.top3"), b.t("lbl.top10"), b.t("lbl.top20"), b.t("lbl.beyond")}
		cur := make([]float64, 4)
		var prev []float64
		hasPrev := true
		for i, m := range ms {
			cur[i] = m.Current
			if m.Previous == nil {
				hasPrev = false
			}
		}
		if hasPrev {
			prev = make([]float64, 4)
			for i, m := range ms {
				prev[i] = *m.Previous
			}
		}
		svg := func(w, h float64) template.HTML {
			return b.columns(charts.ColumnsOpts{Width: w, Height: h, Labels: labels, Current: cur, Previous: prev, CurrentName: b.t("label.current"), PreviousName: b.t("label.previous"), Format: b.fmtNum(), Title: b.t("chart.positions")})
		}
		bl := block{Kind: "chart", Title: b.t("chart.positions"), Sub: b.t("chart.positions.sub"), mk: svg}
		var st []stat
		for i, m := range ms {
			st = append(st, stat{Label: labels[i], Value: b.num(m.Current), Delta: b.deltaRel(m, false, b.num)})
		}
		bl.Stats = st
		s.Blocks = add(s.Blocks, bl, 6, b.hint("positions"))
	}

	// Brand vs non-brand.
	if bs := sc.BrandSplit; bs != nil && bs.Branded.Current+bs.NonBranded.Current > 0 {
		total := bs.Branded.Current + bs.NonBranded.Current
		svg := func(w, h float64) template.HTML {
			return b.donut(charts.DonutOpts{
				Slices: []charts.Slice{
					{Label: b.t("lbl.branded"), Value: bs.Branded.Current, Color: b.accent},
					{Label: b.t("lbl.non_branded"), Value: bs.NonBranded.Current, Color: charts.ColorWarning},
				},
				CenterValue: b.pct0(bs.NonBranded.Current / total * 100),
				CenterLabel: b.t("lbl.non_branded"),
				Format:      b.fmtNum(),
				Title:       b.t("chart.brand"),
			})
		}
		bl := block{Kind: "chart", Title: b.t("chart.brand"), Sub: b.t("chart.brand.sub"), mk: svg}
		bl.Stats = []stat{
			{Label: b.t("lbl.branded"), Value: b.num(bs.Branded.Current), Delta: b.deltaRel(bs.Branded, false, b.num)},
			{Label: b.t("lbl.non_branded"), Value: b.num(bs.NonBranded.Current), Delta: b.deltaRel(bs.NonBranded, false, b.num)},
		}
		if len(bs.BrandTerms) > 0 {
			bl.Sub = b.tf("chart.brand.terms", strings.Join(bs.BrandTerms, ", "))
		}
		s.Blocks = add(s.Blocks, bl, 6, b.hint("brand"))
	}

	// CTR by position bucket.
	if idx, vals, n := ctrByPosition(sc.Queries); n >= 5 && len(vals) >= 2 {
		names := []string{b.t("lbl.top3"), b.t("lbl.top10"), b.t("lbl.top20"), b.t("lbl.beyond")}
		var labels []string
		for _, i := range idx {
			labels = append(labels, names[i])
		}
		svg := func(w, h float64) template.HTML {
			return b.columns(charts.ColumnsOpts{Width: w, Height: h, Labels: labels, Current: vals, CurrentName: b.t("col.ctr"), Format: b.fmtPct(), Title: b.t("chart.ctr_by_position")})
		}
		s.Blocks = add(s.Blocks, block{Kind: "chart", Title: b.t("chart.ctr_by_position"), Sub: b.plural(float64(n), "chart.ctr_by_position.sub"), mk: svg}, 6, b.hint("ctr_by_position"))
	}

	// Keep the grid balanced: a lone half-width chart spans the full row.
	balance(s.Blocks)

	if len(sc.SearchAppearance) > 0 {
		t := b.gscTable(sc.SearchAppearance, b.t("col.appearance"), nil)
		s.Blocks = add(s.Blocks, block{Kind: "table", Title: b.t("table.search_appearance"), Table: t}, 12, "")
	}
	if len(s.Blocks) == 0 {
		return nil
	}
	return s
}

// balance widens the last block of a run of half-width blocks when the run
// has an odd length.
func balance(blocks []block) {
	run := 0
	for i := range blocks {
		if blocks[i].Span == 6 {
			run++
			continue
		}
		if run%2 == 1 {
			blocks[i-1].Span = 12
		}
		run = 0
	}
	if run%2 == 1 {
		blocks[len(blocks)-1].Span = 12
	}
}

// gscTable renders GSC rows. keyFmt shortens the key for display.
func (b *builder) gscTable(rows []model.GSCRow, keyLabel string, keyFmt func(string) string) *table {
	hasPrevC, hasPrevI, hasPrevP, hasPos := false, false, false, false
	maxClicks := 0.0
	for i, r := range rows {
		if r.PrevClicks != nil {
			hasPrevC = true
		}
		if r.PrevImpressions != nil {
			hasPrevI = true
		}
		if r.PrevPosition != nil {
			hasPrevP = true
		}
		if r.Position > 0 {
			hasPos = true
		}
		if i < b.maxRows {
			maxClicks = math.Max(maxClicks, r.Clicks)
		}
	}
	t := &table{}
	t.Columns = append(t.Columns, column{Label: "#", Num: true, Width: "rank"}, column{Label: keyLabel, Width: "key"}, column{Label: b.t("col.clicks"), Num: true, Width: "bar"})
	if hasPrevC {
		t.Columns = append(t.Columns, column{Label: b.t("col.delta"), Num: true})
	}
	t.Columns = append(t.Columns, column{Label: b.t("col.impressions"), Num: true})
	if hasPrevI {
		t.Columns = append(t.Columns, column{Label: b.t("col.delta"), Num: true})
	}
	t.Columns = append(t.Columns, column{Label: b.t("col.ctr"), Num: true})
	if hasPos {
		t.Columns = append(t.Columns, column{Label: b.t("col.position"), Num: true})
		if hasPrevP {
			t.Columns = append(t.Columns, column{Label: b.t("col.delta"), Num: true})
		}
	}
	for i, r := range rows {
		if i >= b.maxRows {
			break
		}
		key := r.Key
		title := ""
		if keyFmt != nil {
			if k := keyFmt(r.Key); k != r.Key {
				key, title = k, r.Key
			}
		}
		cells := []cell{
			{Text: itoa(i + 1), Num: true, Muted: true},
			{Text: key, Title: title, Key: true, Wrap: true},
			{Text: b.num(r.Clicks), Sub: b.prevText(r.PrevClicks, b.num), Num: true, Bar: barStyle(r.Clicks, maxClicks)},
		}
		if hasPrevC {
			cells = append(cells, cell{Num: true, Delta: b.deltaRel(metricOf(r.Clicks, r.PrevClicks), false, b.num)})
		}
		cells = append(cells, cell{Text: b.num(r.Impressions), Sub: b.prevText(r.PrevImpressions, b.num), Num: true})
		if hasPrevI {
			cells = append(cells, cell{Num: true, Delta: b.deltaRel(metricOf(r.Impressions, r.PrevImpressions), false, nil)})
		}
		cells = append(cells, cell{Text: i18nPct2(b, r.CTR), Num: true})
		if hasPos {
			pc := cell{Text: "–", Num: true, Muted: true}
			if r.Position > 0 {
				pc = cell{Text: b.pos(r.Position), Sub: b.prevText(r.PrevPosition, b.pos), Num: true, Badges: posBadge(r.Position)}
			}
			cells = append(cells, pc)
			if hasPrevP {
				dc := cell{Num: true}
				if r.Position > 0 {
					dc.Delta = b.deltaPosition(metricOf(r.Position, r.PrevPosition))
				}
				cells = append(cells, dc)
			}
		}
		t.Rows = append(t.Rows, row{Cells: cells})
	}
	return b.limit(t, len(rows))
}

func posBadge(p float64) []badge {
	if p > 0 && p < 3.5 {
		return []badge{{Text: "Top 3", Tone: "pos"}}
	}
	return nil
}

func itoa(i int) string { return i18n.Number("en", float64(i)) }

// gscBars renders the top rows by clicks as horizontal bars.
func (b *builder) gscBars(rows []model.GSCRow, n int, title string, label func(string) string) block {
	top := topN(sortedCopy(rows, func(r model.GSCRow) float64 { return r.Clicks }), n)
	if len(top) < 2 {
		return block{}
	}
	var items []charts.BarItem
	for _, r := range top {
		l := r.Key
		if label != nil {
			l = label(l)
		}
		items = append(items, charts.BarItem{Label: truncate(l, 60), Value: r.Clicks, Previous: r.PrevClicks, Color: b.accent})
	}
	return block{Kind: "chart", Title: title, mk: func(w, h float64) template.HTML {
		return b.bars(charts.BarsOpts{Width: w, Items: items, Format: b.fmtNum(), ShowDelta: true, Title: title})
	}}
}

// moversTable is a compact table of winners or losers.
func (b *builder) moversTable(rows []model.GSCRow) *table {
	t := &table{Compact: true, Columns: []column{{Label: b.t("col.query"), Width: "key"}, {Label: b.t("col.clicks"), Num: true}, {Label: b.t("col.delta"), Num: true}}}
	for _, r := range rows {
		t.Rows = append(t.Rows, row{Cells: []cell{
			{Text: truncate(r.Key, 70), Title: r.Key, Key: true, Wrap: true},
			{Text: b.num(r.Clicks), Sub: b.prevText(r.PrevClicks, b.num), Num: true},
			{Num: true, Delta: b.deltaRel(metricOf(r.Clicks, r.PrevClicks), false, b.num)},
		}})
	}
	return t
}

// ---------------------------------------------------------------------------
// Queries

func (b *builder) queries() *section {
	sc := b.r.SearchConsole
	if sc == nil || len(sc.Queries) == 0 {
		return nil
	}
	s := &section{ID: "queries"}
	s.Blocks = append(s.Blocks, b.sectionInsights("queries")...)
	s.Blocks = add(s.Blocks, b.gscBars(sc.Queries, 10, b.t("chart.top_queries"), nil), 12, b.hint("top_queries"))
	win, lose := b.winnersLosers()
	span := 6
	if len(win) == 0 || len(lose) == 0 {
		span = 12
	}
	if len(win) > 0 {
		s.Blocks = add(s.Blocks, block{Kind: "table", Title: b.t("table.winners"), Tone: "pos", Table: b.moversTable(win)}, span, "")
	}
	if len(lose) > 0 {
		s.Blocks = add(s.Blocks, block{Kind: "table", Title: b.t("table.losers"), Tone: "neg", Table: b.moversTable(lose)}, span, "")
	}
	s.Blocks = add(s.Blocks, block{Kind: "table", Title: b.t("table.queries"), Table: b.gscTable(sc.Queries, b.t("col.query"), func(k string) string { return truncate(k, 90) })}, 12, b.hint("table_gsc"))
	return s
}

// ---------------------------------------------------------------------------
// Pages

func (b *builder) pages() *section {
	sc := b.r.SearchConsole
	if sc == nil || len(sc.Pages) == 0 {
		return nil
	}
	s := &section{ID: "pages"}
	s.Blocks = append(s.Blocks, b.sectionInsights("pages")...)
	s.Blocks = add(s.Blocks, b.gscBars(sc.Pages, 10, b.t("chart.top_pages"), shortURL), 12, b.hint("top_pages"))
	s.Blocks = add(s.Blocks, block{Kind: "table", Title: b.t("table.pages"), Table: b.gscTable(sc.Pages, b.t("col.page"), func(k string) string { return truncate(shortURL(k), 90) })}, 12, b.hint("table_gsc"))
	return s
}

// ---------------------------------------------------------------------------
// Countries & devices

func (b *builder) deviceLabel(k string) string {
	switch strings.ToLower(strings.TrimSpace(k)) {
	case "mobile":
		return b.t("lbl.mobile")
	case "desktop", "ordinateur":
		return b.t("lbl.desktop")
	case "tablet", "tablette":
		return b.t("lbl.tablet")
	}
	return k
}

func (b *builder) countriesDevices() *section {
	sc := b.r.SearchConsole
	if sc == nil || (len(sc.Countries) == 0 && len(sc.Devices) == 0) {
		return nil
	}
	s := &section{ID: "countries_devices"}
	s.Blocks = append(s.Blocks, b.sectionInsights("countries_devices")...)
	if len(sc.Devices) > 0 {
		var slices []charts.Slice
		total := 0.0
		for i, d := range sortedCopy(sc.Devices, func(r model.GSCRow) float64 { return r.Clicks }) {
			slices = append(slices, charts.Slice{Label: b.deviceLabel(d.Key), Value: d.Clicks, Color: b.seriesColor(i)})
			total += d.Clicks
		}
		svg := func(w, h float64) template.HTML {
			return b.donut(charts.DonutOpts{Slices: slices, CenterValue: i18n.Compact(b.lang, total), CenterLabel: b.t("col.clicks"), Format: b.fmtNum(), Title: b.t("chart.devices")})
		}
		s.Blocks = add(s.Blocks, block{Kind: "chart", Title: b.t("chart.devices"), Sub: b.t("chart.devices.sub"), mk: svg}, 6, b.hint("devices"))
	}
	if len(sc.Countries) > 0 {
		s.Blocks = add(s.Blocks, b.gscBars(sc.Countries, 8, b.t("chart.countries"), nil), 6, b.hint("countries"))
	}
	balance(s.Blocks)
	if len(sc.Devices) > 0 {
		s.Blocks = add(s.Blocks, block{Kind: "table", Title: b.t("table.devices"), Table: b.gscTable(sc.Devices, b.t("col.device"), b.deviceLabel)}, 12, "")
	}
	if len(sc.Countries) > 0 {
		s.Blocks = add(s.Blocks, block{Kind: "table", Title: b.t("table.countries"), Table: b.gscTable(sc.Countries, b.t("col.country"), nil)}, 12, "")
	}
	return s
}
