package example

import (
	"fmt"
	"math"
	"strings"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// growth formats the relative change of m as "+34 %" (fr) or "+34%" (en).
func growth(m model.Metric, lang string) string {
	pct, ok := m.DeltaPct()
	if !ok {
		return "–"
	}
	s := fmt.Sprintf("%+.0f", math.Round(pct))
	if lang == "en" {
		return s + "%"
	}
	return s + "\u202f%"
}

// num formats an integer with the language's thousands separator.
func num(v float64, lang string) string {
	s := fmt.Sprintf("%.0f", math.Abs(math.Round(v)))
	sep := ","
	if lang != "en" {
		sep = "\u202f"
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteString(sep)
		}
		b.WriteRune(c)
	}
	if v < 0 {
		return "-" + b.String()
	}
	return b.String()
}

func narrative(lang string, r *model.Report) *model.Narrative {
	sc, ga := r.SearchConsole, r.Analytics
	clicks := growth(sc.Totals.Clicks, lang)
	impr := growth(sc.Totals.Impressions, lang)
	var aiCur, aiPrev float64
	for _, d := range ga.Daily {
		aiCur += d.AISessions
	}
	for _, d := range ga.PreviousDaily {
		aiPrev += d.AISessions
	}
	ai := growth(model.M(aiCur, aiPrev), lang)
	keyEv := growth(*ga.Totals.KeyEvents, lang)
	revenue := growth(*ga.Totals.Revenue, lang)

	if lang == "en" {
		return &model.Narrative{
			ExecutiveSummary: fmt.Sprintf(`Q3 2026 confirms the momentum started in the spring: **Google clicks are up %s** (%s clicks) and impressions are up %s, driven by the new category guides and the redesign of the pendant light pages.

Traffic from **AI assistants** (ChatGPT, Perplexity, Gemini…) grew by **%s**: still a small channel, but its visitors are among the most engaged and they already generate sales. Maison Lumen is now cited in 2 out of 3 tested AI answers.

On the business side, conversions are up %s and organic revenue follows the same trend (%s for all channels). The main point of attention remains **mobile speed**: 14%% of mobile URLs are rated "poor" in Core Web Vitals.`,
				clicks, num(sc.Totals.Clicks.Current, lang), impr, ai, keyEv, revenue),
			Highlights: []string{
				"4 strategic queries entered Google's top 3, including “suspension rotin” and “lampe de chevet tactile”.",
				"The “How to choose a pendant light” guide nearly tripled its traffic and is cited by Perplexity and Google AI Overviews.",
				"ChatGPT has become the leading AI source of visits, with an engagement rate above the site average.",
				"Indexed pages went from 1,610 to 1,840 after the sitemap clean-up.",
			},
			Concerns: []string{
				"Click-through rate is slipping because many new impressions come from queries still ranked on page 2.",
				"250 mobile URLs have poor Core Web Vitals, mostly product pages with heavy image galleries.",
				"The site lost its Perplexity citation on “luminaire scandinave pas cher” after the category merge.",
			},
			Actions: []model.Action{
				{Title: "Redesign of the pendant light category pages", Description: "New buying-guide introduction, FAQ block and internal links to the best-selling products.", Date: "2026-07-08", Status: "done", Category: "content", Impact: "+71 % clicks on /suspensions/rotin"},
				{Title: "Publication of two buying guides", Description: "*How to choose a pendant light* and *Colour temperature guide*, with structured FAQ data.", Date: "2026-07-22", Status: "done", Category: "geo", Impact: "Cited by Perplexity, Gemini and AI Overviews"},
				{Title: "Sitemap and canonical clean-up", Description: "Removal of redirected URLs from sitemaps, canonical tags on filtered pages.", Date: "2026-08-05", Status: "done", Category: "technical", Impact: "−90 non-indexed pages"},
				{Title: "Image optimisation on product pages", Description: "AVIF conversion and lazy loading of galleries to improve LCP on mobile.", Date: "2026-09-15", Status: "in_progress", Category: "technical"},
				{Title: "Digital PR campaign with design magazines", Description: "Three partnerships signed, articles expected in October.", Status: "in_progress", Category: "netlinking"},
				{Title: "llms.txt file and brand fact sheet", Description: "Give AI engines a clear, up-to-date description of the brand, delivery terms and best-sellers.", Status: "planned", Category: "geo"},
			},
			Recommendations: []model.Recommendation{
				{Title: "Rewrite titles and meta descriptions of the 20 most viewed pages", Description: "Impressions grow faster than clicks: highlight prices, free delivery and the French design in snippets.", Priority: "high", Effort: "low", Category: "seo"},
				{Title: "Fix Core Web Vitals on mobile product pages", Description: "Target LCP < 2.5 s: compress gallery images, preload the main image, defer third-party scripts.", Priority: "high", Effort: "medium", Category: "technical"},
				{Title: "Create comparison content for AI answers", Description: "Pages such as *Rattan vs wicker pendant* or *Which floor lamp for a cosy living room?* answer the prompts where competitors are cited.", Priority: "medium", Effort: "medium", Category: "geo"},
				{Title: "Restore the Scandinavian category page", Description: "Re-create a dedicated landing page to win back the lost Perplexity citation and the associated queries.", Priority: "medium", Effort: "low", Category: "content"},
				{Title: "Track AI assistants as a dedicated GA4 channel", Description: "Create a custom channel group so AI traffic is reported separately from Referral.", Priority: "low", Effort: "low", Category: "tracking"},
			},
			SectionNotes: map[string]string{
				"geo":       "AI traffic is detected from GA4 sources (chatgpt.com, perplexity.ai…). Citation checks were run manually on **25 September 2026** with logged-out accounts.",
				"technical": "The drop in non-indexed pages follows the sitemap clean-up of August 5. Mobile Core Web Vitals remain the priority for Q4.",
				"queries":   "Queries are shown as typed by users, in French.",
			},
		}
	}

	summary := fmt.Sprintf(`Le troisième trimestre 2026 confirme la dynamique engagée au printemps : **les clics Google progressent de %s** (%s clics) et les impressions de %s, portés par les nouveaux guides d'achat et la refonte des pages suspensions.

Le trafic venant des **assistants IA** (ChatGPT, Perplexity, Gemini…) progresse de **%s** : c'est encore un petit canal, mais ses visiteurs sont parmi les plus engagés et génèrent déjà des ventes. Maison Lumen est désormais cité dans 2 réponses IA testées sur 3.

Côté business, les conversions progressent de %s et le chiffre d'affaires de %s tous canaux confondus. Le principal point de vigilance reste **la vitesse sur mobile** : 14 %% des URL mobiles sont classées « médiocres » en Core Web Vitals.`,
		clicks, num(sc.Totals.Clicks.Current, lang), impr, ai, keyEv, revenue)
	summary = strings.ReplaceAll(summary, " %", "\u202f%")

	return &model.Narrative{
		ExecutiveSummary: summary,
		Highlights: []string{
			"4 requêtes stratégiques entrent dans le top 3 de Google, dont « suspension rotin » et « lampe de chevet tactile ».",
			"Le guide « Comment choisir une suspension » a presque triplé son trafic et il est cité par Perplexity et les AI Overviews de Google.",
			"ChatGPT devient la première source de visites IA, avec un taux d'engagement supérieur à la moyenne du site.",
			"Les pages indexées passent de 1 610 à 1 840 après le nettoyage des sitemaps.",
		},
		Concerns: []string{
			"Le taux de clic s'érode, car beaucoup de nouvelles impressions viennent de requêtes encore classées en page 2.",
			"250 URL mobiles ont des Core Web Vitals médiocres, surtout des fiches produits aux galeries d'images lourdes.",
			"Le site a perdu sa citation Perplexity sur « luminaire scandinave pas cher » après la fusion de catégories.",
		},
		Actions: []model.Action{
			{Title: "Refonte des pages catégories suspensions", Description: "Nouvelle introduction façon guide d'achat, bloc FAQ et maillage interne vers les meilleures ventes.", Date: "2026-07-08", Status: "done", Category: "content", Impact: "+71 % de clics sur /suspensions/rotin"},
			{Title: "Publication de deux guides d'achat", Description: "*Comment choisir une suspension* et *Guide de la température de couleur*, avec données structurées FAQ.", Date: "2026-07-22", Status: "done", Category: "geo", Impact: "Cités par Perplexity, Gemini et les AI Overviews"},
			{Title: "Nettoyage des sitemaps et des balises canoniques", Description: "Retrait des URL redirigées des sitemaps, balises canoniques sur les pages filtrées.", Date: "2026-08-05", Status: "done", Category: "technical", Impact: "−90 pages non indexées"},
			{Title: "Optimisation des images des fiches produits", Description: "Conversion en AVIF et chargement différé des galeries pour améliorer le LCP sur mobile.", Date: "2026-09-15", Status: "in_progress", Category: "technical"},
			{Title: "Campagne de relations presse digitales", Description: "Trois partenariats signés avec des magazines déco, articles attendus en octobre.", Status: "in_progress", Category: "netlinking"},
			{Title: "Fichier llms.txt et fiche d'identité de la marque", Description: "Donner aux moteurs IA une description claire et à jour de la marque, des conditions de livraison et des best-sellers.", Status: "planned", Category: "geo"},
		},
		Recommendations: []model.Recommendation{
			{Title: "Réécrire les titres et meta descriptions des 20 pages les plus vues", Description: "Les impressions progressent plus vite que les clics : mettez en avant prix, livraison offerte et design français dans les extraits.", Priority: "high", Effort: "low", Category: "seo"},
			{Title: "Corriger les Core Web Vitals des fiches produits sur mobile", Description: "Objectif LCP < 2,5 s : compresser les galeries, précharger l'image principale, différer les scripts tiers.", Priority: "high", Effort: "medium", Category: "technical"},
			{Title: "Créer des contenus comparatifs pour les réponses IA", Description: "Des pages comme *Suspension rotin ou osier ?* ou *Quel lampadaire pour un salon cosy ?* répondent aux prompts où les concurrents sont cités.", Priority: "medium", Effort: "medium", Category: "geo"},
			{Title: "Recréer la page catégorie scandinave", Description: "Une page dédiée permettra de regagner la citation Perplexity perdue et les requêtes associées.", Priority: "medium", Effort: "low", Category: "content"},
			{Title: "Suivre les assistants IA comme un canal GA4 dédié", Description: "Créer un groupe de canaux personnalisé pour distinguer le trafic IA du trafic Referral.", Priority: "low", Effort: "low", Category: "tracking"},
		},
		SectionNotes: map[string]string{
			"geo":       "Le trafic IA est détecté à partir des sources GA4 (chatgpt.com, perplexity.ai…). Les tests de citation ont été réalisés manuellement le **25 septembre 2026**, sans être connecté.",
			"technical": "La baisse des pages non indexées fait suite au nettoyage des sitemaps du 5 août. Les Core Web Vitals mobiles restent la priorité du T4.",
			"queries":   "Les requêtes sont affichées telles que tapées par les internautes.",
		},
	}
}

func sections(lang string) []model.TextSection {
	if lang == "en" {
		return []model.TextSection{
			{
				ID:    "geo-method",
				Title: "How we measure visibility in AI assistants",
				After: "geo",
				Style: "info",
				Body: `AI assistants do not provide a "Search Console". We therefore combine three sources:

1. **Visits**: GA4 sessions whose source is an AI assistant (chatgpt.com, perplexity.ai, gemini.google.com…).
2. **Citations**: 12 representative questions asked by hand to each engine, checking whether *Maison Lumen* is cited with a link.
3. **Crawlers**: hits from AI robots (GPTBot, ClaudeBot, PerplexityBot…) in the server logs.

Answers vary from one session to another: these checks are indicators, not exact measurements.`,
			},
			{
				ID:    "q4-plan",
				Title: "Q4 2026 action plan",
				After: "recommendations",
				Style: "plain",
				Body: `| Action | Deadline | Owner | Expected impact |
|---|---|---|---|
| Titles & meta descriptions (top 20 pages) | Oct 15 | Studio Halo | +10 % CTR |
| Mobile image optimisation | Oct 31 | Maison Lumen dev team | LCP < 2.5 s |
| Comparison content (4 pages) | Nov 15 | Studio Halo | New AI citations |
| Scandinavian category page | Nov 30 | Maison Lumen | Recover lost queries |
| GA4 "AI assistants" channel | Dec 5 | Studio Halo | Reliable AI reporting |

Priorities will be reviewed at the next monthly meeting.`,
			},
		}
	}
	return []model.TextSection{
		{
			ID:    "geo-method",
			Title: "Comment nous mesurons la visibilité dans les assistants IA",
			After: "geo",
			Style: "info",
			Body: `Les assistants IA ne proposent pas de « Search Console ». Nous croisons donc trois sources :

1. **Les visites** : sessions GA4 dont la source est un assistant IA (chatgpt.com, perplexity.ai, gemini.google.com…).
2. **Les citations** : 12 questions représentatives posées à la main à chaque moteur, en vérifiant si *Maison Lumen* est cité avec un lien.
3. **Les robots** : passages des robots des IA (GPTBot, ClaudeBot, PerplexityBot…) dans les journaux du serveur.

Les réponses varient d'une session à l'autre : ces tests sont des indicateurs, pas des mesures exactes.`,
		},
		{
			ID:    "q4-plan",
			Title: "Plan d'action du T4 2026",
			After: "recommendations",
			Style: "plain",
			Body: `| Action | Échéance | Responsable | Impact attendu |
|---|---|---|---|
| Titres et meta descriptions (20 pages) | 15 octobre | Studio Halo | +10 % de CTR |
| Optimisation des images mobiles | 31 octobre | Équipe dev Maison Lumen | LCP < 2,5 s |
| Contenus comparatifs (4 pages) | 15 novembre | Studio Halo | Nouvelles citations IA |
| Page catégorie scandinave | 30 novembre | Maison Lumen | Regagner les requêtes perdues |
| Canal GA4 « Assistants IA » | 5 décembre | Studio Halo | Suivi fiable du trafic IA |

Les priorités seront revues lors du prochain point mensuel.`,
		},
	}
}
