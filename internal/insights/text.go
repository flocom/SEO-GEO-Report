package insights

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// writer accumulates localized insights.
type writer struct {
	lang     string
	currency string
	out      []Insight
}

// add appends an insight. fr and en are fmt formats; args must already be
// formatted for w.lang (numbers, quotes). French sentences are written with
// plain spaces and fixed by frTypography.
func (w *writer) add(tone Tone, section string, weight float64, fr, en string, args ...any) {
	var s string
	if w.lang == "en" {
		s = fmt.Sprintf(en, args...)
	} else {
		s = frTypography(fmt.Sprintf(fr, args...))
	}
	w.out = append(w.out, Insight{Tone: tone, Section: section, Text: s, Weight: round(weight, 2)})
}

func (w *writer) n(v float64) string   { return FormatNumber(v, 0, w.lang) }
func (w *writer) p(v float64) string   { return FormatPercent(math.Abs(v), w.lang) }
func (w *writer) sp(v float64) string  { return FormatSignedPercent(v, w.lang) }
func (w *writer) sn(v float64) string  { return FormatSigned(v, 0, w.lang) }
func (w *writer) pts(v float64) string { return FormatPoints(v, w.lang) }
func (w *writer) pos(v float64) string { return FormatNumber(v, 1, w.lang) }

// p2 formats two percentages being compared with the same precision
// ("2,9 % contre 3,0 %").
func (w *writer) p2(a, b float64) (string, string) {
	if math.Abs(a) >= 10 && math.Abs(b) >= 10 {
		return w.p(a), w.p(b)
	}
	sign := percentSign(w.lang)
	return FormatNumber(math.Abs(a), 1, w.lang) + sign, FormatNumber(math.Abs(b), 1, w.lang) + sign
}

func (w *writer) q(s string) string {
	s = strings.TrimSpace(s)
	if w.lang == "en" {
		return "“" + s + "”"
	}
	return "« " + s + " »"
}

func (w *writer) money(v float64) string {
	sym := map[string]string{"EUR": "€", "USD": "$", "GBP": "£", "CHF": "CHF", "CAD": "CA$", "JPY": "¥"}
	cur := strings.ToUpper(strings.TrimSpace(w.currency))
	if cur == "" {
		cur = "EUR"
	}
	s, ok := sym[cur]
	if !ok {
		s = cur
	}
	num := w.n(v)
	if w.lang == "en" && (s == "€" || s == "$" || s == "£" || s == "¥" || s == "CA$") {
		return s + num
	}
	return num + nbsp + s
}

func (w *writer) ordinal(n int) string {
	if w.lang == "en" {
		suf := "th"
		if n%100 < 11 || n%100 > 13 {
			switch n % 10 {
			case 1:
				suf = "st"
			case 2:
				suf = "nd"
			case 3:
				suf = "rd"
			}
		}
		return fmt.Sprintf("%d%s", n, suf)
	}
	if n == 1 {
		return "1er"
	}
	return fmt.Sprintf("%de", n)
}

// pa turns a pair of formatted values into fmt arguments.
func pa(a, b string) []any { return []any{a, b} }

// bump increases an insight weight with the size of a relative change
// (at most +10 for a change of 100 % or more).
func bump(pct float64) float64 { return math.Min(math.Abs(pct), 100) / 10 }

func buildInsights(r *model.Report, a *Analysis) []Insight {
	w := &writer{lang: r.Lang(), currency: r.Meta.Currency}
	if sc := r.SearchConsole; sc != nil {
		gscInsights(w, sc, a)
	}
	if ga := r.Analytics; ga != nil {
		ga4Insights(w, ga)
	}
	geoInsights(w, r, a)

	return selectInsights(w.out, maxInsights)
}

// selectInsights keeps at most max insights, sorted by weight desc, while
// keeping the report balanced: the best insight of every section is kept
// first, then at most 3 insights per section, then the remaining ones.
func selectInsights(all []Insight, max int) []Insight {
	sort.SliceStable(all, func(i, j int) bool { return all[i].Weight > all[j].Weight })
	if len(all) <= max {
		if all == nil {
			return []Insight{}
		}
		return all
	}
	taken := make([]bool, len(all))
	perSection := map[string]int{}
	n := 0
	pass := func(limit int) {
		for i, in := range all {
			if n >= max {
				return
			}
			if !taken[i] && perSection[in.Section] < limit {
				taken[i] = true
				perSection[in.Section]++
				n++
			}
		}
	}
	pass(1)
	pass(3)
	pass(len(all))
	out := make([]Insight, 0, max)
	for i, in := range all {
		if taken[i] {
			out = append(out, in)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Search Console

func gscInsights(w *writer, sc *model.SearchConsole, a *Analysis) {
	const sec = "search_console"
	t := sc.Totals

	// Clicks.
	c := t.Clicks
	if pct, ok := c.DeltaPct(); ok {
		switch toneOf(c, false) {
		case Positive:
			w.add(Positive, sec, 80+bump(pct),
				"Les clics depuis Google progressent de %s : %s contre %s sur la période de comparaison.",
				"Clicks from Google grew by %s: %s vs %s in the comparison period.",
				w.p(pct), w.n(c.Current), w.n(*c.Previous))
		case Negative:
			w.add(Negative, sec, 80+bump(pct),
				"Les clics depuis Google reculent de %s : %s contre %s sur la période de comparaison.",
				"Clicks from Google fell by %s: %s vs %s in the comparison period.",
				w.p(pct), w.n(c.Current), w.n(*c.Previous))
		default:
			w.add(Neutral, sec, 60,
				"Les clics depuis Google sont stables : %s (%s).",
				"Clicks from Google are stable: %s (%s).",
				w.n(c.Current), w.sp(pct))
		}
	} else if c.Current > 0 {
		w.add(Neutral, sec, 40,
			"Le site a reçu %s clics depuis Google sur la période.",
			"The site received %s clicks from Google over the period.",
			w.n(c.Current))
	}

	// Impressions.
	imp := t.Impressions
	impPct, impOK := imp.DeltaPct()
	if impOK {
		switch toneOf(imp, false) {
		case Positive:
			w.add(Positive, sec, 60+bump(impPct),
				"La visibilité dans Google augmente : %s impressions (%s).",
				"Visibility in Google is growing: %s impressions (%s).",
				w.n(imp.Current), w.sp(impPct))
		case Negative:
			w.add(Negative, sec, 60+bump(impPct),
				"Les impressions dans Google baissent de %s (%s).",
				"Impressions in Google dropped by %s (%s).",
				w.p(impPct), w.n(imp.Current))
		}
	}

	// CTR.
	if ctr := gscCTR(sc); ctr != nil && ctr.Previous != nil {
		ctrPct, ok := ctr.DeltaPct()
		switch {
		case ok && impOK && impPct > 5 && ctrPct < -5:
			w.add(Negative, sec, 74,
				"Le taux de clic baisse (%s contre %s) alors que les impressions augmentent : retravaillez les titres et meta descriptions des pages les plus vues pour transformer cette visibilité en visites.",
				"Click-through rate is dropping (%s vs %s) while impressions grow: rewrite the titles and meta descriptions of your most viewed pages to turn this visibility into visits.",
				pa(w.p2(ctr.Current, *ctr.Previous))...)
		case ok && toneOf(*ctr, false) == Positive && ctrPct >= 5:
			w.add(Positive, sec, 45+bump(ctrPct),
				"Le taux de clic s'améliore : %s contre %s, les résultats du site attirent davantage.",
				"Click-through rate improved: %s vs %s, the site's results are more attractive.",
				pa(w.p2(ctr.Current, *ctr.Previous))...)
		case ok && ctrPct <= -5:
			w.add(Negative, sec, 50,
				"Le taux de clic recule : %s contre %s.",
				"Click-through rate declined: %s vs %s.",
				pa(w.p2(ctr.Current, *ctr.Previous))...)
		}
	}

	// Average position.
	if p := t.Position; p != nil && p.Previous != nil && p.Current > 0 && *p.Previous > 0 {
		d := p.Current - *p.Previous
		switch toneOf(*p, true) {
		case Positive:
			w.add(Positive, sec, 55+math.Min(math.Abs(d), 10),
				"La position moyenne s'améliore, de %s à %s (plus le chiffre est bas, mieux c'est).",
				"Average position improved from %s to %s (lower is better).",
				w.pos(*p.Previous), w.pos(p.Current))
		case Negative:
			w.add(Negative, sec, 50+math.Min(math.Abs(d), 10),
				"La position moyenne recule, de %s à %s (plus le chiffre est bas, mieux c'est).",
				"Average position slipped from %s to %s (lower is better).",
				w.pos(*p.Previous), w.pos(p.Current))
		}
	}

	queryInsights(w, sc, a)
	pageInsights(w, sc)
	deviceInsights(w, sc)
	technicalInsights(w, sc)
}

func queryInsights(w *writer, sc *model.SearchConsole, a *Analysis) {
	const sec = "queries"
	if len(a.Winners) > 0 {
		q := a.Winners[0]
		d := q.Clicks - *q.PrevClicks
		var change string
		if *q.PrevClicks > 0 {
			change = w.sp(d / *q.PrevClicks * 100)
		} else if w.lang == "en" {
			change = "new"
		} else {
			change = "nouvelle"
		}
		w.add(Positive, sec, 64,
			"Meilleure progression : la requête %s gagne %s clics (%s).",
			"Top gainer: the query %s gained %s clicks (%s).",
			w.q(q.Key), w.n(d), change)
	}
	if len(a.Losers) > 0 {
		q := a.Losers[0]
		d := q.Clicks - *q.PrevClicks
		if d <= -10 {
			w.add(Negative, sec, 48,
				"Plus forte baisse : %s perd %s clics (%s) ; vérifiez la page qui se positionne sur cette requête.",
				"Largest drop: %s lost %s clicks (%s); check the page ranking for this query.",
				w.q(q.Key), w.n(-d), w.sp(d / *q.PrevClicks * 100))
		}
	}

	// Queries entering the top 3 / the first page.
	var top3, top10 []model.GSCRow
	for _, q := range sc.Queries {
		if q.PrevPosition == nil || q.Position <= 0 {
			continue
		}
		switch {
		case q.Position <= 3 && *q.PrevPosition > 3:
			top3 = append(top3, q)
		case q.Position <= 10 && *q.PrevPosition > 10:
			top10 = append(top10, q)
		}
	}
	byImpr := func(rows []model.GSCRow) {
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].Impressions > rows[j].Impressions })
	}
	byImpr(top3)
	byImpr(top10)
	if len(top3) == 1 {
		q := top3[0]
		w.add(Positive, sec, 63,
			"La requête %s entre dans le top 3 de Google (position %s, contre %s).",
			"The query %s entered Google's top 3 (position %s, up from %s).",
			w.q(q.Key), w.pos(q.Position), w.pos(*q.PrevPosition))
	} else if len(top3) > 1 {
		q := top3[0]
		w.add(Positive, sec, 62+math.Min(float64(len(top3)), 8),
			"%s requêtes entrent dans le top 3 de Google, dont %s (position %s, contre %s).",
			"%s queries entered Google's top 3, including %s (position %s, up from %s).",
			w.n(float64(len(top3))), w.q(q.Key), w.pos(q.Position), w.pos(*q.PrevPosition))
	}
	if len(top10) == 1 {
		q := top10[0]
		w.add(Positive, sec, 56,
			"La requête %s arrive en première page de Google (position %s, contre %s).",
			"The query %s reached Google's first page (position %s, up from %s).",
			w.q(q.Key), w.pos(q.Position), w.pos(*q.PrevPosition))
	} else if len(top10) > 1 {
		q := top10[0]
		w.add(Positive, sec, 55+math.Min(float64(len(top10)), 8),
			"%s requêtes arrivent en première page de Google, dont %s (position %s, contre %s).",
			"%s queries reached Google's first page, including %s (position %s, up from %s).",
			w.n(float64(len(top10))), w.q(q.Key), w.pos(q.Position), w.pos(*q.PrevPosition))
	}
	if len(top3) == 0 && len(top10) == 0 && a.Positions != nil && a.Positions.Top3.Previous != nil {
		t3 := a.Positions.Top3
		if t3.Current > *t3.Previous {
			w.add(Positive, sec, 40,
				"Le nombre de requêtes dans le top 3 passe de %s à %s.",
				"Queries ranking in the top 3 went from %s to %s.",
				w.n(*t3.Previous), w.n(t3.Current))
		} else if t3.Current < *t3.Previous {
			w.add(Negative, sec, 40,
				"Le nombre de requêtes dans le top 3 passe de %s à %s.",
				"Queries ranking in the top 3 went from %s to %s.",
				w.n(*t3.Previous), w.n(t3.Current))
		}
	}

	// Branded vs non-branded.
	if bs := sc.BrandSplit; bs != nil {
		b, nb := bs.Branded, bs.NonBranded
		bPct, bOK := b.DeltaPct()
		nbPct, nbOK := nb.DeltaPct()
		total := b.Current + nb.Current
		switch {
		case bOK && nbOK && nbPct > bPct+5 && nbPct > neutralPct:
			w.add(Positive, sec, 61,
				"Les requêtes hors marque progressent plus vite (%s) que les requêtes de marque (%s) : le site attire de nouveaux visiteurs qui ne le connaissaient pas.",
				"Non-branded queries grow faster (%s) than branded ones (%s): the site reaches new visitors who did not know the brand.",
				w.sp(nbPct), w.sp(bPct))
		case nbOK && nbPct < -neutralPct:
			w.add(Negative, sec, 58,
				"Les clics hors marque reculent de %s : le site dépend davantage des internautes qui connaissent déjà la marque.",
				"Non-branded clicks fell by %s: the site relies more on people who already know the brand.",
				w.p(nbPct))
		case total > 0:
			w.add(Neutral, sec, 32,
				"Les requêtes hors marque représentent %s des clics.",
				"Non-branded queries account for %s of clicks.",
				w.p(nb.Current/total*100))
		}
	}
}

func pageInsights(w *writer, sc *model.SearchConsole) {
	var best *model.GSCRow
	var bestD float64
	for i := range sc.Pages {
		p := &sc.Pages[i]
		if p.PrevClicks == nil {
			continue
		}
		if d := p.Clicks - *p.PrevClicks; d > bestD {
			best, bestD = p, d
		}
	}
	if best != nil {
		change := ""
		if *best.PrevClicks > 0 {
			change = ", " + w.sp(bestD / *best.PrevClicks * 100)
		}
		w.add(Positive, "pages", 52,
			"Page en plus forte progression : %s (%s clics%s).",
			"Fastest-growing page: %s (%s clicks%s).",
			ShortURL(best.Key), w.sn(bestD), change)
	}
}

func deviceInsights(w *writer, sc *model.SearchConsole) {
	const sec = "countries_devices"
	var mobile, total float64
	for _, d := range sc.Devices {
		total += d.Clicks
		k := strings.ToLower(d.Key)
		if strings.Contains(k, "mobile") || strings.Contains(k, "phone") {
			mobile += d.Clicks
		}
	}
	if total > 0 && mobile > 0 {
		share := mobile / total * 100
		if share >= 50 {
			w.add(Neutral, sec, 33,
				"Le mobile représente %s des clics Google : testez en priorité vos pages sur smartphone.",
				"Mobile accounts for %s of Google clicks: test your pages on smartphones first.",
				w.p(share))
		} else {
			w.add(Neutral, sec, 28,
				"Le mobile représente %s des clics Google, l'ordinateur reste majoritaire.",
				"Mobile accounts for %s of Google clicks; desktop remains the main device.",
				w.p(share))
		}
	}
	if len(sc.Countries) >= 2 {
		var top model.GSCRow
		var sum float64
		for _, c := range sc.Countries {
			sum += c.Clicks
			if c.Clicks > top.Clicks {
				top = c
			}
		}
		if sum > 0 && top.Key != "" {
			w.add(Neutral, sec, 22,
				"Premier pays : %s, avec %s des clics.",
				"Top country: %s, with %s of clicks.",
				top.Key, w.p(top.Clicks/sum*100))
		}
	}
}

func technicalInsights(w *writer, sc *model.SearchConsole) {
	const sec = "technical"
	if idx := sc.Indexing; idx != nil {
		var notIdx float64
		hasNot := false
		if idx.NotIndexed != nil {
			notIdx, hasNot = idx.NotIndexed.Current, true
		} else if len(idx.Issues) > 0 {
			for _, is := range idx.Issues {
				notIdx += is.Pages
			}
			hasNot = true
		}
		var top model.IndexingIssue
		for _, is := range idx.Issues {
			if is.Pages > top.Pages {
				top = is
			}
		}
		if hasNot && notIdx > 0 {
			tone := Neutral
			if notIdx > 0.25*idx.Indexed.Current || (idx.NotIndexed != nil && toneOf(*idx.NotIndexed, false) == Positive) {
				tone = Negative
			}
			if top.Reason != "" {
				w.add(tone, sec, 46,
					"%s pages ne sont pas indexées par Google ; raison principale : %s (%s pages).",
					"%s pages are not indexed by Google; main reason: %s (%s pages).",
					w.n(notIdx), w.q(top.Reason), w.n(top.Pages))
			} else {
				w.add(tone, sec, 42,
					"%s pages ne sont pas indexées par Google.",
					"%s pages are not indexed by Google.",
					w.n(notIdx))
			}
		}
		if ip := idx.Indexed; ip.Previous != nil {
			switch toneOf(ip, false) {
			case Positive:
				w.add(Positive, sec, 30,
					"Le nombre de pages indexées passe de %s à %s.",
					"Indexed pages went from %s to %s.",
					w.n(*ip.Previous), w.n(ip.Current))
			case Negative:
				w.add(Negative, sec, 38,
					"Le nombre de pages indexées baisse, de %s à %s.",
					"Indexed pages decreased from %s to %s.",
					w.n(*ip.Previous), w.n(ip.Current))
			}
		}
	}
	if cwv := sc.CoreWebVitals; cwv != nil {
		type dev struct {
			s      *model.CWVStatus
			fr, en string
		}
		for _, d := range []dev{{cwv.Mobile, "mobiles", "mobile"}, {cwv.Desktop, "sur ordinateur", "desktop"}} {
			if d.s == nil {
				continue
			}
			total := d.s.Good + d.s.NeedsImprovement + d.s.Poor
			if total <= 0 {
				continue
			}
			poor := d.s.Poor / total * 100
			bad := (d.s.Poor + d.s.NeedsImprovement) / total * 100
			good := d.s.Good / total * 100
			switch {
			case poor >= 10:
				w.add(Negative, sec, 50,
					"%s des URL "+d.fr+" ont des Core Web Vitals « médiocres » : la lenteur d'affichage peut freiner le référencement et les conversions.",
					"%s of "+d.en+" URLs have “poor” Core Web Vitals: slow pages can hurt rankings and conversions.",
					w.p(poor))
			case bad >= 25:
				w.add(Neutral, sec, 36,
					"%s des URL "+d.fr+" doivent encore améliorer leurs Core Web Vitals.",
					"%s of "+d.en+" URLs still need to improve their Core Web Vitals.",
					w.p(bad))
			case good >= 90:
				w.add(Positive, sec, 24,
					"%s des URL "+d.fr+" ont de bons Core Web Vitals.",
					"%s of "+d.en+" URLs have good Core Web Vitals.",
					w.p(good))
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Google Analytics 4

func ga4Insights(w *writer, ga *model.Analytics) {
	t := ga.Totals
	if s := t.Sessions; s != nil {
		if pct, ok := s.DeltaPct(); ok {
			switch toneOf(*s, false) {
			case Positive:
				w.add(Positive, "analytics", 58+bump(pct),
					"Le site a reçu %s sessions toutes sources confondues (%s).",
					"The site received %s sessions across all channels (%s).",
					w.n(s.Current), w.sp(pct))
			case Negative:
				w.add(Negative, "analytics", 58+bump(pct),
					"Le trafic global recule de %s (%s sessions).",
					"Overall traffic fell by %s (%s sessions).",
					w.p(pct), w.n(s.Current))
			default:
				w.add(Neutral, "analytics", 35,
					"Le trafic global est stable : %s sessions.",
					"Overall traffic is stable: %s sessions.",
					w.n(s.Current))
			}
		}
	}

	org := organicSessions(ga)
	if org != nil {
		if pct, ok := org.DeltaPct(); ok {
			switch toneOf(*org, false) {
			case Positive:
				w.add(Positive, "channels", 66+bump(pct),
					"Le trafic issu du référencement naturel progresse de %s (%s sessions).",
					"Organic search traffic grew by %s (%s sessions).",
					w.p(pct), w.n(org.Current))
			case Negative:
				w.add(Negative, "channels", 66+bump(pct),
					"Le trafic issu du référencement naturel recule de %s (%s sessions).",
					"Organic search traffic fell by %s (%s sessions).",
					w.p(pct), w.n(org.Current))
			default:
				w.add(Neutral, "channels", 40,
					"Le trafic issu du référencement naturel est stable (%s sessions).",
					"Organic search traffic is stable (%s sessions).",
					w.n(org.Current))
			}
		}
	}

	// Channel mix.
	if total := totalSessions(ga); total > 0 {
		if org != nil && org.Current > 0 {
			share := org.Current / total * 100
			suffix := ""
			if org.Previous != nil && t.Sessions != nil && t.Sessions.Previous != nil && *t.Sessions.Previous > 0 {
				prevShare := *org.Previous / *t.Sessions.Previous * 100
				if math.Abs(share-prevShare) >= 0.5 {
					suffix = " (" + w.pts(share-prevShare) + ")"
				}
			}
			w.add(Neutral, "channels", 40,
				"Le référencement naturel apporte %s des sessions du site%s.",
				"Organic search brings %s of the site's sessions%s.",
				w.p(share), suffix)
		}
		var top model.ChannelRow
		for _, c := range ga.Channels {
			if c.Sessions > top.Sessions {
				top = c
			}
		}
		if top.Channel != "" && !IsOrganicSearchChannel(top.Channel) {
			w.add(Neutral, "channels", 30,
				"Le premier canal d'acquisition est %s (%s des sessions).",
				"The top acquisition channel is %s (%s of sessions).",
				w.q(top.Channel), w.p(top.Sessions/total*100))
		}
	}

	if k := t.KeyEvents; k != nil && k.Previous != nil {
		if pct, ok := k.DeltaPct(); ok {
			switch toneOf(*k, false) {
			case Positive:
				w.add(Positive, "analytics", 70+bump(pct),
					"Les conversions (événements clés) progressent de %s : %s contre %s.",
					"Conversions (key events) grew by %s: %s vs %s.",
					w.p(pct), w.n(k.Current), w.n(*k.Previous))
			case Negative:
				w.add(Negative, "analytics", 70+bump(pct),
					"Les conversions (événements clés) reculent de %s : %s contre %s.",
					"Conversions (key events) fell by %s: %s vs %s.",
					w.p(pct), w.n(k.Current), w.n(*k.Previous))
			}
		}
	}
	if rv := t.Revenue; rv != nil && rv.Previous != nil {
		if pct, ok := rv.DeltaPct(); ok {
			switch toneOf(*rv, false) {
			case Positive:
				w.add(Positive, "analytics", 68+bump(pct),
					"Le chiffre d'affaires progresse de %s (%s).",
					"Revenue grew by %s (%s).",
					w.p(pct), w.money(rv.Current))
			case Negative:
				w.add(Negative, "analytics", 68+bump(pct),
					"Le chiffre d'affaires recule de %s (%s).",
					"Revenue fell by %s (%s).",
					w.p(pct), w.money(rv.Current))
			}
		}
	}
	if e := t.EngagementRate; e != nil && e.Previous != nil {
		if tone := toneOf(*e, false); tone != Neutral {
			pct, _ := e.DeltaPct()
			w.add(tone, "analytics", 34+bump(pct)/2,
				"Le taux d'engagement passe de %s à %s.",
				"Engagement rate moved from %s to %s.",
				pa(w.p2(*e.Previous, e.Current))...)
		}
	}

	var best *model.LandingPageRow
	var bestD float64
	for i := range ga.LandingPages {
		lp := &ga.LandingPages[i]
		if lp.PrevSessions == nil {
			continue
		}
		if d := lp.Sessions - *lp.PrevSessions; d > bestD {
			best, bestD = lp, d
		}
	}
	if best != nil {
		w.add(Positive, "landing_pages", 38,
			"Page d'entrée en plus forte hausse : %s (%s sessions).",
			"Landing page with the biggest gain: %s (%s sessions).",
			ShortURL(best.Page), w.sn(bestD))
	}
}

// ---------------------------------------------------------------------------
// GEO

func geoInsights(w *writer, r *model.Report, a *Analysis) {
	const sec = "geo"
	if ai := a.AISessions; ai != nil && ai.Current > 0 {
		var leader model.AIReferralRow
		var leaderShare float64
		if len(a.AIReferrals) > 0 {
			leader = a.AIReferrals[0]
			leaderShare = leader.Sessions / ai.Current * 100
		}
		pct, ok := ai.DeltaPct()
		switch {
		case ai.Previous != nil && *ai.Previous == 0:
			w.add(Positive, sec, 70,
				"Premières visites venant des assistants IA : %s sessions sur la période.",
				"First visits from AI assistants: %s sessions over the period.",
				w.n(ai.Current))
		case ok && toneOf(*ai, false) == Positive && leader.Platform != "":
			w.add(Positive, sec, 72+bump(pct),
				"Les visites venant des assistants IA progressent de %s (%s sessions), portées par %s (%s du trafic IA).",
				"Visits from AI assistants grew by %s (%s sessions), led by %s (%s of AI traffic).",
				w.p(pct), w.n(ai.Current), leader.Platform, w.p(leaderShare))
		case ok && toneOf(*ai, false) == Positive:
			w.add(Positive, sec, 72+bump(pct),
				"Les visites venant des assistants IA progressent de %s (%s sessions).",
				"Visits from AI assistants grew by %s (%s sessions).",
				w.p(pct), w.n(ai.Current))
		case ok && toneOf(*ai, false) == Negative:
			w.add(Negative, sec, 65+bump(pct),
				"Les visites venant des assistants IA reculent de %s (%s sessions).",
				"Visits from AI assistants fell by %s (%s sessions).",
				w.p(pct), w.n(ai.Current))
		case ok:
			w.add(Neutral, sec, 50,
				"Les visites venant des assistants IA sont stables (%s sessions).",
				"Visits from AI assistants are stable (%s sessions).",
				w.n(ai.Current))
		case leader.Platform != "":
			w.add(Neutral, sec, 55,
				"Les assistants IA ont apporté %s sessions, dont %s via %s.",
				"AI assistants brought %s sessions, %s of them via %s.",
				w.n(ai.Current), w.p(leaderShare), leader.Platform)
		default:
			w.add(Neutral, sec, 50,
				"Les assistants IA ont apporté %s sessions sur la période.",
				"AI assistants brought %s sessions over the period.",
				w.n(ai.Current))
		}

		// Fastest-growing platform other than the leader.
		var fast model.AIReferralRow
		fastPct := 0.0
		for _, row := range a.AIReferrals[min(1, len(a.AIReferrals)):] {
			if row.PrevSessions == nil || *row.PrevSessions <= 0 || row.Sessions < math.Max(20, ai.Current*0.05) {
				continue
			}
			if p := (row.Sessions - *row.PrevSessions) / *row.PrevSessions * 100; p > fastPct {
				fast, fastPct = row, p
			}
		}
		leaderPct := 0.0
		if leader.PrevSessions != nil && *leader.PrevSessions > 0 {
			leaderPct = (leader.Sessions - *leader.PrevSessions) / *leader.PrevSessions * 100
		}
		if fast.Platform != "" && fastPct > 20 && fastPct > leaderPct {
			w.add(Positive, sec, 42,
				"%s est l'assistant IA qui progresse le plus vite (%s).",
				"%s is the fastest-growing AI assistant (%s).",
				fast.Platform, w.sp(fastPct))
		}
	}
	if a.AIShare != nil && *a.AIShare > 0 {
		if *a.AIShare < 1 {
			w.add(Neutral, sec, 44,
				"Les assistants IA représentent %s des sessions du site : un canal encore modeste, mais à suivre.",
				"AI assistants account for %s of the site's sessions: still small, but worth watching.",
				w.p(*a.AIShare))
		} else {
			w.add(Neutral, sec, 44,
				"Les assistants IA représentent %s des sessions du site.",
				"AI assistants account for %s of the site's sessions.",
				w.p(*a.AIShare))
		}
	}

	geo := r.GEO
	if geo == nil {
		return
	}
	if cr := a.CitationRate; cr != nil {
		n := float64(len(geo.Citations))
		var cited float64
		for _, c := range geo.Citations {
			if c.Cited {
				cited++
			}
		}
		if cr.Previous != nil {
			d := cr.Current - *cr.Previous
			cur, prev := w.p2(cr.Current, *cr.Previous)
			switch {
			case d >= 0.5:
				w.add(Positive, sec, 66+bump(d),
					"Le site est cité dans %s des réponses IA testées, contre %s précédemment (%s).",
					"The site is cited in %s of tested AI answers, up from %s (%s).",
					cur, prev, w.pts(d))
			case d <= -0.5:
				w.add(Negative, sec, 62+bump(d),
					"Le site est cité dans %s des réponses IA testées, contre %s précédemment (%s).",
					"The site is cited in %s of tested AI answers, down from %s (%s).",
					cur, prev, w.pts(d))
			default:
				w.add(Neutral, sec, 45,
					"Le site est cité dans %s des réponses IA testées, un taux stable.",
					"The site is cited in %s of tested AI answers, a stable rate.",
					w.p(cr.Current))
			}
		} else {
			w.add(Neutral, sec, 50,
				"Le site est cité dans %s des réponses IA testées (%s sur %s).",
				"The site is cited in %s of tested AI answers (%s out of %s).",
				w.p(cr.Current), w.n(cited), w.n(n))
		}

		var gained, lost []model.CitationCheck
		for _, c := range geo.Citations {
			if c.PreviouslyCited == nil {
				continue
			}
			if c.Cited && !*c.PreviouslyCited {
				gained = append(gained, c)
			} else if !c.Cited && *c.PreviouslyCited {
				lost = append(lost, c)
			}
		}
		if len(gained) == 1 {
			c := gained[0]
			w.add(Positive, sec, 54,
				"Nouvelle citation : le site apparaît désormais dans la réponse de %s à %s.",
				"New citation: the site now appears in %s's answer to %s.",
				c.Engine, w.q(c.Prompt))
		} else if len(gained) > 1 {
			c := gained[0]
			w.add(Positive, sec, 54+math.Min(float64(len(gained)), 6),
				"%s nouvelles citations dans les réponses IA, par exemple sur %s pour %s.",
				"%s new citations in AI answers, e.g. on %s for %s.",
				w.n(float64(len(gained))), c.Engine, w.q(c.Prompt))
		}
		if len(lost) == 1 {
			c := lost[0]
			w.add(Negative, sec, 47,
				"Le site n'est plus cité par %s pour %s.",
				"The site is no longer cited by %s for %s.",
				c.Engine, w.q(c.Prompt))
		} else if len(lost) > 1 {
			c := lost[0]
			w.add(Negative, sec, 47+math.Min(float64(len(lost)), 6),
				"%s citations IA perdues, dont %s pour %s.",
				"%s AI citations lost, including %s for %s.",
				w.n(float64(len(lost))), c.Engine, w.q(c.Prompt))
		}
	}

	// Share of voice.
	if len(geo.ShareOfVoice) > 0 {
		sov := append([]model.ShareOfVoice(nil), geo.ShareOfVoice...)
		sort.SliceStable(sov, func(i, j int) bool { return sov[i].Share > sov[j].Share })
		rank := -1
		for i, s := range sov {
			if s.IsSelf {
				rank = i
				break
			}
		}
		if rank >= 0 {
			self := sov[rank]
			tone, suffix := Neutral, ""
			if self.Previous != nil {
				d := self.Share - *self.Previous
				suffix = " (" + w.pts(d) + ")"
				if d >= 0.5 {
					tone = Positive
				} else if d <= -0.5 {
					tone = Negative
				}
			}
			if rank == 0 {
				w.add(tone, sec, 50,
					"Part de voix dans les réponses IA : %s%s, en tête face aux concurrents suivis.",
					"AI share of voice: %s%s, ahead of all tracked competitors.",
					w.p(self.Share), suffix)
			} else {
				leader := sov[0]
				w.add(tone, sec, 50,
					"Part de voix dans les réponses IA : %s%s, au %s rang derrière %s (%s).",
					"AI share of voice: %s%s, ranked %s behind %s (%s).",
					w.p(self.Share), suffix, w.ordinal(rank+1), leader.Brand, w.p(leader.Share))
			}
		}
	}

	// Google AI Overviews.
	if len(geo.AIOverviews) > 0 {
		var present, cited float64
		for _, o := range geo.AIOverviews {
			if o.OverviewPresent {
				present++
				if o.Cited {
					cited++
				}
			}
		}
		n := float64(len(geo.AIOverviews))
		if present == 0 {
			w.add(Neutral, sec, 20,
				"Aucune AI Overview ne s'affiche sur les %s requêtes suivies.",
				"No AI Overview appears on the %s tracked queries.",
				w.n(n))
		} else {
			tone := Neutral
			if cited/present >= 0.5 {
				tone = Positive
			} else if cited == 0 {
				tone = Negative
			}
			w.add(tone, sec, 40,
				"Une AI Overview s'affiche sur %s des %s requêtes suivies ; le site y est cité %s fois.",
				"An AI Overview appears on %s of the %s tracked queries; the site is cited in %s of them.",
				w.n(present), w.n(n), w.n(cited))
		}
	}

	// AI crawlers.
	if len(geo.AICrawlers) > 0 {
		bots := append([]model.CrawlerRow(nil), geo.AICrawlers...)
		sort.SliceStable(bots, func(i, j int) bool { return bots[i].Hits > bots[j].Hits })
		m := model.Metric{}
		var prev float64
		hasPrev := false
		for _, b := range bots {
			m.Current += b.Hits
			if b.PrevHits != nil {
				prev += *b.PrevHits
				hasPrev = true
			}
		}
		if hasPrev {
			m.Previous = model.F(prev)
		}
		names := bots[0].Bot
		if len(bots) > 1 {
			names += ", " + bots[1].Bot
		}
		if len(bots) > 2 {
			names += "…"
		}
		if m.Current > 0 {
			if pct, ok := m.DeltaPct(); ok {
				w.add(toneOf(m, false), sec, 26,
					"Les robots des IA (%s) ont exploré le site %s fois (%s).",
					"AI crawlers (%s) visited the site %s times (%s).",
					names, w.n(m.Current), w.sp(pct))
			} else {
				w.add(Neutral, sec, 24,
					"Les robots des IA (%s) ont exploré le site %s fois.",
					"AI crawlers (%s) visited the site %s times.",
					names, w.n(m.Current))
			}
		}
	}
}
