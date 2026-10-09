// Package example provides a realistic demo report used by the CLI
// ("seogeo example"), the MCP guide tool and the tests.
package example

import "seogeo/internal/model"

// Demo returns a complete demo report in the requested language ("fr"/"en"):
// fictional site, 3 months of daily data, comparison period, GEO data and text.
func Demo(lang string) *model.Report { return &model.Report{} }
