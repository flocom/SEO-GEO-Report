package model

import (
	"strings"
	"testing"
)

func TestBingNormalizeAndValidate(t *testing.T) {
	r := &Report{
		Meta: Meta{SiteName: "S", Period: Period{Start: "2026-07-01", End: "2026-07-02"}},
		Bing: &Bing{
			Totals:  GSCTotals{Clicks: M(10, 8), Impressions: M(400, 320)},
			Queries: []GSCRow{{Key: "a", Clicks: 4, Impressions: 100}},
			AIPerformance: &BingAIPerformance{
				Daily:            []BingAIDailyPoint{{Date: "2026-07-01", Citations: 3, CitedPages: 2}, {Date: "2026-07-02", Citations: 5, CitedPages: 3}},
				PreviousDaily:    []BingAIDailyPoint{{Date: "2026-06-29", Citations: 2, CitedPages: 1}, {Date: "2026-06-30", Citations: 2, CitedPages: 1}},
				GroundingQueries: []BingGroundingQuery{{Query: "q1", Citations: 3, CitationShare: 0.2}, {Query: "q2", Citations: 1, CitationShare: 0.05}},
			},
		},
	}
	r.Normalize()
	bg := r.Bing
	if bg.Totals.CTR == nil || bg.Totals.CTR.Current != 2.5 || *bg.Totals.CTR.Previous != 2.5 || bg.Queries[0].CTR != 4 {
		t.Errorf("CTR not computed: %+v %+v", bg.Totals.CTR, bg.Queries[0])
	}
	ai := bg.AIPerformance
	if ai.Citations.Current != 8 || *ai.Citations.Previous != 4 || ai.CitedPages.Current != 2.5 || *ai.CitedPages.Previous != 1 {
		t.Errorf("AI totals: %+v %+v", ai.Citations, ai.CitedPages)
	}
	errs, warns := r.Validate()
	if len(errs) > 0 || len(warns) != 1 || !strings.Contains(warns[0], "citation_share looks like a ratio") {
		t.Errorf("validate: %v %v", errs, warns)
	}
	// A single small share is a real percentage.
	ai.GroundingQueries = ai.GroundingQueries[:1]
	if _, warns := r.Validate(); len(warns) != 0 {
		t.Errorf("warnings: %v", warns)
	}
}
