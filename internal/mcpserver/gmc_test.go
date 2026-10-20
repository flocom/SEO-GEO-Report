package mcpserver

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// Merchant Center data from set_merchant_center (merge / append) and CSV
// imports, then validation, summary and rendering.
func TestMerchantCenterTools(t *testing.T) {
	svc := newService(t)
	cs := connect(t, svc)
	meta := map[string]any{"site_name": "Shop", "language": "fr", "period": map[string]string{"start": "2026-07-01", "end": "2026-07-03"}}
	id := structured(t, call(t, cs, "create_report", map[string]any{"meta": meta}))["report_id"].(string)

	call(t, cs, "set_merchant_center", map[string]any{"report_id": id, "merchant_center": map[string]any{
		"totals": map[string]any{
			"clicks":      map[string]any{"current": 130, "previous": 100},
			"impressions": map[string]any{"current": 9000, "previous": 8000},
		},
		"issues": []map[string]any{{"issue": "Missing GTIN", "products": 12, "severity": "warning"}},
	}})
	// Merge keeps totals; append adds issues.
	call(t, cs, "set_merchant_center", map[string]any{"report_id": id, "mode": "append", "merchant_center": map[string]any{
		"issues": []map[string]any{{"issue": "Image too small", "products": 3, "severity": "critical"}},
	}})
	prod := call(t, cs, "import_merchant_center_csv", map[string]any{"report_id": id, "file_name": "Produits.csv",
		"csv": "Titre;ID de l'article;Clics;Impressions;Clics (période précédente)\nLampe rotin;ML-1;80;3 000;40\nApplique;ML-2;50;2 000;45\n"})
	if structured(t, prod)["kind"] != "products" {
		t.Errorf("import products: %s", text(prod))
	}
	// A misleading file name is only a hint: the header decides.
	st := call(t, cs, "import_merchant_center_csv", map[string]any{"report_id": id, "file_name": "prices.csv",
		"csv": "Statut,Produits\nApprouvé,90\nLimité,6\nRefusé,4\n"})
	if structured(t, st)["kind"] != "status" {
		t.Errorf("import status: %s", text(st))
	}
	rec, _ := svc.store.Get(id)
	mc := rec.Report.MerchantCenter
	if mc == nil || mc.Totals.Clicks.Current != 130 || len(mc.Issues) != 2 || len(mc.Products) != 2 || mc.ProductStatus == nil || mc.ProductStatus.Approved.Current != 90 {
		t.Fatalf("merchant center data: %+v", mc)
	}

	v := call(t, cs, "validate_report", map[string]any{"report_id": id})
	if !strings.Contains(text(v), "merchant_center.totals") {
		t.Errorf("completeness should list merchant_center: %s", text(v))
	}
	r := call(t, cs, "render_report", map[string]any{"report_id": id})
	for _, want := range []string{"Google Shopping clicks (Merchant Center): 130 (previous 100", "Approved products in Merchant Center (%): 90"} {
		if !strings.Contains(text(r), want) {
			t.Errorf("render summary misses %q:\n%s", want, text(r))
		}
	}

	// Products without totals: the tool says what is missing.
	id2 := structured(t, call(t, cs, "create_report", map[string]any{"meta": meta}))["report_id"].(string)
	res := call(t, cs, "import_merchant_center_csv", map[string]any{"report_id": id2, "csv": "Title,Clicks,Impressions\nLamp,3,30\n"})
	if !strings.Contains(text(res), "totals are still empty") {
		t.Errorf("missing totals hint: %s", text(res))
	}
	bad, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: "import_merchant_center_csv", Arguments: map[string]any{"report_id": id2, "csv": "foo,bar\n1,2\n"}})
	if err != nil || !bad.IsError {
		t.Errorf("unknown CSV must be a tool error: %v %v", err, bad)
	}

	// One-shot generation with Merchant Center data only.
	call(t, cs, "generate_report", map[string]any{"report": map[string]any{
		"meta":            meta,
		"merchant_center": map[string]any{"price_benchmarks": []map[string]any{{"product": "Lamp", "price": 10, "benchmark_price": 9}}},
	}})
}

func TestMerchantCenterCompleteness(t *testing.T) {
	r := &model.Report{Meta: model.Meta{SiteName: "S"}}
	c := CheckCompleteness(r)
	found := false
	for _, s := range c.Suggestions {
		found = found || strings.Contains(s, "set_merchant_center")
	}
	if !found {
		t.Errorf("suggestions: %v", c.Suggestions)
	}
	if !strings.Contains(GuideMarkdown(false, "en"), "import_merchant_center_csv") || !strings.Contains(Instructions, "set_merchant_center") {
		t.Error("guide and instructions must mention the Merchant Center tools")
	}
}
