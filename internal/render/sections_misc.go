package render

import (
	"html/template"
	"sort"
	"strings"

	"github.com/flocom/SEO-GEO-Report/internal/charts"
	"github.com/flocom/SEO-GEO-Report/internal/i18n"
	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// ---------------------------------------------------------------------------
// Technical: indexing + Core Web Vitals

func (b *builder) technical() *section {
	sc := b.r.SearchConsole
	if sc == nil || (sc.Indexing == nil && sc.CoreWebVitals == nil) {
		return nil
	}
	s := &section{ID: "technical"}
	s.Blocks = append(s.Blocks, b.sectionInsights("technical")...)

	if ix := sc.Indexing; ix != nil {
		cards := []kpiCard{b.card("gsc.indexed", "indexed", b.num(ix.Indexed.Current), &ix.Indexed, b.num, b.deltaRel(ix.Indexed, false, b.num))}
		if ix.NotIndexed != nil {
			cards = append(cards, b.card("gsc.not_indexed", "not_indexed", b.num(ix.NotIndexed.Current), ix.NotIndexed, b.num, b.deltaRel(*ix.NotIndexed, true, b.num)))
			total := ix.Indexed.Current + ix.NotIndexed.Current
			if total > 0 {
				c := b.card("gsc.index_rate", "index_rate", b.pct0(ix.Indexed.Current/total*100), nil, nil, nil)
				if ix.Indexed.Previous != nil && ix.NotIndexed.Previous != nil && *ix.Indexed.Previous+*ix.NotIndexed.Previous > 0 {
					m := model.Metric{Current: ix.Indexed.Current / total * 100, Previous: model.F(*ix.Indexed.Previous / (*ix.Indexed.Previous + *ix.NotIndexed.Previous) * 100)}
					c.Delta = b.deltaPts(m)
					c.Tone = c.Delta.Tone
					c.Prev = b.tf("kpi.prev", b.pct0(*m.Previous))
				}
				cards = append(cards, c)
			}
		}
		s.Blocks = append(s.Blocks, block{Kind: "kpis", Span: 12, Title: b.t("tech.indexing"), KPIs: cards})

		if ix.NotIndexed != nil && ix.Indexed.Current+ix.NotIndexed.Current > 0 {
			total := ix.Indexed.Current + ix.NotIndexed.Current
			svg := func(w, h float64) template.HTML {
				return b.donut(charts.DonutOpts{Slices: []charts.Slice{
					{Label: b.t("lbl.indexed"), Value: ix.Indexed.Current, Color: charts.ColorPositive},
					{Label: b.t("lbl.not_indexed"), Value: ix.NotIndexed.Current, Color: "#cbd5e1"},
				}, CenterValue: b.pct0(ix.Indexed.Current / total * 100), CenterLabel: b.t("lbl.indexed"), Format: b.fmtNum(), Title: b.t("chart.indexing")})
			}
			s.Blocks = add(s.Blocks, block{Kind: "chart", Title: b.t("chart.indexing"), mk: svg}, 6, b.hint("indexing"))
		}
		if len(ix.Issues) > 0 {
			rows := sortedCopy(ix.Issues, func(r model.IndexingIssue) float64 { return r.Pages })
			var items []charts.BarItem
			for _, is := range topN(rows, 8) {
				items = append(items, charts.BarItem{Label: truncate(is.Reason, 60), Value: is.Pages, Color: "#f59e0b"})
			}
			svg := func(w, h float64) template.HTML {
				return b.bars(charts.BarsOpts{Width: w, Items: items, Format: b.fmtNum(), Title: b.t("chart.index_issues")})
			}
			s.Blocks = add(s.Blocks, block{Kind: "chart", Title: b.t("chart.index_issues"), mk: svg}, 6, b.hint("index_issues"))
			balance(s.Blocks)
			t := &table{Columns: []column{{Label: b.t("col.reason"), Width: "wide"}, {Label: b.t("col.pages"), Num: true, Width: "bar"}, {Label: b.t("col.share"), Num: true}}}
			total, max := 0.0, 0.0
			for _, r := range rows {
				total += r.Pages
				max = maxOf(max, r.Pages)
			}
			for _, r := range rows {
				t.Rows = append(t.Rows, row{Cells: []cell{
					{Text: r.Reason, Key: true, Wrap: true},
					{Text: b.num(r.Pages), Num: true, Bar: barStyle(r.Pages, max), BarTone: "warn"},
					{Text: b.pct0(r.Pages / total * 100), Num: true},
				}})
			}
			s.Blocks = add(s.Blocks, block{Kind: "table", Title: b.t("table.index_issues"), Table: b.limit(t, len(rows))}, 12, "")
		}
		balance(s.Blocks)
		if len(ix.Sitemaps) > 0 {
			t := &table{Compact: true, Columns: []column{{Label: b.t("col.sitemap"), Width: "wide"}, {Label: b.t("col.discovered"), Num: true}, {Label: b.t("col.status")}}}
			for _, sm := range ix.Sitemaps {
				st := cell{Text: "–", Muted: true}
				if sm.Status != "" {
					tone := "neu"
					l := strings.ToLower(sm.Status)
					if strings.Contains(l, "succ") || strings.Contains(l, "ok") || strings.Contains(l, "effectu") || strings.Contains(l, "réussi") {
						tone = "pos"
					} else if strings.Contains(l, "err") || strings.Contains(l, "fail") || strings.Contains(l, "échec") || strings.Contains(l, "impossible") {
						tone = "neg"
					}
					st = cell{Badges: []badge{{Text: sm.Status, Tone: tone}}}
				}
				t.Rows = append(t.Rows, row{Cells: []cell{{Text: sm.URL, Key: true, Wrap: true, Mono: true}, {Text: b.num(sm.Discovered), Num: true}, st}})
			}
			s.Blocks = add(s.Blocks, block{Kind: "table", Title: b.t("table.sitemaps"), Table: t}, 12, "")
		}
	}

	if cwv := sc.CoreWebVitals; cwv != nil {
		for _, d := range []struct {
			key string
			st  *model.CWVStatus
		}{{"mobile", cwv.Mobile}, {"desktop", cwv.Desktop}} {
			if d.st == nil || d.st.Good+d.st.NeedsImprovement+d.st.Poor == 0 {
				continue
			}
			total := d.st.Good + d.st.NeedsImprovement + d.st.Poor
			svg := func(w, h float64) template.HTML {
				return b.stacked(charts.StackedOpts{Width: w, Segments: []charts.StackSegment{
					{Label: b.t("lbl.good"), Value: d.st.Good, Color: charts.ColorPositive},
					{Label: b.t("lbl.ni"), Value: d.st.NeedsImprovement, Color: "#f59e0b"},
					{Label: b.t("lbl.poor"), Value: d.st.Poor, Color: charts.ColorNegative},
				}, Format: b.fmtNum(), Title: b.t("chart.cwv_" + d.key)})
			}
			tone := "pos"
			goodPct := d.st.Good / total * 100
			if goodPct < 50 || d.st.Poor/total > 0.25 {
				tone = "neg"
			} else if goodPct < 75 {
				tone = "warn"
			}
			stats := []stat{
				{Label: b.t("lbl.good"), Value: b.num(d.st.Good), Sub: b.pct0(goodPct), Tone: "pos"},
				{Label: b.t("lbl.ni"), Value: b.num(d.st.NeedsImprovement), Sub: b.pct0(d.st.NeedsImprovement / total * 100), Tone: "warn"},
				{Label: b.t("lbl.poor"), Value: b.num(d.st.Poor), Sub: b.pct0(d.st.Poor / total * 100), Tone: "neg"},
			}
			bl := block{Kind: "chart", Title: b.t("chart.cwv_" + d.key), Sub: b.tf("chart.cwv.sub", b.pct0(goodPct)), Tone: tone, mk: svg, Stats: stats}
			s.Blocks = add(s.Blocks, bl, 6, b.hint("cwv"))
		}
	}
	if len(s.Blocks) == 0 {
		return nil
	}
	return s
}

// ---------------------------------------------------------------------------
// Actions timeline

func normStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(strings.ReplaceAll(s, " ", "_"))) {
	case "in_progress", "in-progress", "progress", "ongoing", "en_cours", "wip":
		return "in_progress"
	case "planned", "todo", "to_do", "prévu", "prevu", "planifié", "next":
		return "planned"
	}
	return "done"
}

func (b *builder) category(c string) string {
	c = strings.ToLower(strings.TrimSpace(c))
	if c == "" {
		return ""
	}
	if i18n.Has("cat." + c) {
		return b.t("cat." + c)
	}
	return c
}

func (b *builder) actions() *section {
	n := b.r.Narrative
	if n == nil || len(n.Actions) == 0 {
		return nil
	}
	s := &section{ID: "actions"}
	acts := append([]model.Action(nil), n.Actions...)
	rank := map[string]int{"done": 0, "in_progress": 1, "planned": 2}
	sort.SliceStable(acts, func(i, j int) bool {
		si, sj := rank[normStatus(acts[i].Status)], rank[normStatus(acts[j].Status)]
		if si != sj {
			return si < sj
		}
		return acts[i].Date < acts[j].Date
	})
	counts := map[string]int{}
	var views []actionView
	for _, a := range acts {
		st := normStatus(a.Status)
		counts[st]++
		v := actionView{Title: a.Title, Description: Markdown(a.Description), Status: st, StatusLabel: b.t("status." + st), Category: b.category(a.Category), Impact: a.Impact}
		if a.Date != "" {
			v.Date = b.date(a.Date)
		}
		switch st {
		case "done":
			v.Icon = icoCheck
		case "in_progress":
			v.Icon = icoClock
		default:
			v.Icon = icoCalendar
		}
		views = append(views, v)
	}
	var sum []stat
	for _, st := range []string{"done", "in_progress", "planned"} {
		if counts[st] > 0 {
			tone := map[string]string{"done": "pos", "in_progress": "info", "planned": "neu"}[st]
			sum = append(sum, stat{Label: b.t("status." + st), Value: b.num(float64(counts[st])), Tone: tone})
		}
	}
	s.Blocks = append(s.Blocks, block{Kind: "timeline", Span: 12, Actions: views, ActSum: sum})
	return s
}

// ---------------------------------------------------------------------------
// Recommendations

func normLevel(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "high", "haute", "élevée", "elevee", "élevé", "eleve", "fort", "forte":
		return "high"
	case "low", "basse", "faible", "bas":
		return "low"
	case "":
		return ""
	}
	return "medium"
}

func (b *builder) recommendations() *section {
	n := b.r.Narrative
	if n == nil || len(n.Recommendations) == 0 {
		return nil
	}
	s := &section{ID: "recommendations"}
	recs := append([]model.Recommendation(nil), n.Recommendations...)
	rank := map[string]int{"high": 0, "medium": 1, "": 1, "low": 2}
	sort.SliceStable(recs, func(i, j int) bool { return rank[normLevel(recs[i].Priority)] < rank[normLevel(recs[j].Priority)] })
	var views []recView
	for i, r := range recs {
		v := recView{Num: twoDigits(i + 1), Title: r.Title, Description: Markdown(r.Description), Category: b.category(r.Category)}
		if p := normLevel(r.Priority); p != "" {
			v.Priority, v.PrioLabel = p, b.t("prio."+p)
		}
		if e := normLevel(r.Effort); e != "" {
			v.Effort, v.EffortLabel = e, b.t("effort."+e)
		}
		views = append(views, v)
	}
	s.Blocks = append(s.Blocks, block{Kind: "recs", Span: 12, Recs: views})
	return s
}

func twoDigits(i int) string {
	if i < 10 {
		return "0" + itoa(i)
	}
	return itoa(i)
}

// ---------------------------------------------------------------------------
// Glossary

func (b *builder) glossary() *section {
	if !boolPtr(b.opt.ShowGlossary) {
		return nil
	}
	return &section{ID: "glossary", Blocks: []block{{Kind: "glossary", Span: 12, Glossary: i18n.Glossary(b.lang)}}}
}
