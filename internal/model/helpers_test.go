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

func TestMerchantCenterNormalizeAndValidate(t *testing.T) {
	r := &Report{
		Meta: Meta{SiteName: "S", Period: Period{Start: "2026-07-01", End: "2026-07-02"}},
		MerchantCenter: &MerchantCenter{
			Daily:           []GMCDailyPoint{{Date: "2026-07-01", Clicks: 10, Impressions: 1000}, {Date: "2026-07-02", Clicks: 30, Impressions: 1000}},
			PreviousDaily:   []GMCDailyPoint{{Date: "2026-06-29", Clicks: 10, Impressions: 500}, {Date: "2026-06-30", Clicks: 10, Impressions: 500}},
			Products:        []GMCRow{{Key: "Lamp", Clicks: 5, Impressions: 200}, {Key: "Bad", Clicks: 9, Impressions: 3}},
			ProductStatus:   &GMCProductStatus{Approved: M(90, 80), Limited: &Metric{Current: 6, Previous: F(10)}, NotApproved: &Metric{Current: 4, Previous: F(10)}},
			Issues:          []GMCIssue{{Issue: "a", Products: 2, Severity: "Erreur"}, {Issue: "b", Products: 1, Severity: "Avertissement"}, {Issue: "c", Products: 1, Severity: "Suggestion"}, {Issue: "d", Products: 1, Severity: "Other"}},
			PriceBenchmarks: []GMCPriceBenchmark{{Product: "Lamp", Price: 10, BenchmarkPrice: 0}},
		},
	}
	r.Normalize()
	mc := r.MerchantCenter
	if mc.Totals.Clicks.Current != 40 || *mc.Totals.Clicks.Previous != 20 || mc.Totals.Impressions.Current != 2000 {
		t.Errorf("totals from daily: %+v", mc.Totals)
	}
	if mc.Totals.CTR == nil || mc.Totals.CTR.Current != 2 || *mc.Totals.CTR.Previous != 2 || mc.Products[0].CTR != 2.5 {
		t.Errorf("CTR: %+v %+v", mc.Totals.CTR, mc.Products[0])
	}
	var sev []string
	for _, is := range mc.Issues {
		sev = append(sev, is.Severity)
	}
	if strings.Join(sev, ",") != "critical,warning,info,other" {
		t.Errorf("severities: %v", sev)
	}
	if rate := mc.ProductStatus.ApprovedRate(); rate == nil || rate.Current != 90 || *rate.Previous != 80 {
		t.Errorf("approved rate: %+v", rate)
	}
	errs, warns := r.Validate()
	joined := strings.Join(warns, "\n")
	if len(errs) > 0 || !strings.Contains(joined, `merchant_center.products "Bad": clicks are greater than impressions`) ||
		!strings.Contains(joined, "price_benchmarks") || strings.Contains(joined, "merchant_center.totals has no previous") {
		t.Errorf("validate: %v %v", errs, warns)
	}

	// Totals from programs, conversions included; a total overrides the sum.
	mc2 := &MerchantCenter{Programs: []GMCRow{{Key: "Free", Clicks: 3, Impressions: 30, Conversions: 1, PrevClicks: F(2)}, {Key: "Ads", Clicks: 7, Impressions: 70, Conversions: 2}}}
	mc2.Normalize()
	if mc2.Totals.Clicks.Current != 10 || mc2.Totals.Clicks.Previous != nil || mc2.Totals.Conversions == nil || mc2.Totals.Conversions.Current != 3 {
		t.Errorf("totals from programs: %+v", mc2.Totals)
	}
	ps := &GMCProductStatus{Approved: Metric{Current: 50}, Total: &Metric{Current: 200}}
	if rate := ps.ApprovedRate(); rate == nil || rate.Current != 25 || rate.Previous != nil {
		t.Errorf("rate with total: %+v", rate)
	}
	var nilPS *GMCProductStatus
	if nilPS.ApprovedRate() != nil || (&GMCProductStatus{}).ApprovedRate() != nil {
		t.Error("no products: nil rate")
	}
	// No previous values: a warning.
	r.MerchantCenter = &MerchantCenter{Totals: GMCTotals{Clicks: Metric{Current: 5}, Impressions: Metric{Current: 50}}}
	if _, warns := r.Validate(); len(warns) != 1 || !strings.Contains(warns[0], "merchant_center.totals has no previous values") {
		t.Errorf("warnings: %v", warns)
	}
	if !isSectionID("merchant_center") {
		t.Error("merchant_center must be a section id")
	}
}
