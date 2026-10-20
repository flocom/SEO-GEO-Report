package importer

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// GMCKind is the kind of Google Merchant Center export table.
type GMCKind string

const (
	GMCProducts   GMCKind = "products"   // Performance by product (title / item ID)
	GMCDates      GMCKind = "dates"      // Performance chart, one row per day
	GMCPrograms   GMCKind = "programs"   // Performance by program / marketing method
	GMCCategories GMCKind = "categories" // Performance by product category
	GMCBrands     GMCKind = "brands"     // Performance by brand
	GMCCountries  GMCKind = "countries"  // Performance by country of sale
	GMCStatus     GMCKind = "status"     // Products export / status summary
	GMCIssues     GMCKind = "issues"     // Diagnostics / "Needs attention" issues
	GMCPrices     GMCKind = "prices"     // Price competitiveness (price vs benchmark)
)

// GMCResult describes what ParseGMCCSV read.
type GMCResult struct {
	Kind         GMCKind
	Rows         int // rows (or days) read
	PreviousDays int // days of the comparison period (dates table)
}

type gmcCol int

const (
	gcNone gmcCol = iota
	gcSkip
	gcClicks
	gcImpressions
	gcCTR
	gcConversions
	gcConvValue
	gcOfferID
	gcTitle
	gcDate
	gcProgram
	gcCategory
	gcBrand
	gcCountry
	gcStatus
	gcIssue
	gcCount
	gcSeverity
	gcPrice
	gcBenchmark
)

// isMetric reports whether a column holds a performance metric.
func (c gmcCol) isMetric() bool {
	switch c {
	case gcClicks, gcImpressions, gcConversions, gcConvValue, gcCount:
		return true
	}
	return false
}

// classifyGMCHeader maps a folded Merchant Center header (French or English)
// to its column type. Order matters: "Taux de clics" is a CTR, "Catégorie de
// produits" a category, "Articles concernés" a count.
func classifyGMCHeader(h string) gmcCol {
	switch {
	case h == "":
		return gcNone
	case isChangeHeader(h):
		return gcSkip
	case containsAny(h, "price", "prix") && containsAny(h, "benchmark", "reference", "competitor", "concurren", "market", "marche", "average", "moyen"):
		return gcBenchmark
	case containsAny(h, "price", "prix"):
		return gcPrice
	case containsAny(h, "ctr", "taux de clic", "click-through", "click through"):
		return gcCTR
	case containsAny(h, "conversion value", "valeur de conversion", "valeur des conversions", "conv. value", "conv value", "revenue", "revenu", "chiffre d'affaires", "valeur"):
		return gcConvValue
	case (strings.Contains(h, "conversion") || h == "conv." || h == "conv") && !containsAny(h, "rate", "taux", "cost", "cout", " per ", " par "):
		return gcConversions
	case containsAny(h, "click", "clic"):
		return gcClicks
	case strings.Contains(h, "impression"):
		return gcImpressions
	case h == "id" || containsAny(h, "offer id", "item id", "product id", "id de l'article", "id de l'offre", "identifiant de l'offre",
		"identifiant de l'article", "id d'offre", "id d'article", "id produit", "id du produit", "identifiant produit", "sku", "offer_id", "item_id"):
		return gcOfferID
	case h == "date" || h == "day" || h == "jour" || strings.HasPrefix(h, "date ") || h == "week" || h == "semaine":
		return gcDate
	case containsAny(h, "categor", "product type", "type de produit"):
		return gcCategory
	case containsAny(h, "brand", "marque"):
		return gcBrand
	case containsAny(h, "country", "pays"):
		return gcCountry
	case containsAny(h, "severity", "gravite", "criticite", "niveau", "priority", "priorite"):
		return gcSeverity
	case containsAny(h, "status", "statut", "etat"):
		return gcStatus
	case containsAny(h, "affected item", "items affected", "affected product", "products affected", "articles concernes", "produits concernes",
		"articles", "items", "products", "produits", "count", "nombre", "number"):
		return gcCount
	case containsAny(h, "program", "marketing method", "methode marketing", "methode de marketing", "surface", "destination",
		"affects", "affecte", "concerne", "impacte", "channel", "canal"):
		return gcProgram
	case containsAny(h, "issue", "probleme", "erreur", "error", "diagnostic", "reason", "raison", "warning", "avertissement"):
		return gcIssue
	case containsAny(h, "title", "titre", "product", "produit", "item", "article", "name", "nom"):
		return gcTitle
	}
	return gcNone
}

// gmcColumns maps a header row.
type gmcColumns struct {
	first      map[gmcCol]int // first column of each type
	prev       map[gmcCol]int // comparison column of each metric
	prevHeader string
	statusCols map[string]int // wide status layout: status -> column
}

func mapGMCColumns(headers []string) gmcColumns {
	c := gmcColumns{first: map[gmcCol]int{}, prev: map[gmcCol]int{}, statusCols: map[string]int{}}
	for i, h := range headers {
		if st := gmcStatusOf(h); st != "" && classifyGMCHeader(h) != gcStatus {
			if _, ok := c.statusCols[st]; !ok {
				c.statusCols[st] = i
			}
		}
		col := classifyGMCHeader(h)
		switch {
		case col == gcNone || col == gcSkip:
			continue
		case col.isMetric() && isPreviousHeader(h):
			c.prev[col] = i
			c.prevHeader = h
		case col.isMetric():
			if _, ok := c.first[col]; !ok {
				c.first[col] = i
			} else if _, ok := c.prev[col]; !ok {
				c.prev[col] = i // two unlabeled columns: the analysed period comes first
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

func (c gmcColumns) has(cols ...gmcCol) bool {
	for _, col := range cols {
		if _, ok := c.first[col]; !ok {
			return false
		}
	}
	return true
}

func (c gmcColumns) hasPerf() bool { return c.has(gcClicks) || c.has(gcImpressions) }

// guessKind returns the table kind of a header row, or "".
func (c gmcColumns) guessKind() GMCKind {
	switch {
	case c.has(gcPrice, gcBenchmark):
		return GMCPrices
	case c.has(gcIssue):
		return GMCIssues
	case c.has(gcStatus) && !c.hasPerf():
		return GMCStatus
	case len(c.statusCols) >= 2 && !c.hasPerf():
		return GMCStatus
	case !c.hasPerf():
		return ""
	case c.has(gcDate):
		return GMCDates
	case c.has(gcProgram):
		return GMCPrograms
	case c.has(gcCategory):
		return GMCCategories
	case c.has(gcBrand):
		return GMCBrands
	case c.has(gcCountry):
		return GMCCountries
	case c.has(gcTitle) || c.has(gcOfferID):
		return GMCProducts
	}
	return ""
}

// keyCol returns the column holding the row key of a kind.
func (c gmcColumns) keyCol(kind GMCKind) (int, bool) {
	var order []gmcCol
	switch kind {
	case GMCProducts, GMCPrices:
		order = []gmcCol{gcTitle, gcOfferID}
	case GMCDates:
		order = []gmcCol{gcDate}
	case GMCPrograms:
		order = []gmcCol{gcProgram}
	case GMCCategories:
		order = []gmcCol{gcCategory}
	case GMCBrands:
		order = []gmcCol{gcBrand}
	case GMCCountries:
		order = []gmcCol{gcCountry}
	case GMCIssues:
		order = []gmcCol{gcIssue}
	case GMCStatus:
		order = []gmcCol{gcStatus}
	}
	for _, col := range order {
		if i, ok := c.first[col]; ok {
			return i, true
		}
	}
	return -1, false
}

// gmcStatusOf maps a folded status label to approved, limited, not_approved,
// pending or total ("" when unknown).
func gmcStatusOf(v string) string {
	switch {
	case v == "":
		return ""
	case containsAny(v, "disapprov", "not approved", "non approuv", "refus", "rejet", "rejected", "invalid", "bloque", "blocked", "non eligible", "not eligible"):
		return "not_approved"
	case containsAny(v, "limit", "partiel", "partial"):
		return "limited"
	case containsAny(v, "pending", "attente", "under review", "in review", "en cours d'examen", "examen", "processing", "en cours de traitement"):
		return "pending"
	case containsAny(v, "approv", "approuv", "eligible", "active", "actif", "valide"):
		return "approved"
	case v == "total" || v == "totals" || v == "totaux" || strings.HasPrefix(v, "total "):
		return "total"
	}
	return ""
}

// normalizeGMCKind accepts the GMCKind constants and loose aliases; unknown
// values give "" (guess from the header).
func normalizeGMCKind(k GMCKind) GMCKind {
	v := strings.ReplaceAll(fold(string(k)), " ", "_")
	switch v {
	case "", string(GMCProducts), string(GMCDates), string(GMCPrograms), string(GMCCategories), string(GMCBrands),
		string(GMCCountries), string(GMCStatus), string(GMCIssues), string(GMCPrices):
		return GMCKind(v)
	case "product", "produits", "produit", "items", "articles":
		return GMCProducts
	case "date", "daily", "day", "days", "jour", "jours", "chart":
		return GMCDates
	case "program", "programmes", "programme", "marketing_method", "marketing_methods", "surfaces", "destinations":
		return GMCPrograms
	case "category", "categorie", "categories_de_produits":
		return GMCCategories
	case "brand", "marque", "marques":
		return GMCBrands
	case "country", "pays":
		return GMCCountries
	case "statuses", "product_status", "statut", "statuts", "etat":
		return GMCStatus
	case "issue", "diagnostics", "diagnostic", "problemes", "probleme", "needs_attention":
		return GMCIssues
	case "price", "price_benchmarks", "benchmarks", "price_competitiveness", "prix", "competitivite_prix":
		return GMCPrices
	}
	return GuessGMCKind(string(k))
}

// GuessGMCKind guesses the table kind from a file name or a header (French or
// English, accents optional). It returns "" when unknown.
func GuessGMCKind(name string) GMCKind {
	h := fold(strings.TrimSuffix(strings.TrimSuffix(name, ".csv"), ".CSV"))
	switch {
	case containsAny(h, "benchmark", "competitiv", "price", "prix"):
		return GMCPrices
	case containsAny(h, "diagnostic", "issue", "probleme", "needs attention", "a traiter", "attention"):
		return GMCIssues
	case containsAny(h, "status", "statut", "etat"):
		return GMCStatus
	case containsAny(h, "program", "marketing", "surface", "destination"):
		return GMCPrograms
	case containsAny(h, "categor"):
		return GMCCategories
	case containsAny(h, "brand", "marque"):
		return GMCBrands
	case containsAny(h, "countr", "pays"):
		return GMCCountries
	case containsAny(h, "date", "daily", "jour", "day", "chart", "graph"):
		return GMCDates
	case containsAny(h, "product", "produit", "item", "article"):
		return GMCProducts
	}
	return ""
}

func gmcLooksFrench(headers []string) bool {
	if looksFrench(headers) {
		return true
	}
	for _, h := range headers {
		if containsAny(h, "titre", "prix", "produit", "valeur", "probleme", "statut", "marque", "categorie", "articles", "gravite", "identifiant") {
			return true
		}
	}
	return false
}

// ParseGMCCSV parses one Google Merchant Center export CSV (French or English
// headers, ',' / ';' / tab separators, BOM, '#' comment lines, "%" values and
// decimal commas accepted) and merges it into into (created when nil):
// performance by product, date, program, category, brand or country (the
// matching list is replaced; comparison columns fill the previous values),
// product statuses (one row per product, or a status / count summary), feed
// issues or price competitiveness. kind may be empty: it is guessed from the
// header. The daily table also recomputes the click and impression totals.
func ParseGMCCSV(data []byte, kind GMCKind, into *model.MerchantCenter) (*model.MerchantCenter, *GMCResult, error) {
	kind = normalizeGMCKind(kind)
	text := decodeText(data)
	for _, blk := range splitBlocks(text) {
		delim := detectDelimiter(blk)
		recs, err := readRecords(blk, delim)
		if err != nil {
			continue
		}
		for hdr := 0; hdr < len(recs) && hdr < 10; hdr++ {
			headers := foldAll(recs[hdr])
			cols := mapGMCColumns(headers)
			k := kind
			if k == "" {
				k = cols.guessKind()
			} else if !cols.supports(k) {
				continue
			}
			if k == "" {
				continue
			}
			decimalComma := delim == ';' || gmcLooksFrench(headers)
			if into == nil {
				into = &model.MerchantCenter{}
			}
			res := applyGMC(into, k, cols, recs[hdr+1:], decimalComma)
			return into, res, nil
		}
	}
	if kind != "" {
		return into, nil, fmt.Errorf("merchant center CSV: no %s table found (French or English headers expected)", kind)
	}
	return into, nil, errors.New("merchant center CSV: no known table found: expected clicks / impressions by product, date, program, category, brand or country, a status column, an issue column, or price and benchmark price columns")
}

// supports reports whether the header can be read as the given kind.
func (c gmcColumns) supports(kind GMCKind) bool {
	switch kind {
	case GMCPrices:
		return c.has(gcPrice, gcBenchmark)
	case GMCIssues:
		return c.has(gcIssue)
	case GMCStatus:
		return c.has(gcStatus) || len(c.statusCols) >= 2
	}
	if !c.hasPerf() {
		return false
	}
	_, ok := c.keyCol(kind)
	return ok
}

func applyGMC(mc *model.MerchantCenter, kind GMCKind, cols gmcColumns, rows [][]string, decimalComma bool) *GMCResult {
	res := &GMCResult{Kind: kind}
	num := func(rec []string, col gmcCol, prev bool) (float64, bool) {
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
	str := func(rec []string, col gmcCol) string {
		if i, ok := cols.first[col]; ok {
			return cell(rec, i)
		}
		return ""
	}
	switch kind {
	case GMCDates:
		res.Rows, res.PreviousDays = applyGMCDates(mc, cols, rows, decimalComma, num)
	case GMCStatus:
		res.Rows = applyGMCStatus(mc, cols, rows, num, str)
	case GMCIssues:
		res.Rows = applyGMCIssues(mc, rows, num, str)
	case GMCPrices:
		var out []model.GMCPriceBenchmark
		for _, rec := range rows {
			p, pOK := num(rec, gcPrice, false)
			bm, bOK := num(rec, gcBenchmark, false)
			name := str(rec, gcTitle)
			if name == "" {
				name = str(rec, gcOfferID)
			}
			if !pOK || !bOK || name == "" || isTotalRow(name) {
				continue
			}
			out = append(out, model.GMCPriceBenchmark{Product: name, OfferID: str(rec, gcOfferID), Price: round2(p), BenchmarkPrice: round2(bm)})
		}
		mc.PriceBenchmarks = out
		res.Rows = len(out)
	default:
		keyIdx, _ := cols.keyCol(kind)
		var out []model.GMCRow
		for _, rec := range rows {
			key := cell(rec, keyIdx)
			if key == "" || isTotalRow(key) {
				continue
			}
			c, cOK := num(rec, gcClicks, false)
			im, iOK := num(rec, gcImpressions, false)
			if !cOK && !iOK {
				continue
			}
			r := model.GMCRow{Key: key, Clicks: c, Impressions: im}
			if kind == GMCProducts && keyIdx != cols.first[gcOfferID] {
				r.OfferID = str(rec, gcOfferID)
			}
			if v, ok := num(rec, gcConversions, false); ok {
				r.Conversions = round2(v)
			}
			if v, ok := num(rec, gcConvValue, false); ok {
				r.ConversionValue = round2(v)
			}
			if v, ok := num(rec, gcClicks, true); ok {
				r.PrevClicks = model.F(v)
			}
			if v, ok := num(rec, gcImpressions, true); ok {
				r.PrevImpressions = model.F(v)
			}
			out = append(out, r)
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].Clicks > out[j].Clicks })
		switch kind {
		case GMCProducts:
			mc.Products = out
		case GMCPrograms:
			mc.Programs = out
		case GMCCategories:
			mc.Categories = out
		case GMCBrands:
			mc.Brands = out
		case GMCCountries:
			mc.Countries = out
		}
		res.Rows = len(out)
	}
	return res
}

func applyGMCDates(mc *model.MerchantCenter, cols gmcColumns, rows [][]string, decimalComma bool, num func([]string, gmcCol, bool) (float64, bool)) (int, int) {
	keyIdx := cols.first[gcDate]
	dayFirst := dayFirstDates(rows, keyIdx, decimalComma)
	var cur, prev []model.GMCDailyPoint
	var curDates []string
	var conv, pconv, value, pvalue float64
	hasConv, hasPConv, hasValue, hasPValue := false, false, false, false
	for _, rec := range rows {
		date, ok := parseDate(cell(rec, keyIdx), dayFirst)
		if !ok {
			continue
		}
		c, cOK := num(rec, gcClicks, false)
		im, iOK := num(rec, gcImpressions, false)
		if cOK || iOK {
			cur = append(cur, model.GMCDailyPoint{Date: date, Clicks: c, Impressions: im})
			curDates = append(curDates, date)
		}
		if v, ok := num(rec, gcConversions, false); ok {
			conv, hasConv = conv+v, true
		}
		if v, ok := num(rec, gcConvValue, false); ok {
			value, hasValue = value+v, true
		}
		if v, ok := num(rec, gcConversions, true); ok {
			pconv, hasPConv = pconv+v, true
		}
		if v, ok := num(rec, gcConvValue, true); ok {
			pvalue, hasPValue = pvalue+v, true
		}
		pc, pcOK := num(rec, gcClicks, true)
		pim, piOK := num(rec, gcImpressions, true)
		if pcOK || piOK {
			prev = append(prev, model.GMCDailyPoint{Date: date, Clicks: pc, Impressions: pim})
		}
	}
	// Comparison values sit on the rows of the analysed period: shift their
	// dates to the comparison period.
	shift := shiftDates(curDates, isYearComparison(cols.prevHeader))
	for i := range prev {
		prev[i].Date = shift(prev[i].Date)
	}
	sortGMCDaily(cur)
	sortGMCDaily(prev)
	mc.Daily, mc.PreviousDaily = cur, prev
	if len(cur) > 0 {
		mc.Totals.Clicks, mc.Totals.Impressions, mc.Totals.CTR = model.Metric{}, model.Metric{}, nil
		mc.ComputeTotals()
	}
	if hasConv {
		m := &model.Metric{Current: round2(conv)}
		if hasPConv {
			m.Previous = model.F(round2(pconv))
		}
		mc.Totals.Conversions = m
	}
	if hasValue {
		m := &model.Metric{Current: round2(value)}
		if hasPValue {
			m.Previous = model.F(round2(pvalue))
		}
		mc.Totals.ConversionValue = m
	}
	return len(cur), len(prev)
}

func sortGMCDaily(pts []model.GMCDailyPoint) {
	sort.SliceStable(pts, func(i, j int) bool { return pts[i].Date < pts[j].Date })
}

// applyGMCStatus reads product statuses: a status / count summary, one row
// per product (statuses are counted), or one row with a column per status.
func applyGMCStatus(mc *model.MerchantCenter, cols gmcColumns, rows [][]string, num func([]string, gmcCol, bool) (float64, bool), str func([]string, gmcCol) string) int {
	cur := map[string]float64{}
	prev := map[string]float64{}
	hasPrev := false
	n := 0
	switch {
	case cols.has(gcStatus) && cols.has(gcCount):
		for _, rec := range rows {
			st := gmcStatusOf(fold(str(rec, gcStatus)))
			v, ok := num(rec, gcCount, false)
			if st == "" || !ok {
				continue
			}
			cur[st] += v
			if p, ok := num(rec, gcCount, true); ok {
				prev[st] += p
				hasPrev = true
			}
			n++
		}
	case cols.has(gcStatus):
		for _, rec := range rows {
			if st := gmcStatusOf(fold(str(rec, gcStatus))); st != "" && st != "total" {
				cur[st]++
				n++
			}
		}
	default:
		for _, rec := range rows {
			found := false
			for st, i := range cols.statusCols {
				if v, _, ok := parseNumber(cell(rec, i), false); ok {
					cur[st], found = v, true
				}
			}
			if found {
				n = len(cur)
				break // the first data row holds the counts
			}
		}
	}
	if n == 0 {
		return 0
	}
	metric := func(st string) *model.Metric {
		v, ok := cur[st]
		if !ok {
			return nil
		}
		m := &model.Metric{Current: v}
		if hasPrev {
			m.Previous = model.F(prev[st])
		}
		return m
	}
	ps := &model.GMCProductStatus{Limited: metric("limited"), NotApproved: metric("not_approved"), Pending: metric("pending"), Total: metric("total")}
	if m := metric("approved"); m != nil {
		ps.Approved = *m
	}
	mc.ProductStatus = ps
	return n
}

// applyGMCIssues reads feed issues. Rows of the same issue (one per
// destination or per product) are merged: counts are taken from the count
// column (the largest value) or else the rows are counted.
func applyGMCIssues(mc *model.MerchantCenter, rows [][]string, num func([]string, gmcCol, bool) (float64, bool), str func([]string, gmcCol) string) int {
	type agg struct {
		is      model.GMCIssue
		affects []string
	}
	byName := map[string]*agg{}
	var order []string
	for _, rec := range rows {
		name := str(rec, gcIssue)
		if name == "" || isTotalRow(name) {
			continue
		}
		key := fold(name)
		a, ok := byName[key]
		if !ok {
			a = &agg{is: model.GMCIssue{Issue: name}}
			byName[key] = a
			order = append(order, key)
		}
		if v, ok := num(rec, gcCount, false); ok {
			a.is.Products = max(a.is.Products, v)
		} else {
			a.is.Products++
		}
		if v, ok := num(rec, gcCount, true); ok {
			if a.is.PrevProducts == nil || v > *a.is.PrevProducts {
				a.is.PrevProducts = model.F(v)
			}
		}
		if a.is.Severity == "" {
			a.is.Severity = model.NormSeverity(str(rec, gcSeverity))
		}
		if aff := str(rec, gcProgram); aff != "" {
			dup := false
			for _, x := range a.affects {
				dup = dup || strings.EqualFold(x, aff)
			}
			if !dup {
				a.affects = append(a.affects, aff)
			}
		}
	}
	out := make([]model.GMCIssue, 0, len(order))
	for _, k := range order {
		a := byName[k]
		a.is.Affects = strings.Join(a.affects, ", ")
		out = append(out, a.is)
	}
	mc.Issues = out
	return len(out)
}
