package insights

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/flocom/SEO-GEO-Report/internal/example"
	"github.com/flocom/SEO-GEO-Report/internal/model"
)

func TestDetectAIPlatform(t *testing.T) {
	cases := []struct{ in, want string }{
		{"chatgpt.com", PlatformChatGPT},
		{"ChatGPT.com / referral", PlatformChatGPT},
		{"chat.openai.com", PlatformChatGPT},
		{"openai", PlatformChatGPT},
		{"https://chatgpt.com/", PlatformChatGPT},
		{"utm_source=chatgpt.com", PlatformChatGPT},
		{"https://www.example.com/page?utm_source=chatgpt.com&utm_medium=referral", PlatformChatGPT},
		{"chatgpt", PlatformChatGPT},
		{"perplexity.ai", PlatformPerplexity},
		{"www.perplexity.ai / referral", PlatformPerplexity},
		{"Perplexity", PlatformPerplexity},
		{"gemini.google.com", PlatformGemini},
		{"bard.google.com", PlatformGemini},
		{"gemini", PlatformGemini},
		{"copilot.microsoft.com", PlatformCopilot},
		{"copilot.com", PlatformCopilot},
		{"bing.com/chat", PlatformCopilot},
		{"www.bing.com/chat / referral", PlatformCopilot},
		{"edgeservices.bing.com", PlatformCopilot},
		{"claude.ai", PlatformClaude},
		{"anthropic", PlatformClaude},
		{"chat.mistral.ai", PlatformMistral},
		{"chat.deepseek.com", PlatformDeepSeek},
		{"you.com", PlatformYou},
		{"phind.com", PlatformPhind},
		{"poe.com", PlatformPoe},
		{"meta.ai", PlatformMetaAI},
		{"grok.com", PlatformGrok},
		{"x.ai", PlatformGrok},
		{"kagi.com/assistant", PlatformKagi},
		{"Kagi Assistant", PlatformKagi},
		{"huggingface.co/chat", PlatformHugging},
		{"chat.qwen.ai", PlatformQwen},
		{"kimi.moonshot.cn", PlatformKimi},
		{"duckduckgo.com/aichat", PlatformDuckAI},
		{"notebooklm.google.com", PlatformNotebookLM},
		{"  CLAUDE.AI  ", PlatformClaude},
		// Not AI assistants.
		{"", ""},
		{"google", ""},
		{"google / organic", ""},
		{"bing.com", ""},
		{"bing / organic", ""},
		{"(direct) / (none)", ""},
		{"news.google.com", ""},
		{"facebook.com", ""},
		{"gemini.com", ""},
		{"kagi.com", ""},
		{"x.com", ""},
		{"duckduckgo", ""},
		{"youtube.com", ""},
	}
	for _, c := range cases {
		if got := DetectAIPlatform(c.in); got != c.want {
			t.Errorf("DetectAIPlatform(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatting(t *testing.T) {
	cases := []struct{ got, want string }{
		{FormatNumber(1234567.891, 2, "fr"), "1\u202f234\u202f567,89"},
		{FormatNumber(1234567.891, 2, "en"), "1,234,567.89"},
		{FormatNumber(-1234, 0, "en"), "-1,234"},
		{FormatNumber(-0.001, 1, "en"), "0.0"},
		{FormatNumber(999, 0, "fr"), "999"},
		{FormatAuto(12.0, "fr"), "12"},
		{FormatAuto(12.34, "fr"), "12,3"},
		{FormatAuto(1234.5, "en"), "1,235"},
		{FormatPercent(23.4, "fr"), "23\u202f%"},
		{FormatPercent(3.24, "fr"), "3,2\u202f%"},
		{FormatPercent(3.0, "en"), "3%"},
		{FormatSignedPercent(23.4, "fr"), "+23\u202f%"},
		{FormatSignedPercent(-4.5, "en"), "-4.5%"},
		{FormatSignedPercent(0.01, "en"), "0%"},
		{FormatSigned(340, 0, "fr"), "+340"},
		{FormatSigned(-12.5, 1, "fr"), "-12,5"},
		{FormatPoints(6, "fr"), "+6\u00a0pts"},
		{FormatPoints(-0.5, "en"), "-0.5 pt"},
		{FormatPoints(33.33, "en"), "+33 pts"},
		{ShortURL("https://www.site.fr/a/b?x=1"), "/a/b?x=1"},
		{ShortURL("https://www.site.fr"), "/"},
		{ShortURL("/landing"), "/landing"},
		{NormLang("EN-gb"), "en"},
		{NormLang(""), "fr"},
		{frTypography("Note : « a » ; b ! c ?"), "Note\u00a0: «\u00a0a\u00a0»\u202f; b\u202f! c\u202f?"},
	}
	for i, c := range cases {
		if c.got != c.want {
			t.Errorf("case %d: got %q, want %q", i, c.got, c.want)
		}
	}
}

func TestKPITones(t *testing.T) {
	cases := []struct {
		name        string
		m           model.Metric
		lowerBetter bool
		tone        Tone
		hasDelta    bool
	}{
		{"no previous", model.Metric{Current: 10}, false, Neutral, false},
		{"tiny up", model.M(101.5, 100), false, Neutral, true},
		{"tiny down", model.M(98.5, 100), false, Neutral, true},
		{"up", model.M(103, 100), false, Positive, true},
		{"down", model.M(90, 100), false, Negative, true},
		{"from zero", model.M(5, 0), false, Positive, false},
		{"zero to zero", model.M(0, 0), false, Neutral, false},
		{"position tiny", model.M(10.2, 10.4), true, Neutral, true},
		{"position better", model.M(9.5, 10.4), true, Positive, true},
		{"position worse", model.M(11, 10.4), true, Negative, true},
	}
	for _, c := range cases {
		st := kpiStatus("k", c.m, c.lowerBetter)
		if st.Tone != c.tone || st.HasDelta != c.hasDelta {
			t.Errorf("%s: got tone %s hasDelta %v, want %s %v", c.name, st.Tone, st.HasDelta, c.tone, c.hasDelta)
		}
	}
}

func gscOnly() *model.Report {
	return &model.Report{
		Meta: model.Meta{SiteName: "Test", Language: "en", Period: model.Period{Start: "2026-01-01", End: "2026-01-31"}},
		SearchConsole: &model.SearchConsole{
			Totals: model.GSCTotals{
				Clicks:      model.M(1500, 1000),
				Impressions: model.M(60000, 40000),
				Position:    &model.Metric{Current: 12.1, Previous: model.F(14.0)},
			},
			Queries: []model.GSCRow{
				{Key: "alpha", Clicks: 400, Impressions: 4000, Position: 2.5, PrevClicks: model.F(100), PrevPosition: model.F(5.0)},
				{Key: "beta", Clicks: 300, Impressions: 9000, Position: 8, PrevClicks: model.F(250), PrevPosition: model.F(14)},
				{Key: "gamma", Clicks: 50, Impressions: 3000, Position: 12, PrevClicks: model.F(200), PrevPosition: model.F(6)},
				{Key: "delta", Clicks: 90, Impressions: 1000, Position: 4, PrevClicks: model.F(100), PrevPosition: model.F(4)},
				{Key: "epsilon", Clicks: 80, Impressions: 800, Position: 30},
			},
			Devices: []model.GSCRow{{Key: "MOBILE", Clicks: 1000}, {Key: "DESKTOP", Clicks: 500}},
		},
	}
}

func keys(m map[string]KPIStatus) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func TestAnalyzeOnlyGSC(t *testing.T) {
	r := gscOnly()
	r.Normalize()
	a := Analyze(r)
	want := []string{KeyGSCClicks, KeyGSCCTR, KeyGSCImpressions, KeyGSCPosition}
	if got := keys(a.KPIs); !slices.Equal(got, want) {
		t.Fatalf("KPIs = %v, want %v", got, want)
	}
	if a.KPIs[KeyGSCClicks].Tone != Positive || a.KPIs[KeyGSCPosition].Tone != Positive {
		t.Errorf("tones: %+v", a.KPIs)
	}
	if a.KPIs[KeyGSCCTR].Tone != Neutral { // 2.5 % vs 2.5 %
		t.Errorf("ctr tone = %s", a.KPIs[KeyGSCCTR].Tone)
	}
	if a.Progress == nil || *a.Progress <= 50 || *a.Progress > 100 {
		t.Errorf("progress = %v", a.Progress)
	}
	if a.AISessions != nil || a.AIShare != nil || a.CitationRate != nil || len(a.AIReferrals) != 0 {
		t.Errorf("unexpected GEO data: %+v", a)
	}
	// Winners / losers.
	if len(a.Winners) != 2 || a.Winners[0].Key != "alpha" || a.Winners[1].Key != "beta" {
		t.Errorf("winners = %+v", a.Winners)
	}
	if len(a.Losers) != 2 || a.Losers[0].Key != "gamma" || a.Losers[1].Key != "delta" {
		t.Errorf("losers = %+v", a.Losers)
	}
	// Position distribution computed from queries.
	p := a.Positions
	if p == nil || p.Top3.Current != 1 || p.Top10.Current != 2 || p.Top20.Current != 1 || p.Beyond20.Current != 1 {
		t.Fatalf("positions = %+v", p)
	}
	if p.Top3.Previous == nil || *p.Top3.Previous != 0 || *p.Top10.Previous != 3 || *p.Top20.Previous != 1 {
		t.Errorf("previous positions = %v %v %v", *p.Top3.Previous, *p.Top10.Previous, *p.Top20.Previous)
	}
	texts := joinTexts(a)
	for _, want := range []string{"Clicks from Google grew by 50%", "“alpha” entered Google's top 3", "“beta” reached Google's first page", "Mobile accounts for 67%", "Largest drop: “gamma” lost 150 clicks"} {
		if !strings.Contains(texts, want) {
			t.Errorf("missing insight %q in:\n%s", want, texts)
		}
	}
	for _, in := range a.Insights {
		if in.Section == "geo" || in.Section == "analytics" {
			t.Errorf("unexpected section %s: %s", in.Section, in.Text)
		}
	}
}

func TestAnalyzeOnlyGA4(t *testing.T) {
	r := &model.Report{
		Meta: model.Meta{SiteName: "Test", Language: "fr"},
		Analytics: &model.Analytics{
			Totals: model.GA4Totals{
				Sessions:       &model.Metric{Current: 10000, Previous: model.F(8000)},
				EngagementRate: &model.Metric{Current: 60, Previous: model.F(55)},
				KeyEvents:      &model.Metric{Current: 300, Previous: model.F(200)},
			},
			Channels: []model.ChannelRow{
				{Channel: "Recherche organique", Sessions: 4000, PrevSessions: model.F(3000)},
				{Channel: "Direct", Sessions: 6000, PrevSessions: model.F(5000)},
			},
			Sources: []model.SourceRow{
				{Source: "google", Medium: "organic", Sessions: 3800, PrevSessions: model.F(2900)},
				{Source: "chatgpt.com", Medium: "referral", Sessions: 150, PrevSessions: model.F(50), EngagementRate: 70, KeyEvents: 3},
				{Source: "chat.openai.com", Medium: "referral", Sessions: 50, PrevSessions: model.F(30), EngagementRate: 50, KeyEvents: 1},
				{Source: "perplexity.ai", Medium: "referral", Sessions: 100},
			},
		},
	}
	r.Normalize()
	a := Analyze(r)
	for _, k := range []string{KeyGA4Sessions, KeyGA4EngagementRate, KeyGA4KeyEvents, KeyGA4Organic, KeyGEOAISessions} {
		if _, ok := a.KPIs[k]; !ok {
			t.Errorf("missing KPI %s (got %v)", k, keys(a.KPIs))
		}
	}
	if _, ok := a.KPIs[KeyGSCClicks]; ok {
		t.Error("gsc KPI without search console data")
	}
	if a.KPIs[KeyGA4Organic].DeltaPct < 33.3 || a.KPIs[KeyGA4Organic].DeltaPct > 33.4 {
		t.Errorf("organic delta = %v", a.KPIs[KeyGA4Organic].DeltaPct)
	}
	if len(a.AIReferrals) != 2 {
		t.Fatalf("ai referrals = %+v", a.AIReferrals)
	}
	cg := a.AIReferrals[0]
	if cg.Platform != PlatformChatGPT || cg.Sessions != 200 || cg.PrevSessions == nil || *cg.PrevSessions != 80 ||
		cg.EngagementRate != 65 || cg.KeyEvents != 4 || cg.Source != "chatgpt.com, chat.openai.com" {
		t.Errorf("chatgpt row = %+v", cg)
	}
	if a.AIReferrals[1].Platform != PlatformPerplexity || a.AIReferrals[1].PrevSessions != nil {
		t.Errorf("perplexity row = %+v", a.AIReferrals[1])
	}
	if a.AISessions == nil || a.AISessions.Current != 300 || *a.AISessions.Previous != 80 {
		t.Errorf("ai sessions = %+v", a.AISessions)
	}
	if a.AIShare == nil || *a.AIShare != 3 {
		t.Errorf("ai share = %v", a.AIShare)
	}
	if a.Progress == nil {
		t.Error("progress should exist (organic sessions, key events, AI sessions)")
	}
	texts := joinTexts(a)
	for _, want := range []string{"référencement naturel progresse de 33\u202f%", "portées par ChatGPT", "Le premier canal d'acquisition est «\u00a0Direct\u00a0»"} {
		if !strings.Contains(texts, want) {
			t.Errorf("missing %q in:\n%s", want, texts)
		}
	}
	if a.Positions != nil || a.Winners != nil {
		t.Error("no search console data expected")
	}
}

func TestAnalyzeNoPrevious(t *testing.T) {
	r := example.Demo("en")
	stripPrevious(r)
	r.Normalize()
	a := Analyze(r)
	if a.Progress != nil {
		t.Errorf("progress = %v, want nil", *a.Progress)
	}
	for k, st := range a.KPIs {
		if st.HasDelta || st.Tone != Neutral {
			t.Errorf("%s: %+v", k, st)
		}
	}
	if len(a.Winners) != 0 || len(a.Losers) != 0 {
		t.Error("winners/losers need prev_clicks")
	}
	if len(a.Insights) < 6 {
		t.Errorf("only %d insights without previous data:\n%s", len(a.Insights), joinTexts(a))
	}
	checkInsights(t, a, "en")
}

func stripPrevious(r *model.Report) {
	sc, ga, geo := r.SearchConsole, r.Analytics, r.GEO
	sc.Totals.Clicks.Previous, sc.Totals.Impressions.Previous = nil, nil
	sc.Totals.CTR.Previous, sc.Totals.Position.Previous = nil, nil
	sc.PreviousDaily = nil
	for _, rows := range [][]model.GSCRow{sc.Queries, sc.Pages, sc.Countries, sc.Devices, sc.SearchAppearance} {
		for i := range rows {
			rows[i].PrevClicks, rows[i].PrevImpressions, rows[i].PrevPosition = nil, nil, nil
		}
	}
	sc.BrandSplit.Branded.Previous, sc.BrandSplit.NonBranded.Previous = nil, nil
	sc.PositionDistribution = nil
	sc.Indexing.Indexed.Previous, sc.Indexing.NotIndexed.Previous = nil, nil
	for _, t := range []*model.GA4Totals{&ga.Totals, ga.OrganicTotals} {
		for _, m := range []*model.Metric{t.Sessions, t.Users, t.NewUsers, t.EngagedSessions, t.EngagementRate, t.AvgEngagementTime, t.PageViews, t.KeyEvents, t.Revenue} {
			if m != nil {
				m.Previous = nil
			}
		}
	}
	ga.PreviousDaily = nil
	for i := range ga.Channels {
		ga.Channels[i].PrevSessions = nil
	}
	for i := range ga.Sources {
		ga.Sources[i].PrevSessions = nil
	}
	for i := range ga.LandingPages {
		ga.LandingPages[i].PrevSessions = nil
	}
	for i := range geo.Citations {
		geo.Citations[i].PreviouslyCited = nil
	}
	for i := range geo.ShareOfVoice {
		geo.ShareOfVoice[i].Previous = nil
	}
	for i := range geo.AICrawlers {
		geo.AICrawlers[i].PrevHits = nil
	}
}

func TestAnalyzeDemo(t *testing.T) {
	for _, lang := range []string{"fr", "en"} {
		r := example.Demo(lang)
		r.Normalize()
		a := Analyze(r)
		want := []string{KeyGA4EngagementRate, KeyGA4KeyEvents, KeyGA4Organic, KeyGA4Revenue, KeyGA4Sessions, KeyGA4Users,
			KeyGEOAISessions, KeyGEOCitationRate, KeyGSCClicks, KeyGSCCTR, KeyGSCImpressions, KeyGSCIndexed, KeyGSCPosition}
		if got := keys(a.KPIs); !slices.Equal(got, want) {
			t.Errorf("%s: KPIs = %v", lang, got)
		}
		if a.KPIs[KeyGSCCTR].Tone != Negative || a.KPIs[KeyGSCClicks].Tone != Positive {
			t.Errorf("%s: unexpected tones %+v", lang, a.KPIs)
		}
		if a.Progress == nil || *a.Progress < 60 || *a.Progress > 100 {
			t.Errorf("%s: progress = %v", lang, a.Progress)
		}
		if len(a.AIReferrals) < 5 || a.AIReferrals[0].Platform != PlatformChatGPT {
			t.Errorf("%s: AI referrals = %+v", lang, a.AIReferrals)
		}
		if a.AIShare == nil || *a.AIShare <= 0 || *a.AIShare > 10 {
			t.Errorf("%s: AI share = %v", lang, a.AIShare)
		}
		if a.CitationRate == nil || a.CitationRate.Previous == nil || a.CitationRate.Current <= *a.CitationRate.Previous {
			t.Errorf("%s: citation rate = %+v", lang, a.CitationRate)
		}
		if len(a.Winners) != 5 || len(a.Losers) == 0 {
			t.Errorf("%s: winners %d losers %d", lang, len(a.Winners), len(a.Losers))
		}
		if len(a.Insights) < 6 || len(a.Insights) > 15 {
			t.Errorf("%s: %d insights", lang, len(a.Insights))
		}
		checkInsights(t, a, lang)
		sections := map[string]bool{}
		for _, in := range a.Insights {
			sections[in.Section] = true
		}
		for _, s := range []string{"search_console", "queries", "analytics", "channels", "geo", "technical"} {
			if !sections[s] {
				t.Errorf("%s: no insight for section %s", lang, s)
			}
		}
	}
}

// checkInsights verifies generic properties of insights.
func checkInsights(t *testing.T, a Analysis, lang string) {
	t.Helper()
	for i, in := range a.Insights {
		if i > 0 && in.Weight > a.Insights[i-1].Weight {
			t.Errorf("insights not sorted by weight at %d", i)
		}
		if !slices.Contains(model.SectionIDs, in.Section) {
			t.Errorf("unknown section %q", in.Section)
		}
		if in.Tone != Positive && in.Tone != Negative && in.Tone != Neutral {
			t.Errorf("bad tone %q", in.Tone)
		}
		if strings.Contains(in.Text, "%!") || strings.Contains(in.Text, "NaN") || strings.Contains(in.Text, "Inf") {
			t.Errorf("formatting error: %q", in.Text)
		}
		if in.Text == "" || !strings.HasSuffix(in.Text, ".") {
			t.Errorf("insight must be a sentence: %q", in.Text)
		}
		if lang == "en" && (strings.Contains(in.Text, "\u202f") || strings.Contains(in.Text, "«")) {
			t.Errorf("French typography in English text: %q", in.Text)
		}
		if lang == "fr" && (strings.Contains(in.Text, " :") || strings.Contains(in.Text, " %")) {
			t.Errorf("missing no-break space in French text: %q", in.Text)
		}
	}
}

func joinTexts(a Analysis) string {
	var b strings.Builder
	for _, in := range a.Insights {
		b.WriteString(in.Text)
		b.WriteByte('\n')
	}
	return b.String()
}

func TestAnalyzeEmptyAndPartial(t *testing.T) {
	a := Analyze(nil)
	if a.KPIs == nil || a.Insights == nil || a.Progress != nil {
		t.Errorf("nil report: %+v", a)
	}
	reports := []*model.Report{
		{},
		{SearchConsole: &model.SearchConsole{}},
		{Analytics: &model.Analytics{}},
		{GEO: &model.GEO{}},
		{SearchConsole: &model.SearchConsole{}, Analytics: &model.Analytics{}, GEO: &model.GEO{}, Narrative: &model.Narrative{}},
		{SearchConsole: &model.SearchConsole{
			Totals:        model.GSCTotals{Clicks: model.M(10, 0), Impressions: model.M(0, 0), Position: &model.Metric{}},
			Queries:       []model.GSCRow{{Key: "x", PrevClicks: model.F(0)}, {Key: "y", Position: 3, PrevPosition: model.F(0)}},
			Pages:         []model.GSCRow{{Key: "https://a.b", PrevClicks: model.F(0)}},
			Devices:       []model.GSCRow{{Key: "mobile"}},
			Countries:     []model.GSCRow{{Key: "FR"}, {Key: "BE"}},
			BrandSplit:    &model.BrandSplit{},
			Indexing:      &model.Indexing{Issues: []model.IndexingIssue{{Reason: "404", Pages: 3}}},
			CoreWebVitals: &model.CoreWebVitals{Mobile: &model.CWVStatus{}, Desktop: &model.CWVStatus{Poor: 5}},
		}},
		{Analytics: &model.Analytics{
			Totals:        model.GA4Totals{Sessions: &model.Metric{Current: 0, Previous: model.F(0)}, Revenue: &model.Metric{Current: 5}},
			OrganicTotals: &model.GA4Totals{},
			Channels:      []model.ChannelRow{{Channel: "Organic Search"}},
			Sources:       []model.SourceRow{{Source: "perplexity", Sessions: 0}},
			LandingPages:  []model.LandingPageRow{{Page: "/", PrevSessions: model.F(0)}},
			Daily:         []model.GA4DailyPoint{{Date: "2026-01-01", AISessions: 3}},
		}},
		{GEO: &model.GEO{
			AIReferrals:  []model.AIReferralRow{{Source: "chatgpt.com", Sessions: 0}},
			Citations:    []model.CitationCheck{{Prompt: "p", Engine: "ChatGPT", PreviouslyCited: new(bool)}},
			ShareOfVoice: []model.ShareOfVoice{{Brand: "x"}},
			AIOverviews:  []model.AIOverviewRow{{Query: "q"}},
			AICrawlers:   []model.CrawlerRow{{Bot: "GPTBot"}},
		}},
	}
	for i, r := range reports {
		func() {
			defer func() {
				if p := recover(); p != nil {
					t.Errorf("report %d: panic %v", i, p)
				}
			}()
			a := Analyze(r)
			r.Normalize()
			b := Analyze(r)
			if a.KPIs == nil || b.Insights == nil {
				t.Errorf("report %d: nil maps/slices", i)
			}
			checkInsights(t, b, r.Lang())
		}()
	}
}

func TestProgressIndex(t *testing.T) {
	mk := func(clicks model.Metric) *model.Report {
		return &model.Report{SearchConsole: &model.SearchConsole{Totals: model.GSCTotals{Clicks: clicks, Impressions: model.Metric{Current: 1}}}}
	}
	cases := []struct {
		name string
		r    *model.Report
		want float64
	}{
		{"stable", mk(model.M(100, 100)), 50},
		{"clicks +50%", mk(model.M(150, 100)), round(50+50*math.Tanh(1), 1)},
		{"clicks -50%", mk(model.M(50, 100)), round(50-50*math.Tanh(1), 1)},
	}
	for _, c := range cases {
		a := Analyze(c.r)
		if a.Progress == nil || *a.Progress != c.want {
			t.Errorf("%s: progress = %v, want %v", c.name, a.Progress, c.want)
		}
	}
	// Outliers saturate.
	r := mk(model.M(100000, 1))
	r.SearchConsole.Totals.Position = &model.Metric{Current: 1, Previous: model.F(80)}
	if p := Analyze(r).Progress; p == nil || *p > 100 || *p < 99 {
		t.Errorf("outlier progress = %v", p)
	}
	// Weighted mix: clicks +50 % (w .25) and position worse by 50 % (w .15).
	r = mk(model.M(150, 100))
	r.SearchConsole.Totals.Position = &model.Metric{Current: 15, Previous: model.F(10)}
	want := round(50+50*(0.25*math.Tanh(1)+0.15*math.Tanh(-1))/0.40, 1)
	if p := Analyze(r).Progress; p == nil || *p != want {
		t.Errorf("mixed progress = %v, want %v", p, want)
	}
	if p := Analyze(&model.Report{SearchConsole: &model.SearchConsole{}}).Progress; p != nil {
		t.Errorf("no comparison: progress = %v", *p)
	}
}

func TestCitationRate(t *testing.T) {
	tr, fa := true, false
	checks := []model.CitationCheck{
		{Cited: true, PreviouslyCited: &fa},
		{Cited: true, PreviouslyCited: &tr},
		{Cited: false, PreviouslyCited: &tr},
		{Cited: false},
	}
	m := citationRate(checks)
	if m == nil || m.Current != 50 || m.Previous == nil || math.Abs(*m.Previous-66.67) > 0.01 {
		t.Errorf("citation rate = %+v", m)
	}
	if citationRate(nil) != nil {
		t.Error("nil checks")
	}
	if m := citationRate([]model.CitationCheck{{Cited: true}}); m.Previous != nil || m.Current != 100 {
		t.Errorf("no previous: %+v", m)
	}
}

func TestGEOReferralsProvidedAndFallbacks(t *testing.T) {
	r := &model.Report{
		Analytics: &model.Analytics{Totals: model.GA4Totals{Sessions: &model.Metric{Current: 1000}},
			Sources: []model.SourceRow{{Source: "chatgpt.com", Sessions: 999}}},
		GEO: &model.GEO{AIReferrals: []model.AIReferralRow{
			{Source: "perplexity.ai", Sessions: 10},
			{Platform: "ChatGPT", Sessions: 40, PrevSessions: model.F(20)},
			{Source: "unknown-bot.io", Sessions: 5},
		}},
	}
	a := Analyze(r)
	if len(a.AIReferrals) != 3 || a.AIReferrals[0].Platform != "ChatGPT" || a.AIReferrals[1].Platform != PlatformPerplexity || a.AIReferrals[2].Platform != "unknown-bot.io" {
		t.Errorf("geo referrals must win over sources: %+v", a.AIReferrals)
	}
	if a.AISessions.Current != 55 || *a.AISessions.Previous != 20 || *a.AIShare != 5.5 {
		t.Errorf("ai sessions = %+v share %v", a.AISessions, *a.AIShare)
	}

	// Fallback on geo.ai_daily.
	r = &model.Report{GEO: &model.GEO{AIDaily: []model.DailyValue{{Date: "2026-01-01", Value: 4}, {Date: "2026-01-02", Value: 6}}}}
	if a := Analyze(r); a.AISessions == nil || a.AISessions.Current != 10 || a.AISessions.Previous != nil {
		t.Errorf("ai_daily fallback = %+v", a.AISessions)
	}
	// Fallback on analytics daily series.
	r = &model.Report{Analytics: &model.Analytics{
		Daily:         []model.GA4DailyPoint{{Sessions: 100, AISessions: 6}},
		PreviousDaily: []model.GA4DailyPoint{{Sessions: 90, AISessions: 3}},
	}}
	if a := Analyze(r); a.AISessions == nil || a.AISessions.Current != 6 || *a.AISessions.Previous != 3 || *a.AIShare != 6 {
		t.Errorf("daily fallback = %+v", a.AISessions)
	}
}

func TestAutoInsightsDisabled(t *testing.T) {
	r := example.Demo("fr")
	off := false
	r.Options = &model.Options{AutoInsights: &off}
	a := Analyze(r)
	if len(a.Insights) != 0 || a.Insights == nil {
		t.Errorf("insights = %v", a.Insights)
	}
	if a.Progress == nil || len(a.KPIs) == 0 {
		t.Error("KPIs and progress are still computed")
	}
}

func TestSelectInsightsBalancesSections(t *testing.T) {
	var all []Insight
	for i := 0; i < 20; i++ {
		all = append(all, Insight{Section: "search_console", Weight: float64(100 - i), Text: "x."})
	}
	all = append(all, Insight{Section: "technical", Weight: 1, Text: "t."})
	out := selectInsights(all, 15)
	if len(out) != 15 || out[len(out)-1].Section != "technical" {
		t.Errorf("technical insight must be kept: %+v", out[len(out)-1])
	}
	for i := 1; i < len(out); i++ {
		if out[i].Weight > out[i-1].Weight {
			t.Fatal("not sorted")
		}
	}
}
