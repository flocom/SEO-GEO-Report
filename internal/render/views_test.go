package render

import (
	"regexp"
	"strings"
	"testing"

	"github.com/flocom/SEO-GEO-Report/internal/example"
	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// sectionClasses maps section ids to their class attribute.
func sectionClasses(s string) map[string]string {
	re := regexp.MustCompile(`<section class="([^"]*)" id="([^"]+)"`)
	out := map[string]string{}
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		out[m[2]] = m[1]
	}
	return out
}

func checkedView(t *testing.T, s string) string {
	t.Helper()
	re := regexp.MustCompile(`<input class="view-radio" type="radio" name="seogeo-view" id="view-(essential|advanced)"[^>]*>`)
	var checked []string
	ms := re.FindAllStringSubmatch(s, -1)
	if len(ms) != 2 {
		t.Fatalf("want 2 version radios, got %d", len(ms))
	}
	for _, m := range ms {
		if strings.HasSuffix(m[0], " checked>") {
			checked = append(checked, m[1])
		}
	}
	if len(checked) != 1 {
		t.Fatalf("want exactly one checked version, got %v", checked)
	}
	return checked[0]
}

// TestViewsInOnePage: both versions are in the page, with the switch, and the
// sections are flagged for the CSS that hides the other version.
func TestViewsInOnePage(t *testing.T) {
	for _, lang := range []string{"fr", "en"} {
		r := example.Demo(lang)
		s := mustRender(t, r, Options{})
		if v := checkedView(t, s); v != model.ViewEssential {
			t.Errorf("%s: default view = %s, want essential", lang, v)
		}
		// The radios come before .doc so that "~" reaches every part.
		if strings.Index(s, `id="view-advanced"`) > strings.Index(s, `<div class="doc">`) {
			t.Errorf("%s: radios must precede .doc", lang)
		}
		for _, want := range []string{
			`<label class="vs-opt vs-ess" for="view-essential">`, `<label class="vs-opt vs-adv" for="view-advanced">`,
			`aria-labelledby="vs-label vs-ess-name"`, `aria-describedby="vs-adv-sub"`,
			`#view-essential:checked ~ .doc .adv-only`, `#view-advanced:checked ~ .doc .ess-only`,
			`class="next-step ess-only"`, `<label class="next-btn" for="view-advanced" data-goto="part-2">`,
			`<script>`, `class="ess-only link-switch" for="view-advanced" data-goto="sec-review"`,
		} {
			if !strings.Contains(s, want) {
				t.Errorf("%s: output misses %q", lang, want)
			}
		}
		cls := sectionClasses(s)
		// Part 1 is in both versions; part 2 only in the advanced one.
		for _, id := range []string{"part-1", "sec-summary", "sec-kpis", "sec-ai_glance", "sec-plan_glance"} {
			if c, ok := cls[id]; !ok || strings.Contains(c, "-only") {
				t.Errorf("%s: %s should be in both versions (class %q)", lang, id, c)
			}
		}
		for _, id := range []string{"part-2", "sec-review", "sec-search_console", "sec-queries", "sec-pages", "sec-bing", "sec-analytics",
			"sec-merchant_center", "sec-business_profile", "sec-geo", "sec-technical", "sec-actions", "sec-recommendations", "sec-glossary", "sec-c-geo-method"} {
			if c := cls[id]; !strings.Contains(c, "adv-only") {
				t.Errorf("%s: %s should be advanced-only (class %q)", lang, id, c)
			}
		}
		if c := cls["sec-"+idGlossaryEss]; !strings.Contains(c, "ess-only") {
			t.Errorf("%s: short glossary should be essential-only (class %q)", lang, c)
		}
		// No dense table in the essential version.
		for id, c := range cls {
			if strings.Contains(c, "adv-only") {
				continue
			}
			sec := s[strings.Index(s, `id="`+id+`"`):]
			if end := strings.Index(sec, "</section>"); end > 0 {
				sec = sec[:end]
			}
			if strings.Contains(sec, `class="tbl`) {
				t.Errorf("%s: section %s of the essential version holds a table", lang, id)
			}
		}
		// Table of contents: part 2 and its entries flagged, the short
		// glossary for the essential version only.
		if !strings.Contains(s, `<div class="toc-part adv-only">`) || !strings.Contains(s, `<li class="ess-only"><a href="#sec-`+idGlossaryEss+`">`) {
			t.Errorf("%s: table of contents not flagged per version", lang)
		}
		// Each version is numbered without a gap: the short glossary takes
		// the number part 2 starts with.
		num := regexp.MustCompile(`id="sec-` + idGlossaryEss + `">\s*<div class="sec-keep">\s*<header class="sec-head"><p class="sec-num">(\d+)</p>`).FindStringSubmatch(s)
		first := regexp.MustCompile(`id="sec-review">\s*<div class="sec-keep">\s*<header class="sec-head"><p class="sec-num">(\d+)</p>`).FindStringSubmatch(s)
		if num == nil || first == nil || num[1] != "05" || first[1] != "05" {
			t.Errorf("%s: numbering short glossary %v, part 2 %v", lang, num, first)
		}
	}
}

func TestDefaultView(t *testing.T) {
	r := example.Demo("fr")
	if v := checkedView(t, mustRender(t, r, Options{View: "advanced"})); v != model.ViewAdvanced {
		t.Errorf("Options.View advanced: got %s", v)
	}
	r.Options = &model.Options{DefaultView: "advanced"}
	if v := checkedView(t, mustRender(t, r, Options{})); v != model.ViewAdvanced {
		t.Errorf("options.default_view advanced: got %s", v)
	}
	if v := checkedView(t, mustRender(t, r, Options{View: "essentiel"})); v != model.ViewEssential {
		t.Errorf("Options.View overrides the default: got %s", v)
	}
	if v := checkedView(t, mustRender(t, r, Options{View: "bogus"})); v != model.ViewAdvanced {
		t.Errorf("unknown Options.View falls back to the default: got %s", v)
	}
	r.Options.DefaultView = "nope"
	if v := checkedView(t, mustRender(t, r, Options{})); v != model.ViewEssential {
		t.Errorf("unknown default_view falls back to essential: got %s", v)
	}
}

func TestViewDownloadButtons(t *testing.T) {
	r := example.Demo("en")
	s := mustRender(t, r, Options{PDFURL: "/r/x.pdf", PDFEssentialURL: "/r/x.pdf?view=essential", PDFAdvancedURL: "/r/x.pdf?view=advanced", HTMLDownloadURL: "/r/x/download"})
	for _, want := range []string{`btn-pdf-ess" href="/r/x.pdf?view=essential"`, `btn-pdf-adv" href="/r/x.pdf?view=advanced"`, "Essential PDF", "Full PDF", `href="/r/x/download"`,
		`class="next-pdf" href="/r/x.pdf?view=advanced"`} {
		if !strings.Contains(s, want) {
			t.Errorf("output misses %q", want)
		}
	}
	if strings.Contains(s, ">Download PDF<") {
		t.Error("the single PDF button should be replaced by the two version buttons")
	}
	// Only the legacy link: one button, as before.
	s = mustRender(t, r, Options{PDFURL: "/r/x.pdf", PDFEssentialURL: "/r/x.pdf?view=essential"})
	if !strings.Contains(s, ">Download PDF<") || strings.Contains(s, `btn-pdf-ess" href`) {
		t.Error("with PDFURL only, the single PDF button is expected")
	}
}

// TestEssentialGlossary keeps the terms of the data present only.
func TestEssentialGlossary(t *testing.T) {
	r := smallSiteReport("en")
	r.MerchantCenter, r.BusinessProfile = nil, nil
	s := mustRender(t, r, Options{})
	i := strings.Index(s, `id="sec-`+idGlossaryEss+`"`)
	if i < 0 {
		t.Fatal("no short glossary")
	}
	g := s[i : i+strings.Index(s[i:], "</section>")]
	if !strings.Contains(g, "<dt>Clicks</dt>") || strings.Contains(g, "Merchant Center</dt>") || strings.Contains(g, "Business Profile") {
		t.Errorf("unexpected short glossary terms")
	}
	if strings.Count(g, `class="g-item"`) >= strings.Count(s[strings.Index(s, `id="sec-glossary"`):], `class="g-item"`) {
		t.Error("the short glossary should be shorter than the full one")
	}
	// show_glossary=false or hidden glossary: no short glossary either.
	f := false
	r.Options = &model.Options{ShowGlossary: &f}
	if strings.Contains(mustRender(t, r, Options{}), `id="sec-`+idGlossaryEss+`"`) {
		t.Error("show_glossary=false should drop the short glossary")
	}
	r.Options = &model.Options{HideSections: []string{"glossary"}}
	if strings.Contains(mustRender(t, r, Options{}), `id="sec-`+idGlossaryEss+`"`) {
		t.Error("hide_sections glossary should drop the short glossary")
	}
}
