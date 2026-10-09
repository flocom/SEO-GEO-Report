// Package i18n holds the French and English texts of the report and the
// locale-aware formatting helpers (numbers, percentages, currencies, dates,
// durations).
//
// Supported languages are "fr" (default) and "en". Every function accepts any
// language string and normalizes it with Lang.
package i18n

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Typographic characters used by the French locale.
const (
	// NNBSP is the narrow no-break space used as French thousands separator
	// and before the percent sign.
	NNBSP = " "
	// NBSP is the regular no-break space.
	NBSP = " "
	// Minus is the typographic minus sign used for negative values.
	Minus = "−"
)

// Lang normalizes a language code: anything starting with "en" is English,
// everything else is French.
func Lang(lang string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(lang)), "en") {
		return "en"
	}
	return "fr"
}

// T returns the text for key in lang. It falls back to English, then to the key
// itself so that a missing translation is visible but never fatal.
func T(lang, key string) string {
	if m, ok := messages[key]; ok {
		if Lang(lang) == "en" || m[0] == "" {
			if m[1] != "" {
				return m[1]
			}
			return m[0]
		}
		return m[0]
	}
	return key
}

// Has reports whether key exists in the catalog.
func Has(key string) bool {
	_, ok := messages[key]
	return ok
}

// Tf formats the text for key with fmt.Sprintf.
func Tf(lang, key string, args ...any) string {
	return fmt.Sprintf(T(lang, key), args...)
}

// Plural picks key+".one" or key+".other" according to n and the language
// plural rule (French: 0 and 1 are singular), then formats n into it (the
// message may contain one %s receiving the formatted number).
func Plural(lang string, n float64, key string) string {
	form := ".other"
	a := math.Abs(n)
	if Lang(lang) == "fr" {
		if a < 2 {
			form = ".one"
		}
	} else if a == 1 {
		form = ".one"
	}
	msg := T(lang, key+form)
	if strings.Contains(msg, "%s") {
		return fmt.Sprintf(msg, Number(lang, n))
	}
	return msg
}

func seps(lang string) (group, decimal string) {
	if Lang(lang) == "en" {
		return ",", "."
	}
	return NNBSP, ","
}

// Decimal formats v with exactly digits decimals and locale separators:
// "12 345,6" (fr) / "12,345.6" (en).
func Decimal(lang string, v float64, digits int) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "–"
	}
	if digits < 0 {
		digits = 0
	}
	group, dec := seps(lang)
	s := strconv.FormatFloat(math.Abs(v), 'f', digits, 64)
	intPart, frac, _ := strings.Cut(s, ".")
	var b strings.Builder
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteString(group)
		}
		b.WriteRune(c)
	}
	out := b.String()
	if frac != "" {
		out += dec + frac
	}
	if v < 0 && strings.Trim(s, "0.") != "" {
		out = Minus + out
	}
	return out
}

// Number formats v rounded to an integer: "12 345" / "12,345".
func Number(lang string, v float64) string { return Decimal(lang, math.Round(v), 0) }

// Auto formats v with a precision adapted to its magnitude: integers are shown
// without decimals, small fractional values with one or two decimals.
func Auto(lang string, v float64) string {
	a := math.Abs(v)
	switch {
	case a >= 100 || a == math.Trunc(a):
		return Number(lang, v)
	case a >= 10:
		return trimZeros(lang, Decimal(lang, v, 1))
	default:
		return trimZeros(lang, Decimal(lang, v, 2))
	}
}

func trimZeros(lang, s string) string {
	_, dec := seps(lang)
	if !strings.Contains(s, dec) {
		return s
	}
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, dec)
}

// Compact formats large numbers in a short form for chart axes:
// "12,3 k" / "12.3K", "1,2 M" / "1.2M".
func Compact(lang string, v float64) string {
	a := math.Abs(v)
	unit, div := "", 1.0
	switch {
	case a >= 1e9:
		unit, div = "G", 1e9
		if Lang(lang) == "en" {
			unit = "B"
		}
	case a >= 1e6:
		unit, div = "M", 1e6
	case a >= 1e4:
		unit, div = "k", 1e3
		if Lang(lang) == "en" {
			unit = "K"
		}
	}
	if unit == "" {
		return Auto(lang, v)
	}
	digits := 1
	if a/div >= 100 {
		digits = 0
	}
	s := trimZeros(lang, Decimal(lang, v/div, digits))
	if Lang(lang) == "en" {
		return s + unit
	}
	return s + NNBSP + unit
}

// Percent formats a value expressed in percent (3.2 means 3.2 %):
// "3,2 %" (fr, narrow no-break space) / "3.2%" (en).
func Percent(lang string, v float64, digits int) string {
	s := Decimal(lang, v, digits)
	if Lang(lang) == "en" {
		return s + "%"
	}
	return s + NNBSP + "%"
}

func sign(v float64, s string) string {
	if v > 0 && !strings.HasPrefix(s, Minus) && strings.Trim(s, "0,. %") != "" {
		return "+" + s
	}
	return s
}

// SignedPercent is Percent with an explicit "+" for positive values.
func SignedPercent(lang string, v float64, digits int) string {
	return sign(v, Percent(lang, v, digits))
}

// SignedNumber is Number with an explicit "+" for positive values.
func SignedNumber(lang string, v float64) string { return sign(v, Number(lang, v)) }

// SignedDecimal is Decimal with an explicit "+" for positive values.
func SignedDecimal(lang string, v float64, digits int) string {
	return sign(v, Decimal(lang, v, digits))
}

// Points formats a difference between two percentages, in percentage points:
// "+1,2 pt" (fr) / "+1.2 pts" (en).
func Points(lang string, v float64, digits int) string {
	s := SignedDecimal(lang, v, digits)
	if Lang(lang) == "en" {
		return s + NBSP + "pts"
	}
	if math.Abs(v) >= 2 {
		return s + NBSP + "pts"
	}
	return s + NBSP + "pt"
}

var currencySymbols = map[string]string{
	"EUR": "€", "USD": "$", "GBP": "£", "JPY": "¥", "CHF": "CHF", "CAD": "CA$",
	"AUD": "A$", "CNY": "¥", "INR": "₹", "BRL": "R$", "MAD": "MAD", "XOF": "FCFA",
	"SEK": "kr", "NOK": "kr", "DKK": "kr", "PLN": "zł",
}

// Currency formats an amount: "12 345 €" (fr) / "€12,345" (en). digits < 0
// selects 0 decimals for amounts >= 1000 and 2 otherwise.
func Currency(lang string, v float64, code string, digits int) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		code = "EUR"
	}
	if digits < 0 {
		digits = 2
		if math.Abs(v) >= 1000 || v == math.Trunc(v) {
			digits = 0
		}
	}
	sym, ok := currencySymbols[code]
	if !ok {
		sym = code
	}
	n := Decimal(lang, math.Abs(v), digits)
	neg := ""
	if v < 0 && strings.Trim(n, "0,. ") != "" {
		neg = Minus
	}
	if Lang(lang) == "en" && sym != code {
		return neg + sym + n
	}
	return neg + n + NBSP + sym
}

// Duration formats seconds: "1 min 32 s" (fr) / "1m 32s" (en); hours are
// added when needed.
func Duration(lang string, seconds float64) string {
	if math.IsNaN(seconds) || seconds < 0 {
		return "–"
	}
	total := int(math.Round(seconds))
	h, m, s := total/3600, (total%3600)/60, total%60
	en := Lang(lang) == "en"
	var parts []string
	if h > 0 {
		if en {
			parts = append(parts, fmt.Sprintf("%dh", h))
		} else {
			parts = append(parts, fmt.Sprintf("%d"+NBSP+"h", h))
		}
	}
	if m > 0 || h > 0 {
		if en {
			parts = append(parts, fmt.Sprintf("%dm", m))
		} else {
			parts = append(parts, fmt.Sprintf("%d"+NBSP+"min", m))
		}
	}
	if h == 0 {
		if en {
			parts = append(parts, fmt.Sprintf("%ds", s))
		} else {
			parts = append(parts, fmt.Sprintf("%d"+NBSP+"s", s))
		}
	}
	return strings.Join(parts, " ")
}

var (
	monthsFR = []string{"janv.", "févr.", "mars", "avr.", "mai", "juin", "juil.", "août", "sept.", "oct.", "nov.", "déc."}
	monthsEN = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
)

// ParseDate parses a YYYY-MM-DD date.
func ParseDate(iso string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(iso))
	return t, err == nil
}

func month(lang string, m time.Month) string {
	if Lang(lang) == "en" {
		return monthsEN[m-1]
	}
	return monthsFR[m-1]
}

// FormatTime formats a date: "1 juil. 2026" / "Jul 1, 2026".
func FormatTime(lang string, t time.Time) string {
	if Lang(lang) == "en" {
		return fmt.Sprintf("%s %d, %d", month(lang, t.Month()), t.Day(), t.Year())
	}
	return strconv.Itoa(t.Day()) + NBSP + month(lang, t.Month()) + " " + strconv.Itoa(t.Year())
}

// Date formats a YYYY-MM-DD date: "1 juil. 2026" / "Jul 1, 2026". Invalid
// input is returned unchanged.
func Date(lang, iso string) string {
	t, ok := ParseDate(iso)
	if !ok {
		return iso
	}
	return FormatTime(lang, t)
}

// ShortDate formats a YYYY-MM-DD date without the year: "1 juil." / "Jul 1".
func ShortDate(lang, iso string) string {
	t, ok := ParseDate(iso)
	if !ok {
		return iso
	}
	if Lang(lang) == "en" {
		return fmt.Sprintf("%s %d", month(lang, t.Month()), t.Day())
	}
	return strconv.Itoa(t.Day()) + NBSP + month(lang, t.Month())
}

// DateRange formats an inclusive period compactly:
// "1 juil. – 30 sept. 2026" / "Jul 1 – Sep 30, 2026".
func DateRange(lang, start, end string) string {
	s, ok1 := ParseDate(start)
	e, ok2 := ParseDate(end)
	switch {
	case !ok1 && !ok2:
		return strings.TrimSpace(start + " – " + end)
	case !ok1:
		return Date(lang, end)
	case !ok2:
		return Date(lang, start)
	}
	en := Lang(lang) == "en"
	if s.Year() != e.Year() {
		return FormatTime(lang, s) + " – " + FormatTime(lang, e)
	}
	if en {
		return fmt.Sprintf("%s %d – %s %d, %d", month(lang, s.Month()), s.Day(), month(lang, e.Month()), e.Day(), e.Year())
	}
	return strconv.Itoa(s.Day()) + NBSP + month(lang, s.Month()) + " – " + FormatTime(lang, e)
}

// Days returns the number of days of an inclusive period, or 0 if invalid.
func Days(start, end string) int {
	s, ok1 := ParseDate(start)
	e, ok2 := ParseDate(end)
	if !ok1 || !ok2 || e.Before(s) {
		return 0
	}
	return int(e.Sub(s).Hours()/24) + 1
}
