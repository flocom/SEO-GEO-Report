// Package importer converts raw exports (CSV files from Google Search Console,
// Bing Webmaster Tools and Google Analytics 4, in French or English) into
// model structures.
package importer

import "github.com/flocom/SEO-GEO-Report/internal/model"

// Implementations live in gsc.go (ParseGSCCSV, ImportGSCExport,
// ApplyGSCTable), bing.go (ParseBingCSV, ImportBingExport, ApplyBingTable)
// and ga4.go (ParseGA4CSV).

// GSCKind is the kind of Search Console export table.
type GSCKind string

const (
	GSCQueries    GSCKind = "queries"    // Requêtes.csv / Queries.csv
	GSCPages      GSCKind = "pages"      // Pages.csv
	GSCCountries  GSCKind = "countries"  // Pays.csv / Countries.csv
	GSCDevices    GSCKind = "devices"    // Appareils.csv / Devices.csv
	GSCDates      GSCKind = "dates"      // Dates.csv
	GSCAppearance GSCKind = "appearance" // Apparence dans les résultats de recherche.csv / Search appearance.csv
)

// GSCTable is the parsed content of one Search Console CSV. Rows or Daily is
// filled depending on the kind. When the export contains a comparison (two
// columns per metric), Prev* fields / PreviousDaily are filled.
type GSCTable struct {
	Kind          GSCKind
	Rows          []model.GSCRow
	Daily         []model.GSCDailyPoint
	PreviousDaily []model.GSCDailyPoint
}

// BingKind is the kind of Bing Webmaster Tools export table.
type BingKind string

const (
	BingQueries   BingKind = "queries"    // Search Performance > Keywords
	BingPages     BingKind = "pages"      // Search Performance > Pages
	BingDates     BingKind = "dates"      // Search Performance chart, one row per day
	BingAIQueries BingKind = "ai_queries" // AI Performance > Grounding queries
	BingAIPages   BingKind = "ai_pages"   // AI Performance > Pages
	BingAIDates   BingKind = "ai_dates"   // AI Performance chart, one row per day
)

// BingTable is the parsed content of one Bing Webmaster Tools CSV. Search
// Performance tables fill Rows or Daily (like a Search Console table), AI
// Performance tables fill GroundingQueries, CitedPages or AIDaily.
type BingTable struct {
	Kind             BingKind
	Rows             []model.GSCRow
	Daily            []model.GSCDailyPoint
	PreviousDaily    []model.GSCDailyPoint
	GroundingQueries []model.BingGroundingQuery
	CitedPages       []model.BingCitedPage
	AIDaily          []model.BingAIDailyPoint
	AIPreviousDaily  []model.BingAIDailyPoint
}

// GA4Kind is the kind of GA4 table.
type GA4Kind string

const (
	GA4Channels     GA4Kind = "channels"      // Session default channel group
	GA4Sources      GA4Kind = "sources"       // Session source / medium
	GA4LandingPages GA4Kind = "landing_pages" // Landing page
	GA4Daily        GA4Kind = "daily"         // Date
)
