package insights

import (
	"math"
	"sort"
	"strings"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// PriceGapTolerance is the relative price gap (percent) under which a product
// is considered priced like the market.
const PriceGapTolerance = 2.0

// PriceGap returns the gap between a price and its benchmark in percent
// (+12 = 12 % more expensive than the benchmark), and false when it cannot
// be computed.
func PriceGap(p model.GMCPriceBenchmark) (float64, bool) {
	if p.Price <= 0 || p.BenchmarkPrice <= 0 {
		return 0, false
	}
	return (p.Price - p.BenchmarkPrice) / p.BenchmarkPrice * 100, true
}

// IsFreeListingsProgram reports whether a Merchant Center program / marketing
// method is the free (organic) listings: "Free listings", "Fiches gratuites",
// "Unpaid", "Organic"...
func IsFreeListingsProgram(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	return strings.Contains(n, "free") || strings.Contains(n, "gratuit") || strings.Contains(n, "organic") ||
		strings.Contains(n, "organique") || strings.Contains(n, "unpaid") || strings.Contains(n, "non payant") ||
		strings.Contains(n, "non-payant") || strings.Contains(n, "naturel")
}

// MainGMCIssue returns the issue to fix first: the most severe, then the one
// affecting the most products. ok is false without issues.
func MainGMCIssue(issues []model.GMCIssue) (model.GMCIssue, bool) {
	var best model.GMCIssue
	found := false
	for _, is := range issues {
		if strings.TrimSpace(is.Issue) == "" || is.Products <= 0 {
			continue
		}
		if !found || model.SeverityRank(is.Severity) < model.SeverityRank(best.Severity) ||
			(model.SeverityRank(is.Severity) == model.SeverityRank(best.Severity) && is.Products > best.Products) {
			best, found = is, true
		}
	}
	return best, found
}

func gmcKPIs(a *Analysis, mc *model.MerchantCenter) {
	t := mc.Totals
	if mc.HasPerformance() {
		a.KPIs[KeyGMCClicks] = kpiStatus(KeyGMCClicks, t.Clicks, false)
		a.KPIs[KeyGMCImpressions] = kpiStatus(KeyGMCImpressions, t.Impressions, false)
		if t.CTR != nil {
			a.KPIs[KeyGMCCTR] = kpiStatus(KeyGMCCTR, *t.CTR, false)
		}
	}
	if t.Conversions != nil {
		a.KPIs[KeyGMCConversions] = kpiStatus(KeyGMCConversions, *t.Conversions, false)
	}
	if rate := mc.ProductStatus.ApprovedRate(); rate != nil {
		a.KPIs[KeyGMCApprovedRate] = kpiStatus(KeyGMCApprovedRate, *rate, false)
	}
}

func gmcInsights(w *writer, mc *model.MerchantCenter) {
	const sec = "merchant_center"
	t := mc.Totals

	// Clicks. Weights stay close to Bing: Shopping is a complementary channel.
	c := t.Clicks
	if pct, ok := c.DeltaPct(); ok {
		switch toneOf(c, false) {
		case Positive:
			w.add(Positive, sec, 52+bump(pct)/2,
				"Les clics sur vos produits dans Google Shopping progressent de %s : %s contre %s sur la période de comparaison.",
				"Clicks on your products in Google Shopping grew by %s: %s vs %s in the comparison period.",
				w.p(pct), w.n(c.Current), w.n(*c.Previous))
		case Negative:
			w.add(Negative, sec, 52+bump(pct)/2,
				"Les clics sur vos produits dans Google Shopping reculent de %s : %s contre %s sur la période de comparaison.",
				"Clicks on your products in Google Shopping fell by %s: %s vs %s in the comparison period.",
				w.p(pct), w.n(c.Current), w.n(*c.Previous))
		default:
			w.add(Neutral, sec, 38,
				"Les clics sur vos produits dans Google Shopping sont stables : %s (%s).",
				"Clicks on your products in Google Shopping are stable: %s (%s).",
				w.n(c.Current), w.sp(pct))
		}
	} else if c.Current > 0 {
		w.add(Neutral, sec, 32,
			"Vos produits ont reçu %s clics depuis Google Shopping sur la période.",
			"Your products received %s clicks from Google Shopping over the period.",
			w.n(c.Current))
	}

	// Impressions.
	if imp := t.Impressions; imp.Current > 0 {
		if pct, ok := imp.DeltaPct(); ok && toneOf(imp, false) != Neutral {
			tone, fr, en := Positive, "Vos produits sont plus visibles dans Google : %s impressions (%s).",
				"Your products are more visible in Google: %s impressions (%s)."
			if toneOf(imp, false) == Negative {
				tone, fr, en = Negative, "Vos produits sont moins visibles dans Google : %s impressions (%s).",
					"Your products are less visible in Google: %s impressions (%s)."
			}
			w.add(tone, sec, 36+bump(pct)/2, fr, en, w.n(imp.Current), w.sp(pct))
		}
	}

	// Conversions.
	if cv := t.Conversions; cv != nil {
		if pct, ok := cv.DeltaPct(); ok && toneOf(*cv, false) != Neutral {
			tone, fr, en := Positive, "Les ventes attribuées à vos fiches produits progressent : %s conversions (%s).",
				"Sales attributed to your product listings are growing: %s conversions (%s)."
			if toneOf(*cv, false) == Negative {
				tone, fr, en = Negative, "Les ventes attribuées à vos fiches produits reculent : %s conversions (%s).",
					"Sales attributed to your product listings are declining: %s conversions (%s)."
			}
			w.add(tone, sec, 40+bump(pct)/2, fr, en, w.n(cv.Current), w.sp(pct))
		}
	}

	// Feed health: refused or limited products and the main issue.
	issue, hasIssue := MainGMCIssue(mc.Issues)
	// issueText is appended as an argument (never inside a format string:
	// issue names may contain "%").
	issueText := func() string {
		if !hasIssue {
			return ""
		}
		if w.lang == "en" {
			return " Main issue to fix: " + w.q(issue.Issue) + " (" + w.plural(issue.Products, "product", "products") + ")."
		}
		return " Problème principal à corriger : " + w.q(issue.Issue) + " (" + w.plural(issue.Products, "produit", "produits") + ")."
	}
	healthDone := false
	if st := mc.ProductStatus; st != nil {
		total := st.TotalProducts()
		var bad float64
		for _, m := range []*model.Metric{st.NotApproved, st.Limited} {
			if m != nil {
				bad += m.Current
			}
		}
		rate := st.ApprovedRate()
		if total.Current > 0 {
			share := bad / total.Current * 100
			switch {
			case share >= 5:
				w.add(Negative, sec, 54+math.Min(share, 30)/3,
					"%s de vos produits sont refusés ou limités dans Merchant Center (%s sur %s) : ils sont absents ou peu visibles dans Google Shopping.%s",
					"%s of your products are disapproved or limited in Merchant Center (%s of %s): they are missing or barely visible in Google Shopping.%s",
					w.p(share), w.n(bad), w.n(total.Current), issueText())
				healthDone = true
			case rate != nil && rate.Previous != nil && rate.Current-*rate.Previous >= 1:
				cur, prev := w.p2(rate.Current, *rate.Previous)
				w.add(Positive, sec, 34,
					"La part de produits approuvés dans Merchant Center progresse : %s contre %s.",
					"The share of approved products in Merchant Center is rising: %s vs %s.",
					cur, prev)
			case rate != nil:
				w.add(Positive, sec, 26,
					"Votre flux produits est sain : %s des produits sont approuvés dans Merchant Center.",
					"Your product feed is healthy: %s of products are approved in Merchant Center.",
					w.p(rate.Current))
			}
		}
	}
	if hasIssue && !healthDone {
		tone, weight := Neutral, 30.0
		if model.SeverityRank(issue.Severity) == 0 {
			tone, weight = Negative, 44
		}
		w.add(tone, sec, weight,
			"Problème principal du flux produits : %s (%s).",
			"Main product feed issue: %s (%s).",
			w.q(issue.Issue), w.plural2(issue.Products, "produit concerné", "produits concernés", "product affected", "products affected"))
	}

	// Rising product.
	var best model.GMCRow
	bestGain := 0.0
	for _, p := range mc.Products {
		if p.PrevClicks != nil && p.Clicks-*p.PrevClicks > bestGain {
			best, bestGain = p, p.Clicks-*p.PrevClicks
		}
	}
	if bestGain >= 10 && strings.TrimSpace(best.Key) != "" {
		w.add(Positive, sec, 30,
			"Dans Google Shopping, le produit %s gagne le plus de clics (%s).",
			"In Google Shopping, the product %s gains the most clicks (%s).",
			w.q(best.Key), w.sn(bestGain))
	}

	// Price competitiveness.
	var above, priced int
	var worst model.GMCPriceBenchmark
	worstGap := 0.0
	for _, p := range mc.PriceBenchmarks {
		gap, ok := PriceGap(p)
		if !ok {
			continue
		}
		priced++
		if gap > PriceGapTolerance {
			above++
			if gap > worstGap {
				worst, worstGap = p, gap
			}
		}
	}
	switch {
	case above > 0:
		tone := Neutral
		if above*2 >= priced {
			tone = Negative
		}
		w.add(tone, sec, 34+math.Min(float64(above)/float64(priced)*10, 10),
			"Produits plus chers que la moyenne du marché : %s sur %s suivis (jusqu'à %s pour %s), un frein au clic dans Google Shopping.",
			"Products priced above the market average: %s of %s tracked (up to %s for %s), a brake on clicks in Google Shopping.",
			w.n(float64(above)), w.n(float64(priced)), w.sp(worstGap), w.q(worst.Product))
	case priced >= 2:
		w.add(Positive, sec, 26,
			"Vos prix sont compétitifs : vos %s produits suivis sont au prix du marché ou en dessous.",
			"Your prices are competitive: your %s tracked products are priced at or below the market average.",
			w.n(float64(priced)))
	}

	// Program mix: free listings.
	var free, all float64
	var freeRow *model.GMCRow
	for i, p := range mc.Programs {
		all += p.Clicks
		if IsFreeListingsProgram(p.Key) && freeRow == nil {
			freeRow = &mc.Programs[i]
			free = p.Clicks
		}
	}
	if freeRow != nil && all > 0 && len(mc.Programs) >= 2 {
		m := model.Metric{Current: freeRow.Clicks, Previous: freeRow.PrevClicks}
		if pct, ok := m.DeltaPct(); ok && toneOf(m, false) == Positive {
			w.add(Positive, sec, 33+bump(pct)/4,
				"Les fiches produits gratuites progressent de %s (%s clics, %s du total) : du trafic Shopping sans coût publicitaire.",
				"Free product listings grew by %s (%s clicks, %s of the total): Shopping traffic at no advertising cost.",
				w.p(pct), w.n(free), w.p(free/all*100))
		} else {
			w.add(Neutral, sec, 27,
				"Les fiches produits gratuites représentent %s des clics de vos produits dans Google.",
				"Free product listings account for %s of the clicks on your products in Google.",
				w.p(free/all*100))
		}
	}
}

// plural formats a count with its singular / plural noun ("1 produit",
// "12 produits").
func (w *writer) plural(n float64, one, other string) string {
	if (w.lang == "en" && n == 1) || (w.lang != "en" && math.Abs(n) < 2) {
		return w.n(n) + " " + one
	}
	return w.n(n) + " " + other
}

// plural2 is plural with French and English nouns.
func (w *writer) plural2(n float64, frOne, frOther, enOne, enOther string) string {
	if w.lang == "en" {
		return w.plural(n, enOne, enOther)
	}
	return w.plural(n, frOne, frOther)
}

// SortGMCIssues returns the issues sorted by severity (critical first), then
// by number of products affected.
func SortGMCIssues(issues []model.GMCIssue) []model.GMCIssue {
	out := append([]model.GMCIssue(nil), issues...)
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := model.SeverityRank(out[i].Severity), model.SeverityRank(out[j].Severity)
		if ri != rj {
			return ri < rj
		}
		return out[i].Products > out[j].Products
	})
	return out
}
