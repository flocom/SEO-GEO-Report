package mcpserver

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/flocom/SEO-GEO-Report/internal/favicon"
	"github.com/flocom/SEO-GEO-Report/internal/model"
	"github.com/flocom/SEO-GEO-Report/internal/oauth"
	"github.com/flocom/SEO-GEO-Report/internal/store"
	"github.com/flocom/SEO-GEO-Report/internal/update"
)

func newService(t *testing.T) *Service {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return NewService(Config{Store: st, MCPApps: true, Version: "test"})
}

func connect(t *testing.T, svc *Service) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	if _, err := svc.NewServer().Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func call(t *testing.T, cs *mcp.ClientSession, name string, args any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("%s returned an error: %s", name, text(res))
	}
	return res
}

func text(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func structured(t *testing.T, res *mcp.CallToolResult) map[string]any {
	t.Helper()
	b, _ := json.Marshal(res.StructuredContent)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("structured content: %v (%s)", err, b)
	}
	return m
}

var wantTools = []string{
	"get_report_guide", "create_report", "update_meta", "set_search_console", "set_analytics", "set_geo",
	"set_narrative", "add_text_section", "update_text_section", "remove_text_section", "set_options",
	"import_search_console_csv", "import_analytics_csv", "validate_report", "render_report",
	"generate_report", "get_report", "list_reports", "duplicate_report", "delete_report",
}

func TestInMemoryWorkflow(t *testing.T) {
	svc := newService(t)
	cs := connect(t, svc)
	ctx := context.Background()

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]*mcp.Tool{}
	for _, tl := range tools.Tools {
		got[tl.Name] = tl
	}
	for _, n := range wantTools {
		if got[n] == nil {
			t.Errorf("missing tool %s", n)
		} else if len(got[n].Description) < 40 {
			t.Errorf("tool %s has a too short description", n)
		}
	}
	if ui, _ := got["render_report"].Meta["ui"].(map[string]any); ui["resourceUri"] != viewerURI {
		t.Errorf("render_report _meta = %v", got["render_report"].Meta)
	}

	guide := call(t, cs, "get_report_guide", map[string]any{"language": "en"})
	if !strings.Contains(text(guide), "Percentages are in percent") {
		t.Error("guide lacks conventions")
	}

	res := call(t, cs, "create_report", map[string]any{"meta": map[string]any{
		"site_name": "Acme Shop", "site_url": "https://acme.example", "language": "en",
		"period":            map[string]string{"start": "2026-01-01", "end": "2026-03-31"},
		"comparison_period": map[string]string{"start": "2025-10-01", "end": "2025-12-31"},
	}})
	id, _ := structured(t, res)["report_id"].(string)
	if !strings.HasPrefix(id, "acme-shop-") {
		t.Fatalf("report_id = %q", id)
	}

	call(t, cs, "set_search_console", map[string]any{"report_id": id, "search_console": map[string]any{
		"totals": map[string]any{
			"clicks":      map[string]any{"current": 1200, "previous": 1000},
			"impressions": map[string]any{"current": 40000, "previous": 36000},
			"position":    map[string]any{"current": 12.4, "previous": 14.1},
		},
	}})
	// Merge: only queries, totals kept.
	call(t, cs, "set_search_console", map[string]any{"report_id": id, "search_console": map[string]any{
		"queries": []map[string]any{{"key": "acme shoes", "clicks": 300, "impressions": 2000, "position": 2.1, "prev_clicks": 250}},
	}})
	rec, _ := svc.store.Get(id)
	if rec.Report.SearchConsole.Totals.Clicks.Current != 1200 || len(rec.Report.SearchConsole.Queries) != 1 {
		t.Fatalf("merge failed: %+v", rec.Report.SearchConsole)
	}

	call(t, cs, "set_narrative", map[string]any{"report_id": id, "narrative": map[string]any{
		"executive_summary": "Clicks **+20 %**.", "section_notes": map[string]string{"queries": "Brand queries lead."},
	}})
	call(t, cs, "set_narrative", map[string]any{"report_id": id, "mode": "append", "narrative": map[string]any{
		"highlights": []string{"First"}, "section_notes": map[string]string{"pages": "Blog grows."},
	}})
	rec, _ = svc.store.Get(id)
	if n := rec.Report.Narrative; n.ExecutiveSummary == "" || len(n.SectionNotes) != 2 || len(n.Highlights) != 1 {
		t.Fatalf("narrative merge: %+v", n)
	}

	sec := call(t, cs, "add_text_section", map[string]any{"report_id": id, "title": "Méthodologie", "body": "Data from GSC.", "after": "kpis"})
	secID, _ := structured(t, sec)["section_id"].(string)
	call(t, cs, "update_text_section", map[string]any{"report_id": id, "id": secID, "style": "info"})
	call(t, cs, "set_options", map[string]any{"report_id": id, "options": map[string]any{"max_table_rows": 10}})

	v := call(t, cs, "validate_report", map[string]any{"report_id": id})
	if structured(t, v)["valid"] != true {
		t.Fatalf("validate: %s", text(v))
	}

	r := call(t, cs, "render_report", map[string]any{"report_id": id})
	out := structured(t, r)
	if out["report_id"] != id {
		t.Errorf("render output = %v", out)
	}
	// stdio / in-memory mode: no public URL, a file path instead.
	if f, _ := out["html_file"].(string); !strings.HasSuffix(f, id+".en.html") {
		t.Errorf("html_file = %v", out["html_file"])
	}

	list := call(t, cs, "list_reports", map[string]any{})
	if !strings.Contains(text(list), id) {
		t.Error("list_reports misses the report")
	}
	dup := call(t, cs, "duplicate_report", map[string]any{"report_id": id, "period": map[string]string{"start": "2026-04-01", "end": "2026-06-30"}})
	dupID, _ := structured(t, dup)["report_id"].(string)
	if dupID == "" || dupID == id {
		t.Fatalf("duplicate id = %q", dupID)
	}
	call(t, cs, "delete_report", map[string]any{"report_id": dupID, "confirm": true})
	if _, err := svc.store.Get(dupID); err == nil {
		t.Error("report not deleted")
	}

	// Errors are reported as tool errors.
	bad, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_report", Arguments: map[string]any{"report_id": "nope"}})
	if err != nil || !bad.IsError {
		t.Errorf("unknown report: %v %v", err, bad)
	}

	// Resources and prompt.
	for _, uri := range []string{guideURI, schemaURI, exampleURIBase + "fr", viewerURI} {
		rr, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
		if err != nil || len(rr.Contents) == 0 || rr.Contents[0].Text == "" {
			t.Errorf("resource %s: %v", uri, err)
		}
	}
	p, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{Name: "create_seo_geo_report", Arguments: map[string]string{"site": "acme.example", "language": "en"}})
	if err != nil || len(p.Messages) == 0 {
		t.Errorf("prompt: %v", err)
	}
}

func TestGenerateReport(t *testing.T) {
	svc := newService(t)
	cs := connect(t, svc)
	rep := model.Report{
		Meta:          model.Meta{SiteName: "One Shot", Period: model.Period{Start: "2026-01-01", End: "2026-01-31"}},
		SearchConsole: &model.SearchConsole{Totals: model.GSCTotals{Clicks: model.M(10, 8), Impressions: model.M(100, 90)}},
	}
	res := call(t, cs, "generate_report", map[string]any{"report": rep, "include_html": true})
	if structured(t, res)["report_id"] == "" {
		t.Fatal("no report_id")
	}
	found := false
	for _, c := range res.Content {
		if er, ok := c.(*mcp.EmbeddedResource); ok && er.Resource.MIMEType == "text/html" && len(er.Resource.Text) > 0 {
			found = true
		}
	}
	if !found {
		t.Error("include_html did not embed the HTML")
	}
	// Invalid report -> tool error, not a protocol error.
	bad, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "generate_report", Arguments: map[string]any{
		"report": map[string]any{"meta": map[string]any{"site_name": "x", "period": map[string]string{"start": "bad", "end": "2026-01-01"}}},
	}})
	if err != nil || !bad.IsError {
		t.Errorf("invalid period accepted: %v", err)
	}
}

func TestMergeObject(t *testing.T) {
	existing := &model.GEO{Citations: []model.CitationCheck{{Prompt: "a", Engine: "ChatGPT"}}, AICrawlers: []model.CrawlerRow{{Bot: "GPTBot", Hits: 3}}}
	var out model.GEO
	if err := mergeObject(existing, json.RawMessage(`{"citations":[{"prompt":"b","engine":"Gemini","cited":true}],"ai_crawlers":null}`), true, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Citations) != 2 || out.AICrawlers != nil {
		t.Errorf("append/null: %+v", out)
	}
	out = model.GEO{}
	_ = mergeObject(existing, json.RawMessage(`{"citations":[{"prompt":"b","engine":"Gemini","cited":true}]}`), false, &out)
	if len(out.Citations) != 1 || len(out.AICrawlers) != 1 {
		t.Errorf("merge: %+v", out)
	}
}

// --- HTTP end-to-end ----------------------------------------------------------

type bearerTransport struct {
	token   string
	headers map[string]string
}

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	for k, v := range b.headers {
		r.Header.Set(k, v)
	}
	return http.DefaultTransport.RoundTrip(r)
}

func newHTTP(t *testing.T) (*httptest.Server, *Service) {
	t.Helper()
	dir := t.TempDir()
	st, _ := store.Open(dir)
	svc := NewService(Config{Store: st, MCPApps: true, Version: "test"})
	auth, err := oauth.New(oauth.Config{DataDir: dir, Password: "test-password-123"})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(NewHTTPHandler(svc, auth, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(ts.Close)
	return ts, svc
}

func TestHTTPEndToEnd(t *testing.T) {
	ts, _ := newHTTP(t)
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	// 401 with discovery header.
	resp, _ := http.Post(ts.URL+"/mcp", "application/json", strings.NewReader(`{}`))
	resp.Body.Close()
	if resp.StatusCode != 401 || !strings.Contains(resp.Header.Get("WWW-Authenticate"), ts.URL+"/.well-known/oauth-protected-resource") {
		t.Fatalf("unauthenticated /mcp: %d %q", resp.StatusCode, resp.Header.Get("WWW-Authenticate"))
	}

	// OAuth: register -> authorize -> token.
	const redirect = "https://claude.ai/api/mcp/auth_callback"
	resp, _ = http.Post(ts.URL+"/register", "application/json", strings.NewReader(`{"redirect_uris":["`+redirect+`"],"token_endpoint_auth_method":"none","client_name":"claude.ai"}`))
	var reg map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&reg)
	resp.Body.Close()
	verifier := strings.Repeat("v", 50)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	resp, _ = noRedirect.PostForm(ts.URL+"/authorize", url.Values{
		"response_type": {"code"}, "client_id": {reg["client_id"].(string)}, "redirect_uri": {redirect},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"}, "state": {"s1"},
		"resource": {ts.URL + "/mcp"}, "password": {"test-password-123"},
	})
	resp.Body.Close()
	loc, _ := url.Parse(resp.Header.Get("Location"))
	resp, _ = http.PostForm(ts.URL+"/token", url.Values{"grant_type": {"authorization_code"}, "code": {loc.Query().Get("code")},
		"client_id": {reg["client_id"].(string)}, "redirect_uri": {redirect}, "code_verifier": {verifier}})
	var tok map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&tok)
	resp.Body.Close()
	access, _ := tok["access_token"].(string)
	if access == "" {
		t.Fatalf("no access token: %v", tok)
	}

	// Raw JSON-RPC initialize as claude.ai does (2025-06-18 protocol).
	req, _ := http.NewRequest("POST", ts.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"claude-ai","version":"0.1.0"}}}`))
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), `"serverInfo"`) {
		t.Fatalf("initialize: %d %s", resp.StatusCode, body)
	}

	// SDK client over Streamable HTTP with the OAuth token.
	ctx := context.Background()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "http-test", Version: "1"}, nil).Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: ts.URL + "/mcp", HTTPClient: &http.Client{Transport: bearerTransport{token: access}}, DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	tools, err := cs.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) != len(wantTools) {
		t.Fatalf("tools/list: %v (%d tools)", err, len(tools.Tools))
	}
	res := call(t, cs, "create_report", map[string]any{"meta": map[string]any{"site_name": "HTTP Site", "period": map[string]string{"start": "2026-01-01", "end": "2026-01-31"}}})
	id := structured(t, res)["report_id"].(string)
	call(t, cs, "set_search_console", map[string]any{"report_id": id, "search_console": map[string]any{
		"totals": map[string]any{"clicks": map[string]any{"current": 5, "previous": 4}, "impressions": map[string]any{"current": 50, "previous": 40}}}})
	r := call(t, cs, "render_report", map[string]any{"report_id": id})
	links := structured(t, r)["links"].(map[string]any)
	htmlURL, _ := links["html_url"].(string)
	if !strings.HasPrefix(htmlURL, ts.URL+"/r/") || !strings.HasSuffix(links["pdf_url"].(string), ".pdf") {
		t.Fatalf("links = %v", links)
	}

	// Hosted report (no auth needed).
	resp, _ = http.Get(htmlURL)
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") || len(b) == 0 {
		t.Fatalf("GET report: %d", resp.StatusCode)
	}
	resp, _ = http.Get(links["html_download_url"].(string))
	resp.Body.Close()
	if !strings.Contains(resp.Header.Get("Content-Disposition"), `attachment; filename="rapport-seo-geo-http-site-2026-01-01_2026-01-31.html"`) {
		t.Errorf("download disposition = %q", resp.Header.Get("Content-Disposition"))
	}
	resp, _ = http.Get(ts.URL + "/r/unknown-token")
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("unknown token: %d", resp.StatusCode)
	}

	// The password works as a static bearer token.
	req, _ = http.NewRequest("POST", ts.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`))
	req.Header.Set("Authorization", "Bearer test-password-123")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Protocol-Version", "2025-06-18")
	resp, _ = http.DefaultClient.Do(req)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), "render_report") {
		t.Errorf("password bearer: %d %s", resp.StatusCode, body)
	}
}

func TestHTTPForwardedLinks(t *testing.T) {
	ts, _ := newHTTP(t)
	ctx := context.Background()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "fwd", Version: "1"}, nil).Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: ts.URL + "/mcp", DisableStandaloneSSE: true,
		HTTPClient: &http.Client{Transport: bearerTransport{token: "test-password-123", headers: map[string]string{
			"X-Forwarded-Proto": "https", "X-Forwarded-Host": "seo.example.org", BaseURLHeader: "https://evil.example",
		}}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	res := call(t, cs, "create_report", map[string]any{"meta": map[string]any{"site_name": "Fwd", "period": map[string]string{"start": "2026-01-01", "end": "2026-01-31"}}})
	links := structured(t, res)["links"].(map[string]any)
	if u, _ := links["html_url"].(string); !strings.HasPrefix(u, "https://seo.example.org/r/") {
		t.Errorf("links do not use the forwarded host: %v", links)
	}

	// Home page and health check.
	req, _ := http.NewRequest("GET", ts.URL+"/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "seo.example.org")
	resp, _ := http.DefaultClient.Do(req)
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(b), "https://seo.example.org/mcp") {
		t.Error("home page does not show the detected MCP URL")
	}
	resp, _ = http.Get(ts.URL + "/healthz")
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("healthz: %d", resp.StatusCode)
	}
}

func TestFaviconAndUpdateNotice(t *testing.T) {
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...)
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = io.WriteString(w, `<html><head><link rel="icon" href="/i.png" sizes="32x32"></head></html>`)
		case "/i.png":
			_, _ = w.Write(png)
		default:
			http.NotFound(w, r)
		}
	}))
	defer site.Close()
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"tag_name":"v9.0.0","html_url":"https://github.com/flocom/SEO-GEO-Report/releases/tag/v9.0.0"}`)
	}))
	defer gh.Close()

	dir := t.TempDir()
	st, _ := store.Open(dir)
	fav := &favicon.Resolver{CacheDir: t.TempDir(), AllowPrivate: true}
	upd := &update.Checker{Current: "0.1.0", APIURL: gh.URL}
	upd.CheckNow(context.Background())
	svc := NewService(Config{Store: st, Version: "0.1.0", Favicons: fav, Updates: upd})
	cs := connect(t, svc)

	res := call(t, cs, "create_report", map[string]any{"meta": map[string]any{"site_name": "Fav", "site_url": site.URL,
		"period": map[string]string{"start": "2026-01-01", "end": "2026-01-31"}}})
	id := structured(t, res)["report_id"].(string)
	call(t, cs, "render_report", map[string]any{"report_id": id})
	rec, _ := st.Get(id)
	if !strings.HasPrefix(rec.Report.Meta.FaviconURL, "data:image/png;base64,") {
		t.Errorf("favicon not stored: %.40q", rec.Report.Meta.FaviconURL)
	}

	auth, _ := oauth.New(oauth.Config{DataDir: dir, Disabled: true})
	ts := httptest.NewServer(NewHTTPHandler(svc, auth, slog.New(slog.NewTextHandler(io.Discard, nil))))
	defer ts.Close()
	resp, _ := http.Get(ts.URL + "/")
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(b), "New version v9.0.0 available") {
		t.Error("home page lacks the update notice")
	}
}
