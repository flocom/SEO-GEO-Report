package i18n

// Texts of the two report versions shown on the same page: the selector at
// the top, the version names, the "go further" note that closes the
// essential version and its short glossary. Merged into the catalog at init.
//
// French strings use a no-break space ( ) before ":" and "?".

var viewMessages = map[string][2]string{
	// Selector (pill toggle of the top bar). view.label names it for
	// screen readers; the .sub texts are the helper line under the toggle.
	"view.label":         {"Version du rapport", "Report version"},
	"view.essential":     {"Essentiel", "Essential"},
	"view.essential.sub": {"Les résultats clés en 5 minutes", "The key results in 5 minutes"},
	"view.advanced":      {"Avancé", "Advanced"},
	"view.advanced.sub":  {"Toute l’analyse détaillée", "The full, detailed analysis"},

	// Shown in the cover kicker, on screen and on paper.
	"view.essential.kicker": {"Version essentielle", "Essential version"},
	"view.advanced.kicker":  {"Version avancée", "Advanced version"},

	// Cover lead of the essential version (the advanced one keeps cover.lead).
	"cover.lead_ess": {
		"Vos résultats sur Google et dans les assistants IA (ChatGPT, Perplexity, Gemini), expliqués simplement : ce qui progresse, ce qui recule et ce qui arrive.",
		"Your results on Google and in AI assistants (ChatGPT, Perplexity, Gemini), explained simply: what is growing, what is slipping and what comes next.",
	},

	// Download menu of the top bar.
	"dl.button":           {"Télécharger", "Download"},
	"dl.current":          {"Affichée", "On screen"},
	"cover.pdf_ess":       {"PDF — version essentielle", "PDF — essential version"},
	"cover.pdf_adv":       {"PDF — version complète", "PDF — full version"},
	"cover.pdf_ess.sub":   {"Quelques pages, les résultats clés", "A few pages, the key results"},
	"cover.pdf_adv.sub":   {"Le rapport intégral, toutes les sections", "The whole report, every section"},
	"cover.pdf.sub":       {"Pour imprimer ou partager", "To print or share"},
	"cover.html.sub":      {"Les deux versions, consultables hors ligne", "Both versions, readable offline"},
	"cover.pdf_ess.title": {"Télécharger la version essentielle en PDF (quelques pages)", "Download the essential version as a PDF (a few pages)"},
	"cover.pdf_adv.title": {"Télécharger le rapport complet en PDF", "Download the full report as a PDF"},
	"cover.html.title": {
		"Télécharger le fichier HTML autonome : les deux versions, consultables hors ligne",
		"Download the standalone HTML file: both versions, readable offline",
	},

	// "More" note of the part 1 lists, in the essential version.
	"summary.more_adv.one":   {"+ %s autre dans la version avancée", "+ %s more in the advanced version"},
	"summary.more_adv.other": {"+ %s autres dans la version avancée", "+ %s more in the advanced version"},

	// Section intro variants for the essential version (no part 2 there).
	"sec.plan_glance.intro_ess": {
		"Les actions menées pendant la période et les prochaines étapes conseillées. Le détail se trouve dans la version avancée.",
		"The work done during the period and the recommended next steps. Details are in the advanced version.",
	},

	// Short glossary closing the essential version.
	"sec.glossary_essential.title": {"Les mots à connaître", "Key terms"},
	"sec.glossary_essential.intro": {
		"Les quelques termes utilisés dans cette version, en une phrase chacun.",
		"The few terms used in this version, in one sentence each.",
	},

	// "Go further" note at the end of the essential version.
	"next.kicker": {"Pour aller plus loin", "To go further"},
	"next.title":  {"Passez en version avancée", "Switch to the advanced version"},
	"next.text": {
		"Cette version donne l’essentiel. La version avancée, sur la même page, détaille chaque chiffre pour votre équipe ou votre prestataire.",
		"This version gives the essentials. The advanced version, on the same page, details every figure for your team or your agency.",
	},
	"next.count.one":   {"%s section détaillée de plus :", "%s more detailed section:"},
	"next.count.other": {"%s sections détaillées de plus :", "%s more detailed sections:"},
	"next.button":      {"Voir la version avancée", "Show the advanced version"},
	"next.print": {
		"La version avancée est disponible dans le rapport en ligne et dans le PDF complet.",
		"The advanced version is available in the online report and in the full PDF.",
	},
}

func init() {
	for k, v := range viewMessages {
		if _, dup := messages[k]; dup {
			catalogDuplicates = append(catalogDuplicates, k)
		}
		messages[k] = v
	}
}
