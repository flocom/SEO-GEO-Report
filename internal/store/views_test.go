package store

import (
	"os"
	"testing"
	"time"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// TestPDFCachePerView: each (language, version) pair has its own cached PDF,
// and saving the report invalidates all of them.
func TestPDFCachePerView(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rec, err := s.Create(testReport())
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	for _, lang := range []string{"fr", "en"} {
		for _, view := range []string{"essential", "advanced"} {
			p := s.PDFPath(rec.ID, lang, view)
			if paths[p] {
				t.Fatalf("duplicate PDF path %s", p)
			}
			paths[p] = true
			if _, err := s.SavePDF(rec.ID, lang, view, []byte("%PDF-"+lang+view)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if s.PDFPath(rec.ID, "fr", "") != s.PDFPath(rec.ID, "fr", "advanced") {
		t.Error("an empty view should map to the advanced file")
	}
	b, ok := s.CachedPDF(rec, "en", "essential")
	if !ok || string(b) != "%PDF-enessential" {
		t.Fatalf("cached essential PDF = %q, %v", b, ok)
	}
	if b, _ := s.CachedPDF(rec, "en", "advanced"); string(b) != "%PDF-enadvanced" {
		t.Errorf("cached advanced PDF = %q", b)
	}
	time.Sleep(10 * time.Millisecond)
	rec, err = s.Update(rec.ID, func(r *model.Report) error { r.Meta.Title = "changed"; return nil })
	if err != nil {
		t.Fatal(err)
	}
	for _, view := range []string{"essential", "advanced"} {
		if _, ok := s.CachedPDF(rec, "fr", view); ok {
			t.Errorf("%s PDF still cached after an update", view)
		}
	}
	if err := s.Delete(rec.ID); err != nil {
		t.Fatal(err)
	}
	for p := range paths {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("%s left after Delete", p)
		}
	}
}
