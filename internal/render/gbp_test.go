package render

import (
	"slices"
	"strings"
	"testing"

	"github.com/flocom/SEO-GEO-Report/internal/example"
	"github.com/flocom/SEO-GEO-Report/internal/model"
)

func gbpSection(t *testing.T, s string) string {
	t.Helper()
	i := strings.Index(s, `id="sec-business_profile"`)
	if i < 0 {
		t.Fatal("no business_profile section")
	}
	rest := s[i:]
	if j := strings.Index(rest[1:], `<section class="section`); j > 0 {
		return rest[:j+1]
	}
	return rest
}

func TestBusinessProfileSection(t *testing.T) {
	for _, lang := range []string{"fr", "en"} {
		s := mustRender(t, example.Demo(lang), Options{})
		ids := sectionIDs(s)
		i := slices.Index(ids, "sec-business_profile")
		if i < 0 || ids[i-1] != "sec-merchant_center" || ids[i+1] != "sec-geo" {
			t.Fatalf("%s: business_profile section misplaced in %v", lang, ids)
		}
		sec := gbpSection(t, s)
		want := map[string][]string{
			"fr": {"Fiche Google (Google Business Profile)", "la carte de votre établissement", "Vues de la fiche Google par jour", "Où les clients vous ont trouvé",
				"Google Maps · mobile", "Ce que les clients ont fait", "Demandes d’itinéraire", "Réservations", "Avis clients", "sur 5", "187 avis",
				"Répartition des notes", "5 étoiles", "Complétude de la fiche", "Horaires exceptionnels", "À compléter", "Recherches locales qui affichent votre fiche",
				"moins de 15", "Derniers avis clients", "Showroom magnifique", "Sans réponse", "Publications", "Seuls 74", "gbp-stars-fg"},
			"en": {"Google Business Profile (local visibility)", "the card of your business", "Daily Google profile views", "Where people found you",
				"Google Maps · mobile", "What people did", "Direction requests", "Bookings", "Customer reviews", "out of 5", "187 reviews",
				"Rating breakdown", "5 stars", "Profile completeness", "Special hours", "To do", "Local searches that show your profile",
				"fewer than 15", "Latest customer reviews", "Showroom magnifique", "No reply", "Posts", "Only 74%", "gbp-stars-fg"},
		}[lang]
		for _, w := range want {
			if !strings.Contains(sec, w) {
				t.Errorf("%s: business_profile section misses %q", lang, w)
			}
		}
		if strings.Contains(sec, "Détail par établissement") || strings.Contains(sec, "Details by location") {
			t.Errorf("%s: one location, no location table", lang)
		}
		if !strings.Contains(s, `<li class="chip">Google Merchant Center</li><li class="chip">Google Business Profile</li>`) {
			t.Errorf("%s: cover sources miss Google Business Profile", lang)
		}
		// Part 1: the profile actions card, eight cards on one desktop row.
		kpis := s[strings.Index(s, `id="sec-kpis"`):]
		kpis = kpis[:strings.Index(kpis, "</section>")]
		if !strings.Contains(kpis, `class="kpi-grid bal n-8 big" style="--bd:8;`) || !strings.Contains(kpis, map[string]string{"fr": "Actions sur la fiche Google", "en": "Google profile actions"}[lang]) {
			t.Errorf("%s: key indicators should hold the Google profile card on one row", lang)
		}
		if !strings.Contains(s, map[string]string{"fr": "Demande d’itinéraire", "en": "Direction request"}[lang]) || !strings.Contains(s, map[string]string{"fr": "pack local", "en": "local pack"}[lang]) {
			t.Errorf("%s: glossary misses the Business Profile terms", lang)
		}
	}
}

func TestBusinessProfileAbsentOrHidden(t *testing.T) {
	r := example.Demo("en")
	r.BusinessProfile = nil
	s := mustRender(t, r, Options{})
	if strings.Contains(s, `id="sec-business_profile"`) || strings.Contains(s, "Google profile actions") || strings.Contains(s, "local pack") || strings.Contains(s, "Google Business Profile") {
		t.Error("no Business Profile data: no section, no KPI card, no glossary terms, no cover source")
	}
	if !strings.Contains(s, `class="kpi-grid bal n-7 big" style="--bd:7;`) {
		t.Error("without Business Profile the key indicators keep seven cards")
	}

	r = example.Demo("en")
	r.Options = &model.Options{HideSections: []string{"business_profile"}}
	if s := mustRender(t, r, Options{}); strings.Contains(s, `id="sec-business_profile"`) {
		t.Error("business_profile section should be hidden")
	}

	// Note and custom section placed after the section.
	r = example.Demo("fr")
	r.Narrative.SectionNotes["business_profile"] = "Note **GBP**"
	r.Sections = append(r.Sections, model.TextSection{ID: "after-gbp", Title: "Après la fiche", Body: "x", After: "business_profile"})
	s = mustRender(t, r, Options{})
	ids := sectionIDs(s)
	if i := slices.Index(ids, "sec-business_profile"); i < 0 || ids[i+1] != "sec-c-after-gbp" || !strings.Contains(s, "Note <strong>GBP</strong>") {
		t.Errorf("note / custom section: %v", ids)
	}

	// An empty object renders no section and no glossary terms.
	full := example.Demo("en")
	s = mustRender(t, &model.Report{Meta: full.Meta, BusinessProfile: &model.BusinessProfile{}}, Options{})
	if strings.Contains(s, `id="sec-business_profile"`) || strings.Contains(s, "local pack") {
		t.Error("empty business_profile: no section")
	}
}

func TestBusinessProfileOnlyPartialAndLocations(t *testing.T) {
	full := example.Demo("fr")

	// Business Profile alone.
	s := mustRender(t, &model.Report{Meta: full.Meta, BusinessProfile: full.BusinessProfile}, Options{})
	if !strings.Contains(s, `id="sec-business_profile"`) || strings.Contains(s, `id="sec-search_console"`) || !strings.Contains(s, `id="sec-kpis"`) {
		t.Errorf("GBP-only report: sections %v", sectionIDs(s))
	}
	// Part 1 keeps the three most meaningful profile cards.
	kpis := s[strings.Index(s, `id="sec-kpis"`):]
	kpis = kpis[:strings.Index(kpis, "</section>")]
	if !strings.Contains(kpis, `class="kpi-grid bal n-3 big"`) || !strings.Contains(kpis, "Actions sur la fiche Google") ||
		!strings.Contains(kpis, "Appels depuis la fiche") || !strings.Contains(kpis, "Note Google") || strings.Contains(kpis, "Avis Google") {
		t.Errorf("GBP-only key indicators should hold actions, calls and rating")
	}

	cases := map[string]*model.BusinessProfile{
		"reviews only":   {Reviews: &model.GBPReviews{AverageRating: &model.Metric{Current: 4.2}}},
		"distribution":   {Reviews: &model.GBPReviews{RatingDistribution: &model.GBPRatingDistribution{Five: 3, One: 1}}},
		"recent only":    {Reviews: &model.GBPReviews{Recent: []model.GBPReview{{Rating: 3}}}},
		"keywords only":  {SearchKeywords: []model.GBPKeyword{{Keyword: "lamp", Impressions: 15, BelowThreshold: true}}},
		"daily only":     {Daily: []model.GBPDailyPoint{{Date: "2026-07-01", Calls: 1}, {Date: "2026-07-02", WebsiteClicks: 3}}},
		"checklist only": {Checklist: []model.GBPChecklistItem{{Item: "hours"}, {Item: "Custom item", Done: true}}},
		"activity only":  {Posts: &model.Metric{Current: 2}},
		"views split":    {Totals: model.GBPTotals{ViewsMapsMobile: &model.Metric{Current: 3}, ViewsSearchMobile: &model.Metric{Current: 0}}},
		"single action":  {Totals: model.GBPTotals{Calls: &model.Metric{Current: 3, Previous: model.F(0)}}},
	}
	for name, bp := range cases {
		s := mustRender(t, &model.Report{Meta: full.Meta, BusinessProfile: bp}, Options{})
		if !strings.Contains(s, `id="sec-business_profile"`) {
			t.Errorf("%s: section missing", name)
		}
	}

	// Several locations: a comparison table and summed totals.
	m := func(c, p float64) *model.Metric { return &model.Metric{Current: c, Previous: model.F(p)} }
	bp := &model.BusinessProfile{Locations: []model.GBPLocation{
		{Name: "Showroom Lyon", City: "Lyon", StoreCode: "LY1", Totals: model.GBPTotals{Views: m(900, 700), Calls: m(20, 10)}, Rating: model.F(4.6), ReviewCount: model.F(120)},
		{Name: "Boutique Paris", City: "Paris", Totals: model.GBPTotals{Views: m(1200, 1300), Calls: m(30, 35), DirectionRequests: m(40, 30)}},
	}}
	s = mustRender(t, &model.Report{Meta: full.Meta, BusinessProfile: bp}, Options{})
	sec := gbpSection(t, s)
	if !strings.Contains(sec, "Détail par établissement") || strings.Index(sec, "Boutique Paris") > strings.Index(sec, "Showroom Lyon") || !strings.Contains(sec, "LY1 · Lyon") || !strings.Contains(sec, "2\u202f100") {
		t.Errorf("locations table / summed views missing or misordered")
	}
}

func TestBusinessProfileEscaping(t *testing.T) {
	full := example.Demo("en")
	evil := `<script>alert("x")</script>`
	bp := &model.BusinessProfile{
		Totals:         model.GBPTotals{Views: &model.Metric{Current: 10, Previous: model.F(5)}, Calls: &model.Metric{Current: 1}, WebsiteClicks: &model.Metric{Current: 2}},
		SearchKeywords: []model.GBPKeyword{{Keyword: evil, Impressions: 20}},
		Reviews:        &model.GBPReviews{AverageRating: &model.Metric{Current: 4}, Recent: []model.GBPReview{{Rating: 1, Text: evil, Author: evil, Date: evil}}},
		Checklist:      []model.GBPChecklistItem{{Item: evil}, {Item: "hours", Done: true}},
		Locations:      []model.GBPLocation{{Name: evil, Address: evil, Totals: model.GBPTotals{Views: &model.Metric{Current: 1}}}, {Name: "B", Totals: model.GBPTotals{Views: &model.Metric{Current: 2}}}},
	}
	s := mustRender(t, &model.Report{Meta: full.Meta, BusinessProfile: bp}, Options{})
	if strings.Contains(s, evil) {
		t.Error("unescaped Business Profile data in output")
	}
	if !strings.Contains(s, "&lt;script&gt;") {
		t.Error("the escaped review text should be shown")
	}
}
