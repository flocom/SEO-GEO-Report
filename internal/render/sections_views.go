package render

import (
	"html/template"

	"github.com/flocom/SEO-GEO-Report/internal/i18n"
	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// Two versions of the same report live in one page:
//
//   - essential: the cover, part 1 (key results in plain words), a short
//     glossary and a "go further" note;
//   - advanced: the full report (part 1 and the detailed analysis).
//
// Sections carry Only = "essential" / "advanced" when they belong to one
// version; the template turns it into the ess-only / adv-only classes that
// views.css hides according to the radio button checked at the top of the
// body. No JavaScript is needed (a small script only syncs the URL).

// idGlossaryEss is the short glossary of the essential version.
const idGlossaryEss = "glossary_essential"

// onlyClass maps a section's version to its CSS class (template helper).
func onlyClass(only string) string {
	switch only {
	case model.ViewEssential:
		return "ess-only"
	case model.ViewAdvanced:
		return "adv-only"
	}
	return ""
}

// essentialTerms lists the glossary entries used by part 1, with the data
// that makes them relevant.
var essentialTerms = []struct {
	key  string
	need func(r *model.Report) bool
}{
	{"seo", nil},
	{"geo", nil},
	{"clicks", func(r *model.Report) bool { return r.SearchConsole != nil || r.Bing != nil }},
	{"impressions", func(r *model.Report) bool { return r.SearchConsole != nil || r.Bing != nil }},
	{"position", func(r *model.Report) bool { return r.SearchConsole != nil || r.Bing != nil }},
	{"session", func(r *model.Report) bool { return r.Analytics != nil }},
	{"organic", func(r *model.Report) bool { return r.Analytics != nil }},
	{"key_event", func(r *model.Report) bool { return r.Analytics != nil && r.Analytics.Totals.KeyEvents != nil }},
	{"ai_referral", func(r *model.Report) bool { return r.Analytics != nil || r.GEO != nil }},
	{"citation", func(r *model.Report) bool { return r.GEO != nil && len(r.GEO.Citations) > 0 }},
	{"aio", func(r *model.Report) bool { return r.GEO != nil }},
	{"gmc", func(r *model.Report) bool { return r.MerchantCenter != nil }},
	{"gbp", func(r *model.Report) bool { return !r.BusinessProfile.IsEmpty() }},
	{"gbp_interactions", func(r *model.Report) bool { return !r.BusinessProfile.IsEmpty() }},
}

// glossaryEssential is the short glossary closing the essential version:
// only the terms its pages use, in the order of the full glossary. It
// follows the show_glossary option and hide_sections "glossary".
func (b *builder) glossaryEssential() *section {
	if !boolPtr(b.opt.ShowGlossary) || b.r.SectionHidden("glossary") {
		return nil
	}
	keep := map[string]bool{}
	for _, t := range essentialTerms {
		if t.need == nil || t.need(b.r) {
			keep[t.key] = true
		}
	}
	var entries []i18n.GlossaryEntry
	for _, e := range i18n.Glossary(b.lang) {
		if keep[e.Key] {
			entries = append(entries, e)
		}
	}
	if len(entries) == 0 {
		return nil
	}
	return &section{
		ID: idGlossaryEss, Anchor: "sec-" + idGlossaryEss, Only: model.ViewEssential,
		Title:  b.t("sec." + idGlossaryEss + ".title"),
		Intro:  b.t("sec." + idGlossaryEss + ".intro"),
		Icon:   sectionIcon("glossary"),
		Blocks: []block{{Kind: "glossary", Span: 12, Tone: "compact", Glossary: entries}},
	}
}

// nextStep builds the "go further" note of the essential version from the
// sections only the advanced version shows. Nil when there are none.
func (b *builder) nextStep(sections []*section, pdfFull template.URL) *nextStep {
	var items []string
	for _, s := range sections {
		if s.Only == model.ViewAdvanced && !s.IsPart && s.Title != "" {
			items = append(items, s.Title)
		}
	}
	if len(items) == 0 {
		return nil
	}
	return &nextStep{Count: b.plural(float64(len(items)), "next.count"), Items: items, PDF: pdfFull}
}
