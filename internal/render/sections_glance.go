package render

import (
	"html/template"
	"sort"
	"strings"

	"github.com/flocom/SEO-GEO-Report/internal/charts"
	"github.com/flocom/SEO-GEO-Report/internal/insights"
	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// Sections of part 1 ("at a glance") that have no built-in id of their own:
// they summarise the GEO, actions and recommendations sections and follow
// their hide_sections settings.
const (
	idAIGlance   = "ai_glance"
	idPlanGlance = "plan_glance"
)

// aiGlance is a simple visual of the visibility in AI assistants.
func (b *builder) aiGlance() *section {
	if b.r.SectionHidden("geo") {
		return nil
	}
	s := &section{ID: idAIGlance}
	refs := b.aiReferrals()
	gauge := b.citationRate() != nil && b.r.GEO != nil
	// One or two assistants: a lonely full-width bar looks empty, big numbers
	// tell more.
	if tl := b.aiTiles(refs, gauge); len(refs) <= 2 && tl.Kind != "" {
		span := 12
		if gauge {
			span, tl.Tone = 6, "stack"
		}
		s.Blocks = add(s.Blocks, tl, span, "")
	} else if len(refs) > 0 {
		var items []charts.BarItem
		for i, r := range topN(refs, 6) {
			items = append(items, charts.BarItem{Label: r.Platform, Value: r.Sessions, Previous: r.PrevSessions, Color: b.seriesColor(i)})
		}
		mk := func(w, h float64) template.HTML {
			// The neighbouring citation gauge card is tall: space the rows out
			// so this card does not end with a large empty area.
			return b.bars(charts.BarsOpts{Items: items, Width: w, Height: w * 0.62, Format: b.fmtNum(), ShowDelta: true, Title: b.t("chart.ai_platforms_delta")})
		}
		sub := ""
		if ai := b.aiSessions(); ai != nil {
			sub = b.plural(ai.Current, "glance.ai_visits")
			if share := b.aiShare(); share != nil {
				sub += " · " + b.tf("glance.ai_share", i18nPct2(b, *share))
			}
		}
		s.Blocks = add(s.Blocks, block{Kind: "chart", Title: b.t("chart.ai_platforms_delta"), Sub: sub, mk: mk}, 6, b.hint("ai_platforms_glance"))
	}
	if cr := b.citationRate(); gauge {
		v := cr.Current
		mk := func(w, h float64) template.HTML {
			return b.gauge(charts.GaugeOpts{Value: v, Min: 0, Max: 100, Display: b.pct0(v), Label: b.t("kpi.citation_rate.label"),
				Bands: []charts.GaugeBand{{Upto: 20, Color: charts.ColorNegative}, {Upto: 50, Color: charts.ColorWarning}, {Upto: 100, Color: charts.ColorPositive}},
				Title: b.t("kpi.citation_rate.label")})
		}
		cited := 0
		for _, c := range b.r.GEO.Citations {
			if c.Cited {
				cited++
			}
		}
		stats := []stat{
			{Label: b.t("stat.prompts"), Value: b.num(float64(len(b.r.GEO.Citations)))},
			{Label: b.t("stat.cited"), Value: b.num(float64(cited)), Tone: "pos", Delta: b.deltaPts(*cr)},
		}
		for _, sv := range b.r.GEO.ShareOfVoice {
			if sv.IsSelf {
				m := metricOf(sv.Share, sv.Previous)
				stats = append(stats, stat{Label: b.t("kpi.share_of_voice.label"), Value: b.pct0(sv.Share), Delta: b.deltaPts(m)})
			}
		}
		s.Blocks = add(s.Blocks, block{Kind: "chart", Title: b.t("glance.citations"), Sub: b.t("glance.citations.sub"), Tone: "gauge", mk: mk, Stats: stats}, 6, b.hint("citations_glance"))
	}
	// Share of voice gets its own chart only when there is no citation gauge
	// (which already shows it as a figure): part 1 must stay short.
	if g := b.r.GEO; g != nil && len(g.ShareOfVoice) > 1 && b.citationRate() == nil {
		rows := topN(sortedCopy(g.ShareOfVoice, func(r model.ShareOfVoice) float64 { return r.Share }), 6)
		var items []charts.BarItem
		for _, r := range rows {
			col, label := charts.ColorPrevious, r.Brand
			if r.IsSelf {
				col, label = b.accent, r.Brand+" ("+b.t("badge.you")+")"
			}
			items = append(items, charts.BarItem{Label: label, Value: r.Share, Color: col})
		}
		mk := func(w, h float64) template.HTML {
			return b.bars(charts.BarsOpts{Items: items, Width: w, Format: b.fmtPct(), Title: b.t("chart.sov")})
		}
		s.Blocks = add(s.Blocks, block{Kind: "chart", Title: b.t("chart.sov"), Sub: b.t("chart.sov.sub"), mk: mk}, 6, b.hint("sov"))
	}
	if len(s.Blocks) == 0 {
		return nil
	}
	return s
}

// planGlance lists what was done and what comes next, titles only.
func (b *builder) planGlance() *section {
	n := b.r.Narrative
	if n == nil {
		return nil
	}
	s := &section{ID: idPlanGlance}
	var done, next []planItem
	if !b.r.SectionHidden("actions") {
		acts := append([]model.Action(nil), n.Actions...)
		sort.SliceStable(acts, func(i, j int) bool { return acts[i].Date > acts[j].Date })
		for _, a := range acts {
			st := normStatus(a.Status)
			it := planItem{Title: a.Title, Badges: []badge{{Text: b.t("status." + st), Tone: map[string]string{"done": "pos", "in_progress": "info", "planned": "neu"}[st]}}}
			if a.Date != "" {
				it.Date = b.date(a.Date)
			}
			if c := b.category(a.Category); c != "" {
				it.Badges = append(it.Badges, badge{Text: c, Tone: "tag"})
			}
			switch st {
			case "planned":
				it.Icon, it.Tone = icoCalendar, "neu"
				next = append(next, it)
			case "in_progress":
				it.Icon, it.Tone = icoClock, "info"
				done = append(done, it)
			default:
				it.Icon, it.Tone = icoCheck, "pos"
				done = append(done, it)
			}
		}
	}
	if !b.r.SectionHidden("recommendations") {
		recs := append([]model.Recommendation(nil), n.Recommendations...)
		rank := map[string]int{"high": 0, "medium": 1, "": 1, "low": 2}
		sort.SliceStable(recs, func(i, j int) bool { return rank[normLevel(recs[i].Priority)] < rank[normLevel(recs[j].Priority)] })
		for _, r := range recs {
			it := planItem{Title: r.Title, Icon: icoFlag, Tone: "accent"}
			if p := normLevel(r.Priority); p != "" {
				it.Badges = append(it.Badges, badge{Text: b.t("prio." + p), Tone: map[string]string{"high": "neg", "medium": "warn", "low": "neu"}[p]})
			}
			if e := normLevel(r.Effort); e != "" {
				it.Badges = append(it.Badges, badge{Text: b.t("effort." + e), Tone: "outline"})
			}
			next = append(next, it)
		}
	}
	if len(done) > 0 {
		s.Blocks = add(s.Blocks, block{Kind: "plan", Title: b.t("glance.done"), Tone: "pos", Plan: topN(done, 6)}, 6, "")
	}
	if len(next) > 0 {
		s.Blocks = add(s.Blocks, block{Kind: "plan", Title: b.t("glance.next"), Tone: "accent", Plan: topN(next, 6)}, 6, "")
	}
	if len(s.Blocks) == 0 {
		return nil
	}
	return s
}

// aiTiles are the big-number tiles of the AI section when one or two
// assistants send visits: visits (with their change), share of the traffic
// and, without citation gauge, the citations found in AI answers.
func (b *builder) aiTiles(refs []model.AIReferralRow, gauge bool) block {
	var out []tile
	if ai := b.aiSessions(); ai != nil && (ai.Current > 0 || ai.Previous != nil) {
		t := tile{Icon: icoAI, Big: b.num(ai.Current), Label: b.t("kpi.ai_sessions.label"), Tone: "neu"}
		if ai.Previous != nil {
			t.Tone = b.kpiTone(insights.KeyGEOAISessions, ai, false)
			t.Sign = toneSign[t.Tone]
			t.Text = b.tf("tiles.vs_before", b.num(*ai.Previous))
			if c, ok := b.humanChange(ai.Current, *ai.Previous, b.num); ok {
				t.Text = c.Main + " · " + t.Text
				t.Title = b.changeTitle(c, *ai, b.num)
			}
		}
		var names []string
		for _, r := range topN(refs, 2) {
			if r.Sessions > 0 {
				names = append(names, r.Platform)
			}
		}
		if len(names) > 0 {
			via := b.tf("tiles.via", strings.Join(names, ", "))
			if t.Text != "" {
				t.Text += " · " + via
			} else {
				t.Text = via
			}
		}
		out = append(out, t)
	}
	if share := b.aiShare(); share != nil && len(out) > 0 {
		out = append(out, tile{Icon: icoUsers, Big: i18nPct2(b, *share), Label: b.t("tiles.ai_share"), Text: b.t("tiles.ai_share.sub"), Tone: "neu"})
	}
	if !gauge {
		if g := b.r.GEO; g != nil && len(g.Citations) > 0 {
			cited := 0
			for _, c := range g.Citations {
				if c.Cited {
					cited++
				}
			}
			out = append(out, tile{Icon: icoQuote, Big: b.num(float64(cited)) + " / " + b.num(float64(len(g.Citations))), Label: b.t("tiles.ai_cited"), Text: b.t("tiles.ai_cited.sub"), Tone: "neu"})
		}
	}
	if bg := b.r.Bing; bg != nil && bg.AIPerformance != nil && !b.r.SectionHidden("bing") {
		if m := bg.AIPerformance.Citations; m.Current > 0 {
			t := tile{Icon: icoQuote, Big: b.num(m.Current), Label: b.t("tiles.copilot"), Text: b.t("tiles.copilot.sub"), Tone: "neu"}
			if m.Previous != nil {
				t.Tone = b.kpiTone(insights.KeyBingAICitations, &m, false)
				t.Sign = toneSign[t.Tone]
				if *m.Previous == 0 {
					t.Text = b.t("tiles.copilot.first")
				} else {
					t.Text = b.tf("tiles.vs_before", b.num(*m.Previous))
				}
			}
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return block{}
	}
	return block{Kind: "tiles", Title: b.t("tiles.ai_title"), Tiles: topN(out, 3)}
}
