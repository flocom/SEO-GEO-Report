package i18n

import (
	"regexp"
	"strings"
	"testing"
)

// Words that are the same in French and English (brand names, technical
// terms used as is in French): a message may be identical in both languages
// only when every word belongs to this list.
var sameWords = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`GEO SEO UX Core Web Vitals mobile Mobile Citations CTR Engagement Actions Date
		Impressions Notes AI Overview Page Pages Position Sessions Sitemap Sitemaps Messages Logo Menu Services Stable
		GA4 Google Maps Top Bing Shopping Merchant Center Suggestion Webmaster Tools Analytics Business Profile Search
		Console places via action actions s`) {
		sameWords[w] = true
	}
}

var words = regexp.MustCompile(`[\pL\d]+`)

// Telltale words of the other language, matched as whole words.
var (
	englishInFrench = regexp.MustCompile(`(?i)\b(the|and|of|your|with|from|this|is|are|visits|clicks|before)\b`)
	frenchInEnglish = regexp.MustCompile(`(?i)\b(le|la|les|des|du|est|sont|votre|vos|avec|depuis|contre|avant|visites)\b`)
)

// TestCatalogTranslated checks every message of the catalog, including the
// ones merged from other files (messages_*.go): both languages are filled,
// a message identical in both languages only uses words valid in both, and
// no message carries telltale words of the other language.
func TestCatalogTranslated(t *testing.T) {
	if len(catalogDuplicates) > 0 {
		t.Errorf("keys defined twice across message files: %v", catalogDuplicates)
	}
	for k, v := range messages {
		fr, en := strings.TrimSpace(v[0]), strings.TrimSpace(v[1])
		if fr == "" || en == "" {
			t.Errorf("%q misses a translation", k)
			continue
		}
		if fr == en {
			for _, w := range words.FindAllString(fr, -1) {
				if !sameWords[w] && !regexp.MustCompile(`^\d+$`).MatchString(w) {
					t.Errorf("%q looks untranslated (same text in both languages: %q)", k, fr)
					break
				}
			}
			continue
		}
		if m := englishInFrench.FindString(stripNames(fr)); m != "" {
			t.Errorf("%q: English word %q in the French text %q", k, m, fr)
		}
		if m := frenchInEnglish.FindString(stripNames(en)); m != "" {
			t.Errorf("%q: French word %q in the English text %q", k, m, en)
		}
	}
	for _, lang := range []string{"fr", "en"} {
		for _, g := range Glossary(lang) {
			if strings.TrimSpace(g.Term) == "" || strings.TrimSpace(g.Definition) == "" {
				t.Errorf("%s glossary entry %q incomplete", lang, g.Key)
			}
		}
	}
}

// stripNames removes quoted examples and product names (« Direct »,
// "Organic Search", GA4 labels) that legitimately keep their original
// language.
var quoted = regexp.MustCompile(`«[^»]*»|“[^”]*”|"[^"]*"|\([^)]*\)`)

func stripNames(s string) string { return quoted.ReplaceAllString(s, "") }

func TestChannelName(t *testing.T) {
	cases := []struct{ lang, in, want string }{
		{"fr", "Organic Search", "Recherche naturelle"},
		{"fr", "  organic   search ", "Recherche naturelle"},
		{"fr", "Direct", "Accès direct"},
		{"fr", "Organic Social", "Réseaux sociaux"},
		{"fr", "Paid Social", "Réseaux sociaux (payant)"},
		{"fr", "Referral", "Sites référents"},
		{"fr", "AI Assistant", "Assistants IA"},
		{"fr", "Email", "E-mail"},
		{"fr", "Unassigned", "Non attribué"},
		{"fr", "Mobile Push Notifications", "Notifications mobiles"},
		{"en", "Organic Search", "Organic search"},
		{"en", "Recherche naturelle", "Organic search"},
		{"en", "Direct", "Direct"},
		{"fr", "Ma campagne", "Ma campagne"},
		{"en", "", ""},
	}
	for _, c := range cases {
		if got := ChannelName(c.lang, c.in); got != c.want {
			t.Errorf("ChannelName(%q, %q) = %q, want %q", c.lang, c.in, got, c.want)
		}
	}
	for k, v := range channelNames {
		if k != strings.ToLower(k) || v[0] == "" || v[1] == "" {
			t.Errorf("channel %q: lower-case key and both labels required", k)
		}
	}
}
