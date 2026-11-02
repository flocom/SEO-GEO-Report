package mcpserver

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/flocom/SEO-GEO-Report/internal/model"
	"github.com/flocom/SEO-GEO-Report/internal/store"
)

func TestLinksForViews(t *testing.T) {
	rec := &store.Record{ID: "x", ShareToken: "TOK", Report: model.Report{Meta: model.Meta{Language: "fr"}}}
	l := LinksFor("https://h", rec, "", "")
	want := Links{
		HTML: "https://h/r/TOK", HTMLDownload: "https://h/r/TOK/download", PDF: "https://h/r/TOK.pdf",
		HTMLEssential: "https://h/r/TOK?view=essential", HTMLAdvanced: "https://h/r/TOK?view=advanced",
		PDFEssential: "https://h/r/TOK.pdf?view=essential", PDFAdvanced: "https://h/r/TOK.pdf?view=advanced",
		DefaultView: "essential",
	}
	if l != want {
		t.Errorf("default links:\n got %+v\nwant %+v", l, want)
	}
	// A requested view and language: html_url / pdf_url follow them, ?lang=
	// and ?view= combine.
	l = LinksFor("https://h", rec, "en", "advanced")
	if l.HTML != "https://h/r/TOK?lang=en&view=advanced" || l.PDF != "https://h/r/TOK.pdf?lang=en&view=advanced" ||
		l.PDFEssential != "https://h/r/TOK.pdf?lang=en&view=essential" || l.HTMLAdvanced != "https://h/r/TOK?lang=en&view=advanced" ||
		l.HTMLDownload != "https://h/r/TOK/download?lang=en&view=advanced" || l.DefaultView != "advanced" {
		t.Errorf("en/advanced links: %+v", l)
	}
	// options.default_view advanced: the plain link opens the advanced view.
	rec.Report.Options = &model.Options{DefaultView: "advanced"}
	l = LinksFor("https://h", rec, "", "")
	if l.HTML != "https://h/r/TOK" || l.PDF != "https://h/r/TOK.pdf" || l.HTMLAdvanced != "https://h/r/TOK?view=advanced" || l.DefaultView != "advanced" {
		t.Errorf("default_view advanced links: %+v", l)
	}
	if (LinksFor("", rec, "", "") != Links{}) {
		t.Error("no base URL: no links")
	}
}

func TestDownloadNameView(t *testing.T) {
	rec := &store.Record{Report: model.Report{Meta: model.Meta{SiteName: "Acme", Period: model.Period{Start: "2026-01-01", End: "2026-01-31"}}}}
	for _, c := range []struct{ lang, view, ext, want string }{
		{"fr", "essential", "pdf", "rapport-seo-geo-acme-2026-01-01_2026-01-31-essentiel.pdf"},
		{"en", "essential", "pdf", "seo-geo-report-acme-2026-01-01_2026-01-31-essential.pdf"},
		{"fr", "advanced", "pdf", "rapport-seo-geo-acme-2026-01-01_2026-01-31.pdf"},
		{"fr", "", "html", "rapport-seo-geo-acme-2026-01-01_2026-01-31.html"},
	} {
		if got := DownloadName(rec, c.lang, c.view, c.ext); got != c.want {
			t.Errorf("DownloadName(%s, %s, %s) = %q, want %q", c.lang, c.view, c.ext, got, c.want)
		}
	}
}

func getBody(t *testing.T, u string) (string, *http.Response) {
	t.Helper()
	resp, err := http.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b), resp
}

func checkedIn(html string) string {
	for _, v := range []string{"essential", "advanced"} {
		i := strings.Index(html, `id="view-`+v+`"`)
		if i < 0 {
			continue
		}
		tag := html[i : i+strings.Index(html[i:], ">")]
		if strings.HasSuffix(tag, " checked") {
			return v
		}
	}
	return ""
}

// TestRenderViewsHTTP: render_report returns the links of both versions,
// the hosted page honours ?view= (with ?lang=), and the PDFs are cached per
// version.
func TestRenderViewsHTTP(t *testing.T) {
	ts, svc := newHTTP(t)
	ctx := context.Background()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "views", Version: "1"}, nil).Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: ts.URL + "/mcp", DisableStandaloneSSE: true,
		HTTPClient: &http.Client{Transport: bearerTransport{token: "test-password-123"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	rep := model.Report{
		Meta:          model.Meta{SiteName: "Views", Period: model.Period{Start: "2026-01-01", End: "2026-01-31"}},
		SearchConsole: &model.SearchConsole{Totals: model.GSCTotals{Clicks: model.M(10, 8), Impressions: model.M(100, 90)}},
	}
	res := call(t, cs, "generate_report", map[string]any{"report": rep})
	sc := structured(t, res)
	links := sc["links"].(map[string]any)
	for _, k := range []string{"html_url", "pdf_url", "html_download_url", "view_essential_url", "view_advanced_url", "pdf_essential_url", "pdf_advanced_url"} {
		if u, _ := links[k].(string); !strings.HasPrefix(u, ts.URL+"/r/") {
			t.Errorf("links[%s] = %v", k, links[k])
		}
	}
	if sc["view"] != "essential" || links["default_view"] != "essential" {
		t.Errorf("view = %v, default_view = %v", sc["view"], links["default_view"])
	}
	txt := text(res)
	for _, k := range []string{"view_essential_url", "view_advanced_url", "pdf_essential_url", "pdf_advanced_url", "html_download_url"} {
		if !strings.Contains(txt, links[k].(string)) {
			t.Errorf("text summary misses %s", k)
		}
	}
	if !strings.Contains(txt, "Essential") || !strings.Contains(txt, "Advanced") {
		t.Error("text summary should explain both versions")
	}

	// Hosted page: default, ?view=advanced, ?view=essential&lang=en.
	page, _ := getBody(t, links["html_url"].(string))
	if checkedIn(page) != "essential" || !strings.Contains(page, `btn-pdf-adv" href="`+strings.ReplaceAll(links["pdf_advanced_url"].(string), "&", "&amp;")+`"`) {
		t.Errorf("default page: view %q / PDF buttons", checkedIn(page))
	}
	page, _ = getBody(t, links["view_advanced_url"].(string))
	if checkedIn(page) != "advanced" {
		t.Errorf("?view=advanced opens %q", checkedIn(page))
	}
	page, _ = getBody(t, links["view_essential_url"].(string)+"&lang=en")
	if checkedIn(page) != "essential" || !strings.Contains(page, `<html lang="en">`) {
		t.Errorf("?view=essential&lang=en: view %q", checkedIn(page))
	}
	_, resp := getBody(t, links["html_download_url"].(string)+"?view=advanced")
	if !strings.Contains(resp.Header.Get("Content-Disposition"), `.html"`) {
		t.Errorf("download disposition %q", resp.Header.Get("Content-Disposition"))
	}

	// PDFs: one cache entry per version (seeded here: no Chromium needed).
	id := sc["report_id"].(string)
	rec, err := svc.store.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"essential", "advanced"} {
		if _, err := svc.store.SavePDF(rec.ID, "fr", v, []byte("%PDF-"+v)); err != nil {
			t.Fatal(err)
		}
	}
	body, resp := getBody(t, links["pdf_essential_url"].(string))
	if body != "%PDF-essential" || !strings.Contains(resp.Header.Get("Content-Disposition"), `-essentiel.pdf"`) {
		t.Errorf("essential PDF: %q, %q", body, resp.Header.Get("Content-Disposition"))
	}
	if body, _ = getBody(t, links["pdf_advanced_url"].(string)); body != "%PDF-advanced" {
		t.Errorf("advanced PDF: %q", body)
	}
	if body, _ = getBody(t, links["pdf_url"].(string)); body != "%PDF-essential" {
		t.Errorf("pdf_url should serve the default (essential) version: %q", body)
	}

	// view on render_report, invalid view, default_view through set_options.
	r := call(t, cs, "render_report", map[string]any{"report_id": id, "view": "advanced"})
	l2 := structured(t, r)["links"].(map[string]any)
	if !strings.HasSuffix(l2["html_url"].(string), "?view=advanced") || !strings.HasSuffix(l2["pdf_url"].(string), ".pdf?view=advanced") {
		t.Errorf("view=advanced links: %v", l2)
	}
	bad, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "render_report", Arguments: map[string]any{"report_id": id, "view": "medium"}})
	if err != nil || !bad.IsError {
		t.Errorf("an unknown view should be a tool error: %v", err)
	}
	call(t, cs, "set_options", map[string]any{"report_id": id, "options": map[string]any{"default_view": "advanced"}})
	l3 := structured(t, call(t, cs, "render_report", map[string]any{"report_id": id}))["links"].(map[string]any)
	if l3["default_view"] != "advanced" || strings.Contains(l3["html_url"].(string), "view=") {
		t.Errorf("default_view advanced: %v", l3)
	}
	page, _ = getBody(t, l3["html_url"].(string))
	if checkedIn(page) != "advanced" {
		t.Errorf("default_view advanced: page opens %q", checkedIn(page))
	}
}
