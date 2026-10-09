package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/flocom/SEO-GEO-Report/internal/favicon"
	"github.com/flocom/SEO-GEO-Report/internal/insights"
	"github.com/flocom/SEO-GEO-Report/internal/model"
	"github.com/flocom/SEO-GEO-Report/internal/pdf"
	"github.com/flocom/SEO-GEO-Report/internal/render"
	"github.com/flocom/SEO-GEO-Report/internal/store"
	"github.com/flocom/SEO-GEO-Report/internal/update"
)

// Config configures the MCP server.
type Config struct {
	Store *store.Store
	// PublicURL is the static public base URL, if known (PUBLIC_URL). In HTTP
	// mode the base URL is detected per request instead.
	PublicURL string
	// MCPApps enables the inline report viewer (MCP Apps extension).
	MCPApps bool
	Version string
	// Favicons fetches and embeds site icons (nil = disabled).
	Favicons *favicon.Resolver
	// Updates reports newer releases on the home page (nil = disabled).
	Updates *update.Checker
}

// Service holds the shared logic of the MCP tools and the HTTP endpoints.
type Service struct {
	cfg   Config
	store *store.Store
}

// NewService creates the service.
func NewService(cfg Config) *Service {
	if cfg.Version == "" {
		cfg.Version = "dev"
	}
	return &Service{cfg: cfg, store: cfg.Store}
}

// Links are the public URLs of a report.
type Links struct {
	HTML         string `json:"html_url,omitempty"`
	HTMLDownload string `json:"html_download_url,omitempty"`
	PDF          string `json:"pdf_url,omitempty"`
}

// LinksFor builds the public URLs of a record for a base URL ("" = unknown).
func LinksFor(base string, rec *store.Record, lang string) Links {
	if base == "" {
		return Links{}
	}
	q := ""
	if lang != "" && lang != rec.Report.Lang() {
		q = "?lang=" + url.QueryEscape(lang)
	}
	p := base + "/r/" + rec.ShareToken
	return Links{HTML: p + q, HTMLDownload: p + "/download" + q, PDF: p + ".pdf" + q}
}

// RenderHTML renders a record. links carries the download buttons shown in
// the page (empty = no buttons, e.g. for the PDF itself).
func (s *Service) RenderHTML(rec *store.Record, lang string, links Links) ([]byte, error) {
	r := deepCopy(rec.Report)
	html, err := render.HTML(&r, renderOptions(lang, links))
	if err != nil {
		return nil, err
	}
	if len(html) == 0 {
		return nil, fmt.Errorf("renderer returned an empty document")
	}
	return html, nil
}

// EnsureFavicon resolves the site icon when needed (see favicon.Apply) and
// stores it in the report so it is fetched only once. Failures are silent.
func (s *Service) EnsureFavicon(ctx context.Context, rec *store.Record) *store.Record {
	if s.cfg.Favicons == nil || !favicon.Needed(&rec.Report) {
		return rec
	}
	r := deepCopy(rec.Report)
	if !s.cfg.Favicons.Apply(ctx, &r) {
		return rec
	}
	before := rec.Report.Meta.FaviconURL
	updated, err := s.store.Update(rec.ID, func(rep *model.Report) error {
		if rep.Meta.FaviconURL == before {
			rep.Meta.FaviconURL = r.Meta.FaviconURL
		}
		return nil
	})
	if err != nil {
		cp := *rec
		cp.Report = r
		return &cp
	}
	return updated
}

// renderOptions maps the public links to the renderer's download buttons.
func renderOptions(lang string, l Links) render.Options {
	return render.Options{Lang: lang, PDFURL: l.PDF, HTMLDownloadURL: l.HTMLDownload}
}

// RenderPDF returns the PDF of a record, using the cache when it is fresh.
func (s *Service) RenderPDF(ctx context.Context, rec *store.Record, lang string) ([]byte, string, error) {
	lang = normLang(lang)
	if b, ok := s.store.CachedPDF(rec, lang); ok {
		return b, s.store.PDFPath(rec.ID, lang), nil
	}
	html, err := s.RenderHTML(rec, lang, Links{})
	if err != nil {
		return nil, "", err
	}
	out, err := pdf.FromHTML(ctx, html)
	if err != nil {
		return nil, "", err
	}
	p, err := s.store.SavePDF(rec.ID, lang, out)
	if err != nil {
		return nil, "", err
	}
	return out, p, nil
}

// ---------------------------------------------------------------------------
// Summaries

// KPILine is a headline metric in the textual summary.
type KPILine struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Current  float64  `json:"current"`
	Previous *float64 `json:"previous,omitempty"`
	DeltaPct *float64 `json:"delta_pct,omitempty"`
	Tone     string   `json:"tone,omitempty"`
}

// Summary is the analysis returned to the model after rendering.
type Summary struct {
	KPIs     []KPILine          `json:"kpis"`
	Insights []insights.Insight `json:"insights,omitempty"`
	Progress *float64           `json:"progress_index,omitempty"`
}

// Summarize computes headline KPIs and the top automatic insights.
func Summarize(rep model.Report) Summary {
	r := deepCopy(rep)
	r.Normalize()
	a := insights.Analyze(&r)
	var kpis []KPILine
	add := func(key, label string, m *model.Metric) {
		if m == nil {
			return
		}
		k := KPILine{Key: key, Label: label, Current: round2(m.Current), Previous: m.Previous}
		if d, ok := m.DeltaPct(); ok {
			d = round2(d)
			k.DeltaPct = &d
		}
		if st, ok := a.KPIs[key]; ok {
			k.Tone = string(st.Tone)
		}
		kpis = append(kpis, k)
	}
	if sc := r.SearchConsole; sc != nil {
		add("gsc.clicks", "Google clicks", &sc.Totals.Clicks)
		add("gsc.impressions", "Google impressions", &sc.Totals.Impressions)
		add("gsc.ctr", "Average CTR (%)", sc.Totals.CTR)
		add("gsc.position", "Average position (lower is better)", sc.Totals.Position)
	}
	if an := r.Analytics; an != nil {
		add("ga4.sessions", "Sessions (all channels)", an.Totals.Sessions)
		add("ga4.users", "Active users", an.Totals.Users)
		add("ga4.engagement_rate", "Engagement rate (%)", an.Totals.EngagementRate)
		add("ga4.key_events", "Key events", an.Totals.KeyEvents)
		add("ga4.revenue", "Revenue", an.Totals.Revenue)
		if an.OrganicTotals != nil {
			add("ga4.organic_sessions", "Organic search sessions", an.OrganicTotals.Sessions)
		}
	}
	add("geo.ai_sessions", "Sessions from AI assistants", a.AISessions)
	add("geo.citation_rate", "AI citation rate (%)", a.CitationRate)

	ins := a.Insights
	sort.SliceStable(ins, func(i, j int) bool { return ins[i].Weight > ins[j].Weight })
	if len(ins) > 8 {
		ins = ins[:8]
	}
	return Summary{KPIs: kpis, Insights: ins, Progress: a.Progress}
}

// Text renders the summary as plain text lines.
func (s Summary) Text() string {
	var b strings.Builder
	if len(s.KPIs) > 0 {
		b.WriteString("Key figures:\n")
		for _, k := range s.KPIs {
			fmt.Fprintf(&b, "- %s: %s", k.Label, fmtNum(k.Current))
			if k.Previous != nil {
				fmt.Fprintf(&b, " (previous %s", fmtNum(*k.Previous))
				if k.DeltaPct != nil {
					fmt.Fprintf(&b, ", %+.1f %%", *k.DeltaPct)
				}
				b.WriteString(")")
			}
			b.WriteString("\n")
		}
	}
	if s.Progress != nil {
		fmt.Fprintf(&b, "Progress index: %.0f/100 (50 = stable)\n", *s.Progress)
	}
	if len(s.Insights) > 0 {
		b.WriteString("Automatic insights:\n")
		for _, in := range s.Insights {
			fmt.Fprintf(&b, "- [%s] %s\n", in.Tone, in.Text)
		}
	}
	return b.String()
}

// Completeness lists which parts of the report are filled.
type Completeness struct {
	Filled      []string `json:"filled"`
	Missing     []string `json:"missing"`
	Suggestions []string `json:"suggestions,omitempty"`
}

// CheckCompleteness inspects a report.
func CheckCompleteness(r *model.Report) Completeness {
	var c Completeness
	check := func(ok bool, name string) {
		if ok {
			c.Filled = append(c.Filled, name)
		} else {
			c.Missing = append(c.Missing, name)
		}
	}
	sc := r.SearchConsole
	check(sc != nil && sc.Totals.Impressions.Current > 0, "search_console.totals")
	check(sc != nil && len(sc.Daily) > 0, "search_console.daily")
	check(sc != nil && len(sc.Queries) > 0, "search_console.queries")
	check(sc != nil && len(sc.Pages) > 0, "search_console.pages")
	check(sc != nil && (len(sc.Countries) > 0 || len(sc.Devices) > 0), "search_console.countries/devices")
	check(sc != nil && sc.Indexing != nil, "search_console.indexing")
	check(sc != nil && sc.CoreWebVitals != nil, "search_console.core_web_vitals")
	an := r.Analytics
	check(an != nil && an.Totals.Sessions != nil, "analytics.totals")
	check(an != nil && an.OrganicTotals != nil, "analytics.organic_totals")
	check(an != nil && len(an.Daily) > 0, "analytics.daily")
	check(an != nil && len(an.Channels) > 0, "analytics.channels")
	check(an != nil && len(an.Sources) > 0, "analytics.sources")
	check(an != nil && len(an.LandingPages) > 0, "analytics.landing_pages")
	g := r.GEO
	check(g != nil && len(g.AIReferrals) > 0 || an != nil && len(an.Sources) > 0, "geo.ai_referrals (or analytics.sources)")
	check(g != nil && len(g.Citations) > 0, "geo.citations")
	check(g != nil && len(g.ShareOfVoice) > 0, "geo.share_of_voice")
	check(g != nil && len(g.AIOverviews) > 0, "geo.ai_overviews")
	n := r.Narrative
	check(n != nil && n.ExecutiveSummary != "", "narrative.executive_summary")
	check(n != nil && len(n.Highlights)+len(n.Concerns) > 0, "narrative.highlights/concerns")
	check(n != nil && len(n.Actions) > 0, "narrative.actions")
	check(n != nil && len(n.Recommendations) > 0, "narrative.recommendations")

	if sc != nil && !sc.Totals.Clicks.HasPrevious() {
		c.Suggestions = append(c.Suggestions, "Add previous values to search_console.totals so progress can be shown.")
	}
	if sc != nil && len(sc.Daily) > 0 && len(sc.PreviousDaily) == 0 {
		c.Suggestions = append(c.Suggestions, "search_console.previous_daily (comparison period) adds a dashed overlay on the trend chart.")
	}
	if an != nil && an.Totals.Sessions != nil && !an.Totals.Sessions.HasPrevious() {
		c.Suggestions = append(c.Suggestions, "Add previous values to analytics.totals.")
	}
	if n == nil || n.ExecutiveSummary == "" {
		c.Suggestions = append(c.Suggestions, "Write an executive summary with set_narrative once the data is in (cite figures).")
	}
	if g == nil || len(g.Citations) == 0 {
		c.Suggestions = append(c.Suggestions, "For the GEO section, ask the user for a few manual AI citation checks (prompt, engine, cited yes/no).")
	}
	return c
}

func deepCopy(r model.Report) model.Report {
	var out model.Report
	b, _ := json.Marshal(r)
	_ = json.Unmarshal(b, &out)
	return out
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func fmtNum(v float64) string {
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.2f", v)
}

func nowUTC() time.Time { return time.Now().UTC() }
