package render

import (
	"html/template"
	"math"

	"github.com/flocom/SEO-GEO-Report/internal/charts"
	"github.com/flocom/SEO-GEO-Report/internal/i18n"
	"github.com/flocom/SEO-GEO-Report/internal/insights"
	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// ---------------------------------------------------------------------------
// Google Merchant Center (Google Shopping)

func gmcDates(pts []model.GMCDailyPoint) []string {
	out := make([]string, len(pts))
	for i, p := range pts {
		out[i] = p.Date
	}
	return out
}

func gmcSeries(pts []model.GMCDailyPoint, f func(model.GMCDailyPoint) float64) []float64 {
	out := make([]float64, 0, len(pts))
	for _, p := range pts {
		out = append(out, f(p))
	}
	return out
}

func gmcClicks(p model.GMCDailyPoint) float64      { return p.Clicks }
func gmcImpressions(p model.GMCDailyPoint) float64 { return p.Impressions }

// gmcKPIs builds the Merchant Center cards: performance (clicks, impressions,
// CTR, conversions, conversion value) and feed health (approved products).
func (b *builder) gmcKPIs() []kpiCard {
	mc := b.r.MerchantCenter
	if mc == nil {
		return nil
	}
	var cards []kpiCard
	t := mc.Totals
	dates := gmcDates(mc.Daily)
	if mc.HasPerformance() {
		cards = append(cards,
			b.withSpark(b.card(insights.KeyGMCClicks, "gmc_clicks", b.num(t.Clicks.Current), &t.Clicks, b.num, b.deltaRel(t.Clicks, false, b.num)),
				dates, gmcSeries(mc.Daily, gmcClicks), false, b.num),
			b.withSpark(b.card(insights.KeyGMCImpressions, "gmc_impressions", b.num(t.Impressions.Current), &t.Impressions, b.num, b.deltaRel(t.Impressions, false, b.num)),
				dates, gmcSeries(mc.Daily, gmcImpressions), false, b.num))
		if t.CTR != nil {
			pct := func(v float64) string { return i18nPct2(b, v) }
			cards = append(cards, b.card(insights.KeyGMCCTR, "gmc_ctr", pct(t.CTR.Current), t.CTR, pct, b.deltaPts(*t.CTR)))
		}
	}
	if m := t.Conversions; m != nil {
		cards = append(cards, b.card(insights.KeyGMCConversions, "gmc_conversions", b.num(m.Current), m, b.num, b.deltaRel(*m, false, b.num)))
	}
	if m := t.ConversionValue; m != nil {
		cards = append(cards, b.card("gmc.conversion_value", "gmc_conversion_value", b.money(m.Current), m, b.money, b.deltaRel(*m, false, b.money)))
	}
	if rate := mc.ProductStatus.ApprovedRate(); rate != nil {
		cards = append(cards, b.card(insights.KeyGMCApprovedRate, "gmc_approved_rate", b.pct(rate.Current), rate, b.pct, b.deltaPts(*rate)))
	}
	return cards
}

func (b *builder) merchantCenter() *section {
	mc := b.r.MerchantCenter
	if mc == nil {
		return nil
	}
	s := &section{ID: "merchant_center"}
	s.Blocks = append(s.Blocks, b.sectionInsights("merchant_center")...)
	if cards := b.gmcKPIs(); len(cards) > 0 {
		for i := range cards {
			cards[i].Explain, cards[i].Spark = "", ""
		}
		s.Blocks = append(s.Blocks, block{Kind: "kpis", Span: 12, Tone: "compact", KPIs: cards})
	}

	dates := gmcDates(mc.Daily)
	s.Blocks = add(s.Blocks, b.trend(b.t("chart.gmc_clicks_daily"), dates, gmcSeries(mc.Daily, gmcClicks), gmcSeries(mc.PreviousDaily, gmcClicks), b.fmtCompact(), false, true), 12, b.hint("gmc_clicks_daily"))
	s.Blocks = add(s.Blocks, b.gmcProgramsBlock(mc.Programs), 6, b.hint("gmc_programs"))
	s.Blocks = add(s.Blocks, b.gmcStatusBlock(mc.ProductStatus), 6, b.hint("gmc_status"))
	s.Blocks = add(s.Blocks, b.gmcBars(mc.Products, 10, b.t("chart.gmc_top_products")), 6, b.hint("gmc_top_products"))
	s.Blocks = add(s.Blocks, b.gmcBars(mc.Categories, 8, b.t("chart.gmc_categories")), 6, b.hint("gmc_categories"))
	balance(s.Blocks)

	if len(mc.Products) > 0 {
		s.Blocks = add(s.Blocks, block{Kind: "table", Title: b.t("table.gmc_products"), Table: b.gmcTable(mc.Products, b.t("col.product"))}, 12, b.hint("table_gmc_products"))
	}
	if len(mc.Issues) > 0 {
		s.Blocks = add(s.Blocks, block{Kind: "table", Title: b.t("table.gmc_issues"), Table: b.gmcIssuesTable(mc.Issues)}, 12, b.hint("table_gmc_issues"))
	}
	if t := b.gmcPricesTable(mc.PriceBenchmarks); t != nil {
		s.Blocks = add(s.Blocks, block{Kind: "table", Title: b.t("table.gmc_prices"), Sub: b.t("table.gmc_prices.sub"), Table: t}, 12, b.hint("table_gmc_prices"))
	}
	for _, d := range []struct {
		rows       []model.GMCRow
		title, col string
	}{
		{mc.Categories, "table.gmc_categories", "col.category"},
		{mc.Brands, "table.gmc_brands", "col.brand"},
		{mc.Countries, "table.gmc_countries", "col.country"},
	} {
		if len(d.rows) > 0 {
			s.Blocks = add(s.Blocks, block{Kind: "table", Title: b.t(d.title), Table: b.gmcTable(d.rows, b.t(d.col))}, 12, "")
		}
	}
	if len(s.Blocks) == 0 {
		return nil
	}
	return s
}

// gmcProgramsBlock splits the clicks by program (free listings, Shopping
// ads, Images, YouTube...).
func (b *builder) gmcProgramsBlock(rows []model.GMCRow) block {
	rows = sortedCopy(rows, func(r model.GMCRow) float64 { return r.Clicks })
	var slices []charts.Slice
	var st []stat
	total := 0.0
	for i, r := range rows {
		if r.Clicks <= 0 {
			continue
		}
		slices = append(slices, charts.Slice{Label: r.Key, Value: r.Clicks, Color: b.seriesColor(i)})
		total += r.Clicks
		if len(st) < 4 {
			st = append(st, stat{Label: r.Key, Value: b.num(r.Clicks), Delta: b.deltaRel(metricOf(r.Clicks, r.PrevClicks), false, nil)})
		}
	}
	if len(slices) < 2 {
		return block{}
	}
	title := b.t("chart.gmc_programs")
	return block{Kind: "chart", Title: title, Sub: b.t("chart.gmc_programs.sub"), Stats: st, mk: func(w, h float64) template.HTML {
		return b.donut(charts.DonutOpts{Slices: slices, CenterValue: i18n.Compact(b.lang, total), CenterLabel: b.t("col.clicks"), Format: b.fmtNum(), Title: title})
	}}
}

// gmcStatusBlock shows the products of the feed per status as a 100 %
// stacked bar with one tile per status.
func (b *builder) gmcStatusBlock(ps *model.GMCProductStatus) block {
	rate := ps.ApprovedRate()
	if rate == nil {
		return block{}
	}
	total := ps.TotalProducts()
	type part struct {
		key   string
		m     *model.Metric
		color string
		tone  string
		lower bool // fewer is better
	}
	parts := []part{
		{"lbl.approved", &ps.Approved, charts.ColorPositive, "pos", false},
		{"lbl.limited", ps.Limited, charts.ColorWarning, "warn", true},
		{"lbl.not_approved", ps.NotApproved, charts.ColorNegative, "neg", true},
		{"lbl.pending", ps.Pending, charts.ColorPrevious, "", true},
	}
	var segs []charts.StackSegment
	var st []stat
	for _, p := range parts {
		if p.m == nil {
			continue
		}
		segs = append(segs, charts.StackSegment{Label: b.t(p.key), Value: p.m.Current, Color: p.color})
		s := stat{Label: b.t(p.key), Value: b.num(p.m.Current), Tone: p.tone, Delta: b.deltaRel(*p.m, p.lower, nil)}
		if total.Current > 0 {
			s.Sub = b.pct0(p.m.Current / total.Current * 100)
		}
		st = append(st, s)
	}
	tone := "pos"
	switch {
	case rate.Current < 75:
		tone = "neg"
	case rate.Current < 90:
		tone = "warn"
	}
	title := b.t("chart.gmc_status")
	return block{Kind: "chart", Title: title, Sub: b.tf("chart.gmc_status.sub", b.pct0(rate.Current), b.num(total.Current)), Tone: tone, Stats: st, mk: func(w, h float64) template.HTML {
		return b.stacked(charts.StackedOpts{Width: w, Segments: segs, Format: b.fmtNum(), Title: title})
	}}
}

// gmcBars renders the top rows by clicks as horizontal bars.
func (b *builder) gmcBars(rows []model.GMCRow, n int, title string) block {
	top := topN(sortedCopy(rows, func(r model.GMCRow) float64 { return r.Clicks }), n)
	if len(top) < 2 {
		return block{}
	}
	var items []charts.BarItem
	for _, r := range top {
		items = append(items, charts.BarItem{Label: truncate(r.Key, 60), Value: r.Clicks, Previous: r.PrevClicks, Color: b.accent})
	}
	return block{Kind: "chart", Title: title, mk: func(w, h float64) template.HTML {
		return b.bars(charts.BarsOpts{Width: w, Items: items, Format: b.fmtNum(), ShowDelta: true, Title: title})
	}}
}

// gmcTable renders Merchant Center performance rows (products, categories,
// brands, countries), sorted by clicks.
func (b *builder) gmcTable(rows []model.GMCRow, keyLabel string) *table {
	rows = sortedCopy(rows, func(r model.GMCRow) float64 { return r.Clicks })
	hasPrevC, hasPrevI, hasConv, hasValue := false, false, false, false
	maxClicks := 0.0
	for i, r := range rows {
		hasPrevC = hasPrevC || r.PrevClicks != nil
		hasPrevI = hasPrevI || r.PrevImpressions != nil
		hasConv = hasConv || r.Conversions > 0
		hasValue = hasValue || r.ConversionValue > 0
		if i < b.maxRows {
			maxClicks = math.Max(maxClicks, r.Clicks)
		}
	}
	t := &table{Columns: []column{{Label: "#", Num: true, Width: "rank"}, {Label: keyLabel, Width: "key"}, {Label: b.t("col.clicks"), Num: true, Width: "bar"}}}
	if hasPrevC {
		t.Columns = append(t.Columns, column{Label: b.t("col.delta"), Num: true})
	}
	t.Columns = append(t.Columns, column{Label: b.t("col.impressions"), Num: true})
	if hasPrevI {
		t.Columns = append(t.Columns, column{Label: b.t("col.delta"), Num: true})
	}
	t.Columns = append(t.Columns, column{Label: b.t("col.ctr"), Num: true})
	if hasConv {
		t.Columns = append(t.Columns, column{Label: b.t("col.conversions"), Num: true})
	}
	if hasValue {
		t.Columns = append(t.Columns, column{Label: b.t("col.conv_value"), Num: true})
	}
	for i, r := range rows {
		if i >= b.maxRows {
			break
		}
		key := truncate(r.Key, 90)
		title := ""
		if key != r.Key {
			title = r.Key
		}
		cells := []cell{
			{Text: itoa(i + 1), Num: true, Muted: true},
			{Text: key, Title: title, Sub: r.OfferID, Key: true, Wrap: true},
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
		if hasConv {
			cells = append(cells, cell{Text: b.num(r.Conversions), Num: true, Muted: r.Conversions == 0})
		}
		if hasValue {
			cells = append(cells, cell{Text: b.money(r.ConversionValue), Num: true, Muted: r.ConversionValue == 0})
		}
		t.Rows = append(t.Rows, row{Cells: cells})
	}
	return b.limit(t, len(rows))
}

func (b *builder) severityBadge(s string) []badge {
	switch model.NormSeverity(s) {
	case model.SeverityCritical:
		return []badge{{Text: b.t("sev.critical"), Tone: "neg"}}
	case model.SeverityWarning:
		return []badge{{Text: b.t("sev.warning"), Tone: "warn"}}
	case model.SeverityInfo:
		return []badge{{Text: b.t("sev.info"), Tone: "neu"}}
	case "":
		return nil
	}
	return []badge{{Text: s, Tone: "neu"}}
}

// gmcIssuesTable lists the feed issues, most severe and widespread first.
func (b *builder) gmcIssuesTable(issues []model.GMCIssue) *table {
	rows := insights.SortGMCIssues(issues)
	hasSev, hasPrev, hasAffects := false, false, false
	maxP := 0.0
	for i, r := range rows {
		hasSev = hasSev || r.Severity != ""
		hasPrev = hasPrev || r.PrevProducts != nil
		hasAffects = hasAffects || r.Affects != ""
		if i < b.maxRows {
			maxP = math.Max(maxP, r.Products)
		}
	}
	t := &table{Columns: []column{{Label: b.t("col.issue"), Width: "wide"}}}
	if hasSev {
		t.Columns = append(t.Columns, column{Label: b.t("col.severity")})
	}
	t.Columns = append(t.Columns, column{Label: b.t("col.products"), Num: true, Width: "bar"})
	if hasPrev {
		t.Columns = append(t.Columns, column{Label: b.t("col.delta"), Num: true})
	}
	if hasAffects {
		t.Columns = append(t.Columns, column{Label: b.t("col.affects")})
	}
	for i, r := range rows {
		if i >= b.maxRows {
			break
		}
		tone := "warn"
		if model.SeverityRank(r.Severity) == 0 {
			tone = "neg"
		}
		cells := []cell{{Text: r.Issue, Key: true, Wrap: true}}
		if hasSev {
			cells = append(cells, cell{Badges: b.severityBadge(r.Severity)})
		}
		cells = append(cells, cell{Text: b.num(r.Products), Sub: b.prevText(r.PrevProducts, b.num), Num: true, Bar: barStyle(r.Products, maxP), BarTone: tone})
		if hasPrev {
			cells = append(cells, cell{Num: true, Delta: b.deltaRel(metricOf(r.Products, r.PrevProducts), true, b.num)})
		}
		if hasAffects {
			cells = append(cells, cell{Text: r.Affects, Muted: r.Affects == "", Wrap: true})
		}
		t.Rows = append(t.Rows, row{Cells: cells})
	}
	return b.limit(t, len(rows))
}

// gmcPricesTable compares product prices with the benchmark price, most
// expensive products (relative to the market) first.
func (b *builder) gmcPricesTable(prices []model.GMCPriceBenchmark) *table {
	type item struct {
		p   model.GMCPriceBenchmark
		gap float64
	}
	var items []item
	for _, p := range prices {
		if gap, ok := insights.PriceGap(p); ok {
			items = append(items, item{p, gap})
		}
	}
	if len(items) == 0 {
		return nil
	}
	items = sortedCopy(items, func(it item) float64 { return it.gap })
	t := &table{Columns: []column{
		{Label: b.t("col.product"), Width: "wide"}, {Label: b.t("col.price"), Num: true},
		{Label: b.t("col.benchmark"), Num: true}, {Label: b.t("col.price_gap"), Num: true},
	}}
	for i, it := range items {
		if i >= b.maxRows {
			break
		}
		tone, sub := "neu", b.t("price.market")
		switch {
		case it.gap > insights.PriceGapTolerance:
			tone, sub = "neg", b.t("price.above")
		case it.gap < -insights.PriceGapTolerance:
			tone, sub = "pos", b.t("price.below")
		}
		digits := 1
		if math.Abs(it.gap) >= 100 {
			digits = 0
		}
		t.Rows = append(t.Rows, row{Cells: []cell{
			{Text: truncate(it.p.Product, 90), Title: it.p.Product, Sub: it.p.OfferID, Key: true, Wrap: true},
			{Text: b.money(it.p.Price), Num: true},
			{Text: b.money(it.p.BenchmarkPrice), Num: true, Muted: true},
			{Num: true, Badges: []badge{{Text: i18n.SignedPercent(b.lang, it.gap, digits), Tone: tone}}, Sub: sub},
		}})
	}
	return b.limit(t, len(items))
}
