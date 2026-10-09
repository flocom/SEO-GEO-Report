package render

import (
	"fmt"
	"math"
	"time"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

func ip(i int) *int        { return &i }
func bp(b bool) *bool      { return &b }
func f(v float64) *float64 { return &v }

func days(start string, n int) []string {
	t, _ := time.Parse("2006-01-02", start)
	out := make([]string, n)
	for i := range out {
		out[i] = t.AddDate(0, 0, i).Format("2006-01-02")
	}
	return out
}

// richReport returns a report filling every field of the model.
func richReport(lang string) *model.Report {
	cur := days("2026-07-01", 92)
	prev := days("2026-04-01", 92)
	r := &model.Report{
		Meta: model.Meta{
			SiteName: "Atelier Lumière", SiteURL: "https://www.atelier-lumiere.fr", Language: lang,
			Period:           model.Period{Start: "2026-07-01", End: "2026-09-30"},
			ComparisonPeriod: &model.Period{Start: "2026-04-01", End: "2026-06-30"},
			PreparedBy:       "Agence Zénith", PreparedFor: "Atelier Lumière SAS",
			BrandColor: "#7c3aed", Currency: "EUR",
		},
	}
	sc := &model.SearchConsole{
		Totals: model.GSCTotals{
			Clicks: model.M(48210, 39870), Impressions: model.M(1843000, 1502000),
			Position: &model.Metric{Current: 11.4, Previous: f(13.1)},
		},
	}
	for i, d := range cur {
		w := 1 + 0.18*math.Sin(float64(i)/7*2*math.Pi)
		c := (450 + float64(i)*2.2) * w
		im := (17000 + float64(i)*60) * w
		sc.Daily = append(sc.Daily, model.GSCDailyPoint{Date: d, Clicks: math.Round(c), Impressions: math.Round(im), Position: 12.5 - float64(i)*0.02})
	}
	for i, d := range prev {
		w := 1 + 0.18*math.Sin(float64(i)/7*2*math.Pi)
		sc.PreviousDaily = append(sc.PreviousDaily, model.GSCDailyPoint{Date: d, Clicks: math.Round((420 + float64(i)*0.6) * w), Impressions: math.Round((16000 + float64(i)*20) * w), Position: 13.4 - float64(i)*0.005})
	}
	queries := []string{"suspension luminaire", "lampe design", "atelier lumière", "luminaire salon", "applique murale laiton",
		"lampadaire arc", "suspension rotin", "<script>alert(1)</script>", "lampe de chevet tactile", "plafonnier led design",
		"suspension cuisine îlot", "luminaire scandinave", "lampe bureau architecte", "guirlande guinguette", "ampoule filament e27",
		"spot encastrable", "lampe à poser céramique", "suspension verre soufflé"}
	for i, q := range queries {
		c := 3200 / float64(i+1)
		pc := c * (0.7 + 0.05*float64(i%7))
		pos := 1.2 + float64(i)*1.9
		sc.Queries = append(sc.Queries, model.GSCRow{Key: q, Clicks: math.Round(c), Impressions: math.Round(c * (25 + float64(i)*6)), Position: pos,
			PrevClicks: f(math.Round(pc)), PrevImpressions: f(math.Round(c * 22)), PrevPosition: f(pos + 1.5 - float64(i%4))})
	}
	for i := 0; i < 12; i++ {
		c := 5200 / float64(i+1)
		sc.Pages = append(sc.Pages, model.GSCRow{Key: fmt.Sprintf("https://www.atelier-lumiere.fr/collections/luminaire-%d/?ref=%d", i, i), Clicks: math.Round(c), Impressions: math.Round(c * 30), Position: 3 + float64(i), PrevClicks: f(math.Round(c * 0.85)), PrevPosition: f(4 + float64(i))})
	}
	sc.Countries = []model.GSCRow{{Key: "France", Clicks: 39100, Impressions: 1490000, Position: 10.2, PrevClicks: f(32800)}, {Key: "Belgique", Clicks: 4200, Impressions: 160000, Position: 12.8, PrevClicks: f(3500)}, {Key: "Suisse", Clicks: 2600, Impressions: 98000, Position: 14.1, PrevClicks: f(2400)}, {Key: "Canada", Clicks: 1300, Impressions: 51000, Position: 18.3, PrevClicks: f(900)}, {Key: "Luxembourg", Clicks: 610, Impressions: 21000, Position: 11.0, PrevClicks: f(700)}}
	sc.Devices = []model.GSCRow{{Key: "mobile", Clicks: 30100, Impressions: 1210000, Position: 11.9, PrevClicks: f(24100)}, {Key: "desktop", Clicks: 16500, Impressions: 580000, Position: 10.3, PrevClicks: f(14600)}, {Key: "tablet", Clicks: 1610, Impressions: 53000, Position: 12.4, PrevClicks: f(1170)}}
	sc.SearchAppearance = []model.GSCRow{{Key: "Extraits produits", Clicks: 8200, Impressions: 210000, Position: 6.1, PrevClicks: f(5100)}, {Key: "Avis", Clicks: 2100, Impressions: 64000, Position: 7.9, PrevClicks: f(2300)}}
	sc.BrandSplit = &model.BrandSplit{Branded: model.M(12100, 11800), NonBranded: model.M(36110, 28070), BrandTerms: []string{"atelier lumière", "atelier lumiere"}}
	sc.Indexing = &model.Indexing{Indexed: model.M(1840, 1720), NotIndexed: &model.Metric{Current: 410, Previous: f(530)},
		Issues:   []model.IndexingIssue{{Reason: "Explorée, actuellement non indexée", Pages: 180}, {Reason: "Page avec redirection", Pages: 120}, {Reason: "Autre page avec balise canonique correcte", Pages: 80}, {Reason: "Introuvable (404)", Pages: 30}},
		Sitemaps: []model.Sitemap{{URL: "https://www.atelier-lumiere.fr/sitemap.xml", Discovered: 2210, Status: "Opération effectuée"}, {URL: "https://www.atelier-lumiere.fr/sitemap-blog.xml", Discovered: 140, Status: "Erreur"}}}
	sc.CoreWebVitals = &model.CoreWebVitals{Mobile: &model.CWVStatus{Good: 820, NeedsImprovement: 310, Poor: 90}, Desktop: &model.CWVStatus{Good: 1150, NeedsImprovement: 60, Poor: 10}}
	r.SearchConsole = sc

	a := &model.Analytics{
		Totals: model.GA4Totals{Sessions: &model.Metric{Current: 81200, Previous: f(70100)}, Users: &model.Metric{Current: 60400, Previous: f(53900)},
			NewUsers: &model.Metric{Current: 48100, Previous: f(44000)}, EngagedSessions: &model.Metric{Current: 52300, Previous: f(43100)},
			EngagementRate: &model.Metric{Current: 64.4, Previous: f(61.5)}, AvgEngagementTime: &model.Metric{Current: 92, Previous: f(84)},
			PageViews: &model.Metric{Current: 251000, Previous: f(221000)}, KeyEvents: &model.Metric{Current: 1830, Previous: f(1510)},
			Revenue: &model.Metric{Current: 214500, Previous: f(181200)}},
		OrganicTotals: &model.GA4Totals{Sessions: &model.Metric{Current: 44100, Previous: f(36900)}, Users: &model.Metric{Current: 35200, Previous: f(30100)},
			EngagementRate: &model.Metric{Current: 66.1, Previous: f(62.0)}, KeyEvents: &model.Metric{Current: 940, Previous: f(760)}, Revenue: &model.Metric{Current: 112000, Previous: f(91000)}},
	}
	for i, d := range cur {
		w := 1 + 0.15*math.Sin(float64(i)/7*2*math.Pi)
		a.Daily = append(a.Daily, model.GA4DailyPoint{Date: d, Sessions: math.Round((820 + float64(i)*2) * w), OrganicSessions: math.Round((440 + float64(i)*1.5) * w), AISessions: math.Round((8 + float64(i)*0.15) * w)})
	}
	for i, d := range prev {
		a.PreviousDaily = append(a.PreviousDaily, model.GA4DailyPoint{Date: d, Sessions: math.Round(760 + float64(i)*0.3)})
	}
	a.Channels = []model.ChannelRow{{Channel: "Organic Search", Sessions: 44100, PrevSessions: f(36900), Users: 35200, EngagementRate: 66.1, KeyEvents: 940, Revenue: 112000},
		{Channel: "Direct", Sessions: 17800, PrevSessions: f(16900), Users: 14100, EngagementRate: 61.2, KeyEvents: 420, Revenue: 51000},
		{Channel: "Referral", Sessions: 7200, PrevSessions: f(6100), Users: 5600, EngagementRate: 58.4, KeyEvents: 160, Revenue: 17800},
		{Channel: "Organic Social", Sessions: 6100, PrevSessions: f(6900), Users: 5300, EngagementRate: 44.0, KeyEvents: 70, Revenue: 6100},
		{Channel: "Email", Sessions: 3900, PrevSessions: f(2600), Users: 2800, EngagementRate: 71.3, KeyEvents: 190, Revenue: 23400},
		{Channel: "Paid Search", Sessions: 2100, PrevSessions: f(700), Users: 1900, EngagementRate: 55.1, KeyEvents: 50, Revenue: 4200}}
	a.Sources = []model.SourceRow{{Source: "google", Medium: "organic", Sessions: 41800, PrevSessions: f(35100), EngagementRate: 66.3, KeyEvents: 900},
		{Source: "(direct)", Medium: "(none)", Sessions: 17800, PrevSessions: f(16900), EngagementRate: 61.2, KeyEvents: 420},
		{Source: "chatgpt.com", Medium: "referral", Sessions: 1240, PrevSessions: f(610), EngagementRate: 72.5, KeyEvents: 48},
		{Source: "perplexity.ai", Medium: "referral", Sessions: 420, PrevSessions: f(250), EngagementRate: 69.0, KeyEvents: 15},
		{Source: "bing", Medium: "organic", Sessions: 1900, PrevSessions: f(1500), EngagementRate: 63.0, KeyEvents: 31}}
	for i := 0; i < 10; i++ {
		s := 9000 / float64(i+1)
		a.LandingPages = append(a.LandingPages, model.LandingPageRow{Page: fmt.Sprintf("/collections/luminaire-%d", i), Sessions: math.Round(s), PrevSessions: f(math.Round(s * 0.9)), EngagementRate: 60 + float64(i), KeyEvents: math.Round(s / 50), Revenue: math.Round(s * 2.4)})
	}
	r.Analytics = a

	r.GEO = &model.GEO{
		AIReferrals: []model.AIReferralRow{{Platform: "ChatGPT", Source: "chatgpt.com", Sessions: 1240, PrevSessions: f(610), Users: 1010, EngagementRate: 72.5, KeyEvents: 48, Revenue: 6100},
			{Platform: "Perplexity", Source: "perplexity.ai", Sessions: 420, PrevSessions: f(250), Users: 360, EngagementRate: 69.0, KeyEvents: 15, Revenue: 1900},
			{Platform: "Gemini", Source: "gemini.google.com", Sessions: 160, PrevSessions: f(40), Users: 140, EngagementRate: 64.0, KeyEvents: 4},
			{Platform: "Copilot", Source: "copilot.microsoft.com", Sessions: 75, PrevSessions: f(80), Users: 70, EngagementRate: 58.0}},
		Citations: []model.CitationCheck{
			{Prompt: "Quelle marque française pour une suspension en laiton ?", Engine: "ChatGPT", Cited: true, Position: ip(2), PreviouslyCited: bp(false), CompetitorsCited: []string{"Maison Lux"}, Date: "2026-09-25"},
			{Prompt: "Meilleur luminaire design pour salon", Engine: "Perplexity", Cited: true, Position: ip(1), PreviouslyCited: bp(true), Date: "2026-09-25"},
			{Prompt: "Où acheter une lampe en céramique artisanale ?", Engine: "Gemini", Cited: false, Mentioned: true, PreviouslyCited: bp(true), CompetitorsCited: []string{"Céramiques & Co", "Lampes de France"}, Date: "2026-09-26", Notes: "Mention dans le texte, sans lien"},
			{Prompt: "Suspension rotin pas chère", Engine: "Google AI Overviews", Cited: false, PreviouslyCited: bp(false), CompetitorsCited: []string{"Maison Lux"}, Date: "2026-09-26"},
			{Prompt: "Quelle ampoule pour une suspension en verre ?", Engine: "ChatGPT", Cited: true, Position: ip(3), PreviouslyCited: bp(true), Date: "2026-09-27"},
			{Prompt: "Luminaire artisanal fabriqué en France", Engine: "Copilot", Cited: false, PreviouslyCited: bp(false), Date: "2026-09-27"},
		},
		ShareOfVoice: []model.ShareOfVoice{{Brand: "Atelier Lumière", Share: 24, Previous: f(17), IsSelf: true}, {Brand: "Maison Lux", Share: 31, Previous: f(33)}, {Brand: "Lampes de France", Share: 18, Previous: f(19)}, {Brand: "Céramiques & Co", Share: 9, Previous: f(8)}},
		AIOverviews:  []model.AIOverviewRow{{Query: "suspension luminaire", OverviewPresent: true, Cited: true}, {Query: "lampe design", OverviewPresent: true, Cited: false, Notes: "Concurrent cité"}, {Query: "luminaire salon", OverviewPresent: false}, {Query: "applique murale laiton", OverviewPresent: true, Cited: true}},
		AICrawlers:   []model.CrawlerRow{{Bot: "GPTBot", Hits: 8200, PrevHits: f(5100)}, {Bot: "OAI-SearchBot", Hits: 3100, PrevHits: f(1200)}, {Bot: "ClaudeBot", Hits: 2600, PrevHits: f(2900)}, {Bot: "PerplexityBot", Hits: 1900, PrevHits: f(800)}, {Bot: "Google-Extended", Hits: 1200}},
	}
	for i, d := range cur {
		r.GEO.AIDaily = append(r.GEO.AIDaily, model.DailyValue{Date: d, Value: math.Round(12 + float64(i)*0.22 + 4*math.Sin(float64(i)/3))})
	}

	r.Narrative = &model.Narrative{
		ExecutiveSummary: "Le trimestre est **nettement positif** : les clics Google progressent de **+21 %** et les visites venues des assistants IA doublent.\n\n- Les pages collections portent la croissance\n- La position moyenne gagne 1,7 place\n\nLa priorité du prochain trimestre : consolider la présence dans les **AI Overviews**.",
		Highlights:       []string{"Clics Google en hausse de **21 %**", "Le trafic IA a doublé (ChatGPT en tête)", "Première citation sur Perplexity pour « luminaire design »"},
		Concerns:         []string{"Les Core Web Vitals mobiles restent perfectibles (27 % d'URL non « bonnes »)", "La requête « lampe design » perd des clics"},
		Actions: []model.Action{
			{Title: "Refonte des pages collections", Description: "Ajout de textes d'introduction, FAQ et **données structurées** Product.", Date: "2026-07-08", Status: "done", Category: "content", Impact: "+34 % de clics sur les collections"},
			{Title: "Optimisation des images (WebP + lazy loading)", Date: "2026-08-02", Status: "done", Category: "technical"},
			{Title: "Création de contenus « guides d'achat » pour les IA", Description: "Pages questions/réponses citables.", Date: "2026-09-10", Status: "in_progress", Category: "geo"},
			{Title: "Campagne de netlinking presse déco", Date: "2026-10-15", Status: "planned", Category: "netlinking"},
		},
		Recommendations: []model.Recommendation{
			{Title: "Corriger le LCP mobile des fiches produits", Description: "Précharger l'image principale et réduire le JavaScript tiers.", Priority: "high", Effort: "medium", Category: "technical"},
			{Title: "Publier 6 guides d'achat orientés questions", Description: "Format FAQ, réponses courtes et sourcées pour maximiser les citations IA.", Priority: "high", Effort: "high", Category: "geo"},
			{Title: "Réécrire les balises title des pages à fort potentiel", Priority: "medium", Effort: "low", Category: "seo"},
			{Title: "Mettre en place un suivi des citations IA mensuel", Priority: "low", Effort: "low", Category: "tracking"},
		},
		SectionNotes: map[string]string{
			"search_console": "La hausse des impressions s'explique par l'indexation des **nouvelles collections**.",
			"geo":            "ChatGPT représente désormais plus de 60 % du trafic IA.",
		},
	}
	r.Sections = []model.TextSection{
		{ID: "methodo", Title: "Méthodologie", Body: "Les données proviennent de :\n\n| Source | Période | Remarque |\n|---|---|---|\n| Search Console | T3 2026 | Export complet |\n| GA4 | T3 2026 | Hors trafic interne |\n\n<script>alert('x')</script>\n\n[lien dangereux](javascript:alert(1))", After: "start", Style: "info"},
		{Title: "Focus saisonnalité", Body: "L'été est traditionnellement plus calme pour la décoration intérieure.", After: "search_console", Style: "note"},
		{Title: "Prochain rendez-vous", Body: "Point trimestriel prévu le **15 janvier 2027**.", Style: "success"},
	}
	r.Options = &model.Options{MaxTableRows: 10}
	if lang == "en" {
		r.Meta.PreparedBy = "Zenith Agency"
	}
	return r
}
