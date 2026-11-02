package update

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"v0.2.0", "0.1.0", true},
		{"v0.1.0", "0.1.0", false},
		{"v0.1.10", "v0.1.9", true},
		{"v1.0.0", "v1.0.0-rc1", false},
		{"v0.1.0", "dev", false},
		{"garbage", "0.1.0", false},
		{"v0.1", "0.0.9", true},
	}
	for _, c := range cases {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

func TestCheck(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","html_url":"https://github.com/flocom/SEO-GEO-Report/releases/tag/v9.9.9"}`))
	}))
	defer ts.Close()
	c := &Checker{Current: "0.1.0", APIURL: ts.URL}
	c.CheckNow(context.Background())
	if r := c.Available(); r == nil || r.Tag != "v9.9.9" {
		t.Fatalf("Available = %+v", r)
	}
	// Offline: nothing breaks.
	off := &Checker{Current: "0.1.0", APIURL: "http://127.0.0.1:1/nope"}
	off.CheckNow(context.Background())
	if off.Available() != nil {
		t.Error("offline check reported a release")
	}
	var nilChecker *Checker
	if nilChecker.Latest() != nil {
		t.Error("nil checker")
	}
}

func TestForceCheck(t *testing.T) {
	hits := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","html_url":"https://github.com/x","body":"notes","published_at":"2026-11-02T08:00:00Z"}`))
	}))
	defer ts.Close()
	c := &Checker{Current: "0.1.0", APIURL: ts.URL}
	if _, err := c.ForceCheck(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r := c.Latest(); r == nil || r.Body != "notes" || r.PublishedAt.IsZero() {
		t.Fatalf("Latest = %+v", r)
	}
	if last, err := c.LastCheck(); last.IsZero() || err != nil {
		t.Fatalf("LastCheck = %v, %v", last, err)
	}
	wait, err := c.ForceCheck(context.Background())
	if !errors.Is(err, ErrTooSoon) || wait <= 0 || hits != 1 {
		t.Fatalf("second ForceCheck: %v %v hits=%d", wait, err, hits)
	}
	// Errors are recorded.
	bad := &Checker{Current: "0.1.0", APIURL: "http://127.0.0.1:1/nope"}
	if _, err := bad.ForceCheck(context.Background()); err == nil {
		t.Fatal("expected an error")
	}
	if _, err := bad.LastCheck(); err == nil {
		t.Error("LastCheck lost the error")
	}
}
