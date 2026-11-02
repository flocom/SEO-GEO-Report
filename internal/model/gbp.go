package model

import (
	"fmt"
	"math"
	"strings"
)

// ---------------------------------------------------------------------------
// Google Business Profile (ex Google My Business)

// BusinessProfile holds Google Business Profile data: how people find the
// business card shown in Google Search and Google Maps (address, opening
// hours, reviews) and what they do on it. It maps to the "Performance" page
// of the profile (or its CSV export / the Business Profile Performance API)
// plus the reviews. Every field is optional.
type BusinessProfile struct {
	Locations           []GBPLocation      `json:"locations,omitempty" jsonschema:"One row per establishment when the business has several locations (stores, agencies, showrooms), each with its own totals. The root totals are summed from the locations when omitted. Leave empty for a single location: put its numbers in totals"`
	Totals              GBPTotals          `json:"totals,omitempty" jsonschema:"Totals of the GBP Performance page over the period (all locations). Views are summed from the four view splits and interactions from the actions when omitted; the whole object is computed from locations, else from daily, when empty"`
	Daily               []GBPDailyPoint    `json:"daily,omitempty" jsonschema:"Daily profile views and actions for the analysed period (chronological)"`
	PreviousDaily       []GBPDailyPoint    `json:"previous_daily,omitempty" jsonschema:"Daily profile views and actions for the comparison period (dashed overlay)"`
	SearchKeywords      []GBPKeyword       `json:"search_keywords,omitempty" jsonschema:"Searches that showed the profile (Performance > 'Searches breakdown' / 'Search terms used to find your business'), with the number of searches. GBP shows '<15' for small volumes: send 15 with below_threshold true"`
	Reviews             *GBPReviews        `json:"reviews,omitempty" jsonschema:"Customer reviews: average rating (1 to 5 stars), number of reviews, new reviews, reply rate, star distribution and a few recent reviews"`
	Posts               *Metric            `json:"posts,omitempty" jsonschema:"Number of posts (updates, offers, events) published on the profile over the period"`
	Photos              *Metric            `json:"photos,omitempty" jsonschema:"Number of photos added to the profile over the period (or photo count)"`
	ProfileCompleteness *Metric            `json:"profile_completeness,omitempty" jsonschema:"Profile completeness in percent (85 = 85%), as shown by Google or estimated. Computed from the checklist when omitted"`
	Checklist           []GBPChecklistItem `json:"checklist,omitempty" jsonschema:"Profile checklist: which key information is filled in (opening hours, categories, description, photos, products / services, website, phone, attributes, Q&A...)"`
}

// GBPTotals are the headline metrics of the GBP Performance page. Every
// metric is optional.
type GBPTotals struct {
	Views              *Metric `json:"views,omitempty" jsonschema:"Profile views ('people who viewed your Business Profile'), all platforms. Summed from the four view splits when omitted"`
	ViewsSearchMobile  *Metric `json:"views_search_mobile,omitempty" jsonschema:"Views from Google Search on mobile"`
	ViewsSearchDesktop *Metric `json:"views_search_desktop,omitempty" jsonschema:"Views from Google Search on desktop"`
	ViewsMapsMobile    *Metric `json:"views_maps_mobile,omitempty" jsonschema:"Views from Google Maps on mobile"`
	ViewsMapsDesktop   *Metric `json:"views_maps_desktop,omitempty" jsonschema:"Views from Google Maps on desktop"`
	Interactions       *Metric `json:"interactions,omitempty" jsonschema:"Total interactions (calls, website clicks, direction requests, messages, bookings, food orders...). Summed from the actions when omitted"`
	Calls              *Metric `json:"calls,omitempty" jsonschema:"Calls made from the profile"`
	WebsiteClicks      *Metric `json:"website_clicks,omitempty" jsonschema:"Clicks on the website link"`
	DirectionRequests  *Metric `json:"direction_requests,omitempty" jsonschema:"Direction requests (itinéraires)"`
	Messages           *Metric `json:"messages,omitempty" jsonschema:"Chat conversations started"`
	Bookings           *Metric `json:"bookings,omitempty" jsonschema:"Bookings made from the profile"`
	FoodOrders         *Metric `json:"food_orders,omitempty" jsonschema:"Food orders (restaurants)"`
	MenuClicks         *Metric `json:"menu_clicks,omitempty" jsonschema:"Menu clicks (restaurants)"`
}

// GBPLocation is one establishment of a multi-location business.
type GBPLocation struct {
	Name        string    `json:"name,omitempty" jsonschema:"Business / location name"`
	StoreCode   string    `json:"store_code,omitempty" jsonschema:"Store code of the location in Business Profile"`
	Address     string    `json:"address,omitempty"`
	City        string    `json:"city,omitempty"`
	Totals      GBPTotals `json:"totals,omitempty" jsonschema:"Performance totals of this location"`
	Rating      *float64  `json:"rating,omitempty" jsonschema:"Average rating of this location, 1 to 5"`
	ReviewCount *float64  `json:"review_count,omitempty" jsonschema:"Number of reviews of this location"`
}

// GBPDailyPoint is one day of profile performance.
type GBPDailyPoint struct {
	Date              string  `json:"date" jsonschema:"YYYY-MM-DD"`
	Views             float64 `json:"views,omitempty"`
	Interactions      float64 `json:"interactions,omitempty"`
	Calls             float64 `json:"calls,omitempty"`
	WebsiteClicks     float64 `json:"website_clicks,omitempty"`
	DirectionRequests float64 `json:"direction_requests,omitempty"`
}

// GBPKeyword is one search term that showed the profile.
type GBPKeyword struct {
	Keyword         string   `json:"keyword" jsonschema:"Search term as shown by Google"`
	Impressions     float64  `json:"impressions" jsonschema:"Number of searches (GBP 'Searches'). For '<15' send 15 with below_threshold true"`
	BelowThreshold  bool     `json:"below_threshold,omitempty" jsonschema:"True when GBP only shows a threshold such as '<15'"`
	PrevImpressions *float64 `json:"prev_impressions,omitempty" jsonschema:"Searches in the comparison period"`
}

// GBPReviews summarizes customer reviews.
type GBPReviews struct {
	AverageRating      *Metric                `json:"average_rating,omitempty" jsonschema:"Average rating, 1 to 5 stars (4.6), current and previous. Computed from rating_distribution when omitted"`
	TotalReviews       *Metric                `json:"total_reviews,omitempty" jsonschema:"Total number of reviews at the end of the period (and at the end of the comparison period). Summed from rating_distribution when omitted"`
	NewReviews         *Metric                `json:"new_reviews,omitempty" jsonschema:"Reviews received during the period (and during the comparison period)"`
	ResponseRate       *Metric                `json:"response_rate,omitempty" jsonschema:"Share of reviews that received a reply from the owner, in percent (80 = 80%)"`
	RatingDistribution *GBPRatingDistribution `json:"rating_distribution,omitempty" jsonschema:"Number of reviews per star rating"`
	Recent             []GBPReview            `json:"recent,omitempty" jsonschema:"A few recent reviews, text kept as written by the customer"`
}

// GBPRatingDistribution counts reviews per star rating.
type GBPRatingDistribution struct {
	One   float64 `json:"1,omitempty" jsonschema:"1-star reviews"`
	Two   float64 `json:"2,omitempty" jsonschema:"2-star reviews"`
	Three float64 `json:"3,omitempty" jsonschema:"3-star reviews"`
	Four  float64 `json:"4,omitempty" jsonschema:"4-star reviews"`
	Five  float64 `json:"5,omitempty" jsonschema:"5-star reviews"`
}

// GBPReview is one customer review.
type GBPReview struct {
	Rating    float64 `json:"rating" jsonschema:"Stars, 1 to 5"`
	Text      string  `json:"text,omitempty" jsonschema:"Review text as written by the customer (not translated)"`
	Date      string  `json:"date,omitempty" jsonschema:"YYYY-MM-DD"`
	Author    string  `json:"author,omitempty" jsonschema:"Reviewer name as shown publicly (optional)"`
	Responded bool    `json:"responded,omitempty" jsonschema:"True when the owner replied"`
}

// GBPChecklistItem is one line of the profile checklist.
type GBPChecklistItem struct {
	Item string `json:"item" jsonschema:"Profile information, e.g. 'Opening hours', 'Description', 'Photos', 'Products / services'"`
	Done bool   `json:"done,omitempty" jsonschema:"True when filled in / up to date"`
}

// Counts returns the review counts from 1 to 5 stars.
func (d *GBPRatingDistribution) Counts() [5]float64 {
	if d == nil {
		return [5]float64{}
	}
	return [5]float64{d.One, d.Two, d.Three, d.Four, d.Five}
}

// Total returns the number of reviews of the distribution.
func (d *GBPRatingDistribution) Total() float64 {
	t := 0.0
	for _, v := range d.Counts() {
		t += v
	}
	return t
}

// Average returns the average rating of the distribution (0 when empty).
func (d *GBPRatingDistribution) Average() float64 {
	t, s := 0.0, 0.0
	for i, v := range d.Counts() {
		t += v
		s += v * float64(i+1)
	}
	if t <= 0 {
		return 0
	}
	return s / t
}

// GBPAction is one kind of action on the profile.
type GBPAction struct {
	Key    string // calls, website_clicks, direction_requests, messages, bookings, food_orders, menu_clicks
	Metric *Metric
}

// Actions returns the actions given, in display order.
func (t *GBPTotals) Actions() []GBPAction {
	var out []GBPAction
	for _, a := range []GBPAction{
		{"calls", t.Calls}, {"website_clicks", t.WebsiteClicks}, {"direction_requests", t.DirectionRequests},
		{"messages", t.Messages}, {"bookings", t.Bookings}, {"food_orders", t.FoodOrders}, {"menu_clicks", t.MenuClicks},
	} {
		if a.Metric != nil {
			out = append(out, a)
		}
	}
	return out
}

// ViewSplits returns the four view splits (search mobile, search desktop,
// maps mobile, maps desktop); missing ones are nil.
func (t *GBPTotals) ViewSplits() [4]*Metric {
	return [4]*Metric{t.ViewsSearchMobile, t.ViewsSearchDesktop, t.ViewsMapsMobile, t.ViewsMapsDesktop}
}

// IsEmpty reports whether no metric is given.
func (t *GBPTotals) IsEmpty() bool {
	if t.Views != nil || t.Interactions != nil || len(t.Actions()) > 0 {
		return false
	}
	for _, v := range t.ViewSplits() {
		if v != nil {
			return false
		}
	}
	return true
}

// fill derives views from the splits and interactions from the actions.
func (t *GBPTotals) fill() {
	if t.Views == nil {
		s := t.ViewSplits()
		t.Views = sumMetrics(s[:]...)
	}
	if t.Interactions == nil {
		var ms []*Metric
		for _, a := range t.Actions() {
			ms = append(ms, a.Metric)
		}
		t.Interactions = sumMetrics(ms...)
	}
}

// sumMetrics sums metrics: nil when all are nil; the previous value is set
// when every given metric has one.
func sumMetrics(ms ...*Metric) *Metric {
	var out *Metric
	allPrev := true
	prev := 0.0
	for _, m := range ms {
		if m == nil {
			continue
		}
		if out == nil {
			out = &Metric{}
		}
		out.Current += m.Current
		if m.Previous == nil {
			allPrev = false
		} else {
			prev += *m.Previous
		}
	}
	if out != nil && allPrev {
		out.Previous = F(prev)
	}
	return out
}

// sumTotals sums the totals of several locations field by field.
func sumTotals(locs []GBPLocation) GBPTotals {
	pick := func(f func(*GBPTotals) *Metric) *Metric {
		var ms []*Metric
		for i := range locs {
			if m := f(&locs[i].Totals); m != nil {
				ms = append(ms, m)
			}
		}
		// A field missing for some locations is still summed over the others.
		return sumMetrics(ms...)
	}
	return GBPTotals{
		Views:              pick(func(t *GBPTotals) *Metric { return t.Views }),
		ViewsSearchMobile:  pick(func(t *GBPTotals) *Metric { return t.ViewsSearchMobile }),
		ViewsSearchDesktop: pick(func(t *GBPTotals) *Metric { return t.ViewsSearchDesktop }),
		ViewsMapsMobile:    pick(func(t *GBPTotals) *Metric { return t.ViewsMapsMobile }),
		ViewsMapsDesktop:   pick(func(t *GBPTotals) *Metric { return t.ViewsMapsDesktop }),
		Interactions:       pick(func(t *GBPTotals) *Metric { return t.Interactions }),
		Calls:              pick(func(t *GBPTotals) *Metric { return t.Calls }),
		WebsiteClicks:      pick(func(t *GBPTotals) *Metric { return t.WebsiteClicks }),
		DirectionRequests:  pick(func(t *GBPTotals) *Metric { return t.DirectionRequests }),
		Messages:           pick(func(t *GBPTotals) *Metric { return t.Messages }),
		Bookings:           pick(func(t *GBPTotals) *Metric { return t.Bookings }),
		FoodOrders:         pick(func(t *GBPTotals) *Metric { return t.FoodOrders }),
		MenuClicks:         pick(func(t *GBPTotals) *Metric { return t.MenuClicks }),
	}
}

// dailyTotals computes totals from the daily series (previous values from
// the comparison series).
func dailyTotals(cur, prev []GBPDailyPoint) GBPTotals {
	sum := func(pts []GBPDailyPoint, f func(GBPDailyPoint) float64) (float64, bool) {
		s, seen := 0.0, false
		for _, p := range pts {
			v := f(p)
			s += v
			seen = seen || v != 0
		}
		return s, seen
	}
	field := func(f func(GBPDailyPoint) float64) *Metric {
		c, ok := sum(cur, f)
		if !ok {
			return nil
		}
		m := &Metric{Current: c}
		if p, pok := sum(prev, f); pok {
			m.Previous = F(p)
		}
		return m
	}
	return GBPTotals{
		Views:             field(func(p GBPDailyPoint) float64 { return p.Views }),
		Interactions:      field(func(p GBPDailyPoint) float64 { return p.Interactions }),
		Calls:             field(func(p GBPDailyPoint) float64 { return p.Calls }),
		WebsiteClicks:     field(func(p GBPDailyPoint) float64 { return p.WebsiteClicks }),
		DirectionRequests: field(func(p GBPDailyPoint) float64 { return p.DirectionRequests }),
	}
}

// Normalize fills the derived values: views from the four splits,
// interactions from the actions (per location and at the root), root totals
// from the locations (else from the daily series), review average and count
// from the star distribution, and profile completeness from the checklist.
// It is idempotent.
func (bp *BusinessProfile) Normalize() {
	for i := range bp.Locations {
		bp.Locations[i].Totals.fill()
	}
	if bp.Totals.IsEmpty() {
		switch {
		case len(bp.Locations) > 0:
			bp.Totals = sumTotals(bp.Locations)
		case len(bp.Daily) > 0:
			bp.Totals = dailyTotals(bp.Daily, bp.PreviousDaily)
		}
	}
	bp.Totals.fill()
	if rv := bp.Reviews; rv != nil && rv.RatingDistribution != nil && rv.RatingDistribution.Total() > 0 {
		d := rv.RatingDistribution
		if rv.AverageRating == nil {
			rv.AverageRating = &Metric{Current: math.Round(d.Average()*100) / 100}
		}
		if rv.TotalReviews == nil {
			rv.TotalReviews = &Metric{Current: d.Total()}
		}
	}
	if bp.ProfileCompleteness == nil && len(bp.Checklist) >= 3 {
		done := 0
		for _, c := range bp.Checklist {
			if c.Done {
				done++
			}
		}
		bp.ProfileCompleteness = &Metric{Current: math.Round(float64(done) / float64(len(bp.Checklist)) * 100)}
	}
}

// HasPerformance reports whether the profile holds views or actions (as
// opposed to reviews or the checklist only).
func (bp *BusinessProfile) HasPerformance() bool {
	if bp == nil {
		return false
	}
	t := bp.Totals
	t.fill()
	return !t.IsEmpty() || len(bp.Daily) > 0
}

// IsEmpty reports whether the profile holds no data at all.
func (bp *BusinessProfile) IsEmpty() bool {
	return bp == nil || (!bp.HasPerformance() && len(bp.Locations) == 0 && len(bp.SearchKeywords) == 0 &&
		bp.Reviews == nil && bp.Posts == nil && bp.Photos == nil && bp.ProfileCompleteness == nil && len(bp.Checklist) == 0)
}

func ratingOut(v float64) bool { return v != 0 && (v < 1 || v > 5) }

func (bp *BusinessProfile) warnings() (warnings []string) {
	const p = "business_profile"
	check := func(where string, t GBPTotals) {
		if t.Interactions != nil {
			sum := 0.0
			for _, a := range t.Actions() {
				sum += a.Metric.Current
			}
			if sum > t.Interactions.Current*1.01+1 {
				warnings = append(warnings, fmt.Sprintf("%s: interactions (%g) are lower than the sum of the actions (%g)", where, t.Interactions.Current, sum))
			}
		}
	}
	check(p+".totals", bp.Totals)
	for _, l := range bp.Locations {
		check(fmt.Sprintf("%s.locations %q", p, l.Name), l.Totals)
		if l.Rating != nil && ratingOut(*l.Rating) {
			warnings = append(warnings, fmt.Sprintf("%s.locations %q: rating must be between 1 and 5", p, l.Name))
		}
	}
	t := bp.Totals
	main := t.Views
	if main == nil {
		main = t.Interactions
	}
	if main != nil && main.Current > 0 && !main.HasPrevious() {
		warnings = append(warnings, p+".totals has no previous values: progress cannot be shown")
	}
	if rv := bp.Reviews; rv != nil {
		if m := rv.AverageRating; m != nil && (ratingOut(m.Current) || (m.Previous != nil && ratingOut(*m.Previous))) {
			warnings = append(warnings, p+".reviews.average_rating must be between 1 and 5 stars")
		}
		if m := rv.ResponseRate; m != nil {
			if m.Current > 100 {
				warnings = append(warnings, p+".reviews.response_rate must be in percent, at most 100")
			} else if m.Current > 0 && m.Current <= 1 {
				warnings = append(warnings, p+".reviews.response_rate looks like a ratio: it must be in percent (80 = 80%)")
			}
		}
		if d := rv.RatingDistribution; d != nil && rv.TotalReviews != nil && d.Total() > rv.TotalReviews.Current*1.01+1 {
			warnings = append(warnings, p+".reviews.rating_distribution adds up to more than total_reviews")
		}
		for _, r := range rv.Recent {
			if r.Rating < 1 || r.Rating > 5 {
				warnings = append(warnings, fmt.Sprintf("%s.reviews.recent: rating %g must be between 1 and 5 stars", p, r.Rating))
				break
			}
		}
	}
	if m := bp.ProfileCompleteness; m != nil && (m.Current > 100 || m.Current < 0) {
		warnings = append(warnings, p+".profile_completeness must be a percentage between 0 and 100")
	}
	for _, k := range bp.SearchKeywords {
		if strings.TrimSpace(k.Keyword) == "" {
			warnings = append(warnings, p+".search_keywords: a keyword is empty")
			break
		}
	}
	return warnings
}
