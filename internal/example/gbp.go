package example

import (
	"math"
	"math/rand/v2"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// seedGBP gives Google Business Profile its own random stream, so that the
// other demo data stay unchanged.
const seedGBP = 0x474250 // "GBP"

// businessProfile is the Google Business Profile of the Maison Lumen
// showroom in Lyon. Review texts and search terms stay as customers wrote
// them (in French) in both languages; checklist items are known keys,
// shown in the report language.
func businessProfile(lang string) *model.BusinessProfile {
	rng := rand.New(rand.NewPCG(seed1, seedGBP))
	series := func(start string, vFrom, vTo, actRate float64) []model.GBPDailyPoint {
		var out []model.GBPDailyPoint
		for i, d := range days(start, periodDay) {
			t := float64(i) / float64(periodDay-1)
			// A showroom is busier at the end of the week.
			w := map[int]float64{0: 1.05, 1: 0.82, 2: 0.95, 3: 0.98, 4: 1.02, 5: 1.12, 6: 1.18}[int(d.Weekday())]
			v := math.Round(lerp(vFrom, vTo, t) * w * (1 + noise(rng, 0.10)))
			calls := math.Round(v * actRate * 0.17 * (1 + noise(rng, 0.5)))
			web := math.Round(v * actRate * 0.42 * (1 + noise(rng, 0.3)))
			dir := math.Round(v * actRate * 0.33 * (1 + noise(rng, 0.4)))
			out = append(out, model.GBPDailyPoint{Date: d.Format("2006-01-02"), Views: v, Calls: calls, WebsiteClicks: web, DirectionRequests: dir,
				Interactions: calls + web + dir})
		}
		return out
	}
	daily := series(curStart, 98, 126, 0.088)
	prevDaily := series(prevStart, 82, 94, 0.082)
	sum := func(pts []model.GBPDailyPoint, f func(model.GBPDailyPoint) float64) float64 {
		s := 0.0
		for _, p := range pts {
			s += f(p)
		}
		return s
	}
	views, pviews := sum(daily, gbpView), sum(prevDaily, gbpView)
	calls, pcalls := sum(daily, gbpCall), sum(prevDaily, gbpCall)
	web, pweb := sum(daily, gbpWeb), sum(prevDaily, gbpWeb)
	dir, pdir := sum(daily, gbpDir), sum(prevDaily, gbpDir)
	msgs, pmsgs := 38.0, 31.0
	book, pbook := 22.0, 12.0
	// Daily interactions cover calls, website clicks and directions; messages
	// and bookings are only known as totals.
	inter, pinter := calls+web+dir+msgs+book, pcalls+pweb+pdir+pmsgs+pbook

	vs := split(views, []float64{0.46, 0.14, 0.33, 0.07})
	pvs := split(pviews, []float64{0.49, 0.16, 0.29, 0.06})
	m := func(c, p float64) *model.Metric { return &model.Metric{Current: c, Previous: f(p)} }

	return &model.BusinessProfile{
		Totals: model.GBPTotals{
			Views:              m(views, pviews),
			ViewsSearchMobile:  m(vs[0], pvs[0]),
			ViewsSearchDesktop: m(vs[1], pvs[1]),
			ViewsMapsMobile:    m(vs[2], pvs[2]),
			ViewsMapsDesktop:   m(vs[3], pvs[3]),
			Interactions:       m(inter, pinter),
			Calls:              m(calls, pcalls),
			WebsiteClicks:      m(web, pweb),
			DirectionRequests:  m(dir, pdir),
			Messages:           m(msgs, pmsgs),
			Bookings:           m(book, pbook),
		},
		Daily:         daily,
		PreviousDaily: prevDaily,
		SearchKeywords: []model.GBPKeyword{
			{Keyword: "maison lumen", Impressions: 1240, PrevImpressions: f(1010)},
			{Keyword: "maison lumen lyon", Impressions: 860, PrevImpressions: f(690)},
			{Keyword: "luminaire lyon", Impressions: 410, PrevImpressions: f(300)},
			{Keyword: "magasin luminaire lyon", Impressions: 365, PrevImpressions: f(250)},
			{Keyword: "suspension rotin", Impressions: 180, PrevImpressions: f(150)},
			{Keyword: "showroom luminaire", Impressions: 140, PrevImpressions: f(95)},
			{Keyword: "lampe design lyon", Impressions: 120, PrevImpressions: f(88)},
			{Keyword: "luminaires près de moi", Impressions: 95, PrevImpressions: f(60)},
			{Keyword: "boutique luminaire lyon 6", Impressions: 40, PrevImpressions: f(22)},
			{Keyword: "applique murale lyon", Impressions: 15, BelowThreshold: true},
		},
		Reviews: &model.GBPReviews{
			AverageRating:      m(4.6, 4.5),
			TotalReviews:       m(187, 160),
			NewReviews:         m(27, 18),
			ResponseRate:       m(74, 61),
			RatingDistribution: &model.GBPRatingDistribution{Five: 139, Four: 31, Three: 8, Two: 4, One: 5},
			Recent: []model.GBPReview{
				{Rating: 5, Text: "Showroom magnifique et conseils au top ! Nous sommes repartis avec deux suspensions en rotin, livrées en 3 jours.", Date: "2026-09-24", Author: "Camille R.", Responded: true},
				{Rating: 5, Text: "Accueil chaleureux, choix énorme de lampes à poser. Je recommande.", Date: "2026-09-17", Author: "Julien M.", Responded: true},
				{Rating: 2, Text: "Belle boutique mais l'applique commandée est arrivée avec un défaut, toujours pas de retour du SAV.", Date: "2026-09-11", Author: "Sophie L."},
				{Rating: 4, Text: "Très beaux produits, un peu chers mais la qualité est là. Parking difficile dans le quartier.", Date: "2026-09-03", Author: "Thomas B."},
				{Rating: 5, Text: "Les horaires sur Google étaient à jour et le vendeur nous a aidés à choisir l'ampoule adaptée.", Date: "2026-08-28", Author: "Nadia K.", Responded: true},
			},
		},
		Posts:               m(9, 4),
		Photos:              m(24, 11),
		ProfileCompleteness: m(75, 67),
		Checklist: []model.GBPChecklistItem{
			{Item: "hours", Done: true}, {Item: "special_hours"}, {Item: "categories", Done: true},
			{Item: "description", Done: true}, {Item: "photos", Done: true}, {Item: "products", Done: true},
			{Item: "services"}, {Item: "website", Done: true}, {Item: "phone", Done: true},
			{Item: "attributes"}, {Item: "booking", Done: true}, {Item: "posts", Done: true},
		},
	}
}

func gbpView(p model.GBPDailyPoint) float64 { return p.Views }
func gbpCall(p model.GBPDailyPoint) float64 { return p.Calls }
func gbpWeb(p model.GBPDailyPoint) float64  { return p.WebsiteClicks }
func gbpDir(p model.GBPDailyPoint) float64  { return p.DirectionRequests }
