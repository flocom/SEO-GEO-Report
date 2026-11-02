package importer

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// GBPKind is the kind of Google Business Profile export table.
type GBPKind string

const (
	GBPPerformance GBPKind = "performance" // Performance / bulk "insights" export: one row per location
	GBPDaily       GBPKind = "daily"       // Date + metric columns, one row per day
	GBPKeywords    GBPKind = "keywords"    // Search keywords (searches that showed the profile)
	GBPReviews     GBPKind = "reviews"     // Reviews export: rating, comment, date, reply
)

// GBPResult describes what ParseGBPCSV read.
type GBPResult struct {
	Kind         GBPKind
	Rows         int // rows read (locations, days, keywords or reviews)
	Locations    int // locations kept (performance table with several rows)
	PreviousDays int // days of the comparison period (daily table)
}

type gbpCol int

const (
	bcNone gbpCol = iota
	bcSkip
	bcStoreCode
	bcName
	bcAddress
	bcCity
	bcDate
	bcKeyword
	bcRating
	bcReviewCount
	bcComment
	bcReply
	bcAuthor
	bcViews
	bcSearchMobile
	bcSearchDesktop
	bcMapsMobile
	bcMapsDesktop
	bcSearch // all-device search views (old "Total searches" style)
	bcMaps
	bcInteractions
	bcCalls
	bcWebsite
	bcDirections
	bcMessages
	bcBookings
	bcFoodOrders
	bcMenu
	bcValue // number of searches of a keyword
)

// isMetric reports whether a column holds a performance metric.
func (c gbpCol) isMetric() bool { return c >= bcViews }

// classifyGBPHeader maps a folded Google Business Profile header (French or
// English, UI export or Performance API metric names such as
// BUSINESS_IMPRESSIONS_MOBILE_MAPS) to its column type.
func classifyGBPHeader(h string) gbpCol {
	h = strings.ReplaceAll(h, "_", " ")
	h = strings.Join(strings.Fields(strings.NewReplacer("–", " ", "—", " ", " - ", " ", "(", " ", ")", " ").Replace(h)), " ")
	mobile := containsAny(h, "mobile", "smartphone")
	desktop := containsAny(h, "desktop", "ordinateur", "computer", "bureau")
	maps := strings.Contains(h, "maps") || strings.Contains(h, "carte")
	search := containsAny(h, "google search", "recherche google", "search", "recherche")
	switch {
	case h == "":
		return bcNone
	case isChangeHeader(h):
		return bcSkip
	case containsAny(h, "store code", "code de magasin", "code magasin", "code d'etablissement", "code etablissement", "store id"):
		return bcStoreCode
	case containsAny(h, "keyword", "mot cle", "mots cles", "mot-cle", "requete", "search term", "terme de recherche", "termes de recherche", "search query"):
		return bcKeyword
	case containsAny(h, "review count", "number of reviews", "nombre d'avis", "total reviews", "avis au total", "reviews count", "nb avis"):
		return bcReviewCount
	case containsAny(h, "rating", "star", "etoile", "note") && !containsAny(h, "notes", "start"):
		return bcRating
	case containsAny(h, "reply", "response", "reponse"):
		return bcReply
	case containsAny(h, "reviewer", "author", "auteur", "client name", "nom du client"):
		return bcAuthor
	case containsAny(h, "comment", "commentaire", "review text", "texte", "contenu") || h == "review" || h == "avis" || h == "text":
		return bcComment
	case h == "date" || h == "day" || h == "jour" || strings.HasPrefix(h, "date ") || containsAny(h, "create time", "publication", "published"):
		return bcDate
	case containsAny(h, "address", "adresse"):
		return bcAddress
	case containsAny(h, "city", "ville", "locality", "localite", "commune"):
		return bcCity
	case maps && mobile:
		return bcMapsMobile
	case maps && desktop:
		return bcMapsDesktop
	case search && mobile:
		return bcSearchMobile
	case search && desktop:
		return bcSearchDesktop
	case containsAny(h, "call", "appel"):
		return bcCalls
	case containsAny(h, "website", "site web", "site internet", "clics vers le site", "clic vers le site", "site"):
		return bcWebsite
	case containsAny(h, "direction", "itineraire"):
		return bcDirections
	case containsAny(h, "message", "conversation"):
		return bcMessages
	case containsAny(h, "booking", "reservation"):
		return bcBookings
	case containsAny(h, "food order", "commande"):
		return bcFoodOrders
	case containsAny(h, "menu"):
		return bcMenu
	case containsAny(h, "interaction", "actions", "total actions"):
		return bcInteractions
	case maps:
		return bcMaps
	case containsAny(h, "views", "vues", "vue de la fiche", "impressions", "profile view", "people who viewed", "personnes ayant consulte"):
		return bcViews
	case search && containsAny(h, "total", "searches") && !containsAny(h, "discovery", "direct", "branded"):
		return bcSearch
	case containsAny(h, "value", "valeur", "searches", "recherches", "volume", "count", "nombre"):
		return bcValue
	case containsAny(h, "business name", "nom de l'etablissement", "location name", "nom de l'entreprise", "nom", "name", "title", "titre", "etablissement", "location"):
		return bcName
	}
	return bcNone
}

type gbpColumns struct {
	first      map[gbpCol]int
	prev       map[gbpCol]int
	prevHeader string
}

func mapGBPColumns(headers []string) gbpColumns {
	c := gbpColumns{first: map[gbpCol]int{}, prev: map[gbpCol]int{}}
	for i, h := range headers {
		col := classifyGBPHeader(h)
		switch {
		case col == bcNone || col == bcSkip:
			continue
		case (col.isMetric() || col == bcValue) && isPreviousHeader(h):
			if _, ok := c.prev[col]; !ok {
				c.prev[col] = i
				c.prevHeader = h
			}
		default:
			if _, ok := c.first[col]; !ok {
				c.first[col] = i
			}
		}
	}
	for m, i := range c.prev {
		if _, ok := c.first[m]; !ok {
			c.first[m] = i
			delete(c.prev, m)
		}
	}
	return c
}

func (c gbpColumns) has(cols ...gbpCol) bool {
	for _, col := range cols {
		if _, ok := c.first[col]; !ok {
			return false
		}
	}
	return true
}

func (c gbpColumns) hasMetric() bool {
	for col := range c.first {
		if col.isMetric() {
			return true
		}
	}
	return false
}

func (c gbpColumns) guessKind() GBPKind {
	switch {
	case c.has(bcRating) && (c.has(bcComment) || c.has(bcReply) || c.has(bcAuthor)):
		return GBPReviews
	case c.has(bcKeyword) && (c.has(bcValue) || c.has(bcViews) || c.has(bcSearch)):
		return GBPKeywords
	case c.has(bcDate) && c.hasMetric():
		return GBPDaily
	case c.hasMetric():
		return GBPPerformance
	}
	return ""
}

func (c gbpColumns) supports(kind GBPKind) bool {
	switch kind {
	case GBPReviews:
		return c.has(bcRating)
	case GBPKeywords:
		return c.has(bcKeyword) && (c.has(bcValue) || c.has(bcViews) || c.has(bcSearch))
	case GBPDaily:
		return c.has(bcDate) && c.hasMetric()
	case GBPPerformance:
		return c.hasMetric()
	}
	return false
}

// normalizeGBPKind accepts the GBPKind constants and loose aliases; unknown
// values give "" (guess from the header).
func normalizeGBPKind(k GBPKind) GBPKind {
	v := strings.ReplaceAll(fold(string(k)), " ", "_")
	switch v {
	case "", string(GBPPerformance), string(GBPDaily), string(GBPKeywords), string(GBPReviews):
		return GBPKind(v)
	case "insights", "bulk", "locations", "location", "totals", "statistiques", "performances":
		return GBPPerformance
	case "date", "dates", "day", "days", "jour", "jours", "chart", "quotidien":
		return GBPDaily
	case "keyword", "search_keywords", "searches", "queries", "mots_cles", "recherches", "requetes":
		return GBPKeywords
	case "review", "avis", "ratings":
		return GBPReviews
	}
	return GuessGBPKind(string(k))
}

// GuessGBPKind guesses the table kind from a file name (French or English,
// accents optional). It returns "" when unknown.
func GuessGBPKind(name string) GBPKind {
	h := fold(strings.TrimSuffix(strings.TrimSuffix(name, ".csv"), ".CSV"))
	switch {
	case containsAny(h, "review", "avis", "rating"):
		return GBPReviews
	case containsAny(h, "keyword", "mot cle", "mots cles", "mots-cles", "search term", "requete", "terme"):
		return GBPKeywords
	case containsAny(h, "daily", "quotidien", "jour", "day", "date", "chart", "graph"):
		return GBPDaily
	case containsAny(h, "insight", "performance", "statisti", "bulk", "location", "etablissement"):
		return GBPPerformance
	}
	return ""
}

func gbpLooksFrench(headers []string) bool {
	if looksFrench(headers) {
		return true
	}
	for _, h := range headers {
		if containsAny(h, "appel", "itineraire", "recherche google", "ordinateur", "etablissement", "valeur", "avis", "note", "adresse", "ville", "magasin") {
			return true
		}
	}
	return false
}

// ParseGBPCSV parses one Google Business Profile export CSV (French or
// English headers, ',' / ';' / tab separators, BOM, '#' comment lines,
// decimal commas, "<15" thresholds) and merges it into into (created when
// nil):
//   - performance: the Performance / bulk insights export, one row per
//     location (views by Google Search / Maps and mobile / desktop, calls,
//     website clicks, direction requests, messages, bookings, food orders);
//     several rows fill locations and the summed totals, one row the totals;
//   - daily: one row per day (date and metric columns), comparison columns
//     fill previous_daily;
//   - keywords: search terms with their number of searches;
//   - reviews: rating, comment, date and reply (the 10 most recent reviews,
//     the star distribution and the reply rate).
//
// kind may be empty: it is guessed from the header.
func ParseGBPCSV(data []byte, kind GBPKind, into *model.BusinessProfile) (*model.BusinessProfile, *GBPResult, error) {
	kind = normalizeGBPKind(kind)
	text := decodeText(data)
	for _, blk := range splitBlocks(text) {
		delim := detectDelimiter(blk)
		recs, err := readRecords(blk, delim)
		if err != nil {
			continue
		}
		for hdr := 0; hdr < len(recs) && hdr < 10; hdr++ {
			headers := foldAll(recs[hdr])
			cols := mapGBPColumns(headers)
			k := kind
			if k == "" {
				k = cols.guessKind()
			} else if !cols.supports(k) {
				continue
			}
			if k == "" {
				continue
			}
			decimalComma := delim == ';' || gbpLooksFrench(headers)
			if into == nil {
				into = &model.BusinessProfile{}
			}
			res := applyGBP(into, k, cols, recs[hdr+1:], decimalComma)
			if res.Rows == 0 {
				return into, res, fmt.Errorf("business profile CSV: the %s table has no data row", k)
			}
			into.Normalize()
			return into, res, nil
		}
	}
	if kind != "" {
		return into, nil, fmt.Errorf("business profile CSV: no %s table found (French or English headers expected)", kind)
	}
	return into, nil, errors.New("business profile CSV: no known table found: expected profile views / calls / website clicks / direction requests (per location or per day), search keywords with their number of searches, or reviews with a rating column")
}

func applyGBP(bp *model.BusinessProfile, kind GBPKind, cols gbpColumns, rows [][]string, decimalComma bool) *GBPResult {
	res := &GBPResult{Kind: kind}
	num := func(rec []string, col gbpCol, prev bool) (float64, bool) {
		idx, ok := cols.first[col]
		if prev {
			idx, ok = cols.prev[col]
		}
		if !ok {
			return 0, false
		}
		v, _, ok := parseNumber(cell(rec, idx), decimalComma)
		return v, ok
	}
	str := func(rec []string, col gbpCol) string {
		if i, ok := cols.first[col]; ok {
			return cell(rec, i)
		}
		return ""
	}
	switch kind {
	case GBPPerformance:
		applyGBPPerformance(bp, cols, rows, num, str, res)
	case GBPDaily:
		res.Rows, res.PreviousDays = applyGBPDaily(bp, cols, rows, decimalComma, num)
	case GBPKeywords:
		res.Rows = applyGBPKeywords(bp, cols, rows, decimalComma, str)
	case GBPReviews:
		res.Rows = applyGBPReviews(bp, rows, str)
	}
	return res
}

type gbpNum func([]string, gbpCol, bool) (float64, bool)

// gbpTotalsOf reads the metric columns of one row; ok is false when no
// metric could be read (e.g. a description row under the header).
func gbpTotalsOf(rec []string, num gbpNum) (model.GBPTotals, bool) {
	var t model.GBPTotals
	found := false
	metric := func(col gbpCol) *model.Metric {
		v, ok := num(rec, col, false)
		if !ok {
			return nil
		}
		found = true
		m := &model.Metric{Current: v}
		if p, ok := num(rec, col, true); ok {
			m.Previous = model.F(p)
		}
		return m
	}
	t.Views = metric(bcViews)
	t.ViewsSearchMobile = metric(bcSearchMobile)
	t.ViewsSearchDesktop = metric(bcSearchDesktop)
	t.ViewsMapsMobile = metric(bcMapsMobile)
	t.ViewsMapsDesktop = metric(bcMapsDesktop)
	t.Interactions = metric(bcInteractions)
	t.Calls = metric(bcCalls)
	t.WebsiteClicks = metric(bcWebsite)
	t.DirectionRequests = metric(bcDirections)
	t.Messages = metric(bcMessages)
	t.Bookings = metric(bcBookings)
	t.FoodOrders = metric(bcFoodOrders)
	t.MenuClicks = metric(bcMenu)
	// Exports without the device split: all-device Search / Maps views.
	if t.Views == nil && t.ViewsSearchMobile == nil && t.ViewsSearchDesktop == nil && t.ViewsMapsMobile == nil && t.ViewsMapsDesktop == nil {
		s, m := metric(bcSearch), metric(bcMaps)
		if s != nil || m != nil {
			views := &model.Metric{}
			allPrev := true
			for _, x := range []*model.Metric{s, m} {
				if x == nil {
					continue
				}
				views.Current += x.Current
				if x.Previous == nil {
					allPrev = false
				}
			}
			if allPrev {
				p := 0.0
				for _, x := range []*model.Metric{s, m} {
					if x != nil {
						p += *x.Previous
					}
				}
				views.Previous = model.F(p)
			}
			t.Views = views
		}
	}
	return t, found
}

func applyGBPPerformance(bp *model.BusinessProfile, cols gbpColumns, rows [][]string, num gbpNum, str func([]string, gbpCol) string, res *GBPResult) {
	var locs []model.GBPLocation
	for _, rec := range rows {
		name := str(rec, bcName)
		code := str(rec, bcStoreCode)
		if isTotalRow(name) || isTotalRow(code) {
			continue
		}
		t, ok := gbpTotalsOf(rec, num)
		if !ok {
			continue
		}
		l := model.GBPLocation{Name: name, StoreCode: code, Address: str(rec, bcAddress), City: str(rec, bcCity), Totals: t}
		if v, ok := num(rec, bcRating, false); ok && v > 0 {
			l.Rating = model.F(round2(v))
		}
		if v, ok := num(rec, bcReviewCount, false); ok {
			l.ReviewCount = model.F(v)
		}
		locs = append(locs, l)
	}
	res.Rows = len(locs)
	switch {
	case len(locs) == 1:
		bp.Totals = locs[0].Totals
		bp.Locations = nil
	case len(locs) > 1:
		bp.Locations = locs
		bp.Totals = model.GBPTotals{} // summed from the locations by Normalize
		res.Locations = len(locs)
	}
}

func applyGBPDaily(bp *model.BusinessProfile, cols gbpColumns, rows [][]string, decimalComma bool, num gbpNum) (int, int) {
	keyIdx := cols.first[bcDate]
	dayFirst := dayFirstDates(rows, keyIdx, decimalComma)
	read := func(rec []string, prev bool) (model.GBPDailyPoint, bool) {
		var p model.GBPDailyPoint
		found := false
		get := func(col gbpCol) float64 {
			v, ok := num(rec, col, prev)
			found = found || ok
			return v
		}
		views := get(bcViews)
		if views == 0 {
			views = get(bcSearchMobile) + get(bcSearchDesktop) + get(bcMapsMobile) + get(bcMapsDesktop) + get(bcSearch) + get(bcMaps)
		}
		p.Views = views
		p.Calls = get(bcCalls)
		p.WebsiteClicks = get(bcWebsite)
		p.DirectionRequests = get(bcDirections)
		p.Interactions = get(bcInteractions)
		if p.Interactions == 0 {
			p.Interactions = p.Calls + p.WebsiteClicks + p.DirectionRequests + get(bcMessages) + get(bcBookings) + get(bcFoodOrders) + get(bcMenu)
		}
		return p, found
	}
	var cur, prev []model.GBPDailyPoint
	var curDates []string
	for _, rec := range rows {
		date, ok := parseDate(cell(rec, keyIdx), dayFirst)
		if !ok {
			continue
		}
		if p, ok := read(rec, false); ok {
			p.Date = date
			cur = append(cur, p)
			curDates = append(curDates, date)
		}
		if len(cols.prev) > 0 {
			if p, ok := read(rec, true); ok {
				p.Date = date
				prev = append(prev, p)
			}
		}
	}
	shift := shiftDates(curDates, isYearComparison(cols.prevHeader))
	for i := range prev {
		prev[i].Date = shift(prev[i].Date)
	}
	sort.SliceStable(cur, func(i, j int) bool { return cur[i].Date < cur[j].Date })
	sort.SliceStable(prev, func(i, j int) bool { return prev[i].Date < prev[j].Date })
	bp.Daily, bp.PreviousDaily = cur, prev
	return len(cur), len(prev)
}

// gbpThreshold reads a number of searches: "1 240", "<15", "< 15",
// "moins de 15", "fewer than 15".
func gbpThreshold(s string, decimalComma bool) (float64, bool, bool) {
	f := fold(s)
	below := strings.HasPrefix(f, "<") || containsAny(f, "moins de", "fewer than", "less than", "under ")
	v, _, ok := parseNumber(s, decimalComma)
	return v, below, ok
}

func applyGBPKeywords(bp *model.BusinessProfile, cols gbpColumns, rows [][]string, decimalComma bool, str func([]string, gbpCol) string) int {
	valCol := bcValue
	if !cols.has(bcValue) {
		valCol = bcViews
		if !cols.has(bcViews) {
			valCol = bcSearch
		}
	}
	var out []model.GBPKeyword
	for _, rec := range rows {
		kw := str(rec, bcKeyword)
		if kw == "" || isTotalRow(kw) {
			continue
		}
		v, below, ok := gbpThreshold(cell(rec, cols.first[valCol]), decimalComma)
		if !ok {
			continue
		}
		k := model.GBPKeyword{Keyword: kw, Impressions: v, BelowThreshold: below}
		if i, ok := cols.prev[valCol]; ok {
			if p, pBelow, ok := gbpThreshold(cell(rec, i), decimalComma); ok && !pBelow {
				k.PrevImpressions = model.F(p)
			}
		}
		out = append(out, k)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Impressions > out[j].Impressions })
	if len(out) > 0 {
		bp.SearchKeywords = out
	}
	return len(out)
}

// gbpStars reads a star rating: "5", "4,0", "FIVE" (API enum), "cinq",
// "5 étoiles", "★★★★☆".
func gbpStars(s string) (float64, bool) {
	if n := strings.Count(s, "★"); n > 0 {
		return float64(n), true
	}
	f := fold(s)
	words := map[string]float64{"one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "un": 1, "une": 1, "deux": 2, "trois": 3, "quatre": 4, "cinq": 5}
	for _, w := range strings.Fields(strings.NewReplacer("_", " ", "-", " ").Replace(f)) {
		if v, ok := words[w]; ok {
			return v, true
		}
	}
	v, _, ok := parseNumber(s, true)
	if !ok || v < 1 || v > 5 {
		return 0, false
	}
	return v, true
}

func applyGBPReviews(bp *model.BusinessProfile, rows [][]string, str func([]string, gbpCol) string) int {
	var all []model.GBPReview
	dist := &model.GBPRatingDistribution{}
	replied := 0
	for _, rec := range rows {
		rating, ok := gbpStars(str(rec, bcRating))
		if !ok {
			continue
		}
		r := model.GBPReview{Rating: rating, Text: str(rec, bcComment), Author: str(rec, bcAuthor)}
		if d, ok := parseDate(str(rec, bcDate), true); ok {
			r.Date = d
		}
		if rep := fold(str(rec, bcReply)); rep != "" && rep != "no" && rep != "non" && rep != "false" && rep != "0" && rep != "-" {
			r.Responded = true
			replied++
		}
		switch int(rating + 0.5) {
		case 1:
			dist.One++
		case 2:
			dist.Two++
		case 3:
			dist.Three++
		case 4:
			dist.Four++
		default:
			dist.Five++
		}
		all = append(all, r)
	}
	if len(all) == 0 {
		return 0
	}
	recent := append([]model.GBPReview(nil), all...)
	sort.SliceStable(recent, func(i, j int) bool { return recent[i].Date > recent[j].Date })
	if len(recent) > 10 {
		recent = recent[:10]
	}
	if bp.Reviews == nil {
		bp.Reviews = &model.GBPReviews{}
	}
	rv := bp.Reviews
	rv.Recent = recent
	if rv.RatingDistribution == nil {
		rv.RatingDistribution = dist
	}
	if rv.ResponseRate == nil && len(all) >= 3 {
		rv.ResponseRate = &model.Metric{Current: round2(float64(replied) / float64(len(all)) * 100)}
	}
	return len(all)
}
