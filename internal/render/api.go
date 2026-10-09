// Package render turns a report into a single self-contained HTML document.
package render

import "seogeo/internal/model"

// Options tune rendering.
type Options struct {
	// Lang overrides r.Meta.Language when non-empty ("fr" or "en").
	Lang string
}

// HTML renders the report as a standalone HTML page (inline CSS and SVG, no
// external requests, print-ready A4). It normalizes a copy of r.
func HTML(r *model.Report, opts Options) ([]byte, error) { return nil, nil }
