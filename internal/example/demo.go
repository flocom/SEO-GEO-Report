package example

import (
	"bytes"
	"encoding/json"
	"math"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// Fixed identity and periods of the demo: a fictional French lighting
// e-commerce site, Q3 2026 compared with the previous 90 days.
const (
	SiteName  = "Maison Lumen"
	SiteURL   = "https://www.maison-lumen.fr"
	periodDay = 90

	curStart  = "2026-07-01"
	curEnd    = "2026-09-28"
	prevStart = "2026-04-02"
	prevEnd   = "2026-06-30"

	seed1, seed2 = 2026, 0x4c756d656e // "Lumen"
)

func demo(lang string) *model.Report {
	lang = normLang(lang)
	rng := rand.New(rand.NewPCG(seed1, seed2))

	sc := searchConsole(rng, lang)
	ga := analytics(rng, sc)
	r := &model.Report{
		Meta:          meta(lang),
		SearchConsole: sc,
		Analytics:     ga,
		GEO:           geo(lang, ga),
	}
	r.Narrative = narrative(lang, r)
	r.Sections = sections(lang)
	return r
}

// DemoJSON returns Demo(lang) as indented JSON (two spaces, no HTML
// escaping, trailing newline): the exact content of examples/demo-<lang>.json.
func DemoJSON(lang string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(Demo(lang)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func normLang(l string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(l)), "en") {
		return "en"
	}
	return "fr"
}

func tr(lang, fr, en string) string {
	if lang == "en" {
		return en
	}
	return fr
}

func meta(lang string) model.Meta {
	return model.Meta{
		Title:            tr(lang, "Rapport de visibilité SEO & GEO — T3 2026", "SEO & GEO visibility report — Q3 2026"),
		SiteName:         SiteName,
		SiteURL:          SiteURL,
		Language:         lang,
		Period:           model.Period{Start: curStart, End: curEnd},
		ComparisonPeriod: &model.Period{Start: prevStart, End: prevEnd},
		ComparisonLabel:  tr(lang, "vs période précédente (2 avril – 30 juin 2026)", "vs previous period (Apr 2 – Jun 30, 2026)"),
		PreparedBy:       "Studio Halo",
		PreparedFor:      SiteName,
		BrandColor:       "#b45309",
		Currency:         "EUR",
	}
}

// ---------------------------------------------------------------------------
// Helpers

// weekly is the day-of-week seasonality of an e-commerce lighting site:
// strong at the start of the week, weaker on Friday and Saturday.
var weekly = map[time.Weekday]float64{
	time.Monday: 1.08, time.Tuesday: 1.10, time.Wednesday: 1.06, time.Thursday: 1.02,
	time.Friday: 0.93, time.Saturday: 0.84, time.Sunday: 0.97,
}

func days(start string, n int) []time.Time {
	t0, err := time.Parse("2006-01-02", start)
	if err != nil {
		panic(err)
	}
	out := make([]time.Time, n)
	for i := range out {
		out[i] = t0.AddDate(0, 0, i)
	}
	return out
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func noise(rng *rand.Rand, amp float64) float64 { return amp * (2*rng.Float64() - 1) }

func round(v float64, d int) float64 {
	p := math.Pow(10, float64(d))
	return math.Round(v*p) / p
}

func f(v float64) *float64 { return &v }

func b(v bool) *bool { return &v }

func ip(v int) *int { return &v }

// split distributes total into integer parts proportional to shares; the
// last part takes the rounding remainder.
func split(total float64, shares []float64) []float64 {
	out := make([]float64, len(shares))
	var sum float64
	for i, s := range shares {
		if i == len(shares)-1 {
			out[i] = total - sum
			break
		}
		out[i] = math.Round(total * s)
		sum += out[i]
	}
	return out
}

// ---------------------------------------------------------------------------
// Search Console

type seriesSpec struct {
	start              string
	clickFrom, clickTo float64
	ctrFrom, ctrTo     float64 // percent
	posFrom, posTo     float64
}

func gscSeries(rng *rand.Rand, s seriesSpec) []model.GSCDailyPoint {
	ds := days(s.start, periodDay)
	out := make([]model.GSCDailyPoint, len(ds))
	for i, d := range ds {
		t := float64(i) / float64(len(ds)-1)
		clicks := math.Round(lerp(s.clickFrom, s.clickTo, t) * weekly[d.Weekday()] * (1 + noise(rng, 0.06)))
		impr := math.Round(clicks / (lerp(s.ctrFrom, s.ctrTo, t) / 100) * (1 + noise(rng, 0.04)))
		pos := round(lerp(s.posFrom, s.posTo, t)+noise(rng, 0.35), 1)
		out[i] = model.GSCDailyPoint{
			Date:        d.Format("2006-01-02"),
			Clicks:      clicks,
			Impressions: impr,
			CTR:         round(clicks/impr*100, 2),
			Position:    pos,
		}
	}
	return out
}

func gscTotals(pts []model.GSCDailyPoint) (clicks, impr, ctr, pos float64) {
	var wpos float64
	for _, p := range pts {
		clicks += p.Clicks
		impr += p.Impressions
		wpos += p.Position * p.Impressions
	}
	return clicks, impr, round(clicks/impr*100, 2), round(wpos/impr, 1)
}

type queryData struct {
	key                string
	clicks, prevClicks float64
	impr, prevImpr     float64
	pos, prevPos       float64
}

var demoQueries = []queryData{
	{"maison lumen", 6200, 5400, 14800, 13100, 1.1, 1.2},
	{"maison lumen avis", 980, 720, 3900, 3100, 1.4, 1.6},
	{"suspension rotin", 1850, 1120, 52000, 41000, 2.8, 4.6},
	{"lampe de chevet tactile", 1420, 610, 38500, 22000, 3.0, 6.2},
	{"luminaire salon", 1180, 940, 96000, 88000, 6.4, 7.8},
	{"applique murale design", 960, 700, 41000, 36000, 5.1, 6.0},
	{"suspension cuisine", 870, 520, 47000, 39000, 7.2, 11.4},
	{"lampadaire arc", 760, 690, 28000, 27500, 4.2, 4.3},
	{"lampe à poser laiton", 640, 310, 15200, 9800, 2.2, 3.9},
	{"luminaire scandinave", 590, 640, 33000, 34000, 8.9, 8.1},
	{"ampoule filament e27", 540, 580, 61000, 58000, 9.8, 9.2},
	{"suspension verre", 510, 280, 26000, 17000, 6.8, 12.3},
	{"spot led encastrable", 430, 470, 52000, 50000, 14.6, 13.9},
	{"lampe champignon", 420, 150, 19800, 8800, 4.8, 9.6},
	{"luminaire extérieur", 390, 220, 44000, 30000, 11.2, 16.8},
	{"plafonnier chambre", 360, 300, 37000, 33500, 9.4, 10.6},
	{"lampe de bureau vintage", 330, 410, 21000, 22500, 7.7, 6.4},
	{"guirlande guinguette", 280, 90, 18000, 6200, 5.9, 14.2},
	{"variateur lumière", 210, 240, 26500, 27000, 16.4, 15.8},
	{"comment choisir une suspension", 190, 40, 12800, 3900, 2.9, 8.8},
}

var demoPages = []queryData{
	{SiteURL + "/", 7400, 6300, 21500, 19200, 1.6, 1.8},
	{SiteURL + "/suspensions/rotin", 2900, 1700, 88000, 66000, 3.9, 5.6},
	{SiteURL + "/lampes-a-poser/chevet-tactile", 2100, 900, 61000, 37000, 3.6, 6.8},
	{SiteURL + "/luminaires/salon", 1800, 1450, 142000, 128000, 6.9, 8.1},
	{SiteURL + "/appliques-murales", 1500, 1150, 70000, 61000, 5.8, 6.7},
	{SiteURL + "/suspensions/cuisine", 1350, 820, 76000, 60000, 7.4, 11.0},
	{SiteURL + "/lampadaires", 1200, 1100, 51000, 49500, 5.2, 5.4},
	{SiteURL + "/blog/comment-choisir-une-suspension", 1100, 380, 64000, 24000, 4.4, 9.1},
	{SiteURL + "/lampes-a-poser/laiton", 980, 520, 26000, 17500, 3.1, 4.8},
	{SiteURL + "/ampoules/filament", 900, 950, 98000, 93000, 10.1, 9.5},
	{SiteURL + "/suspensions/verre", 820, 450, 41000, 27000, 7.0, 11.9},
	{SiteURL + "/luminaires-exterieurs", 700, 430, 69000, 48000, 11.6, 16.2},
	{SiteURL + "/blog/guide-temperature-couleur", 640, 290, 47000, 21000, 6.1, 10.4},
	{SiteURL + "/spots-encastrables", 560, 610, 83000, 80000, 14.2, 13.6},
	{SiteURL + "/plafonniers", 520, 470, 58000, 54000, 9.8, 10.9},
}

func gscRows(data []queryData) []model.GSCRow {
	out := make([]model.GSCRow, len(data))
	for i, q := range data {
		out[i] = model.GSCRow{
			Key:             q.key,
			Clicks:          q.clicks,
			Impressions:     q.impr,
			CTR:             round(q.clicks/q.impr*100, 2),
			Position:        q.pos,
			PrevClicks:      f(q.prevClicks),
			PrevImpressions: f(q.prevImpr),
			PrevPosition:    f(q.prevPos),
		}
	}
	return out
}

// shareRows builds dimension rows (countries, devices...) whose clicks and
// impressions split the totals.
func shareRows(keys []string, clicks, prevClicks, impr, prevImpr float64, cur, prev, imprShares, prevImprShares, pos, prevPos []float64) []model.GSCRow {
	c, pc := split(clicks, cur), split(prevClicks, prev)
	im, pim := split(impr, imprShares), split(prevImpr, prevImprShares)
	out := make([]model.GSCRow, len(keys))
	for i, k := range keys {
		out[i] = model.GSCRow{
			Key:             k,
			Clicks:          c[i],
			Impressions:     im[i],
			CTR:             round(c[i]/im[i]*100, 2),
			Position:        pos[i],
			PrevClicks:      f(pc[i]),
			PrevImpressions: f(pim[i]),
			PrevPosition:    f(prevPos[i]),
		}
	}
	return out
}

func searchConsole(rng *rand.Rand, lang string) *model.SearchConsole {
	daily := gscSeries(rng, seriesSpec{curStart, 470, 560, 2.95, 2.80, 13.6, 12.4})
	prevDaily := gscSeries(rng, seriesSpec{prevStart, 360, 400, 3.10, 3.00, 15.2, 14.4})
	c, im, ctr, pos := gscTotals(daily)
	pc, pim, pctr, ppos := gscTotals(prevDaily)

	sc := &model.SearchConsole{
		Totals: model.GSCTotals{
			Clicks:      model.M(c, pc),
			Impressions: model.M(im, pim),
			CTR:         &model.Metric{Current: ctr, Previous: f(pctr)},
			Position:    &model.Metric{Current: pos, Previous: f(ppos)},
		},
		Daily:         daily,
		PreviousDaily: prevDaily,
		Queries:       gscRows(demoQueries),
		Pages:         gscRows(demoPages),
	}

	sc.Countries = shareRows(
		[]string{"France", tr(lang, "Belgique", "Belgium"), tr(lang, "Suisse", "Switzerland"), "Canada", "Luxembourg", tr(lang, "Maroc", "Morocco")},
		c, pc, im, pim,
		[]float64{0.858, 0.064, 0.036, 0.021, 0.011, 0.010},
		[]float64{0.866, 0.060, 0.034, 0.019, 0.011, 0.010},
		[]float64{0.842, 0.071, 0.039, 0.026, 0.011, 0.011},
		[]float64{0.850, 0.067, 0.037, 0.024, 0.011, 0.011},
		[]float64{11.9, 13.8, 15.1, 18.4, 14.2, 21.5},
		[]float64{14.0, 15.6, 16.8, 20.1, 15.9, 23.0},
	)
	sc.Devices = shareRows(
		[]string{"Mobile", tr(lang, "Ordinateur", "Desktop"), tr(lang, "Tablette", "Tablet")},
		c, pc, im, pim,
		[]float64{0.632, 0.329, 0.039},
		[]float64{0.601, 0.358, 0.041},
		[]float64{0.668, 0.296, 0.036},
		[]float64{0.642, 0.320, 0.038},
		[]float64{12.9, 11.6, 13.4},
		[]float64{14.9, 13.7, 15.2},
	)
	sc.SearchAppearance = []model.GSCRow{
		{Key: tr(lang, "Extraits de produits", "Product snippets"), Clicks: 14200, Impressions: 402000, Position: 8.1, PrevClicks: f(9800), PrevImpressions: f(281000), PrevPosition: f(9.4)},
		{Key: tr(lang, "Fiches marchands", "Merchant listings"), Clicks: 6900, Impressions: 251000, Position: 9.6, PrevClicks: f(4100), PrevImpressions: f(168000), PrevPosition: f(11.2)},
		{Key: tr(lang, "Extraits d'avis", "Review snippets"), Clicks: 2300, Impressions: 64000, Position: 5.4, PrevClicks: f(1900), PrevImpressions: f(57000), PrevPosition: f(5.9)},
		{Key: tr(lang, "Vidéos", "Videos"), Clicks: 610, Impressions: 38000, Position: 7.8, PrevClicks: f(240), PrevImpressions: f(15000), PrevPosition: f(9.9)},
	}
	for i := range sc.SearchAppearance {
		r := &sc.SearchAppearance[i]
		r.CTR = round(r.Clicks/r.Impressions*100, 2)
	}

	branded := model.M(9850, 8640)
	sc.BrandSplit = &model.BrandSplit{
		Branded:    branded,
		NonBranded: model.M(c-branded.Current, pc-*branded.Previous),
		BrandTerms: []string{"maison lumen", "lumen"},
	}
	sc.PositionDistribution = &model.PositionDistribution{
		Top3:     model.M(148, 102),
		Top10:    model.M(412, 318),
		Top20:    model.M(690, 605),
		Beyond20: model.M(2350, 2180),
	}
	sc.Indexing = &model.Indexing{
		Indexed:    model.M(1840, 1610),
		NotIndexed: &model.Metric{Current: 420, Previous: f(510)},
		Issues: []model.IndexingIssue{
			{Reason: tr(lang, "Explorée, actuellement non indexée", "Crawled - currently not indexed"), Pages: 180},
			{Reason: tr(lang, "Page en double sans URL canonique sélectionnée par l'utilisateur", "Duplicate without user-selected canonical"), Pages: 95},
			{Reason: tr(lang, "Page avec redirection", "Page with redirect"), Pages: 70},
			{Reason: tr(lang, "Introuvable (404)", "Not found (404)"), Pages: 45},
			{Reason: tr(lang, "Exclue par la balise « noindex »", "Excluded by ‘noindex’ tag"), Pages: 30},
		},
		Sitemaps: []model.Sitemap{
			{URL: SiteURL + "/sitemap_products.xml", Discovered: 1460, Status: tr(lang, "Opération effectuée", "Success")},
			{URL: SiteURL + "/sitemap_collections.xml", Discovered: 185, Status: tr(lang, "Opération effectuée", "Success")},
			{URL: SiteURL + "/sitemap_blog.xml", Discovered: 92, Status: tr(lang, "Opération effectuée", "Success")},
		},
	}
	sc.CoreWebVitals = &model.CoreWebVitals{
		Mobile:  &model.CWVStatus{Good: 1210, NeedsImprovement: 380, Poor: 250},
		Desktop: &model.CWVStatus{Good: 1640, NeedsImprovement: 150, Poor: 50},
	}
	return sc
}

// ---------------------------------------------------------------------------
// Google Analytics 4

type gaSpec struct {
	aiFrom, aiTo       float64
	otherFrom, otherTo float64
}

func gaSeries(rng *rand.Rand, gsc []model.GSCDailyPoint, s gaSpec) []model.GA4DailyPoint {
	out := make([]model.GA4DailyPoint, len(gsc))
	for i, p := range gsc {
		d, _ := time.Parse("2006-01-02", p.Date)
		t := float64(i) / float64(len(gsc)-1)
		organic := math.Round(p.Clicks * 0.88 * (1 + noise(rng, 0.03)))
		ai := math.Round(lerp(s.aiFrom, s.aiTo, t) * weekly[d.Weekday()] * (1 + noise(rng, 0.18)))
		other := math.Round(lerp(s.otherFrom, s.otherTo, t) * (0.5 + weekly[d.Weekday()]/2) * (1 + noise(rng, 0.05)))
		out[i] = model.GA4DailyPoint{Date: p.Date, Sessions: organic + ai + other, OrganicSessions: organic, AISessions: ai}
	}
	return out
}

func sumDaily(pts []model.GA4DailyPoint) (sessions, organic, ai float64) {
	for _, p := range pts {
		sessions += p.Sessions
		organic += p.OrganicSessions
		ai += p.AISessions
	}
	return
}

func analytics(rng *rand.Rand, sc *model.SearchConsole) *model.Analytics {
	daily := gaSeries(rng, sc.Daily, gaSpec{14, 30, 560, 600})
	prevDaily := gaSeries(rng, sc.PreviousDaily, gaSpec{5, 11, 520, 550})
	s, org, ai := sumDaily(daily)
	ps, porg, pai := sumDaily(prevDaily)

	ga := &model.Analytics{
		Totals: model.GA4Totals{
			Sessions:          &model.Metric{Current: s, Previous: f(ps)},
			Users:             &model.Metric{Current: math.Round(s * 0.78), Previous: f(math.Round(ps * 0.79))},
			NewUsers:          &model.Metric{Current: math.Round(s * 0.61), Previous: f(math.Round(ps * 0.63))},
			EngagedSessions:   &model.Metric{Current: math.Round(s * 0.614), Previous: f(math.Round(ps * 0.589))},
			EngagementRate:    &model.Metric{Current: 61.4, Previous: f(58.9)},
			AvgEngagementTime: &model.Metric{Current: 74, Previous: f(69)},
			PageViews:         &model.Metric{Current: math.Round(s * 3.1), Previous: f(math.Round(ps * 2.9))},
			KeyEvents:         &model.Metric{Current: 2140, Previous: f(1610)},
			Revenue:           &model.Metric{Current: 186400, Previous: f(139800)},
		},
		OrganicTotals: &model.GA4Totals{
			Sessions:       &model.Metric{Current: org, Previous: f(porg)},
			Users:          &model.Metric{Current: math.Round(org * 0.81), Previous: f(math.Round(porg * 0.82))},
			EngagementRate: &model.Metric{Current: 66.2, Previous: f(63.5)},
			KeyEvents:      &model.Metric{Current: 980, Previous: f(690)},
			Revenue:        &model.Metric{Current: 84600, Previous: f(59300)},
		},
		Daily:         daily,
		PreviousDaily: prevDaily,
	}

	// Channels: organic and AI come from the daily series, the rest of the
	// traffic is split with fixed shares.
	other, pother := s-org-ai, ps-porg-pai
	shares := []float64{0.40, 0.24, 0.12, 0.09, 0.09, 0.06} // Direct, Paid Search, Organic Social, Referral (non-AI), Email, Paid Social
	pshares := []float64{0.41, 0.25, 0.11, 0.09, 0.08, 0.06}
	o, po := split(other, shares), split(pother, pshares)
	type ch struct {
		name          string
		cur, prev     float64
		eng, key, rev float64
	}
	chans := []ch{
		{"Organic Search", org, porg, 66.2, 980, 84600},
		{"Direct", o[0], po[0], 63.8, 540, 51200},
		{"Paid Search", o[1], po[1], 55.1, 330, 27900},
		{"Organic Social", o[2], po[2], 48.7, 70, 4800},
		{"Referral", o[3] + ai, po[3] + pai, 64.9, 105, 8100},
		{"Email", o[4], po[4], 69.5, 90, 8300},
		{"Paid Social", o[5], po[5], 41.2, 25, 1500},
	}
	for _, c := range chans {
		ga.Channels = append(ga.Channels, model.ChannelRow{
			Channel: c.name, Sessions: c.cur, PrevSessions: f(c.prev),
			Users: math.Round(c.cur * 0.8), EngagementRate: c.eng, KeyEvents: c.key, Revenue: c.rev,
		})
	}

	// Sources, including AI assistants (detected automatically by insights).
	aiNames := []string{"chatgpt.com", "perplexity.ai", "gemini.google.com", "copilot.microsoft.com", "claude.ai", "chat.mistral.ai"}
	aiCur := split(ai, []float64{0.55, 0.20, 0.11, 0.07, 0.05, 0.02})
	aiPrev := split(pai, []float64{0.60, 0.17, 0.09, 0.10, 0.03, 0.01})
	aiEng := []float64{71.4, 74.2, 68.0, 63.5, 77.9, 70.0}
	aiKey := []float64{38, 16, 7, 4, 4, 1}
	aiRev := []float64{3420, 1480, 610, 350, 390, 80}
	orgSplit, porgSplit := split(org, []float64{0.93, 0.05, 0.02}), split(porg, []float64{0.93, 0.05, 0.02})
	soc, psoc := split(o[2], []float64{0.52, 0.31, 0.17}), split(po[2], []float64{0.50, 0.33, 0.17})
	ref, pref := split(o[3], []float64{0.55, 0.45}), split(po[3], []float64{0.58, 0.42})
	type src struct {
		source, medium string
		cur, prev      float64
		eng, key, rev  float64
	}
	srcs := []src{
		{"google", "organic", orgSplit[0], porgSplit[0], 66.4, 915, 79100},
		{"(direct)", "(none)", o[0], po[0], 63.8, 540, 51200},
		{"google", "cpc", o[1], po[1], 55.1, 330, 27900},
		{"bing", "organic", orgSplit[1], porgSplit[1], 64.0, 48, 4200},
		{"duckduckgo", "organic", orgSplit[2], porgSplit[2], 62.7, 17, 1300},
		{"newsletter", "email", o[4], po[4], 69.5, 90, 8300},
		{"instagram.com", "referral", soc[0], psoc[0], 47.9, 31, 2100},
		{"pinterest.com", "referral", soc[1], psoc[1], 52.3, 29, 2000},
		{"l.facebook.com", "referral", soc[2], psoc[2], 44.1, 10, 700},
		{"facebook", "paid", o[5], po[5], 41.2, 25, 1500},
		{"houzz.fr", "referral", ref[0], pref[0], 61.2, 22, 1900},
		{"marieclaire.fr", "referral", ref[1], pref[1], 58.4, 13, 1100},
	}
	for i, n := range aiNames {
		srcs = append(srcs, src{n, "referral", aiCur[i], aiPrev[i], aiEng[i], aiKey[i], aiRev[i]})
	}
	for _, x := range srcs {
		ga.Sources = append(ga.Sources, model.SourceRow{
			Source: x.source, Medium: x.medium, Sessions: x.cur, PrevSessions: f(x.prev),
			Users: math.Round(x.cur * 0.8), EngagementRate: x.eng, KeyEvents: x.key, Revenue: x.rev,
		})
	}

	// Organic landing pages.
	type lp struct {
		page          string
		cur, prev     float64
		eng, key, rev float64
	}
	for _, x := range []lp{
		{"/", 6620, 5600, 64.1, 210, 18400},
		{"/suspensions/rotin", 2540, 1490, 69.8, 118, 11900},
		{"/lampes-a-poser/chevet-tactile", 1850, 790, 71.2, 96, 7300},
		{"/luminaires/salon", 1590, 1270, 62.4, 64, 6800},
		{"/appliques-murales", 1320, 1010, 65.0, 52, 5200},
		{"/suspensions/cuisine", 1190, 720, 67.3, 49, 5600},
		{"/lampadaires", 1050, 970, 60.8, 38, 4900},
		{"/blog/comment-choisir-une-suspension", 980, 330, 78.5, 21, 1700},
		{"/lampes-a-poser/laiton", 860, 455, 70.1, 41, 3800},
		{"/ampoules/filament", 790, 840, 52.6, 33, 1200},
		{"/blog/guide-temperature-couleur", 560, 250, 81.0, 9, 600},
		{"/luminaires-exterieurs", 610, 380, 63.9, 27, 2900},
	} {
		ga.LandingPages = append(ga.LandingPages, model.LandingPageRow{
			Page: x.page, Sessions: x.cur, PrevSessions: f(x.prev), EngagementRate: x.eng, KeyEvents: x.key, Revenue: x.rev,
		})
	}
	return ga
}

// ---------------------------------------------------------------------------
// GEO

func geo(lang string, ga *model.Analytics) *model.GEO {
	g := &model.GEO{}
	for _, d := range ga.Daily {
		g.AIDaily = append(g.AIDaily, model.DailyValue{Date: d.Date, Value: d.AISessions})
	}
	const checked = "2026-09-25"
	type cc struct {
		prompt, engine   string
		cited, mentioned bool
		pos              int
		prev             bool
		competitors      []string
		noteFR, noteEN   string
	}
	for _, c := range []cc{
		{"Quelle est la meilleure boutique en ligne de luminaires design en France ?", "ChatGPT", true, true, 2, false, []string{"Luminaires Atelier", "Lampes & Cie"},
			"Cité grâce à la nouvelle page « À propos » et aux avis clients.", "Cited thanks to the new About page and customer reviews."},
		{"Quelle est la meilleure boutique en ligne de luminaires design en France ?", "Perplexity", true, true, 1, true, []string{"Luminaires Atelier"}, "", ""},
		{"Quelle suspension choisir pour un îlot de cuisine ?", "Perplexity", true, true, 3, false, []string{"Lampes & Cie"},
			"Le guide « Comment choisir une suspension » est la source citée.", "The “How to choose a pendant light” guide is the cited source."},
		{"Quelle suspension choisir pour un îlot de cuisine ?", "Google AI Overviews", false, false, 0, false, []string{"Luminaires Atelier", "Lampes & Cie"}, "", ""},
		{"Lampe de chevet tactile : quel modèle acheter ?", "ChatGPT", true, true, 1, false, []string{"Nordlicht Design"}, "", ""},
		{"Comment choisir la température de couleur d'une ampoule ?", "Gemini", true, true, 2, false, nil,
			"Cite le guide de la température de couleur publié en juillet.", "Cites the colour temperature guide published in July."},
		{"Comment choisir la température de couleur d'une ampoule ?", "ChatGPT", false, true, 0, false, []string{"Lampes & Cie"},
			"Marque citée sans lien.", "Brand named without a link."},
		{"Suspension en rotin : quelles marques françaises ?", "Copilot", true, true, 2, true, []string{"Luminaires Atelier"}, "", ""},
		{"Avis Maison Lumen", "ChatGPT", true, true, 1, true, nil, "", ""},
		{"Luminaire scandinave pas cher", "Perplexity", false, false, 0, true, []string{"Nordlicht Design", "Lampes & Cie"},
			"Citation perdue : la page catégorie a été fusionnée.", "Citation lost: the category page was merged."},
		{"Quel lampadaire pour un salon cosy ?", "Google AI Mode", false, false, 0, false, []string{"Lampes & Cie"}, "", ""},
		{"Guirlande guinguette extérieure : comment l'installer ?", "Claude", true, true, 2, false, nil, "", ""},
	} {
		chk := model.CitationCheck{
			Prompt: c.prompt, Engine: c.engine, Cited: c.cited, Mentioned: c.mentioned,
			PreviouslyCited: b(c.prev), CompetitorsCited: c.competitors, Date: checked,
			Notes: tr(lang, c.noteFR, c.noteEN),
		}
		if c.pos > 0 {
			chk.Position = ip(c.pos)
		}
		g.Citations = append(g.Citations, chk)
	}
	g.ShareOfVoice = []model.ShareOfVoice{
		{Brand: "Luminaires Atelier", Share: 29, Previous: f(31)},
		{Brand: "Lampes & Cie", Share: 24, Previous: f(23)},
		{Brand: SiteName, Share: 21, Previous: f(13), IsSelf: true},
		{Brand: "Nordlicht Design", Share: 12, Previous: f(14)},
	}
	note := func(fr, en string) string { return tr(lang, fr, en) }
	g.AIOverviews = []model.AIOverviewRow{
		{Query: "luminaire salon", OverviewPresent: true, Cited: true},
		{Query: "suspension cuisine", OverviewPresent: true, Cited: false, Notes: note("Luminaires Atelier cité en premier.", "Luminaires Atelier cited first.")},
		{Query: "comment choisir une suspension", OverviewPresent: true, Cited: true, Notes: note("Notre guide est la première source.", "Our guide is the first source.")},
		{Query: "ampoule filament e27", OverviewPresent: true, Cited: false},
		{Query: "lampe de chevet tactile", OverviewPresent: false, Cited: false},
		{Query: "suspension rotin", OverviewPresent: true, Cited: true},
		{Query: "luminaire extérieur", OverviewPresent: true, Cited: false},
		{Query: "variateur lumière", OverviewPresent: true, Cited: false},
	}
	g.AICrawlers = []model.CrawlerRow{
		{Bot: "GPTBot", Hits: 18400, PrevHits: f(9200)},
		{Bot: "ClaudeBot", Hits: 7300, PrevHits: f(4100)},
		{Bot: "OAI-SearchBot", Hits: 6100, PrevHits: f(1900)},
		{Bot: "PerplexityBot", Hits: 4200, PrevHits: f(1600)},
		{Bot: "ChatGPT-User", Hits: 2300, PrevHits: f(700)},
		{Bot: "Bytespider", Hits: 2900, PrevHits: f(3400)},
		{Bot: "Meta-ExternalAgent", Hits: 1500, PrevHits: f(600)},
	}
	return g
}
