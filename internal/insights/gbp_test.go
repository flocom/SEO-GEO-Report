package insights

import (
	"strings"
	"testing"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

func gbpReport(lang string) *model.Report {
	m := func(c, p float64) *model.Metric { return &model.Metric{Current: c, Previous: model.F(p)} }
	return &model.Report{
		Meta: model.Meta{Language: lang},
		SearchConsole: &model.SearchConsole{Totals: model.GSCTotals{
			Clicks: model.M(9000, 8000), Impressions: model.M(300000, 280000),
		}},
		BusinessProfile: &model.BusinessProfile{
			Totals: model.GBPTotals{
				ViewsSearchMobile: m(4000, 3500), ViewsSearchDesktop: m(1000, 1000),
				ViewsMapsMobile: m(3000, 2200), ViewsMapsDesktop: m(500, 450),
				Calls: m(120, 100), WebsiteClicks: m(300, 280), DirectionRequests: m(260, 150),
			},
			SearchKeywords: []model.GBPKeyword{
				{Keyword: "luminaire lyon", Impressions: 15, BelowThreshold: true},
				{Keyword: "maison lumen", Impressions: 820, PrevImpressions: model.F(700)},
			},
			Reviews: &model.GBPReviews{
				AverageRating: m(4.6, 4.4), TotalReviews: m(187, 160), NewReviews: m(27, 18),
				ResponseRate: &model.Metric{Current: 62},
				Recent:       []model.GBPReview{{Rating: 5, Text: "Super", Responded: true}, {Rating: 2, Text: "Bof %d"}},
			},
			Checklist: []model.GBPChecklistItem{{Item: "hours", Done: true}, {Item: "special_hours"}, {Item: "Ma ligne %s"}},
		},
	}
}

func TestGBPAnalysis(t *testing.T) {
	for _, lang := range []string{"fr", "en"} {
		r := gbpReport(lang)
		r.Normalize()
		a := Analyze(r)
		for _, k := range []string{KeyGBPViews, KeyGBPInteractions, KeyGBPCalls, KeyGBPWebsiteClicks, KeyGBPDirections, KeyGBPRating, KeyGBPReviews} {
			if st, ok := a.KPIs[k]; !ok || st.Tone != Positive {
				t.Errorf("%s %s: %+v", lang, k, st)
			}
		}
		var texts []string
		n := 0
		for _, in := range a.Insights {
			if in.Section == "business_profile" {
				n++
				texts = append(texts, in.Text)
			}
		}
		if n < maxOptional {
			t.Errorf("%s: %d business_profile insights, want at least %d: %v", lang, n, maxOptional, texts)
		}
		// Every insight, kept or not, is written properly.
		w := &writer{lang: lang}
		gbpInsights(w, r.BusinessProfile)
		all := ""
		for _, in := range w.out {
			all += in.Text + "\n"
			if strings.Contains(in.Text, "%!") {
				t.Errorf("%s: bad format: %s", lang, in.Text)
			}
		}
		want := map[string][]string{
			"fr": {"vue 8 500 fois", "demandes d'itinéraire", "Google Maps", "smartphone", "4,6/5", "27 nouveaux avis", "Seuls 62 %", "« maison lumen »", "Horaires exceptionnels", "Ma ligne %s"},
			"en": {"viewed 8,500 times", "direction requests", "Google Maps", "smartphone", "4.6/5", "27 new reviews", "Only 62%", "“maison lumen”", "Special hours", "Ma ligne %s"},
		}[lang]
		for _, s := range want {
			if !strings.Contains(strings.ReplaceAll(strings.ReplaceAll(all, "\u202f", " "), "\u00a0", " "), s) {
				t.Errorf("%s: insights miss %q:\n%s", lang, s, all)
			}
		}
	}
}

func TestGBPProgressAndPartial(t *testing.T) {
	r := &model.Report{Meta: model.Meta{Language: "en"}, BusinessProfile: &model.BusinessProfile{
		Totals: model.GBPTotals{Interactions: &model.Metric{Current: 150, Previous: model.F(100)}},
	}}
	r.Normalize()
	a := Analyze(r)
	if a.Progress == nil || *a.Progress <= 50 {
		t.Errorf("progress from GBP interactions: %v", a.Progress)
	}
	// Partial or odd data never panics.
	for _, bp := range []*model.BusinessProfile{
		{},
		{Reviews: &model.GBPReviews{}},
		{Reviews: &model.GBPReviews{AverageRating: &model.Metric{Current: 3.6, Previous: model.F(4)}, Recent: []model.GBPReview{{Rating: 1}}}},
		{Reviews: &model.GBPReviews{RatingDistribution: &model.GBPRatingDistribution{One: 2}}},
		{Totals: model.GBPTotals{ViewsSearchMobile: &model.Metric{Current: 10}}},
		{SearchKeywords: []model.GBPKeyword{{Keyword: " "}}},
		{Daily: []model.GBPDailyPoint{{Date: "2026-07-01"}}},
		{Locations: []model.GBPLocation{{Name: "A"}}, ProfileCompleteness: &model.Metric{Current: 100}},
	} {
		r := &model.Report{Meta: model.Meta{Language: "fr"}, BusinessProfile: bp}
		r.Normalize()
		_ = Analyze(r)
	}
	// Low ratings and unanswered reviews are flagged.
	r = &model.Report{Meta: model.Meta{Language: "en"}, BusinessProfile: &model.BusinessProfile{Reviews: &model.GBPReviews{
		AverageRating: &model.Metric{Current: 3.8}, Recent: []model.GBPReview{{Rating: 1}, {Rating: 2, Responded: true}},
	}}}
	r.Normalize()
	got := ""
	for _, in := range Analyze(r).Insights {
		got += in.Text + "\n"
	}
	if !strings.Contains(got, "below 4") || !strings.Contains(got, "1 review without a reply") {
		t.Errorf("insights: %s", got)
	}
	if maps, mobile, ok := GBPViewShares(model.GBPTotals{ViewsSearchMobile: &model.Metric{Current: 10}}); ok {
		t.Errorf("no Maps figures: no share (%v %v)", maps, mobile)
	}
}

// stripGBPPrevious removes every comparison value of a Business Profile.
func stripGBPPrevious(bp *model.BusinessProfile) {
	if bp == nil {
		return
	}
	strip := func(t *model.GBPTotals) {
		for _, m := range []*model.Metric{t.Views, t.ViewsSearchMobile, t.ViewsSearchDesktop, t.ViewsMapsMobile, t.ViewsMapsDesktop,
			t.Interactions, t.Calls, t.WebsiteClicks, t.DirectionRequests, t.Messages, t.Bookings, t.FoodOrders, t.MenuClicks} {
			if m != nil {
				m.Previous = nil
			}
		}
	}
	strip(&bp.Totals)
	for i := range bp.Locations {
		strip(&bp.Locations[i].Totals)
	}
	bp.PreviousDaily = nil
	for i := range bp.SearchKeywords {
		bp.SearchKeywords[i].PrevImpressions = nil
	}
	for _, m := range []*model.Metric{bp.Posts, bp.Photos, bp.ProfileCompleteness} {
		if m != nil {
			m.Previous = nil
		}
	}
	if rv := bp.Reviews; rv != nil {
		for _, m := range []*model.Metric{rv.AverageRating, rv.TotalReviews, rv.NewReviews, rv.ResponseRate} {
			if m != nil {
				m.Previous = nil
			}
		}
	}
}
