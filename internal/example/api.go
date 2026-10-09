// Package example provides a realistic demo report used by the CLI
// ("seogeo example"), the MCP guide tool and the tests.
package example

import "github.com/flocom/SEO-GEO-Report/internal/model"

// Demo returns a complete demo report in the requested language ("fr"/"en"):
// fictional site, 3 months of daily data, comparison period, GEO data and text.
//
// The output is deterministic (seeded PCG generator, fixed dates) and the
// numbers are identical in both languages; only the human text changes.
func Demo(lang string) *model.Report { return demo(lang) }
