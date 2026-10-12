package importer

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

func (k BingKind) isAI() bool {
	return k == BingAIQueries || k == BingAIPages || k == BingAIDates
}

// aiKindOf maps a Search Performance kind to its AI Performance counterpart.
func aiKindOf(k BingKind) BingKind {
	switch k {
	case BingQueries:
		return BingAIQueries
	case BingPages:
		return BingAIPages
	case BingDates:
		return BingAIDates
	}
	return k
}

// GuessBingKind guesses the table kind from a file name or a header (French or
// English, accents optional). It returns "" when unknown.
func GuessBingKind(name string) BingKind {
	h := fold(strings.TrimSuffix(strings.TrimSuffix(name, ".csv"), ".CSV"))
	if containsAny(h, "grounding", "ancrage") {
		return BingAIQueries
	}
	ai := containsAny(h, "ai performance", "ai-performance", "ai_performance", "aiperformance",
		"performance ia", "performance-ia", "performances ia", "citation", "copilot")
	var k BingKind
	switch GuessGSCKind(name) {
	case GSCQueries:
		k = BingQueries
	case GSCPages:
		k = BingPages
	case GSCDates:
		k = BingDates
	}
	// "Performances de recherche - Pages": "recherche" names the report, not
	// the table.
	if k == BingQueries && containsAny(h, "page", "url") && !containsAny(h, "quer", "requete", "keyword", "mot cle", "mots cles", "mot-cle", "mots-cles") {
		k = BingPages
	}
	if ai {
		return aiKindOf(k)
	}
	return k
}

// normalizeBingKind accepts the BingKind constants and loose aliases
// ("keywords", "grounding queries", "cited pages"...); unknown values give ""
// (guess).
func normalizeBingKind(k BingKind) BingKind {
	v := strings.ReplaceAll(fold(string(k)), " ", "_")
	switch v {
	case "", string(BingQueries), string(BingPages), string(BingDates), string(BingAIQueries), string(BingAIPages), string(BingAIDates):
		return BingKind(v)
	case "keywords", "keyword", "mots_cles":
		return BingQueries
	case "grounding_queries", "grounding", "ai_keywords":
		return BingAIQueries
	case "cited_pages", "ai_page":
		return BingAIPages
	case "ai_daily", "ai_chart", "citations":
		return BingAIDates
	}
	return GuessBingKind(string(k))
}

type aiMetric int

const (
	aiNone aiMetric = iota
	aiCitations
	aiShare
	aiCitedPages
	aiIntent
	aiTopic
	aiSkip
)

func classifyAIHeader(h string) aiMetric {
	switch {
	case isChangeHeader(h):
		return aiSkip
	case containsAny(h, "share", "part ", "part des", "taux") && containsAny(h, "citation", "cite"):
		return aiShare
	case containsAny(h, "cited page", "pages cited", "page cited", "pages citees", "page citee", "unique page", "pages uniques"):
		return aiCitedPages
	case strings.Contains(h, "citation"):
		return aiCitations
	case containsAny(h, "intent", "intention"):
		return aiIntent
	case containsAny(h, "topic", "sujet", "theme", "thematique"):
		return aiTopic
	}
	return aiNone
}

// aiColumns maps an AI Performance header row to its columns.
type aiColumns struct {
	key, prevKey      int
	cur, prev         map[aiMetric]int
	prevHeader        string
	share, intent, tp int
}

func mapAIColumns(headers []string) (aiColumns, bool) {
	c := aiColumns{key: -1, prevKey: -1, share: -1, intent: -1, tp: -1, cur: map[aiMetric]int{}, prev: map[aiMetric]int{}}
	for i, h := range headers {
		switch m := classifyAIHeader(h); m {
		case aiSkip:
		case aiNone:
			if c.key < 0 {
				c.key = i
			} else if c.prevKey < 0 {
				c.prevKey = i
			}
		case aiShare:
			if c.share < 0 {
				c.share = i
			}
		case aiIntent:
			if c.intent < 0 {
				c.intent = i
			}
		case aiTopic:
			if c.tp < 0 {
				c.tp = i
			}
		default:
			if isPreviousHeader(h) {
				c.prev[m] = i
				c.prevHeader = h
			} else if _, ok := c.cur[m]; !ok {
				c.cur[m] = i
			} else if _, ok := c.prev[m]; !ok {
				c.prev[m] = i // two unlabeled columns: the analysed period comes first
			}
		}
	}
	for m, i := range c.prev {
		if _, ok := c.cur[m]; !ok {
			c.cur[m] = i
			delete(c.prev, m)
		}
	}
	// A "Cited pages" column holding the URLs is the key of the pages table.
	if i, ok := c.cur[aiCitedPages]; ok && c.key < 0 {
		c.key = i
		delete(c.cur, aiCitedPages)
	}
	_, hasCitations := c.cur[aiCitations]
	return c, c.key >= 0 && hasCitations
}

// ParseBingCSV parses one Bing Webmaster Tools export CSV: a Search
// Performance table (keywords, pages or the daily chart, read like a Search
// Console table) or an AI Performance table (grounding queries, cited pages or
// the daily chart, recognized by their Citations column). kind may be empty:
// it is then guessed from the header.
func ParseBingCSV(data []byte, kind BingKind) (*BingTable, error) {
	kind = normalizeBingKind(kind)
	text := decodeText(data)
	delim := detectDelimiter(text)
	recs, err := readRecords(text, delim)
	if err != nil {
		return nil, err
	}
	hdr := -1
	var cols aiColumns
	var headers []string
	for i, rec := range recs {
		hs := make([]string, len(rec))
		for j, h := range rec {
			hs[j] = fold(h)
		}
		if c, ok := mapAIColumns(hs); ok {
			hdr, cols, headers = i, c, hs
			break
		}
		// A Search Performance header ends the search: data rows below it
		// may contain "citation" (a keyword, a URL) and must not be read as
		// an AI Performance header.
		if _, ok := mapGSCColumns(hs); ok {
			break
		}
	}
	if hdr < 0 {
		if kind.isAI() {
			return nil, errors.New("bing AI Performance CSV: no Citations column found")
		}
		return parseBingSearchCSV(data, kind)
	}
	if !kind.isAI() {
		kind = aiKindOf(kind)
	}
	decimalComma := delim == ';' || looksFrench(headers)
	rows := recs[hdr+1:]
	if kind == "" {
		switch k := GuessBingKind(headers[cols.key]); {
		case k.isAI():
			kind = k
		case k != "":
			kind = aiKindOf(k)
		case containsAny(headers[cols.key], "date", "day", "jour"):
			kind = BingAIDates
		case len(rows) > 0:
			if _, ok := parseDate(cell(rows[0], cols.key), decimalComma); ok {
				kind = BingAIDates
			}
		}
		if kind == "" {
			kind = BingAIQueries
		}
	}
	return parseBingAI(rows, cols, kind, decimalComma), nil
}

// parseBingSearchCSV reads a Search Performance table with the Search Console
// parser.
func parseBingSearchCSV(data []byte, kind BingKind) (*BingTable, error) {
	t, err := ParseGSCCSV(data, GSCKind(kind))
	if err != nil {
		return nil, errors.New(strings.Replace(err.Error(), "search console CSV", "bing CSV", 1))
	}
	switch t.Kind {
	case GSCQueries, GSCPages, GSCDates:
	default:
		return nil, fmt.Errorf("bing CSV: %s tables are not supported (use keywords, pages or the daily chart)", t.Kind)
	}
	return &BingTable{Kind: BingKind(t.Kind), Rows: t.Rows, Daily: t.Daily, PreviousDaily: t.PreviousDaily}, nil
}

func parseBingAI(rows [][]string, cols aiColumns, kind BingKind, decimalComma bool) *BingTable {
	num := func(rec []string, m aiMetric, prev bool) (float64, bool) {
		idx, ok := cols.cur[m]
		if prev {
			idx, ok = cols.prev[m]
		}
		if !ok {
			return 0, false
		}
		v, _, ok := parseNumber(cell(rec, idx), decimalComma)
		return v, ok
	}
	t := &BingTable{Kind: kind}
	switch kind {
	case BingAIDates:
		var curDates []string
		type prevPoint struct {
			date, from string
			p          model.BingAIDailyPoint
		}
		var prevs []prevPoint
		for _, rec := range rows {
			date, ok := parseDate(cell(rec, cols.key), decimalComma)
			if !ok {
				continue
			}
			c, cOK := num(rec, aiCitations, false)
			pg, _ := num(rec, aiCitedPages, false)
			if cOK {
				t.AIDaily = append(t.AIDaily, model.BingAIDailyPoint{Date: date, Citations: c, CitedPages: round2(pg)})
				curDates = append(curDates, date)
			}
			pc, pcOK := num(rec, aiCitations, true)
			if !pcOK {
				continue
			}
			ppg, _ := num(rec, aiCitedPages, true)
			pp := prevPoint{from: date, p: model.BingAIDailyPoint{Citations: pc, CitedPages: round2(ppg)}}
			switch {
			case cols.prevKey >= 0:
				if d, ok := parseDate(cell(rec, cols.prevKey), decimalComma); ok {
					pp.date = d
				}
			case !cOK:
				pp.date = date // row of the comparison period itself
			}
			prevs = append(prevs, pp)
		}
		shift := shiftDates(curDates, isYearComparison(cols.prevHeader))
		for _, pp := range prevs {
			if pp.date == "" {
				pp.date = shift(pp.from)
			}
			pp.p.Date = pp.date
			t.AIPreviousDaily = append(t.AIPreviousDaily, pp.p)
		}
		sortAIDaily(t.AIDaily)
		sortAIDaily(t.AIPreviousDaily)
	case BingAIPages:
		for _, rec := range rows {
			url := cell(rec, cols.key)
			c, ok := num(rec, aiCitations, false)
			if url == "" || !ok {
				continue
			}
			p := model.BingCitedPage{URL: url, Citations: c}
			if v, ok := num(rec, aiCitations, true); ok {
				p.PrevCitations = model.F(v)
			}
			t.CitedPages = append(t.CitedPages, p)
		}
	default:
		for _, rec := range rows {
			q := cell(rec, cols.key)
			c, ok := num(rec, aiCitations, false)
			if q == "" || !ok {
				continue
			}
			g := model.BingGroundingQuery{Query: q, Citations: c, Intent: cell(rec, cols.intent), Topic: cell(rec, cols.tp)}
			if v, ok := num(rec, aiCitations, true); ok {
				g.PrevCitations = model.F(v)
			}
			if v, _, ok := parseNumber(cell(rec, cols.share), decimalComma); ok {
				g.CitationShare = round2(v)
			}
			if v, ok := num(rec, aiCitedPages, false); ok {
				g.CitedPages = round2(v)
			}
			t.GroundingQueries = append(t.GroundingQueries, g)
		}
	}
	return t
}

func sortAIDaily(pts []model.BingAIDailyPoint) {
	sort.SliceStable(pts, func(i, j int) bool { return pts[i].Date < pts[j].Date })
}

// ApplyBingTable merges a parsed table into bg (creating it when nil). The
// daily Search Performance table recomputes the search totals; the daily AI
// Performance table recomputes the total citations and average cited pages,
// and the cited pages table gives the total citations when it is unknown.
func ApplyBingTable(bg *model.Bing, t *BingTable) *model.Bing {
	if bg == nil {
		bg = &model.Bing{}
	}
	if t == nil {
		return bg
	}
	ai := func() *model.BingAIPerformance {
		if bg.AIPerformance == nil {
			bg.AIPerformance = &model.BingAIPerformance{}
		}
		return bg.AIPerformance
	}
	switch t.Kind {
	case BingQueries:
		bg.Queries = t.Rows
	case BingPages:
		bg.Pages = t.Rows
	case BingDates:
		bg.Daily = t.Daily
		bg.PreviousDaily = t.PreviousDaily
		RecomputeBingTotals(bg)
	case BingAIQueries:
		ai().GroundingQueries = t.GroundingQueries
	case BingAIPages:
		a := ai()
		a.Pages = t.CitedPages
		if len(a.Daily) == 0 {
			a.Citations = model.Metric{} // the pages give the total: recompute it
		}
		a.ComputeTotals()
	case BingAIDates:
		a := ai()
		a.Daily = t.AIDaily
		a.PreviousDaily = t.AIPreviousDaily
		a.Citations, a.CitedPages = model.Metric{}, nil
		a.ComputeTotals()
	}
	return bg
}

// RecomputeBingTotals sets bg.Totals from the daily series, like
// RecomputeGSCTotals.
func RecomputeBingTotals(bg *model.Bing) {
	if bg == nil {
		return
	}
	sc := &model.SearchConsole{Totals: bg.Totals, Daily: bg.Daily, PreviousDaily: bg.PreviousDaily}
	RecomputeGSCTotals(sc)
	bg.Totals = sc.Totals
}

// ImportBingExport reads Bing Webmaster Tools exports, given as a .zip file, a
// directory of CSV files or a single CSV file. Files are recognized by their
// name (Keywords, Pages, AI Performance / Grounding queries...) or, failing
// that, by their header.
func ImportBingExport(path string) (*model.Bing, error) {
	files, err := readExportFiles(path)
	if err != nil {
		return nil, err
	}
	var bg *model.Bing
	var errs []string
	for _, f := range files {
		base := filepath.Base(f.name)
		if isGSCFilterFile(base) {
			continue
		}
		t, err := ParseBingCSV(f.data, GuessBingKind(base))
		if err != nil {
			errs = append(errs, base+": "+err.Error())
			continue
		}
		bg = ApplyBingTable(bg, t)
	}
	if bg == nil {
		if len(errs) > 0 {
			return nil, fmt.Errorf("no Bing Webmaster Tools table found in %s (%s)", path, strings.Join(errs, "; "))
		}
		return nil, fmt.Errorf("no Bing Webmaster Tools CSV file found in %s", path)
	}
	return bg, nil
}
