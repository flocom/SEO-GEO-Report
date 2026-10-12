package model

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// F returns a pointer to v. Handy for optional values.
func F(v float64) *float64 { return &v }

// M builds a Metric with a previous value.
func M(current, previous float64) Metric { return Metric{Current: current, Previous: &previous} }

// HasPrevious reports whether a comparison value exists.
func (m Metric) HasPrevious() bool { return m.Previous != nil }

// Delta returns current - previous, and false when there is no comparison.
func (m Metric) Delta() (float64, bool) {
	if m.Previous == nil {
		return 0, false
	}
	return m.Current - *m.Previous, true
}

// DeltaPct returns the relative change in percent, and false when it cannot be
// computed (no comparison or previous value equal to zero).
func (m Metric) DeltaPct() (float64, bool) {
	if m.Previous == nil || *m.Previous == 0 {
		return 0, false
	}
	return (m.Current - *m.Previous) / math.Abs(*m.Previous) * 100, true
}

// Lang returns the normalized report language ("fr" or "en").
func (r *Report) Lang() string {
	l := strings.ToLower(strings.TrimSpace(r.Meta.Language))
	if strings.HasPrefix(l, "en") {
		return "en"
	}
	return "fr"
}

// Opt returns options with defaults applied (never nil).
func (r *Report) Opt() Options {
	o := Options{}
	if r.Options != nil {
		o = *r.Options
	}
	t := true
	if o.ShowGlossary == nil {
		o.ShowGlossary = &t
	}
	if o.BeginnerHints == nil {
		o.BeginnerHints = &t
	}
	if o.AutoInsights == nil {
		o.AutoInsights = &t
	}
	if o.MaxTableRows <= 0 {
		o.MaxTableRows = 15
	}
	return o
}

// SectionHidden reports whether a built-in section is hidden.
func (r *Report) SectionHidden(id string) bool {
	if r.Options == nil {
		return false
	}
	for _, h := range r.Options.HideSections {
		if strings.EqualFold(h, id) {
			return true
		}
	}
	return false
}

// Note returns the analyst note attached to a built-in section, if any.
func (r *Report) Note(section string) string {
	if r.Narrative == nil || r.Narrative.SectionNotes == nil {
		return ""
	}
	return r.Narrative.SectionNotes[section]
}

// Normalize fills derived values in place: language, currency, CTRs computed
// from clicks/impressions, section ids and defaults. It is idempotent.
func (r *Report) Normalize() {
	r.Meta.Language = r.Lang()
	if r.Meta.Currency == "" {
		r.Meta.Currency = "EUR"
	}
	if sc := r.SearchConsole; sc != nil {
		fillCTR(&sc.Totals, [][]GSCRow{sc.Queries, sc.Pages, sc.Countries, sc.Devices, sc.SearchAppearance}, sc.Daily, sc.PreviousDaily)
	}
	if bg := r.Bing; bg != nil {
		fillCTR(&bg.Totals, [][]GSCRow{bg.Queries, bg.Pages}, bg.Daily, bg.PreviousDaily)
		if ai := bg.AIPerformance; ai != nil {
			ai.ComputeTotals()
		}
	}
	for i := range r.Sections {
		s := &r.Sections[i]
		if s.ID == "" {
			s.ID = fmt.Sprintf("custom-%d", i+1)
		}
		if s.After == "" {
			s.After = "end"
		}
		if s.Style == "" {
			s.Style = "plain"
		}
	}
	if r.Narrative != nil {
		for i := range r.Narrative.Actions {
			if r.Narrative.Actions[i].Status == "" {
				r.Narrative.Actions[i].Status = "done"
			}
		}
	}
}

// fillCTR computes the missing CTRs of search performance data from clicks
// and impressions.
func fillCTR(t *GSCTotals, tables [][]GSCRow, daily ...[]GSCDailyPoint) {
	if t.CTR == nil && t.Impressions.Current > 0 {
		ctr := Metric{Current: t.Clicks.Current / t.Impressions.Current * 100}
		if t.Clicks.Previous != nil && t.Impressions.Previous != nil && *t.Impressions.Previous > 0 {
			ctr.Previous = F(*t.Clicks.Previous / *t.Impressions.Previous * 100)
		}
		t.CTR = &ctr
	}
	for _, rows := range tables {
		for i := range rows {
			if rows[i].CTR == 0 && rows[i].Impressions > 0 {
				rows[i].CTR = rows[i].Clicks / rows[i].Impressions * 100
			}
		}
	}
	for _, pts := range daily {
		for i := range pts {
			if pts[i].CTR == 0 && pts[i].Impressions > 0 {
				pts[i].CTR = pts[i].Clicks / pts[i].Impressions * 100
			}
		}
	}
}

// ComputeTotals computes the total citations when omitted: from the daily
// series, or else from the cited pages (every citation points to one page).
// The average cited pages is computed from the daily series when omitted.
func (ai *BingAIPerformance) ComputeTotals() {
	sum := func(pts []BingAIDailyPoint) (citations, pages float64) {
		for _, p := range pts {
			citations += p.Citations
			pages += p.CitedPages
		}
		return citations, pages
	}
	c, pg := sum(ai.Daily)
	pc, ppg := sum(ai.PreviousDaily)
	if ai.Citations.Current == 0 && ai.Citations.Previous == nil {
		switch {
		case len(ai.Daily) > 0:
			ai.Citations.Current = c
			if len(ai.PreviousDaily) > 0 {
				ai.Citations.Previous = F(pc)
			}
		case len(ai.Pages) > 0:
			var cur, prev float64
			hasPrev := false
			for _, p := range ai.Pages {
				cur += p.Citations
				if p.PrevCitations != nil {
					prev += *p.PrevCitations
					hasPrev = true
				}
			}
			ai.Citations.Current = cur
			if hasPrev {
				ai.Citations.Previous = F(prev)
			}
		}
	}
	if ai.CitedPages == nil && len(ai.Daily) > 0 && pg > 0 {
		m := Metric{Current: math.Round(pg/float64(len(ai.Daily))*10) / 10}
		if len(ai.PreviousDaily) > 0 && ppg > 0 {
			m.Previous = F(math.Round(ppg/float64(len(ai.PreviousDaily))*10) / 10)
		}
		ai.CitedPages = &m
	}
}

// Validate returns blocking errors and non-blocking warnings.
func (r *Report) Validate() (errs []string, warnings []string) {
	if strings.TrimSpace(r.Meta.SiteName) == "" {
		errs = append(errs, "meta.site_name is required")
	}
	start, err1 := time.Parse("2006-01-02", r.Meta.Period.Start)
	end, err2 := time.Parse("2006-01-02", r.Meta.Period.End)
	if err1 != nil || err2 != nil {
		errs = append(errs, "meta.period.start and meta.period.end must be YYYY-MM-DD dates")
	} else if end.Before(start) {
		errs = append(errs, "meta.period.end is before meta.period.start")
	}
	if l := strings.ToLower(r.Meta.Language); l != "" && !strings.HasPrefix(l, "fr") && !strings.HasPrefix(l, "en") {
		warnings = append(warnings, fmt.Sprintf("meta.language %q is not supported, French is used", r.Meta.Language))
	}
	if r.SearchConsole == nil && r.Bing == nil && r.Analytics == nil && r.GEO == nil {
		warnings = append(warnings, "no data provided: add search_console, bing, analytics and/or geo")
	}
	if sc := r.SearchConsole; sc != nil {
		warnings = append(warnings, searchWarnings("search_console", sc.Totals)...)
	}
	if bg := r.Bing; bg != nil {
		if bg.Totals.Clicks.Current > 0 || bg.Totals.Impressions.Current > 0 {
			warnings = append(warnings, searchWarnings("bing", bg.Totals)...)
		}
		if ai := bg.AIPerformance; ai != nil {
			// Shares sent as ratios are all at most 1; real percentages rarely are.
			n, top := 0, 0.0
			for _, q := range ai.GroundingQueries {
				if q.CitationShare > 0 {
					n++
					top = math.Max(top, q.CitationShare)
				}
			}
			if n >= 2 && top <= 1 {
				warnings = append(warnings, "bing.ai_performance.grounding_queries[].citation_share looks like a ratio: it must be in percent (12.5 = 12.5%)")
			}
		}
	}
	if a := r.Analytics; a != nil && a.Totals.EngagementRate != nil && a.Totals.EngagementRate.Current > 0 && a.Totals.EngagementRate.Current <= 1 {
		warnings = append(warnings, "analytics.totals.engagement_rate looks like a ratio: it must be in percent (62.5 = 62.5%)")
	}
	for _, s := range r.Sections {
		if s.After != "" && s.After != "start" && s.After != "end" && !isSectionID(s.After) {
			warnings = append(warnings, fmt.Sprintf("section %q: unknown 'after' value %q, placed at the end", s.Title, s.After))
		}
	}
	return errs, warnings
}

// searchWarnings checks search performance totals (Search Console or Bing).
func searchWarnings(prefix string, t GSCTotals) (warnings []string) {
	if t.Clicks.Current > t.Impressions.Current && t.Impressions.Current > 0 {
		warnings = append(warnings, prefix+".totals: clicks are greater than impressions")
	}
	if t.CTR != nil && t.CTR.Current > 0 && t.CTR.Current < 1 &&
		t.Impressions.Current > 0 && t.Clicks.Current/t.Impressions.Current*100 > 1 {
		warnings = append(warnings, prefix+".totals.ctr looks like a ratio: CTR must be in percent (3.2 = 3.2%)")
	}
	if !t.Clicks.HasPrevious() {
		warnings = append(warnings, prefix+".totals has no previous values: progress cannot be shown")
	}
	return warnings
}

func isSectionID(id string) bool {
	for _, s := range SectionIDs {
		if s == id {
			return true
		}
	}
	return false
}
