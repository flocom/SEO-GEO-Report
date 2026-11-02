package insights

import (
	"strings"
	"testing"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// smallSiteReport mimics a site that just launched new pages: impressions
// ×3.8, clicks up, many new ranking queries, worse position and CTR.
func dilutionReport(lang string, impPrev float64) *model.Report {
	r := &model.Report{
		Meta: model.Meta{SiteName: "Test", Language: lang, Period: model.Period{Start: "2026-10-01", End: "2026-10-31"}},
		SearchConsole: &model.SearchConsole{
			Totals: model.GSCTotals{
				Clicks:      model.M(102, 63),
				Impressions: model.M(3017, impPrev),
				CTR:         &model.Metric{Current: 3.4, Previous: model.F(7.9)},
				Position:    &model.Metric{Current: 40.2, Previous: model.F(29.6)},
			},
			PositionDistribution: &model.PositionDistribution{
				Top3: model.M(1, 4), Top10: model.M(5, 2), Top20: model.M(2, 0), Beyond20: model.M(14, 4),
			},
		},
	}
	r.Normalize()
	return r
}

func TestDilutionExpected(t *testing.T) {
	for _, lang := range []string{"fr", "en"} {
		a := Analyze(dilutionReport(lang, 796))
		for _, k := range []string{KeyGSCCTR, KeyGSCPosition} {
			st := a.KPIs[k]
			if !st.Expected || st.Tone != Neutral || st.Note == "" {
				t.Errorf("%s %s: want expected neutral with a note, got %+v", lang, k, st)
			}
		}
		if st := a.KPIs[KeyGSCClicks]; st.Expected || st.Tone != Positive {
			t.Errorf("%s clicks must stay positive: %+v", lang, st)
		}
		texts := joinTexts(a)
		if strings.Contains(texts, "retravaillez") || strings.Contains(texts, "rewrite the titles") {
			t.Errorf("%s: the CTR alert must not fire on dilution:\n%s", lang, texts)
		}
		want := map[string]string{"fr": "souvent plus bas au début", "en": "often lower down at first"}[lang]
		if !strings.Contains(texts, want) || !strings.Contains(a.KPIs[KeyGSCPosition].Note, want) {
			t.Errorf("%s: missing dilution explanation %q", lang, want)
		}
	}
	if n := DilutionNote("fr", "position"); !strings.HasPrefix(n, "Normal :") {
		t.Errorf("FR note typography: %q", n)
	}
	if DilutionNote("en", "other") != "" {
		t.Error("unknown kind must give an empty note")
	}
}

func TestDilutionNotApplied(t *testing.T) {
	// Impressions +20 % only: a worse position is a real decline.
	a := Analyze(dilutionReport("fr", 2500))
	if st := a.KPIs[KeyGSCPosition]; st.Expected || st.Tone != Negative {
		t.Errorf("position should stay negative: %+v", st)
	}
	// Strong impressions growth but fewer ranking queries: not dilution.
	r := dilutionReport("fr", 796)
	r.SearchConsole.PositionDistribution = &model.PositionDistribution{
		Top3: model.M(1, 10), Top10: model.M(1, 10), Top20: model.M(1, 10), Beyond20: model.M(1, 10),
	}
	if a := Analyze(r); a.KPIs[KeyGSCCTR].Expected {
		t.Error("fewer ranking queries: CTR must not be expected")
	}
	// Clicks falling: not dilution either.
	r = dilutionReport("en", 796)
	r.SearchConsole.Totals.Clicks = model.M(40, 63)
	if a := Analyze(r); a.KPIs[KeyGSCPosition].Expected {
		t.Error("falling clicks: position must not be expected")
	}
}

func TestDilutionProgress(t *testing.T) {
	// The expected position decline does not drag the progress index down.
	a := Analyze(dilutionReport("fr", 796))
	r := dilutionReport("fr", 2900) // +4 % impressions: position counts
	b := Analyze(r)
	if a.Progress == nil || b.Progress == nil || *a.Progress < 70 {
		t.Fatalf("progress = %v / %v", a.Progress, b.Progress)
	}
}

func TestChangeKind(t *testing.T) {
	cases := []struct {
		cur, prev float64
		want      string
	}{
		{6, 1, "abs"}, {3017, 796, "ratio"}, {102, 63, "pct"}, {44, 11, "ratio"}, {47, 1, "abs"},
		{4, 0, "new"}, {0, 0, ""}, {0, 4, "abs"}, {0, 40, "pct"}, {12, 15, "abs"}, {149, 50, "pct"}, {150, 50, "ratio"},
	}
	for _, c := range cases {
		if got := ChangeKind(c.cur, c.prev); got != c.want {
			t.Errorf("ChangeKind(%v, %v) = %q, want %q", c.cur, c.prev, got, c.want)
		}
	}
	if FormatRatio(3.77, "fr") != "×3,8" || FormatRatio(4.01, "en") != "×4" || FormatRatio(47.2, "fr") != "×47" {
		t.Error("FormatRatio")
	}
	// Insight sentences follow the same rule: no "+500 %" for 1 → 6 visits.
	r := dilutionReport("fr", 796)
	r.GEO = &model.GEO{AIReferrals: []model.AIReferralRow{{Platform: "ChatGPT", Sessions: 6, PrevSessions: model.F(1)}}}
	texts := joinTexts(Analyze(r))
	if strings.Contains(texts, "500") || !strings.Contains(texts, "progressent de 5 (6 sessions)") || !strings.Contains(texts, "(×3,8)") {
		t.Errorf("insights should use readable changes:\n%s", texts)
	}
}
