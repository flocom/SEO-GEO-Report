// Package model defines the input data of a SEO/GEO visibility report.
//
// Every value is supplied by the user (or by Claude through MCP): the tool never
// connects to Google. Data typically comes from Google Search Console and
// Google Analytics 4 exports, plus optional manual GEO checks (AI citations).
//
// Conventions:
//   - Dates are "YYYY-MM-DD".
//   - Percentages (CTR, engagement rate, share of voice) are expressed in percent:
//     3.2 means 3.2 %.
//   - Durations are in seconds.
//   - A Metric holds the current value and, optionally, the value for the
//     comparison period, which is what drives the "progress" visuals.
package model

// Report is the complete input of a report.
type Report struct {
	Meta          Meta           `json:"meta" jsonschema:"Report identity: site, period, language, branding"`
	SearchConsole *SearchConsole `json:"search_console,omitempty" jsonschema:"Data exported from Google Search Console (organic Google search performance)"`
	Analytics     *Analytics     `json:"analytics,omitempty" jsonschema:"Data exported from Google Analytics 4 (traffic, engagement, conversions)"`
	GEO           *GEO           `json:"geo,omitempty" jsonschema:"Generative Engine Optimization data: traffic from AI assistants, citation checks in AI answers, share of voice"`
	Narrative     *Narrative     `json:"narrative,omitempty" jsonschema:"Free text written by the analyst: executive summary, highlights, actions, recommendations, per-section notes"`
	Sections      []TextSection  `json:"sections,omitempty" jsonschema:"Additional free-text sections (Markdown) inserted anywhere in the report"`
	Options       *Options       `json:"options,omitempty" jsonschema:"Display options"`
}

// Meta identifies the report.
type Meta struct {
	Title            string  `json:"title,omitempty" jsonschema:"Report title. Defaults to a localized 'SEO & GEO visibility report'"`
	SiteName         string  `json:"site_name" jsonschema:"Name of the website or brand"`
	SiteURL          string  `json:"site_url,omitempty" jsonschema:"Website URL, e.g. https://www.example.com"`
	Language         string  `json:"language,omitempty" jsonschema:"Report language: 'fr' or 'en'. Defaults to 'fr'"`
	Period           Period  `json:"period" jsonschema:"Analysed period"`
	ComparisonPeriod *Period `json:"comparison_period,omitempty" jsonschema:"Period used for comparison (previous period or same period last year)"`
	ComparisonLabel  string  `json:"comparison_label,omitempty" jsonschema:"Optional human label of the comparison, e.g. 'vs same period last year'"`
	PreparedBy       string  `json:"prepared_by,omitempty" jsonschema:"Author or agency name"`
	PreparedFor      string  `json:"prepared_for,omitempty" jsonschema:"Client name"`
	LogoURL          string  `json:"logo_url,omitempty" jsonschema:"Logo as an https URL or a data: URI"`
	BrandColor       string  `json:"brand_color,omitempty" jsonschema:"Accent color as hex, e.g. #2563eb"`
	Currency         string  `json:"currency,omitempty" jsonschema:"ISO currency code for revenue, e.g. EUR. Defaults to EUR"`
}

// Period is an inclusive date range.
type Period struct {
	Start string `json:"start" jsonschema:"First day, YYYY-MM-DD"`
	End   string `json:"end" jsonschema:"Last day, YYYY-MM-DD"`
}

// Metric is a value for the current period with an optional comparison value.
type Metric struct {
	Current  float64  `json:"current" jsonschema:"Value for the analysed period"`
	Previous *float64 `json:"previous,omitempty" jsonschema:"Value for the comparison period (omit if unknown)"`
}

// ---------------------------------------------------------------------------
// Google Search Console

// SearchConsole holds Google Search Console performance data.
type SearchConsole struct {
	Totals               GSCTotals             `json:"totals" jsonschema:"Totals for the whole property over the period"`
	Daily                []GSCDailyPoint       `json:"daily,omitempty" jsonschema:"Daily performance for the analysed period (chronological)"`
	PreviousDaily        []GSCDailyPoint       `json:"previous_daily,omitempty" jsonschema:"Daily performance for the comparison period, same length as daily if possible (used as a dashed overlay)"`
	Queries              []GSCRow              `json:"queries,omitempty" jsonschema:"Top search queries (key = query text)"`
	Pages                []GSCRow              `json:"pages,omitempty" jsonschema:"Top pages (key = URL)"`
	Countries            []GSCRow              `json:"countries,omitempty" jsonschema:"Performance by country (key = country name or ISO code)"`
	Devices              []GSCRow              `json:"devices,omitempty" jsonschema:"Performance by device (key = desktop, mobile, tablet)"`
	SearchAppearance     []GSCRow              `json:"search_appearance,omitempty" jsonschema:"Performance by search appearance (rich results, videos...)"`
	BrandSplit           *BrandSplit           `json:"brand_split,omitempty" jsonschema:"Clicks from branded vs non-branded queries"`
	PositionDistribution *PositionDistribution `json:"position_distribution,omitempty" jsonschema:"Number of ranking queries per position bucket. Computed from queries when omitted"`
	Indexing             *Indexing             `json:"indexing,omitempty" jsonschema:"Page indexing report"`
	CoreWebVitals        *CoreWebVitals        `json:"core_web_vitals,omitempty" jsonschema:"Core Web Vitals report (number of URLs per status)"`
}

// GSCTotals are the headline Search Console metrics.
type GSCTotals struct {
	Clicks      Metric  `json:"clicks" jsonschema:"Total clicks from Google search"`
	Impressions Metric  `json:"impressions" jsonschema:"Total impressions in Google search"`
	CTR         *Metric `json:"ctr,omitempty" jsonschema:"Average click-through rate in percent (3.2 = 3.2%). Computed from clicks/impressions when omitted"`
	Position    *Metric `json:"position,omitempty" jsonschema:"Average position (lower is better)"`
}

// GSCDailyPoint is one day of Search Console data.
type GSCDailyPoint struct {
	Date        string  `json:"date" jsonschema:"YYYY-MM-DD"`
	Clicks      float64 `json:"clicks"`
	Impressions float64 `json:"impressions"`
	CTR         float64 `json:"ctr,omitempty" jsonschema:"Percent"`
	Position    float64 `json:"position,omitempty"`
}

// GSCRow is a dimension row (query, page, country, device...).
type GSCRow struct {
	Key             string   `json:"key" jsonschema:"Dimension value: query text, page URL, country, device..."`
	Clicks          float64  `json:"clicks"`
	Impressions     float64  `json:"impressions"`
	CTR             float64  `json:"ctr,omitempty" jsonschema:"Percent. Computed when omitted"`
	Position        float64  `json:"position,omitempty" jsonschema:"Average position"`
	PrevClicks      *float64 `json:"prev_clicks,omitempty" jsonschema:"Clicks in the comparison period"`
	PrevImpressions *float64 `json:"prev_impressions,omitempty" jsonschema:"Impressions in the comparison period"`
	PrevPosition    *float64 `json:"prev_position,omitempty" jsonschema:"Average position in the comparison period"`
}

// BrandSplit separates branded and non-branded clicks.
type BrandSplit struct {
	Branded    Metric   `json:"branded" jsonschema:"Clicks on queries containing the brand name"`
	NonBranded Metric   `json:"non_branded" jsonschema:"Clicks on all other queries"`
	BrandTerms []string `json:"brand_terms,omitempty" jsonschema:"Terms used to classify branded queries"`
}

// PositionDistribution counts ranking queries per position bucket.
type PositionDistribution struct {
	Top3     Metric `json:"top3" jsonschema:"Queries ranking in positions 1-3"`
	Top10    Metric `json:"top4_10" jsonschema:"Queries ranking in positions 4-10"`
	Top20    Metric `json:"top11_20" jsonschema:"Queries ranking in positions 11-20"`
	Beyond20 Metric `json:"beyond20" jsonschema:"Queries ranking beyond position 20"`
}

// Indexing summarises the page indexing report.
type Indexing struct {
	Indexed    Metric          `json:"indexed" jsonschema:"Indexed pages"`
	NotIndexed *Metric         `json:"not_indexed,omitempty" jsonschema:"Pages not indexed"`
	Issues     []IndexingIssue `json:"issues,omitempty" jsonschema:"Reasons why pages are not indexed"`
	Sitemaps   []Sitemap       `json:"sitemaps,omitempty"`
}

// IndexingIssue is one 'why pages aren't indexed' reason.
type IndexingIssue struct {
	Reason string  `json:"reason" jsonschema:"e.g. 'Crawled - currently not indexed'"`
	Pages  float64 `json:"pages"`
}

// Sitemap is a submitted sitemap.
type Sitemap struct {
	URL        string  `json:"url"`
	Discovered float64 `json:"discovered_urls,omitempty"`
	Status     string  `json:"status,omitempty"`
}

// CoreWebVitals contains URL counts per device.
type CoreWebVitals struct {
	Mobile  *CWVStatus `json:"mobile,omitempty"`
	Desktop *CWVStatus `json:"desktop,omitempty"`
}

// CWVStatus counts URLs per Core Web Vitals status.
type CWVStatus struct {
	Good             float64 `json:"good" jsonschema:"URLs rated Good"`
	NeedsImprovement float64 `json:"needs_improvement" jsonschema:"URLs rated Needs improvement"`
	Poor             float64 `json:"poor" jsonschema:"URLs rated Poor"`
}

// ---------------------------------------------------------------------------
// Google Analytics 4

// Analytics holds GA4 data.
type Analytics struct {
	Totals        GA4Totals        `json:"totals" jsonschema:"Site-wide totals, all channels"`
	OrganicTotals *GA4Totals       `json:"organic_totals,omitempty" jsonschema:"Totals restricted to the Organic Search channel"`
	Daily         []GA4DailyPoint  `json:"daily,omitempty" jsonschema:"Daily sessions (chronological)"`
	PreviousDaily []GA4DailyPoint  `json:"previous_daily,omitempty" jsonschema:"Daily sessions for the comparison period"`
	Channels      []ChannelRow     `json:"channels,omitempty" jsonschema:"Sessions by default channel group (Organic Search, Direct, Referral, Paid Search, Organic Social, Email...)"`
	Sources       []SourceRow      `json:"sources,omitempty" jsonschema:"Sessions by source / medium. AI assistants (chatgpt.com, perplexity.ai...) are detected automatically from this list"`
	LandingPages  []LandingPageRow `json:"landing_pages,omitempty" jsonschema:"Top landing pages (ideally for organic traffic)"`
}

// GA4Totals are the headline GA4 metrics. Every metric is optional.
type GA4Totals struct {
	Sessions          *Metric `json:"sessions,omitempty"`
	Users             *Metric `json:"users,omitempty" jsonschema:"Active users"`
	NewUsers          *Metric `json:"new_users,omitempty"`
	EngagedSessions   *Metric `json:"engaged_sessions,omitempty"`
	EngagementRate    *Metric `json:"engagement_rate,omitempty" jsonschema:"Percent (62.5 = 62.5%)"`
	AvgEngagementTime *Metric `json:"avg_engagement_time,omitempty" jsonschema:"Average engagement time per session, in seconds"`
	PageViews         *Metric `json:"page_views,omitempty" jsonschema:"Views"`
	KeyEvents         *Metric `json:"key_events,omitempty" jsonschema:"Key events (conversions)"`
	Revenue           *Metric `json:"revenue,omitempty" jsonschema:"Total revenue in meta.currency"`
}

// GA4DailyPoint is one day of GA4 sessions.
type GA4DailyPoint struct {
	Date            string  `json:"date" jsonschema:"YYYY-MM-DD"`
	Sessions        float64 `json:"sessions"`
	OrganicSessions float64 `json:"organic_sessions,omitempty"`
	AISessions      float64 `json:"ai_sessions,omitempty" jsonschema:"Sessions referred by AI assistants"`
}

// ChannelRow is one GA4 default channel group.
type ChannelRow struct {
	Channel        string   `json:"channel" jsonschema:"e.g. Organic Search, Direct, Referral"`
	Sessions       float64  `json:"sessions"`
	PrevSessions   *float64 `json:"prev_sessions,omitempty"`
	Users          float64  `json:"users,omitempty"`
	EngagementRate float64  `json:"engagement_rate,omitempty" jsonschema:"Percent"`
	KeyEvents      float64  `json:"key_events,omitempty"`
	Revenue        float64  `json:"revenue,omitempty"`
}

// SourceRow is one GA4 source / medium pair.
type SourceRow struct {
	Source         string   `json:"source" jsonschema:"e.g. google, chatgpt.com, perplexity.ai"`
	Medium         string   `json:"medium,omitempty" jsonschema:"e.g. organic, referral"`
	Sessions       float64  `json:"sessions"`
	PrevSessions   *float64 `json:"prev_sessions,omitempty"`
	Users          float64  `json:"users,omitempty"`
	EngagementRate float64  `json:"engagement_rate,omitempty" jsonschema:"Percent"`
	KeyEvents      float64  `json:"key_events,omitempty"`
	Revenue        float64  `json:"revenue,omitempty"`
}

// LandingPageRow is one GA4 landing page.
type LandingPageRow struct {
	Page           string   `json:"page" jsonschema:"Landing page path or URL"`
	Sessions       float64  `json:"sessions"`
	PrevSessions   *float64 `json:"prev_sessions,omitempty"`
	EngagementRate float64  `json:"engagement_rate,omitempty" jsonschema:"Percent"`
	KeyEvents      float64  `json:"key_events,omitempty"`
	Revenue        float64  `json:"revenue,omitempty"`
}

// ---------------------------------------------------------------------------
// GEO (Generative Engine Optimization)

// GEO holds visibility data in AI assistants and answer engines.
type GEO struct {
	AIReferrals  []AIReferralRow `json:"ai_referrals,omitempty" jsonschema:"Sessions referred by AI assistants. When omitted they are detected from analytics.sources"`
	AIDaily      []DailyValue    `json:"ai_daily,omitempty" jsonschema:"Daily sessions from AI assistants"`
	Citations    []CitationCheck `json:"citations,omitempty" jsonschema:"Manual checks: is the site cited when a given prompt is asked to an AI engine?"`
	ShareOfVoice []ShareOfVoice  `json:"share_of_voice,omitempty" jsonschema:"Share of AI answers mentioning each brand (self + competitors)"`
	AIOverviews  []AIOverviewRow `json:"ai_overviews,omitempty" jsonschema:"Queries checked for Google AI Overviews presence and citation"`
	AICrawlers   []CrawlerRow    `json:"ai_crawlers,omitempty" jsonschema:"AI crawler hits from server logs (GPTBot, ClaudeBot, PerplexityBot, Google-Extended...)"`
}

// AIReferralRow is traffic from one AI assistant.
type AIReferralRow struct {
	Platform       string   `json:"platform" jsonschema:"ChatGPT, Perplexity, Gemini, Copilot, Claude, Mistral Le Chat, DeepSeek..."`
	Source         string   `json:"source,omitempty" jsonschema:"GA4 source, e.g. chatgpt.com"`
	Sessions       float64  `json:"sessions"`
	PrevSessions   *float64 `json:"prev_sessions,omitempty"`
	Users          float64  `json:"users,omitempty"`
	EngagementRate float64  `json:"engagement_rate,omitempty" jsonschema:"Percent"`
	KeyEvents      float64  `json:"key_events,omitempty"`
	Revenue        float64  `json:"revenue,omitempty"`
}

// DailyValue is a generic dated value.
type DailyValue struct {
	Date  string  `json:"date" jsonschema:"YYYY-MM-DD"`
	Value float64 `json:"value"`
}

// CitationCheck is one manual prompt test in an AI engine.
type CitationCheck struct {
	Prompt           string   `json:"prompt" jsonschema:"The question asked to the AI engine"`
	Engine           string   `json:"engine" jsonschema:"ChatGPT, Perplexity, Gemini, Google AI Overviews, Google AI Mode, Copilot, Claude..."`
	Cited            bool     `json:"cited" jsonschema:"True if the site or brand is cited/linked in the answer"`
	Mentioned        bool     `json:"mentioned,omitempty" jsonschema:"True if the brand is named even without a link"`
	Position         *int     `json:"position,omitempty" jsonschema:"Rank of the citation among sources (1 = first)"`
	PreviouslyCited  *bool    `json:"previously_cited,omitempty" jsonschema:"Result of the same check in the previous report"`
	CompetitorsCited []string `json:"competitors_cited,omitempty"`
	Date             string   `json:"date,omitempty" jsonschema:"YYYY-MM-DD of the check"`
	Notes            string   `json:"notes,omitempty"`
}

// ShareOfVoice is a brand's share of AI answers.
type ShareOfVoice struct {
	Brand    string   `json:"brand"`
	Share    float64  `json:"share" jsonschema:"Percent of answers mentioning the brand"`
	Previous *float64 `json:"previous,omitempty" jsonschema:"Share in the comparison period, percent"`
	IsSelf   bool     `json:"is_self,omitempty" jsonschema:"True for the brand the report is about"`
}

// AIOverviewRow is a query checked for Google AI Overviews.
type AIOverviewRow struct {
	Query           string `json:"query"`
	OverviewPresent bool   `json:"overview_present" jsonschema:"An AI Overview is displayed for this query"`
	Cited           bool   `json:"cited" jsonschema:"The site is cited in the AI Overview"`
	Notes           string `json:"notes,omitempty"`
}

// CrawlerRow counts hits from one AI crawler.
type CrawlerRow struct {
	Bot      string   `json:"bot" jsonschema:"e.g. GPTBot, OAI-SearchBot, ClaudeBot, PerplexityBot, Google-Extended"`
	Hits     float64  `json:"hits"`
	PrevHits *float64 `json:"prev_hits,omitempty"`
}

// ---------------------------------------------------------------------------
// Free text

// Narrative is the analyst's text. Text fields accept Markdown.
type Narrative struct {
	ExecutiveSummary string            `json:"executive_summary,omitempty" jsonschema:"Markdown summary shown at the top of the report"`
	Highlights       []string          `json:"highlights,omitempty" jsonschema:"Key wins of the period, one sentence each"`
	Concerns         []string          `json:"concerns,omitempty" jsonschema:"Points of attention, one sentence each"`
	Actions          []Action          `json:"actions,omitempty" jsonschema:"Work done or in progress during the period"`
	Recommendations  []Recommendation  `json:"recommendations,omitempty" jsonschema:"Next steps"`
	SectionNotes     map[string]string `json:"section_notes,omitempty" jsonschema:"Markdown commentary displayed inside a built-in section. Keys: summary, kpis, search_console, queries, pages, countries_devices, analytics, channels, landing_pages, geo, technical, actions, recommendations"`
}

// Action is a task performed for the site.
type Action struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty" jsonschema:"Markdown"`
	Date        string `json:"date,omitempty" jsonschema:"YYYY-MM-DD"`
	Status      string `json:"status,omitempty" jsonschema:"done, in_progress or planned. Defaults to done"`
	Category    string `json:"category,omitempty" jsonschema:"seo, geo, technical, content, netlinking, ux, tracking"`
	Impact      string `json:"impact,omitempty" jsonschema:"Observed or expected impact"`
}

// Recommendation is a suggested next step.
type Recommendation struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty" jsonschema:"Markdown"`
	Priority    string `json:"priority,omitempty" jsonschema:"high, medium or low"`
	Effort      string `json:"effort,omitempty" jsonschema:"low, medium or high"`
	Category    string `json:"category,omitempty" jsonschema:"seo, geo, technical, content, netlinking, ux, tracking"`
}

// TextSection is a custom Markdown section.
type TextSection struct {
	ID    string `json:"id,omitempty" jsonschema:"Identifier, generated when omitted"`
	Title string `json:"title" jsonschema:"Section title"`
	Body  string `json:"body" jsonschema:"Markdown content (lists, tables, links, bold...)"`
	After string `json:"after,omitempty" jsonschema:"Built-in section after which this one is inserted: summary, kpis, search_console, queries, pages, countries_devices, analytics, channels, landing_pages, geo, technical, actions, recommendations. Use 'start' or 'end'. Defaults to 'end'"`
	Style string `json:"style,omitempty" jsonschema:"plain, info, success, warning or note. Defaults to plain"`
}

// Options tune the display.
type Options struct {
	ShowGlossary   *bool    `json:"show_glossary,omitempty" jsonschema:"Show the glossary of terms at the end (default true)"`
	BeginnerHints  *bool    `json:"beginner_hints,omitempty" jsonschema:"Show 'how to read this' explanations next to charts (default true)"`
	HideSections   []string `json:"hide_sections,omitempty" jsonschema:"Built-in section ids to hide"`
	MaxTableRows   int      `json:"max_table_rows,omitempty" jsonschema:"Rows displayed per table (default 15)"`
	AutoInsights   *bool    `json:"auto_insights,omitempty" jsonschema:"Generate automatic insights from the numbers (default true)"`
}

// SectionIDs lists built-in sections in display order.
var SectionIDs = []string{
	"summary", "kpis", "search_console", "queries", "pages", "countries_devices",
	"analytics", "channels", "landing_pages", "geo", "technical",
	"actions", "recommendations", "glossary",
}
