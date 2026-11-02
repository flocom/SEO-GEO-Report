package i18n

import "strings"

// channelNames translates the Google Analytics 4 default channel groups,
// which GA4 exports in English whatever the interface language. Keys are
// lower-case; the French labels are written for business owners.
var channelNames = map[string][2]string{
	"organic search":             {"Recherche naturelle", "Organic search"},
	"direct":                     {"Accès direct", "Direct"},
	"organic social":             {"Réseaux sociaux", "Organic social"},
	"paid social":                {"Réseaux sociaux (payant)", "Paid social"},
	"referral":                   {"Sites référents", "Referral"},
	"ai assistant":               {"Assistants IA", "AI assistants"},
	"ai assistants":              {"Assistants IA", "AI assistants"},
	"paid search":                {"Recherche payante (publicité)", "Paid search"},
	"email":                      {"E-mail", "Email"},
	"e-mail":                     {"E-mail", "Email"},
	"display":                    {"Bannières publicitaires", "Display"},
	"unassigned":                 {"Non attribué", "Unassigned"},
	"(other)":                    {"Autres", "Other"},
	"cross-network":              {"Publicité multiréseau", "Cross-network"},
	"organic video":              {"Vidéo (naturel)", "Organic video"},
	"organic shopping":           {"Shopping (naturel)", "Organic shopping"},
	"paid shopping":              {"Shopping (payant)", "Paid shopping"},
	"paid video":                 {"Vidéo (payant)", "Paid video"},
	"paid other":                 {"Autre publicité", "Paid other"},
	"affiliates":                 {"Affiliation", "Affiliates"},
	"sms":                        {"SMS", "SMS"},
	"audio":                      {"Audio", "Audio"},
	"mobile push notifications":  {"Notifications mobiles", "Mobile push notifications"},
	"recherche naturelle":        {"Recherche naturelle", "Organic search"},
	"recherche organique":        {"Recherche naturelle", "Organic search"},
	"accès direct":               {"Accès direct", "Direct"},
	"réseaux sociaux":            {"Réseaux sociaux", "Organic social"},
	"réseaux sociaux organiques": {"Réseaux sociaux", "Organic social"},
	"sites référents":            {"Sites référents", "Referral"},
	"référent":                   {"Sites référents", "Referral"},
	"assistants ia":              {"Assistants IA", "AI assistants"},
}

// ChannelName returns the display name of a GA4 default channel group in the
// report language ("Organic Search" -> "Recherche naturelle"). Matching is
// case-insensitive and ignores surrounding spaces; unknown names (custom
// channel groups) are returned unchanged.
func ChannelName(lang, name string) string {
	k := strings.ToLower(strings.Join(strings.Fields(name), " "))
	if m, ok := channelNames[k]; ok {
		if Lang(lang) == "en" {
			return m[1]
		}
		return m[0]
	}
	return name
}
