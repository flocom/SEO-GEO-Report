package model

import (
	"strings"
	"testing"
)

func TestBusinessProfileNormalize(t *testing.T) {
	// Views from the splits, interactions from the actions.
	bp := &BusinessProfile{Totals: GBPTotals{
		ViewsSearchMobile: &Metric{Current: 100, Previous: F(80)}, ViewsMapsMobile: &Metric{Current: 50, Previous: F(40)},
		ViewsMapsDesktop: &Metric{Current: 10},
		Calls:            &Metric{Current: 5, Previous: F(4)}, WebsiteClicks: &Metric{Current: 7, Previous: F(6)},
	}}
	bp.Normalize()
	if v := bp.Totals.Views; v == nil || v.Current != 160 || v.Previous != nil {
		t.Errorf("views: %+v", v)
	}
	if v := bp.Totals.Interactions; v == nil || v.Current != 12 || *v.Previous != 10 {
		t.Errorf("interactions: %+v", v)
	}
	bp.Normalize() // idempotent
	if bp.Totals.Views.Current != 160 || bp.Totals.Interactions.Current != 12 {
		t.Error("Normalize must be idempotent")
	}

	// Root totals from the locations.
	bp = &BusinessProfile{Locations: []GBPLocation{
		{Name: "Lyon", Totals: GBPTotals{Views: &Metric{Current: 100, Previous: F(90)}, Calls: &Metric{Current: 3, Previous: F(1)}}},
		{Name: "Paris", Totals: GBPTotals{Views: &Metric{Current: 50, Previous: F(60)}, Calls: &Metric{Current: 2}}},
	}}
	bp.Normalize()
	if v := bp.Totals.Views; v == nil || v.Current != 150 || *v.Previous != 150 {
		t.Errorf("views from locations: %+v", v)
	}
	if v := bp.Totals.Calls; v == nil || v.Current != 5 || v.Previous != nil {
		t.Errorf("calls from locations: %+v", v)
	}
	if bp.Locations[0].Totals.Interactions == nil || bp.Locations[0].Totals.Interactions.Current != 3 {
		t.Errorf("location interactions: %+v", bp.Locations[0].Totals)
	}

	// Totals from the daily series, reviews from the distribution,
	// completeness from the checklist.
	bp = &BusinessProfile{
		Daily:         []GBPDailyPoint{{Date: "2026-07-01", Views: 10, Calls: 1}, {Date: "2026-07-02", Views: 20, WebsiteClicks: 2}},
		PreviousDaily: []GBPDailyPoint{{Date: "2026-06-29", Views: 5}, {Date: "2026-06-30", Views: 5}},
		Reviews:       &GBPReviews{RatingDistribution: &GBPRatingDistribution{Five: 3, Four: 1}},
		Checklist:     []GBPChecklistItem{{"Hours", true}, {"Photos", true}, {"Description", false}, {"Products", true}},
	}
	bp.Normalize()
	if v := bp.Totals.Views; v == nil || v.Current != 30 || *v.Previous != 10 {
		t.Errorf("views from daily: %+v", v)
	}
	if v := bp.Totals.Interactions; v == nil || v.Current != 3 {
		t.Errorf("interactions from daily actions: %+v", v)
	}
	if rv := bp.Reviews; rv.AverageRating.Current != 4.75 || rv.TotalReviews.Current != 4 {
		t.Errorf("reviews: %+v %+v", rv.AverageRating, rv.TotalReviews)
	}
	if bp.ProfileCompleteness == nil || bp.ProfileCompleteness.Current != 75 {
		t.Errorf("completeness: %+v", bp.ProfileCompleteness)
	}
	if !bp.HasPerformance() || bp.IsEmpty() {
		t.Error("performance expected")
	}
	if !(&BusinessProfile{}).IsEmpty() || (&BusinessProfile{Reviews: &GBPReviews{}}).HasPerformance() {
		t.Error("empty profile")
	}
	var nilBP *BusinessProfile
	if nilBP.HasPerformance() || !nilBP.IsEmpty() {
		t.Error("nil profile")
	}
}

func TestBusinessProfileValidate(t *testing.T) {
	r := &Report{
		Meta: Meta{SiteName: "S", Period: Period{Start: "2026-07-01", End: "2026-07-02"}},
		BusinessProfile: &BusinessProfile{
			Totals: GBPTotals{Views: &Metric{Current: 100}, Interactions: &Metric{Current: 5}, Calls: &Metric{Current: 20}},
			Reviews: &GBPReviews{AverageRating: &Metric{Current: 6}, ResponseRate: &Metric{Current: 0.8},
				TotalReviews: &Metric{Current: 2}, RatingDistribution: &GBPRatingDistribution{Five: 10},
				Recent: []GBPReview{{Rating: 0, Text: "?"}}},
			Locations:           []GBPLocation{{Name: "A", Rating: F(9)}},
			ProfileCompleteness: &Metric{Current: 140},
		},
	}
	r.Normalize()
	errs, warns := r.Validate()
	joined := strings.Join(warns, "\n")
	for _, want := range []string{"interactions (5) are lower than the sum", "has no previous values", "average_rating must be between 1 and 5",
		"response_rate looks like a ratio", "adds up to more than total_reviews", "recent: rating 0", `locations "A": rating`, "profile_completeness"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing warning %q in:\n%s", want, joined)
		}
	}
	if len(errs) > 0 {
		t.Errorf("errors: %v", errs)
	}
	if !isSectionID("business_profile") {
		t.Error("business_profile must be a section id")
	}
	if i, j := sectionIndex("merchant_center"), sectionIndex("business_profile"); i < 0 || j != i+1 || SectionIDs[j+1] != "geo" {
		t.Errorf("business_profile must follow merchant_center: %v", SectionIDs)
	}
	// A profile alone is enough data.
	r = &Report{Meta: r.Meta, BusinessProfile: &BusinessProfile{Totals: GBPTotals{Views: &Metric{Current: 10, Previous: F(8)}}}}
	r.Normalize()
	if _, warns := r.Validate(); len(warns) != 0 {
		t.Errorf("warnings: %v", warns)
	}
}

func sectionIndex(id string) int {
	for i, s := range SectionIDs {
		if s == id {
			return i
		}
	}
	return -1
}
