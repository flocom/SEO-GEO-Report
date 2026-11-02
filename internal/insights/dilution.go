package insights

import "github.com/flocom/SEO-GEO-Report/internal/model"

// Dilution: when a site suddenly appears on many new searches (a new site, new
// pages, a new topic), its impressions jump. Those new appearances usually
// rank lower at first, so the average position and the click-through rate
// drop mechanically even though the site gains visibility and clicks. Showing
// these two figures in red next to a "strong progress" verdict reads as a
// contradiction to a beginner: they are qualified as "expected" instead.

// dilutionMinImpressions is the impressions growth (percent) above which a
// worse position / CTR is considered the expected dilution effect.
const dilutionMinImpressions = 50.0

// isDilution reports whether a search engine's figures show the dilution
// effect: impressions grew by more than 50 %, clicks did not fall, and, when
// the number of ranking queries is known for both periods, it grew too.
func isDilution(t model.GSCTotals, pd *model.PositionDistribution) bool {
	pct, ok := t.Impressions.DeltaPct()
	if !ok || pct < dilutionMinImpressions {
		return false
	}
	if toneOf(t.Clicks, false) == Negative {
		return false
	}
	if pd != nil {
		ms := []model.Metric{pd.Top3, pd.Top10, pd.Top20, pd.Beyond20}
		var cur, prev float64
		for _, m := range ms {
			if m.Previous == nil {
				return true // unknown: impressions alone decide
			}
			cur += m.Current
			prev += *m.Previous
		}
		if prev > 0 && cur <= prev {
			return false
		}
	}
	return true
}

// dilutionNotes are the one-line explanations shown on the KPI cards.
var dilutionNotes = map[string][2]string{
	"position": {
		"Normal : le site apparaît sur beaucoup de nouvelles recherches, souvent plus bas au début.",
		"Expected: the site now shows up on many new searches, often lower down at first.",
	},
	"ctr": {
		"Normal : beaucoup plus d'affichages, dont de nouveaux plus bas dans les résultats, donc une part de clics plus faible.",
		"Expected: many more appearances, including new ones lower in the results, so a smaller share of clicks.",
	},
}

// DilutionNote returns the localized explanation of an expected decline for
// "position" or "ctr" ("" for other kinds).
func DilutionNote(lang, kind string) string {
	n, ok := dilutionNotes[kind]
	if !ok {
		return ""
	}
	if NormLang(lang) == "en" {
		return n[1]
	}
	return frTypography(n[0])
}

// markExpected turns the negative CTR / position statuses of one engine into
// expected (neutral) ones when its figures show the dilution effect. It
// returns true when the effect applies.
func markExpected(a *Analysis, lang string, t model.GSCTotals, pd *model.PositionDistribution, ctrKey, posKey string) bool {
	if !isDilution(t, pd) {
		return false
	}
	applied := false
	for kind, key := range map[string]string{"ctr": ctrKey, "position": posKey} {
		st, ok := a.KPIs[key]
		if !ok || st.Tone != Negative {
			continue
		}
		st.Tone, st.Expected, st.Note = Neutral, true, DilutionNote(lang, kind)
		a.KPIs[key] = st
		applied = true
	}
	return applied
}

// Expected reports whether the KPI key is an expected decline (dilution).
func (a Analysis) Expected(key string) bool {
	st, ok := a.KPIs[key]
	return ok && st.Expected
}
