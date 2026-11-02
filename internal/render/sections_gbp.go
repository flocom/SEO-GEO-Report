package render

import (
	"html"
	"html/template"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/flocom/SEO-GEO-Report/internal/charts"
	"github.com/flocom/SEO-GEO-Report/internal/i18n"
	"github.com/flocom/SEO-GEO-Report/internal/insights"
	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// ---------------------------------------------------------------------------
// Google Business Profile (local visibility)

func gbpDates(pts []model.GBPDailyPoint) []string {
	out := make([]string, len(pts))
	for i, p := range pts {
		out[i] = p.Date
	}
	return out
}

func gbpSeries(pts []model.GBPDailyPoint, f func(model.GBPDailyPoint) float64) []float64 {
	out := make([]float64, 0, len(pts))
	for _, p := range pts {
		out = append(out, f(p))
	}
	return out
}

func gbpViews(p model.GBPDailyPoint) float64 { return p.Views }

// gbpInteractions returns the daily interactions, or the sum of the daily
// actions when the total is not given.
func gbpInteractions(p model.GBPDailyPoint) float64 {
	if p.Interactions > 0 {
		return p.Interactions
	}
	return p.Calls + p.WebsiteClicks + p.DirectionRequests
}

// gbpRatingDelta shows the change of an average rating in stars.
func (b *builder) gbpRatingDelta(m model.Metric) *deltaView {
	d, ok := m.Delta()
	if !ok {
		return nil
	}
	rel := 100.0
	if math.Abs(d) < 0.05 {
		rel, d = 0, 0
	}
	dir, tone := direction(d, rel, false)
	return &deltaView{Text: i18n.SignedDecimal(b.lang, d, 1), Tone: tone, Dir: dir, Arrow: arrowFor(dir, tone)}
}

func (b *builder) rating(v float64) string { return b.dec(v, 1) }

// gbpKPIs builds the Business Profile cards: views, interactions, the main
// actions and the reviews.
func (b *builder) gbpKPIs() []kpiCard {
	bp := b.r.BusinessProfile
	if bp == nil {
		return nil
	}
	t := bp.Totals
	dates := gbpDates(bp.Daily)
	var cards []kpiCard
	rel := func(key, label string, m *model.Metric, daily func(model.GBPDailyPoint) float64) {
		if m == nil {
			return
		}
		c := b.card(key, label, b.num(m.Current), m, b.num, b.deltaRel(*m, false, b.num))
		if daily != nil {
			c = b.withSpark(c, dates, gbpSeries(bp.Daily, daily), false, b.num)
		}
		cards = append(cards, c)
	}
	rel(insights.KeyGBPViews, "gbp_views", t.Views, gbpViews)
	rel(insights.KeyGBPInteractions, "gbp_interactions", t.Interactions, gbpInteractions)
	rel(insights.KeyGBPCalls, "gbp_calls", t.Calls, nil)
	rel(insights.KeyGBPDirections, "gbp_directions", t.DirectionRequests, nil)
	rel(insights.KeyGBPWebsiteClicks, "gbp_website_clicks", t.WebsiteClicks, nil)
	if rv := bp.Reviews; rv != nil {
		if m := rv.AverageRating; m != nil && m.Current > 0 {
			cards = append(cards, b.card(insights.KeyGBPRating, "gbp_rating", b.rating(m.Current)+" / 5", m, b.rating, b.gbpRatingDelta(*m)))
		}
		if m := rv.TotalReviews; m != nil {
			cards = append(cards, b.card(insights.KeyGBPReviews, "gbp_reviews", b.num(m.Current), m, b.num, b.deltaRel(*m, false, b.num)))
		}
	}
	return cards
}

func (b *builder) businessProfile() *section {
	bp := b.r.BusinessProfile
	if bp.IsEmpty() {
		return nil
	}
	s := &section{ID: "business_profile"}
	s.Blocks = append(s.Blocks, b.sectionInsights("business_profile")...)
	if cards := b.gbpKPIs(); len(cards) > 0 {
		for i := range cards {
			cards[i].Explain, cards[i].Spark = "", ""
		}
		s.Blocks = append(s.Blocks, block{Kind: "kpis", Span: 12, Tone: "compact", KPIs: cards})
	}

	dates := gbpDates(bp.Daily)
	if views := gbpSeries(bp.Daily, gbpViews); nonZero(views) {
		s.Blocks = add(s.Blocks, b.trend(b.t("chart.gbp_views_daily"), dates, views, gbpSeries(bp.PreviousDaily, gbpViews), b.fmtCompact(), false, true), 12, b.hint("gbp_views_daily"))
	} else {
		s.Blocks = add(s.Blocks, b.trend(b.t("chart.gbp_interactions_daily"), dates, gbpSeries(bp.Daily, gbpInteractions), gbpSeries(bp.PreviousDaily, gbpInteractions), b.fmtCompact(), false, true), 12, b.hint("gbp_views_daily"))
	}
	s.Blocks = add(s.Blocks, b.gbpFoundBlock(bp.Totals), 6, b.hint("gbp_found"))
	s.Blocks = add(s.Blocks, b.gbpActionsBlock(bp.Totals), 6, b.hint("gbp_actions"))
	s.Blocks = add(s.Blocks, b.gbpRatingBlock(bp.Reviews), 6, b.hint("gbp_rating"))
	s.Blocks = add(s.Blocks, b.gbpDistributionBlock(bp.Reviews), 6, b.hint("gbp_distribution"))
	s.Blocks = add(s.Blocks, b.gbpChecklistBlock(bp), 6, b.hint("gbp_checklist"))
	s.Blocks = add(s.Blocks, b.gbpActivityBlock(bp), 6, b.hint("gbp_activity"))
	balance(s.Blocks)

	if t := b.gbpKeywordsTable(bp.SearchKeywords); t != nil {
		s.Blocks = add(s.Blocks, block{Kind: "table", Title: b.t("table.gbp_keywords"), Sub: b.t("table.gbp_keywords.sub"), Table: t}, 12, b.hint("table_gbp_keywords"))
	}
	if rv := bp.Reviews; rv != nil && len(rv.Recent) > 0 {
		s.Blocks = add(s.Blocks, block{Kind: "table", Title: b.t("table.gbp_reviews"), Sub: b.t("table.gbp_reviews.sub"), Table: b.gbpReviewsTable(rv.Recent)}, 12, b.hint("table_gbp_reviews"))
	}
	if len(bp.Locations) >= 2 {
		s.Blocks = add(s.Blocks, block{Kind: "table", Title: b.t("table.gbp_locations"), Table: b.gbpLocationsTable(bp.Locations)}, 12, b.hint("table_gbp_locations"))
	}
	if len(s.Blocks) == 0 {
		return nil
	}
	return s
}

// gbpFoundBlock splits the profile views by platform (Google Search / Maps)
// and device (mobile / desktop).
func (b *builder) gbpFoundBlock(t model.GBPTotals) block {
	splits := t.ViewSplits()
	keys := []string{"lbl.gbp_search_mobile", "lbl.gbp_search_desktop", "lbl.gbp_maps_mobile", "lbl.gbp_maps_desktop"}
	colors := []string{b.accent, charts.Palette[1%len(charts.Palette)], charts.ColorPositive, "#82b896"}
	var slices []charts.Slice
	total := 0.0
	for i, m := range splits {
		if m == nil || m.Current <= 0 {
			continue
		}
		slices = append(slices, charts.Slice{Label: b.t(keys[i]), Value: m.Current, Color: colors[i]})
		total += m.Current
	}
	if len(slices) < 2 {
		return block{}
	}
	var st []stat
	if maps, mobile, ok := insights.GBPViewShares(t); ok {
		st = []stat{
			{Label: b.t("lbl.gbp_search"), Value: b.pct0(100 - maps)},
			{Label: b.t("lbl.gbp_maps"), Value: b.pct0(maps)},
			{Label: b.t("lbl.gbp_mobile"), Value: b.pct0(mobile)},
			{Label: b.t("lbl.gbp_desktop"), Value: b.pct0(100 - mobile)},
		}
	}
	title := b.t("chart.gbp_found")
	return block{Kind: "chart", Title: title, Sub: b.t("chart.gbp_found.sub"), Stats: st, mk: func(w, h float64) template.HTML {
		return b.donut(charts.DonutOpts{Slices: slices, CenterValue: i18n.Compact(b.lang, total), CenterLabel: b.t("chart.gbp_views"), Format: b.fmtNum(), Title: title})
	}}
}

// gbpActionsBlock shows what people did from the profile.
func (b *builder) gbpActionsBlock(t model.GBPTotals) block {
	var items []charts.BarItem
	for _, a := range t.Actions() {
		if a.Metric.Current <= 0 && (a.Metric.Previous == nil || *a.Metric.Previous <= 0) {
			continue
		}
		items = append(items, charts.BarItem{Label: b.t("gbp.act." + a.Key), Value: a.Metric.Current, Previous: a.Metric.Previous, Color: b.accent})
	}
	if len(items) < 2 {
		return block{}
	}
	title := b.t("chart.gbp_actions")
	return block{Kind: "chart", Title: title, Sub: b.t("chart.gbp_actions.sub"), mk: func(w, h float64) template.HTML {
		return b.bars(charts.BarsOpts{Width: w, Items: items, Format: b.fmtNum(), ShowDelta: true, Title: title})
	}}
}

// gbpStar is one star of a 24×24 grid.
const gbpStar = `M12 2.2l2.95 6.1 6.7.85-4.9 4.65 1.25 6.6L12 17.2l-5.95 3.2 1.25-6.6-4.9-4.65 6.7-.85z`

// gbpStarsSVG draws five stars in one color.
func gbpStarsSVG(color string) string {
	var sb strings.Builder
	sb.WriteString(`<svg class="gbp-stars-svg" viewBox="0 0 120 24" width="150" height="30" aria-hidden="true">`)
	for i := 0; i < 5; i++ {
		sb.WriteString(`<path transform="translate(` + strconv.Itoa(i*24) + ` 0)" d="` + gbpStar + `" fill="` + color + `"/>`)
	}
	sb.WriteString(`</svg>`)
	return sb.String()
}

// gbpStars renders a rating as five stars filled up to the rating (no
// script, no SVG id: a clipped overlay in CSS).
func (b *builder) gbpStars(rating float64) string {
	w := math.Max(0, math.Min(rating, 5)) / 5 * 100
	label := html.EscapeString(b.tf("lbl.gbp_stars_aria", b.rating(rating)))
	return `<span class="gbp-stars" role="img" aria-label="` + label + `">` + gbpStarsSVG("#d6d9de") +
		`<span class="gbp-stars-fg" style="width:` + strconv.FormatFloat(w, 'f', 1, 64) + `%">` + gbpStarsSVG("#f2b01e") + `</span></span>`
}

// gbpRatingBlock shows the average rating in big, with stars, the number of
// reviews and the review statistics.
func (b *builder) gbpRatingBlock(rv *model.GBPReviews) block {
	if rv == nil || rv.AverageRating == nil || rv.AverageRating.Current <= 0 {
		return block{}
	}
	m := *rv.AverageRating
	var sb strings.Builder
	sb.WriteString(`<div class="gbp-rating"><div class="gbp-rating-main"><span class="gbp-rating-value">`)
	sb.WriteString(html.EscapeString(b.rating(m.Current)))
	sb.WriteString(`</span><span class="gbp-rating-of">`)
	sb.WriteString(html.EscapeString(b.t("lbl.gbp_out_of")))
	sb.WriteString(`</span></div>`)
	sb.WriteString(b.gbpStars(m.Current))
	var meta []string
	if rv.TotalReviews != nil && rv.TotalReviews.Current > 0 {
		meta = append(meta, html.EscapeString(b.plural(rv.TotalReviews.Current, "lbl.gbp_reviews")))
	}
	if d := b.gbpRatingDelta(m); d != nil {
		meta = append(meta, `<span class="delta d-`+d.Tone+`">`+string(d.Arrow)+`<span>`+html.EscapeString(d.Text)+`</span></span> <span class="gbp-rating-prev">`+
			html.EscapeString(b.tf("kpi.prev", b.rating(*m.Previous)))+`</span>`)
	}
	if len(meta) > 0 {
		sb.WriteString(`<p class="gbp-rating-meta">` + strings.Join(meta, ` <span class="gbp-sep">·</span> `) + `</p>`)
	}
	sb.WriteString(`</div>`)

	var st []stat
	if n := rv.NewReviews; n != nil {
		st = append(st, stat{Label: b.t("lbl.gbp_new_reviews"), Value: b.num(n.Current), Delta: b.deltaRel(*n, false, nil)})
	}
	if r := rv.ResponseRate; r != nil {
		tone := ""
		if r.Current < insights.GBPResponseTarget {
			tone = "warn"
		}
		st = append(st, stat{Label: b.t("lbl.gbp_response_rate"), Value: b.pct0(r.Current), Tone: tone, Delta: b.deltaPts(*r)})
	}
	if t := rv.TotalReviews; t != nil && t.Previous != nil {
		st = append(st, stat{Label: b.t("lbl.gbp_total_reviews"), Value: b.num(t.Current), Delta: b.deltaRel(*t, false, nil)})
	}
	tone := "pos"
	switch {
	case m.Current < 4:
		tone = "neg"
	case m.Current < 4.5:
		tone = "warn"
	}
	return block{Kind: "chart", Title: b.t("chart.gbp_rating"), Tone: tone, Stats: st,
		Chart: template.HTML(sb.String())} //nolint:gosec // every dynamic part is escaped
}

// gbpDistributionBlock shows the number of reviews per star rating.
func (b *builder) gbpDistributionBlock(rv *model.GBPReviews) block {
	if rv == nil || rv.RatingDistribution == nil || rv.RatingDistribution.Total() <= 0 {
		return block{}
	}
	counts := rv.RatingDistribution.Counts()
	colors := []string{charts.ColorNegative, charts.ColorNegative, charts.ColorWarning, "#82b896", charts.ColorPositive}
	var items []charts.BarItem
	for i := 4; i >= 0; i-- {
		items = append(items, charts.BarItem{Label: b.plural(float64(i+1), "lbl.gbp_stars"), Value: counts[i], Color: colors[i]})
	}
	title := b.t("chart.gbp_distribution")
	return block{Kind: "chart", Title: title, Sub: b.t("chart.gbp_distribution.sub"), mk: func(w, h float64) template.HTML {
		return b.bars(charts.BarsOpts{Width: w, Items: items, Format: b.fmtNum(), Title: title})
	}}
}

// gbpChecklistBlock lists the profile information filled in or missing.
func (b *builder) gbpChecklistBlock(bp *model.BusinessProfile) block {
	if len(bp.Checklist) == 0 {
		return block{}
	}
	var todo []planItem
	var done []string
	for _, c := range bp.Checklist {
		label := i18n.GBPChecklistLabel(b.lang, c.Item)
		if label == "" {
			continue
		}
		if c.Done {
			done = append(done, label)
		} else {
			todo = append(todo, planItem{Title: label, Icon: icoAlert, Tone: "warn", Badges: []badge{{Text: b.t("lbl.gbp_todo"), Tone: "warn"}}})
		}
	}
	// What is missing first, one line each; what is done on a single line
	// so that the card stays short.
	items := todo
	if len(done) > 0 {
		items = append(items, planItem{Title: strings.Join(done, ", "), Icon: icoCheck, Tone: "pos",
			Badges: []badge{{Text: b.t("lbl.gbp_done") + " · " + b.num(float64(len(done))), Tone: "pos"}}})
	}
	if len(items) == 0 {
		return block{}
	}
	bl := block{Kind: "plan", Title: b.t("chart.gbp_checklist"), Tone: "pos", Plan: items}
	if m := bp.ProfileCompleteness; m != nil {
		bl.Sub = b.tf("chart.gbp_checklist.sub", b.pct0(m.Current))
	}
	if len(todo) > 0 {
		bl.Tone = "warn"
	}
	return bl
}

// gbpActivityBlock shows the publishing activity (posts, photos) and the
// profile completeness when there is no checklist.
func (b *builder) gbpActivityBlock(bp *model.BusinessProfile) block {
	var st []stat
	if m := bp.Posts; m != nil {
		st = append(st, stat{Label: b.t("lbl.gbp_posts"), Value: b.num(m.Current), Sub: b.prevText(m.Previous, b.num), Delta: b.deltaRel(*m, false, nil)})
	}
	if m := bp.Photos; m != nil {
		st = append(st, stat{Label: b.t("lbl.gbp_photos"), Value: b.num(m.Current), Sub: b.prevText(m.Previous, b.num), Delta: b.deltaRel(*m, false, nil)})
	}
	if m := bp.ProfileCompleteness; m != nil && len(bp.Checklist) == 0 {
		st = append(st, stat{Label: b.t("lbl.gbp_completeness"), Value: b.pct0(m.Current), Delta: b.deltaPts(*m)})
	}
	if len(st) == 0 {
		return block{}
	}
	return block{Kind: "stats", Title: b.t("chart.gbp_activity"), Sub: b.t("chart.gbp_activity.sub"), Stats: st}
}

// gbpSearches formats a number of searches, "fewer than 15" for thresholds.
func (b *builder) gbpSearches(v float64, below bool) string {
	if below {
		return b.tf("lbl.gbp_below", b.num(v))
	}
	return b.num(v)
}

// gbpKeywordsTable lists the local searches that showed the profile.
func (b *builder) gbpKeywordsTable(rows []model.GBPKeyword) *table {
	var kept []model.GBPKeyword
	for _, k := range insights.SortGBPKeywords(rows) {
		if strings.TrimSpace(k.Keyword) != "" {
			kept = append(kept, k)
		}
	}
	if len(kept) == 0 {
		return nil
	}
	hasPrev := false
	maxV := 0.0
	for i, k := range kept {
		hasPrev = hasPrev || k.PrevImpressions != nil
		if i < b.maxRows {
			maxV = math.Max(maxV, k.Impressions)
		}
	}
	t := &table{Columns: []column{{Label: "#", Num: true, Width: "rank"}, {Label: b.t("col.gbp_keyword"), Width: "key"}, {Label: b.t("col.gbp_searches"), Num: true, Width: "bar"}}}
	if hasPrev {
		t.Columns = append(t.Columns, column{Label: b.t("col.delta"), Num: true})
	}
	for i, k := range kept {
		if i >= b.maxRows {
			break
		}
		cells := []cell{
			{Text: itoa(i + 1), Num: true, Muted: true},
			{Text: truncate(k.Keyword, 90), Title: k.Keyword, Key: true, Wrap: true},
			{Text: b.gbpSearches(k.Impressions, k.BelowThreshold), Sub: b.prevText(k.PrevImpressions, b.num), Num: true, Bar: barStyle(k.Impressions, maxV), Muted: k.BelowThreshold},
		}
		if hasPrev {
			var d *deltaView
			if !k.BelowThreshold {
				d = b.deltaRel(metricOf(k.Impressions, k.PrevImpressions), false, b.num)
			}
			cells = append(cells, cell{Num: true, Delta: d})
		}
		t.Rows = append(t.Rows, row{Cells: cells})
	}
	return b.limit(t, len(kept))
}

// gbpReviewsTable lists recent reviews, newest first; texts are kept as the
// customers wrote them (escaped by the template).
func (b *builder) gbpReviewsTable(reviews []model.GBPReview) *table {
	rows := append([]model.GBPReview(nil), reviews...)
	hasDate := false
	for _, r := range rows {
		hasDate = hasDate || r.Date != ""
	}
	if hasDate {
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].Date > rows[j].Date })
	}
	t := &table{Columns: []column{{Label: b.t("col.gbp_rating")}, {Label: b.t("col.gbp_review"), Width: "wide"}}}
	if hasDate {
		t.Columns = append(t.Columns, column{Label: b.t("col.gbp_date")})
	}
	t.Columns = append(t.Columns, column{Label: b.t("col.gbp_reply")})
	for i, r := range rows {
		if i >= b.maxRows {
			break
		}
		tone := "pos"
		switch {
		case r.Rating <= 2:
			tone = "neg"
		case r.Rating < 4:
			tone = "warn"
		}
		cells := []cell{
			{Badges: []badge{{Text: "★ " + b.dec(r.Rating, 0), Tone: tone}}},
			{Text: truncate(strings.TrimSpace(r.Text), 400), Sub: r.Author, Wrap: true, Muted: strings.TrimSpace(r.Text) == ""},
		}
		if hasDate {
			cells = append(cells, cell{Text: b.date(r.Date), Muted: true})
		}
		reply := badge{Text: b.t("lbl.gbp_replied"), Tone: "pos"}
		if !r.Responded {
			reply = badge{Text: b.t("lbl.gbp_not_replied"), Tone: "warn"}
		}
		cells = append(cells, cell{Badges: []badge{reply}})
		t.Rows = append(t.Rows, row{Cells: cells})
	}
	return b.limit(t, len(rows))
}

// gbpLocationsTable compares the establishments of a multi-location
// business, sorted by views (else interactions).
func (b *builder) gbpLocationsTable(locs []model.GBPLocation) *table {
	val := func(m *model.Metric) float64 {
		if m == nil {
			return 0
		}
		return m.Current
	}
	rows := sortedCopy(locs, func(l model.GBPLocation) float64 {
		return val(l.Totals.Views)*1e6 + val(l.Totals.Interactions)
	})
	has := func(f func(model.GBPTotals) *model.Metric) bool {
		for _, l := range rows {
			if f(l.Totals) != nil {
				return true
			}
		}
		return false
	}
	type col struct {
		key string
		f   func(model.GBPTotals) *model.Metric
	}
	cols := []col{
		{"col.gbp_views", func(t model.GBPTotals) *model.Metric { return t.Views }},
		{"col.gbp_actions", func(t model.GBPTotals) *model.Metric { return t.Interactions }},
		{"col.gbp_calls", func(t model.GBPTotals) *model.Metric { return t.Calls }},
		{"col.gbp_directions", func(t model.GBPTotals) *model.Metric { return t.DirectionRequests }},
		{"col.gbp_website", func(t model.GBPTotals) *model.Metric { return t.WebsiteClicks }},
	}
	var shown []col
	for _, c := range cols {
		if has(c.f) {
			shown = append(shown, c)
		}
	}
	hasRating, hasReviews := false, false
	maxViews := 0.0
	for _, l := range rows {
		hasRating = hasRating || l.Rating != nil
		hasReviews = hasReviews || l.ReviewCount != nil
		maxViews = math.Max(maxViews, val(l.Totals.Views))
	}
	t := &table{Columns: []column{{Label: b.t("col.gbp_location"), Width: "key"}}}
	for i, c := range shown {
		w := ""
		if i == 0 && c.key == "col.gbp_views" {
			w = "bar"
		}
		t.Columns = append(t.Columns, column{Label: b.t(c.key), Num: true, Width: w})
	}
	if hasRating {
		t.Columns = append(t.Columns, column{Label: b.t("col.gbp_rating"), Num: true})
	}
	if hasReviews {
		t.Columns = append(t.Columns, column{Label: b.t("col.gbp_reviews"), Num: true})
	}
	for i, l := range rows {
		if i >= b.maxRows {
			break
		}
		sub := strings.TrimSpace(strings.Join(nonEmpty(l.Address, l.City), ", "))
		if l.StoreCode != "" {
			sub = strings.TrimSpace(strings.Join(nonEmpty(l.StoreCode, sub), " · "))
		}
		name := l.Name
		if strings.TrimSpace(name) == "" {
			name = l.StoreCode
		}
		cells := []cell{{Text: name, Sub: sub, Key: true, Wrap: true}}
		for j, c := range shown {
			m := c.f(l.Totals)
			if m == nil {
				cells = append(cells, cell{Text: "–", Num: true, Muted: true})
				continue
			}
			ce := cell{Text: b.num(m.Current), Num: true}
			if d := b.deltaRel(*m, false, nil); d != nil {
				ce.Sub = d.Text
			}
			if j == 0 && c.key == "col.gbp_views" {
				ce.Bar = barStyle(m.Current, maxViews)
			}
			cells = append(cells, ce)
		}
		if hasRating {
			if l.Rating != nil {
				cells = append(cells, cell{Text: b.rating(*l.Rating) + " ★", Num: true})
			} else {
				cells = append(cells, cell{Text: "–", Num: true, Muted: true})
			}
		}
		if hasReviews {
			if l.ReviewCount != nil {
				cells = append(cells, cell{Text: b.num(*l.ReviewCount), Num: true})
			} else {
				cells = append(cells, cell{Text: "–", Num: true, Muted: true})
			}
		}
		t.Rows = append(t.Rows, row{Cells: cells})
	}
	return b.limit(t, len(rows))
}

func nonEmpty(ss ...string) []string {
	var out []string
	for _, s := range ss {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// gbpPart1Key returns the Business Profile card shown in the Part 1 key
// indicators: interactions (what people did), else views; "" without data.
func gbpPart1Key(all map[string]kpiCard) string {
	for _, k := range []string{insights.KeyGBPInteractions, insights.KeyGBPViews} {
		if _, ok := all[k]; ok {
			return k
		}
	}
	return ""
}

// gbpPart1Keys returns the Business Profile cards allowed in the Part 1 key
// indicators: the headline card (interactions, else views), calls (else
// website clicks) and the rating. The full set stays in the section.
func gbpPart1Keys(all map[string]kpiCard) map[string]bool {
	kept := map[string]bool{}
	if k := gbpPart1Key(all); k != "" {
		kept[k] = true
	}
	for _, k := range []string{insights.KeyGBPCalls, insights.KeyGBPWebsiteClicks} {
		if _, ok := all[k]; ok {
			kept[k] = true
			break
		}
	}
	if _, ok := all[insights.KeyGBPRating]; ok {
		kept[insights.KeyGBPRating] = true
	}
	return kept
}

// gbpSkipped reports whether a Part 1 card is a Business Profile card left
// for the section.
func gbpSkipped(key string, kept map[string]bool) bool {
	return strings.HasPrefix(key, "gbp.") && !kept[key]
}

// gbpGlossaryKept reports whether a glossary entry is shown: the Business
// Profile terms only with Business Profile data.
func (b *builder) gbpGlossaryKept(key string) bool {
	return !i18n.GBPGlossaryKeys[key] || !b.r.BusinessProfile.IsEmpty()
}

// icoStore is the Business Profile section icon: a storefront with a pin.
var icoStore = icon(`<path d="M4 10v10h16V10"/><path d="M3 10l1.5-6h15L21 10a3 3 0 0 1-6 0 3 3 0 0 1-6 0 3 3 0 0 1-6 0z"/><path d="M10 20v-5h4v5"/>`)
