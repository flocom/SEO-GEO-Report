package render

import (
	"html/template"
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/flocom/SEO-GEO-Report/internal/i18n"
	"github.com/flocom/SEO-GEO-Report/internal/insights"
	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// Beginner-friendly building blocks of part 1: the color legend, the channel
// scorecard and the "wins / to watch" tiles. They read in a few seconds: a
// pictogram, one big figure, a colored arrow and a few plain words.

// ---------------------------------------------------------------------------
// Tones

// Traffic-light symbols of the four tones, shared by the legend, the
// scorecard and the tiles.
var toneSign = map[string]string{"pos": "▲", "neg": "▼", "exp": "●", "neu": "▶"}

// kpiTone returns the display tone of a KPI change: the insights status when
// known (with the "expected" amber tone), else the direction of the change.
func (b *builder) kpiTone(key string, m *model.Metric, lowerBetter bool) string {
	if st, ok := b.an.KPIs[key]; ok {
		if st.Expected {
			return "exp"
		}
		if st.HasDelta || m == nil || m.Previous == nil || *m.Previous != 0 {
			return toneFromInsights(st.Tone)
		}
	}
	if m == nil || m.Previous == nil {
		return "neu"
	}
	d := m.Current - *m.Previous
	rel := 100.0
	if *m.Previous != 0 {
		rel = d / math.Abs(*m.Previous) * 100
	}
	_, tone := direction(d, rel, lowerBetter)
	return tone
}

// legend is the color code, shown once at the top of part 1.
func (b *builder) legend() block {
	items := []legendItem{}
	for _, t := range []string{"pos", "neg", "exp", "neu"} {
		items = append(items, legendItem{Label: b.t("legend." + t), Tone: t, Sign: toneSign[t]})
	}
	return block{Kind: "legend", Span: 12, Title: b.t("legend.title"), Legend: items}
}

// hasComparison reports whether at least one headline figure has a
// comparison value (the colors then mean something).
func (b *builder) hasComparison() bool {
	for _, st := range b.an.KPIs {
		if st.HasDelta {
			return true
		}
	}
	return b.an.Progress != nil
}

// expectedLine explains, once under the key indicators, the figures whose
// decline is expected (dilution): "Classement moyen et taux de clic : normal
// — …". The cards themselves only carry a short "Normal" pill.
func (b *builder) expectedLine() string {
	var labels []string
	for _, k := range []struct{ key, label string }{
		{insights.KeyGSCPosition, "position"}, {insights.KeyGSCCTR, "ctr"},
		{insights.KeyBingPosition, "bing_position"}, {insights.KeyBingCTR, "bing_ctr"},
	} {
		if b.an.Expected(k.key) {
			l := b.plainLabel(k.label)
			if len(labels) > 0 {
				l = lowerFirst(l)
			}
			labels = append(labels, l)
		}
	}
	if len(labels) == 0 {
		return ""
	}
	list := labels[0]
	if n := len(labels); n > 1 {
		list = strings.Join(labels[:n-1], ", ") + b.t("list.and") + labels[n-1]
	}
	return b.tf("kpi.expected_line", list)
}

// lowerFirst lower-cases the first letter of a label used inside a sentence
// ("Taux de clic" -> "taux de clic"), but keeps acronyms and brand names
// ("CTR Bing").
func lowerFirst(s string) string {
	if f := strings.Fields(s); len(f) > 0 && (f[0] == "Bing" || f[0] == "Google") {
		return s
	}
	r := []rune(s)
	if len(r) > 1 && unicode.IsUpper(r[0]) && unicode.IsLower(r[1]) {
		r[0] = unicode.ToLower(r[0])
	}
	return string(r)
}

// ---------------------------------------------------------------------------
// Scorecard

// scoreRow is one line of the scorecard: one channel, one figure, one arrow,
// a few plain words.
type scoreRow struct {
	ID     string
	Name   string
	Icon   template.HTML
	Value  string
	Unit   string
	Change string
	Tone   string // pos, neg, neu, exp
	Sign   string
	Status string
	Title  string
}

// scoreChannel builds the scorecard row of one channel; ok is false when the
// report has no data for it.
type scoreChannel func(b *builder) (row scoreRow, ok bool)

// scoreChannels are the scorecard rows, in display order. A new data source
// adds its row with registerScoreChannel (one call, see init below).
var scoreChannels []scoreChannel

func registerScoreChannel(fn scoreChannel) { scoreChannels = append(scoreChannels, fn) }

func init() {
	registerScoreChannel(scoreGoogle)
	registerScoreChannel(scoreSite)
	registerScoreChannel(scoreAI)
	registerScoreChannel(scoreBing)
	registerScoreChannel(scoreShopping)
	registerScoreChannel(scoreBusinessProfile)
}

// scoreFor fills a scorecard row from a metric. id selects the texts
// (score.<id>.name, score.<id>.up / down / stable / none / new), unit the
// plural key of the figure ("unit.visits").
func (b *builder) scoreFor(id, key string, m *model.Metric, unit string, icon template.HTML) scoreRow {
	r := scoreRow{ID: id, Name: b.t("score." + id + ".name"), Icon: icon, Value: b.num(m.Current)}
	r.Unit = unitWord(b.plural(m.Current, unit), r.Value)
	r.Tone = "neu"
	state := "none"
	if m.Previous != nil {
		if c, ok := b.humanChange(m.Current, *m.Previous, b.compactNum); ok {
			r.Change = c.Main
			r.Title = b.changeTitle(c, *m, b.compactNum)
			r.Tone = b.kpiTone(key, m, false)
			switch {
			case c.Kind == "new":
				state = "new"
			case r.Tone == "pos":
				state = "up"
			case r.Tone == "neg":
				state = "down"
			default:
				state = "stable"
			}
		} else {
			state = "stable"
		}
	}
	r.Sign = toneSign[r.Tone]
	r.Status = b.t("score." + id + "." + state)
	return r
}

// unitWord strips the number from a pluralized "%s visites" text.
func unitWord(s, num string) string {
	for _, p := range []string{num + " ", num + i18n.NBSP} {
		if len(s) > len(p) && s[:len(p)] == p {
			return s[len(p):]
		}
	}
	return s
}

func scoreGoogle(b *builder) (scoreRow, bool) {
	sc := b.r.SearchConsole
	if sc == nil || (sc.Totals.Clicks.Current == 0 && sc.Totals.Clicks.Previous == nil) {
		return scoreRow{}, false
	}
	return b.scoreFor("google", insights.KeyGSCClicks, &sc.Totals.Clicks, "unit.visits", icoSearch), true
}

func scoreSite(b *builder) (scoreRow, bool) {
	a := b.r.Analytics
	if a == nil {
		return scoreRow{}, false
	}
	if m := a.Totals.Sessions; m != nil {
		return b.scoreFor("site", insights.KeyGA4Sessions, m, "unit.visits", icoHome), true
	}
	if a.OrganicTotals != nil && a.OrganicTotals.Sessions != nil {
		return b.scoreFor("site", insights.KeyGA4Organic, a.OrganicTotals.Sessions, "unit.visits", icoHome), true
	}
	return scoreRow{}, false
}

func scoreAI(b *builder) (scoreRow, bool) {
	if b.r.SectionHidden("geo") {
		return scoreRow{}, false
	}
	if m := b.aiSessions(); m != nil && (m.Current > 0 || m.Previous != nil) {
		return b.scoreFor("ai", insights.KeyGEOAISessions, m, "unit.visits", icoAI), true
	}
	if cr := b.citationRate(); cr != nil {
		r := scoreRow{ID: "ai", Name: b.t("score.ai.name"), Icon: icoAI, Value: b.pct0(cr.Current), Unit: b.t("unit.cited"), Tone: "neu"}
		state := "cited"
		if d := b.deltaPts(*cr); d != nil {
			r.Change, r.Tone = d.Text, b.kpiTone(insights.KeyGEOCitationRate, cr, false)
		}
		r.Sign = toneSign[r.Tone]
		r.Status = b.t("score.ai." + state)
		return r, true
	}
	return scoreRow{}, false
}

func scoreBing(b *builder) (scoreRow, bool) {
	bg := b.r.Bing
	if bg == nil || b.r.SectionHidden("bing") || (bg.Totals.Clicks.Current == 0 && bg.Totals.Clicks.Previous == nil) {
		return scoreRow{}, false
	}
	return b.scoreFor("bing", insights.KeyBingClicks, &bg.Totals.Clicks, "unit.visits", icoCompass), true
}

func scoreShopping(b *builder) (scoreRow, bool) {
	mc := b.r.MerchantCenter
	if mc == nil || b.r.SectionHidden("merchant_center") || !mc.HasPerformance() {
		return scoreRow{}, false
	}
	return b.scoreFor("shopping", insights.KeyGMCClicks, &mc.Totals.Clicks, "unit.clicks", icoBag), true
}

// scoreBusinessProfile is the "Google Business Profile" row: profile views,
// else interactions (calls, website clicks, directions...).
func scoreBusinessProfile(b *builder) (scoreRow, bool) {
	bp := b.r.BusinessProfile
	if bp == nil || b.r.SectionHidden("business_profile") {
		return scoreRow{}, false
	}
	if m := bp.Totals.Views; m != nil && (m.Current > 0 || m.Previous != nil) {
		return b.scoreFor("gbp", "gbp.views", m, "unit.views", icoPin), true
	}
	if m := bp.Totals.Interactions; m != nil && (m.Current > 0 || m.Previous != nil) {
		return b.scoreFor("gbp_actions", "gbp.interactions", m, "unit.actions", icoPin), true
	}
	return scoreRow{}, false
}

// scorecard is the "channel by channel" block of part 1.
func (b *builder) scorecard() block {
	var rows []scoreRow
	for _, fn := range scoreChannels {
		if r, ok := fn(b); ok {
			rows = append(rows, r)
		}
	}
	if len(rows) == 0 {
		return block{}
	}
	return block{Kind: "scorecard", Span: 12, Title: b.t("score.title"), Sub: b.t("score.sub"), Score: rows}
}

// ---------------------------------------------------------------------------
// Wins and points to watch

// tile is one big-number tile ("×3,8 — Affichages dans Google").
type tile struct {
	Kicker string
	Icon   template.HTML
	Big    string
	Label  string
	Text   string
	Tone   string
	Sign   string
	Title  string
}

// tileCandidate is a KPI change that may become a tile.
type tileCandidate struct {
	key    string
	label  string // plain label key: kpi.<label>.plain, else .label
	m      model.Metric
	abs    func(float64) string
	weight float64
	kind   string // "count", "rate" (points) or "position"
}

func (b *builder) tileCandidates() []tileCandidate {
	var out []tileCandidate
	addM := func(key, label string, m *model.Metric, abs func(float64) string, w float64, kind string) {
		if m != nil && m.Previous != nil {
			out = append(out, tileCandidate{key: key, label: label, m: *m, abs: abs, weight: w, kind: kind})
		}
	}
	if sc := b.r.SearchConsole; sc != nil {
		addM(insights.KeyGSCClicks, "clicks", &sc.Totals.Clicks, b.num, 1, "count")
		addM(insights.KeyGSCImpressions, "impressions", &sc.Totals.Impressions, b.num, 0.75, "count")
		addM(insights.KeyGSCPosition, "position", sc.Totals.Position, b.pos, 0.6, "position")
		if ctr := sc.Totals.CTR; ctr != nil {
			addM(insights.KeyGSCCTR, "ctr", ctr, func(v float64) string { return i18nPct2(b, v) }, 0.4, "rate")
		}
		if ix := sc.Indexing; ix != nil {
			addM(insights.KeyGSCIndexed, "indexed", &ix.Indexed, b.num, 0.6, "count")
		}
	}
	if a := b.r.Analytics; a != nil {
		if a.OrganicTotals != nil {
			addM(insights.KeyGA4Organic, "organic_sessions", a.OrganicTotals.Sessions, b.num, 0.9, "count")
		}
		addM(insights.KeyGA4Sessions, "sessions", a.Totals.Sessions, b.num, 0.7, "count")
		addM(insights.KeyGA4KeyEvents, "key_events", a.Totals.KeyEvents, b.num, 1, "count")
		addM(insights.KeyGA4Revenue, "revenue", a.Totals.Revenue, b.money, 1, "count")
		addM(insights.KeyGA4EngagementRate, "engagement_rate", a.Totals.EngagementRate, b.pct0, 0.45, "rate")
	}
	if !b.r.SectionHidden("geo") {
		addM(insights.KeyGEOAISessions, "ai_sessions", b.aiSessions(), b.num, 0.8, "count")
		addM(insights.KeyGEOCitationRate, "citation_rate", b.citationRate(), b.pct0, 0.6, "rate")
	}
	if bg := b.r.Bing; bg != nil && !b.r.SectionHidden("bing") {
		addM(insights.KeyBingClicks, "bing_clicks", &bg.Totals.Clicks, b.num, 0.5, "count")
		if ai := bg.AIPerformance; ai != nil {
			addM(insights.KeyBingAICitations, "bing_ai_citations", &ai.Citations, b.num, 0.5, "count")
		}
	}
	if mc := b.r.MerchantCenter; mc != nil && mc.HasPerformance() && !b.r.SectionHidden("merchant_center") {
		addM(insights.KeyGMCClicks, "gmc_clicks", &mc.Totals.Clicks, b.num, 0.6, "count")
	}
	if bp := b.r.BusinessProfile; bp != nil && !b.r.SectionHidden("business_profile") {
		addM("gbp.views", "gbp_views", bp.Totals.Views, b.num, 0.6, "count")
	}
	return out
}

// tileScore rates a candidate: the size of the change, squashed so that huge
// percentages do not crowd out everything, times the importance of the
// figure. Tiny bases weigh less (6 visits instead of 1 is not a trend).
func tileScore(c tileCandidate) float64 {
	prev, cur := *c.m.Previous, c.m.Current
	var rel float64
	switch c.kind {
	case "position":
		if prev <= 0 || cur <= 0 {
			return 0
		}
		rel = (prev - cur) / prev * 100
	case "rate":
		rel = (cur - prev) * 3 // one point of rate weighs like 3 % of a count
	default:
		if prev == 0 {
			rel = 100
		} else {
			rel = (cur - prev) / math.Abs(prev) * 100
		}
	}
	s := c.weight * math.Tanh(math.Abs(rel)/60)
	if c.kind == "count" && math.Abs(prev) < insights.SmallBase {
		s *= 0.6
	}
	return s
}

// tiles returns the strongest positive changes (at most 3) and the clearest
// declines to watch (at most 2), as big-number tiles.
func (b *builder) tiles() block {
	type scored struct {
		c    tileCandidate
		s    float64
		tone string
	}
	var wins, watch []scored
	for _, c := range b.tileCandidates() {
		lower := c.kind == "position"
		tone := b.kpiTone(c.key, &c.m, lower)
		d := c.m.Current - *c.m.Previous
		// A change of one or two units is not a story to tell.
		if c.kind == "count" && math.Abs(d) < 3 {
			continue
		}
		sc := scored{c: c, s: tileScore(c), tone: tone}
		switch tone {
		case "pos":
			wins = append(wins, sc)
		case "neg":
			watch = append(watch, sc)
		}
	}
	byScore := func(l []scored) {
		sort.SliceStable(l, func(i, j int) bool { return l[i].s > l[j].s })
	}
	byScore(wins)
	byScore(watch)
	wins, watch = topN(wins, 3), topN(watch, 2)
	if len(wins)+len(watch) == 0 {
		return block{}
	}
	var out []tile
	for _, l := range [][]scored{wins, watch} {
		for _, w := range l {
			out = append(out, b.tileOf(w.c, w.tone))
		}
	}
	return block{Kind: "tiles", Span: 12, Title: b.t("tiles.title"), Tiles: out}
}

func (b *builder) tileOf(c tileCandidate, tone string) tile {
	t := tile{Icon: kpiIcon(c.key), Tone: tone, Sign: toneSign[tone], Label: b.plainLabel(c.label)}
	if tone == "pos" {
		t.Kicker = b.t("tiles.win")
	} else {
		t.Kicker = b.t("tiles.watch")
	}
	prev, cur := *c.m.Previous, c.m.Current
	switch c.kind {
	case "position":
		gain := prev - cur
		t.Big = b.tf("tiles.places", i18n.SignedDecimal(b.lang, gain, 1))
	case "rate":
		d, digits := cur-prev, 0
		switch {
		case math.Abs(d) < 0.95:
			digits = 2
			if math.Abs(d) >= 0.095 {
				digits = 1
			}
		}
		t.Big = i18n.Points(b.lang, d, digits)
	default:
		if ch, ok := b.humanChange(cur, prev, c.abs); ok {
			t.Big = ch.Main
			t.Title = b.changeTitle(ch, c.m, c.abs)
		}
	}
	t.Text = b.tf("tiles.vs", c.abs(cur), c.abs(prev))
	return t
}

// plainLabel is the beginner label of a KPI (kpi.<k>.plain), else its
// regular label.
func (b *builder) plainLabel(k string) string {
	if i18n.Has("kpi." + k + ".plain") {
		return b.t("kpi." + k + ".plain")
	}
	return b.t("kpi." + k + ".label")
}

// ---------------------------------------------------------------------------
// Highlights: a few in part 1, the full lists in part 2

// Part 1 keeps a handful of highlights; the complete lists move to the
// "review" section at the start of part 2.
const (
	maxGlanceHighlights = 4
	maxGlanceConcerns   = 2
)

// idReview is the part 2 section holding the complete highlights and
// concerns lists. It follows the hide_sections setting of "summary".
const idReview = "review"

// glanceLists returns the highlight / concern lists of part 1.
func (b *builder) glanceLists() []block {
	n := b.r.Narrative
	if n == nil {
		return nil
	}
	hi, co := nonEmptyItems(n.Highlights), nonEmptyItems(n.Concerns)
	maxHi := maxGlanceHighlights
	if len(co) > 0 {
		maxHi = maxGlanceHighlights - 1
	}
	var out []block
	span := 6
	if len(hi) == 0 || len(co) == 0 {
		span = 12
	}
	more := len(hi) > maxHi || len(co) > maxGlanceConcerns
	if len(hi) > 0 {
		bl := block{Kind: "list", Span: span, Tone: "pos", Title: b.t("summary.highlights"), Items: mdItems(topN(hi, maxHi))}
		if len(hi) > maxHi {
			bl.Sub = b.plural(float64(len(hi)-maxHi), "summary.more")
		}
		out = append(out, bl)
	}
	if len(co) > 0 {
		bl := block{Kind: "list", Span: span, Tone: "neg", Title: b.t("summary.concerns"), Items: mdItems(topN(co, maxGlanceConcerns))}
		if len(co) > maxGlanceConcerns {
			bl.Sub = b.plural(float64(len(co)-maxGlanceConcerns), "summary.more")
		}
		out = append(out, bl)
	}
	if more && !b.r.SectionHidden("summary") {
		for i := range out {
			if out[i].Sub != "" {
				out[i].Href = "#sec-" + idReview
			}
		}
	}
	return out
}

func nonEmptyItems(items []string) []string {
	var out []string
	for _, it := range items {
		if it != "" {
			out = append(out, it)
		}
	}
	return out
}

// review is the first section of part 2: the complete highlights and
// concerns, when part 1 shows only some of them. (The automatic insights are
// shown in the part 2 sections they relate to.)
func (b *builder) review() *section {
	if b.r.SectionHidden("summary") {
		return nil
	}
	s := &section{ID: idReview}
	if n := b.r.Narrative; n != nil {
		hi, co := nonEmptyItems(n.Highlights), nonEmptyItems(n.Concerns)
		maxHi := maxGlanceHighlights
		if len(co) > 0 {
			maxHi--
		}
		if len(hi) > maxHi || len(co) > maxGlanceConcerns {
			span := 6
			if len(hi) == 0 || len(co) == 0 {
				span = 12
			}
			if len(hi) > 0 {
				s.Blocks = append(s.Blocks, block{Kind: "list", Span: span, Tone: "pos", Title: b.t("summary.highlights"), Items: mdItems(hi)})
			}
			if len(co) > 0 {
				s.Blocks = append(s.Blocks, block{Kind: "list", Span: span, Tone: "neg", Title: b.t("summary.concerns"), Items: mdItems(co)})
			}
		}
	}
	if len(s.Blocks) == 0 {
		return nil
	}
	return s
}
