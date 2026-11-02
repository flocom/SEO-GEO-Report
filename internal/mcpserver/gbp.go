package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/flocom/SEO-GEO-Report/internal/importer"
	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// Google Business Profile tools: set_business_profile and
// import_business_profile_csv.

type gbpInput struct {
	ReportID        string                `json:"report_id" jsonschema:"Report id"`
	BusinessProfile model.BusinessProfile `json:"business_profile" jsonschema:"Google Business Profile data. In merge mode only the keys you send are replaced (e.g. send only reviews)"`
	Mode            string                `json:"mode,omitempty" jsonschema:"merge (default): replace only the top-level keys you send; replace: replace the whole business_profile section; append: lists are appended to existing ones"`
}

type importGBPInput struct {
	ReportID string `json:"report_id" jsonschema:"Report id"`
	CSV      string `json:"csv" jsonschema:"Full raw text of ONE CSV file exported from Google Business Profile (Performance / bulk insights export per location, daily metrics, search keywords, or reviews)"`
	Kind     string `json:"kind,omitempty" jsonschema:"performance, daily, keywords or reviews. Guessed from the header or file name when omitted"`
	FileName string `json:"file_name,omitempty" jsonschema:"Original file name (helps guessing the kind)"`
}

// gbpToolDesc describes set_business_profile (also used by the guide test).
const gbpToolDesc = "Store Google Business Profile data supplied by the user (ex Google My Business: the business card in Google Search and Google Maps; this tool does not fetch anything). " +
	"Use it whenever the user gives GBP figures: never put them in a free-text section. Maps to the profile's Performance page. " +
	"totals {views, views_search_mobile, views_search_desktop, views_maps_mobile, views_maps_desktop, interactions, calls, website_clicks, direction_requests, messages, bookings, food_orders, menu_clicks} as {current, previous} metrics (views are summed from the four splits and interactions from the actions when omitted); " +
	"locations [{name, store_code, address, city, totals, rating, review_count}] when there are several establishments (root totals are then summed); daily / previous_daily [{date, views, interactions, calls, website_clicks, direction_requests}]; " +
	"search_keywords [{keyword, impressions (number of searches), below_threshold (true for '<15'), prev_impressions}]; " +
	"reviews {average_rating (1-5), total_reviews, new_reviews, response_rate (percent) as metrics, rating_distribution {\"1\"..\"5\": counts}, recent [{rating, text as written, date, author, responded}]}; " +
	"posts, photos (metrics), profile_completeness (percent metric), checklist [{item, done}] where item is preferably a known key (hours, special_hours, categories, description, photos, logo, cover_photo, products, services, website, phone, address, attributes, posts, booking, menu, service_area, messaging, qa, review_replies) so it is shown in the report language. " +
	"Default mode merge replaces only the top-level keys you send. For raw CSV exports use import_business_profile_csv instead."

func (s *Service) registerGBPTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_business_profile",
		Title:       "Set Google Business Profile data",
		Description: gbpToolDesc,
		InputSchema: relaxed[gbpInput]("business_profile"),
		Annotations: idempotent,
	}, s.toolSetGBP)

	mcp.AddTool(srv, &mcp.Tool{
		Name:  "import_business_profile_csv",
		Title: "Import a Google Business Profile CSV",
		Description: "Parse the raw text of ONE CSV file exported from Google Business Profile and merge it into report.business_profile: the Performance / bulk insights export (one row per location: views by Google Search / Maps and mobile / desktop, calls, website clicks, direction requests, messages, bookings; several rows fill locations and summed totals), a daily metrics table (date + metrics, comparison columns fill previous_daily), the search keywords table (values such as '<15' are kept as thresholds) or a reviews export (rating, comment, date, reply: recent reviews, star distribution and reply rate). French or English headers. " +
			"Call once per file. Use this when the user attaches or pastes the CSV files; otherwise use set_business_profile.",
		Annotations: idempotent,
	}, s.toolImportGBP)
}

func (s *Service) toolSetGBP(_ context.Context, req *mcp.CallToolRequest, in gbpInput) (*mcp.CallToolResult, any, error) {
	mode, err := checkMode(in.Mode, "merge", "replace", "append")
	if err != nil {
		return nil, nil, err
	}
	raw := rawArg(req, "business_profile")
	rec, err := s.update(in.ReportID, func(r *model.Report) error {
		var existing any
		if mode != "replace" && r.BusinessProfile != nil {
			existing = r.BusinessProfile
		}
		var bp model.BusinessProfile
		if err := mergeObject(existing, raw, mode == "append", &bp); err != nil {
			return fmt.Errorf("business_profile: %w", err)
		}
		r.BusinessProfile = &bp
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return s.savedText(rec, "Google Business Profile data"), nil, nil
}

func (s *Service) toolImportGBP(_ context.Context, _ *mcp.CallToolRequest, in importGBPInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.CSV) == "" {
		return nil, nil, errors.New("csv is empty: pass the full text content of the CSV file")
	}
	kind := importer.GBPKind(strings.TrimSpace(in.Kind))
	if kind == "" && in.FileName != "" {
		kind = importer.GuessGBPKind(filepath.Base(in.FileName))
	}
	var res *importer.GBPResult
	var noTotals bool
	rec, err := s.update(in.ReportID, func(r *model.Report) error {
		bp, got, err := importer.ParseGBPCSV([]byte(in.CSV), kind, r.BusinessProfile)
		if err != nil && kind != "" && in.Kind == "" {
			// The file name was only a hint: retry with the header alone.
			bp, got, err = importer.ParseGBPCSV([]byte(in.CSV), "", r.BusinessProfile)
		}
		if err != nil {
			return fmt.Errorf("cannot parse the Business Profile CSV: %w", err)
		}
		r.BusinessProfile, res = bp, got
		noTotals = !bp.HasPerformance()
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	what := fmt.Sprintf("Business Profile %s table (%d rows", res.Kind, res.Rows)
	if res.Locations > 0 {
		what += fmt.Sprintf(", %d locations", res.Locations)
	}
	if res.PreviousDays > 0 {
		what += fmt.Sprintf(", %d comparison days", res.PreviousDays)
	}
	what += ")"
	out := s.savedText(rec, what)
	if noTotals {
		out.Content[0].(*mcp.TextContent).Text += "\nBusiness Profile views and actions are still empty: import the Performance export (or daily metrics), or send business_profile.totals (views, calls, website_clicks, direction_requests with previous values) with set_business_profile."
	}
	sc := out.StructuredContent.(map[string]any)
	sc["kind"], sc["rows"] = res.Kind, res.Rows
	return out, nil, nil
}

// gbpSummary adds the Business Profile figures to the render summary.
func gbpSummary(bp *model.BusinessProfile, add func(key, label string, m *model.Metric)) {
	if bp == nil {
		return
	}
	t := bp.Totals
	add("gbp.views", "Google Business Profile views", t.Views)
	add("gbp.interactions", "Google Business Profile interactions (calls, directions, website clicks...)", t.Interactions)
	add("gbp.calls", "Calls from the Google profile", t.Calls)
	add("gbp.directions", "Direction requests from the Google profile", t.DirectionRequests)
	add("gbp.website_clicks", "Website clicks from the Google profile", t.WebsiteClicks)
	if rv := bp.Reviews; rv != nil {
		add("gbp.rating", "Google average rating (out of 5)", rv.AverageRating)
		add("gbp.reviews", "Google reviews", rv.TotalReviews)
		add("gbp.response_rate", "Google reviews replied to (%)", rv.ResponseRate)
	}
}

// gbpCompleteness checks the Business Profile parts of a report.
func gbpCompleteness(r *model.Report, check func(bool, string), c *Completeness) {
	bp := r.BusinessProfile
	check(bp != nil && bp.HasPerformance(), "business_profile.totals (or daily / locations)")
	check(bp != nil && bp.Reviews != nil, "business_profile.reviews")
	if bp == nil {
		c.Suggestions = append(c.Suggestions, "If the business has a Google Business Profile (local card in Google Search and Maps), its Performance page (views, calls, direction requests, website clicks, searches) and reviews add a local visibility section: set_business_profile or import_business_profile_csv. Never put GBP numbers in a free-text section.")
		return
	}
	m := bp.Totals.Interactions
	if bp.Totals.Views != nil {
		m = bp.Totals.Views
	}
	if m != nil && !m.HasPrevious() && len(bp.PreviousDaily) == 0 {
		c.Suggestions = append(c.Suggestions, "Add previous values to business_profile.totals (or previous_daily) so local visibility progress can be shown.")
	}
}
