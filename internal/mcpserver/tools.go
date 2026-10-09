package mcpserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/flocom/SEO-GEO-Report/internal/importer"
	"github.com/flocom/SEO-GEO-Report/internal/model"
	"github.com/flocom/SEO-GEO-Report/internal/store"
)

// BaseURLHeader is set by the HTTP layer on every /mcp request with the
// auto-detected public base URL (any client-supplied value is removed first).
const BaseURLHeader = "X-Seogeo-Base-Url"

// maxInlinePDF is the largest PDF returned as an embedded resource.
const maxInlinePDF = 5 << 20

// ---------------------------------------------------------------------------
// Inputs

type reportIDInput struct {
	ReportID string `json:"report_id" jsonschema:"Id returned by create_report (see list_reports)"`
}

type guideInput struct {
	Language       string `json:"language,omitempty" jsonschema:"Language of the compact example: fr or en (default fr). The guide itself is in English"`
	IncludeExample *bool  `json:"include_example,omitempty" jsonschema:"Include a compact JSON example of a complete report (default true)"`
}

type createInput struct {
	Meta    model.Meta     `json:"meta" jsonschema:"Report identity: site_name and period are required; language fr or en; comparison_period strongly recommended"`
	Options *model.Options `json:"options,omitempty" jsonschema:"Optional display options"`
}

type metaInput struct {
	ReportID string     `json:"report_id" jsonschema:"Report id"`
	Meta     model.Meta `json:"meta" jsonschema:"Fields to change. Only the keys you send are replaced (send null to clear a field)"`
}

type scInput struct {
	ReportID      string              `json:"report_id" jsonschema:"Report id"`
	SearchConsole model.SearchConsole `json:"search_console" jsonschema:"Google Search Console data. In merge mode only the keys you send are replaced (e.g. send only queries)"`
	Mode          string              `json:"mode,omitempty" jsonschema:"merge (default): replace only the top-level keys you send; replace: replace the whole search_console section"`
}

type gaInput struct {
	ReportID  string          `json:"report_id" jsonschema:"Report id"`
	Analytics model.Analytics `json:"analytics" jsonschema:"Google Analytics 4 data. In merge mode only the keys you send are replaced"`
	Mode      string          `json:"mode,omitempty" jsonschema:"merge (default) or replace"`
}

type geoInput struct {
	ReportID string    `json:"report_id" jsonschema:"Report id"`
	GEO      model.GEO `json:"geo" jsonschema:"Visibility in AI assistants. In merge mode only the keys you send are replaced"`
	Mode     string    `json:"mode,omitempty" jsonschema:"merge (default), replace, or append (lists are appended to existing ones)"`
}

type narrativeInput struct {
	ReportID  string          `json:"report_id" jsonschema:"Report id"`
	Narrative model.Narrative `json:"narrative" jsonschema:"Analyst texts (Markdown), written in the report language"`
	Mode      string          `json:"mode,omitempty" jsonschema:"merge (default): keys you send replace existing ones, section_notes are merged per key; append: lists (highlights, concerns, actions, recommendations) are appended; replace: replace the whole narrative"`
}

type addSectionInput struct {
	ReportID string `json:"report_id" jsonschema:"Report id"`
	ID       string `json:"id,omitempty" jsonschema:"Optional identifier (generated when omitted)"`
	Title    string `json:"title" jsonschema:"Section title"`
	Body     string `json:"body" jsonschema:"Markdown content (lists, tables, links, bold...)"`
	After    string `json:"after,omitempty" jsonschema:"Built-in section id after which the section is inserted, or start / end (default end)"`
	Style    string `json:"style,omitempty" jsonschema:"plain (default), info, success, warning or note"`
}

type updateSectionInput struct {
	ReportID string  `json:"report_id" jsonschema:"Report id"`
	ID       string  `json:"id" jsonschema:"Section id (returned by add_text_section, visible in get_report)"`
	Title    *string `json:"title,omitempty"`
	Body     *string `json:"body,omitempty" jsonschema:"Markdown"`
	After    *string `json:"after,omitempty"`
	Style    *string `json:"style,omitempty"`
}

type removeSectionInput struct {
	ReportID string `json:"report_id" jsonschema:"Report id"`
	ID       string `json:"id" jsonschema:"Section id"`
}

type optionsInput struct {
	ReportID string        `json:"report_id" jsonschema:"Report id"`
	Options  model.Options `json:"options" jsonschema:"Options to change (merged with existing ones)"`
}

type importGSCInput struct {
	ReportID string `json:"report_id" jsonschema:"Report id"`
	CSV      string `json:"csv" jsonschema:"Full raw text of ONE CSV file from a Search Console export (French or English headers), e.g. the content of Queries.csv / Requêtes.csv"`
	Kind     string `json:"kind,omitempty" jsonschema:"queries, pages, countries, devices, dates or appearance. Guessed from the header or file name when omitted"`
	FileName string `json:"file_name,omitempty" jsonschema:"Original file name (helps guessing the kind)"`
}

type importGAInput struct {
	ReportID string `json:"report_id" jsonschema:"Report id"`
	CSV      string `json:"csv" jsonschema:"Full raw text of a GA4 'Download file > CSV' export (comment lines starting with # are fine)"`
	Kind     string `json:"kind,omitempty" jsonschema:"channels, sources, landing_pages or daily. Guessed from the header when omitted"`
}

type renderInput struct {
	ReportID    string `json:"report_id" jsonschema:"Report id"`
	Language    string `json:"language,omitempty" jsonschema:"Override the report language: fr or en"`
	Format      string `json:"format,omitempty" jsonschema:"html (default), pdf, or both. HTML is always saved; pdf/both also generate the PDF now (needs Chromium, available in the Docker image)"`
	IncludeHTML bool   `json:"include_html,omitempty" jsonschema:"Also return the full HTML document as an embedded text/html resource (to display it as an artifact). Large: only when the user asks"`
	IncludePDF  bool   `json:"include_pdf,omitempty" jsonschema:"Also return the PDF as an embedded application/pdf resource (when smaller than 5 MB)"`
}

type generateInput struct {
	Report      model.Report `json:"report" jsonschema:"The complete report (see get_report_guide and the seogeo://schema resource)"`
	ReportID    string       `json:"report_id,omitempty" jsonschema:"Optional: overwrite this existing report instead of creating a new one"`
	Language    string       `json:"language,omitempty" jsonschema:"Override the report language: fr or en"`
	Format      string       `json:"format,omitempty" jsonschema:"html (default), pdf or both"`
	IncludeHTML bool         `json:"include_html,omitempty" jsonschema:"Also return the full HTML as an embedded text/html resource"`
	IncludePDF  bool         `json:"include_pdf,omitempty" jsonschema:"Also return the PDF as an embedded resource (when smaller than 5 MB)"`
}

type duplicateInput struct {
	ReportID         string        `json:"report_id" jsonschema:"Report to copy"`
	KeepData         bool          `json:"keep_data,omitempty" jsonschema:"Keep search_console, analytics and geo data (default false: cleared for the next period)"`
	KeepNarrative    bool          `json:"keep_narrative,omitempty" jsonschema:"Keep all narrative texts (default false: only pending actions and recommendations are kept)"`
	Period           *model.Period `json:"period,omitempty" jsonschema:"New analysed period"`
	ComparisonPeriod *model.Period `json:"comparison_period,omitempty" jsonschema:"New comparison period"`
	Title            string        `json:"title,omitempty" jsonschema:"New title"`
}

type deleteInput struct {
	ReportID string `json:"report_id" jsonschema:"Report id"`
	Confirm  bool   `json:"confirm" jsonschema:"Must be true. Deletion is permanent: ask the user first"`
}

// ---------------------------------------------------------------------------
// Registration

func boolPtr(b bool) *bool { return &b }

var (
	readOnly    = &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)}
	additive    = &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)}
	idempotent  = &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), IdempotentHint: true, OpenWorldHint: boolPtr(false)}
	destructive = &mcp.ToolAnnotations{DestructiveHint: boolPtr(true), OpenWorldHint: boolPtr(false)}
)

// relaxed returns the inferred input schema of T with the "required" list of
// the given properties removed, so that partial objects can be sent.
func relaxed[T any](props ...string) *jsonschema.Schema {
	s, err := jsonschema.For[T](nil)
	if err != nil {
		panic(err)
	}
	for _, p := range props {
		if ps := s.Properties[p]; ps != nil {
			ps.Required = nil
		}
	}
	return s
}

func (s *Service) registerTools(srv *mcp.Server) {
	viewerMeta := mcp.Meta(nil)
	if s.cfg.MCPApps {
		viewerMeta = mcp.Meta{
			"ui":             map[string]any{"resourceUri": viewerURI},
			"ui/resourceUri": viewerURI, // legacy key, still read by some hosts
		}
	}

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_report_guide",
		Title:       "Read the report guide",
		Description: "Call this FIRST. Returns the workflow to build a SEO & GEO report with this server, the field conventions (percentages in percent: 3.2 = 3.2 %; dates YYYY-MM-DD; metrics as {current, previous}), the list of section ids, what to ask the user (which Google Search Console and GA4 exports, which AI citation checks), narrative tips and a compact JSON example. This server never fetches data itself: all numbers come from the user.",
		Annotations: readOnly,
	}, s.toolGuide)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_report",
		Title:       "Create a report draft",
		Description: "Create a new report draft and return its report_id. Requires meta.site_name and meta.period {start, end} (YYYY-MM-DD); set meta.language (fr or en, default fr) and meta.comparison_period (previous period or same period last year) so progress can be shown. Then fill data with set_search_console / set_analytics / set_geo (or import_*_csv), write texts with set_narrative and call render_report.",
		Annotations: additive,
	}, s.toolCreate)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_meta",
		Title:       "Update report identity",
		Description: "Change report metadata: title, site name/URL, language (fr/en), period, comparison period and label, prepared_by / prepared_for, branding (logo_url as https URL or data: URI, brand_color hex), currency. Only the keys you send are changed.",
		InputSchema: relaxed[metaInput]("meta"),
		Annotations: idempotent,
	}, s.toolUpdateMeta)

	mcp.AddTool(srv, &mcp.Tool{
		Name:  "set_search_console",
		Title: "Set Google Search Console data",
		Description: "Store Google Search Console data supplied by the user (this tool does not fetch anything). Fields: totals {clicks, impressions, ctr, position} as {current, previous} metrics (CTR in percent: 3.2 = 3.2 %; position lower = better), daily and previous_daily [{date YYYY-MM-DD, clicks, impressions, ctr, position}], queries / pages / countries / devices / search_appearance rows [{key, clicks, impressions, ctr, position, prev_clicks, prev_impressions, prev_position}], brand_split, position_distribution, indexing, core_web_vitals. " +
			"Default mode merge replaces only the top-level keys you send, so you can send queries and pages in separate calls. For raw CSV exports use import_search_console_csv instead.",
		InputSchema: relaxed[scInput]("search_console"),
		Annotations: idempotent,
	}, s.toolSetSC)

	mcp.AddTool(srv, &mcp.Tool{
		Name:  "set_analytics",
		Title: "Set Google Analytics 4 data",
		Description: "Store GA4 data supplied by the user (never fetched). Fields: totals and organic_totals {sessions, users, new_users, engaged_sessions, engagement_rate (percent: 62.5 = 62.5 %), avg_engagement_time (seconds), page_views, key_events, revenue} as {current, previous} metrics; daily / previous_daily [{date, sessions, organic_sessions, ai_sessions}]; channels [{channel, sessions, prev_sessions, ...}]; sources [{source, medium, sessions, prev_sessions, ...}] (AI assistants like chatgpt.com / perplexity.ai are detected from sources automatically); landing_pages. " +
			"Default mode merge replaces only the keys you send. For raw GA4 CSV exports use import_analytics_csv.",
		InputSchema: relaxed[gaInput]("analytics"),
		Annotations: idempotent,
	}, s.toolSetGA)

	mcp.AddTool(srv, &mcp.Tool{
		Name:  "set_geo",
		Title: "Set GEO (AI visibility) data",
		Description: "Store Generative Engine Optimization data supplied by the user: ai_referrals [{platform, source, sessions, prev_sessions, ...}] (optional: detected from analytics.sources when omitted), ai_daily, citations [{prompt, engine, cited, mentioned, position, previously_cited, competitors_cited, date, notes}] from manual checks in ChatGPT / Perplexity / Gemini / AI Overviews / Copilot / Claude, share_of_voice [{brand, share (percent), previous, is_self}], ai_overviews [{query, overview_present, cited}], ai_crawlers [{bot, hits, prev_hits}] from server logs. " +
			"Mode merge (default), replace, or append (to add citation checks to existing ones).",
		InputSchema: relaxed[geoInput]("geo"),
		Annotations: idempotent,
	}, s.toolSetGEO)

	mcp.AddTool(srv, &mcp.Tool{
		Name:  "set_narrative",
		Title: "Write the analysis texts",
		Description: "Set the analyst texts, in the report language, based on the numbers: executive_summary (Markdown, 3-6 sentences with figures), highlights and concerns (one sentence each), actions [{title, description, date, status done|in_progress|planned, category seo|geo|technical|content|netlinking|ux|tracking, impact}], recommendations [{title, description, priority high|medium|low, effort low|medium|high, category}], section_notes {section_id: Markdown comment shown inside that section}. " +
			"Mode merge (default), append (lists appended) or replace.",
		InputSchema: relaxed[narrativeInput]("narrative"),
		Annotations: idempotent,
	}, s.toolSetNarrative)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "add_text_section",
		Title:       "Add a free text section",
		Description: "Add a custom Markdown section (methodology, competitor analysis, notes...). Place it with after = a built-in section id (summary, kpis, search_console, queries, pages, countries_devices, analytics, channels, landing_pages, geo, technical, actions, recommendations), start or end. Returns the section id.",
		Annotations: additive,
	}, s.toolAddSection)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_text_section",
		Title:       "Edit a free text section",
		Description: "Change the title, body (Markdown), position (after) or style of a custom section. Only the fields you send are changed.",
		Annotations: idempotent,
	}, s.toolUpdateSection)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "remove_text_section",
		Title:       "Remove a free text section",
		Description: "Delete a custom Markdown section (added with add_text_section) by its id. Built-in sections cannot be removed: hide them with set_options hide_sections.",
		Annotations: destructive,
	}, s.toolRemoveSection)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_options",
		Title:       "Set display options",
		Description: "Display options (merged with existing ones): hide_sections (built-in section ids), max_table_rows (default 15), show_glossary, beginner_hints ('how to read this' explanations for non-experts), auto_insights (automatic observations computed from the numbers).",
		Annotations: idempotent,
	}, s.toolSetOptions)

	mcp.AddTool(srv, &mcp.Tool{
		Name:  "import_search_console_csv",
		Title: "Import a Search Console CSV",
		Description: "Parse the raw text of ONE CSV file from a Google Search Console Performance export (French or English: Queries/Requêtes, Pages, Countries/Pays, Devices/Appareils, Dates, Search appearance) and merge it into the report. Exports with a comparison (two columns per metric) fill the previous values. The Dates table also recomputes the totals. " +
			"Call once per file. Use this when the user attaches or pastes the CSV files; otherwise use set_search_console.",
		Annotations: idempotent,
	}, s.toolImportGSC)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "import_analytics_csv",
		Title:       "Import a GA4 CSV",
		Description: "Parse the raw text of a Google Analytics 4 report CSV ('Share > Download file > CSV'; French or English headers; channel group, source / medium, landing page or date tables) and merge it into report.analytics. Totals are not always present in these files: complete them with set_analytics if needed.",
		Annotations: idempotent,
	}, s.toolImportGA)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "validate_report",
		Title:       "Check a report",
		Description: "Return blocking errors, warnings (e.g. CTR sent as a ratio instead of percent, missing previous values) and a completeness summary (which sections are filled / missing, suggestions). Call before render_report.",
		Annotations: readOnly,
	}, s.toolValidate)

	mcp.AddTool(srv, &mcp.Tool{
		Name:  "render_report",
		Title: "Render the report",
		Description: "Render the report and return its links: the online HTML view URL, the standalone HTML download URL and the PDF URL (hosted by this server, shareable, unguessable), or the local file paths when running locally without a public URL. Also returns key KPIs and automatic insights so you can comment on the results for the user. " +
			"format: html (default), pdf or both (pdf generates the PDF immediately; otherwise it is generated on first download). include_html / include_pdf embed the document in the response. Rendering again after changes updates the same links.",
		Annotations: idempotent,
		Meta:        viewerMeta,
	}, s.toolRender)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "generate_report",
		Title:       "Generate a report in one call",
		Description: "One-shot: store a complete report (same JSON structure as the seogeo://schema resource: meta, search_console, analytics, geo, narrative, sections, options) and render it. Returns the report_id (editable afterwards with the set_* tools), the HTML / download / PDF links and a summary of KPIs and insights. Use it when you already have all the data; otherwise prefer create_report + set_* tools.",
		Annotations: additive,
		Meta:        viewerMeta,
	}, s.toolGenerate)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_report",
		Title:       "Get a report",
		Description: "Return the stored report JSON (all data and texts) with its links.",
		Annotations: readOnly,
	}, s.toolGet)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_reports",
		Title:       "List reports",
		Description: "List stored reports (id, site, period, last update), most recent first.",
		Annotations: readOnly,
	}, s.toolList)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "duplicate_report",
		Title:       "Duplicate a report",
		Description: "Copy a report, typically to prepare the next period: keeps meta, branding, options, custom sections, pending actions and recommendations; clears data and texts unless keep_data / keep_narrative. Optionally sets the new period and comparison period. Returns the new report_id.",
		Annotations: additive,
	}, s.toolDuplicate)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_report",
		Title:       "Delete a report",
		Description: "Permanently delete a report and its rendered files; its public links stop working. Ask the user for confirmation first and pass confirm=true.",
		Annotations: destructive,
	}, s.toolDelete)
}

// ---------------------------------------------------------------------------
// Helpers

func textResult(text string, structured any) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}, StructuredContent: structured}
}

func (s *Service) baseURL(req *mcp.CallToolRequest) string {
	if req != nil && req.Extra != nil && req.Extra.Header != nil {
		if v := req.Extra.Header.Get(BaseURLHeader); v != "" {
			return v
		}
	}
	return strings.TrimRight(s.cfg.PublicURL, "/")
}

func (s *Service) get(id string) (*store.Record, error) {
	id = strings.TrimSpace(id)
	rec, err := s.store.Get(id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, fmt.Errorf("report %q not found: call list_reports to see existing ids, or create_report", id)
	}
	return rec, err
}

func (s *Service) update(id string, fn func(r *model.Report) error) (*store.Record, error) {
	id = strings.TrimSpace(id)
	rec, err := s.store.Update(id, fn)
	if errors.Is(err, store.ErrNotFound) {
		return nil, fmt.Errorf("report %q not found: call list_reports to see existing ids, or create_report", id)
	}
	return rec, err
}

// rawArg returns the raw JSON of a top-level argument.
func rawArg(req *mcp.CallToolRequest, name string) json.RawMessage {
	if req == nil || req.Params == nil || len(req.Params.Arguments) == 0 {
		return nil
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(req.Params.Arguments, &m) != nil {
		return nil
	}
	return m[name]
}

// mergeObject overlays the keys of patch (a JSON object) on existing and
// decodes the result into out. A null value removes the key. In append mode,
// arrays are concatenated instead of replaced.
func mergeObject(existing any, patch json.RawMessage, appendLists bool, out any) error {
	base := map[string]json.RawMessage{}
	if existing != nil {
		b, err := json.Marshal(existing)
		if err != nil {
			return err
		}
		if string(b) != "null" {
			if err := json.Unmarshal(b, &base); err != nil {
				return err
			}
		}
	}
	var p map[string]json.RawMessage
	if err := json.Unmarshal(patch, &p); err != nil {
		return fmt.Errorf("expected a JSON object: %w", err)
	}
	for k, v := range p {
		switch {
		case string(v) == "null":
			delete(base, k)
		case appendLists && len(v) > 0 && v[0] == '[' && len(base[k]) > 0 && base[k][0] == '[':
			var a, b []json.RawMessage
			_ = json.Unmarshal(base[k], &a)
			_ = json.Unmarshal(v, &b)
			merged, _ := json.Marshal(append(a, b...))
			base[k] = merged
		default:
			base[k] = v
		}
	}
	b, err := json.Marshal(base)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func checkMode(mode string, allowed ...string) (string, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		return allowed[0], nil
	}
	for _, a := range allowed {
		if mode == a {
			return mode, nil
		}
	}
	return "", fmt.Errorf("invalid mode %q: use %s", mode, strings.Join(allowed, ", "))
}

func (s *Service) savedText(rec *store.Record, what string) *mcp.CallToolResult {
	errs, warns := rec.Report.Validate()
	var b strings.Builder
	fmt.Fprintf(&b, "%s saved in report %s.", what, rec.ID)
	if len(errs) > 0 {
		fmt.Fprintf(&b, "\nErrors to fix before rendering: %s", strings.Join(errs, "; "))
	}
	if len(warns) > 0 {
		fmt.Fprintf(&b, "\nWarnings: %s", strings.Join(warns, "; "))
	}
	return textResult(b.String(), map[string]any{"report_id": rec.ID, "errors": nonNil(errs), "warnings": nonNil(warns)})
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// ---------------------------------------------------------------------------
// Handlers

func (s *Service) toolGuide(_ context.Context, _ *mcp.CallToolRequest, in guideInput) (*mcp.CallToolResult, any, error) {
	include := in.IncludeExample == nil || *in.IncludeExample
	return textResult(GuideMarkdown(include, in.Language), nil), nil, nil
}

func (s *Service) toolCreate(_ context.Context, req *mcp.CallToolRequest, in createInput) (*mcp.CallToolResult, any, error) {
	r := &model.Report{Meta: in.Meta, Options: in.Options}
	if strings.TrimSpace(r.Meta.SiteName) == "" {
		return nil, nil, errors.New("meta.site_name is required")
	}
	r.Meta.Language = r.Lang()
	rec, err := s.store.Create(r)
	if err != nil {
		return nil, nil, err
	}
	errs, warns := rec.Report.Validate()
	links := LinksFor(s.baseURL(req), rec, "")
	next := []string{
		"set_search_console (or import_search_console_csv) with the user's Google Search Console numbers",
		"set_analytics (or import_analytics_csv) with GA4 numbers",
		"set_geo with AI visibility data (citation checks, AI referrals, share of voice)",
		"set_narrative with the executive summary, highlights, actions and recommendations",
		"validate_report then render_report",
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Report created. report_id: %s\nNext steps:\n", rec.ID)
	for i, n := range next {
		fmt.Fprintf(&b, "%d. %s\n", i+1, n)
	}
	if len(errs) > 0 {
		fmt.Fprintf(&b, "Errors to fix (update_meta): %s\n", strings.Join(errs, "; "))
	}
	if r.Meta.ComparisonPeriod == nil {
		b.WriteString("Tip: set meta.comparison_period with update_meta so progress can be shown.\n")
	}
	return textResult(b.String(), map[string]any{
		"report_id": rec.ID, "links": links, "next_steps": next, "errors": nonNil(errs), "warnings": nonNil(warns),
	}), nil, nil
}

func (s *Service) toolUpdateMeta(_ context.Context, req *mcp.CallToolRequest, in metaInput) (*mcp.CallToolResult, any, error) {
	raw := rawArg(req, "meta")
	if raw == nil {
		return nil, nil, errors.New("meta is required")
	}
	rec, err := s.update(in.ReportID, func(r *model.Report) error {
		var m model.Meta
		if err := mergeObject(r.Meta, raw, false, &m); err != nil {
			return fmt.Errorf("meta: %w", err)
		}
		r.Meta = m
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s.savedText(rec, "Meta"), nil, nil
}

func (s *Service) toolSetSC(_ context.Context, req *mcp.CallToolRequest, in scInput) (*mcp.CallToolResult, any, error) {
	mode, err := checkMode(in.Mode, "merge", "replace", "append")
	if err != nil {
		return nil, nil, err
	}
	raw := rawArg(req, "search_console")
	rec, err := s.update(in.ReportID, func(r *model.Report) error {
		var existing any
		if mode != "replace" && r.SearchConsole != nil {
			existing = r.SearchConsole
		}
		var sc model.SearchConsole
		if err := mergeObject(existing, raw, mode == "append", &sc); err != nil {
			return fmt.Errorf("search_console: %w", err)
		}
		r.SearchConsole = &sc
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s.savedText(rec, "Search Console data"), nil, nil
}

func (s *Service) toolSetGA(_ context.Context, req *mcp.CallToolRequest, in gaInput) (*mcp.CallToolResult, any, error) {
	mode, err := checkMode(in.Mode, "merge", "replace", "append")
	if err != nil {
		return nil, nil, err
	}
	raw := rawArg(req, "analytics")
	rec, err := s.update(in.ReportID, func(r *model.Report) error {
		var existing any
		if mode != "replace" && r.Analytics != nil {
			existing = r.Analytics
		}
		var a model.Analytics
		if err := mergeObject(existing, raw, mode == "append", &a); err != nil {
			return fmt.Errorf("analytics: %w", err)
		}
		r.Analytics = &a
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s.savedText(rec, "Analytics data"), nil, nil
}

func (s *Service) toolSetGEO(_ context.Context, req *mcp.CallToolRequest, in geoInput) (*mcp.CallToolResult, any, error) {
	mode, err := checkMode(in.Mode, "merge", "replace", "append")
	if err != nil {
		return nil, nil, err
	}
	raw := rawArg(req, "geo")
	rec, err := s.update(in.ReportID, func(r *model.Report) error {
		var existing any
		if mode != "replace" && r.GEO != nil {
			existing = r.GEO
		}
		var g model.GEO
		if err := mergeObject(existing, raw, mode == "append", &g); err != nil {
			return fmt.Errorf("geo: %w", err)
		}
		r.GEO = &g
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s.savedText(rec, "GEO data"), nil, nil
}

func (s *Service) toolSetNarrative(_ context.Context, req *mcp.CallToolRequest, in narrativeInput) (*mcp.CallToolResult, any, error) {
	mode, err := checkMode(in.Mode, "merge", "append", "replace")
	if err != nil {
		return nil, nil, err
	}
	raw := rawArg(req, "narrative")
	rec, err := s.update(in.ReportID, func(r *model.Report) error {
		var existing *model.Narrative
		if mode != "replace" && r.Narrative != nil {
			existing = r.Narrative
		}
		var n model.Narrative
		var ex any
		if existing != nil {
			ex = existing
		}
		if err := mergeObject(ex, raw, mode == "append", &n); err != nil {
			return fmt.Errorf("narrative: %w", err)
		}
		// section_notes are merged per key (unless replace).
		if existing != nil && len(existing.SectionNotes) > 0 && in.Narrative.SectionNotes != nil {
			notes := map[string]string{}
			for k, v := range existing.SectionNotes {
				notes[k] = v
			}
			for k, v := range in.Narrative.SectionNotes {
				if v == "" {
					delete(notes, k)
				} else {
					notes[k] = v
				}
			}
			n.SectionNotes = notes
		}
		r.Narrative = &n
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s.savedText(rec, "Narrative"), nil, nil
}

func (s *Service) toolAddSection(_ context.Context, _ *mcp.CallToolRequest, in addSectionInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Title) == "" {
		return nil, nil, errors.New("title is required")
	}
	var id string
	rec, err := s.update(in.ReportID, func(r *model.Report) error {
		id = store.Slugify(in.ID, 40)
		if id == "" {
			id = store.Slugify(in.Title, 40)
		}
		if id == "" {
			id = "section"
		}
		base := id
		for n := 2; sectionIndex(r, id) >= 0; n++ {
			id = fmt.Sprintf("%s-%d", base, n)
		}
		r.Sections = append(r.Sections, model.TextSection{ID: id, Title: in.Title, Body: in.Body, After: in.After, Style: in.Style})
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	res := s.savedText(rec, fmt.Sprintf("Section %q (id %s)", in.Title, id))
	res.StructuredContent.(map[string]any)["section_id"] = id
	return res, nil, nil
}

func sectionIndex(r *model.Report, id string) int {
	for i, s := range r.Sections {
		if s.ID == id {
			return i
		}
	}
	return -1
}

func (s *Service) toolUpdateSection(_ context.Context, _ *mcp.CallToolRequest, in updateSectionInput) (*mcp.CallToolResult, any, error) {
	rec, err := s.update(in.ReportID, func(r *model.Report) error {
		r.Normalize() // assigns ids to sections created without one
		i := sectionIndex(r, in.ID)
		if i < 0 {
			return fmt.Errorf("section %q not found (existing: %s)", in.ID, sectionIDs(r))
		}
		sec := &r.Sections[i]
		if in.Title != nil {
			sec.Title = *in.Title
		}
		if in.Body != nil {
			sec.Body = *in.Body
		}
		if in.After != nil {
			sec.After = *in.After
		}
		if in.Style != nil {
			sec.Style = *in.Style
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s.savedText(rec, "Section "+in.ID), nil, nil
}

func sectionIDs(r *model.Report) string {
	var ids []string
	for _, s := range r.Sections {
		ids = append(ids, s.ID)
	}
	if len(ids) == 0 {
		return "none"
	}
	return strings.Join(ids, ", ")
}

func (s *Service) toolRemoveSection(_ context.Context, _ *mcp.CallToolRequest, in removeSectionInput) (*mcp.CallToolResult, any, error) {
	rec, err := s.update(in.ReportID, func(r *model.Report) error {
		r.Normalize()
		i := sectionIndex(r, in.ID)
		if i < 0 {
			return fmt.Errorf("section %q not found (existing: %s)", in.ID, sectionIDs(r))
		}
		r.Sections = append(r.Sections[:i], r.Sections[i+1:]...)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s.savedText(rec, "Removal of section "+in.ID), nil, nil
}

func (s *Service) toolSetOptions(_ context.Context, req *mcp.CallToolRequest, in optionsInput) (*mcp.CallToolResult, any, error) {
	raw := rawArg(req, "options")
	rec, err := s.update(in.ReportID, func(r *model.Report) error {
		var existing any
		if r.Options != nil {
			existing = r.Options
		}
		var o model.Options
		if err := mergeObject(existing, raw, false, &o); err != nil {
			return fmt.Errorf("options: %w", err)
		}
		r.Options = &o
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s.savedText(rec, "Options"), nil, nil
}

func guessGSCKind(kind, fileName string) importer.GSCKind {
	if k := strings.TrimSpace(kind); k != "" {
		return importer.GSCKind(k)
	}
	if fileName != "" {
		return importer.GuessGSCKind(filepath.Base(fileName))
	}
	return ""
}

func (s *Service) toolImportGSC(_ context.Context, _ *mcp.CallToolRequest, in importGSCInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.CSV) == "" {
		return nil, nil, errors.New("csv is empty: pass the full text content of the CSV file")
	}
	t, err := importer.ParseGSCCSV([]byte(in.CSV), guessGSCKind(in.Kind, in.FileName))
	if err != nil {
		return nil, nil, fmt.Errorf("cannot parse the Search Console CSV: %w", err)
	}
	if t == nil {
		return nil, nil, errors.New("the CSV could not be recognized as a Search Console export: pass kind, or use set_search_console")
	}
	rec, err := s.update(in.ReportID, func(r *model.Report) error {
		r.SearchConsole = importer.ApplyGSCTable(r.SearchConsole, t)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	what := fmt.Sprintf("Search Console %s table (%d rows, %d days", t.Kind, len(t.Rows), len(t.Daily))
	if len(t.PreviousDaily) > 0 {
		what += fmt.Sprintf(", %d comparison days", len(t.PreviousDaily))
	}
	what += ")"
	res := s.savedText(rec, what)
	res.StructuredContent.(map[string]any)["kind"] = t.Kind
	res.StructuredContent.(map[string]any)["rows"] = len(t.Rows)
	res.StructuredContent.(map[string]any)["days"] = len(t.Daily)
	return res, nil, nil
}

func (s *Service) toolImportGA(_ context.Context, _ *mcp.CallToolRequest, in importGAInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.CSV) == "" {
		return nil, nil, errors.New("csv is empty: pass the full text content of the CSV file")
	}
	var counts string
	rec, err := s.update(in.ReportID, func(r *model.Report) error {
		a, err := importer.ParseGA4CSV([]byte(in.CSV), importer.GA4Kind(strings.ToLower(strings.TrimSpace(in.Kind))), r.Analytics)
		if err != nil {
			return fmt.Errorf("cannot parse the GA4 CSV: %w", err)
		}
		if a == nil {
			return errors.New("the CSV could not be recognized as a GA4 export: pass kind, or use set_analytics")
		}
		r.Analytics = a
		counts = fmt.Sprintf("%d channels, %d sources, %d landing pages, %d days", len(a.Channels), len(a.Sources), len(a.LandingPages), len(a.Daily))
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s.savedText(rec, "GA4 data ("+counts+" now in the report)"), nil, nil
}

func (s *Service) toolValidate(_ context.Context, _ *mcp.CallToolRequest, in reportIDInput) (*mcp.CallToolResult, any, error) {
	rec, err := s.get(in.ReportID)
	if err != nil {
		return nil, nil, err
	}
	errs, warns := rec.Report.Validate()
	c := CheckCompleteness(&rec.Report)
	var b strings.Builder
	if len(errs) == 0 {
		b.WriteString("No blocking error: the report can be rendered.\n")
	} else {
		fmt.Fprintf(&b, "Errors (must be fixed):\n- %s\n", strings.Join(errs, "\n- "))
	}
	if len(warns) > 0 {
		fmt.Fprintf(&b, "Warnings:\n- %s\n", strings.Join(warns, "\n- "))
	}
	fmt.Fprintf(&b, "Filled: %s\nMissing (optional): %s\n", strings.Join(c.Filled, ", "), strings.Join(c.Missing, ", "))
	if len(c.Suggestions) > 0 {
		fmt.Fprintf(&b, "Suggestions:\n- %s\n", strings.Join(c.Suggestions, "\n- "))
	}
	return textResult(b.String(), map[string]any{
		"report_id": rec.ID, "valid": len(errs) == 0, "errors": nonNil(errs), "warnings": nonNil(warns), "completeness": c,
	}), nil, nil
}

// RenderResult is the structured output of render_report / generate_report.
type RenderResult struct {
	ReportID string   `json:"report_id"`
	Language string   `json:"language"`
	SiteName string   `json:"site_name"`
	Period   string   `json:"period"`
	Links    Links    `json:"links"`
	HTMLFile string   `json:"html_file,omitempty"`
	PDFFile  string   `json:"pdf_file,omitempty"`
	PDFError string   `json:"pdf_error,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
	Summary  Summary  `json:"summary"`
}

func (s *Service) toolRender(ctx context.Context, req *mcp.CallToolRequest, in renderInput) (*mcp.CallToolResult, any, error) {
	rec, err := s.get(in.ReportID)
	if err != nil {
		return nil, nil, err
	}
	return s.renderRecord(ctx, req, rec, in.Language, in.Format, in.IncludeHTML, in.IncludePDF)
}

func (s *Service) toolGenerate(ctx context.Context, req *mcp.CallToolRequest, in generateInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Report.Meta.SiteName) == "" {
		return nil, nil, errors.New("report.meta.site_name is required")
	}
	var rec *store.Record
	var err error
	if in.ReportID != "" {
		rec, err = s.update(in.ReportID, func(r *model.Report) error { *r = in.Report; return nil })
	} else {
		rec, err = s.store.Create(&in.Report)
	}
	if err != nil {
		return nil, nil, err
	}
	return s.renderRecord(ctx, req, rec, in.Language, in.Format, in.IncludeHTML, in.IncludePDF)
}

func (s *Service) renderRecord(ctx context.Context, req *mcp.CallToolRequest, rec *store.Record, lang, format string, includeHTML, includePDF bool) (*mcp.CallToolResult, any, error) {
	format, err := checkMode(format, "html", "pdf", "both")
	if err != nil {
		return nil, nil, fmt.Errorf("format: %w", err)
	}
	errs, warns := rec.Report.Validate()
	if len(errs) > 0 {
		return nil, nil, fmt.Errorf("the report has blocking errors, fix them first: %s", strings.Join(errs, "; "))
	}
	rec = s.EnsureFavicon(ctx, rec)
	if lang == "" {
		lang = rec.Report.Lang()
	}
	lang = normLang(lang)
	base := s.baseURL(req)
	links := LinksFor(base, rec, lang)

	html, err := s.RenderHTML(rec, lang, links)
	if err != nil {
		return nil, nil, fmt.Errorf("rendering failed: %w", err)
	}
	htmlPath, err := s.store.SaveHTML(rec.ID, lang, html)
	if err != nil {
		return nil, nil, err
	}
	_ = s.store.MarkRendered(rec.ID, nowUTC())

	out := RenderResult{
		ReportID: rec.ID,
		Language: lang,
		SiteName: rec.Report.Meta.SiteName,
		Period:   rec.Report.Meta.Period.Start + " → " + rec.Report.Meta.Period.End,
		Links:    links,
		Warnings: warns,
		Summary:  Summarize(rec.Report),
	}
	if base == "" {
		out.HTMLFile = htmlPath
	}
	var pdfBytes []byte
	if format != "html" || includePDF {
		b, p, err := s.RenderPDF(ctx, rec, lang)
		if err != nil {
			out.PDFError = err.Error()
		} else {
			pdfBytes = b
			if base == "" {
				out.PDFFile = p
			}
		}
	}

	var t strings.Builder
	fmt.Fprintf(&t, "Report rendered: %s (%s, %s).\n", out.SiteName, out.Period, lang)
	if links.HTML != "" {
		fmt.Fprintf(&t, "View online: %s\nDownload HTML: %s\nDownload PDF: %s\n", links.HTML, links.HTMLDownload, links.PDF)
		t.WriteString("These links are public but unguessable: share them with the user.\n")
	} else {
		fmt.Fprintf(&t, "HTML file: %s\n", htmlPath)
		if out.PDFFile != "" {
			fmt.Fprintf(&t, "PDF file: %s\n", out.PDFFile)
		} else if format == "html" {
			t.WriteString("PDF: call render_report with format=pdf to generate it (needs Chromium / Google Chrome).\n")
		}
		t.WriteString("(No public URL is known in local mode: set PUBLIC_URL or use the HTTP server for shareable links.)\n")
	}
	if out.PDFError != "" {
		fmt.Fprintf(&t, "PDF could not be generated (the HTML export works): %s\n", out.PDFError)
	}
	if len(warns) > 0 {
		fmt.Fprintf(&t, "Warnings: %s\n", strings.Join(warns, "; "))
	}
	t.WriteString("\n")
	t.WriteString(out.Summary.Text())
	t.WriteString("\nComment the key takeaways for the user in plain language.")

	res := textResult(t.String(), out)
	if includeHTML {
		uri := links.HTML
		if uri == "" {
			uri = "file://" + filepath.ToSlash(htmlPath)
		}
		res.Content = append(res.Content, &mcp.EmbeddedResource{Resource: &mcp.ResourceContents{
			URI: uri, MIMEType: "text/html", Text: string(html),
		}})
	}
	if includePDF && pdfBytes != nil {
		if len(pdfBytes) <= maxInlinePDF {
			uri := links.PDF
			if uri == "" {
				uri = "file://" + filepath.ToSlash(out.PDFFile)
			}
			res.Content = append(res.Content, &mcp.EmbeddedResource{Resource: &mcp.ResourceContents{
				URI: uri, MIMEType: "application/pdf", Blob: pdfBytes,
			}})
		} else {
			res.Content = append(res.Content, &mcp.TextContent{Text: fmt.Sprintf("The PDF is %d MB, too large to embed: use the PDF link.", len(pdfBytes)>>20)})
		}
	}
	if s.cfg.MCPApps && links.HTML == "" {
		// Stdio mode: the inline viewer cannot load a URL, give it the HTML
		// out-of-band (_meta is not shown to the model).
		res.Meta = mcp.Meta{"github.com/flocom/SEO-GEO-Report/html_base64": base64.StdEncoding.EncodeToString(html)}
	}
	return res, nil, nil
}

func (s *Service) toolGet(_ context.Context, req *mcp.CallToolRequest, in reportIDInput) (*mcp.CallToolResult, any, error) {
	rec, err := s.get(in.ReportID)
	if err != nil {
		return nil, nil, err
	}
	out := map[string]any{
		"report_id":   rec.ID,
		"created_at":  rec.CreatedAt,
		"updated_at":  rec.UpdatedAt,
		"rendered_at": rec.RenderedAt,
		"links":       LinksFor(s.baseURL(req), rec, ""),
		"report":      rec.Report,
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	return textResult(string(b), out), nil, nil
}

func (s *Service) toolList(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	list, err := s.store.List()
	if err != nil {
		return nil, nil, err
	}
	var b strings.Builder
	if len(list) == 0 {
		b.WriteString("No report yet: call create_report (or generate_report).")
	}
	for _, r := range list {
		fmt.Fprintf(&b, "- %s — %s (%s → %s, %s), updated %s\n", r.ID, r.SiteName, r.Period.Start, r.Period.End, r.Language, r.UpdatedAt.Format("2006-01-02 15:04"))
	}
	return textResult(b.String(), map[string]any{"reports": list}), nil, nil
}

func (s *Service) toolDuplicate(_ context.Context, req *mcp.CallToolRequest, in duplicateInput) (*mcp.CallToolResult, any, error) {
	rec, err := s.store.Duplicate(strings.TrimSpace(in.ReportID), store.DuplicateOptions{
		KeepData: in.KeepData, KeepNarrative: in.KeepNarrative,
		Period: in.Period, ComparisonPeriod: in.ComparisonPeriod, Title: in.Title,
	})
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil, fmt.Errorf("report %q not found", in.ReportID)
	}
	if err != nil {
		return nil, nil, err
	}
	text := fmt.Sprintf("Report duplicated. New report_id: %s (period %s → %s).", rec.ID, rec.Report.Meta.Period.Start, rec.Report.Meta.Period.End)
	if in.Period == nil {
		text += " Set the new period with update_meta."
	}
	if !in.KeepData {
		text += " Data was cleared: fill it with set_search_console / set_analytics / set_geo."
	}
	return textResult(text, map[string]any{"report_id": rec.ID, "links": LinksFor(s.baseURL(req), rec, "")}), nil, nil
}

func (s *Service) toolDelete(_ context.Context, _ *mcp.CallToolRequest, in deleteInput) (*mcp.CallToolResult, any, error) {
	if !in.Confirm {
		return nil, nil, errors.New("deletion not confirmed: ask the user, then call again with confirm=true")
	}
	if err := s.store.Delete(strings.TrimSpace(in.ReportID)); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil, fmt.Errorf("report %q not found", in.ReportID)
		}
		return nil, nil, err
	}
	return textResult("Report "+in.ReportID+" deleted.", map[string]any{"report_id": in.ReportID, "deleted": true}), nil, nil
}
