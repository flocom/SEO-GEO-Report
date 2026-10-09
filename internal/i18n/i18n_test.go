package i18n

import (
	"strings"
	"testing"
	"time"
)

func TestFormats(t *testing.T) {
	cases := []struct{ got, want string }{
		{Number("fr", 12345), "12 345"},
		{Number("en", 12345), "12,345"},
		{Number("fr", -1234567.6), Minus + "1 234 568"},
		{Number("en", -0.2), "0"},
		{Decimal("fr", 3.14159, 2), "3,14"},
		{Decimal("en", 1234.5, 1), "1,234.5"},
		{Percent("fr", 3.2, 1), "3,2 %"},
		{Percent("en", 3.2, 1), "3.2%"},
		{SignedPercent("en", 12.34, 1), "+12.3%"},
		{SignedPercent("fr", -5, 0), Minus + "5 %"},
		{SignedPercent("en", 0, 1), "0.0%"},
		{Points("fr", 1.26, 1), "+1,3 pt"},
		{Points("en", -0.5, 1), Minus + "0.5 pts"},
		{Compact("fr", 12345), "12,3 k"},
		{Compact("en", 1500000), "1.5M"},
		{Compact("en", 950), "950"},
		{Auto("en", 4.25), "4.25"},
		{Auto("fr", 12.5), "12,5"},
		{Currency("fr", 12345, "EUR", 0), "12 345 €"},
		{Currency("en", 12345, "EUR", 0), "€12,345"},
		{Currency("en", 99.5, "USD", -1), "$99.50"},
		{Currency("fr", 10, "CHF", 0), "10 CHF"},
		{Currency("en", 10, "XYZ", 0), "10 XYZ"},
		{Duration("fr", 92), "1 min 32 s"},
		{Duration("en", 92), "1m 32s"},
		{Duration("en", 3725), "1h 2m"},
		{Duration("fr", 8), "8 s"},
		{Date("fr", "2026-07-01"), "1 juil. 2026"},
		{Date("en", "2026-07-01"), "Jul 1, 2026"},
		{Date("en", "not a date"), "not a date"},
		{ShortDate("fr", "2026-08-15"), "15 août"},
		{DateRange("fr", "2026-07-01", "2026-09-30"), "1 juil. – 30 sept. 2026"},
		{DateRange("en", "2026-07-01", "2026-09-30"), "Jul 1 – Sep 30, 2026"},
		{DateRange("en", "2025-12-01", "2026-01-31"), "Dec 1, 2025 – Jan 31, 2026"},
		{FormatTime("en", time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)), "Oct 9, 2026"},
		{Plural("fr", 1, "cover.days"), "1 jour"},
		{Plural("fr", 0, "cover.days"), "0 jour"},
		{Plural("en", 0, "cover.days"), "0 days"},
		{Plural("en", 92, "cover.days"), "92 days"},
		{Lang("EN-us"), "en"},
		{Lang(""), "fr"},
		{T("en", "toc.title"), "Contents"},
		{T("fr", "toc.title"), "Sommaire"},
		{T("fr", "missing.key"), "missing.key"},
	}
	for i, c := range cases {
		if c.got != c.want {
			t.Errorf("case %d: got %q, want %q", i, c.got, c.want)
		}
	}
	if Days("2026-07-01", "2026-09-30") != 92 {
		t.Errorf("Days: got %d", Days("2026-07-01", "2026-09-30"))
	}
}

func TestCatalogComplete(t *testing.T) {
	for k, v := range messages {
		if strings.TrimSpace(v[0]) == "" || strings.TrimSpace(v[1]) == "" {
			t.Errorf("key %q misses a translation", k)
		}
		if strings.Count(v[0], "%s") != strings.Count(v[1], "%s") {
			t.Errorf("key %q: placeholders differ between fr and en", k)
		}
	}
	fr, en := Glossary("fr"), Glossary("en")
	if len(fr) != len(en) || len(fr) < 15 {
		t.Fatalf("glossary sizes: fr=%d en=%d", len(fr), len(en))
	}
	for i := range fr {
		if fr[i].Term == "" || en[i].Definition == "" || fr[i].Key != en[i].Key {
			t.Errorf("glossary entry %d incomplete", i)
		}
	}
}
