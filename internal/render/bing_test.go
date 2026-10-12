package render

import (
	"slices"
	"strings"
	"testing"

	"github.com/flocom/SEO-GEO-Report/internal/example"
	"github.com/flocom/SEO-GEO-Report/internal/model"
)

func TestBingSection(t *testing.T) {
	for _, lang := range []string{"fr", "en"} {
		s := mustRender(t, example.Demo(lang), Options{})
		ids := sectionIDs(s)
		i := slices.Index(ids, "sec-bing")
		if i < 0 || ids[i-1] != "sec-countries_devices" || ids[i+1] != "sec-analytics" {
			t.Fatalf("%s: bing section misplaced in %v", lang, ids)
		}
		bing := s[strings.Index(s, `id="sec-bing"`):strings.Index(s, `id="sec-analytics"`)]
		want := map[string][]string{
			"fr": {"Performance dans Bing", "Clics\u00a0: Google vs Bing", "Part de Bing", "Citations IA (Copilot)", "Requêtes d’ancrage les plus citées", "Citations par intention", "Détail des pages citées par l’IA"},
			"en": {"Bing Search performance", "Clicks: Google vs Bing", "Bing share", "AI citations (Copilot)", "Most cited grounding queries", "Citations by intent", "Details of pages cited by AI"},
		}[lang]
		for _, w := range want {
			if !strings.Contains(bing, w) {
				t.Errorf("%s: bing section misses %q", lang, w)
			}
		}
		if !strings.Contains(s, "Bing Webmaster Tools") {
			t.Errorf("%s: cover sources miss Bing Webmaster Tools", lang)
		}
	}
}

func TestBingOnlyAndHidden(t *testing.T) {
	full := example.Demo("en")

	// Bing alone: no Google comparison, the section still renders.
	r := &model.Report{Meta: full.Meta, Bing: full.Bing}
	s := mustRender(t, r, Options{})
	if !strings.Contains(s, `id="sec-bing"`) || strings.Contains(s, `id="sec-search_console"`) || strings.Contains(s, "Clicks: Google vs Bing") {
		t.Errorf("bing-only report: sections %v", sectionIDs(s))
	}

	// AI Performance alone (no search performance totals).
	r = &model.Report{Meta: full.Meta, Bing: &model.Bing{AIPerformance: &model.BingAIPerformance{
		GroundingQueries: []model.BingGroundingQuery{{Query: "a", Citations: 3}, {Query: "b", Citations: 1}},
	}}}
	s = mustRender(t, r, Options{})
	if !strings.Contains(s, `id="sec-bing"`) || !strings.Contains(s, "Grounding query details") {
		t.Errorf("AI-only bing report: sections %v", sectionIDs(s))
	}

	// hide_sections hides it.
	r = example.Demo("en")
	r.Options = &model.Options{HideSections: []string{"bing"}}
	if s := mustRender(t, r, Options{}); strings.Contains(s, `id="sec-bing"`) {
		t.Error("bing section should be hidden")
	}
}
