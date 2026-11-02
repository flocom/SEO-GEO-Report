package model

import (
	"strings"
	"testing"
)

func TestNormViewAndDefault(t *testing.T) {
	for in, want := range map[string]string{
		"essential": ViewEssential, " Essentiel ": ViewEssential, "advanced": ViewAdvanced, "avancé": ViewAdvanced,
		"complet": ViewAdvanced, "": "", "other": "",
	} {
		if got := NormView(in); got != want {
			t.Errorf("NormView(%q) = %q, want %q", in, got, want)
		}
	}
	r := &Report{}
	if v := r.Opt().DefaultView; v != ViewEssential {
		t.Errorf("default view = %q, want essential", v)
	}
	r.Options = &Options{DefaultView: "Advanced"}
	if v := r.Opt().DefaultView; v != ViewAdvanced {
		t.Errorf("default view = %q, want advanced", v)
	}
	r = &Report{Meta: Meta{SiteName: "x", Period: Period{Start: "2026-01-01", End: "2026-01-31"}}, Options: &Options{DefaultView: "weird"}}
	_, warns := r.Validate()
	if !strings.Contains(strings.Join(warns, "|"), "options.default_view") {
		t.Errorf("an unknown default_view should be reported: %v", warns)
	}
}
