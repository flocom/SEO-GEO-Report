package insights

import (
	"math"
	"sort"
	"strings"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// BingShare returns Bing's share of the search clicks (Google + Bing), in
// percent, with the previous share when both engines have comparison values.
// It is nil when the report lacks Google or Bing clicks.
func BingShare(r *model.Report) *model.Metric {
	if r == nil || r.SearchConsole == nil || r.Bing == nil {
		return nil
	}
	g, b := r.SearchConsole.Totals.Clicks, r.Bing.Totals.Clicks
	if g.Current <= 0 || b.Current <= 0 {
		return nil // without clicks on both engines a share is meaningless (missing data)
	}
	m := model.Metric{Current: round(b.Current/(g.Current+b.Current)*100, 2)}
	if g.Previous != nil && b.Previous != nil && *g.Previous+*b.Previous > 0 {
		m.Previous = model.F(round(*b.Previous/(*g.Previous+*b.Previous)*100, 2))
	}
	return &m
}

func bingKPIs(a *Analysis, bg *model.Bing) {
	t := bg.Totals
	if t.Clicks.Current > 0 || t.Impressions.Current > 0 || t.Clicks.Previous != nil {
		a.KPIs[KeyBingClicks] = kpiStatus(KeyBingClicks, t.Clicks, false)
		a.KPIs[KeyBingImpressions] = kpiStatus(KeyBingImpressions, t.Impressions, false)
		if t.CTR != nil {
			a.KPIs[KeyBingCTR] = kpiStatus(KeyBingCTR, *t.CTR, false)
		}
		if t.Position != nil && t.Position.Current > 0 {
			a.KPIs[KeyBingPosition] = kpiStatus(KeyBingPosition, *t.Position, true)
		}
	}
	if ai := bg.AIPerformance; ai != nil {
		a.KPIs[KeyBingAICitations] = kpiStatus(KeyBingAICitations, ai.Citations, false)
	}
}

func bingInsights(w *writer, r *model.Report) {
	const sec = "bing"
	bg := r.Bing
	t := bg.Totals

	// Clicks. Weights stay below the Google ones: Bing is a secondary engine.
	c := t.Clicks
	if pct, ok := c.DeltaPct(); ok {
		switch toneOf(c, false) {
		case Positive:
			w.add(Positive, sec, 50+bump(pct)/2,
				"Les clics depuis Bing progressent de %s : %s contre %s sur la période de comparaison.",
				"Clicks from Bing grew by %s: %s vs %s in the comparison period.",
				w.p(pct), w.n(c.Current), w.n(*c.Previous))
		case Negative:
			w.add(Negative, sec, 50+bump(pct)/2,
				"Les clics depuis Bing reculent de %s : %s contre %s sur la période de comparaison.",
				"Clicks from Bing fell by %s: %s vs %s in the comparison period.",
				w.p(pct), w.n(c.Current), w.n(*c.Previous))
		default:
			w.add(Neutral, sec, 38,
				"Les clics depuis Bing sont stables : %s (%s).",
				"Clicks from Bing are stable: %s (%s).",
				w.n(c.Current), w.sp(pct))
		}
	} else if c.Current > 0 {
		w.add(Neutral, sec, 32,
			"Le site a reçu %s clics depuis Bing sur la période.",
			"The site received %s clicks from Bing over the period.",
			w.n(c.Current))
	}

	// Share of Bing in search clicks.
	if s := BingShare(r); s != nil {
		if s.Previous != nil && math.Abs(s.Current-*s.Previous) >= 0.5 {
			cur, prev := w.p2(s.Current, *s.Previous)
			tone, fr, en := Positive, "Bing pèse davantage dans vos clics de recherche : %s des clics Google + Bing, contre %s auparavant.",
				"Bing weighs more in your search clicks: %s of Google + Bing clicks, up from %s."
			if s.Current < *s.Previous {
				tone, fr, en = Neutral, "Bing pèse moins dans vos clics de recherche : %s des clics Google + Bing, contre %s auparavant.",
					"Bing weighs less in your search clicks: %s of Google + Bing clicks, down from %s."
			}
			w.add(tone, sec, 36, fr, en, cur, prev)
		} else {
			w.add(Neutral, sec, 34,
				"Bing représente %s des clics de recherche (Google + Bing).",
				"Bing accounts for %s of search clicks (Google + Bing).",
				w.p(s.Current))
		}
	}

	// Average position.
	if p := t.Position; p != nil && p.Previous != nil && p.Current > 0 && *p.Previous > 0 {
		switch toneOf(*p, true) {
		case Positive:
			w.add(Positive, sec, 33+math.Min(math.Abs(p.Current-*p.Previous), 10),
				"La position moyenne dans Bing s'améliore, de %s à %s.",
				"Average position in Bing improved from %s to %s.",
				w.pos(*p.Previous), w.pos(p.Current))
		case Negative:
			w.add(Negative, sec, 33+math.Min(math.Abs(p.Current-*p.Previous), 10),
				"La position moyenne dans Bing recule, de %s à %s.",
				"Average position in Bing slipped from %s to %s.",
				w.pos(*p.Previous), w.pos(p.Current))
		}
	}

	// Rising Bing keyword.
	var best model.GSCRow
	bestGain := 0.0
	for _, q := range bg.Queries {
		if q.PrevClicks != nil && q.Clicks-*q.PrevClicks > bestGain {
			best, bestGain = q, q.Clicks-*q.PrevClicks
		}
	}
	if bestGain >= 10 {
		w.add(Positive, sec, 30,
			"Sur Bing, la requête %s gagne le plus de clics (%s).",
			"On Bing, the query %s gains the most clicks (%s).",
			w.q(best.Key), w.sn(bestGain))
	}

	ai := bg.AIPerformance
	if ai == nil {
		return
	}
	cit := ai.Citations
	if pct, ok := cit.DeltaPct(); ok {
		switch toneOf(cit, false) {
		case Positive:
			w.add(Positive, sec, 58+bump(pct)/2,
				"Le site est davantage cité comme source par Copilot et les réponses IA de Bing : %s citations (%s).",
				"The site is cited more often as a source by Copilot and Bing AI answers: %s citations (%s).",
				w.n(cit.Current), w.sp(pct))
		case Negative:
			w.add(Negative, sec, 55+bump(pct)/2,
				"Le site est moins cité comme source par Copilot et les réponses IA de Bing : %s citations (%s).",
				"The site is cited less often as a source by Copilot and Bing AI answers: %s citations (%s).",
				w.n(cit.Current), w.sp(pct))
		default:
			w.add(Neutral, sec, 40,
				"Les citations du site dans Copilot et les réponses IA de Bing sont stables : %s.",
				"Citations of the site in Copilot and Bing AI answers are stable: %s.",
				w.n(cit.Current))
		}
	} else if cit.Current > 0 {
		w.add(Neutral, sec, 45,
			"Le site a été cité %s fois comme source par Copilot et les réponses IA de Bing.",
			"The site was cited %s times as a source by Copilot and Bing AI answers.",
			w.n(cit.Current))
	}

	if len(ai.GroundingQueries) > 0 {
		qs := append([]model.BingGroundingQuery(nil), ai.GroundingQueries...)
		sort.SliceStable(qs, func(i, j int) bool { return qs[i].Citations > qs[j].Citations })
		if top := qs[0]; top.Citations > 0 && strings.TrimSpace(top.Query) != "" {
			w.add(Neutral, sec, 28,
				"La requête d'ancrage qui vous vaut le plus de citations IA est %s (%s citations).",
				"The grounding query that earns you the most AI citations is %s (%s citations).",
				w.q(top.Query), w.n(top.Citations))
		}
	}
}
