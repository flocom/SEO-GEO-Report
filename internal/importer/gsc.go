package importer

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

type gscMetric int

const (
	gscNone gscMetric = iota
	gscClicks
	gscImpressions
	gscCTR
	gscPosition
)

func classifyGSCHeader(h string) gscMetric {
	switch {
	case containsAny(h, "ctr", "taux de clic"):
		return gscCTR
	case containsAny(h, "clicks", "clics", "click", "clic"):
		return gscClicks
	case strings.Contains(h, "impression"):
		return gscImpressions
	case strings.Contains(h, "position"):
		return gscPosition
	}
	return gscNone
}

// GuessGSCKind guesses the table kind from a header or a file name (French
// or English, accents optional). It returns "" when unknown.
func GuessGSCKind(name string) GSCKind {
	h := fold(strings.TrimSuffix(strings.TrimSuffix(name, ".csv"), ".CSV"))
	switch {
	case containsAny(h, "appearance", "apparence"):
		return GSCAppearance
	case containsAny(h, "quer", "requete", "recherche"):
		return GSCQueries
	case containsAny(h, "page"):
		return GSCPages
	case containsAny(h, "countr", "pays"):
		return GSCCountries
	case containsAny(h, "device", "appareil"):
		return GSCDevices
	case containsAny(h, "date", "chart", "graphique", "jour", "day"):
		return GSCDates
	}
	return ""
}

// normalizeGSCKind accepts the GSCKind constants and loose aliases
// ("Query", "requêtes", "date"...); unknown values give "" (guess).
func normalizeGSCKind(k GSCKind) GSCKind {
	switch v := GSCKind(fold(string(k))); v {
	case "", GSCQueries, GSCPages, GSCCountries, GSCDevices, GSCDates, GSCAppearance:
		return v
	}
	return GuessGSCKind(string(k))
}

func isGSCFilterFile(name string) bool {
	h := fold(name)
	return strings.HasPrefix(h, "filter") || strings.HasPrefix(h, "filtre")
}

// gscColumns maps a header row to the metric columns.
type gscColumns struct {
	key        int
	prevKey    int // second dimension column (comparison dates), -1 if none
	cur, prev  map[gscMetric]int
	prevHeader string
}

func mapGSCColumns(headers []string) (gscColumns, bool) {
	c := gscColumns{key: -1, prevKey: -1, cur: map[gscMetric]int{}, prev: map[gscMetric]int{}}
	seen := map[gscMetric]int{}
	for i, h := range headers {
		m := classifyGSCHeader(h)
		if m == gscNone {
			if c.key < 0 {
				c.key = i
			} else if c.prevKey < 0 {
				c.prevKey = i
			}
			continue
		}
		seen[m]++
		if isPreviousHeader(h) {
			c.prev[m] = i
			c.prevHeader = h
		} else if _, ok := c.cur[m]; !ok {
			c.cur[m] = i
		} else if _, ok := c.prev[m]; !ok {
			// Two unlabeled columns (e.g. date ranges as labels): GSC puts
			// the analysed period first.
			c.prev[m] = i
		}
	}
	// A metric only present with a "previous" label and nothing else is
	// treated as current (e.g. a single "Previous" column is unlikely).
	for m, i := range c.prev {
		if _, ok := c.cur[m]; !ok && seen[m] == 1 {
			c.cur[m] = i
			delete(c.prev, m)
		}
	}
	_, hasClicks := c.cur[gscClicks]
	_, hasImpr := c.cur[gscImpressions]
	return c, c.key >= 0 && (hasClicks || hasImpr)
}

// ParseGSCCSV parses one Search Console export CSV. kind may be empty: it is
// then guessed from the header.
func ParseGSCCSV(data []byte, kind GSCKind) (*GSCTable, error) {
	text := decodeText(data)
	delim := detectDelimiter(text)
	recs, err := readRecords(text, delim)
	if err != nil {
		return nil, err
	}
	// The header is the first row with a clicks or impressions column.
	hdr := -1
	var cols gscColumns
	var headers []string
	for i, rec := range recs {
		hs := make([]string, len(rec))
		for j, h := range rec {
			hs[j] = fold(h)
		}
		if c, ok := mapGSCColumns(hs); ok {
			hdr, cols, headers = i, c, hs
			break
		}
	}
	if hdr < 0 {
		return nil, errors.New("search console CSV: no Clicks/Impressions header found")
	}
	kind = normalizeGSCKind(kind)
	if kind == "" {
		kind = GuessGSCKind(headers[cols.key])
	}
	decimalComma := delim == ';' || looksFrench(headers)
	rows := recs[hdr+1:]
	if kind == "" && len(rows) > 0 {
		if _, ok := parseDate(cell(rows[0], cols.key), decimalComma); ok {
			kind = GSCDates
		}
	}
	if kind == "" {
		return nil, fmt.Errorf("search console CSV: cannot guess the table kind from column %q", headers[cols.key])
	}

	num := func(rec []string, m gscMetric, prev bool) (float64, bool) {
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
	ctrOf := func(clicks, impr, parsed float64, ok bool) float64 {
		if impr > 0 {
			return round2(clicks / impr * 100)
		}
		if ok {
			return round2(parsed)
		}
		return 0
	}

	t := &GSCTable{Kind: kind}
	if kind == GSCDates {
		var curDates []string
		type prevPoint struct {
			date string // explicit or "" (to be shifted)
			from string // current date it is aligned with
			p    model.GSCDailyPoint
		}
		var prevs []prevPoint
		for _, rec := range rows {
			date, ok := parseDate(cell(rec, cols.key), decimalComma)
			if !ok {
				continue
			}
			c, cOK := num(rec, gscClicks, false)
			im, iOK := num(rec, gscImpressions, false)
			ctr, ctrOK := num(rec, gscCTR, false)
			pos, _ := num(rec, gscPosition, false)
			if cOK || iOK {
				t.Daily = append(t.Daily, model.GSCDailyPoint{Date: date, Clicks: c, Impressions: im, CTR: ctrOf(c, im, ctr, ctrOK), Position: round2(pos)})
				curDates = append(curDates, date)
			}
			pc, pcOK := num(rec, gscClicks, true)
			pim, piOK := num(rec, gscImpressions, true)
			if !pcOK && !piOK {
				continue
			}
			pctr, pctrOK := num(rec, gscCTR, true)
			ppos, _ := num(rec, gscPosition, true)
			pp := prevPoint{p: model.GSCDailyPoint{Clicks: pc, Impressions: pim, CTR: ctrOf(pc, pim, pctr, pctrOK), Position: round2(ppos)}}
			switch {
			case cols.prevKey >= 0:
				if d, ok := parseDate(cell(rec, cols.prevKey), decimalComma); ok {
					pp.date = d
				} else {
					pp.from = date
				}
			case !cOK && !iOK:
				pp.date = date // row of the comparison period itself
			default:
				pp.from = date
			}
			prevs = append(prevs, pp)
		}
		shift := shiftDates(curDates, isYearComparison(cols.prevHeader))
		for _, pp := range prevs {
			if pp.date == "" {
				pp.date = shift(pp.from)
			}
			pp.p.Date = pp.date
			t.PreviousDaily = append(t.PreviousDaily, pp.p)
		}
		sortDaily(t.Daily)
		sortDaily(t.PreviousDaily)
		return t, nil
	}

	for _, rec := range rows {
		key := cell(rec, cols.key)
		if key == "" {
			continue
		}
		c, _ := num(rec, gscClicks, false)
		im, _ := num(rec, gscImpressions, false)
		ctr, ctrOK := num(rec, gscCTR, false)
		pos, _ := num(rec, gscPosition, false)
		row := model.GSCRow{Key: key, Clicks: c, Impressions: im, CTR: ctrOf(c, im, ctr, ctrOK), Position: round2(pos)}
		if v, ok := num(rec, gscClicks, true); ok {
			row.PrevClicks = model.F(v)
		}
		if v, ok := num(rec, gscImpressions, true); ok {
			row.PrevImpressions = model.F(v)
		}
		if v, ok := num(rec, gscPosition, true); ok && v > 0 {
			row.PrevPosition = model.F(round2(v))
		}
		t.Rows = append(t.Rows, row)
	}
	return t, nil
}

func sortDaily(pts []model.GSCDailyPoint) {
	sort.SliceStable(pts, func(i, j int) bool { return pts[i].Date < pts[j].Date })
}

// ApplyGSCTable merges a parsed table into sc (creating it when nil) and
// recomputes totals when the table is the Dates table.
func ApplyGSCTable(sc *model.SearchConsole, t *GSCTable) *model.SearchConsole {
	if sc == nil {
		sc = &model.SearchConsole{}
	}
	if t == nil {
		return sc
	}
	switch t.Kind {
	case GSCQueries:
		sc.Queries = t.Rows
	case GSCPages:
		sc.Pages = t.Rows
	case GSCCountries:
		sc.Countries = t.Rows
	case GSCDevices:
		sc.Devices = t.Rows
	case GSCAppearance:
		sc.SearchAppearance = t.Rows
	case GSCDates:
		sc.Daily = t.Daily
		sc.PreviousDaily = t.PreviousDaily
		RecomputeGSCTotals(sc)
	}
	return sc
}

// RecomputeGSCTotals sets sc.Totals from the daily series: clicks and
// impressions are summed, CTR = clicks / impressions × 100 and the average
// position is weighted by impressions. Previous values come from
// PreviousDaily. When there is no daily series, totals are derived from the
// devices table (or the countries table), which covers the whole property.
func RecomputeGSCTotals(sc *model.SearchConsole) {
	if sc == nil {
		return
	}
	type agg struct{ clicks, impr, wpos, posImpr float64 }
	sumDaily := func(pts []model.GSCDailyPoint) (agg, bool) {
		var a agg
		for _, p := range pts {
			a.clicks += p.Clicks
			a.impr += p.Impressions
			if p.Position > 0 {
				a.wpos += p.Position * p.Impressions
				a.posImpr += p.Impressions
			}
		}
		return a, len(pts) > 0
	}
	sumRows := func(rows []model.GSCRow, prev bool) (agg, bool) {
		var a agg
		has := false
		for _, r := range rows {
			c, im, pos := r.Clicks, r.Impressions, r.Position
			if prev {
				if r.PrevClicks == nil && r.PrevImpressions == nil {
					continue
				}
				c, im, pos = deref(r.PrevClicks), deref(r.PrevImpressions), deref(r.PrevPosition)
			}
			has = true
			a.clicks += c
			a.impr += im
			if pos > 0 {
				a.wpos += pos * im
				a.posImpr += im
			}
		}
		return a, has
	}

	var cur, prev agg
	var hasCur, hasPrev bool
	switch {
	case len(sc.Daily) > 0:
		cur, hasCur = sumDaily(sc.Daily)
		prev, hasPrev = sumDaily(sc.PreviousDaily)
	case len(sc.Devices) > 0:
		cur, hasCur = sumRows(sc.Devices, false)
		prev, hasPrev = sumRows(sc.Devices, true)
	case len(sc.Countries) > 0:
		cur, hasCur = sumRows(sc.Countries, false)
		prev, hasPrev = sumRows(sc.Countries, true)
	}
	if !hasCur {
		return
	}
	t := model.GSCTotals{
		Clicks:      model.Metric{Current: cur.clicks},
		Impressions: model.Metric{Current: cur.impr},
	}
	if hasPrev {
		t.Clicks.Previous = model.F(prev.clicks)
		t.Impressions.Previous = model.F(prev.impr)
	}
	if cur.impr > 0 {
		ctr := model.Metric{Current: round2(cur.clicks / cur.impr * 100)}
		if hasPrev && prev.impr > 0 {
			ctr.Previous = model.F(round2(prev.clicks / prev.impr * 100))
		}
		t.CTR = &ctr
	}
	if cur.posImpr > 0 {
		pos := model.Metric{Current: round2(cur.wpos / cur.posImpr)}
		if hasPrev && prev.posImpr > 0 {
			pos.Previous = model.F(round2(prev.wpos / prev.posImpr))
		}
		t.Position = &pos
	}
	sc.Totals = t
}

func deref(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

// ImportGSCExport reads a Search Console export, given as a .zip file or a
// directory containing the CSV files, and returns a SearchConsole with totals
// computed from the Dates table (CTR and position impression-weighted).
//
// Files are recognized by their name (Queries.csv / Requêtes.csv, Pages.csv,
// Countries.csv / Pays.csv, Devices.csv / Appareils.csv, Dates.csv,
// Search appearance.csv / Apparence dans les résultats de recherche.csv) or,
// failing that, by their header. Filters.csv / Filtres.csv is ignored. Without
// a Dates table, totals are derived from the devices (or countries) table.
func ImportGSCExport(path string) (*model.SearchConsole, error) {
	files, err := readExportFiles(path)
	if err != nil {
		return nil, err
	}
	var sc *model.SearchConsole
	var errs []string
	hasDates := false
	for _, f := range files {
		base := filepath.Base(f.name)
		if isGSCFilterFile(base) {
			continue
		}
		t, err := ParseGSCCSV(f.data, GuessGSCKind(base))
		if err != nil {
			errs = append(errs, base+": "+err.Error())
			continue
		}
		hasDates = hasDates || t.Kind == GSCDates
		sc = ApplyGSCTable(sc, t)
	}
	if sc == nil {
		if len(errs) > 0 {
			return nil, fmt.Errorf("no Search Console table found in %s (%s)", path, strings.Join(errs, "; "))
		}
		return nil, fmt.Errorf("no Search Console CSV file found in %s", path)
	}
	if !hasDates {
		RecomputeGSCTotals(sc)
	}
	return sc, nil
}

type exportFile struct {
	name string
	data []byte
}

const maxCSVSize = 200 << 20 // 200 MB per file

// readExportFiles returns the CSV files of a directory or a zip archive,
// sorted by name.
func readExportFiles(path string) ([]exportFile, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	var files []exportFile
	isCSV := func(name string) bool {
		base := filepath.Base(name)
		return strings.EqualFold(filepath.Ext(base), ".csv") && !strings.HasPrefix(base, "._") &&
			!strings.Contains(name, "__MACOSX")
	}
	switch {
	case fi.IsDir():
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() || !isCSV(e.Name()) {
				continue
			}
			data, err := os.ReadFile(filepath.Join(path, e.Name()))
			if err != nil {
				return nil, err
			}
			files = append(files, exportFile{e.Name(), data})
		}
	case strings.EqualFold(filepath.Ext(path), ".zip"):
		zr, err := zip.OpenReader(path)
		if err != nil {
			return nil, fmt.Errorf("open zip: %w", err)
		}
		defer zr.Close()
		for _, zf := range zr.File {
			if zf.FileInfo().IsDir() || !isCSV(zf.Name) {
				continue
			}
			rc, err := zf.Open()
			if err != nil {
				return nil, err
			}
			data, err := io.ReadAll(io.LimitReader(rc, maxCSVSize))
			rc.Close()
			if err != nil {
				return nil, err
			}
			files = append(files, exportFile{zf.Name, data})
		}
	case strings.EqualFold(filepath.Ext(path), ".csv"):
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		files = append(files, exportFile{filepath.Base(path), data})
	default:
		return nil, fmt.Errorf("%s: expected a .zip file, a .csv file or a directory", path)
	}
	sort.SliceStable(files, func(i, j int) bool { return files[i].name < files[j].name })
	return files, nil
}
