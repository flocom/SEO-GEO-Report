package render

import (
	"slices"
	"strings"
	"testing"

	"github.com/flocom/SEO-GEO-Report/internal/example"
	"github.com/flocom/SEO-GEO-Report/internal/model"
)

func TestMerchantCenterSection(t *testing.T) {
	for _, lang := range []string{"fr", "en"} {
		s := mustRender(t, example.Demo(lang), Options{})
		ids := sectionIDs(s)
		i := slices.Index(ids, "sec-merchant_center")
		if i < 0 || ids[i-1] != "sec-landing_pages" || ids[i+1] != "sec-geo" {
			t.Fatalf("%s: merchant_center section misplaced in %v", lang, ids)
		}
		sec := s[strings.Index(s, `id="sec-merchant_center"`):strings.Index(s, `id="sec-geo"`)]
		want := map[string][]string{
			"fr": {"Google Shopping (Merchant Center)", "Clics Google Shopping par jour", "Clics par programme", "Fiches gratuites", "État des produits du flux",
				"Refusés", "Top 10 des produits", "Détail des produits", "ML-SUS-ROT-60", "Problèmes du flux produits", "Critique", "Compétitivité prix",
				"plus cher que le marché", "Produits approuvés", "Valeur des ventes Shopping", "refusés ou limités"},
			"en": {"Google Shopping (Merchant Center)", "Daily Google Shopping clicks", "Clicks by program", "Free listings", "Product feed status",
				"Disapproved", "Top 10 products", "Product details", "Product feed issues", "Critical", "Price competitiveness", "above the market",
				"Approved products", "Shopping conversion value", "disapproved or limited"},
		}[lang]
		for _, w := range want {
			if !strings.Contains(sec, w) {
				t.Errorf("%s: merchant_center section misses %q", lang, w)
			}
		}
		// Issues are sorted by severity: the critical ones first.
		if a, b := strings.Index(sec, map[string]string{"fr": "Prix non concordant", "en": "Price mismatch"}[lang]), strings.Index(sec, map[string]string{"fr": "Code GTIN manquant", "en": "Missing GTIN"}[lang]); a < 0 || b < 0 || a > b {
			t.Errorf("%s: issues not sorted by severity", lang)
		}
		if !strings.Contains(s, "Google Merchant Center") {
			t.Errorf("%s: cover sources miss Google Merchant Center", lang)
		}
		// Part 1: a seventh key indicator, on one row on desktop.
		kpis := s[strings.Index(s, `id="sec-kpis"`):]
		kpis = kpis[:strings.Index(kpis, "</section>")]
		if !strings.Contains(kpis, `class="kpi-grid bal n-7 big" style="--bd:7;`) || !strings.Contains(kpis, map[string]string{"fr": "Clics Google Shopping", "en": "Google Shopping clicks"}[lang]) {
			t.Errorf("%s: key indicators should hold the Google Shopping clicks card on one row", lang)
		}
		if !strings.Contains(s, map[string]string{"fr": "Fiches produits gratuites", "en": "Free listings"}[lang]) {
			t.Errorf("%s: glossary misses the Merchant Center terms", lang)
		}
	}
}

func TestMerchantCenterAbsentOrHidden(t *testing.T) {
	r := example.Demo("en")
	r.MerchantCenter = nil
	s := mustRender(t, r, Options{})
	if strings.Contains(s, `id="sec-merchant_center"`) || strings.Contains(s, "Google Shopping clicks") || strings.Contains(s, "Offer ID") {
		t.Error("no Merchant Center data: no section, no KPI card, no glossary terms")
	}
	if !strings.Contains(s, `class="kpi-grid bal n-6 big" style="--bd:6;`) {
		t.Error("without Merchant Center the key indicators keep six cards on one row")
	}

	r = example.Demo("en")
	r.Options = &model.Options{HideSections: []string{"merchant_center"}}
	if s := mustRender(t, r, Options{}); strings.Contains(s, `id="sec-merchant_center"`) {
		t.Error("merchant_center section should be hidden")
	}

	// Note and custom section placed after the section.
	r = example.Demo("fr")
	r.Narrative.SectionNotes["merchant_center"] = "Note **GMC**"
	r.Sections = append(r.Sections, model.TextSection{ID: "after-gmc", Title: "Après GMC", Body: "x", After: "merchant_center"})
	s = mustRender(t, r, Options{})
	ids := sectionIDs(s)
	if i := slices.Index(ids, "sec-merchant_center"); i < 0 || ids[i+1] != "sec-c-after-gmc" || !strings.Contains(s, "Note <strong>GMC</strong>") {
		t.Errorf("note / custom section: %v", ids)
	}
}

func TestMerchantCenterOnlyAndPartial(t *testing.T) {
	full := example.Demo("fr")

	// Merchant Center alone.
	s := mustRender(t, &model.Report{Meta: full.Meta, MerchantCenter: full.MerchantCenter}, Options{})
	if !strings.Contains(s, `id="sec-merchant_center"`) || strings.Contains(s, `id="sec-search_console"`) || !strings.Contains(s, `id="sec-kpis"`) {
		t.Errorf("GMC-only report: sections %v", sectionIDs(s))
	}

	cases := map[string]*model.MerchantCenter{
		"status only": {ProductStatus: &model.GMCProductStatus{Approved: model.Metric{Current: 10}, NotApproved: &model.Metric{Current: 2, Previous: model.F(0)}}},
		"issues only": {Issues: []model.GMCIssue{{Issue: "Missing GTIN", Products: 3}, {Issue: "Odd", Products: 1, Severity: "whatever"}}},
		"prices only": {PriceBenchmarks: []model.GMCPriceBenchmark{{Product: "A", Price: 10, BenchmarkPrice: 9}, {Product: "B", Price: 0, BenchmarkPrice: 9}}},
		"daily only":  {Daily: []model.GMCDailyPoint{{Date: "2026-07-01", Clicks: 3, Impressions: 30}, {Date: "2026-07-02", Clicks: 5, Impressions: 40}}},
		"products":    {Products: []model.GMCRow{{Key: "Lamp", Clicks: 3, Impressions: 30}}},
		"programs":    {Programs: []model.GMCRow{{Key: "Free listings", Clicks: 3, Impressions: 30, PrevClicks: model.F(0)}, {Key: "Ads", Clicks: 1, Impressions: 10}}},
	}
	for name, mc := range cases {
		s := mustRender(t, &model.Report{Meta: full.Meta, MerchantCenter: mc}, Options{})
		if !strings.Contains(s, `id="sec-merchant_center"`) {
			t.Errorf("%s: section missing", name)
		}
	}
	// An empty object renders no section.
	if s := mustRender(t, &model.Report{Meta: full.Meta, MerchantCenter: &model.MerchantCenter{}}, Options{}); strings.Contains(s, `id="sec-merchant_center"`) {
		t.Error("empty merchant_center: no section")
	}
}

func TestMerchantCenterEscaping(t *testing.T) {
	full := example.Demo("en")
	evil := `<script>alert("x")</script>`
	mc := &model.MerchantCenter{
		Totals:          model.GMCTotals{Clicks: model.M(10, 5), Impressions: model.M(100, 90)},
		Programs:        []model.GMCRow{{Key: evil, Clicks: 6, Impressions: 50}, {Key: "Ads", Clicks: 4, Impressions: 50}},
		Products:        []model.GMCRow{{Key: evil, OfferID: evil, Clicks: 6, Impressions: 50, PrevClicks: model.F(1)}, {Key: "B", Clicks: 4, Impressions: 50}},
		Issues:          []model.GMCIssue{{Issue: evil, Products: 2, Severity: evil, Affects: evil}},
		PriceBenchmarks: []model.GMCPriceBenchmark{{Product: evil, OfferID: evil, Price: 12, BenchmarkPrice: 10}},
		ProductStatus:   &model.GMCProductStatus{Approved: model.M(8, 7), Limited: &model.Metric{Current: 2}},
	}
	s := mustRender(t, &model.Report{Meta: full.Meta, MerchantCenter: mc}, Options{})
	if strings.Contains(s, evil) {
		t.Error("unescaped product data in output")
	}
}
