// Package render turns a report into a single self-contained HTML document.
package render

import "github.com/flocom/SEO-GEO-Report/internal/model"

// Options tune rendering.
type Options struct {
	// Lang overrides r.Meta.Language when non-empty ("fr" or "en").
	Lang string

	// PDFURL, when non-empty, adds a discreet "Download PDF" link in the
	// report header (hidden when printing). Only http(s) and relative URLs
	// are accepted.
	PDFURL string

	// HTMLDownloadURL, when non-empty, adds a "Download (HTML)" link next to
	// the PDF one (hidden when printing). Same URL rules as PDFURL.
	HTMLDownloadURL string
}

// HTML renders the report as a standalone HTML page (inline CSS and SVG, no
// external requests, print-ready A4). It normalizes a copy of r.
func HTML(r *model.Report, opts Options) ([]byte, error) { return render(r, opts) }
