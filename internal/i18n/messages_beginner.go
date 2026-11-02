package i18n

// Texts of the beginner-friendly part 1: color legend, channel scorecard,
// big-number tiles, plain-language KPI titles, weekly charts. They live in
// their own file and are merged into the catalog (messages) at init.
//
// French strings use a no-break space ( ) before ":" and "?".

var beginnerMessages = map[string][2]string{
	// Color code, shown once at the top of part 1.
	"legend.title": {"Code couleur", "Color code"},
	"legend.pos":   {"Bonne nouvelle", "Good news"},
	"legend.neg":   {"À surveiller", "Needs attention"},
	"legend.exp":   {"Attendu, normal", "Expected, normal"},
	"legend.neu":   {"Stable", "Stable"},

	// Scorecard: one row per channel.
	"score.title": {"Canal par canal", "Channel by channel"},
	"score.sub":   {"Chaque ligne compare la période à la précédente.", "Each row compares the period with the previous one."},

	"score.google.name":   {"Google", "Google"},
	"score.google.up":     {"Plus de visites depuis Google", "More visits from Google"},
	"score.google.down":   {"Moins de visites depuis Google", "Fewer visits from Google"},
	"score.google.stable": {"Autant de visites depuis Google", "As many visits from Google"},
	"score.google.none":   {"Visites reçues depuis Google", "Visits received from Google"},
	"score.google.new":    {"Premières visites depuis Google", "First visits from Google"},

	"score.site.name":   {"Votre site", "Your website"},
	"score.site.up":     {"Plus de visites sur votre site", "More visits to your website"},
	"score.site.down":   {"Moins de visites sur votre site", "Fewer visits to your website"},
	"score.site.stable": {"Fréquentation du site stable", "Website traffic is stable"},
	"score.site.none":   {"Visites sur votre site", "Visits to your website"},
	"score.site.new":    {"Premières visites mesurées", "First visits measured"},

	"score.ai.name":   {"Assistants IA", "AI assistants"},
	"score.ai.up":     {"Les IA vous envoient plus de visiteurs", "AI assistants send you more visitors"},
	"score.ai.down":   {"Les IA vous envoient moins de visiteurs", "AI assistants send you fewer visitors"},
	"score.ai.stable": {"Visites depuis les IA stables", "Visits from AI are stable"},
	"score.ai.none":   {"Visiteurs envoyés par les IA", "Visitors sent by AI assistants"},
	"score.ai.new":    {"Premières visites venues des IA", "First visits from AI assistants"},
	"score.ai.cited":  {"Part des réponses IA testées qui vous citent", "Share of tested AI answers citing you"},

	"score.bing.name":   {"Bing", "Bing"},
	"score.bing.up":     {"Plus de visites depuis Bing", "More visits from Bing"},
	"score.bing.down":   {"Moins de visites depuis Bing", "Fewer visits from Bing"},
	"score.bing.stable": {"Autant de visites depuis Bing", "As many visits from Bing"},
	"score.bing.none":   {"Visites reçues depuis Bing", "Visits received from Bing"},
	"score.bing.new":    {"Premières visites depuis Bing", "First visits from Bing"},

	"score.shopping.name":   {"Google Shopping", "Google Shopping"},
	"score.shopping.up":     {"Vos produits attirent plus de clics", "Your products get more clicks"},
	"score.shopping.down":   {"Vos produits attirent moins de clics", "Your products get fewer clicks"},
	"score.shopping.stable": {"Clics sur vos produits stables", "Product clicks are stable"},
	"score.shopping.none":   {"Clics sur vos produits dans Google", "Clicks on your products in Google"},
	"score.shopping.new":    {"Premiers clics sur vos produits", "First clicks on your products"},

	"score.gbp.name":   {"Fiche Google", "Google profile"},
	"score.gbp.up":     {"Votre fiche Google est plus vue", "Your Google profile is seen more"},
	"score.gbp.down":   {"Votre fiche Google est moins vue", "Your Google profile is seen less"},
	"score.gbp.stable": {"Vues de votre fiche stables", "Profile views are stable"},
	"score.gbp.none":   {"Vues de votre fiche Google", "Views of your Google profile"},
	"score.gbp.new":    {"Premières vues de votre fiche", "First views of your profile"},

	"score.gbp_actions.name":   {"Fiche Google", "Google profile"},
	"score.gbp_actions.up":     {"Plus d’appels, d’itinéraires et de visites", "More calls, directions and visits"},
	"score.gbp_actions.down":   {"Moins d’appels, d’itinéraires et de visites", "Fewer calls, directions and visits"},
	"score.gbp_actions.stable": {"Actions sur votre fiche stables", "Profile actions are stable"},
	"score.gbp_actions.none":   {"Actions depuis votre fiche Google", "Actions from your Google profile"},
	"score.gbp_actions.new":    {"Premières actions sur votre fiche", "First actions on your profile"},

	"unit.visits.one":    {"%s visite", "%s visit"},
	"unit.visits.other":  {"%s visites", "%s visits"},
	"unit.clicks.one":    {"%s clic", "%s click"},
	"unit.clicks.other":  {"%s clics", "%s clicks"},
	"unit.views.one":     {"%s vue", "%s view"},
	"unit.views.other":   {"%s vues", "%s views"},
	"unit.actions.one":   {"%s action", "%s action"},
	"unit.actions.other": {"%s actions", "%s actions"},
	"unit.cited":         {"de réponses citant le site", "of answers citing the site"},

	// Big-number tiles.
	"tiles.title":         {"Ce qu’il faut retenir", "Key takeaways"},
	"tiles.win":           {"Bonne nouvelle", "Good news"},
	"tiles.watch":         {"À surveiller", "To watch"},
	"tiles.places":        {"%s places", "%s places"},
	"tiles.vs":            {"%s contre %s", "%s vs %s"},
	"tiles.vs_before":     {"avant : %s", "before: %s"},
	"tiles.via":           {"via %s", "via %s"},
	"tiles.ai_title":      {"Visites et citations des IA", "AI visits and citations"},
	"tiles.ai_share":      {"Part de votre trafic", "Share of your traffic"},
	"tiles.ai_share.sub":  {"des visites du site viennent des IA", "of the site’s visits come from AI"},
	"tiles.ai_cited":      {"Réponses IA qui vous citent", "AI answers citing you"},
	"tiles.ai_cited.sub":  {"questions testées dans ChatGPT, Perplexity, Gemini…", "questions tested in ChatGPT, Perplexity, Gemini…"},
	"tiles.copilot":       {"Citations dans Copilot", "Citations in Copilot"},
	"tiles.copilot.sub":   {"fois où l’IA de Microsoft cite votre site", "times Microsoft’s AI cites your site"},
	"tiles.copilot.first": {"premières citations de votre site", "first citations of your site"},

	// Highlights: a few in part 1, the full lists in part 2.
	"summary.more.one":   {"+ %s autre dans la partie 2", "+ %s more in part 2"},
	"summary.more.other": {"+ %s autres dans la partie 2", "+ %s more in part 2"},
	"sec.review.title":   {"Points forts et points de vigilance", "Highlights and points of attention"},
	"sec.review.intro": {
		"La liste complète des points forts et des points de vigilance relevés sur la période.",
		"The complete list of highlights and points of attention of the period.",
	},

	// Before / after mini bars on KPI cards without daily data.
	"mini.before": {"Avant", "Before"},

	// Expected changes (dilution): a pill on the card, one line under the grid.
	"kpi.expected_pill": {"Normal", "Expected"},
	"kpi.expected_line": {
		"%s : normal — le site apparaît sur beaucoup de nouvelles recherches, souvent plus bas au début.",
		"%s: expected — the site now shows up on many new searches, often lower down at first.",
	},
	"list.and": {" et ", " and "},
	"mini.now": {"Maintenant", "Now"},

	// Plain-language KPI titles; the technical term is shown below.
	"kpi.clicks.plain":           {"Visites depuis Google", "Visits from Google"},
	"kpi.clicks.term":            {"Clics (Search Console)", "Clicks (Search Console)"},
	"kpi.impressions.plain":      {"Affichages dans Google", "Appearances in Google"},
	"kpi.impressions.term":       {"Impressions", "Impressions"},
	"kpi.ctr.plain":              {"Taux de clic", "Click rate"},
	"kpi.ctr.term":               {"CTR", "CTR"},
	"kpi.ctr.per100":             {"Sur 100 affichages dans Google, environ %s deviennent une visite.", "Out of 100 appearances in Google, about %s turn into a visit."},
	"kpi.ctr.per1000":            {"Sur 1 000 affichages dans Google, environ %s deviennent une visite.", "Out of 1,000 appearances in Google, about %s turn into a visit."},
	"kpi.position.plain":         {"Classement moyen", "Average ranking"},
	"kpi.position.term":          {"Position moyenne", "Average position"},
	"kpi.organic_sessions.plain": {"Visites via les moteurs", "Visits via search engines"},
	"kpi.organic_sessions.term":  {"Sessions organiques (GA4)", "Organic sessions (GA4)"},
	"kpi.sessions.plain":         {"Visites sur le site", "Website visits"},
	"kpi.sessions.term":          {"Sessions (GA4)", "Sessions (GA4)"},
	"kpi.users.plain":            {"Visiteurs", "Visitors"},
	"kpi.users.term":             {"Utilisateurs actifs", "Active users"},
	"kpi.key_events.plain":       {"Contacts et conversions", "Leads and conversions"},
	"kpi.key_events.term":        {"Événements clés (GA4)", "Key events (GA4)"},
	"kpi.engagement_rate.plain":  {"Visites actives", "Engaged visits"},
	"kpi.engagement_rate.term":   {"Taux d’engagement", "Engagement rate"},
	"kpi.ai_sessions.plain":      {"Visites depuis les IA", "Visits from AI"},
	"kpi.ai_sessions.term":       {"Sessions référées par les IA", "AI-referred sessions"},
	"kpi.indexed.plain":          {"Pages connues de Google", "Pages known to Google"},
	"kpi.indexed.term":           {"Pages indexées", "Indexed pages"},
	"kpi.bing_clicks.plain":      {"Visites depuis Bing", "Visits from Bing"},
	"kpi.bing_clicks.term":       {"Clics (Bing Webmaster Tools)", "Clicks (Bing Webmaster Tools)"},
	"kpi.bing_impressions.plain": {"Affichages dans Bing", "Appearances in Bing"},
	"kpi.bing_impressions.term":  {"Impressions Bing", "Bing impressions"},
	"kpi.bing_ctr.plain":         {"Taux de clic Bing", "Bing click rate"},
	"kpi.bing_ctr.term":          {"CTR Bing", "Bing CTR"},
	"kpi.bing_position.plain":    {"Classement moyen Bing", "Average ranking in Bing"},
	"kpi.bing_position.term":     {"Position moyenne Bing", "Bing average position"},

	// Weekly charts (tiny volumes, long periods in part 1).
	"chart.clicks_weekly":      {"Visites depuis Google par semaine", "Visits from Google per week"},
	"chart.impressions_weekly": {"Affichages par semaine", "Appearances per week"},
	"chart.position_weekly":    {"Position moyenne par semaine", "Average position per week"},
	"chart.ctr_weekly":         {"Taux de clic par semaine", "Click-through rate per week"},
	"chart.sessions_weekly":    {"Visites par semaine", "Visits per week"},
	"chart.ai_weekly":          {"Visites depuis les IA par semaine", "Visits from AI per week"},
	"chart.bing_clicks_weekly": {"Visites depuis Bing par semaine", "Visits from Bing per week"},
	"chart.weekly_sub":         {"Une valeur par semaine (survolez un point pour les dates)", "One value per week (hover a point for the dates)"},

	// Short chart captions of part 1 (one sentence).
	"hint.weekly": {
		"Avec peu de visites par jour, la courbe jour par jour fait des dents de scie : chaque point additionne ici une semaine.",
		"With few visits a day, a daily line zig-zags: each point here adds up one week.",
	},
	"hint.clicks_glance": {
		"Au-dessus de la ligne grise (la période précédente), vous faites mieux qu’avant.",
		"Above the grey line (the previous period), you are doing better than before.",
	},
	"hint.clicks_weekly_glance": {
		"Un point par semaine ; au-dessus de la ligne grise (période précédente), vous faites mieux qu’avant.",
		"One point per week; above the grey line (previous period), you are doing better than before.",
	},
	"hint.sessions_weekly_glance": {
		"Un point par semaine ; la ligne grise montre la période précédente.",
		"One point per week; the grey line shows the previous period.",
	},
	"hint.channels_glance": {
		"D’où viennent vos visiteurs : Google, accès direct, réseaux sociaux, IA…",
		"Where your visitors come from: Google, direct visits, social networks, AI…",
	},
}

// catalogDuplicates lists the keys defined twice across the message files
// (checked by the tests).
var catalogDuplicates []string

func init() {
	for k, v := range beginnerMessages {
		if _, dup := messages[k]; dup {
			catalogDuplicates = append(catalogDuplicates, k)
		}
		messages[k] = v
	}
}
