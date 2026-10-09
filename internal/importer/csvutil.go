package importer

import (
	"encoding/csv"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

// decodeText returns the content as UTF-8 text without BOM, with "\n" line
// endings. UTF-16 files (with BOM, as saved by some spreadsheet tools) are
// converted; invalid UTF-8 is read as Windows-1252/Latin-1.
func decodeText(data []byte) string {
	switch {
	case len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE:
		data = utf16ToUTF8(data[2:], false)
	case len(data) >= 2 && data[0] == 0xFE && data[1] == 0xFF:
		data = utf16ToUTF8(data[2:], true)
	}
	s := string(data)
	s = strings.TrimPrefix(s, "\uFEFF")
	if !utf8.ValidString(s) {
		rs := make([]rune, 0, len(data))
		for _, b := range []byte(s) {
			rs = append(rs, rune(b))
		}
		s = string(rs)
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

func utf16ToUTF8(b []byte, bigEndian bool) []byte {
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		if bigEndian {
			u = append(u, uint16(b[i])<<8|uint16(b[i+1]))
		} else {
			u = append(u, uint16(b[i+1])<<8|uint16(b[i]))
		}
	}
	return []byte(string(utf16.Decode(u)))
}

// detectDelimiter picks ',', ';' or '\t' from the first non-empty line,
// counting separators outside quotes.
func detectDelimiter(text string) rune {
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		counts := map[rune]int{}
		inQuotes := false
		for _, r := range line {
			switch {
			case r == '"':
				inQuotes = !inQuotes
			case !inQuotes && (r == ',' || r == ';' || r == '\t'):
				counts[r]++
			}
		}
		best, n := ',', 0
		for _, d := range []rune{',', ';', '\t'} {
			if counts[d] > n {
				best, n = d, counts[d]
			}
		}
		return best
	}
	return ','
}

// readRecords parses CSV text leniently (variable field counts, lazy quotes,
// blank lines skipped).
func readRecords(text string, delim rune) ([][]string, error) {
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = delim
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	recs, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("invalid CSV: %w", err)
	}
	return recs, nil
}

var foldMap = map[rune]string{
	'à': "a", 'â': "a", 'ä': "a", 'á': "a", 'ã': "a",
	'é': "e", 'è': "e", 'ê': "e", 'ë': "e",
	'î': "i", 'ï': "i", 'í': "i",
	'ô': "o", 'ö': "o", 'ó': "o",
	'ù': "u", 'û': "u", 'ü': "u", 'ú': "u",
	'ç': "c", 'œ': "oe", 'æ': "ae", 'ñ': "n",
	'’': "'", '‘': "'", '\u00a0': " ", '\u202f': " ", '\t': " ", '\uFEFF': "",
}

// fold lowercases s, removes accents and collapses spaces, for header and
// file name matching.
func fold(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if rep, ok := foldMap[r]; ok {
			b.WriteString(rep)
		} else {
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(strings.Trim(b.String(), "\" ")), " ")
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// isPreviousHeader reports whether a (folded) column header refers to the
// comparison period: "Previous 3 months Clicks", "3 mois précédents Clics",
// "Same period last year Clicks"...
func isPreviousHeader(h string) bool {
	return containsAny(h, "previous", "precedent", "prior", "last year", "year ago", "earlier",
		"annee derniere", "an dernier", "annee precedente", "l'an passe", "annee passee",
		"compared", "comparison", "comparaison", "reference")
}

// isYearComparison reports whether a previous-period header is a
// year-over-year comparison.
func isYearComparison(h string) bool {
	return containsAny(h, "year", "annee", "an dernier", "an passe")
}

// looksFrench reports whether folded headers contain French vocabulary.
func looksFrench(headers []string) bool {
	for _, h := range headers {
		if containsAny(h, "clics", "requete", "pays", "appareil", "utilisateurs", "taux", "groupe de canaux",
			"page de destination", "evenements", "revenu", "support", "precedent", "apparence", "mois") {
			return true
		}
	}
	return false
}

// parseNumber parses a localized number: "1,234.5", "1 234,5", "1.234,5",
// "3,2 %", "12.5%", "€1,234". pct reports whether a percent sign was present.
// decimalComma is a hint used for ambiguous values such as "1,234".
func parseNumber(s string, decimalComma bool) (v float64, pct bool, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false, false
	}
	pct = strings.ContainsRune(s, '%')
	neg := false
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r == '.', r == ',':
			b.WriteRune(r)
		case r == '-' || r == '−':
			if b.Len() == 0 {
				neg = true
			}
		case r == '(' && b.Len() == 0: // accounting negative "(12)"
			neg = true
		}
	}
	t := b.String()
	if t == "" || strings.Trim(t, ".,") == "" {
		return 0, pct, false
	}
	hasDot, hasComma := strings.Contains(t, "."), strings.Contains(t, ",")
	switch {
	case hasDot && hasComma:
		if strings.LastIndex(t, ",") > strings.LastIndex(t, ".") {
			t = strings.ReplaceAll(t, ".", "")
			t = strings.Replace(t, ",", ".", 1)
		} else {
			t = strings.ReplaceAll(t, ",", "")
		}
	case hasComma:
		if strings.Count(t, ",") > 1 || (!decimalComma && thousandsGrouped(t, ",")) {
			t = strings.ReplaceAll(t, ",", "")
		} else {
			t = strings.Replace(t, ",", ".", 1)
		}
	case hasDot:
		if strings.Count(t, ".") > 1 || (decimalComma && thousandsGrouped(t, ".")) {
			t = strings.ReplaceAll(t, ".", "")
		}
	}
	f, err := strconv.ParseFloat(t, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, pct, false
	}
	if neg {
		f = -f
	}
	return f, pct, true
}

// thousandsGrouped reports whether t (one separator) looks like "1,234":
// 1-3 leading digits not starting with 0, then exactly 3 digits.
func thousandsGrouped(t, sep string) bool {
	before, after, found := strings.Cut(t, sep)
	return found && len(before) >= 1 && len(before) <= 3 && before[0] != '0' && len(after) == 3
}

var monthPrefixes = []struct {
	prefix string
	month  int
}{
	{"janv", 1}, {"jan", 1}, {"fevr", 2}, {"fev", 2}, {"feb", 2}, {"mars", 3}, {"mar", 3},
	{"avr", 4}, {"apr", 4}, {"mai", 5}, {"may", 5}, {"juin", 6}, {"jun", 6},
	{"juil", 7}, {"jul", 7}, {"aout", 8}, {"aug", 8}, {"sept", 9}, {"sep", 9},
	{"oct", 10}, {"nov", 11}, {"dec", 12},
}

func monthIndex(tok string) int {
	for _, m := range monthPrefixes {
		if strings.HasPrefix(tok, m.prefix) {
			return m.month
		}
	}
	return 0
}

func validDate(y, m, d int) (string, bool) {
	if y < 100 {
		y += 2000
	}
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	if t.Year() != y || int(t.Month()) != m || t.Day() != d {
		return "", false
	}
	return t.Format("2006-01-02"), true
}

// parseDate converts GSC/GA4 date formats to YYYY-MM-DD: "20260701" (GA4),
// "2026-07-01", "01/07/2026" (dayFirst) or "7/1/2026", "Jul 1, 2026",
// "1 juil. 2026".
func parseDate(s string, dayFirst bool) (string, bool) {
	s = strings.TrimSpace(s)
	if len(s) == 8 && allDigits(s) {
		y, _ := strconv.Atoi(s[:4])
		m, _ := strconv.Atoi(s[4:6])
		d, _ := strconv.Atoi(s[6:])
		return validDate(y, m, d)
	}
	if len(s) >= 10 && allDigits(s[:4]) && (s[4] == '-' || s[4] == '/' || s[4] == '.') {
		y, e1 := strconv.Atoi(s[:4])
		m, e2 := strconv.Atoi(s[5:7])
		d, e3 := strconv.Atoi(s[8:10])
		if e1 == nil && e2 == nil && e3 == nil {
			return validDate(y, m, d)
		}
	}
	for _, sep := range []string{"/", ".", "-"} {
		parts := strings.Split(s, sep)
		if len(parts) == 3 && allDigits(parts[0]) && allDigits(parts[1]) && allDigits(parts[2]) && len(parts[2]) >= 2 {
			a, _ := strconv.Atoi(parts[0])
			b, _ := strconv.Atoi(parts[1])
			y, _ := strconv.Atoi(parts[2])
			switch {
			case a > 12:
				return validDate(y, b, a)
			case b > 12:
				return validDate(y, a, b)
			case dayFirst:
				return validDate(y, b, a)
			default:
				return validDate(y, a, b)
			}
		}
	}
	// Month names: "Jul 1, 2026", "July 1 2026", "1 juil. 2026", "1 July 2026".
	toks := strings.FieldsFunc(fold(s), func(r rune) bool { return r == ' ' || r == ',' || r == '.' })
	if len(toks) >= 3 {
		if m := monthIndex(toks[0]); m > 0 && allDigits(toks[1]) && allDigits(toks[2]) {
			d, _ := strconv.Atoi(toks[1])
			y, _ := strconv.Atoi(toks[2])
			return validDate(y, m, d)
		}
		if m := monthIndex(toks[1]); m > 0 && allDigits(toks[0]) && allDigits(toks[2]) {
			d, _ := strconv.Atoi(toks[0])
			y, _ := strconv.Atoi(toks[2])
			return validDate(y, m, d)
		}
	}
	return "", false
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func cell(rec []string, i int) string {
	if i < 0 || i >= len(rec) {
		return ""
	}
	return strings.TrimSpace(rec[i])
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// shiftDates returns the function mapping a current-period date to the
// matching comparison date: one year earlier for year-over-year comparisons,
// otherwise the period length earlier (previous period right before).
func shiftDates(dates []string, yearOverYear bool) func(string) string {
	var first, last time.Time
	for _, d := range dates {
		t, err := time.Parse("2006-01-02", d)
		if err != nil {
			continue
		}
		if first.IsZero() || t.Before(first) {
			first = t
		}
		if t.After(last) {
			last = t
		}
	}
	n := int(last.Sub(first).Hours()/24) + 1
	return func(d string) string {
		t, err := time.Parse("2006-01-02", d)
		if err != nil {
			return d
		}
		if yearOverYear {
			return t.AddDate(-1, 0, 0).Format("2006-01-02")
		}
		return t.AddDate(0, 0, -n).Format("2006-01-02")
	}
}
