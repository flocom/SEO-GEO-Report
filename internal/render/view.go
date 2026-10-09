package render

import (
	"html/template"

	"github.com/flocom/SEO-GEO-Report/internal/i18n"
)

// The view model is fully prepared in Go (formatted strings, SVG charts,
// sanitized Markdown) so that templates only lay things out.

type page struct {
	Lang        string
	Title       string
	CSS         template.CSS
	ThemeCSS    template.CSS
	Cover       cover
	TOC         []tocPart
	Favicon     template.URL
	Sections    []*section
	Generated   string
	PrintHeader string
}

type cover struct {
	Title       string
	SiteName    string
	SiteURL     string
	SiteHref    template.URL
	Logo        template.URL
	Initials    string
	Period      string
	PeriodDays  string
	Comparison  string
	PreparedBy  string
	PreparedFor string
	Sources     []string
	PDF         template.URL
	HTMLDL      template.URL
	Favicon     template.URL
}

type tocPart struct {
	Num   string
	Title string
	Items []tocItem
}

type tocItem struct {
	Anchor string
	Num    string
	Title  string
}

type section struct {
	ID     string
	Anchor string
	Num    string
	Title  string
	Intro  string
	Icon   template.HTML
	Note   template.HTML
	Blocks []block
	Major  bool // starts a new printed page

	// Part banners ("At a glance" / "Detailed analysis").
	IsPart  bool
	PartNum string
	Part    int

	// Custom Markdown sections.
	Custom bool
	Style  string
	Body   template.HTML
}

// LeadBlocks are the blocks printed on the same page as the section header:
// the leading insight callouts and the first grid row. Tables, timelines and
// other long blocks may break across pages, so they never join the lead.
func (s *section) LeadBlocks() []block { return s.Blocks[:s.leadLen()] }

// RestBlocks are the blocks that follow the lead.
func (s *section) RestBlocks() []block { return s.Blocks[s.leadLen():] }

func (s *section) leadLen() int {
	i := 0
	for i < len(s.Blocks) && s.Blocks[i].Kind == "callouts" {
		i++
	}
	span := 0
	for i < len(s.Blocks) {
		bl := s.Blocks[i]
		switch bl.Kind {
		case "table", "timeline", "glossary", "recs", "markdown":
			return i
		}
		sp := bl.Span
		if sp <= 0 || sp > 12 {
			sp = 12
		}
		if span+sp > 12 {
			return i
		}
		span += sp
		i++
		if span == 12 {
			return i
		}
	}
	return i
}

// block is one tile of a section grid. Kind selects the template.
type block struct {
	Kind     string // hero, kpis, chart, table, callouts, markdown, list, stats, timeline, recs, glossary
	Span     int    // grid columns out of 12
	Title    string
	Sub      string
	Hint     string // beginner explanation ("how to read")
	HintKind string // "chart" or "table"
	Tone     string

	Chart    template.HTML
	ChartSm  template.HTML                    // phone-sized variant of full-width charts
	ChartPr  template.HTML                    // print variant of half-width charts
	mk       func(w, h float64) template.HTML // lazy chart, sized once the grid span is final
	Table    *table
	KPIs     []kpiCard
	Callouts []callout
	HTML     template.HTML
	Items    []listItem
	Stats    []stat
	Hero     *hero
	Actions  []actionView
	ActSum   []stat
	Recs     []recView
	Glossary []i18n.GlossaryEntry
	Legend   []legendItem
	Plan     []planItem
}

// planItem is one line of the "what we did / what's next" summary.
type planItem struct {
	Title  string
	Date   string
	Icon   template.HTML
	Tone   string
	Badges []badge
}

type legendItem struct {
	Label string
	Color template.CSS
}

type hero struct {
	Gauge     template.HTML
	Score     string
	Verdict   string
	Desc      string
	Tone      string
	Scale     string
	Period    string
	Stats     []kpiCard
	NoCompare string
}

type deltaView struct {
	Text  string // main text, e.g. "+12,3 %"
	Sub   string // secondary text, e.g. "+134"
	Tone  string // pos, neg, neu
	Dir   string // up, down, flat
	Arrow template.HTML
}

type kpiCard struct {
	Key     string
	Label   string
	Value   string
	Prev    string
	Explain string
	Delta   *deltaView
	Spark   template.HTML
	Tone    string
	Icon    template.HTML
}

type callout struct {
	Tone  string // pos, neg, neu, info, warn
	Title string
	Text  string
	HTML  template.HTML
	Icon  template.HTML
}

type listItem struct {
	Text string
	HTML template.HTML
}

type stat struct {
	Label string
	Value string
	Sub   string
	Tone  string
	Delta *deltaView
}

type table struct {
	Columns   []column
	Rows      []row
	Truncated string
	Compact   bool
}

type column struct {
	Label string
	Num   bool
	Width string // optional CSS class suffix: w-key, w-wide
}

type row struct {
	Cells     []cell
	Highlight bool
}

type cell struct {
	Text     string
	Sub      string
	Title    string // full text in a tooltip when Text is shortened
	Num      bool
	Key      bool // first, descriptive column
	Mono     bool
	Bar      template.CSS // width style of the inline bar
	BarTone  string
	Delta    *deltaView
	Badges   []badge
	Rank     int
	Muted    bool
	Wrap     bool
	Children template.HTML
}

type badge struct {
	Text string
	Tone string // pos, neg, neu, info, warn, accent
	Icon template.HTML
}

type actionView struct {
	Date        string
	Title       string
	Description template.HTML
	Status      string
	StatusLabel string
	Category    string
	Impact      string
	Icon        template.HTML
}

type recView struct {
	Num         string
	Title       string
	Description template.HTML
	Priority    string
	PrioLabel   string
	Effort      string
	EffortLabel string
	Category    string
}
