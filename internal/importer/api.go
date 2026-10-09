// Package importer converts raw exports (CSV files from Google Search Console
// and Google Analytics 4, in French or English) into model structures.
package importer

import "seogeo/internal/model"

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

// ParseGSCCSV parses one Search Console export CSV. kind may be empty: it is
// then guessed from the header.
func ParseGSCCSV(data []byte, kind GSCKind) (*GSCTable, error) { return nil, nil }

// ImportGSCExport reads a Search Console export, given as a .zip file or a
// directory containing the CSV files, and returns a SearchConsole with totals
// computed from the Dates table (CTR and position impression-weighted).
func ImportGSCExport(path string) (*model.SearchConsole, error) { return nil, nil }

// ApplyGSCTable merges a parsed table into sc (creating it when nil) and
// recomputes totals when the table is the Dates table.
func ApplyGSCTable(sc *model.SearchConsole, t *GSCTable) *model.SearchConsole { return sc }

// GA4Kind is the kind of GA4 table.
type GA4Kind string

const (
	GA4Channels     GA4Kind = "channels"      // Session default channel group
	GA4Sources      GA4Kind = "sources"       // Session source / medium
	GA4LandingPages GA4Kind = "landing_pages" // Landing page
	GA4Daily        GA4Kind = "daily"         // Date
)

// ParseGA4CSV parses a GA4 "Export > Download CSV" file (comment lines starting
// with '#' are skipped, French or English headers) and merges the rows into a
// (possibly nil) Analytics. kind may be empty: it is guessed from the header.
func ParseGA4CSV(data []byte, kind GA4Kind, into *model.Analytics) (*model.Analytics, error) {
	return into, nil
}
