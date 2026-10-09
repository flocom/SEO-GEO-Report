// Package importer converts raw exports (CSV files from Google Search Console
// and Google Analytics 4, in French or English) into model structures.
package importer

import "github.com/flocom/SEO-GEO-Report/internal/model"

// Implementations live in gsc.go (ParseGSCCSV, ImportGSCExport,
// ApplyGSCTable) and ga4.go (ParseGA4CSV).

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

// GA4Kind is the kind of GA4 table.
type GA4Kind string

const (
	GA4Channels     GA4Kind = "channels"      // Session default channel group
	GA4Sources      GA4Kind = "sources"       // Session source / medium
	GA4LandingPages GA4Kind = "landing_pages" // Landing page
	GA4Daily        GA4Kind = "daily"         // Date
)
