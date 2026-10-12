package importer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

func TestParseBingKeywords(t *testing.T) {
	csv := "Keyword,Clicks,Impressions,CTR,Avg. Position,Clicks change\n" +
		"suspension rotin,120,4500,2.67%,3.1,+12%\n" +
		"\"luminaire salon\",\"1,050\",\"32,000\",3.28%,5.4,-3%\n"
	tb, err := ParseBingCSV([]byte(csv), "")
	if err != nil {
		t.Fatal(err)
	}
	if tb.Kind != BingQueries || len(tb.Rows) != 2 {
		t.Fatalf("kind %q, %d rows", tb.Kind, len(tb.Rows))
	}
	r := tb.Rows[1]
	if r.Key != "luminaire salon" || r.Clicks != 1050 || r.Impressions != 32000 || r.Position != 5.4 || r.CTR != 3.28 {
		t.Errorf("row = %+v", r)
	}
	if r.PrevClicks != nil {
		t.Errorf("a change column must not be read as the previous value: %v", *r.PrevClicks)
	}
}

func TestParseBingPagesClickPosition(t *testing.T) {
	// Older exports carry two positions: the impression position is kept.
	csv := "Page;Impressions;Clics;Position moy. des clics;Position moy. des impressions\n" +
		"https://www.example.fr/a;1 200;48;2,1;6,4\n"
	tb, err := ParseBingCSV([]byte(csv), "pages")
	if err != nil {
		t.Fatal(err)
	}
	if tb.Kind != BingPages || len(tb.Rows) != 1 {
		t.Fatalf("kind %q, %d rows", tb.Kind, len(tb.Rows))
	}
	if r := tb.Rows[0]; r.Clicks != 48 || r.Impressions != 1200 || r.Position != 6.4 || r.PrevPosition != nil {
		t.Errorf("row = %+v", r)
	}
}

func TestParseBingDatesComparison(t *testing.T) {
	csv := "Date,Clicks,Impressions,Previous period Clicks,Previous period Impressions\n" +
		"7/2/2026,30,1000,20,900\n" +
		"7/1/2026,25,800,18,850\n"
	tb, err := ParseBingCSV([]byte(csv), "")
	if err != nil {
		t.Fatal(err)
	}
	if tb.Kind != BingDates || len(tb.Daily) != 2 || len(tb.PreviousDaily) != 2 {
		t.Fatalf("kind %q, %d days, %d previous", tb.Kind, len(tb.Daily), len(tb.PreviousDaily))
	}
	if tb.Daily[0].Date != "2026-07-01" || tb.PreviousDaily[0].Date != "2026-06-29" {
		t.Errorf("dates: %s / %s", tb.Daily[0].Date, tb.PreviousDaily[0].Date)
	}
	bg := ApplyBingTable(nil, tb)
	tt := bg.Totals
	if tt.Clicks.Current != 55 || *tt.Clicks.Previous != 38 || tt.Impressions.Current != 1800 || tt.CTR == nil {
		t.Errorf("totals = %+v", tt)
	}
}

func TestParseBingAIGroundingQueries(t *testing.T) {
	csv := "Grounding query,Intent,Topic,Citations,Citation share,Avg. cited pages\n" +
		"best rattan pendant light,Commercial,Home decor,42,18.5%,1.4\n" +
		"how to choose a pendant light,Informational,Home decor,\"1,204\",7.2%,2\n" +
		",,,3,,\n"
	tb, err := ParseBingCSV([]byte(csv), "")
	if err != nil {
		t.Fatal(err)
	}
	if tb.Kind != BingAIQueries || len(tb.GroundingQueries) != 2 {
		t.Fatalf("kind %q, %d queries", tb.Kind, len(tb.GroundingQueries))
	}
	q := tb.GroundingQueries[1]
	if q.Query != "how to choose a pendant light" || q.Citations != 1204 || q.CitationShare != 7.2 || q.CitedPages != 2 || q.Intent != "Informational" || q.Topic != "Home decor" {
		t.Errorf("query = %+v", q)
	}
	// The kind given for a Search Performance table is mapped to its AI counterpart.
	if tb, err := ParseBingCSV([]byte(csv), "queries"); err != nil || tb.Kind != BingAIQueries {
		t.Errorf("kind queries with citations: %v %v", tb, err)
	}
}

func TestParseBingAIPagesAndDaily(t *testing.T) {
	pages := "Page,Citations,Previous period Citations\n" +
		"https://www.example.com/a,120,80\n" +
		"https://www.example.com/b,30,40\n"
	tb, err := ParseBingCSV([]byte(pages), "")
	if err != nil {
		t.Fatal(err)
	}
	if tb.Kind != BingAIPages || len(tb.CitedPages) != 2 || *tb.CitedPages[1].PrevCitations != 40 {
		t.Fatalf("pages table = %+v", tb)
	}
	bg := ApplyBingTable(nil, tb)
	ai := bg.AIPerformance
	if ai == nil || ai.Citations.Current != 150 || *ai.Citations.Previous != 120 {
		t.Fatalf("citations from pages = %+v", ai)
	}

	daily := "Date;Citations;Pages citées\n" +
		"01/07/2026;10;3\n" +
		"02/07/2026;14;5\n"
	tb, err = ParseBingCSV([]byte(daily), "")
	if err != nil {
		t.Fatal(err)
	}
	if tb.Kind != BingAIDates || len(tb.AIDaily) != 2 || tb.AIDaily[1].Date != "2026-07-02" || tb.AIDaily[1].CitedPages != 5 {
		t.Fatalf("daily table = %+v", tb)
	}
	bg = ApplyBingTable(bg, tb)
	ai = bg.AIPerformance
	if ai.Citations.Current != 24 || ai.Citations.Previous != nil || ai.CitedPages == nil || ai.CitedPages.Current != 4 || len(ai.Pages) != 2 {
		t.Errorf("after daily: citations %+v, cited pages %+v, %d pages", ai.Citations, ai.CitedPages, len(ai.Pages))
	}
}

func TestParseBingErrors(t *testing.T) {
	if _, err := ParseBingCSV([]byte("Country,Clicks,Impressions\nFrance,10,100\n"), "countries"); err == nil {
		t.Error("countries table: want an error")
	}
	if _, err := ParseBingCSV([]byte("Keyword,Clicks\nx,1\n"), "ai_queries"); err == nil {
		t.Error("AI kind without citations: want an error")
	}
	if _, err := ParseBingCSV([]byte("a,b\n1,2\n"), ""); err == nil {
		t.Error("unknown table: want an error")
	}
}

func TestGuessBingKind(t *testing.T) {
	cases := map[string]BingKind{
		"Keywords.csv":                 BingQueries,
		"SearchPerformance_Pages.csv":  BingPages,
		"Mots-clés.csv":                BingQueries,
		"AI Performance - Pages.csv":   BingAIPages,
		"GroundingQueries.csv":         BingAIQueries,
		"ai_performance_chart.csv":     BingAIDates,
		"Search performance chart.csv": BingDates,
		"notes.csv":                    "",
	}
	for name, want := range cases {
		if got := GuessBingKind(name); got != want {
			t.Errorf("GuessBingKind(%q) = %q, want %q", name, got, want)
		}
	}
	if normalizeBingKind("Grounding queries") != BingAIQueries || normalizeBingKind("keywords") != BingQueries {
		t.Error("normalizeBingKind aliases")
	}
}

func TestImportBingExportDir(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"Keywords.csv":         "Keyword,Clicks,Impressions,CTR,Avg. Position\nrotin,12,300,4%,2.5\n",
		"Pages.csv":            "URL,Clicks,Impressions,CTR,Avg. Position\nhttps://x.fr/a,9,200,4.5%,3\n",
		"Chart.csv":            "Date,Clicks,Impressions\n2026-07-01,5,100\n2026-07-02,7,140\n",
		"AI_Performance.csv":   "Grounding query,Citations\nrotin pendant,8\n",
		"GroundingQueries.txt": "ignored",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	bg, err := ImportBingExport(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(bg.Queries) != 1 || len(bg.Pages) != 1 || len(bg.Daily) != 2 || bg.Totals.Clicks.Current != 12 {
		t.Errorf("search: %d queries, %d pages, %d days, clicks %v", len(bg.Queries), len(bg.Pages), len(bg.Daily), bg.Totals.Clicks.Current)
	}
	if bg.AIPerformance == nil || len(bg.AIPerformance.GroundingQueries) != 1 {
		t.Errorf("ai performance = %+v", bg.AIPerformance)
	}
	if _, err := ImportBingExport(t.TempDir()); err == nil {
		t.Error("empty dir: want an error")
	}
}

func TestParseBingRealExportShapes(t *testing.T) {
	// Daily chart as exported by Bing: dates with a time, "Avg. CTR".
	daily := "\uFEFFDate,Clicks,Impressions,Avg. CTR\n6/6/2026 12:00:00 AM,12,400,3\n6/7/2026 12:00:00 AM,8,300,2.67\n"
	tb, err := ParseBingCSV([]byte(daily), GuessBingKind("www.example.fr_SearchPerformanceOverview_All_6_7_2026.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if tb.Kind != BingDates || len(tb.Daily) != 2 || tb.Daily[0].Date != "2026-06-06" || tb.Daily[0].CTR != 3 {
		t.Fatalf("daily chart = %+v", tb)
	}
	ai := "Date,Citations,Cited Pages\n6/6/2026 12:00:00 AM,14,5\n"
	if tb, err := ParseBingCSV([]byte(ai), GuessBingKind("www.example.fr_AIPerformanceOverviewStats_6_7_2026.csv")); err != nil || tb.Kind != BingAIDates || tb.AIDaily[0].CitedPages != 5 {
		t.Fatalf("AI chart = %+v, %v", tb, err)
	}
	gq := "Grounding Query,Intent,Topic,Citations,Citation Share\nbest lamp,Commercial,Lighting,\"1,234\",12.5%\n"
	if tb, err := ParseBingCSV([]byte(gq), GuessBingKind("www.example.fr_AISearchQueriesReport_6_7_2026.csv")); err != nil || tb.Kind != BingAIQueries || tb.GroundingQueries[0].Citations != 1234 {
		t.Fatalf("grounding queries = %+v, %v", tb, err)
	}
	pages := "Page,Citations\nhttps://www.example.fr/a,7\n"
	if tb, err := ParseBingCSV([]byte(pages), GuessBingKind("www.example.fr_AIPageStatsReport_6_7_2026.csv")); err != nil || tb.Kind != BingAIPages {
		t.Fatalf("AI pages = %+v, %v", tb, err)
	}
}

func TestParseBingKeywordsMentioningCitations(t *testing.T) {
	// "citation" in keywords or URLs must not turn a search table into an AI table.
	csv := "Keyword,Clicks,Impressions,CTR,Avg. Position\nlampe rotin,10,300,3.3%,2\ncitation amour,80,3000,2.6%,4.2\ncitations celebres 2024,50,2000,2.5%,5\n"
	tb, err := ParseBingCSV([]byte(csv), "")
	if err != nil {
		t.Fatal(err)
	}
	if tb.Kind != BingQueries || len(tb.Rows) != 3 || len(tb.GroundingQueries) != 0 {
		t.Fatalf("kind %q, %d rows, %d grounding queries", tb.Kind, len(tb.Rows), len(tb.GroundingQueries))
	}
}

func TestBingAIPagesReimportAndPreviousRows(t *testing.T) {
	first := &BingTable{Kind: BingAIPages, CitedPages: []model.BingCitedPage{{URL: "a", Citations: 100}}}
	second := &BingTable{Kind: BingAIPages, CitedPages: []model.BingCitedPage{{URL: "a", Citations: 7}}}
	bg := ApplyBingTable(ApplyBingTable(nil, first), second)
	if c := bg.AIPerformance.Citations.Current; c != 7 {
		t.Errorf("citations after re-import = %v, want 7", c)
	}

	// Rows of the comparison period carry only the previous values.
	csv := "Date,Citations,Previous period Citations\n2026-07-01,5,\n2026-07-02,6,\n2026-06-29,,3\n2026-06-30,,4\n"
	tb, err := ParseBingCSV([]byte(csv), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(tb.AIPreviousDaily) != 2 || tb.AIPreviousDaily[0].Date != "2026-06-29" {
		t.Errorf("previous days = %+v", tb.AIPreviousDaily)
	}
}

func TestGuessBingKindFrenchPages(t *testing.T) {
	if k := GuessBingKind("Performances de recherche - Pages.csv"); k != BingPages {
		t.Errorf("got %q, want pages", k)
	}
	if k := GuessBingKind("Performances de recherche - Mots-clés.csv"); k != BingQueries {
		t.Errorf("got %q, want queries", k)
	}
}
