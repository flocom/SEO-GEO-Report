package insights

import (
	"math"
	"strconv"
	"strings"
)

// Typographic spaces used by the French formatting.
const (
	nbsp       = "\u00a0" // no-break space (before ':' and inside « »)
	narrowNbsp = "\u202f" // narrow no-break space (thousands, before %, ; ! ?)
)

// NormLang returns "en" for English language codes and "fr" otherwise, the
// same rule as model.Report.Lang.
func NormLang(lang string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(lang)), "en") {
		return "en"
	}
	return "fr"
}

// FormatNumber formats v with a fixed number of decimals following the
// conventions of lang: French uses a narrow no-break space as thousands
// separator and a decimal comma (12 345,6), English uses 12,345.6.
func FormatNumber(v float64, decimals int, lang string) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "–"
	}
	if decimals < 0 {
		decimals = 0
	}
	// Round half away from zero first (strconv rounds half to even).
	s := strconv.FormatFloat(round(math.Abs(v), decimals), 'f', decimals, 64)
	intPart, frac, _ := strings.Cut(s, ".")
	sep, dec := ",", "."
	if NormLang(lang) == "fr" {
		sep, dec = narrowNbsp, ","
	}
	var b strings.Builder
	n := len(intPart)
	for i, c := range intPart {
		if i > 0 && (n-i)%3 == 0 {
			b.WriteString(sep)
		}
		b.WriteRune(c)
	}
	out := b.String()
	if frac != "" {
		out += dec + frac
	}
	if v < 0 && strings.Trim(s, "0.") != "" {
		out = "-" + out
	}
	return out
}

// FormatAuto formats v with "smart" precision: no decimals from 100 upward,
// one decimal below (a trailing ",0"/".0" is dropped).
func FormatAuto(v float64, lang string) string {
	if math.Abs(v) >= 100 {
		return FormatNumber(v, 0, lang)
	}
	return trimZeroDecimal(FormatNumber(v, 1, lang))
}

// FormatPercent formats a percentage (23.4 → "23 %" in French, "23%" in
// English). Values below 10 keep one decimal ("3,2 %").
func FormatPercent(v float64, lang string) string {
	return percentNumber(v, lang) + percentSign(lang)
}

// FormatSignedPercent formats a relative change with an explicit sign:
// "+23 %" / "-4,5 %" in French, "+23%" / "-4.5%" in English.
func FormatSignedPercent(v float64, lang string) string {
	s := FormatPercent(v, lang)
	if !strings.HasPrefix(s, "-") && percentNumber(v, lang) != "0" {
		s = "+" + s
	}
	return s
}

// FormatSigned formats a number with an explicit sign ("+340", "-12,5").
func FormatSigned(v float64, decimals int, lang string) string {
	s := FormatNumber(v, decimals, lang)
	if v > 0 && strings.Trim(s, "0.,") != "" {
		s = "+" + s
	}
	return s
}

// FormatPoints formats a difference of percentages in points: "+6 pts" in
// French and English.
func FormatPoints(v float64, lang string) string {
	var s string
	if math.Abs(v) >= 10 {
		s = FormatNumber(v, 0, lang)
	} else {
		s = trimZeroDecimal(FormatNumber(v, 1, lang))
	}
	if v > 0 && s != "0" {
		s = "+" + s
	}
	sp := " "
	if NormLang(lang) == "fr" {
		sp = nbsp
	}
	unit := "pts"
	if math.Abs(v) < 1.05 {
		unit = "pt"
	}
	return s + sp + unit
}

func percentNumber(v float64, lang string) string {
	if math.Abs(v) >= 10 {
		return FormatNumber(v, 0, lang)
	}
	return trimZeroDecimal(FormatNumber(v, 1, lang))
}

func percentSign(lang string) string {
	if NormLang(lang) == "fr" {
		return narrowNbsp + "%"
	}
	return "%"
}

func trimZeroDecimal(s string) string {
	if strings.HasSuffix(s, ",0") || strings.HasSuffix(s, ".0") {
		s = s[:len(s)-2]
	}
	if s == "-0" {
		s = "0"
	}
	return s
}

// frTypography applies French spacing rules to a sentence written with plain
// spaces: no-break space before ':' and inside guillemets, narrow no-break
// space before ';', '!' and '?'.
func frTypography(s string) string {
	r := strings.NewReplacer(
		" :", nbsp+":",
		" ;", narrowNbsp+";",
		" !", narrowNbsp+"!",
		" ?", narrowNbsp+"?",
		"« ", "«"+nbsp,
		" »", nbsp+"»",
	)
	return r.Replace(s)
}

// ShortURL returns the path of a URL ("https://www.site.fr/a/b" → "/a/b"),
// or the input unchanged when it is not an absolute URL.
func ShortURL(u string) string {
	s := strings.TrimSpace(u)
	for _, p := range []string{"https://", "http://"} {
		if strings.HasPrefix(strings.ToLower(s), p) {
			s = s[len(p):]
			if i := strings.IndexByte(s, '/'); i >= 0 {
				return s[i:]
			}
			return "/"
		}
	}
	return s
}

func round(v float64, decimals int) float64 {
	p := math.Pow(10, float64(decimals))
	return math.Round(v*p) / p
}
