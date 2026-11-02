package insights

import (
	"math"
	"sort"
	"strings"

	"github.com/flocom/SEO-GEO-Report/internal/i18n"
	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// KPI keys of Google Business Profile.
const (
	KeyGBPViews         = "gbp.views"
	KeyGBPInteractions  = "gbp.interactions"
	KeyGBPCalls         = "gbp.calls"
	KeyGBPWebsiteClicks = "gbp.website_clicks"
	KeyGBPDirections    = "gbp.directions"
	KeyGBPRating        = "gbp.rating"
	KeyGBPReviews       = "gbp.reviews"
)

// GBPResponseTarget is the review reply rate (percent) under which the
// report recommends replying to more reviews.
const GBPResponseTarget = 80.0

// GBPViewShares returns the share of profile views coming from Google Maps
// and from mobile devices, in percent; ok is false without view splits.
func GBPViewShares(t model.GBPTotals) (maps, mobile float64, ok bool) {
	s := t.ViewSplits()
	val := func(m *model.Metric) float64 {
		if m == nil {
			return 0
		}
		return m.Current
	}
	sm, sd, mm, md := val(s[0]), val(s[1]), val(s[2]), val(s[3])
	total := sm + sd + mm + md
	if total <= 0 || (s[0] == nil && s[1] == nil) || (s[2] == nil && s[3] == nil) {
		// Without both Search and Maps figures a share would be misleading.
		return 0, 0, false
	}
	return (mm + md) / total * 100, (sm + mm) / total * 100, true
}

// GBPUnanswered counts the recent reviews without an owner reply.
func GBPUnanswered(rv *model.GBPReviews) int {
	if rv == nil {
		return 0
	}
	n := 0
	for _, r := range rv.Recent {
		if !r.Responded {
			n++
		}
	}
	return n
}

// SortGBPKeywords returns the keywords sorted by number of searches (exact
// figures before "<15" thresholds of the same value).
func SortGBPKeywords(rows []model.GBPKeyword) []model.GBPKeyword {
	out := append([]model.GBPKeyword(nil), rows...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Impressions != out[j].Impressions {
			return out[i].Impressions > out[j].Impressions
		}
		return !out[i].BelowThreshold && out[j].BelowThreshold
	})
	return out
}

func gbpKPIs(a *Analysis, bp *model.BusinessProfile) {
	t := bp.Totals
	for _, k := range []struct {
		key string
		m   *model.Metric
	}{
		{KeyGBPViews, t.Views}, {KeyGBPInteractions, t.Interactions}, {KeyGBPCalls, t.Calls},
		{KeyGBPWebsiteClicks, t.WebsiteClicks}, {KeyGBPDirections, t.DirectionRequests},
	} {
		if k.m != nil {
			a.KPIs[k.key] = kpiStatus(k.key, *k.m, false)
		}
	}
	if rv := bp.Reviews; rv != nil {
		if m := rv.AverageRating; m != nil && m.Current > 0 {
			st := kpiStatus(KeyGBPRating, *m, false)
			// A rating moves little: a change of 0.05 star or less is stable.
			if d, ok := m.Delta(); ok && math.Abs(d) <= 0.05 {
				st.Tone = Neutral
			}
			a.KPIs[KeyGBPRating] = st
		}
		if m := rv.TotalReviews; m != nil {
			a.KPIs[KeyGBPReviews] = kpiStatus(KeyGBPReviews, *m, false)
		}
	}
}

// gbpMain returns the headline metric of the profile for the progress index:
// interactions (what people did), else views.
func gbpMain(bp *model.BusinessProfile) *model.Metric {
	if bp == nil {
		return nil
	}
	if bp.Totals.Interactions != nil {
		return bp.Totals.Interactions
	}
	return bp.Totals.Views
}

// gbpActionNames are the localized action names used in sentences.
var gbpActionNames = map[string][2]string{
	"calls":              {"les appels", "calls"},
	"website_clicks":     {"les clics vers le site", "website clicks"},
	"direction_requests": {"les demandes d'itinéraire", "direction requests"},
	"messages":           {"les messages", "messages"},
	"bookings":           {"les réservations", "bookings"},
	"food_orders":        {"les commandes de repas", "food orders"},
	"menu_clicks":        {"les consultations du menu", "menu views"},
}

func gbpInsights(w *writer, bp *model.BusinessProfile) {
	const sec = "business_profile"
	t := bp.Totals

	// Views.
	if v := t.Views; v != nil {
		if pct, ok := v.DeltaPct(); ok {
			switch toneOf(*v, false) {
			case Positive:
				w.add(Positive, sec, 50+bump(pct)/2,
					"Votre fiche Google a été vue %s fois (%s) : plus de clients potentiels vous découvrent dans Google Search et Maps.",
					"Your Google profile was viewed %s times (%s): more potential customers find you in Google Search and Maps.",
					w.n(v.Current), w.spc(pct, v.Current, *v.Previous))
			case Negative:
				w.add(Negative, sec, 50+bump(pct)/2,
					"Votre fiche Google a été vue %s fois (%s par rapport à la période de comparaison).",
					"Your Google profile was viewed %s times (%s from the comparison period).",
					w.n(v.Current), w.spc(pct, v.Current, *v.Previous))
			default:
				w.add(Neutral, sec, 36,
					"Les vues de votre fiche Google sont stables : %s (%s).",
					"Views of your Google profile are stable: %s (%s).",
					w.n(v.Current), w.spc(pct, v.Current, *v.Previous))
			}
		} else if v.Current > 0 {
			w.add(Neutral, sec, 30,
				"Votre fiche Google a été vue %s fois sur la période.",
				"Your Google profile was viewed %s times over the period.",
				w.n(v.Current))
		}
	}

	// Interactions.
	if m := t.Interactions; m != nil {
		if pct, ok := m.DeltaPct(); ok && toneOf(*m, false) != Neutral {
			if toneOf(*m, false) == Positive {
				w.add(Positive, sec, 48+bump(pct)/2,
					"Les actions sur votre fiche Google (appels, itinéraires, clics vers le site…) progressent %s : %s contre %s.",
					"Actions on your Google profile (calls, directions, website clicks…) grew %s: %s vs %s.",
					w.by(pct, m.Current, *m.Previous), w.n(m.Current), w.n(*m.Previous))
			} else {
				w.add(Negative, sec, 48+bump(pct)/2,
					"Les actions sur votre fiche Google (appels, itinéraires, clics vers le site…) reculent %s : %s contre %s.",
					"Actions on your Google profile (calls, directions, website clicks…) fell %s: %s vs %s.",
					w.by(pct, m.Current, *m.Previous), w.n(m.Current), w.n(*m.Previous))
			}
		} else if !ok && m.Current > 0 && t.Views == nil {
			w.add(Neutral, sec, 30,
				"Votre fiche Google a généré %s actions (appels, itinéraires, clics vers le site…) sur la période.",
				"Your Google profile generated %s actions (calls, directions, website clicks…) over the period.",
				w.n(m.Current))
		}
	}

	// The action growing most (in relative terms, with a meaningful gain).
	var best model.GBPAction
	bestPct := 0.0
	actions := t.Actions()
	for _, a := range actions {
		d, ok := a.Metric.Delta()
		pct, okPct := a.Metric.DeltaPct()
		if ok && okPct && d >= 5 && pct >= neutralPct && pct > bestPct {
			best, bestPct = a, pct
		}
	}
	if best.Metric != nil && len(actions) >= 2 {
		name := gbpActionNames[best.Key]
		fr, en := name[0], name[1]
		w.add(Positive, sec, 34+bump(bestPct)/3,
			"Sur la fiche Google, ce sont %s qui progressent le plus : %s (%s).",
			"On the Google profile, %s grow the most: %s (%s).",
			w.pick(fr, en), w.n(best.Metric.Current), w.spc(bestPct, best.Metric.Current, *best.Metric.Previous))
	}

	// Where people found the profile.
	if maps, mobile, ok := GBPViewShares(t); ok {
		if maps >= 50 {
			w.add(Neutral, sec, 30,
				"%s des vues de la fiche viennent de Google Maps : les clients vous cherchent sur la carte, l'adresse, les horaires et les photos doivent être impeccables.",
				"%s of the profile views come from Google Maps: customers look for you on the map, so the address, opening hours and photos must be spot on.",
				w.p(maps))
		} else {
			w.add(Neutral, sec, 28,
				"%s des vues de la fiche viennent de la recherche Google et %s de Google Maps.",
				"%s of the profile views come from Google Search and %s from Google Maps.",
				w.p(100-maps), w.p(maps))
		}
		if mobile >= 60 {
			w.add(Neutral, sec, 24,
				"%s des vues de la fiche se font sur smartphone : le bouton d'appel et l'itinéraire sont souvent le premier contact.",
				"%s of the profile views happen on a smartphone: the call button and directions are often the first contact.",
				w.p(mobile))
		}
	}

	// Reviews.
	if rv := bp.Reviews; rv != nil {
		reviewInsights(w, rv)
	}

	// Top local search term.
	if kws := SortGBPKeywords(bp.SearchKeywords); len(kws) > 0 && strings.TrimSpace(kws[0].Keyword) != "" && kws[0].Impressions > 0 {
		k := kws[0]
		count := w.n(k.Impressions)
		if k.BelowThreshold {
			count = "< " + count
		}
		w.add(Neutral, sec, 27,
			"Recherche locale qui affiche le plus votre fiche : %s (%s recherches).",
			"Local search that shows your profile most: %s (%s searches).",
			w.q(k.Keyword), count)
	}

	// Profile completeness gaps.
	var missing []string
	for _, c := range bp.Checklist {
		if !c.Done && strings.TrimSpace(c.Item) != "" {
			missing = append(missing, w.q(i18n.GBPChecklistLabel(w.lang, c.Item)))
		}
	}
	if len(missing) > 0 {
		list := strings.Join(missing[:min(len(missing), 4)], ", ")
		if len(missing) > 4 {
			list += "…"
		}
		weight := 36 + math.Min(float64(len(missing)), 5)*2
		w.add(Negative, sec, weight,
			"Fiche Google incomplète, à compléter en priorité : %s. Une fiche complète inspire confiance et ressort mieux dans Google Maps.",
			"Incomplete Google profile, to complete first: %s. A complete profile builds trust and ranks better in Google Maps.",
			list)
	} else if m := bp.ProfileCompleteness; m != nil && m.Current >= 95 {
		w.add(Positive, sec, 22,
			"Votre fiche Google est complète (%s) : horaires, description, photos et services sont renseignés.",
			"Your Google profile is complete (%s): hours, description, photos and services are filled in.",
			w.p(m.Current))
	}
}

func reviewInsights(w *writer, rv *model.GBPReviews) {
	const sec = "business_profile"
	if m := rv.AverageRating; m != nil && m.Current > 0 {
		reviews := ""
		if rv.TotalReviews != nil && rv.TotalReviews.Current > 0 {
			reviews = w.plural2(rv.TotalReviews.Current, "avis", "avis", "review", "reviews")
		}
		d, ok := m.Delta()
		switch {
		case ok && d >= 0.1:
			w.add(Positive, sec, 40,
				"La note moyenne de vos avis Google monte à %s/5 contre %s%s.",
				"Your average Google rating rises to %s/5 from %s%s.",
				w.pos(m.Current), w.pos(*m.Previous), w.over(reviews))
		case ok && d <= -0.1:
			w.add(Negative, sec, 46,
				"La note moyenne de vos avis Google baisse à %s/5 contre %s%s : lisez les avis récents pour comprendre ce qui déçoit.",
				"Your average Google rating drops to %s/5 from %s%s: read the recent reviews to understand what disappoints.",
				w.pos(m.Current), w.pos(*m.Previous), w.over(reviews))
		case m.Current >= 4.5:
			w.add(Positive, sec, 30,
				"Vos clients vous notent %s/5 sur Google%s : un vrai atout face aux concurrents dans Google Maps.",
				"Customers rate you %s/5 on Google%s: a real asset against competitors in Google Maps.",
				w.pos(m.Current), w.paren(reviews))
		case m.Current < 4:
			w.add(Negative, sec, 42,
				"Votre note Google (%s/5)%s est en dessous de 4 : beaucoup de clients filtrent les établissements mieux notés.",
				"Your Google rating (%s/5)%s is below 4: many customers filter for better-rated businesses.",
				w.pos(m.Current), w.paren(reviews))
		}
	}
	if m := rv.NewReviews; m != nil && m.Current > 0 {
		if pct, ok := m.DeltaPct(); ok && toneOf(*m, false) == Positive {
			w.add(Positive, sec, 30+bump(pct)/4,
				"%s sur la période (%s) : continuez à demander un avis aux clients satisfaits.",
				"%s over the period (%s): keep asking happy customers for a review.",
				w.cap(w.plural2(m.Current, "nouvel avis", "nouveaux avis", "new review", "new reviews")), w.spc(pct, m.Current, *m.Previous))
		} else if ok && toneOf(*m, false) == Negative {
			w.add(Neutral, sec, 30,
				"Moins de nouveaux avis que sur la période de comparaison : %s (%s). Pensez à solliciter vos clients après chaque visite.",
				"Fewer new reviews than in the comparison period: %s (%s). Remember to ask customers after each visit.",
				w.n(m.Current), w.spc(pct, m.Current, *m.Previous))
		}
	}
	unanswered := GBPUnanswered(rv)
	switch rr := rv.ResponseRate; {
	case rr != nil && rr.Current < GBPResponseTarget:
		w.add(Negative, sec, 44,
			"Seuls %s des avis reçoivent une réponse : répondez à chaque avis, surtout aux plus critiques, pour rassurer les futurs clients.",
			"Only %s of reviews get a reply: answer every review, especially the critical ones, to reassure future customers.",
			w.p(rr.Current))
	case rr == nil && unanswered > 0:
		w.add(Negative, sec, 40,
			"%s sans réponse parmi les plus récents : répondre à chaque avis montre que vous êtes à l'écoute.",
			"%s without a reply among the most recent ones: answering every review shows you listen.",
			w.cap(w.plural2(float64(unanswered), "avis", "avis", "review", "reviews")))
	case rr != nil && rr.Current >= 95:
		w.add(Positive, sec, 24,
			"Vous répondez à %s des avis : une excellente habitude pour la confiance des clients.",
			"You reply to %s of reviews: an excellent habit for customer trust.",
			w.p(rr.Current))
	}
}

// pick returns the French or English variant.
func (w *writer) pick(fr, en string) string {
	if w.lang == "en" {
		return en
	}
	return fr
}

// over introduces a number of reviews: " sur 187 avis" / " across 187
// reviews".
func (w *writer) over(s string) string {
	if s == "" {
		return ""
	}
	return w.pick(" sur ", " across ") + s
}

// paren wraps a non-empty string as " (s)".
func (w *writer) paren(s string) string {
	if s == "" {
		return ""
	}
	return " (" + s + ")"
}

// cap upper-cases the first letter.
func (w *writer) cap(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}
