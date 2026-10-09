package importer

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

func TestParseNumber(t *testing.T) {
	cases := []struct {
		in           string
		decimalComma bool
		want         float64
		pct, ok      bool
	}{
		{"1234", false, 1234, false, true},
		{"1,234", false, 1234, false, true},
		{"1,234", true, 1.234, false, true},
		{"1,234,567", false, 1234567, false, true},
		{"1 234 567", true, 1234567, false, true},
		{"1\u202f234,5", true, 1234.5, false, true},
		{"1.234,56", true, 1234.56, false, true},
		{"1,234.56", false, 1234.56, false, true},
		{"1.234.567", true, 1234567, false, true},
		{"1.234", true, 1234, false, true},
		{"1.234", false, 1.234, false, true},
		{"0,6234", false, 0.6234, false, true},
		{"0.6234", true, 0.6234, false, true},
		{"3.2%", false, 3.2, true, true},
		{"3,2 %", true, 3.2, true, true},
		{"12,5", false, 12.5, false, true},
		{"€1,234.50", false, 1234.5, false, true},
		{"1 234,56 €", true, 1234.56, false, true},
		{"-12.5", false, -12.5, false, true},
		{"−12,5", true, -12.5, false, true},
		{"", false, 0, false, false},
		{"-", false, 0, false, false},
		{"n/a", false, 0, false, false},
		{"—", false, 0, false, false},
	}
	for _, c := range cases {
		v, pct, ok := parseNumber(c.in, c.decimalComma)
		if ok != c.ok || pct != c.pct || (ok && v != c.want) {
			t.Errorf("parseNumber(%q, %v) = %v, %v, %v; want %v, %v, %v", c.in, c.decimalComma, v, pct, ok, c.want, c.pct, c.ok)
		}
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		in       string
		dayFirst bool
		want     string
	}{
		{"20260701", false, "2026-07-01"},
		{"2026-07-01", false, "2026-07-01"},
		{"2026-07-01T00:00:00", false, "2026-07-01"},
		{"2026/07/01", false, "2026-07-01"},
		{"01/07/2026", true, "2026-07-01"},
		{"7/1/2026", false, "2026-07-01"},
		{"25/07/2026", false, "2026-07-25"},
		{"07/25/2026", true, "2026-07-25"},
		{"01.07.2026", true, "2026-07-01"},
		{"Jul 1, 2026", false, "2026-07-01"},
		{"July 1, 2026", false, "2026-07-01"},
		{"1 juil. 2026", true, "2026-07-01"},
		{"1 août 2026", true, "2026-08-01"},
		{"3 juin 2026", true, "2026-06-03"},
		{"20261341", false, ""},
		{"Grand total", false, ""},
		{"", false, ""},
	}
	for _, c := range cases {
		got, ok := parseDate(c.in, c.dayFirst)
		if got != c.want || ok != (c.want != "") {
			t.Errorf("parseDate(%q) = %q, %v; want %q", c.in, got, ok, c.want)
		}
	}
}

func TestDetectDelimiterAndDecode(t *testing.T) {
	if d := detectDelimiter("a;b;c\n1;2;3"); d != ';' {
		t.Errorf("delimiter = %q", d)
	}
	if d := detectDelimiter("\"a;b\",c,d\n"); d != ',' {
		t.Errorf("delimiter with quotes = %q", d)
	}
	if d := detectDelimiter("# comment, with, commas\na\tb\tc"); d != '\t' {
		t.Errorf("tab delimiter = %q", d)
	}
	if s := decodeText([]byte("\xEF\xBB\xBFa,b\r\nc,d\r\n")); s != "a,b\nc,d\n" {
		t.Errorf("decode BOM/CRLF = %q", s)
	}
	// UTF-16 LE with BOM.
	u16 := []byte{0xFF, 0xFE, 'D', 0, 'a', 0, 't', 0, 'e', 0, ',', 0, 0xE9, 0}
	if s := decodeText(u16); s != "Date,é" {
		t.Errorf("decode UTF-16 = %q", s)
	}
	if s := decodeText([]byte("Requ\xeates")); s != "Requêtes" {
		t.Errorf("decode Latin-1 = %q", s)
	}
}

func f(v float64) *float64 { return &v }

func eqp(p *float64, v float64) bool { return p != nil && *p == v }

func TestParseGSCQueriesEN(t *testing.T) {
	csv := "Top queries,Clicks,Impressions,CTR,Position\n" +
		"lampe rotin,\"1,234\",\"45,678\",2.7%,4.5\n" +
		"\"suspension, cuisine\",120,3400,3.53%,8.1\n" +
		",5,10,50%,1\n"
	tb, err := ParseGSCCSV([]byte(csv), "")
	if err != nil {
		t.Fatal(err)
	}
	if tb.Kind != GSCQueries || len(tb.Rows) != 2 {
		t.Fatalf("table = %+v", tb)
	}
	r := tb.Rows[0]
	if r.Key != "lampe rotin" || r.Clicks != 1234 || r.Impressions != 45678 || r.CTR != 2.7 || r.Position != 4.5 || r.PrevClicks != nil {
		t.Errorf("row = %+v", r)
	}
	if tb.Rows[1].Key != "suspension, cuisine" || tb.Rows[1].CTR != 3.53 {
		t.Errorf("quoted row = %+v", tb.Rows[1])
	}
}

func TestParseGSCFrenchBOMSemicolon(t *testing.T) {
	csv := "\uFEFFRequêtes les plus fréquentes;Clics;Impressions;CTR;Position\n" +
		"lampe rotin;1 234;45 678;2,7 %;4,5\n" +
		"applique murale;\"1\u00a0001\";20 000;5 %;12,25\n"
	tb, err := ParseGSCCSV([]byte(csv), "")
	if err != nil {
		t.Fatal(err)
	}
	if tb.Kind != GSCQueries || len(tb.Rows) != 2 {
		t.Fatalf("table = %+v", tb)
	}
	if r := tb.Rows[0]; r.Clicks != 1234 || r.Impressions != 45678 || r.Position != 4.5 || r.CTR != 2.7 {
		t.Errorf("row = %+v", r)
	}
	if r := tb.Rows[1]; r.Clicks != 1001 || r.Position != 12.25 {
		t.Errorf("row = %+v", r)
	}
}

func TestParseGSCComparison(t *testing.T) {
	en := "Top pages,Last 3 months Clicks,Previous 3 months Clicks,Last 3 months Impressions,Previous 3 months Impressions,Last 3 months CTR,Previous 3 months CTR,Last 3 months Position,Previous 3 months Position\n" +
		"https://www.site.fr/,500,400,10000,9000,5%,4.44%,3.2,3.9\n" +
		"https://www.site.fr/new,50,,1000,,5%,,9.1,\n"
	fr := "Pages les plus populaires,3 derniers mois Clics,3 mois précédents Clics,3 derniers mois Impressions,3 mois précédents Impressions,3 derniers mois CTR,3 mois précédents CTR,3 derniers mois Position,3 mois précédents Position\n" +
		"https://www.site.fr/,500,400,10000,9000,\"5 %\",\"4,44 %\",\"3,2\",\"3,9\"\n" +
		"https://www.site.fr/new,50,,1000,,\"5 %\",,\"9,1\",\n"
	yoy := "Page,Clicks,Same period last year Clicks,Impressions,Same period last year Impressions\n" +
		"https://www.site.fr/,500,400,10000,9000\n" +
		"https://www.site.fr/new,50,,1000,\n"
	for name, csv := range map[string]string{"en": en, "fr": fr, "yoy": yoy} {
		tb, err := ParseGSCCSV([]byte(csv), "")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if tb.Kind != GSCPages || len(tb.Rows) != 2 {
			t.Fatalf("%s: table = %+v", name, tb)
		}
		r := tb.Rows[0]
		if r.Clicks != 500 || !eqp(r.PrevClicks, 400) || !eqp(r.PrevImpressions, 9000) || r.CTR != 5 {
			t.Errorf("%s: row = %+v", name, r)
		}
		if name != "yoy" && (r.Position != 3.2 || !eqp(r.PrevPosition, 3.9)) {
			t.Errorf("%s: positions = %v / %v", name, r.Position, r.PrevPosition)
		}
		if n := tb.Rows[1]; n.PrevClicks != nil || n.PrevImpressions != nil || n.PrevPosition != nil {
			t.Errorf("%s: new page must have no previous values: %+v", name, n)
		}
	}
}

func TestParseGSCDates(t *testing.T) {
	// Comparison columns on the same row: previous dates are shifted by the
	// period length (3 days).
	csv := "Date,Last 3 days Clicks,Previous 3 days Clicks,Last 3 days Impressions,Previous 3 days Impressions,Last 3 days CTR,Previous 3 days CTR,Last 3 days Position,Previous 3 days Position\n" +
		"2026-07-03,30,20,1000,800,3%,2.5%,10,12\n" +
		"2026-07-01,10,5,500,400,2%,1.25%,11,13\n" +
		"2026-07-02,20,10,800,600,2.5%,1.67%,10.5,12.5\n"
	tb, err := ParseGSCCSV([]byte(csv), "")
	if err != nil {
		t.Fatal(err)
	}
	if tb.Kind != GSCDates || len(tb.Daily) != 3 || len(tb.PreviousDaily) != 3 {
		t.Fatalf("table = %+v", tb)
	}
	if tb.Daily[0].Date != "2026-07-01" || tb.Daily[0].Clicks != 10 || tb.Daily[0].CTR != 2 || tb.Daily[0].Position != 11 {
		t.Errorf("daily[0] = %+v", tb.Daily[0])
	}
	if tb.PreviousDaily[0].Date != "2026-06-28" || tb.PreviousDaily[2].Date != "2026-06-30" || tb.PreviousDaily[2].Clicks != 20 {
		t.Errorf("previous daily = %+v", tb.PreviousDaily)
	}

	sc := ApplyGSCTable(nil, tb)
	tot := sc.Totals
	if tot.Clicks.Current != 60 || !eqp(tot.Clicks.Previous, 35) || tot.Impressions.Current != 2300 || !eqp(tot.Impressions.Previous, 1800) {
		t.Errorf("totals = %+v", tot)
	}
	if tot.CTR == nil || tot.CTR.Current != 2.61 || !eqp(tot.CTR.Previous, 1.94) {
		t.Errorf("ctr = %+v", tot.CTR)
	}
	// (10*1000 + 11*500 + 10.5*800) / 2300 = 10.39
	if tot.Position == nil || tot.Position.Current != 10.39 || !eqp(tot.Position.Previous, 12.39) {
		t.Errorf("position = %+v / %v", tot.Position, *tot.Position.Previous)
	}

	// Year-over-year comparison, French headers, dd/mm/yyyy dates.
	fr := "Date;Clics;Clics (même période l'année précédente);Impressions;Impressions (même période l'année précédente)\n" +
		"01/07/2026;10;8;100;90\n02/07/2026;12;9;110;95\n"
	tb, err = ParseGSCCSV([]byte(fr), GSCDates)
	if err != nil {
		t.Fatal(err)
	}
	if len(tb.PreviousDaily) != 2 || tb.PreviousDaily[0].Date != "2025-07-01" || tb.Daily[1].Date != "2026-07-02" {
		t.Errorf("yoy = %+v / %+v", tb.Daily, tb.PreviousDaily)
	}

	// Comparison rows listed with their own dates (current cells empty).
	rows := "Date,Last 2 days Clicks,Previous 2 days Clicks,Last 2 days Impressions,Previous 2 days Impressions\n" +
		"2026-07-01,10,,100,\n2026-07-02,12,,110,\n2026-06-29,,7,,70\n2026-06-30,,8,,80\n"
	tb, err = ParseGSCCSV([]byte(rows), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(tb.Daily) != 2 || len(tb.PreviousDaily) != 2 || tb.PreviousDaily[0].Date != "2026-06-29" || tb.PreviousDaily[1].Clicks != 8 {
		t.Errorf("own-date comparison = %+v / %+v", tb.Daily, tb.PreviousDaily)
	}

	// Two unlabeled columns per metric (date ranges as labels): first is current.
	ranges := "Date,2026-07-01 - 2026-07-02 Clicks,2026-06-29 - 2026-06-30 Clicks,2026-07-01 - 2026-07-02 Impressions,2026-06-29 - 2026-06-30 Impressions\n" +
		"2026-07-01,10,7,100,70\n2026-07-02,12,8,110,80\n"
	tb, err = ParseGSCCSV([]byte(ranges), "")
	if err != nil {
		t.Fatal(err)
	}
	if tb.Daily[0].Clicks != 10 || len(tb.PreviousDaily) != 2 || tb.PreviousDaily[0].Clicks != 7 || tb.PreviousDaily[0].Date != "2026-06-29" {
		t.Errorf("unlabeled comparison = %+v / %+v", tb.Daily, tb.PreviousDaily)
	}
}

func TestParseGSCKindsAndErrors(t *testing.T) {
	cases := map[string]GSCKind{
		"Top queries,Clicks,Impressions\nx,1,2\n":                                               GSCQueries,
		"Requêtes les plus fréquentes,Clics,Impressions\nx,1,2\n":                               GSCQueries,
		"Top pages,Clicks,Impressions\nx,1,2\n":                                                 GSCPages,
		"Pages les plus populaires,Clics,Impressions\nx,1,2\n":                                  GSCPages,
		"Country,Clicks,Impressions\nFrance,1,2\n":                                              GSCCountries,
		"Pays,Clics,Impressions\nFrance,1,2\n":                                                  GSCCountries,
		"Device,Clicks,Impressions\nMobile,1,2\n":                                               GSCDevices,
		"Appareil,Clics,Impressions\nMobile,1,2\n":                                              GSCDevices,
		"Search Appearance,Clicks,Impressions\nVideos,1,2\n":                                    GSCAppearance,
		"Apparence dans les résultats de recherche,Clics,Impressions\nVidéos,1,2\n":             GSCAppearance,
		"Date,Clicks,Impressions\n2026-07-01,1,2\n":                                             GSCDates,
		"Jour,Clics,Impressions\n2026-07-01,1,2\n":                                              GSCDates,
		"Something,Clicks,Impressions\n2026-07-01,1,2\n":                                        GSCDates,
		"Export date: 2026-10-01\n\nTop queries,Clicks,Impressions,CTR,Position\nx,1,2,50%,3\n": GSCQueries,
	}
	for csv, want := range cases {
		tb, err := ParseGSCCSV([]byte(csv), "")
		if err != nil || tb.Kind != want {
			t.Errorf("%q: kind = %v, err = %v; want %s", csv, tb, err, want)
		}
	}
	if _, err := ParseGSCCSV([]byte("Filter,Value\nSearch type,Web\n"), ""); err == nil {
		t.Error("filters table must fail")
	}
	if _, err := ParseGSCCSV([]byte("Unknown,Clicks\nabc,1\n"), ""); err == nil {
		t.Error("unknown kind must fail")
	}
	if tb, err := ParseGSCCSV([]byte("Unknown,Clicks\nabc,1\n"), GSCQueries); err != nil || len(tb.Rows) != 1 {
		t.Errorf("explicit kind: %v %v", tb, err)
	}
	for name, want := range map[string]GSCKind{
		"Queries.csv": GSCQueries, "Requêtes.csv": GSCQueries, "Pages.csv": GSCPages, "Pays.csv": GSCCountries,
		"Countries.csv": GSCCountries, "Appareils.csv": GSCDevices, "Devices.csv": GSCDevices, "Dates.csv": GSCDates,
		"Chart.csv": GSCDates, "Search appearance.csv": GSCAppearance,
		"Apparence dans les résultats de recherche.csv": GSCAppearance, "Filters.csv": "",
	} {
		if got := GuessGSCKind(name); got != want {
			t.Errorf("GuessGSCKind(%q) = %q, want %q", name, got, want)
		}
	}
}

var gscFiles = map[string]string{
	"Queries.csv": "Top queries,Last 28 days Clicks,Previous period Clicks,Last 28 days Impressions,Previous period Impressions,Last 28 days CTR,Previous period CTR,Last 28 days Position,Previous period Position\n" +
		"lampe rotin,40,20,1000,800,4%,2.5%,3.1,5.2\n",
	"Pages.csv":             "Top pages,Clicks,Impressions,CTR,Position\nhttps://www.site.fr/,30,600,5%,2\n",
	"Devices.csv":           "Device,Clicks,Impressions,CTR,Position\nMobile,40,1500,2.67%,9\nDesktop,20,800,2.5%,8\n",
	"Countries.csv":         "Country,Clicks,Impressions,CTR,Position\nFrance,55,2000,2.75%,8.5\nBelgium,5,300,1.67%,11\n",
	"Search appearance.csv": "Search Appearance,Clicks,Impressions,CTR,Position\nProduct snippets,12,400,3%,7\n",
	"Dates.csv":             "Date,Clicks,Impressions,CTR,Position\n2026-07-02,25,1200,2.08%,9\n2026-07-01,35,1100,3.18%,8\n",
	"Filters.csv":           "Filter,Value\nSearch type,Web\nDate,Last 28 days\n",
}

func checkImported(t *testing.T, sc *model.SearchConsole, withDates bool) {
	t.Helper()
	if sc == nil {
		t.Fatal("nil search console")
	}
	if len(sc.Queries) != 1 || !eqp(sc.Queries[0].PrevClicks, 20) || len(sc.Pages) != 1 || len(sc.Devices) != 2 ||
		len(sc.Countries) != 2 || len(sc.SearchAppearance) != 1 {
		t.Errorf("tables = %+v", sc)
	}
	if withDates {
		if len(sc.Daily) != 2 || sc.Daily[0].Date != "2026-07-01" || sc.Totals.Clicks.Current != 60 || sc.Totals.Impressions.Current != 2300 {
			t.Errorf("dates/totals = %+v / %+v", sc.Daily, sc.Totals)
		}
		if sc.Totals.Position == nil || sc.Totals.Position.Current != 8.52 { // (9*1200+8*1100)/2300
			t.Errorf("position = %+v", sc.Totals.Position)
		}
	} else if sc.Totals.Clicks.Current != 60 || sc.Totals.Impressions.Current != 2300 || sc.Totals.CTR == nil || sc.Totals.CTR.Current != 2.61 {
		t.Errorf("totals from devices = %+v", sc.Totals)
	}
	if sc.Totals.Clicks.Previous != nil {
		t.Errorf("no comparison in dates/devices: %+v", sc.Totals.Clicks)
	}
}

func TestImportGSCExportZip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "export.zip")
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(out)
	for name, content := range gscFiles {
		w, err := zw.Create("export-2026/" + name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte("\uFEFF" + content))
	}
	w, _ := zw.Create("__MACOSX/export-2026/._Queries.csv")
	w.Write([]byte{0, 5, 22, 7, 0xff})
	zw.Create("export-2026/")
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	out.Close()

	sc, err := ImportGSCExport(path)
	if err != nil {
		t.Fatal(err)
	}
	checkImported(t, sc, true)
}

func TestImportGSCExportDirFrench(t *testing.T) {
	dir := t.TempDir()
	frNames := map[string]string{
		"Queries.csv": "Requêtes.csv", "Pages.csv": "Pages.csv", "Devices.csv": "Appareils.csv",
		"Countries.csv": "Pays.csv", "Search appearance.csv": "Apparence dans les résultats de recherche.csv",
		"Filters.csv": "Filtres.csv", "Dates.csv": "Dates.csv",
	}
	for en, content := range gscFiles {
		if err := os.WriteFile(filepath.Join(dir, frNames[en]), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignored"), 0o644)
	sc, err := ImportGSCExport(dir)
	if err != nil {
		t.Fatal(err)
	}
	checkImported(t, sc, true)

	// Without Dates.csv, totals come from the devices table.
	os.Remove(filepath.Join(dir, "Dates.csv"))
	sc, err = ImportGSCExport(dir)
	if err != nil {
		t.Fatal(err)
	}
	checkImported(t, sc, false)
}

func TestImportGSCExportErrors(t *testing.T) {
	if _, err := ImportGSCExport(filepath.Join(t.TempDir(), "missing.zip")); err == nil {
		t.Error("missing path must fail")
	}
	empty := t.TempDir()
	if _, err := ImportGSCExport(empty); err == nil {
		t.Error("empty dir must fail")
	}
	os.WriteFile(filepath.Join(empty, "Filters.csv"), []byte(gscFiles["Filters.csv"]), 0o644)
	os.WriteFile(filepath.Join(empty, "junk.csv"), []byte("a,b\n1,2\n"), 0o644)
	if _, err := ImportGSCExport(empty); err == nil || !strings.Contains(err.Error(), "junk.csv") {
		t.Errorf("err = %v", err)
	}
	txt := filepath.Join(empty, "x.txt")
	os.WriteFile(txt, []byte("x"), 0o644)
	if _, err := ImportGSCExport(txt); err == nil {
		t.Error("txt file must fail")
	}
	// A single CSV file is accepted.
	q := filepath.Join(empty, "Queries.csv")
	os.WriteFile(q, []byte(gscFiles["Queries.csv"]), 0o644)
	if sc, err := ImportGSCExport(q); err != nil || len(sc.Queries) != 1 {
		t.Errorf("single csv: %v %v", sc, err)
	}
}

func TestApplyGSCTableNil(t *testing.T) {
	sc := ApplyGSCTable(nil, nil)
	if sc == nil {
		t.Fatal("ApplyGSCTable(nil, nil) must return a SearchConsole")
	}
	sc.Totals.Clicks = model.M(5, 4)
	sc = ApplyGSCTable(sc, &GSCTable{Kind: GSCQueries, Rows: []model.GSCRow{{Key: "a"}}})
	if len(sc.Queries) != 1 || sc.Totals.Clicks.Current != 5 {
		t.Errorf("sc = %+v", sc)
	}
}

const ga4Header = `# ----------------------------------------
# Acquisition de trafic : Groupe de canaux par défaut de la session
# Compte : Maison Lumen
# Date de début : 20260701
# Date de fin : 20260928
# ----------------------------------------

`

func TestParseGA4ChannelsEN(t *testing.T) {
	csv := `# ----------------------------------------
# Traffic acquisition: Session default channel group
# Start date: 20260701
# End date: 20260928
# ----------------------------------------

Session default channel group,Sessions,Engaged sessions,Engagement rate,Average engagement time per session,Events per session,Event count,Key events,Total revenue
Grand total,1000,600,0.6,50,5,5000,30,1500.5
Organic Search,600,400,0.6667,55,5,3000,20,1000
Direct,400,200,0.5,45,5,2000,10,500.5

# ----------------------------------------
Date,Sessions
20260701,10
`
	a, err := ParseGA4CSV([]byte(csv), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Channels) != 2 || len(a.Daily) != 0 {
		t.Fatalf("analytics = %+v", a)
	}
	o := a.Channels[0]
	if o.Channel != "Organic Search" || o.Sessions != 600 || o.EngagementRate != 66.67 || o.KeyEvents != 20 || o.Revenue != 1000 || o.PrevSessions != nil {
		t.Errorf("organic = %+v", o)
	}
	tt := a.Totals
	if tt.Sessions == nil || tt.Sessions.Current != 1000 || tt.KeyEvents.Current != 30 || tt.Revenue.Current != 1500.5 || tt.EngagementRate.Current != 60 {
		t.Errorf("totals = %+v", tt)
	}
	if tt.Sessions.Previous != nil || tt.Users != nil {
		t.Errorf("unexpected previous / users: %+v", tt)
	}
	if a.OrganicTotals == nil || a.OrganicTotals.Sessions.Current != 600 || a.OrganicTotals.EngagementRate.Current != 66.67 {
		t.Errorf("organic totals = %+v", a.OrganicTotals)
	}
}

func TestParseGA4ChannelsFrenchSemicolon(t *testing.T) {
	csv := "\uFEFF" + ga4Header +
		"Groupe de canaux par défaut de la session;Sessions;Utilisateurs actifs;Taux d'engagement;Événements clés;Revenu total\n" +
		"Organic Search;1 200;950;62,5 %;40;1 234,56 €\n" +
		"Direct;800;700;55 %;10;100\n"
	a, err := ParseGA4CSV([]byte(csv), GA4Channels, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Channels) != 2 {
		t.Fatalf("channels = %+v", a.Channels)
	}
	if c := a.Channels[0]; c.Sessions != 1200 || c.Users != 950 || c.EngagementRate != 62.5 || c.KeyEvents != 40 || c.Revenue != 1234.56 {
		t.Errorf("row = %+v", c)
	}
	// Totals from sums: engagement weighted by sessions (62.5*1200+55*800)/2000 = 59.5.
	tt := a.Totals
	if tt.Sessions.Current != 2000 || tt.EngagementRate.Current != 59.5 || tt.KeyEvents.Current != 50 || tt.Revenue.Current != 1334.56 {
		t.Errorf("totals = %+v %+v %+v %+v", tt.Sessions, tt.EngagementRate, tt.KeyEvents, tt.Revenue)
	}
	if tt.Users != nil {
		t.Error("users are not additive across channels")
	}
	if a.OrganicTotals == nil || a.OrganicTotals.Users.Current != 950 {
		t.Errorf("organic totals = %+v", a.OrganicTotals)
	}
}

func TestParseGA4SourcesAndMerge(t *testing.T) {
	csv := ga4Header + "Source/support de la session,Sessions,Utilisateurs actifs,Taux d'engagement,Événements clés,Revenus totaux\n" +
		"google / organic,5000,4000,0.65,100,9000\n" +
		"chatgpt.com / referral,120,100,0.72,4,300\n" +
		"perplexity.ai / referral,40,35,0.7,1,80\n" +
		"(direct) / (none),3000,2500,0.6,60,5000\n"
	existing := &model.Analytics{Totals: model.GA4Totals{Sessions: &model.Metric{Current: 9999}}}
	a, err := ParseGA4CSV([]byte(csv), "", existing)
	if err != nil {
		t.Fatal(err)
	}
	if a != existing {
		t.Error("must merge into the given Analytics")
	}
	if len(a.Sources) != 4 || a.Totals.Sessions.Current != 9999 {
		t.Fatalf("analytics = %+v", a)
	}
	s := a.Sources[1]
	if s.Source != "chatgpt.com" || s.Medium != "referral" || s.Sessions != 120 || s.EngagementRate != 72 || s.Revenue != 300 {
		t.Errorf("source = %+v", s)
	}
	if a.Sources[3].Source != "(direct)" || a.Sources[3].Medium != "(none)" {
		t.Errorf("direct = %+v", a.Sources[3])
	}

	// Separate source and medium columns.
	csv2 := "Session source,Session medium,Sessions\ngoogle,organic,10\nclaude.ai,referral,3\n"
	a2, err := ParseGA4CSV([]byte(csv2), GA4Sources, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(a2.Sources) != 2 || a2.Sources[1].Source != "claude.ai" || a2.Sources[1].Medium != "referral" {
		t.Errorf("sources = %+v", a2.Sources)
	}
}

func TestParseGA4LandingPagesComparison(t *testing.T) {
	csv := "Page de destination,Sessions,Sessions (période précédente),Taux d'engagement,Événements clés\n" +
		"/,1000,800,64%,10\n" +
		"/suspensions,500,,70%,5\n" +
		"Total,1500,800,66%,15\n"
	a, err := ParseGA4CSV([]byte(csv), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.LandingPages) != 2 {
		t.Fatalf("landing pages = %+v", a.LandingPages)
	}
	if lp := a.LandingPages[0]; lp.Page != "/" || lp.Sessions != 1000 || !eqp(lp.PrevSessions, 800) || lp.EngagementRate != 64 {
		t.Errorf("row = %+v", lp)
	}
	if lp := a.LandingPages[1]; !eqp(lp.PrevSessions, 0) {
		t.Errorf("new page prev = %v", lp.PrevSessions)
	}
	if a.Totals.Sessions != nil {
		t.Error("landing pages must not fill site totals")
	}
}

func TestParseGA4DateRangeComparison(t *testing.T) {
	csv := "Session default channel group,Date range,Sessions,Key events\n" +
		"Organic Search,Jul 1 - Sep 28 2026,600,20\n" +
		"Direct,Jul 1 - Sep 28 2026,400,10\n" +
		"Organic Search,Apr 2 - Jun 30 2026,450,15\n" +
		"Direct,Apr 2 - Jun 30 2026,380,9\n" +
		"Email,Apr 2 - Jun 30 2026,20,1\n"
	a, err := ParseGA4CSV([]byte(csv), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Channels) != 3 {
		t.Fatalf("channels = %+v", a.Channels)
	}
	if c := a.Channels[0]; c.Sessions != 600 || !eqp(c.PrevSessions, 450) {
		t.Errorf("organic = %+v", c)
	}
	if c := a.Channels[2]; c.Channel != "Email" || c.Sessions != 0 || !eqp(c.PrevSessions, 20) {
		t.Errorf("email = %+v", c)
	}
	if s := a.Totals.Sessions; s == nil || s.Current != 1000 || !eqp(s.Previous, 850) {
		t.Errorf("sessions = %+v", s)
	}
	if k := a.Totals.KeyEvents; k == nil || k.Current != 30 || !eqp(k.Previous, 25) {
		t.Errorf("key events = %+v", k)
	}
	if o := a.OrganicTotals; o == nil || o.Sessions.Current != 600 || !eqp(o.Sessions.Previous, 450) {
		t.Errorf("organic totals = %+v", o)
	}
}

func TestParseGA4Daily(t *testing.T) {
	csv := ga4Header + "Date,Sessions,Utilisateurs actifs\n20260702,120,100\n20260701,100,90\n\n# Fin\n"
	a, err := ParseGA4CSV([]byte(csv), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Daily) != 2 || a.Daily[0].Date != "2026-07-01" || a.Daily[1].Sessions != 120 {
		t.Fatalf("daily = %+v", a.Daily)
	}
	if a.Totals.Sessions == nil || a.Totals.Sessions.Current != 220 || a.Totals.Sessions.Previous != nil || a.Totals.Users != nil {
		t.Errorf("totals = %+v", a.Totals)
	}

	// Date x channel: organic sessions per day; Date x source: AI sessions.
	csv = "Date,Session default channel group,Sessions\n" +
		"20260701,Organic Search,60\n20260701,Direct,40\n20260702,Organic Search,70\n20260702,Referral,10\n"
	a, err = ParseGA4CSV([]byte(csv), GA4Daily, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Daily) != 2 || a.Daily[0].Sessions != 100 || a.Daily[0].OrganicSessions != 60 || a.Daily[1].OrganicSessions != 70 {
		t.Errorf("daily x channel = %+v", a.Daily)
	}
	csv = "Date,Session source / medium,Sessions\n" +
		"20260701,google / organic,60\n20260701,chatgpt.com / referral,5\n20260701,perplexity.ai / referral,2\n"
	a, err = ParseGA4CSV([]byte(csv), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Daily) != 1 || a.Daily[0].Sessions != 67 || a.Daily[0].AISessions != 7 || a.Daily[0].OrganicSessions != 60 {
		t.Errorf("daily x source = %+v", a.Daily)
	}

	// Comparison columns: previous dates shifted by the period length.
	csv = "Date,Sessions,Sessions (previous period)\n20260701,100,80\n20260702,110,90\nGrand total,210,170\n"
	a, err = ParseGA4CSV([]byte(csv), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.PreviousDaily) != 2 || a.PreviousDaily[0].Date != "2026-06-29" || a.PreviousDaily[1].Sessions != 90 {
		t.Errorf("previous daily = %+v", a.PreviousDaily)
	}
	if s := a.Totals.Sessions; s.Current != 210 || !eqp(s.Previous, 170) {
		t.Errorf("sessions = %+v", s)
	}
}

func TestParseGA4Errors(t *testing.T) {
	if _, err := ParseGA4CSV([]byte("# only comments\n# here\n"), "", nil); err == nil {
		t.Error("comments only must fail")
	}
	if _, err := ParseGA4CSV([]byte("Foo,Bar\n1,2\n"), "", nil); err == nil {
		t.Error("unknown table must fail")
	}
	a := &model.Analytics{}
	got, err := ParseGA4CSV([]byte("Date,Sessions\n20260701,1\n"), GA4Channels, a)
	if err == nil || got != a {
		t.Errorf("kind mismatch: %v %v", got, err)
	}
	if a, err := ParseGA4CSV([]byte("Date,Sessions\n20260701,1\n"), "Dates", nil); err != nil || len(a.Daily) != 1 {
		t.Errorf("kind alias: %v %v", a, err)
	}
	if a, err := ParseGA4CSV([]byte("Landing page,Sessions\n/,1\n"), "nonsense", nil); err != nil || len(a.LandingPages) != 1 {
		t.Errorf("unknown kind must be guessed: %v %v", a, err)
	}
	if tb, err := ParseGSCCSV([]byte("Unknown,Clicks\nabc,1\n"), "Query"); err != nil || tb.Kind != GSCQueries {
		t.Errorf("gsc kind alias: %v %v", tb, err)
	}
	if k := GuessGA4Kind([]byte(ga4Header + "Page de destination,Sessions\n/,1\n")); k != GA4LandingPages {
		t.Errorf("GuessGA4Kind = %q", k)
	}
	if k := GuessGA4Kind([]byte("x,y\n")); k != "" {
		t.Errorf("GuessGA4Kind = %q", k)
	}
}
