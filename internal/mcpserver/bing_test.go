package mcpserver

import "testing"

// AI Performance data alone (no search totals, no citation total) is valid:
// the total is computed from the daily series.
func TestBingAIOnly(t *testing.T) {
	svc := newService(t)
	cs := connect(t, svc)
	meta := map[string]any{"site_name": "AI Only", "language": "en", "period": map[string]string{"start": "2026-07-01", "end": "2026-07-02"}}

	res := call(t, cs, "create_report", map[string]any{"meta": meta})
	id := structured(t, res)["report_id"].(string)
	call(t, cs, "set_bing", map[string]any{"report_id": id, "bing": map[string]any{"ai_performance": map[string]any{
		"daily": []map[string]any{{"date": "2026-07-01", "citations": 3}, {"date": "2026-07-02", "citations": 5}},
	}}})
	call(t, cs, "render_report", map[string]any{"report_id": id})

	call(t, cs, "generate_report", map[string]any{"report": map[string]any{
		"meta": meta,
		"bing": map[string]any{"ai_performance": map[string]any{"pages": []map[string]any{{"url": "https://x.example/a", "citations": 4}}}},
	}})
}
