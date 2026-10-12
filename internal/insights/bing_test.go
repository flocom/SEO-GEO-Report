package insights

import (
	"math"
	"strings"
	"testing"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

func bingReport(lang string) *model.Report {
	return &model.Report{
		Meta: model.Meta{Language: lang},
		SearchConsole: &model.SearchConsole{Totals: model.GSCTotals{
			Clicks: model.M(9000, 8000), Impressions: model.M(300000, 280000),
		}},
		Bing: &model.Bing{
			Totals: model.GSCTotals{
				Clicks: model.M(1000, 500), Impressions: model.M(40000, 30000),
				Position: &model.Metric{Current: 6.2, Previous: model.F(7.5)},
			},
			Queries: []model.GSCRow{{Key: "lampe rotin", Clicks: 80, Impressions: 900, PrevClicks: model.F(20)}},
			AIPerformance: &model.BingAIPerformance{
				Citations:        model.M(600, 300),
				GroundingQueries: []model.BingGroundingQuery{{Query: "choisir une lampe", Citations: 120}, {Query: "autre", Citations: 5}},
			},
		},
	}
}

func TestBingShare(t *testing.T) {
	s := BingShare(bingReport("fr"))
	if s == nil || s.Current != 10 || s.Previous == nil || math.Abs(*s.Previous-5.88) > 0.01 {
		t.Fatalf("share = %+v", s)
	}
	r := bingReport("fr")
	r.SearchConsole = nil
	if BingShare(r) != nil {
		t.Error("no Google data: share must be nil")
	}
}

func TestBingAnalysis(t *testing.T) {
	r := bingReport("fr")
	r.Normalize()
	a := Analyze(r)
	for _, k := range []string{KeyBingClicks, KeyBingImpressions, KeyBingCTR, KeyBingPosition, KeyBingAICitations} {
		if st, ok := a.KPIs[k]; !ok || st.Tone != Positive {
			t.Errorf("%s: %+v", k, st)
		}
	}
	texts := joinTexts(a)
	for _, want := range []string{"clics depuis Bing progressent de 100 %", "Bing pèse davantage", "Copilot", "« lampe rotin »", "« choisir une lampe »"} {
		if !strings.Contains(texts, want) {
			t.Errorf("missing %q in:\n%s", want, texts)
		}
	}
	for _, in := range a.Insights {
		if strings.Contains(in.Text, "Bing") && in.Section != "bing" {
			t.Errorf("bing insight in section %q: %s", in.Section, in.Text)
		}
	}
	checkInsights(t, a, "fr")

	// Bing clicks and AI citations count in the progress index.
	without := bingReport("fr")
	without.Bing = nil
	if p, q := Analyze(r).Progress, Analyze(without).Progress; p == nil || q == nil || *p <= *q {
		t.Errorf("progress with Bing growth %v should exceed %v", p, q)
	}
}

func TestBingInactiveDoesNotMoveProgress(t *testing.T) {
	r := bingReport("en")
	r.Bing = nil
	base := Analyze(r).Progress
	r.Bing = &model.Bing{
		Totals:        model.GSCTotals{Clicks: model.M(0, 0), Impressions: model.M(0, 0)},
		AIPerformance: &model.BingAIPerformance{Citations: model.M(0, 0)},
	}
	if p := Analyze(r).Progress; p == nil || base == nil || *p != *base {
		t.Errorf("progress with an inactive Bing = %v, want %v", p, base)
	}
}
