package importer

import (
	"testing"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

func parseGBP(t *testing.T, csv string, kind GBPKind, into *model.BusinessProfile) (*model.BusinessProfile, *GBPResult) {
	t.Helper()
	bp, res, err := ParseGBPCSV([]byte(csv), kind, into)
	if err != nil {
		t.Fatalf("ParseGBPCSV: %v\n%s", err, csv)
	}
	return bp, res
}

func cur(m *model.Metric) float64 {
	if m == nil {
		return -1
	}
	return m.Current
}

// Bulk "insights" export (English), several locations, with a description
// row under the header and a total row.
func TestGBPPerformanceEN(t *testing.T) {
	csv := "\uFEFF# Business Profile performance export\n" +
		"Store code,Business name,Address,Google Search - Mobile,Google Search - Desktop,Google Maps - Mobile,Google Maps - Desktop,Calls,Messages,Bookings,Directions,Website clicks,Food orders,Overall rating\n" +
		"Unique store identifier,Business name,Business address,Number of views,,,,Number of calls,,,,,,\n" +
		"LYON-01,Maison Lumen Lyon,\"12 rue Mercière, Lyon\",\"4,701\",1431,3372,715,157,38,22,285,377,0,4.6\n" +
		"PAR-01,Maison Lumen Paris,\"3 rue de Rivoli, Paris\",2100,800,1500,300,90,10,5,140,200,0,4.3\n" +
		"Total,,,6801,2231,4872,1015,247,48,27,425,577,0,\n"
	bp, res := parseGBP(t, csv, "", nil)
	if res.Kind != GBPPerformance || res.Rows != 2 || res.Locations != 2 || len(bp.Locations) != 2 {
		t.Fatalf("result %+v locations %+v", res, bp.Locations)
	}
	l := bp.Locations[0]
	if l.StoreCode != "LYON-01" || l.Name != "Maison Lumen Lyon" || l.Address != "12 rue Mercière, Lyon" || cur(l.Totals.ViewsSearchMobile) != 4701 ||
		cur(l.Totals.ViewsMapsDesktop) != 715 || cur(l.Totals.Calls) != 157 || cur(l.Totals.DirectionRequests) != 285 || cur(l.Totals.WebsiteClicks) != 377 ||
		l.Rating == nil || *l.Rating != 4.6 || cur(l.Totals.Views) != 4701+1431+3372+715 || cur(l.Totals.Interactions) != 157+38+22+285+377 {
		t.Errorf("location: %+v", l)
	}
	if cur(bp.Totals.Views) != 4701+1431+3372+715+2100+800+1500+300 || cur(bp.Totals.Calls) != 247 || cur(bp.Totals.Bookings) != 27 {
		t.Errorf("summed totals: %+v", bp.Totals)
	}
}

// French export (semicolons), one location: totals at the root, previous
// columns fill the comparison values.
func TestGBPPerformanceFR(t *testing.T) {
	csv := "Code de magasin;Nom de l'établissement;Recherche Google – mobile;Recherche Google – ordinateur;Google Maps – mobile;Google Maps – ordinateur;Appels;Appels (période précédente);Demandes d'itinéraire;Clics vers le site Web;Réservations\n" +
		"LYON-01;Maison Lumen;4 701;1 431;3 372;715;157;120;285;377;22\n"
	bp, res := parseGBP(t, csv, "", &model.BusinessProfile{Reviews: &model.GBPReviews{TotalReviews: &model.Metric{Current: 3}}})
	if res.Kind != GBPPerformance || res.Rows != 1 || len(bp.Locations) != 0 {
		t.Fatalf("result %+v", res)
	}
	tt := bp.Totals
	if cur(tt.ViewsSearchMobile) != 4701 || cur(tt.ViewsSearchDesktop) != 1431 || cur(tt.ViewsMapsMobile) != 3372 || cur(tt.Calls) != 157 ||
		tt.Calls.Previous == nil || *tt.Calls.Previous != 120 || cur(tt.DirectionRequests) != 285 || cur(tt.WebsiteClicks) != 377 || cur(tt.Bookings) != 22 {
		t.Errorf("totals: %+v", tt)
	}
	if bp.Reviews == nil || bp.Reviews.TotalReviews.Current != 3 {
		t.Error("existing data must be kept")
	}
}

// Performance API metric names.
func TestGBPPerformanceAPI(t *testing.T) {
	csv := "location,BUSINESS_IMPRESSIONS_MOBILE_SEARCH,BUSINESS_IMPRESSIONS_DESKTOP_MAPS,CALL_CLICKS,WEBSITE_CLICKS,BUSINESS_DIRECTION_REQUESTS,BUSINESS_CONVERSATIONS,BUSINESS_FOOD_MENU_CLICKS\n" +
		"locations/123,500,40,12,30,20,3,7\n"
	bp, _ := parseGBP(t, csv, "", nil)
	tt := bp.Totals
	if cur(tt.ViewsSearchMobile) != 500 || cur(tt.ViewsMapsDesktop) != 40 || cur(tt.Calls) != 12 || cur(tt.WebsiteClicks) != 30 ||
		cur(tt.DirectionRequests) != 20 || cur(tt.Messages) != 3 || cur(tt.MenuClicks) != 7 {
		t.Errorf("API totals: %+v", tt)
	}
}

func TestGBPDaily(t *testing.T) {
	csv := "Date;Vues de la fiche;Appels;Demandes d'itinéraire;Clics vers le site Web;Vues de la fiche (période précédente)\n" +
		"02/07/2026;120;2;3;5;100\n" +
		"01/07/2026;100;1;2;4;90\n" +
		"13/07/2026;80;0;1;3;70\n"
	bp, res := parseGBP(t, csv, "", nil)
	if res.Kind != GBPDaily || res.Rows != 3 || res.PreviousDays != 3 {
		t.Fatalf("result %+v", res)
	}
	if bp.Daily[0].Date != "2026-07-01" || bp.Daily[2].Date != "2026-07-13" || bp.Daily[1].Views != 120 || bp.Daily[1].Interactions != 10 {
		t.Errorf("daily: %+v", bp.Daily)
	}
	if bp.PreviousDaily[0].Date >= "2026-07-01" {
		t.Errorf("previous dates must be shifted: %+v", bp.PreviousDaily)
	}
	if cur(bp.Totals.Views) != 300 || bp.Totals.Views.Previous == nil || *bp.Totals.Views.Previous != 260 || cur(bp.Totals.Calls) != 3 {
		t.Errorf("totals from daily: %+v", bp.Totals)
	}
	// Existing totals are kept.
	bp, _ = parseGBP(t, csv, GBPDaily, &model.BusinessProfile{Totals: model.GBPTotals{Views: &model.Metric{Current: 999}}})
	if cur(bp.Totals.Views) != 999 {
		t.Errorf("totals overwritten: %+v", bp.Totals.Views)
	}
}

func TestGBPKeywords(t *testing.T) {
	en := "Search keyword,Value\nmaison lumen,\"1,240\"\nluminaire lyon,410\napplique murale lyon,<15\n"
	bp, res := parseGBP(t, en, "", nil)
	if res.Kind != GBPKeywords || len(bp.SearchKeywords) != 3 {
		t.Fatalf("result %+v %+v", res, bp.SearchKeywords)
	}
	if k := bp.SearchKeywords[0]; k.Keyword != "maison lumen" || k.Impressions != 1240 || k.BelowThreshold {
		t.Errorf("first: %+v", k)
	}
	if k := bp.SearchKeywords[2]; k.Impressions != 15 || !k.BelowThreshold {
		t.Errorf("threshold: %+v", k)
	}
	fr := "Requête de recherche;Valeur;Valeur (période précédente)\nluminaire lyon;410;300\nlampe design;moins de 15;20\n"
	bp, _ = parseGBP(t, fr, "keywords", nil)
	if k := bp.SearchKeywords[0]; k.Impressions != 410 || k.PrevImpressions == nil || *k.PrevImpressions != 300 {
		t.Errorf("fr: %+v", k)
	}
	if k := bp.SearchKeywords[1]; !k.BelowThreshold || k.Impressions != 15 {
		t.Errorf("fr threshold: %+v", k)
	}
	searches := "Search terms used to find your business,Searches\nlamp shop,50\n"
	if bp, res := parseGBP(t, searches, "", nil); res.Kind != GBPKeywords || bp.SearchKeywords[0].Impressions != 50 {
		t.Errorf("searches header: %+v", res)
	}
}

func TestGBPReviews(t *testing.T) {
	en := "Reviewer,Star rating,Comment,Create time,Reply\n" +
		"Camille,FIVE,\"Great showroom, <b>top</b> advice\",2026-09-24T10:00:00Z,Thank you!\n" +
		"Sophie,2,Defective lamp,2026-09-11,\n" +
		"Thomas,★★★★,,2026-09-03,\n" +
		"Bad,9,ignored,2026-09-01,\n"
	bp, res := parseGBP(t, en, "", nil)
	if res.Kind != GBPReviews || res.Rows != 3 {
		t.Fatalf("result %+v", res)
	}
	rv := bp.Reviews
	if len(rv.Recent) != 3 || rv.Recent[0].Author != "Camille" || rv.Recent[0].Rating != 5 || !rv.Recent[0].Responded || rv.Recent[0].Date != "2026-09-24" ||
		rv.Recent[1].Responded || rv.Recent[2].Rating != 4 {
		t.Errorf("recent: %+v", rv.Recent)
	}
	if rv.RatingDistribution.Five != 1 || rv.RatingDistribution.Two != 1 || rv.RatingDistribution.Four != 1 {
		t.Errorf("distribution: %+v", rv.RatingDistribution)
	}
	if rv.AverageRating == nil || rv.AverageRating.Current != 3.67 || rv.TotalReviews.Current != 3 || rv.ResponseRate == nil || rv.ResponseRate.Current != 33.33 {
		t.Errorf("summary: %+v %+v %+v", rv.AverageRating, rv.TotalReviews, rv.ResponseRate)
	}
	fr := "Auteur;Note;Avis;Date;Réponse du propriétaire\nNadia;5 étoiles;Super accueil;28/08/2026;Merci\n"
	bp, _ = parseGBP(t, fr, "", nil)
	if r := bp.Reviews.Recent[0]; r.Rating != 5 || r.Text != "Super accueil" || r.Date != "2026-08-28" || !r.Responded {
		t.Errorf("fr review: %+v", r)
	}
}

func TestGBPKindsAndErrors(t *testing.T) {
	for name, want := range map[string]GBPKind{
		"Avis Google.csv": GBPReviews, "search keywords.csv": GBPKeywords, "Mots clés.csv": GBPKeywords,
		"daily.csv": GBPDaily, "Insights bulk.csv": GBPPerformance, "other.csv": "",
	} {
		if got := GuessGBPKind(name); got != want {
			t.Errorf("GuessGBPKind(%q) = %q, want %q", name, got, want)
		}
	}
	if normalizeGBPKind("avis") != GBPReviews || normalizeGBPKind("insights") != GBPPerformance {
		t.Error("kind aliases")
	}
	if _, _, err := ParseGBPCSV([]byte("foo,bar\n1,2\n"), "", nil); err == nil {
		t.Error("unknown CSV must fail")
	}
	if _, _, err := ParseGBPCSV([]byte("Search keyword,Value\nfoo,1\n"), GBPReviews, nil); err == nil {
		t.Error("wrong kind must fail")
	}
	if _, _, err := ParseGBPCSV([]byte("Calls,Website clicks\n"), "", nil); err == nil {
		t.Error("header only must fail")
	}
}
