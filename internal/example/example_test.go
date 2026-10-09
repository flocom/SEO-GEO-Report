package example

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "regenerate examples/demo-fr.json and examples/demo-en.json")

func goldenPath(lang string) string {
	return filepath.Join("..", "..", "examples", "demo-"+lang+".json")
}

func TestDemoValidates(t *testing.T) {
	for _, lang := range []string{"fr", "en"} {
		r := Demo(lang)
		if r.Lang() != lang {
			t.Errorf("%s: Lang() = %q", lang, r.Lang())
		}
		r.Normalize()
		errs, warns := r.Validate()
		if len(errs) > 0 {
			t.Errorf("%s: validation errors: %v", lang, errs)
		}
		if len(warns) > 0 {
			t.Errorf("%s: validation warnings: %v", lang, warns)
		}
	}
}

func TestDemoContent(t *testing.T) {
	r := Demo("fr")
	sc, ga, geo := r.SearchConsole, r.Analytics, r.GEO
	if sc == nil || ga == nil || geo == nil || r.Narrative == nil {
		t.Fatal("demo must contain search_console, analytics, geo and narrative")
	}
	if len(sc.Daily) != 90 || len(sc.PreviousDaily) != 90 || len(ga.Daily) != 90 || len(ga.PreviousDaily) != 90 {
		t.Errorf("daily lengths: gsc %d/%d ga4 %d/%d", len(sc.Daily), len(sc.PreviousDaily), len(ga.Daily), len(ga.PreviousDaily))
	}
	if sc.Daily[0].Date != curStart || sc.Daily[89].Date != curEnd || sc.PreviousDaily[0].Date != prevStart || sc.PreviousDaily[89].Date != prevEnd {
		t.Errorf("unexpected date bounds: %s..%s / %s..%s", sc.Daily[0].Date, sc.Daily[89].Date, sc.PreviousDaily[0].Date, sc.PreviousDaily[89].Date)
	}
	pct, ok := sc.Totals.Clicks.DeltaPct()
	if !ok || pct < 28 || pct > 42 {
		t.Errorf("clicks growth = %.1f %%, want ~30-40 %%", pct)
	}
	var sum float64
	for _, d := range sc.Daily {
		sum += d.Clicks
	}
	if sum != sc.Totals.Clicks.Current {
		t.Errorf("totals.clicks %v != sum of daily %v", sc.Totals.Clicks.Current, sum)
	}
	if n := len(sc.Queries); n < 18 || n > 25 {
		t.Errorf("queries = %d", n)
	}
	if n := len(sc.Pages); n < 12 {
		t.Errorf("pages = %d", n)
	}
	var chSum float64
	for _, c := range ga.Channels {
		chSum += c.Sessions
	}
	if chSum != ga.Totals.Sessions.Current {
		t.Errorf("channels sum %v != total sessions %v", chSum, ga.Totals.Sessions.Current)
	}
	var srcSum float64
	for _, s := range ga.Sources {
		srcSum += s.Sessions
	}
	if srcSum != ga.Totals.Sessions.Current {
		t.Errorf("sources sum %v != total sessions %v", srcSum, ga.Totals.Sessions.Current)
	}
	for _, want := range []string{"chatgpt.com", "perplexity.ai", "gemini.google.com", "copilot.microsoft.com", "claude.ai"} {
		found := false
		for _, s := range ga.Sources {
			if s.Source == want {
				found = s.PrevSessions != nil && s.Sessions > *s.PrevSessions
			}
		}
		if !found {
			t.Errorf("source %s missing or not growing", want)
		}
	}
	if len(geo.Citations) < 10 || len(geo.ShareOfVoice) != 4 || len(geo.AIOverviews) == 0 || len(geo.AICrawlers) == 0 {
		t.Errorf("geo content too small")
	}
	tableFound := false
	for _, s := range r.Sections {
		if strings.Contains(s.Body, "|---|") {
			tableFound = true
		}
	}
	if !tableFound {
		t.Error("a custom section with a Markdown table is expected")
	}
}

func TestDemoDeterministicAndLanguageIndependentNumbers(t *testing.T) {
	a, b := Demo("fr"), Demo("fr")
	if !reflect.DeepEqual(a, b) {
		t.Fatal("Demo is not deterministic")
	}
	en := Demo("en")
	if !reflect.DeepEqual(a.SearchConsole.Daily, en.SearchConsole.Daily) || !reflect.DeepEqual(a.Analytics.Daily, en.Analytics.Daily) {
		t.Error("numbers must not depend on the language")
	}
	if a.Narrative.ExecutiveSummary == en.Narrative.ExecutiveSummary {
		t.Error("narrative must be translated")
	}
	if Demo("EN-us").Meta.Language != "en" || Demo("").Meta.Language != "fr" {
		t.Error("language normalization")
	}
}

// TestGoldenJSON checks that examples/demo-*.json match Demo(). Regenerate
// with: go test ./internal/example -run TestGoldenJSON -update
func TestGoldenJSON(t *testing.T) {
	for _, lang := range []string{"fr", "en"} {
		got, err := DemoJSON(lang)
		if err != nil {
			t.Fatal(err)
		}
		path := goldenPath(lang)
		if *update {
			if err := os.WriteFile(path, got, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%v (run go test ./internal/example -run TestGoldenJSON -update)", err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s is out of date: run go test ./internal/example -run TestGoldenJSON -update", path)
		}
	}
}
