package mcpserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/flocom/SEO-GEO-Report/internal/example"
	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// Instructions are sent to clients at initialization (claude.ai shows them to
// the model). Keep them short: the full guide is a tool / resource.
const Instructions = `SEO & GEO Report formats data YOU provide into a beautiful, bilingual (French/English) HTML + PDF report about a website's visibility in Google (Search Console) and Bing (Bing Webmaster Tools), its traffic (GA4), its products in Google Shopping (Merchant Center, optional) and its visibility in AI assistants (GEO: ChatGPT, Perplexity, Gemini, AI Overviews, Copilot...).
It NEVER connects to Google, Bing or any other service: all numbers come from the user (exports, screenshots, copy-paste) and you pass them through the tools.
Start with get_report_guide (workflow, field conventions, what to ask the user), then create_report -> set_search_console / set_bing / set_analytics / set_merchant_center / set_geo (or import_*_csv with raw CSV text) -> set_narrative -> validate_report -> render_report, and give the user the returned URL. generate_report does everything in one call when you already have all the data.
Conventions: percentages in percent (3.2 = 3.2 %), dates YYYY-MM-DD, durations in seconds, every metric = {"current": x, "previous": y} where previous is the comparison period (drives the progress visuals).`

// GuideMarkdown returns the full workflow guide, in English, for the model.
func GuideMarkdown(includeExample bool, lang string) string {
	var b strings.Builder
	b.WriteString(guideText)
	b.WriteString("\n## Section ids\n\nBuilt-in sections, in display order: `")
	b.WriteString(strings.Join(model.SectionIDs, "`, `"))
	b.WriteString("`.\nUse them in `narrative.section_notes` keys, `sections[].after` and `options.hide_sections`.\n")
	if includeExample {
		ex, err := CompactExample(lang)
		if err == nil {
			fmt.Fprintf(&b, "\n## Compact example (%s)\n\nArrays are truncated to 3 items; the full example is the resource `seogeo://example/%s`.\n\n```json\n%s\n```\n", normLang(lang), normLang(lang), ex)
		}
	}
	return b.String()
}

const guideText = `# SEO & GEO Report — guide for the assistant

This server turns data that the USER supplies into a polished, very visual report
(KPI cards with progress arrows, charts, tables, automatic insights, glossary for
beginners), in French or English, as an HTML page hosted by the server (shareable
link) and as a PDF. **It never fetches data itself** and has no access to Google:
you collect the numbers from the user and pass them in.

## Workflow

1. **Ask the user** (keep it short, one message):
   - site name and URL, report language (fr/en), period analysed and comparison
     period (previous period or same period last year), client / agency names (optional);
   - the exports they can share (see "What to ask for" below). Pasted tables,
     CSV files, screenshots and PDF exports all work: you read them and extract the numbers.
2. ` + "`create_report`" + ` with the meta → you get a ` + "`report_id`" + `.
3. Fill the data, section by section (each call can be repeated; ` + "`mode: \"merge\"`" + ` (default) only
   replaces the keys you send, ` + "`mode: \"replace\"`" + ` replaces the whole section):
   - ` + "`set_search_console`" + ` (or ` + "`import_search_console_csv`" + ` with the raw text of a GSC CSV export: Queries, Pages, Countries, Devices, Dates, Search appearance),
   - ` + "`set_bing`" + ` (or ` + "`import_bing_csv`" + ` with the raw text of a Bing Webmaster Tools CSV export: Search Performance keywords / pages / daily chart, AI Performance grounding queries / pages / daily chart),
   - ` + "`set_analytics`" + ` (or ` + "`import_analytics_csv`" + ` with a GA4 "Download CSV" export: channels, source/medium, landing pages, daily),
   - ` + "`set_merchant_center`" + ` (or ` + "`import_merchant_center_csv`" + ` with the raw text of a Google Merchant Center CSV export: performance by product / day / marketing method / category / country, products with their status, issues, price competitiveness) when the site sells products,
   - ` + "`set_geo`" + ` for AI visibility (AI referrals, citation checks, share of voice, AI Overviews, AI crawlers).
4. ` + "`set_narrative`" + `: write the executive summary, highlights, concerns, actions done and
   recommendations **based on the numbers** (be specific, cite figures, plain language for
   non-experts). Add free Markdown sections with ` + "`add_text_section`" + ` if useful.
5. ` + "`validate_report`" + ` → fix errors, consider warnings (missing previous values, ratios instead of percents...).
6. ` + "`render_report`" + ` → returns the public URL of the HTML report and of the PDF, plus a summary of
   KPIs and automatic insights. Give the links to the user and comment the key takeaways.
   Rendering again after a change updates the same URLs.
7. Next month: ` + "`duplicate_report`" + ` (keeps meta, branding, pending actions and recommendations; clears data) then fill the new data.

One-shot alternative: ` + "`generate_report`" + ` with the complete report JSON (same structure as
the resource ` + "`seogeo://schema`" + `) renders immediately.

## Conventions (important)

- **Percentages are in percent**: CTR 3.2 means 3.2 %, engagement rate 62.5 means 62.5 %.
  Never send ratios like 0.032. Share of voice and citation rates too.
- **Dates**: ` + "`YYYY-MM-DD`" + `. Daily arrays are chronological.
- **Durations**: seconds (GA4 "1m 32s" → 92).
- **Metric** objects: ` + "`{\"current\": 1234, \"previous\": 1100}`" + `. ` + "`previous`" + ` is the value for the comparison
  period; omit it when unknown (no progress shown). Rows use ` + "`prev_clicks`" + `, ` + "`prev_sessions`" + `,
  ` + "`prev_position`" + `... for the same purpose.
- **Average position**: lower is better; the report handles the inverted logic.
- **CTR** is computed from clicks/impressions when omitted; **position distribution** is computed from
  queries when omitted; **AI referrals** are detected automatically from ` + "`analytics.sources`" + `
  (chatgpt.com, perplexity.ai, gemini.google.com, copilot.microsoft.com, claude.ai...) when ` + "`geo.ai_referrals`" + ` is empty.
- Numbers must be plain JSON numbers (no thousands separators, no "%" signs).
- Free text fields accept Markdown. Write narrative texts in the report language.
- Only send what you know. Every section is optional; the report adapts and hides empty parts.
  Never invent numbers: if something is missing, ask the user or leave it out.

## What to ask the user for

**Google Search Console** (Performance > Search results; set the date range and "Compare";
then "Export" > CSV/Excel/Google Sheets — the zip contains Queries, Pages, Countries, Devices,
Search appearance, Dates):
- totals: clicks, impressions, average CTR, average position (current + previous),
- daily chart (Dates table) for the period and the comparison period,
- top queries and top pages (with comparison columns if possible),
- countries and devices,
- optional: Indexing > Pages (indexed / not indexed + reasons), Core Web Vitals (good / needs
  improvement / poor URLs, mobile and desktop), sitemaps, branded vs non-branded split
  (give the brand terms; you can compute it from the queries).

**Bing Webmaster Tools** (optional, if the site is verified there; Bing also powers DuckDuckGo,
Ecosia, Yahoo and Microsoft Copilot):
- Search Performance (set the date range and enable the comparison, then export): totals (clicks,
  impressions, average CTR, average position, current + previous), the daily chart, keywords and pages,
- AI Performance (citations of the site in Copilot and Bing AI answers): total citations and average
  cited pages (current + previous), the daily chart, grounding queries (citations, citation share,
  intent, topic) and cited pages. Total citations are computed from the daily chart or the pages
  when omitted.

**Google Merchant Center** (optional, only for sites that sell products: it adds a "Google Shopping"
section and a Shopping clicks card in the key indicators). In Merchant Center (Merchant Center Next),
set the date range and the comparison, then download each table as CSV:
- Performance (Analytics / "Performance" page): totals (clicks, impressions, CTR, conversions and
  conversion value if tracked, current + previous), the daily chart, the tables by product (title,
  item / offer ID), by marketing method / program (free listings vs Shopping ads, Images, YouTube),
  by category, brand and country,
- Products > All products (download: one row per product with its status) or the overview counts:
  approved, limited, not approved (disapproved), pending,
- Products > Needs attention / Diagnostics: issues with the number of products affected, severity
  (critical = disapproved, warning = limited, info = suggestion) and the programs affected,
- Price competitiveness (Analytics / Growth > Price competitiveness): your price vs the benchmark price
  per product.
Totals are computed from the daily chart, else from the program table, when omitted.

**Google Analytics 4** (Reports > Acquisition > Traffic acquisition, Engagement > Landing page;
with a comparison date range; "Share this report" > "Download file" > CSV):
- totals: sessions, active users, new users, engaged sessions, engagement rate, average engagement
  time, views, key events, revenue (all channels, and organic search only if possible),
- sessions by default channel group, by session source / medium (this is where AI assistants
  appear: chatgpt.com / referral, perplexity.ai...), top landing pages, daily sessions.

**GEO (visibility in AI assistants)** — manual checks the user (or you, if you have web
access) performed:
- prompts asked to ChatGPT, Perplexity, Gemini, Copilot, Google AI Overviews / AI Mode, Claude:
  is the site cited (linked)? mentioned? position among sources? competitors cited? (and the
  result of the same check last time → ` + "`previously_cited`" + `),
- share of voice: % of answers mentioning each brand (self + competitors),
- queries with an AI Overview and whether the site is cited,
- AI crawler hits from server logs (GPTBot, OAI-SearchBot, ClaudeBot, PerplexityBot,
  Google-Extended...) if available.

## Narrative tips

- Executive summary: 3-6 sentences, start with the overall trend, quantify (e.g. "+18 % clicks vs
  last quarter"), mention the main driver and the main risk.
- Highlights / concerns: one sentence each, with a figure.
- Actions: what was done during the period (status done / in_progress / planned, category seo,
  geo, technical, content, netlinking, ux, tracking).
- Recommendations: concrete next steps with priority (high/medium/low) and effort.
- ` + "`section_notes`" + `: short Markdown commentary displayed inside a built-in section (keys = section ids).
- Custom sections (` + "`add_text_section`" + `): methodology, glossary additions, competitor analysis...
  placed with ` + "`after`" + ` = a section id, ` + "`start`" + ` or ` + "`end`" + `; style plain, info, success, warning or note.

## Options

` + "`set_options`" + `: ` + "`hide_sections`" + ` (section ids), ` + "`max_table_rows`" + ` (default 15), ` + "`show_glossary`" + `,
` + "`beginner_hints`" + ` ("how to read this chart" explanations), ` + "`auto_insights`" + ` (automatic observations).
Branding: ` + "`meta.brand_color`" + ` (hex), ` + "`meta.logo_url`" + ` (https URL or data: URI), ` + "`meta.prepared_by`" + `, ` + "`meta.prepared_for`" + `.
`

// CompactExample returns the demo report as indented JSON with every array
// truncated to 3 items.
func CompactExample(lang string) (string, error) {
	b, err := example.DemoJSON(normLang(lang))
	if err != nil {
		return "", err
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return "", err
	}
	v = truncateArrays(v, 3)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

func truncateArrays(v any, n int) any {
	switch t := v.(type) {
	case []any:
		if len(t) > n {
			t = t[:n]
		}
		for i := range t {
			t[i] = truncateArrays(t[i], n)
		}
		return t
	case map[string]any:
		for k := range t {
			t[k] = truncateArrays(t[k], n)
		}
		return t
	default:
		return v
	}
}

// ExampleJSON returns the full demo report as indented JSON.
func ExampleJSON(lang string) ([]byte, error) {
	return example.DemoJSON(normLang(lang))
}

// SchemaJSON returns the JSON schema of model.Report.
func SchemaJSON() ([]byte, error) {
	s, err := jsonschema.For[model.Report](nil)
	if err != nil {
		return nil, err
	}
	s.Schema = "https://json-schema.org/draft/2020-12/schema"
	s.Title = "SEO & GEO report input"
	s.Description = "Input data of a SEO/GEO visibility report. Percentages are in percent (3.2 = 3.2 %), dates YYYY-MM-DD, durations in seconds."
	return json.MarshalIndent(s, "", "  ")
}

func normLang(l string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(l)), "en") {
		return "en"
	}
	return "fr"
}
