package render

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/flocom/SEO-GEO-Report/internal/charts"
	"github.com/flocom/SEO-GEO-Report/internal/i18n"
	"github.com/flocom/SEO-GEO-Report/internal/insights"
	"github.com/flocom/SEO-GEO-Report/internal/model"
)

//go:embed templates/*.gohtml
var templateFS embed.FS

//go:embed assets/report.css
var reportCSS string

// baseTemplates is parsed once; each render clones it to bind the language.
var baseTemplates = template.Must(template.New("report").Funcs(templateFuncs("fr")).ParseFS(templateFS, "templates/*.gohtml"))

func templateFuncs(lang string) template.FuncMap {
	return template.FuncMap{
		"t":         func(key string) string { return i18n.T(lang, key) },
		"span":      func(n int) string { return fmt.Sprintf("span-%d", n) },
		"icon":      namedIcon,
		"bal":       balFor,
		"add":       func(a, b int) int { return a + b },
		"withStyle": withStyle,
		"kpiPreset": func(tone string) string {
			switch tone {
			case "hero":
				return "kpi-hero"
			case "compact":
				return "kpi-compact"
			}
			return "kpi"
		},
		"statsPreset": func(span int) string {
			if span > 0 && span <= 6 {
				return "stats-half"
			}
			return "stats"
		},
	}
}

// majorSections start on a new printed page (in part 2).
var majorSections = map[string]bool{
	"summary": true, "search_console": true, "analytics": true, "geo": true,
	"technical": true, "actions": true, "glossary": true,
}

func render(src *model.Report, opts Options) (out []byte, err error) {
	if src == nil {
		return nil, errors.New("render: nil report")
	}
	defer func() {
		if p := recover(); p != nil {
			out, err = nil, fmt.Errorf("render: %v", p)
		}
	}()
	r, err := cloneReport(src)
	if err != nil {
		return nil, err
	}
	r.Normalize()
	lang := r.Lang()
	if opts.Lang != "" {
		lang = i18n.Lang(opts.Lang)
	}
	r.Meta.Language = lang

	opt := r.Opt()
	b := &builder{
		r:       r,
		lang:    lang,
		opt:     opt,
		maxRows: opt.MaxTableRows,
		hints:   boolPtr(opt.BeginnerHints),
		auto:    boolPtr(opt.AutoInsights),
		accent:  accentColor(r.Meta.BrandColor),
		an:      safeAnalyze(r),
	}
	if b.an.KPIs == nil {
		b.an.KPIs = map[string]insights.KPIStatus{}
	}

	p := &page{
		Lang:     lang,
		CSS:      template.CSS(reportCSS), //nolint:gosec // embedded asset
		ThemeCSS: themeCSS(b.accent),
		Cover:    b.cover(opts),
	}
	p.Title = p.Cover.Title
	if r.Meta.SiteName != "" {
		p.Title = p.Cover.Title + " · " + r.Meta.SiteName
	}
	p.Generated = b.tf("footer.generated", i18n.FormatTime(lang, time.Now()))
	p.Sections = b.sections()
	for _, s := range p.Sections {
		if s.IsPart {
			p.TOC = append(p.TOC, tocPart{Num: s.PartNum, Title: s.Title})
			continue
		}
		if len(p.TOC) > 0 {
			last := &p.TOC[len(p.TOC)-1]
			last.Items = append(last.Items, tocItem{Anchor: s.Anchor, Num: s.Num, Title: s.Title})
		}
	}
	p.Favicon = p.Cover.Favicon

	tpl, err := baseTemplates.Clone()
	if err != nil {
		return nil, err
	}
	tpl.Funcs(templateFuncs(lang))
	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "page", p); err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	return buf.Bytes(), nil
}

func cloneReport(r *model.Report) (*model.Report, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	var c model.Report
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	return &c, nil
}

// safeAnalyze shields the rendering from a failing analysis.
func safeAnalyze(r *model.Report) (a insights.Analysis) {
	defer func() {
		if recover() != nil {
			a = insights.Analysis{}
		}
	}()
	return insights.Analyze(r)
}

func (b *builder) cover(opts Options) cover {
	m := b.r.Meta
	c := cover{
		Title:       strings.TrimSpace(m.Title),
		SiteName:    strings.TrimSpace(m.SiteName),
		SiteURL:     strings.TrimSpace(m.SiteURL),
		SiteHref:    siteHref(m.SiteURL),
		Logo:        logoURL(m.LogoURL),
		Initials:    initials(m.SiteName),
		Period:      b.dateRange(m.Period),
		PreparedBy:  strings.TrimSpace(m.PreparedBy),
		PreparedFor: strings.TrimSpace(m.PreparedFor),
		PDF:         pdfURL(opts.PDFURL),
		HTMLDL:      pdfURL(opts.HTMLDownloadURL),
		Favicon:     faviconURL(m.FaviconURL),
	}
	if c.Title == "" {
		c.Title = b.t("report.title_default")
	}
	if d := i18n.Days(m.Period.Start, m.Period.End); d > 0 {
		c.PeriodDays = b.plural(float64(d), "cover.days")
	}
	switch {
	case m.ComparisonPeriod != nil && m.ComparisonLabel != "" && strings.ContainsAny(m.ComparisonLabel, "0123456789"):
		c.Comparison = m.ComparisonLabel // the label already carries the dates
	case m.ComparisonPeriod != nil && m.ComparisonLabel != "":
		c.Comparison = m.ComparisonLabel + " (" + b.dateRange(*m.ComparisonPeriod) + ")"
	case m.ComparisonPeriod != nil:
		c.Comparison = b.dateRange(*m.ComparisonPeriod)
	case m.ComparisonLabel != "":
		c.Comparison = m.ComparisonLabel
	}
	if b.r.SearchConsole != nil {
		c.Sources = append(c.Sources, b.t("src.gsc"))
	}
	if b.r.Analytics != nil {
		c.Sources = append(c.Sources, b.t("src.ga4"))
	}
	if b.r.GEO != nil {
		c.Sources = append(c.Sources, b.t("src.geo"))
	}
	return c
}

func pdfURL(s string) template.URL {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	if u.Scheme == "" || u.Scheme == "http" || u.Scheme == "https" {
		return template.URL(u.String()) //nolint:gosec // http(s) or relative only
	}
	return ""
}

var anchorClean = regexp.MustCompile(`[^a-z0-9-]+`)

// Part 1 is the visual, beginner-friendly overview; part 2 the detailed
// analysis. Built-in ids keep their meaning for notes, hide_sections and the
// placement of custom sections.
var (
	part1IDs = []string{"summary", "kpis", idAIGlance, idPlanGlance}
	part2IDs = []string{"search_console", "queries", "pages", "countries_devices", "analytics",
		"channels", "landing_pages", "geo", "technical", "actions", "recommendations", "glossary"}
)

// sections builds both parts: a banner followed by the built-in sections in
// order, with the custom Markdown sections inserted according to After
// ("start", "summary" and "kpis" land in part 1, "end" before the glossary).
func (b *builder) sections() []*section {
	builders := map[string]func() *section{
		"summary": b.summary, "kpis": b.kpis, idAIGlance: b.aiGlance, idPlanGlance: b.planGlance,
		"search_console": b.searchConsole, "queries": b.queries, "pages": b.pages,
		"countries_devices": b.countriesDevices, "analytics": b.analytics, "channels": b.channels,
		"landing_pages": b.landingPages, "geo": b.geo, "technical": b.technical, "actions": b.actions,
		"recommendations": b.recommendations, "glossary": b.glossary,
	}
	known := map[string]bool{"start": true, "end": true}
	for _, id := range model.SectionIDs {
		known[id] = true
	}
	customs := map[string][]*section{}
	for i, ts := range b.r.Sections {
		if strings.TrimSpace(ts.Title) == "" && strings.TrimSpace(ts.Body) == "" {
			continue
		}
		after := strings.ToLower(strings.TrimSpace(ts.After))
		if !known[after] {
			after = "end"
		}
		id := anchorClean.ReplaceAllString(strings.ToLower(ts.ID), "-")
		if id == "" {
			id = fmt.Sprintf("custom-%d", i+1)
		}
		style := strings.ToLower(ts.Style)
		switch style {
		case "info", "success", "warning", "note":
		default:
			style = "plain"
		}
		customs[after] = append(customs[after], &section{
			ID: "custom", Anchor: "sec-c-" + id, Title: ts.Title, Custom: true, Style: style,
			Body: Markdown(ts.Body), Icon: icoText,
		})
	}

	build := func(id string) *section {
		if b.r.SectionHidden(id) {
			return nil
		}
		s := builders[id]()
		if s == nil {
			return nil
		}
		s.Anchor = "sec-" + id
		s.Title = b.t("sec." + id + ".title")
		s.Intro = b.t("sec." + id + ".intro")
		s.Icon = sectionIcon(id)
		s.Major = majorSections[id]
		s.Note = Markdown(b.r.Note(id))
		balance(s.Blocks)
		s.Blocks = drawCharts(s.Blocks)
		return s
	}

	var part1, part2 []*section
	part1 = append(part1, customs["start"]...)
	for _, id := range part1IDs {
		if s := build(id); s != nil {
			part1 = append(part1, s)
		}
		part1 = append(part1, customs[id]...)
	}
	for _, id := range part2IDs {
		if id == "glossary" {
			part2 = append(part2, customs["end"]...)
		}
		if s := build(id); s != nil {
			part2 = append(part2, s)
		}
		part2 = append(part2, customs[id]...)
	}

	var out []*section
	n := 0
	for pi, list := range [][]*section{part1, part2} {
		if len(list) == 0 {
			continue
		}
		num := pi + 1
		banner := &section{
			IsPart: true, Part: num, PartNum: b.tf("part.label", num),
			Anchor: fmt.Sprintf("part-%d", num),
			Title:  b.t(fmt.Sprintf("part.%d.title", num)),
			Intro:  b.t(fmt.Sprintf("part.%d.intro", num)),
			Major:  num == 2,
		}
		out = append(out, banner)
		for i, s := range list {
			n++
			s.Num = twoDigits(n)
			s.Part = num
			if num == 1 {
				s.Major = false // part 1 flows on as few pages as possible
			}
			// A custom section placed right before a major section carries
			// the page break, so that it prints with the section it introduces.
			if i > 0 && s.Major && list[i-1].Custom {
				s.Major, list[i-1].Major = false, true
			}
		}
		list[0].Major = false // the banner already starts the page
		out = append(out, list...)
	}
	if len(out) > 0 {
		out[0].Major = false // the cover ends with a page break
	}
	return out
}

// inlineMarkdown renders a one-paragraph Markdown snippet without the
// enclosing <p>.
func inlineMarkdown(s string) template.HTML {
	h := strings.TrimSpace(string(Markdown(s)))
	if strings.HasPrefix(h, "<p>") && strings.HasSuffix(h, "</p>") && strings.Count(h, "<p>") == 1 {
		h = strings.TrimSuffix(strings.TrimPrefix(h, "<p>"), "</p>")
	}
	return template.HTML(h) //nolint:gosec // sanitized by Markdown
}

// chartSize maps a grid span to the chart viewBox size, so that chart text
// keeps a constant visual size whatever the card width.
func chartSize(span int) (w, h float64) {
	switch {
	case span >= 12:
		return 1100, 300
	case span >= 8:
		return 720, 280
	case span >= 6:
		return 540, 280
	}
	return 360, 220
}

// drawCharts renders the lazy charts at their final size and drops chart
// blocks that ended up empty.
func drawCharts(blocks []block) []block {
	out := blocks[:0]
	for _, bl := range blocks {
		if bl.mk != nil {
			bl.Chart = bl.mk(chartSize(bl.Span))
			if bl.Kind == "chart" {
				// A phone-sized twin keeps chart text readable on small
				// screens (SVG text scales with the viewBox).
				// Twins carry no hover layer: phones have no hover and paper
				// has no mouse, and it keeps the document light.
				bl.ChartSm = charts.Static(bl.mk(400, 280))
				// Half-width cards are narrower in the PDF than on screen:
				// a print twin keeps their text readable on paper.
				if bl.Span == 6 {
					bl.ChartPr = charts.Static(bl.mk(440, 230))
				}
			}
			bl.mk = nil
		}
		if bl.Kind == "chart" && bl.Chart == "" {
			continue
		}
		out = append(out, bl)
	}
	return out
}
