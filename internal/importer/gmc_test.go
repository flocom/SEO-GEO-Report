package importer

import (
	"strings"
	"testing"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

func parseGMC(t *testing.T, csv string, kind GMCKind, into *model.MerchantCenter) (*model.MerchantCenter, *GMCResult) {
	t.Helper()
	mc, res, err := ParseGMCCSV([]byte(csv), kind, into)
	if err != nil {
		t.Fatalf("ParseGMCCSV: %v\n%s", err, csv)
	}
	return mc, res
}

func TestGMCProductsEN(t *testing.T) {
	csv := "\uFEFF# Merchant Center performance export\n" +
		"Title,Item ID,Clicks,Impressions,CTR,Conversions,Conversion value,Clicks (previous period),Impressions (previous period),Clicks change\n" +
		"Rattan pendant 60 cm,ML-1,\"1,204\",\"52,400\",2.30%,41,\"5,289.00\",455,\"41,200\",+164%\n" +
		"Brass bedside lamp,ML-2,648,47900,1.35%,37,3293.5,330,31800,+96%\n" +
		"Total,,1852,100300,1.85%,78,8582.5,785,73000,\n"
	mc, res := parseGMC(t, csv, "", nil)
	if res.Kind != GMCProducts || res.Rows != 2 || len(mc.Products) != 2 {
		t.Fatalf("result %+v products %+v", res, mc.Products)
	}
	p := mc.Products[0]
	if p.Key != "Rattan pendant 60 cm" || p.OfferID != "ML-1" || p.Clicks != 1204 || p.Impressions != 52400 ||
		p.Conversions != 41 || p.ConversionValue != 5289 || p.PrevClicks == nil || *p.PrevClicks != 455 || *p.PrevImpressions != 41200 {
		t.Errorf("row: %+v", p)
	}
	if mc.Products[1].ConversionValue != 3293.5 {
		t.Errorf("decimal value: %+v", mc.Products[1])
	}
}

func TestGMCProductsFR(t *testing.T) {
	csv := "Titre du produit;Identifiant de l'offre;Clics;Impressions;Taux de clics;Conversions;Valeur de conversion\n" +
		"Suspension rotin Ø 60 cm;ML-SUS-ROT-60;1 204;52 400;2,30 %;41;5 289,50 €\n" +
		"Lampe de chevet;ML-LAP;648;47 900;1,35 %;37;3 293\n"
	mc, res := parseGMC(t, csv, "", &model.MerchantCenter{Programs: []model.GMCRow{{Key: "kept"}}})
	if res.Kind != GMCProducts || len(mc.Products) != 2 || len(mc.Programs) != 1 {
		t.Fatalf("result %+v %+v", res, mc)
	}
	if p := mc.Products[0]; p.Clicks != 1204 || p.Impressions != 52400 || p.ConversionValue != 5289.5 || p.OfferID != "ML-SUS-ROT-60" || p.PrevClicks != nil {
		t.Errorf("row: %+v", p)
	}
}

func TestGMCDates(t *testing.T) {
	csv := "Jour;Clics;Impressions;Conversions;Clics (période précédente);Impressions (période précédente)\n" +
		"02/07/2026;12;1 000;2;10;900\n" +
		"01/07/2026;8;800;1;6;700\n" +
		"13/07/2026;5;500;0;5;400\n"
	mc, res := parseGMC(t, csv, "", &model.MerchantCenter{Totals: model.GMCTotals{Clicks: model.M(1, 1), Impressions: model.M(9, 9)}})
	if res.Kind != GMCDates || res.Rows != 3 || res.PreviousDays != 3 {
		t.Fatalf("result %+v", res)
	}
	if mc.Daily[0].Date != "2026-07-01" || mc.Daily[1].Date != "2026-07-02" || mc.Daily[2].Date != "2026-07-13" {
		t.Errorf("daily: %+v", mc.Daily)
	}
	if mc.PreviousDaily[0].Date != "2026-06-18" || mc.PreviousDaily[0].Clicks != 6 {
		t.Errorf("previous daily (shifted by the 13-day period): %+v", mc.PreviousDaily)
	}
	if mc.Totals.Clicks.Current != 25 || *mc.Totals.Clicks.Previous != 21 || mc.Totals.Impressions.Current != 2300 {
		t.Errorf("totals recomputed from the daily table: %+v", mc.Totals)
	}
	if mc.Totals.Conversions == nil || mc.Totals.Conversions.Current != 3 || mc.Totals.Conversions.Previous != nil {
		t.Errorf("conversions: %+v", mc.Totals.Conversions)
	}
}

func TestGMCDimensions(t *testing.T) {
	cases := []struct {
		csv  string
		kind GMCKind
		get  func(*model.MerchantCenter) []model.GMCRow
	}{
		{"Marketing method,Clicks,Impressions\nFree listings,4492,416000\nShopping ads,3073,195000\n", GMCPrograms, func(m *model.MerchantCenter) []model.GMCRow { return m.Programs }},
		{"Méthode marketing;Clics;Impressions\nFiches gratuites;4 492;416 000\nAnnonces Shopping;3 073;195 000\n", GMCPrograms, func(m *model.MerchantCenter) []model.GMCRow { return m.Programs }},
		{"Catégorie de produits,Clics,Impressions\nSuspensions,2679,190000\nLampadaires,1182,90000\n", GMCCategories, func(m *model.MerchantCenter) []model.GMCRow { return m.Categories }},
		{"Brand\tClicks\tImpressions\nMaison Lumen\t900\t30000\nOther\t100\t3000\n", GMCBrands, func(m *model.MerchantCenter) []model.GMCRow { return m.Brands }},
		{"Pays de vente,Clics,Impressions\nFrance,6777,546000\nBelgique,709,65000\n", GMCCountries, func(m *model.MerchantCenter) []model.GMCRow { return m.Countries }},
	}
	for _, c := range cases {
		mc, res := parseGMC(t, c.csv, "", nil)
		rows := c.get(mc)
		if res.Kind != c.kind || len(rows) != 2 || rows[0].Clicks <= rows[1].Clicks || rows[0].Impressions == 0 {
			t.Errorf("%s: %+v %+v", c.kind, res, rows)
		}
	}
	// Totals are computed from the programs at normalization.
	mc, _ := parseGMC(t, cases[0].csv, "", nil)
	mc.Normalize()
	if mc.Totals.Clicks.Current != 7565 {
		t.Errorf("totals from programs: %+v", mc.Totals)
	}
}

func TestGMCStatus(t *testing.T) {
	// One row per product (products export).
	perProduct := "Title,Item ID,Status\nA,1,Approved\nB,2,Approved\nC,3,Limited\nD,4,Disapproved\nE,5,Pending\nF,6,Not approved\n"
	mc, res := parseGMC(t, perProduct, "", nil)
	ps := mc.ProductStatus
	if res.Kind != GMCStatus || ps == nil || ps.Approved.Current != 2 || ps.Limited.Current != 1 || ps.NotApproved.Current != 2 || ps.Pending.Current != 1 {
		t.Fatalf("per product: %+v %+v", res, ps)
	}
	// French summary with a comparison column.
	summary := "Statut;Produits;Produits (période précédente)\nApprouvé;418;371\nLimité;31;46\nRefusé;14;22\nEn attente;5;8\nTotal;468;447\n"
	mc, _ = parseGMC(t, summary, "", nil)
	ps = mc.ProductStatus
	if ps.Approved.Current != 418 || *ps.Approved.Previous != 371 || ps.NotApproved.Current != 14 || ps.Total == nil || ps.Total.Current != 468 {
		t.Errorf("summary: %+v", ps)
	}
	if r := ps.ApprovedRate(); r == nil || r.Current != 89.3 {
		t.Errorf("rate: %+v", r)
	}
	// One row, one column per status.
	wide := "Approved,Limited,Not approved,Pending\n120,7,3,1\n"
	mc, res = parseGMC(t, wide, "", nil)
	if res.Kind != GMCStatus || mc.ProductStatus.Approved.Current != 120 || mc.ProductStatus.NotApproved.Current != 3 {
		t.Errorf("wide: %+v %+v", res, mc.ProductStatus)
	}
}

func TestGMCIssues(t *testing.T) {
	en := "Issue,Severity,Affected items,Destination\nMissing GTIN,Warning,24,Free listings\nMissing GTIN,Warning,20,Shopping ads\nImage too small,Error,6,All\n"
	mc, res := parseGMC(t, en, "", nil)
	if res.Kind != GMCIssues || len(mc.Issues) != 2 {
		t.Fatalf("%+v %+v", res, mc.Issues)
	}
	if is := mc.Issues[0]; is.Issue != "Missing GTIN" || is.Products != 24 || is.Severity != model.SeverityWarning || is.Affects != "Free listings, Shopping ads" {
		t.Errorf("merged issue: %+v", is)
	}
	if is := mc.Issues[1]; is.Severity != model.SeverityCritical {
		t.Errorf("severity: %+v", is)
	}
	fr := "Problème;Gravité;Articles concernés;Articles concernés (période précédente)\nPrix non concordant;Critique;8;13\nPoids d'expédition manquant;Suggestion;11;11\n"
	mc, _ = parseGMC(t, fr, "", nil)
	if is := mc.Issues[0]; is.Products != 8 || is.PrevProducts == nil || *is.PrevProducts != 13 || is.Severity != model.SeverityCritical || mc.Issues[1].Severity != model.SeverityInfo {
		t.Errorf("fr issues: %+v", mc.Issues)
	}
	// Per-product diagnostics: rows are counted.
	perItem := "Item ID,Title,Issue\n1,A,Missing GTIN\n2,B,Missing GTIN\n3,C,Image too small\n"
	mc, _ = parseGMC(t, perItem, "", nil)
	if mc.Issues[0].Products != 2 || mc.Issues[1].Products != 1 {
		t.Errorf("counted issues: %+v", mc.Issues)
	}
}

func TestGMCPrices(t *testing.T) {
	csv := "Titre;ID de l'article;Votre prix;Prix de référence;Écart de prix\nLampadaire arc;ML-2;299,00 €;259,00 €;+15 %\nApplique;ML-3;89;91,20;-2 %\n"
	mc, res := parseGMC(t, csv, "", nil)
	if res.Kind != GMCPrices || len(mc.PriceBenchmarks) != 2 {
		t.Fatalf("%+v %+v", res, mc.PriceBenchmarks)
	}
	if p := mc.PriceBenchmarks[1]; p.Product != "Applique" || p.OfferID != "ML-3" || p.Price != 89 || p.BenchmarkPrice != 91.2 {
		t.Errorf("price row: %+v", p)
	}
	en := "Product title,Price,Benchmark price,Benchmark price difference\nArc floor lamp,299.00,259.00,15.4%\n"
	mc, _ = parseGMC(t, en, GMCPrices, nil)
	if mc.PriceBenchmarks[0].BenchmarkPrice != 259 {
		t.Errorf("en price: %+v", mc.PriceBenchmarks)
	}
}

func TestGMCKindsAndErrors(t *testing.T) {
	// An explicit kind that does not match the header is an error.
	if _, _, err := ParseGMCCSV([]byte("Title,Clicks,Impressions\nA,1,2\n"), GMCPrices, nil); err == nil || !strings.Contains(err.Error(), "prices") {
		t.Errorf("kind mismatch: %v", err)
	}
	if _, _, err := ParseGMCCSV([]byte("foo,bar\n1,2\n"), "", nil); err == nil {
		t.Error("unknown table: error expected")
	}
	// Loose aliases.
	for in, want := range map[GMCKind]GMCKind{"Products": GMCProducts, "daily": GMCDates, "marketing method": GMCPrograms, "diagnostics": GMCIssues,
		"price competitiveness": GMCPrices, "Statuts": GMCStatus, "": ""} {
		if got := normalizeGMCKind(in); got != want {
			t.Errorf("normalizeGMCKind(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]GMCKind{"Performance - Produits.csv": GMCProducts, "price_competitiveness.csv": GMCPrices, "Diagnostics.csv": GMCIssues,
		"Programmes.csv": GMCPrograms, "Pays.csv": GMCCountries, "random.csv": ""} {
		if got := GuessGMCKind(in); got != want {
			t.Errorf("GuessGMCKind(%q) = %q, want %q", in, got, want)
		}
	}
	// A header after a title line is found; an explicit kind is honoured.
	mc, res := parseGMC(t, "Performance report\nTitle,Clicks,Impressions\nA,3,30\n", "products", nil)
	if res.Kind != GMCProducts || mc.Products[0].Clicks != 3 {
		t.Errorf("preamble: %+v %+v", res, mc.Products)
	}
}
