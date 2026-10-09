package update

import (
	"context"
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
