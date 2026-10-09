package i18n

// GlossaryEntry is one term of the report glossary.
type GlossaryEntry struct {
	Key        string
	Term       string
	Definition string
}

var glossary = []struct {
	key            string
	termFR, termEN string
	defFR, defEN   string
}{
	{"clicks", "Clics", "Clicks",
		"Nombre de fois où un internaute a cliqué sur un lien vers votre site depuis les résultats de Google.",
		"Number of times someone clicked a link to your site from Google search results."},
	{"impressions", "Impressions", "Impressions",
		"Nombre de fois où un lien vers votre site s’est affiché dans les résultats de Google, qu’il ait été cliqué ou non.",
		"Number of times a link to your site was shown in Google results, whether it was clicked or not."},
	{"ctr", "CTR (taux de clic)", "CTR (click-through rate)",
		"Clics divisés par impressions. Un CTR de 3 % signifie que 3 personnes sur 100 qui voient votre site dans Google cliquent dessus.",
		"Clicks divided by impressions. A 3% CTR means 3 out of 100 people who see your site in Google click on it."},
	{"position", "Position moyenne", "Average position",
		"Rang moyen de votre site dans les résultats de Google (1 = tout en haut). Plus le chiffre est petit, mieux c’est.",
		"Average rank of your site in Google results (1 = the very top). The smaller the number, the better."},
	{"top3", "Top 3 / Top 10", "Top 3 / Top 10",
		"Requêtes pour lesquelles vous apparaissez dans les 3 premiers résultats, ou sur la première page (10 premiers résultats). L’essentiel des clics se fait là.",
		"Queries for which you appear in the first 3 results, or on the first page (first 10 results). Most clicks happen there."},
	{"branded", "Requête de marque", "Branded query",
		"Recherche contenant le nom de votre marque ou de votre site. Les autres requêtes sont dites « hors marque » et reflètent votre capacité à attirer de nouveaux visiteurs.",
		"A search containing your brand or site name. Other queries are “non-branded” and reflect your ability to attract new visitors."},
	{"session", "Session", "Session",
		"Une visite sur votre site : elle commence à l’arrivée du visiteur et se termine après 30 minutes d’inactivité.",
		"A visit to your site: it starts when the visitor arrives and ends after 30 minutes of inactivity."},
	{"engaged", "Session engagée", "Engaged session",
		"Visite de plus de 10 secondes, ou avec au moins 2 pages vues, ou avec un événement clé. C’est le signe d’une visite utile.",
		"A visit lasting over 10 seconds, or with at least 2 page views, or with a key event. It signals a useful visit."},
	{"engagement_rate", "Taux d’engagement", "Engagement rate",
		"Part des sessions engagées parmi toutes les sessions.",
		"Share of engaged sessions among all sessions."},
	{"key_event", "Événement clé", "Key event",
		"Action importante définie dans Google Analytics (achat, demande de devis, formulaire de contact, inscription…), autrefois appelée « conversion ».",
		"An important action defined in Google Analytics (purchase, quote request, contact form, sign-up…), formerly called a “conversion”."},
	{"organic", "Recherche organique", "Organic search",
		"Trafic venu des résultats naturels (non payants) des moteurs de recherche comme Google ou Bing.",
		"Traffic coming from the natural (unpaid) results of search engines such as Google or Bing."},
	{"landing", "Page d’entrée", "Landing page",
		"Première page consultée lors d’une visite.",
		"The first page viewed during a visit."},
	{"seo", "SEO", "SEO",
		"Search Engine Optimization : l’ensemble des techniques pour améliorer la visibilité d’un site dans les résultats naturels des moteurs de recherche.",
		"Search Engine Optimization: all the techniques used to improve a site’s visibility in the natural results of search engines."},
	{"geo", "GEO", "GEO",
		"Generative Engine Optimization : l’optimisation de la présence d’une marque dans les réponses des moteurs génératifs (ChatGPT, Perplexity, Gemini, Copilot, AI Overviews…).",
		"Generative Engine Optimization: optimizing a brand’s presence in the answers of generative engines (ChatGPT, Perplexity, Gemini, Copilot, AI Overviews…)."},
	{"aio", "AI Overviews", "AI Overviews",
		"Résumés générés par l’intelligence artificielle de Google, affichés au-dessus des résultats pour certaines recherches, avec des liens vers les sources citées.",
		"AI-generated summaries shown by Google above the results for some searches, with links to the cited sources."},
	{"ai_referral", "Visite depuis une IA", "AI referral",
		"Visite arrivée sur votre site en cliquant sur un lien dans la réponse d’un assistant IA (par exemple chatgpt.com ou perplexity.ai dans Google Analytics).",
		"A visit that reached your site through a link in an AI assistant’s answer (for example chatgpt.com or perplexity.ai in Google Analytics)."},
	{"citation", "Citation", "Citation",
		"Votre site est cité quand un assistant IA affiche un lien vers l’une de vos pages comme source de sa réponse. Une mention sans lien est une simple « mention ».",
		"Your site is cited when an AI assistant shows a link to one of your pages as a source of its answer. A mention without a link is just a “mention”."},
	{"sov", "Part de voix", "Share of voice",
		"Part des réponses des IA sur votre thématique qui mentionnent votre marque, comparée à celle de vos concurrents.",
		"Share of AI answers on your topic that mention your brand, compared with your competitors."},
	{"crawler", "Robot d’exploration IA", "AI crawler",
		"Programme qui parcourt votre site pour le compte d’une IA (GPTBot, ClaudeBot, PerplexityBot, Google-Extended…) afin d’en lire le contenu.",
		"A program that browses your site on behalf of an AI (GPTBot, ClaudeBot, PerplexityBot, Google-Extended…) to read its content."},
	{"indexing", "Indexation", "Indexing",
		"Enregistrement d’une page dans la base de Google. Seules les pages indexées peuvent apparaître dans les résultats.",
		"Storing a page in Google’s database. Only indexed pages can appear in search results."},
	{"sitemap", "Sitemap", "Sitemap",
		"Fichier qui liste les pages de votre site pour aider les moteurs de recherche à les découvrir.",
		"A file listing your site’s pages to help search engines discover them."},
	{"cwv", "Core Web Vitals", "Core Web Vitals",
		"Indicateurs de Google mesurant l’expérience réelle des visiteurs : vitesse d’affichage (LCP), réactivité (INP) et stabilité visuelle (CLS).",
		"Google metrics measuring real visitor experience: loading speed (LCP), responsiveness (INP) and visual stability (CLS)."},
	{"delta", "Évolution (Δ)", "Change (Δ)",
		"Différence entre la période analysée et la période de comparaison, en valeur ou en pourcentage. Pour les taux, l’écart est exprimé en points (pt).",
		"Difference between the analysed period and the comparison period, as a value or a percentage. For rates, the gap is expressed in points (pts)."},
}

// Glossary returns the glossary in the requested language.
func Glossary(lang string) []GlossaryEntry {
	en := Lang(lang) == "en"
	out := make([]GlossaryEntry, 0, len(glossary))
	for _, g := range glossary {
		e := GlossaryEntry{Key: g.key, Term: g.termFR, Definition: g.defFR}
		if en {
			e.Term, e.Definition = g.termEN, g.defEN
		}
		out = append(out, e)
	}
	return out
}
