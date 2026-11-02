package mcpserver

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flocom/SEO-GEO-Report/internal/oauth"
	"github.com/flocom/SEO-GEO-Report/internal/store"
)

func TestAdminRouteAndHealth(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.Open(dir)
	adminHit := false
	admin := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		adminHit = true
		_, _ = io.WriteString(w, "admin page")
	})
	svc := NewService(Config{Store: st, Version: "0.2.0", Admin: admin})
	auth, err := oauth.New(oauth.Config{DataDir: dir, Password: "test-password-123"})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(NewHTTPHandler(svc, auth, slog.New(slog.NewTextHandler(io.Discard, nil))))
	defer ts.Close()

	for _, p := range []string{"/admin", "/admin/api/status"} {
		adminHit = false
		resp, err := http.Get(ts.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if !adminHit {
			t.Errorf("%s not routed to the admin handler", p)
		}
	}
	resp, _ := http.Get(ts.URL + "/")
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(b), `href="/admin"`) {
		t.Error("home page lacks the /admin link")
	}
	resp, _ = http.Get(ts.URL + "/healthz")
	var h map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&h)
	resp.Body.Close()
	if h["version"] != "0.2.0" || h["started"] == "" || resp.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("healthz: %v %v", h, resp.Header)
	}
}
