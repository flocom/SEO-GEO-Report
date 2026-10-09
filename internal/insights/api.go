// Package insights derives automatic, localized observations from report data:
// trends, wins, alerts, AI-assistant traffic detection and a progress index.
package insights

import "github.com/flocom/SEO-GEO-Report/internal/model"

// Tone qualifies an insight.
type Tone string

const (
	Positive Tone = "positive"
	Negative Tone = "negative"
	Neutral  Tone = "neutral"
)

// Insight is one automatic observation, already written in the report language.
type Insight struct {
	Tone    Tone    `json:"tone"`
	Section string  `json:"section"` // built-in section id it relates to (model.SectionIDs)
	Text    string  `json:"text"`    // plain text, one sentence, localized
	Weight  float64 `json:"weight"`  // importance, higher first
}

// KPIStatus is the qualitative evolution of a KPI. For metrics where lower is
// better (average position) the tone is inverted.
type KPIStatus struct {
	Key      string  `json:"key"` // e.g. "gsc.clicks", "ga4.sessions", "geo.ai_sessions"
	Tone     Tone    `json:"tone"`
	DeltaPct float64 `json:"delta_pct"`
	HasDelta bool    `json:"has_delta"`
}

// Analysis is the result of Analyze.
type Analysis struct {
	Insights []Insight `json:"insights"` // sorted by weight desc

	// Progress is a composite progress index in [0,100]: 50 = stable, above
	// 50 = progress, below = decline. Nil when no comparison data exists.
	Progress *float64 `json:"progress,omitempty"`

	// KPIs gives the tone of every headline metric found in the report.
	KPIs map[string]KPIStatus `json:"kpis"`

	// AIReferrals is geo.ai_referrals or, when empty, rows detected from
	// analytics.sources (chatgpt.com, perplexity.ai, gemini.google.com, ...),
	// aggregated by platform and sorted by sessions desc.
	AIReferrals []model.AIReferralRow `json:"ai_referrals"`
	AISessions  *model.Metric         `json:"ai_sessions,omitempty"` // total of AIReferrals

	// AIShare is the share of AI-referred sessions among all sessions, percent.
	AIShare *float64 `json:"ai_share,omitempty"`

	// Positions is search_console.position_distribution or, when omitted,
	// computed from search_console.queries (previous from prev_position).
	Positions *model.PositionDistribution `json:"positions,omitempty"`

	// CitationRate is the share of citation checks where the site is cited,
	// percent, with previous rate computed from previously_cited when present.
	CitationRate *model.Metric `json:"citation_rate,omitempty"`

	// Winners and Losers are queries with the largest click gains / losses.
	Winners []model.GSCRow `json:"winners,omitempty"`
	Losers  []model.GSCRow `json:"losers,omitempty"`
}

// Analyze computes the analysis. r must be normalized (r.Normalize()).
//
// It never panics on partial data: every section of the report may be nil or
// empty. Insights are written in r.Lang() and are empty when
// options.auto_insights is false.
func Analyze(r *model.Report) Analysis { return analyze(r) }

// DetectAIPlatform maps a GA4 source (e.g. "chatgpt.com", "perplexity",
// "copilot.microsoft.com") to an AI platform name, or "" if not an AI source.
//
// Matching is case-insensitive and tolerates "chatgpt.com / referral" style
// values, full URLs and "utm_source=chatgpt.com" query strings.
func DetectAIPlatform(source string) string { return detectAIPlatform(source) }
