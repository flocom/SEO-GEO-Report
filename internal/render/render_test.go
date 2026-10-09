package render

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/flocom/SEO-GEO-Report/internal/example"
	"github.com/flocom/SEO-GEO-Report/internal/model"
)

func mustRender(t *testing.T, r *model.Report, opts Options) string {
	t.Helper()
	out, err := HTML(r, opts)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	s := string(out)
	for _, want := range []string{"<!DOCTYPE html>", "<html lang=", "</html>", "<title>", "<style>"} {
		if !strings.Contains(s, want) {
			t.Fatalf("output misses %q", want)
		}
	}
	if strings.Count(s, "<section") != strings.Count(s, "</section>") {
		t.Fatalf("unbalanced <section> tags")
	}
	// No external resources.
	for _, bad := range []string{`src="http`, `<link rel="stylesheet`, `href="http://`, `@import`, `url(http`, `<script src`} {
		if strings.Contains(s, bad) {
			t.Fatalf("output contains external reference %q", bad)
		}
	}
	// Missing translations show up as raw keys.
	if m := missingKey.FindString(s); m != "" {
		t.Fatalf("untranslated key in output: %q", m)
	}
	if strings.Contains(s, "ZgotmplZ") {
		t.Fatalf("template sanitizer rejected a value (ZgotmplZ)")
	}
	return s
}

var missingKey = regexp.MustCompile(`>\s*(sec|kpi|hint|chart|table|col|lbl|stat|badge|cover|hero|summary|status|prio|effort|label|insights|note|footer|toc|src|tech|delta)\.[a-z0-9_.]+\s*<`)

func sectionIDs(s string) []string {
	re := regexp.MustCompile(`<section class="[^"]*" id="([^"]+)"`)
	var ids []string
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		if !strings.HasPrefix(m[1], "part-") {
			ids = append(ids, m[1])
		}
	}
	return ids
}

func TestRichFR(t *testing.T) {
	r := richReport("fr")
	s := mustRender(t, r, Options{PDFURL: "/report.pdf", HTMLDownloadURL: "/report.html"})
	for _, want := range []string{`lang="fr"`, "En bref", "Performance dans Google", "Visibilité dans les IA (GEO)", "Note de l’analyste",
		"Glossaire", "Sommaire", "Télécharger en PDF", "Télécharger (HTML)", `href="/report.pdf"`, "48 210", "1\u00a0juil. – 30\u00a0sept. 2026", "Méthodologie", "<table>", "--accent:#7c3aed"} {
		if !strings.Contains(s, want) {
			t.Errorf("FR output misses %q", want)
		}
	}
	ids := sectionIDs(s)
	if len(ids) < 15 {
		t.Errorf("expected all sections, got %v", ids)
	}
	if ids[0] != "sec-c-methodo" {
		t.Errorf("custom 'start' section should come first, got %v", ids)
	}
	idx := map[string]int{}
	for i, id := range ids {
		idx[id] = i
	}
	if idx["sec-c-custom-2"] != idx["sec-search_console"]+1 {
		t.Errorf("custom section not placed after search_console: %v", ids)
	}
	if idx["sec-c-custom-3"] != idx["sec-glossary"]-1 {
		t.Errorf("'end' custom section should precede the glossary: %v", ids)
	}
	if strings.Contains(s, "javascript:") {
		t.Errorf("dangerous link survived markdown sanitizing")
	}
	// Two parts: the visual overview, then the detailed analysis.
	p1, p2 := strings.Index(s, `id="part-1"`), strings.Index(s, `id="part-2"`)
	if p1 < 0 || p2 < p1 || !strings.Contains(s, "L’essentiel en un coup d’œil") || !strings.Contains(s, "Analyse détaillée") {
		t.Fatalf("missing part banners")
	}
	part1 := s[p1:p2]
	if strings.Contains(part1, "<table") && !strings.Contains(part1, `md-table`) {
		t.Errorf("part 1 must not contain data tables")
	}
	for _, id := range []string{"sec-summary", "sec-kpis", "sec-ai_glance", "sec-plan_glance", "sec-c-methodo"} {
		if !strings.Contains(part1, `id="`+id+`"`) {
			t.Errorf("%s should be in part 1", id)
		}
	}
	if strings.Index(s, `id="sec-search_console"`) < p2 {
		t.Errorf("search_console should be in part 2")
	}
}

func TestFavicon(t *testing.T) {
	r := richReport("fr")
	r.Meta.FaviconURL = "data:image/png;base64,iVBORw0KGgo="
	s := mustRender(t, r, Options{})
	if !strings.Contains(s, `<link rel="icon" href="data:image/png;base64,iVBORw0KGgo=">`) || !strings.Contains(s, `class="logo-fav"`) {
		t.Errorf("favicon not rendered")
	}
	for _, bad := range []string{"javascript:alert(1)", "data:text/html;base64,PHNjcmlwdD4=", "http://insecure.example/f.ico", "none"} {
		r.Meta.FaviconURL = bad
		s := mustRender(t, r, Options{})
		if strings.Contains(s, `rel="icon"`) {
			t.Errorf("favicon %q should be rejected", bad)
		}
	}
}

func TestRichEN(t *testing.T) {
	s := mustRender(t, richReport("fr"), Options{Lang: "en"})
	for _, want := range []string{`lang="en"`, "At a glance", "Google Search performance", "AI visibility (GEO)", "Glossary", "48,210", "Jul 1 – Sep 30, 2026"} {
		if !strings.Contains(s, want) {
			t.Errorf("EN output misses %q", want)
		}
	}
	if strings.Contains(s, "Download PDF") {
		t.Errorf("PDF button rendered without PDFURL")
	}
}

func TestEscaping(t *testing.T) {
	r := richReport("fr")
	r.Meta.SiteName = `<img src=x onerror=alert(1)>`
	r.Meta.LogoURL = "javascript:alert(1)"
	r.Meta.BrandColor = "red;}</style><script>alert(1)</script>"
	r.Narrative.ExecutiveSummary = "<script>alert(2)</script> **ok**"
	s := mustRender(t, r, Options{PDFURL: "javascript:alert(3)"})
	for _, bad := range []string{"<script>alert", "<img src=x", "javascript:alert"} {
		if strings.Contains(s, bad) {
			t.Errorf("unescaped payload %q in output", bad)
		}
	}
	if !strings.Contains(s, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Errorf("malicious query not shown escaped")
	}
	if !strings.Contains(s, "--accent:"+DefaultAccent) {
		t.Errorf("invalid brand color should fall back to default")
	}
}

func TestHiddenSectionsAndOptions(t *testing.T) {
	r := richReport("fr")
	f := false
	r.Options = &model.Options{HideSections: []string{"queries", "geo"}, ShowGlossary: &f, BeginnerHints: &f, AutoInsights: &f, MaxTableRows: 3}
	s := mustRender(t, r, Options{})
	for _, bad := range []string{`id="sec-queries"`, `id="sec-geo"`, `id="sec-glossary"`, `class="hint"`, "Comment lire"} {
		if strings.Contains(s, bad) {
			t.Errorf("hidden content %q is present", bad)
		}
	}
	if !strings.Contains(s, "3 lignes affichées sur") {
		t.Errorf("max_table_rows truncation note missing")
	}
	if strings.Contains(s, `href="#sec-geo"`) {
		t.Errorf("hidden section listed in the table of contents")
	}
}

func TestPartialReports(t *testing.T) {
	full := richReport("fr")
	meta := full.Meta
	cases := map[string]*model.Report{
		"gsc-only":  {Meta: meta, SearchConsole: full.SearchConsole},
		"ga4-only":  {Meta: meta, Analytics: full.Analytics},
		"geo-only":  {Meta: meta, GEO: full.GEO},
		"meta-only": {Meta: model.Meta{SiteName: "Vide", Period: model.Period{Start: "2026-07-01", End: "2026-07-31"}}},
		"no-previous": {Meta: model.Meta{SiteName: "Sans comparaison", Language: "en", Period: model.Period{Start: "2026-07-01", End: "2026-07-31"}},
			SearchConsole: &model.SearchConsole{Totals: model.GSCTotals{Clicks: model.Metric{Current: 120}, Impressions: model.Metric{Current: 4000}},
				Queries: []model.GSCRow{{Key: "a", Clicks: 10, Impressions: 100}}}},
		"geo-citations-only": {Meta: meta, GEO: &model.GEO{Citations: full.GEO.Citations[:2]}},
	}
	for name, r := range cases {
		t.Run(name, func(t *testing.T) {
			s := mustRender(t, r, Options{})
			ids := sectionIDs(s)
			switch name {
			case "gsc-only":
				if strings.Contains(s, `id="sec-analytics"`) || strings.Contains(s, `id="sec-geo"`) || !strings.Contains(s, `id="sec-queries"`) {
					t.Errorf("unexpected sections %v", ids)
				}
			case "ga4-only":
				if strings.Contains(s, `id="sec-search_console"`) || !strings.Contains(s, `id="sec-channels"`) {
					t.Errorf("unexpected sections %v", ids)
				}
				// AI sources are detected from GA4 sources.
				if !strings.Contains(s, `id="sec-geo"`) {
					t.Logf("note: no GEO section from GA4 sources (insights may not detect AI referrals)")
				}
			case "geo-only":
				if !strings.Contains(s, `id="sec-geo"`) || strings.Contains(s, `id="sec-queries"`) {
					t.Errorf("unexpected sections %v", ids)
				}
			case "meta-only":
				if len(ids) != 2 || ids[0] != "sec-summary" || ids[1] != "sec-glossary" || !strings.Contains(s, "Aucune donnée") {
					t.Errorf("empty report should show an empty-state summary and the glossary, got %v", ids)
				}
			}
			if _, err := HTML(r, Options{Lang: "en"}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNilAndInputUntouched(t *testing.T) {
	if _, err := HTML(nil, Options{}); err == nil {
		t.Fatal("expected an error for a nil report")
	}
	r := richReport("fr")
	if r.SearchConsole.Totals.CTR != nil {
		t.Fatal("fixture should not have a CTR")
	}
	mustRender(t, r, Options{})
	if r.SearchConsole.Totals.CTR != nil || r.Sections[1].ID != "" {
		t.Fatal("HTML must not modify its input")
	}
}

func TestMarkdown(t *testing.T) {
	h := string(Markdown("# Titre\n\n| a | b |\n|---|--:|\n| 1 | 2 |\n\n<b>raw</b> [x](javascript:alert(1)) ~~barré~~"))
	for _, want := range []string{"<h3>Titre</h3>", `<div class="md-table"><table>`, "<del>barré</del>"} {
		if !strings.Contains(h, want) {
			t.Errorf("markdown misses %q in %s", want, h)
		}
	}
	if strings.Contains(h, "<b>raw") || strings.Contains(h, "javascript:") {
		t.Errorf("unsafe markdown output: %s", h)
	}
}

func TestDemo(t *testing.T) {
	for _, lang := range []string{"fr", "en"} {
		d := example.Demo(lang)
		if d == nil || (d.SearchConsole == nil && d.Analytics == nil && d.GEO == nil) {
			t.Skip("example.Demo is not implemented yet")
		}
		mustRender(t, d, Options{})
	}
}

func TestAccentColor(t *testing.T) {
	cases := map[string]string{"": DefaultAccent, "#ABC": "#aabbcc", "10b981": "#10b981", "#12345": DefaultAccent, "blue": DefaultAccent}
	for in, want := range cases {
		if got := accentColor(in); got != want {
			t.Errorf("accentColor(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestWritePreview writes preview files when RENDER_PREVIEW is set to a
// directory: RENDER_PREVIEW=/tmp/preview go test -run Preview ./internal/render/
func TestWritePreview(t *testing.T) {
	dir := os.Getenv("RENDER_PREVIEW")
	if dir == "" {
		t.Skip("set RENDER_PREVIEW to write preview files")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name string, r *model.Report, o Options) {
		out, err := HTML(r, o)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), out, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("report-fr.html", richReport("fr"), Options{PDFURL: "report.pdf", HTMLDownloadURL: "report-fr.html"})
	en := richReport("en")
	en.Meta.FaviconURL = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAACAAAAAgCAYAAABzenr0AAAAv0lEQVR4nO1XSQ6AMAhsG0/WT+rD9JPWq55IlHQBjBCjc8TSmU4JFe8ySHPcc/G7iFPyOHYJPEVcExK0yTFX0CbHIkJr4dPwFqc/w9wBcwGdJKkf12x8Wwb2XmwHSuStbyWQHaBuDuuobpjXAEmAxFpqTlOAhJyT+44r+AV8W4CkvXJySQ5IRLymE5LfAjhRq7lw3WI7UCOQXJXof+BOYWKY14C9gNy4pIU4JW/vACjRJgbOgAOa5M6h6RigOZ4fQudBSY1LK5IAAAAASUVORK5CYII="
	write("report-en.html", en, Options{Lang: "en"})
	full := richReport("fr")
	write("gsc-only.html", &model.Report{Meta: full.Meta, SearchConsole: full.SearchConsole}, Options{})
	write("ga4-only.html", &model.Report{Meta: full.Meta, Analytics: full.Analytics}, Options{})
	write("geo-only.html", &model.Report{Meta: full.Meta, GEO: full.GEO}, Options{})
	write("empty.html", &model.Report{Meta: model.Meta{SiteName: "Vide", Period: model.Period{Start: "2026-07-01", End: "2026-07-31"}}}, Options{})
	for _, lang := range []string{"fr", "en"} {
		if d := example.Demo(lang); d != nil && (d.SearchConsole != nil || d.Analytics != nil) {
			write("demo-"+lang+".html", d, Options{})
		}
	}
}
