package i18n

import "strings"

// Google Business Profile texts. They live in their own file and are merged
// into the catalog (messages) and the glossary at init.

var gbpMessages = map[string][2]string{
	"src.gbp": {"Google Business Profile", "Google Business Profile"},

	"sec.business_profile.title": {"Fiche Google (Google Business Profile)", "Google Business Profile (local visibility)"},
	"sec.business_profile.intro": {
		"Votre fiche Google, c’est la carte de votre établissement qui s’affiche dans la recherche Google et dans Google Maps : adresse, horaires, téléphone, photos et avis clients. C’est souvent le premier contact d’un client du quartier : il vous trouve, lit les avis, puis appelle, demande l’itinéraire ou visite votre site. Cette partie montre combien de personnes ont vu la fiche, ce qu’elles ont fait ensuite et ce que disent vos clients.",
		"Your Google profile is the card of your business shown in Google Search and Google Maps: address, opening hours, phone number, photos and customer reviews. It is often the first contact with nearby customers: they find you, read the reviews, then call, ask for directions or visit your website. This part shows how many people saw the profile, what they did next and what your customers say.",
	},

	// KPI cards.
	"kpi.gbp_views.label":            {"Vues de la fiche Google", "Google profile views"},
	"kpi.gbp_views.explain":          {"Nombre de personnes qui ont vu votre fiche dans Google Search ou Maps.", "Number of people who saw your profile in Google Search or Maps."},
	"kpi.gbp_interactions.label":     {"Actions sur la fiche Google", "Google profile actions"},
	"kpi.gbp_interactions.explain":   {"Appels, demandes d’itinéraire, clics vers le site, messages et réservations depuis votre fiche.", "Calls, direction requests, website clicks, messages and bookings from your profile."},
	"kpi.gbp_calls.label":            {"Appels depuis la fiche", "Calls from the profile"},
	"kpi.gbp_calls.explain":          {"Clients qui vous ont appelé en touchant le bouton « Appeler ».", "Customers who called you with the “Call” button."},
	"kpi.gbp_website_clicks.label":   {"Clics vers le site", "Website clicks"},
	"kpi.gbp_website_clicks.explain": {"Visites de votre site depuis la fiche Google.", "Visits to your website from the Google profile."},
	"kpi.gbp_directions.label":       {"Demandes d’itinéraire", "Direction requests"},
	"kpi.gbp_directions.explain":     {"Personnes qui ont demandé le chemin pour venir chez vous.", "People who asked for directions to your place."},
	"kpi.gbp_rating.label":           {"Note Google", "Google rating"},
	"kpi.gbp_rating.explain":         {"Note moyenne laissée par vos clients, sur 5 étoiles.", "Average rating left by your customers, out of 5 stars."},
	"kpi.gbp_reviews.label":          {"Avis Google", "Google reviews"},
	"kpi.gbp_reviews.explain":        {"Nombre total d’avis clients sur votre fiche.", "Total number of customer reviews on your profile."},
	"kpi.gbp_new_reviews.label":      {"Nouveaux avis", "New reviews"},
	"kpi.gbp_new_reviews.explain":    {"Avis reçus pendant la période.", "Reviews received during the period."},
	"kpi.gbp_response_rate.label":    {"Avis avec réponse", "Reviews replied to"},
	"kpi.gbp_response_rate.explain":  {"Part des avis auxquels vous avez répondu.", "Share of reviews you replied to."},

	// Plain-language titles (Part 1 cards) and the Business Profile term.
	"kpi.gbp_views.plain":          {"Vues de la fiche Google", "Google profile views"},
	"kpi.gbp_views.term":           {"Vues (Business Profile)", "Views (Business Profile)"},
	"kpi.gbp_interactions.plain":   {"Actions sur la fiche Google", "Google profile actions"},
	"kpi.gbp_interactions.term":    {"Interactions (fiche Google)", "Interactions (Business Profile)"},
	"kpi.gbp_calls.plain":          {"Appels depuis la fiche", "Calls from the profile"},
	"kpi.gbp_calls.term":           {"Appels (Business Profile)", "Calls (Business Profile)"},
	"kpi.gbp_directions.plain":     {"Demandes d’itinéraire", "Direction requests"},
	"kpi.gbp_directions.term":      {"Itinéraires (Business Profile)", "Directions (Business Profile)"},
	"kpi.gbp_website_clicks.plain": {"Visites du site depuis la fiche", "Website visits from the profile"},
	"kpi.gbp_website_clicks.term":  {"Clics vers le site (Business Profile)", "Website clicks (Business Profile)"},
	"kpi.gbp_rating.plain":         {"Note Google", "Google rating"},
	"kpi.gbp_rating.term":          {"Note moyenne des avis", "Average review rating"},
	"kpi.gbp_reviews.plain":        {"Avis Google", "Google reviews"},
	"kpi.gbp_reviews.term":         {"Nombre d’avis", "Number of reviews"},

	// Charts and blocks.
	"chart.gbp_views_daily":        {"Vues de la fiche Google par jour", "Daily Google profile views"},
	"chart.gbp_interactions_daily": {"Actions sur la fiche Google par jour", "Daily Google profile actions"},
	"chart.gbp_found":              {"Où les clients vous ont trouvé", "Where people found you"},
	"chart.gbp_found.sub":          {"Recherche Google ou Google Maps, sur mobile ou ordinateur", "Google Search or Google Maps, on mobile or desktop"},
	"chart.gbp_actions":            {"Ce que les clients ont fait", "What people did"},
	"chart.gbp_actions.sub":        {"Actions depuis la fiche, comparées à la période précédente", "Actions from the profile, compared with the previous period"},
	"chart.gbp_rating":             {"Avis clients", "Customer reviews"},
	"chart.gbp_distribution":       {"Répartition des notes", "Rating breakdown"},
	"chart.gbp_distribution.sub":   {"Nombre d’avis par nombre d’étoiles", "Number of reviews per star rating"},
	"chart.gbp_checklist":          {"Complétude de la fiche", "Profile completeness"},
	"chart.gbp_checklist.sub":      {"%s des informations clés sont renseignées", "%s of the key information is filled in"},
	"chart.gbp_activity":           {"Activité sur la fiche", "Profile activity"},
	"chart.gbp_activity.sub":       {"Ce que vous avez publié pendant la période", "What you published during the period"},
	"chart.gbp_views":              {"vues", "views"},

	"lbl.gbp_search_mobile":  {"Recherche Google · mobile", "Google Search · mobile"},
	"lbl.gbp_search_desktop": {"Recherche Google · ordinateur", "Google Search · desktop"},
	"lbl.gbp_maps_mobile":    {"Google Maps · mobile", "Google Maps · mobile"},
	"lbl.gbp_maps_desktop":   {"Google Maps · ordinateur", "Google Maps · desktop"},
	"lbl.gbp_search":         {"Recherche Google", "Google Search"},
	"lbl.gbp_maps":           {"Google Maps", "Google Maps"},
	"lbl.gbp_mobile":         {"Sur mobile", "On mobile"},
	"lbl.gbp_desktop":        {"Sur ordinateur", "On desktop"},
	"lbl.gbp_posts":          {"Publications", "Posts"},
	"lbl.gbp_photos":         {"Photos ajoutées", "Photos added"},
	"lbl.gbp_completeness":   {"Complétude", "Completeness"},
	"lbl.gbp_new_reviews":    {"Nouveaux avis", "New reviews"},
	"lbl.gbp_response_rate":  {"Avis avec réponse", "Replied to"},
	"lbl.gbp_total_reviews":  {"Avis au total", "Total reviews"},
	"lbl.gbp_out_of":         {"sur 5", "out of 5"},
	"lbl.gbp_reviews.one":    {"%s avis", "%s review"},
	"lbl.gbp_reviews.other":  {"%s avis", "%s reviews"},
	"lbl.gbp_stars.one":      {"%s étoile", "%s star"},
	"lbl.gbp_stars.other":    {"%s étoiles", "%s stars"},
	"lbl.gbp_stars_aria":     {"Note de %s sur 5", "Rated %s out of 5"},
	"lbl.gbp_replied":        {"Répondu", "Replied"},
	"lbl.gbp_not_replied":    {"Sans réponse", "No reply"},
	"lbl.gbp_done":           {"Renseigné", "Done"},
	"lbl.gbp_todo":           {"À compléter", "To do"},
	"lbl.gbp_below":          {"moins de %s", "fewer than %s"},

	"gbp.act.calls":              {"Appels", "Calls"},
	"gbp.act.website_clicks":     {"Clics vers le site", "Website clicks"},
	"gbp.act.direction_requests": {"Demandes d’itinéraire", "Direction requests"},
	"gbp.act.messages":           {"Messages", "Messages"},
	"gbp.act.bookings":           {"Réservations", "Bookings"},
	"gbp.act.food_orders":        {"Commandes de repas", "Food orders"},
	"gbp.act.menu_clicks":        {"Consultations du menu", "Menu views"},

	// Profile checklist items (known keys are shown in the report language).
	"gbp.check.hours":          {"Horaires d’ouverture", "Opening hours"},
	"gbp.check.special_hours":  {"Horaires exceptionnels (jours fériés)", "Special hours (holidays)"},
	"gbp.check.categories":     {"Catégories d’activité", "Business categories"},
	"gbp.check.description":    {"Description de l’établissement", "Business description"},
	"gbp.check.photos":         {"Photos récentes", "Recent photos"},
	"gbp.check.logo":           {"Logo", "Logo"},
	"gbp.check.cover_photo":    {"Photo de couverture", "Cover photo"},
	"gbp.check.products":       {"Produits", "Products"},
	"gbp.check.services":       {"Services", "Services"},
	"gbp.check.website":        {"Lien vers le site", "Website link"},
	"gbp.check.phone":          {"Numéro de téléphone", "Phone number"},
	"gbp.check.address":        {"Adresse", "Address"},
	"gbp.check.attributes":     {"Attributs (accessibilité, paiements…)", "Attributes (accessibility, payments…)"},
	"gbp.check.posts":          {"Publications régulières", "Regular posts"},
	"gbp.check.booking":        {"Lien de réservation", "Booking link"},
	"gbp.check.menu":           {"Menu", "Menu"},
	"gbp.check.opening_date":   {"Date d’ouverture", "Opening date"},
	"gbp.check.service_area":   {"Zone desservie", "Service area"},
	"gbp.check.messaging":      {"Messagerie activée", "Messaging enabled"},
	"gbp.check.qa":             {"Questions-réponses", "Questions & answers"},
	"gbp.check.review_replies": {"Réponses aux avis", "Replies to reviews"},

	// Tables.
	"table.gbp_keywords":     {"Recherches locales qui affichent votre fiche", "Local searches that show your profile"},
	"table.gbp_keywords.sub": {"Mots tapés dans Google Search et Maps avant de voir votre fiche", "Words typed in Google Search and Maps before seeing your profile"},
	"table.gbp_reviews":      {"Derniers avis clients", "Latest customer reviews"},
	"table.gbp_reviews.sub":  {"Textes tels que rédigés par les clients", "Texts as written by the customers"},
	"table.gbp_locations":    {"Détail par établissement", "Details by location"},
	"col.gbp_keyword":        {"Recherche", "Search term"},
	"col.gbp_searches":       {"Recherches", "Searches"},
	"col.gbp_rating":         {"Note", "Rating"},
	"col.gbp_review":         {"Avis", "Review"},
	"col.gbp_date":           {"Date", "Date"},
	"col.gbp_reply":          {"Réponse", "Reply"},
	"col.gbp_location":       {"Établissement", "Location"},
	"col.gbp_views":          {"Vues", "Views"},
	"col.gbp_actions":        {"Actions", "Actions"},
	"col.gbp_calls":          {"Appels", "Calls"},
	"col.gbp_directions":     {"Itinéraires", "Directions"},
	"col.gbp_website":        {"Clics site", "Website"},
	"col.gbp_reviews":        {"Avis", "Reviews"},

	// Beginner hints.
	"hint.gbp_views_daily": {
		"Chaque point est le nombre de personnes qui ont vu votre fiche ce jour-là. La ligne pointillée montre la période précédente : si la ligne pleine est au-dessus, votre fiche est plus vue qu’avant.",
		"Each point is the number of people who saw your profile that day. The dashed line shows the previous period: when the solid line is above it, your profile is seen more than before.",
	},
	"hint.gbp_found": {
		"La recherche Google, c’est quand on tape votre nom ou votre activité dans Google ; Google Maps, quand on cherche sur la carte. Beaucoup de vues sur mobile signifient des clients souvent en déplacement, prêts à appeler ou à venir.",
		"Google Search is when people type your name or your trade in Google; Google Maps is when they search on the map. Many mobile views mean customers on the move, ready to call or come by.",
	},
	"hint.gbp_actions": {
		"Ce sont les gestes concrets des clients après avoir vu votre fiche. Les appels et les itinéraires sont souvent les plus précieux : ce sont des clients prêts à venir.",
		"These are the concrete steps customers took after seeing your profile. Calls and directions are often the most valuable: these are customers ready to come.",
	},
	"hint.gbp_rating": {
		"La note moyenne est affichée à côté de votre nom dans Google et Maps. Au-dessus de 4,5 étoiles, elle rassure ; répondre aux avis, même négatifs, montre que vous êtes à l’écoute.",
		"The average rating is shown next to your name in Google and Maps. Above 4.5 stars it reassures; replying to reviews, even negative ones, shows that you listen.",
	},
	"hint.gbp_distribution": {
		"Chaque barre compte les avis d’un nombre d’étoiles donné. Quelques avis à 1 ou 2 étoiles sont normaux : l’important est d’y répondre poliment et de corriger ce qui peut l’être.",
		"Each bar counts the reviews with a given number of stars. A few 1- or 2-star reviews are normal: what matters is to reply politely and fix what can be fixed.",
	},
	"hint.gbp_checklist": {
		"Google met davantage en avant les fiches complètes et à jour. Chaque ligne « À compléter » est une action simple à faire depuis votre compte Google Business Profile.",
		"Google highlights complete, up-to-date profiles. Each “To do” line is a simple action to take from your Google Business Profile account.",
	},
	"hint.gbp_activity": {
		"Publier régulièrement des actualités, offres ou photos montre à Google et aux clients que l’établissement est actif. Une publication par semaine et quelques photos par mois suffisent.",
		"Posting news, offers or photos regularly shows Google and customers that the business is active. One post a week and a few photos a month are enough.",
	},
	"hint.table_gbp_keywords": {
		"Ce sont les mots que les clients ont tapés avant de voir votre fiche. Google affiche « moins de 15 » pour les petits volumes. Utilisez ces mots dans la description de votre fiche et sur votre site.",
		"These are the words customers typed before seeing your profile. Google shows “fewer than 15” for small volumes. Use these words in your profile description and on your website.",
	},
	"hint.table_gbp_reviews": {
		"Les avis sont reproduits tels quels. Une réponse rapide et personnalisée, surtout aux avis critiques, est lue par tous les futurs clients.",
		"Reviews are shown as they are. A quick, personal reply, especially to critical reviews, is read by every future customer.",
	},
	"hint.table_gbp_locations": {
		"Une ligne par établissement : comparez les vues et les actions pour repérer les fiches qui méritent plus d’attention (photos, horaires, avis).",
		"One line per location: compare views and actions to spot the profiles that need more care (photos, hours, reviews).",
	},
}

var gbpGlossary = []struct {
	key            string
	termFR, termEN string
	defFR, defEN   string
}{
	{"gbp", "Google Business Profile (fiche Google)", "Google Business Profile",
		"Fiche gratuite de votre établissement dans la recherche Google et Google Maps (anciennement Google My Business) : adresse, horaires, téléphone, photos et avis.",
		"The free card of your business in Google Search and Google Maps (formerly Google My Business): address, opening hours, phone number, photos and reviews."},
	{"gbp_view", "Vue de la fiche", "Profile view",
		"Une personne a vu votre fiche Google, dans la recherche ou dans Maps. Google compte une personne au plus une fois par jour et par appareil.",
		"Someone saw your Google profile, in Search or in Maps. Google counts a person at most once per day and device."},
	{"gbp_interactions", "Actions (interactions)", "Interactions",
		"Total des gestes faits depuis la fiche : appels, demandes d’itinéraire, clics vers le site, messages, réservations, commandes.",
		"Total of the actions taken from the profile: calls, direction requests, website clicks, messages, bookings, orders."},
	{"gbp_direction", "Demande d’itinéraire", "Direction request",
		"Une personne a demandé à Google le chemin pour venir à votre établissement : un signe fort d’intention de visite.",
		"Someone asked Google for directions to your business: a strong sign they intend to visit."},
	{"gbp_review", "Avis Google", "Review",
		"Note de 1 à 5 étoiles, souvent accompagnée d’un commentaire, laissée par un client sur votre fiche. La note moyenne est affichée à côté de votre nom.",
		"A 1 to 5-star rating, often with a comment, left by a customer on your profile. The average rating is shown next to your name."},
	{"gbp_response_rate", "Taux de réponse aux avis", "Review reply rate",
		"Part des avis auxquels vous avez répondu publiquement depuis votre fiche.",
		"Share of reviews you publicly replied to from your profile."},
	{"local_pack", "Recherche locale (pack local)", "Local search (local pack)",
		"Recherche avec une intention de proximité (« luminaire Lyon », « près de chez moi ») : Google affiche alors une carte et trois établissements, le « pack local ».",
		"A search with a local intent (“lighting store Lyon”, “near me”): Google then shows a map and three businesses, the “local pack”."},
}

// GBPGlossaryKeys lists the glossary keys about Google Business Profile
// (shown only when the report has Business Profile data).
var GBPGlossaryKeys = map[string]bool{}

func init() {
	for k, v := range gbpMessages {
		messages[k] = v
	}
	// The profile terms follow the Merchant Center ones (before "indexing").
	at := len(glossary)
	for i, g := range glossary {
		if g.key == "indexing" {
			at = i
			break
		}
	}
	merged := append([]struct {
		key            string
		termFR, termEN string
		defFR, defEN   string
	}{}, glossary[:at]...)
	merged = append(merged, gbpGlossary...)
	merged = append(merged, glossary[at:]...)
	glossary = merged
	for _, g := range gbpGlossary {
		GBPGlossaryKeys[g.key] = true
	}
}

// gbpChecklistAliases maps loose checklist labels (French / English) to
// the known item keys.
var gbpChecklistAliases = map[string]string{
	"opening_hours": "hours", "horaires": "hours", "horaires_d'ouverture": "hours", "hours_of_operation": "hours",
	"special_hours": "special_hours", "holiday_hours": "special_hours", "horaires_exceptionnels": "special_hours",
	"category": "categories", "categorie": "categories", "categories_d'activite": "categories",
	"business_description": "description", "description_de_l'etablissement": "description",
	"photo": "photos", "cover": "cover_photo", "photo_de_couverture": "cover_photo",
	"product": "products", "produits": "products", "service": "services",
	"website_link": "website", "site": "website", "site_web": "website", "site_internet": "website",
	"phone_number": "phone", "telephone": "phone", "numero_de_telephone": "phone", "adresse": "address",
	"attributs": "attributes", "post": "posts", "publications": "posts", "reservation": "booking", "bookings": "booking",
	"zone_desservie": "service_area", "messages": "messaging", "messagerie": "messaging",
	"q&a": "qa", "questions_answers": "qa", "questions_reponses": "qa", "reponses_aux_avis": "review_replies",
}

// GBPChecklistLabel returns the label of a profile checklist item: a known
// key ("hours", "photos"...) or alias is shown in the report language,
// anything else is kept as written.
func GBPChecklistLabel(lang, item string) string {
	k := strings.ToLower(strings.TrimSpace(item))
	k = strings.NewReplacer(" ", "_", "-", "_", "/", "_", "’", "'", "é", "e", "è", "e", "ê", "e", "à", "a", "ô", "o").Replace(k)
	if a, ok := gbpChecklistAliases[k]; ok {
		k = a
	}
	if Has("gbp.check." + k) {
		return T(lang, "gbp.check."+k)
	}
	return strings.TrimSpace(item)
}
