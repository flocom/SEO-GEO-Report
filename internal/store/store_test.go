package store

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

func testReport() *model.Report {
	return &model.Report{
		Meta: model.Meta{SiteName: "Café Élysée", Period: model.Period{Start: "2026-01-01", End: "2026-01-31"}},
		SearchConsole: &model.SearchConsole{
			Totals: model.GSCTotals{Clicks: model.M(100, 80), Impressions: model.M(1000, 900)},
		},
		Narrative: &model.Narrative{
			ExecutiveSummary: "ok",
			Actions: []model.Action{
				{Title: "done one", Status: "done"},
				{Title: "next one", Status: "planned"},
			},
			Recommendations: []model.Recommendation{{Title: "do it"}},
		},
	}
}

func TestCRUD(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rec, err := s.Create(testReport())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(rec.ID, "cafe-elysee-") {
		t.Errorf("id = %q, want slug prefix", rec.ID)
	}
	if len(rec.ShareToken) < 30 {
		t.Errorf("share token too short: %q", rec.ShareToken)
	}
	got, err := s.Get(rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Report.Meta.SiteName != "Café Élysée" || got.Report.SearchConsole.Totals.Clicks.Current != 100 {
		t.Errorf("unexpected report: %+v", got.Report.Meta)
	}

	if _, err := s.Update(rec.ID, func(r *model.Report) error { r.Meta.Title = "T"; return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(rec.ID, func(r *model.Report) error { r.Meta.Title = "X"; return errors.New("nope") }); err == nil {
		t.Fatal("expected error")
	}
	got, _ = s.Get(rec.ID)
	if got.Report.Meta.Title != "T" {
		t.Errorf("title = %q, want T (failed update must not be saved)", got.Report.Meta.Title)
	}

	byTok, err := s.ByShareToken(rec.ShareToken)
	if err != nil || byTok.ID != rec.ID {
		t.Fatalf("ByShareToken: %v %v", byTok, err)
	}
	if _, err := s.ByShareToken("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}

	if _, err := s.SaveHTML(rec.ID, "en", []byte("<html>")); err != nil {
		t.Fatal(err)
	}
	if b, ok := s.CachedHTML(got, "en"); !ok || string(b) != "<html>" {
		t.Errorf("cached html = %q %v", b, ok)
	}

	list, err := s.List()
	if err != nil || len(list) != 1 || list[0].ID != rec.ID {
		t.Fatalf("list = %+v, %v", list, err)
	}

	if err := s.Delete(rec.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(rec.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("after delete: %v", err)
	}
	if _, err := s.Get("../etc/passwd"); !errors.Is(err, ErrNotFound) {
		t.Errorf("path traversal id must be rejected: %v", err)
	}
}

func TestDuplicate(t *testing.T) {
	s, _ := Open(t.TempDir())
	rec, _ := s.Create(testReport())
	p := model.Period{Start: "2026-02-01", End: "2026-02-28"}
	dup, err := s.Duplicate(rec.ID, DuplicateOptions{Period: &p})
	if err != nil {
		t.Fatal(err)
	}
	if dup.ID == rec.ID || dup.ShareToken == rec.ShareToken {
		t.Fatal("duplicate must have new id and token")
	}
	if dup.Report.SearchConsole != nil {
		t.Error("data should be cleared")
	}
	if dup.Report.Meta.Period != p {
		t.Errorf("period = %+v", dup.Report.Meta.Period)
	}
	n := dup.Report.Narrative
	if n == nil || n.ExecutiveSummary != "" || len(n.Actions) != 1 || n.Actions[0].Title != "next one" || len(n.Recommendations) != 1 {
		t.Errorf("narrative = %+v", n)
	}
	keep, err := s.Duplicate(rec.ID, DuplicateOptions{KeepData: true, KeepNarrative: true})
	if err != nil {
		t.Fatal(err)
	}
	if keep.Report.SearchConsole == nil || keep.Report.Narrative.ExecutiveSummary != "ok" {
		t.Error("keep options ignored")
	}
	// Source untouched.
	src, _ := s.Get(rec.ID)
	if src.Report.SearchConsole == nil {
		t.Error("source modified")
	}
}

func TestConcurrentUpdates(t *testing.T) {
	s, _ := Open(t.TempDir())
	rec, _ := s.Create(testReport())
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Update(rec.ID, func(r *model.Report) error {
				r.Sections = append(r.Sections, model.TextSection{Title: "x", Body: "y"})
				return nil
			})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	got, _ := s.Get(rec.ID)
	if len(got.Report.Sections) != 20 {
		t.Errorf("sections = %d, want 20 (lost updates)", len(got.Report.Sections))
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"https://www.Example.com/": "example-com",
		"  Ma Boutique — Été  ":    "ma-boutique-ete",
		"":                         "",
	}
	for in, want := range cases {
		if got := Slugify(in, 32); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}
