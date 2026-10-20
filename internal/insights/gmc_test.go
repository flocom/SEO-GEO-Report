package insights

import (
	"strings"
	"testing"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

func gmcReport(lang string) *model.Report {
	return &model.Report{
		Meta: model.Meta{Language: lang},
		SearchConsole: &model.SearchConsole{Totals: model.GSCTotals{
			Clicks: model.M(9000, 8000), Impressions: model.M(300000, 280000),
		}},
		MerchantCenter: &model.MerchantCenter{
			Totals: model.GMCTotals{
				Clicks: model.M(1300, 1000), Impressions: model.M(120000, 100000),
				Conversions: &model.Metric{Current: 60, Previous: model.F(40)},
			},
			Programs: []model.GMCRow{
				{Key: "Free listings", Clicks: 800, Impressions: 80000, PrevClicks: model.F(500)},
				{Key: "Shopping ads", Clicks: 500, Impressions: 40000, PrevClicks: model.F(500)},
			},
			Products: []model.GMCRow{
				{Key: "Lampe rotin", Clicks: 120, Impressions: 9000, PrevClicks: model.F(40)},
				{Key: "Applique", Clicks: 60, Impressions: 7000, PrevClicks: model.F(55)},
			},
			ProductStatus: &model.GMCProductStatus{
				Approved:    model.M(180, 170),
				Limited:     &model.Metric{Current: 12, Previous: model.F(15)},
				NotApproved: &model.Metric{Current: 8, Previous: model.F(15)},
			},
			Issues: []model.GMCIssue{
				{Issue: "Missing GTIN", Products: 12, Severity: "Warning"},
				{Issue: "Price mismatch -10%", Products: 5, Severity: "Erreur"},
				{Issue: "Missing weight", Products: 30, Severity: "suggestion"},
			},
			PriceBenchmarks: []model.GMCPriceBenchmark{
				{Product: "Lampe rotin", Price: 129, BenchmarkPrice: 110},
				{Product: "Applique", Price: 80, BenchmarkPrice: 82},
				{Product: "Invalid", Price: 0, BenchmarkPrice: 10},
			},
		},
	}
}

func TestGMCAnalysis(t *testing.T) {
	r := gmcReport("en")
	r.Normalize()
	a := Analyze(r)
	for _, k := range []string{KeyGMCClicks, KeyGMCImpressions, KeyGMCConversions, KeyGMCApprovedRate} {
		if st, ok := a.KPIs[k]; !ok || st.Tone != Positive {
			t.Errorf("%s: %+v", k, st)
		}
	}
	if _, ok := a.KPIs[KeyGMCCTR]; !ok {
		t.Error("gmc.ctr missing (computed by Normalize)")
	}

	// Every Merchant Center observation is available to the section.
	w := &writer{lang: "en"}
	gmcInsights(w, r.MerchantCenter)
	var texts []string
	for _, in := range w.out {
		if in.Section != "merchant_center" {
			t.Errorf("insight in section %q", in.Section)
		}
		texts = append(texts, in.Text)
	}
	all := strings.Join(texts, "\n")
	for _, want := range []string{
		"Clicks on your products in Google Shopping grew by 30%",
		"10% of your products are disapproved or limited in Merchant Center (20 of 200)",
		"Main issue to fix: “Price mismatch -10%” (5 products)", // most severe first, "%" kept as is
		"the product “Lampe rotin” gains the most clicks (+80)",
		"Products priced above the market average: 1 of 2 tracked (up to +17% for “Lampe rotin”)",
		"Free product listings grew by 60%",
		"Sales attributed to your product listings are growing",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q in:\n%s", want, all)
		}
	}
	if strings.Contains(all, "%!") {
		t.Errorf("format error in:\n%s", all)
	}

	// The selected insights keep up to three Merchant Center observations.
	n := 0
	for _, in := range a.Insights {
		if in.Section == "merchant_center" {
			n++
		}
	}
	if n == 0 || n > maxOptional+5 {
		t.Errorf("%d merchant_center insights selected", n)
	}
	checkInsights(t, a, "en")

	// French texts.
	fr := gmcReport("fr")
	fr.Normalize()
	wf := &writer{lang: "fr"}
	gmcInsights(wf, fr.MerchantCenter)
	frAll := ""
	for _, in := range wf.out {
		frAll += in.Text + "\n"
	}
	for _, want := range []string{"refusés ou limités", "Problème principal à corriger\u00a0: «\u00a0Price mismatch -10%\u00a0» (5 produits)", "fiches produits gratuites"} {
		if !strings.Contains(frAll, want) {
			t.Errorf("fr: missing %q in:\n%s", want, frAll)
		}
	}

	// Merchant Center clicks count in the progress index, modestly.
	without := gmcReport("en")
	without.MerchantCenter = nil
	p, q := Analyze(r).Progress, Analyze(without).Progress
	if p == nil || q == nil || *p <= *q || *p-*q > 5 {
		t.Errorf("progress with Merchant Center growth %v vs %v", p, q)
	}
}

func TestGMCPartialData(t *testing.T) {
	cases := map[string]*model.MerchantCenter{
		"empty":          {},
		"status only":    {ProductStatus: &model.GMCProductStatus{Approved: model.Metric{Current: 10}}},
		"zero status":    {ProductStatus: &model.GMCProductStatus{}},
		"issues only":    {Issues: []model.GMCIssue{{Issue: "Image too small", Products: 3, Severity: "critical"}, {Issue: "", Products: 2}}},
		"prices only":    {PriceBenchmarks: []model.GMCPriceBenchmark{{Product: "A", Price: 10, BenchmarkPrice: 11}, {Product: "B", Price: 5, BenchmarkPrice: 5}}},
		"daily only":     {Daily: []model.GMCDailyPoint{{Date: "2026-07-01", Clicks: 3, Impressions: 300}}},
		"one program":    {Programs: []model.GMCRow{{Key: "Free listings", Clicks: 4, Impressions: 100}}},
		"no prev status": {ProductStatus: &model.GMCProductStatus{Approved: model.Metric{Current: 10}, NotApproved: &model.Metric{Current: 5}}},
	}
	for name, mc := range cases {
		for _, lang := range []string{"fr", "en"} {
			r := &model.Report{Meta: model.Meta{Language: lang}, MerchantCenter: mc}
			r.Normalize()
			a := Analyze(r)
			checkInsights(t, a, lang)
			for _, in := range a.Insights {
				if strings.Contains(in.Text, "%!") {
					t.Errorf("%s/%s: %s", name, lang, in.Text)
				}
			}
		}
	}
	r := &model.Report{MerchantCenter: cases["daily only"]}
	r.Normalize()
	if c := r.MerchantCenter.Totals.Clicks.Current; c != 3 {
		t.Errorf("totals from daily: %v", c)
	}
	if a := Analyze(r); a.KPIs[KeyGMCClicks].HasDelta {
		t.Error("no comparison: no delta")
	}
	if a := Analyze(&model.Report{MerchantCenter: cases["prices only"]}); len(a.Insights) != 1 || a.Insights[0].Tone != Positive {
		t.Errorf("competitive prices: %+v", a.Insights)
	}
}

func TestGMCInactiveDoesNotMoveProgress(t *testing.T) {
	r := gmcReport("en")
	r.MerchantCenter = nil
	base := Analyze(r).Progress
	r.MerchantCenter = &model.MerchantCenter{Totals: model.GMCTotals{Clicks: model.M(0, 0), Impressions: model.M(0, 0)}}
	if p := Analyze(r).Progress; p == nil || base == nil || *p != *base {
		t.Errorf("progress with an inactive Merchant Center = %v, want %v", p, base)
	}
}

func TestMainIssueAndSort(t *testing.T) {
	issues := []model.GMCIssue{
		{Issue: "info", Products: 50, Severity: "info"},
		{Issue: "warn big", Products: 40, Severity: "avertissement"},
		{Issue: "crit small", Products: 2, Severity: "Refusé"},
		{Issue: "crit big", Products: 9, Severity: "critical"},
		{Issue: "unknown", Products: 41},
	}
	if m, ok := MainGMCIssue(issues); !ok || m.Issue != "crit big" {
		t.Errorf("main issue = %+v", m)
	}
	var got []string
	for _, is := range SortGMCIssues(issues) {
		got = append(got, is.Issue)
	}
	if strings.Join(got, ",") != "crit big,crit small,unknown,warn big,info" {
		t.Errorf("sorted = %v", got)
	}
	if issues[0].Issue != "info" {
		t.Error("SortGMCIssues must not modify its input")
	}
	if _, ok := MainGMCIssue(nil); ok {
		t.Error("no issue expected")
	}
	if !IsFreeListingsProgram("Fiches gratuites") || !IsFreeListingsProgram("Free listings") || IsFreeListingsProgram("Shopping ads") {
		t.Error("IsFreeListingsProgram")
	}
	if g, ok := PriceGap(model.GMCPriceBenchmark{Price: 110, BenchmarkPrice: 100}); !ok || g < 9.99 || g > 10.01 {
		t.Errorf("gap = %v", g)
	}
}
