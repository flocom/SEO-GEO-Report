package mcpserver

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// Google Business Profile data from set_business_profile (merge / append)
// and CSV imports, then validation, summary and rendering.
func TestBusinessProfileTools(t *testing.T) {
	svc := newService(t)
	cs := connect(t, svc)
	meta := map[string]any{"site_name": "Showroom", "language": "fr", "period": map[string]string{"start": "2026-07-01", "end": "2026-07-03"}}
	id := structured(t, call(t, cs, "create_report", map[string]any{"meta": meta}))["report_id"].(string)

	call(t, cs, "set_business_profile", map[string]any{"report_id": id, "business_profile": map[string]any{
		"totals": map[string]any{
			"views": map[string]any{"current": 1200, "previous": 1000},
			"calls": map[string]any{"current": 40, "previous": 30},
		},
		"reviews": map[string]any{
			"average_rating": map[string]any{"current": 4.6, "previous": 4.5},
			"recent":         []map[string]any{{"rating": 5, "text": "Top"}},
		},
	}})
	// Merge keeps totals; append adds recent reviews.
	call(t, cs, "set_business_profile", map[string]any{"report_id": id, "mode": "append", "business_profile": map[string]any{
		"checklist": []map[string]any{{"item": "hours", "done": true}},
	}})
	kw := call(t, cs, "import_business_profile_csv", map[string]any{"report_id": id, "file_name": "Mots clés.csv",
		"csv": "Requête de recherche;Valeur\nluminaire lyon;410\nlampe design;<15\n"})
	if structured(t, kw)["kind"] != "keywords" {
		t.Errorf("import keywords: %s", text(kw))
	}
	// A misleading file name is only a hint: the header decides.
	rv := call(t, cs, "import_business_profile_csv", map[string]any{"report_id": id, "file_name": "keywords.csv",
		"csv": "Rating,Comment,Date,Reply\n5,Great,2026-07-02,Thanks\n2,Meh,2026-07-01,\n4,Nice,2026-07-01,\n"})
	if structured(t, rv)["kind"] != "reviews" {
		t.Errorf("import reviews: %s", text(rv))
	}
	rec, _ := svc.store.Get(id)
	bp := rec.Report.BusinessProfile
	if bp == nil || bp.Totals.Views == nil || bp.Totals.Views.Current != 1200 || len(bp.SearchKeywords) != 2 || len(bp.Checklist) != 1 ||
		bp.Reviews == nil || len(bp.Reviews.Recent) != 3 || bp.Reviews.AverageRating.Current != 4.6 {
		t.Fatalf("business profile data: %+v", bp)
	}

	v := call(t, cs, "validate_report", map[string]any{"report_id": id})
	if !strings.Contains(text(v), "business_profile.totals") {
		t.Errorf("completeness should list business_profile: %s", text(v))
	}
	r := call(t, cs, "render_report", map[string]any{"report_id": id})
	for _, want := range []string{"Google Business Profile views: 1200 (previous 1000", "Google average rating (out of 5): 4.6"} {
		if !strings.Contains(text(r), want) {
			t.Errorf("render summary misses %q:\n%s", want, text(r))
		}
	}

	// Reviews only: the tool says what is missing.
	id2 := structured(t, call(t, cs, "create_report", map[string]any{"meta": meta}))["report_id"].(string)
	res := call(t, cs, "import_business_profile_csv", map[string]any{"report_id": id2, "csv": "Note;Avis\n5;Super\n"})
	if !strings.Contains(text(res), "views and actions are still empty") {
		t.Errorf("missing totals hint: %s", text(res))
	}
	bad, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: "import_business_profile_csv", Arguments: map[string]any{"report_id": id2, "csv": "foo,bar\n1,2\n"}})
	if err != nil || !bad.IsError {
		t.Errorf("unknown CSV must be a tool error: %v %v", err, bad)
	}
	// Performance export with several locations.
	perf := call(t, cs, "import_business_profile_csv", map[string]any{"report_id": id2,
		"csv": "Store code,Business name,Google Search - Mobile,Google Maps - Mobile,Calls\nA,Lyon,100,50,3\nB,Paris,80,20,2\n"})
	if !strings.Contains(text(perf), "2 locations") {
		t.Errorf("locations: %s", text(perf))
	}

	// One-shot generation with Business Profile data only.
	call(t, cs, "generate_report", map[string]any{"report": map[string]any{
		"meta":             meta,
		"business_profile": map[string]any{"reviews": map[string]any{"rating_distribution": map[string]any{"5": 10, "4": 2}}},
	}})
}

func TestBusinessProfileCompletenessAndGuide(t *testing.T) {
	r := &model.Report{Meta: model.Meta{SiteName: "S"}}
	c := CheckCompleteness(r)
	found := false
	for _, s := range c.Suggestions {
		found = found || strings.Contains(s, "set_business_profile")
	}
	if !found {
		t.Errorf("suggestions: %v", c.Suggestions)
	}
	g := GuideMarkdown(false, "en")
	for _, want := range []string{"import_business_profile_csv", "set_business_profile", "never to a free-text section", "Searches breakdown",
		"Put these numbers in `bing`", "Put these numbers in `merchant_center`", "business_profile"} {
		if !strings.Contains(g, want) {
			t.Errorf("guide misses %q", want)
		}
	}
	if !strings.Contains(Instructions, "set_business_profile") || !strings.Contains(Instructions, "never to a free-text section") {
		t.Error("instructions must mention the Business Profile tool and the native-section rule")
	}
	if !strings.Contains(gbpToolDesc, "never put them in a free-text section") {
		t.Error("set_business_profile description must forbid free-text sections")
	}
}
