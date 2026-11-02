package render

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/flocom/SEO-GEO-Report/internal/example"
	"github.com/flocom/SEO-GEO-Report/internal/i18n"
	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// smallSiteReport is a fictional small local business one month after the
// launch of its new website: about a hundred Google clicks a month, a few
// clicks a day, impressions ×3.7, a worse average position and CTR (new
// searches ranking lower at first), a handful of visits from ChatGPT and the
// first Copilot citations. Numbers are invented.
func smallSiteReport(lang string) *model.Report {
	cur, prev := days("2026-10-01", 31), days("2026-09-01", 30)
	// Clicks per day: noisy, between 0 and 9.
	pattern := []float64{4, 2, 0, 1, 3, 1, 0, 5, 9, 3, 2, 3, 4, 8, 4, 5, 3, 1, 4, 1, 6, 3, 4, 2, 1, 5, 2, 3, 3, 1, 0}
	prevPattern := []float64{3, 3, 1, 1, 0, 0, 1, 5, 0, 2, 3, 0, 1, 1, 2, 1, 3, 2, 1, 1, 1, 1, 6, 1, 1, 7, 3, 3, 4, 1}
	var gsc, gscPrev []model.GSCDailyPoint
	var clicks, clicksPrev, impr, imprPrev float64
	for i, d := range cur {
		im := 25.0 + float64(i%5)*3
		if i >= 12 {
			im = 110 + float64((i*37)%60)
		}
		gsc = append(gsc, model.GSCDailyPoint{Date: d, Clicks: pattern[i], Impressions: im, CTR: pattern[i] / im * 100, Position: 30 + float64((i*7)%18)})
		clicks += pattern[i]
		impr += im
	}
	for i, d := range prev {
		im := 22.0 + float64(i%6)*2
		gscPrev = append(gscPrev, model.GSCDailyPoint{Date: d, Clicks: prevPattern[i], Impressions: im, CTR: prevPattern[i] / im * 100, Position: 20 + float64((i*5)%22)})
		clicksPrev += prevPattern[i]
		imprPrev += im
	}
	var ga, gaPrev []model.GA4DailyPoint
	for i, d := range cur {
		ga = append(ga, model.GA4DailyPoint{Date: d, Sessions: float64(3 + (i*5)%7), OrganicSessions: float64(2 + (i*3)%4)})
	}
	for i, d := range prev {
		gaPrev = append(gaPrev, model.GA4DailyPoint{Date: d, Sessions: float64(2 + (i*3)%6)})
	}
	r := &model.Report{
		Meta: model.Meta{
			SiteName: "Atelier Brume", SiteURL: "https://atelier-brume.example", Language: lang,
			Period:           model.Period{Start: "2026-10-01", End: "2026-10-31"},
			ComparisonPeriod: &model.Period{Start: "2026-09-01", End: "2026-09-30"},
			PreparedBy:       "Studio Nord", BrandColor: "#0f766e", Currency: "CAD",
		},
		SearchConsole: &model.SearchConsole{
			Totals: model.GSCTotals{
				Clicks: model.M(clicks, clicksPrev), Impressions: model.M(impr, imprPrev),
				CTR:      &model.Metric{Current: math.Round(clicks/impr*1000) / 10, Previous: model.F(math.Round(clicksPrev/imprPrev*1000) / 10)},
				Position: &model.Metric{Current: 38.6, Previous: model.F(28.9)},
			},
			Daily: gsc, PreviousDaily: gscPrev,
			Queries: []model.GSCRow{
				{Key: "atelier céramique sherbrooke", Clicks: 21, Impressions: 140, Position: 3.1, PrevClicks: f(12), PrevPosition: f(4.2)},
				{Key: "cours de poterie", Clicks: 9, Impressions: 410, Position: 14.8, PrevClicks: f(1), PrevPosition: f(31)},
				{Key: "atelier brume", Clicks: 30, Impressions: 64, Position: 1.2, PrevClicks: f(28), PrevPosition: f(1.1)},
				{Key: "tasse artisanale", Clicks: 4, Impressions: 380, Position: 22.5},
				{Key: "cadeau fait main", Clicks: 2, Impressions: 290, Position: 41.7},
			},
			PositionDistribution: &model.PositionDistribution{Top3: model.M(2, 3), Top10: model.M(5, 2), Top20: model.M(3, 1), Beyond20: model.M(15, 4)},
			Indexing:             &model.Indexing{Indexed: model.M(41, 10), NotIndexed: &model.Metric{Current: 6, Previous: f(3)}},
		},
		Analytics: &model.Analytics{
			Totals: model.GA4Totals{
				Sessions: &model.Metric{Current: 174, Previous: f(121)}, Users: &model.Metric{Current: 131, Previous: f(98)},
				EngagementRate: &model.Metric{Current: 60.5, Previous: f(47.8)}, KeyEvents: &model.Metric{Current: 7, Previous: f(4)},
			},
			OrganicTotals: &model.GA4Totals{Sessions: &model.Metric{Current: 101, Previous: f(94)}},
			Daily:         ga, PreviousDaily: gaPrev,
			Channels: []model.ChannelRow{
				{Channel: "Organic Search", Sessions: 101, PrevSessions: f(94)},
				{Channel: "Direct", Sessions: 44, PrevSessions: f(17)},
				{Channel: "Organic Social", Sessions: 15, PrevSessions: f(6)},
				{Channel: "AI Assistant", Sessions: 6, PrevSessions: f(1)},
				{Channel: "Paid Social", Sessions: 5, PrevSessions: f(2)},
				{Channel: "Referral", Sessions: 3, PrevSessions: f(1)},
			},
			Sources: []model.SourceRow{
				{Source: "google", Medium: "organic", Sessions: 97, PrevSessions: f(90)},
				{Source: "(direct)", Medium: "(none)", Sessions: 44, PrevSessions: f(17)},
				{Source: "chatgpt.com", Medium: "referral", Sessions: 6, PrevSessions: f(1)},
			},
		},
		Bing: &model.Bing{
			Totals: model.GSCTotals{Clicks: model.M(9, 4), Impressions: model.M(310, 120)},
			AIPerformance: &model.BingAIPerformance{
				Citations: model.M(4, 0),
			},
		},
		Narrative: smallSiteNarrative(lang),
	}
	return r
}

// smallSiteNarrative is the analyst text of the small-site fixture.
func smallSiteNarrative(lang string) *model.Narrative {
	n := &model.Narrative{
		Actions: []model.Action{
			{Title: "Nouveau site en ligne", Date: "2026-10-08", Status: "done", Category: "seo"},
			{Title: "Pages locales pour chaque ville", Date: "2026-10-20", Status: "done", Category: "content"},
		},
		Recommendations:  []model.Recommendation{{Title: "Publier deux guides par mois", Priority: "high", Effort: "medium"}},
		ExecutiveSummary: "Octobre est le mois du lancement du **nouveau site** (8 octobre) : Google l’a vite adopté, avec des **affichages multipliés par 3,7** et des **clics en hausse de plus de 50 %**.\n\nLes visiteurs sont plus engagés et **ChatGPT envoie ses premières visites**.",
		Highlights: []string{
			"Affichages dans Google multipliés par 3,7 en un mois.",
			"Clics en hausse de plus de 50 %.",
			"41 pages indexées par Google, contre 10.",
			"La page des cours de poterie se classe déjà en page 2.",
			"Taux d’engagement de 60 %, contre 48 %.",
			"Premières visites venues de ChatGPT.",
			"Copilot cite le site 4 fois.",
		},
		Concerns: []string{"Le formulaire de contact reste peu utilisé.", "Peu d’avis sur la fiche Google.", "Quelques pages lentes sur mobile."},
	}
	if lang == "en" {
		n.Actions[0].Title, n.Actions[1].Title = "New website online", "Local pages for each city"
		n.Recommendations[0].Title = "Publish two guides per month"
		n.ExecutiveSummary = "October is the launch month of the **new website** (October 8): Google adopted it quickly, with **appearances multiplied by 3.7** and **clicks up by more than 50%**.\n\nVisitors are more engaged and **ChatGPT sent its first visits**."
		n.Highlights = []string{
			"Appearances in Google multiplied by 3.7 in one month.",
			"Clicks up by more than 50%.",
			"41 pages indexed by Google, versus 10.",
			"The pottery class page already ranks on page 2.",
			"Engagement rate of 60%, versus 48%.",
			"First visits from ChatGPT.",
			"Copilot cites the site 4 times.",
		}
		n.Concerns = []string{"The contact form is still rarely used.", "Few reviews on the Google profile.", "Some pages load slowly on mobile."}
	}
	return n
}

func TestHumanChange(t *testing.T) {
	b := &builder{lang: "fr"}
	cases := []struct {
		cur, prev  float64
		kind, main string
	}{
		{6, 1, "abs", "+5"},
		{3017, 796, "ratio", "×3,8"},
		{102, 63, "pct", "+61,9 %"},
		{44, 11, "ratio", "×4"},
		{47, 1, "abs", "+46"},
		{4700, 100, "ratio", "×47"},
		{4, 0, "new", "Nouveau"},
		{50, 100, "pct", "−50,0\u202f%"},
		{12, 15, "abs", "−3"},
		{150, 50, "ratio", "×3"},
		{149, 50, "pct", "+198 %"},
	}
	for _, c := range cases {
		got, ok := b.humanChange(c.cur, c.prev, nil)
		if !ok || got.Kind != c.kind || got.Main != c.main {
			t.Errorf("humanChange(%v, %v) = %+v, want %s %q", c.cur, c.prev, got, c.kind, c.main)
		}
	}
	if _, ok := b.humanChange(0, 0, nil); ok {
		t.Error("0 → 0 must show nothing")
	}
	en := &builder{lang: "en"}
	if c, _ := en.humanChange(3017, 796, nil); c.Main != "×3.8" || c.Pct != "+279%" {
		t.Errorf("EN ratio: %+v", c)
	}
	// Relative changes never exceed +200 %: beyond, a multiplier is used.
	for prev := 20.0; prev < 400; prev += 7 {
		for cur := 0.0; cur < 5000; cur += 13 {
			c, ok := b.humanChange(cur, prev, nil)
			if ok && c.Kind == "pct" && c.Rel > 200 {
				t.Fatalf("%v → %v shown as %q", prev, cur, c.Main)
			}
		}
	}
}

func TestPer100(t *testing.T) {
	fr, en := &builder{lang: "fr"}, &builder{lang: "en"}
	if got := fr.per100(3.4); got != "Sur 100 affichages dans Google, environ 3 deviennent une visite." {
		t.Errorf("per100 fr = %q", got)
	}
	if got := en.per100(0.42); got != "Out of 1,000 appearances in Google, about 4 turn into a visit." {
		t.Errorf("per100 en = %q", got)
	}
}

func TestWeeklyAggregation(t *testing.T) {
	// 31 days: 7 + 7 + 7 + 10 (a 3-day remainder joins the last week).
	bk := weekBuckets(31)
	if len(bk) != 4 || bk[3].from != 21 || bk[3].to != 30 {
		t.Fatalf("weekBuckets(31) = %+v", bk)
	}
	// 30 days: 7 + 7 + 7 + 7 + 2 -> the 2 days join the last week.
	if bk := weekBuckets(30); len(bk) != 4 || bk[3].to != 29 {
		t.Errorf("weekBuckets(30) = %+v", bk)
	}
	// 90 days: 12 full weeks + a 6-day week of its own.
	if bk := weekBuckets(90); len(bk) != 13 || bk[12].to-bk[12].from != 5 {
		t.Errorf("weekBuckets(90) = %+v", bk[len(bk)-1])
	}
	if weekBuckets(0) != nil || len(weekBuckets(3)) != 1 {
		t.Error("short series")
	}
	vals := make([]float64, 31)
	for i := range vals {
		vals[i] = 1
	}
	sum := aggregateWeekly(vals, weekBuckets(31), aggSum)
	for _, v := range sum {
		if v != 7 { // every point is scaled to 7 days, the 10-day week too
			t.Errorf("weekly sums = %v", sum)
			break
		}
	}
	mean := aggregateWeekly([]float64{2, 4, math.NaN(), 6, 2, 4, 6, 8}, weekBuckets(8), aggMean)
	if len(mean) != 1 || mean[0] != 32.0/7 {
		t.Errorf("weekly mean = %v", mean)
	}
	if v := aggregateWeekly([]float64{math.NaN(), math.NaN()}, weekBuckets(2), aggSum); !math.IsNaN(v[0]) {
		t.Error("an empty week is NaN")
	}
	if !wantWeekly(make([]float64, 31), false) || wantWeekly(make([]float64, 10), false) {
		t.Error("tiny volumes over a month are weekly, under two weeks never")
	}
	big := make([]float64, 90)
	for i := range big {
		big[i] = 100
	}
	if wantWeekly(big, false) || !wantWeekly(big, true) {
		t.Error("long periods are weekly in part 1 only")
	}
	b := &builder{lang: "fr"}
	w := b.toWeekly(days("2026-10-01", 31), vals, vals[:30], aggSum)
	if len(w.labels) != 4 || w.labels[1] != "2026-10-08" || w.ranges["2026-10-22"] != i18n.DateRange("fr", "2026-10-22", "2026-10-31") || len(w.prev) != 4 {
		t.Errorf("toWeekly = %+v", w)
	}
}

// part1 returns the HTML of part 1.
func part1(s string) string {
	p1, p2 := strings.Index(s, `id="part-1"`), strings.Index(s, `id="part-2"`)
	if p1 < 0 || p2 < p1 {
		return ""
	}
	return s[p1:p2]
}

func TestSmallSiteBeginner(t *testing.T) {
	for _, lang := range []string{"fr", "en"} {
		s := mustRender(t, smallSiteReport(lang), Options{})
		p1 := part1(s)
		want := map[string][]string{
			"fr": {"Code couleur", "Canal par canal", "Visites depuis Google", "Clics (Search Console)", "Affichages dans Google",
				"Plus de visites depuis Google", "Votre site", "Assistants IA", "Bonne nouvelle", "×3,7", "Classement moyen, taux de clic et ", "\u00a0: normal —", "</span> Normal<",
				"par semaine", "Recherche naturelle", "Accès direct", "Réseaux sociaux", "autres dans la partie 2"},
			"en": {"Color code", "Channel by channel", "Visits from Google", "Clicks (Search Console)", "Appearances in Google",
				"More visits from Google", "Your website", "AI assistants", "Good news", "×3.7", "Average ranking, click rate and Bing click rate: expected —", "</span> Expected<",
				"per week", "Organic search", "more in part 2"},
		}[lang]
		for _, w := range want {
			if !strings.Contains(p1, w) {
				t.Errorf("%s: part 1 misses %q", lang, w)
			}
		}
		for _, bad := range []string{"+279", "+500", "Organic Search<", "Comment lire ces indicateurs", "How to read these indicators", "kpi.", "score.", "tiles."} {
			if strings.Contains(p1, bad) {
				t.Errorf("%s: part 1 must not contain %q", lang, bad)
			}
		}
		// The legend appears once, at the top of part 1.
		if n := strings.Count(s, `class="legend-bar"`); n != 1 {
			t.Errorf("%s: %d legends", lang, n)
		}
		// The expected decline of position / CTR is amber, never red.
		if !strings.Contains(p1, `class="delta d-exp"`) {
			t.Errorf("%s: position / CTR should use the expected tone", lang)
		}
		// One AI assistant: big-number tiles, no lonely bar chart.
		ai := s[strings.Index(s, `id="sec-ai_glance"`):]
		ai = ai[:strings.Index(ai, "</section>")]
		if !strings.Contains(ai, `class="tiles bal`) || strings.Contains(ai, `class="sgc sgc-bars`) {
			t.Errorf("%s: AI glance should show tiles", lang)
		}
		// Part 1 shows a few highlights; part 2 starts with the full lists.
		last := map[string]string{"fr": "Copilot cite le site 4 fois", "en": "Copilot cites the site 4 times"}[lang]
		if strings.Contains(p1, last) {
			t.Errorf("%s: part 1 should truncate the highlights", lang)
		}
		rev := s[strings.Index(s, `id="sec-review"`):]
		if !strings.Contains(rev[:strings.Index(rev, "</section>")], last) || strings.Index(s, `id="sec-review"`) < strings.Index(s, `id="part-2"`) {
			t.Errorf("%s: the review section should open part 2 with every highlight", lang)
		}
	}
}

func TestDemoBeginner(t *testing.T) {
	s := mustRender(t, example.Demo("fr"), Options{})
	p1 := part1(s)
	for _, w := range []string{"Code couleur", "Canal par canal", "Ce qu’il faut retenir", "Visites depuis Google", "Recherche naturelle"} {
		if !strings.Contains(p1, w) {
			t.Errorf("demo part 1 misses %q", w)
		}
	}
	// No percentage above +200 % in the visible text of the report (hover
	// details keep the percentage next to the multiplier: "×3,1 (+212 %)").
	visible := regexp.MustCompile(`(?s)<style>.*?</style>|title="[^"]*"|\([^()<]*\)`).ReplaceAllString(s, "")
	re := regexp.MustCompile(`\+(\d[\d\x{202f}]*)\x{202f}?%`)
	for _, m := range re.FindAllStringSubmatch(visible, -1) {
		n := strings.ReplaceAll(m[1], " ", "")
		if len(n) >= 4 || (len(n) == 3 && n > "200") {
			t.Errorf("unreadable percentage %q", m[0])
		}
	}
}

// Language switch: no UI text of the other language may leak. User data
// (queries, narrative, page titles) is excluded by checking only strings the
// templates and the i18n catalog produce.
func TestLanguageLeaks(t *testing.T) {
	frOnly := []string{"Comment lire", "Période", "Visites depuis", "Recherche naturelle", "avant :", " contre ", "Accès direct",
		"Code couleur", "Bonne nouvelle", "Canal par canal", "par semaine", "Sommaire", "Partie 1"}
	enOnly := []string{"How to read", "before:", ">Organic Search<", ">Direct<", ">Referral<", ">Organic Social<", "Color code",
		"Good news", "Channel by channel", "per week", "Contents", "Part 1"}
	reports := map[string]func(string) *model.Report{"demo": example.Demo, "small": smallSiteReport, "rich": richReport}
	for name, mk := range reports {
		for _, lang := range []string{"fr", "en"} {
			// The language comes from meta.language or from the render override.
			for _, viaOpt := range []bool{false, true} {
				r := mk(lang)
				opts := Options{}
				if viaOpt {
					r = mk(map[string]string{"fr": "en", "en": "fr"}[lang])
					opts.Lang = lang
				}
				if r.Narrative != nil { // user text is not checked
					r.Narrative = nil
				}
				r.Sections = nil
				s := mustRender(t, r, opts)
				if !strings.Contains(s, `<html lang="`+lang+`">`) {
					t.Errorf("%s/%s: html lang attribute", name, lang)
				}
				leaks := enOnly
				if lang == "en" {
					leaks = frOnly
				}
				for _, l := range leaks {
					if strings.Contains(stripUserData(s), l) {
						t.Errorf("%s/%s (override %v): %q leaks from the other language", name, lang, viaOpt, l)
					}
				}
			}
		}
	}
}

// stripUserData removes table cells and chart labels holding user data
// (queries, URLs, product names) that may legitimately contain any word.
var userCell = regexp.MustCompile(`(?s)<style>.*?</style>|<td class="[^"]*key[^"]*".*?</td>|<title>[^<]*</title>`)

func stripUserData(s string) string { return userCell.ReplaceAllString(s, "") }

// TestFormatsFollowLanguage checks numbers and dates of the small site in
// both languages.
func TestFormatsFollowLanguage(t *testing.T) {
	fr := mustRender(t, smallSiteReport("fr"), Options{})
	en := mustRender(t, smallSiteReport("en"), Options{})
	if !strings.Contains(fr, "1 oct. – 31 oct. 2026") || !strings.Contains(en, "Oct 1 – Oct 31, 2026") {
		t.Error("period dates do not follow the language")
	}
	if !strings.Contains(fr, "60,5 %") || !strings.Contains(en, "60.5%") {
		t.Error("percentages do not follow the language")
	}
}

// TestWriteBeginnerPreview writes the small-site fixture (JSON and HTML) and
// the demo (HTML) when BEGINNER_PREVIEW is set to a directory.
func TestWriteBeginnerPreview(t *testing.T) {
	dir := os.Getenv("BEGINNER_PREVIEW")
	if dir == "" {
		t.Skip("set BEGINNER_PREVIEW to write the small-site fixture")
	}
	for _, lang := range []string{"fr", "en"} {
		data, err := json.MarshalIndent(smallSiteReport(lang), "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "small-"+lang+".json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
		for name, r := range map[string]*model.Report{"small-" + lang: smallSiteReport(lang), "demo-" + lang: example.Demo(lang)} {
			out, err := HTML(r, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, name+".html"), out, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
