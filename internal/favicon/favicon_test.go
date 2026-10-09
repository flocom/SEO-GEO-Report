package favicon

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

var (
	pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{1}, 64)...)
	icoBytes = append([]byte{0, 0, 1, 0, 1, 0}, bytes.Repeat([]byte{2}, 64)...)
	svgOK    = []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><defs><linearGradient id="g"/></defs><rect width="10" height="10" fill="url(#g)"/><use href="#g"/></svg>`)
)

func testResolver(t *testing.T) *Resolver {
	t.Helper()
	return &Resolver{CacheDir: t.TempDir(), AllowPrivate: true}
}

// site serves the given home page HTML and files.
func site(t *testing.T, home string, files map[string][]byte) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/" {
			if home == "" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(home))
			return
		}
		if b, ok := files[r.URL.Path]; ok {
			_, _ = w.Write(b)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(ts.Close)
	return ts, &hits
}

func mimeOf(uri string) string {
	return strings.TrimPrefix(strings.SplitN(uri, ";", 2)[0], "data:")
}

func TestPrefersAppleTouchIcon(t *testing.T) {
	home := `<html><head>
<link rel="icon" href="/small.png" sizes="16x16" type="image/png">
<LINK REL='apple-touch-icon' HREF='/apple.png'>
<link rel="shortcut icon" href="/favicon.ico">
</head><body><link rel="icon" href="/ignored-in-body.png"></body></html>`
	ts, _ := site(t, home, map[string][]byte{"/small.png": pngBytes, "/apple.png": pngBytes, "/favicon.ico": icoBytes})
	uri, err := testResolver(t).Resolve(context.Background(), ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	if mimeOf(uri) != "image/png" {
		t.Errorf("mime = %s", mimeOf(uri))
	}
	cands := parseIcons([]byte(home), mustURL(ts.URL+"/"))
	best := cands[0]
	for _, c := range cands {
		if c.score > best.score {
			best = c
		}
	}
	if best.url.Path != "/apple.png" {
		t.Errorf("best candidate = %s", best.url)
	}
}

func mustURL(s string) *url.URL {
	u, _ := url.Parse(s)
	return u
}

func TestParseVariants(t *testing.T) {
	base := mustURL("https://example.com/blog/post")
	html := `<head><base href="https://cdn.example.com/assets/">
<link href="icon.svg" rel="icon" type="image/svg+xml">
<link rel=icon href=fav32.png sizes=32x32>
<link rel="mask-icon" href="/mask.svg" color="#000">
<link rel="stylesheet" href="/style.css">
<link rel="icon" href="data:image/png;base64,AAAA">
<link rel="icon" href="javascript:alert(1)">
</head>`
	c := parseIcons([]byte(html), base)
	got := map[string]int{}
	for _, x := range c {
		got[x.url.String()] = x.score
	}
	if len(got) != 3 {
		t.Fatalf("candidates = %v", got)
	}
	if got["https://cdn.example.com/assets/icon.svg"] <= got["https://cdn.example.com/assets/fav32.png"] {
		t.Errorf("svg should beat png32: %v", got)
	}
	if got["https://cdn.example.com/mask.svg"] >= got["https://cdn.example.com/assets/fav32.png"] {
		t.Errorf("mask-icon should rank last: %v", got)
	}
}

func TestICOFallback(t *testing.T) {
	ts, _ := site(t, `<html><head><title>No icons</title></head></html>`, map[string][]byte{"/favicon.ico": icoBytes})
	uri, err := testResolver(t).Resolve(context.Background(), ts.URL)
	if err != nil || mimeOf(uri) != "image/x-icon" {
		t.Fatalf("uri=%.40s err=%v", uri, err)
	}
	// Home page failing (404) still falls back to /favicon.ico.
	ts2, _ := site(t, "", map[string][]byte{"/favicon.ico": icoBytes})
	if _, err := testResolver(t).Resolve(context.Background(), ts2.URL); err != nil {
		t.Fatalf("fallback without home page: %v", err)
	}
}

func TestBrokenCandidateSkipped(t *testing.T) {
	home := `<link rel="apple-touch-icon" href="/missing.png"><link rel="icon" href="/notimage.png">`
	ts, _ := site(t, home, map[string][]byte{"/notimage.png": []byte("<html>oops</html>"), "/favicon.ico": icoBytes})
	uri, err := testResolver(t).Resolve(context.Background(), ts.URL)
	if err != nil || mimeOf(uri) != "image/x-icon" {
		t.Fatalf("uri=%.40s err=%v", uri, err)
	}
}

func TestSizeLimit(t *testing.T) {
	big := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{1}, maxIconBytes+10)...)
	ts, _ := site(t, `<link rel="icon" href="/big.png">`, map[string][]byte{"/big.png": big})
	_, err := testResolver(t).Resolve(context.Background(), ts.URL)
	if err == nil {
		t.Fatal("oversized icon accepted")
	}
}

func TestSVGSanitization(t *testing.T) {
	bad := map[string]string{
		"script":   `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`,
		"onload":   `<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"></svg>`,
		"external": `<svg xmlns="http://www.w3.org/2000/svg"><image href="https://evil.example/x.png"/></svg>`,
		"xlink":    `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><use xlink:href="http://evil/x.svg#a"/></svg>`,
		"css url":  `<svg xmlns="http://www.w3.org/2000/svg"><style>rect{fill:url(https://evil/x)}</style></svg>`,
		"foreign":  `<svg xmlns="http://www.w3.org/2000/svg"><foreignObject><div/></foreignObject></svg>`,
		"js href":  `<svg xmlns="http://www.w3.org/2000/svg"><a href="javascript:alert(1)"><rect/></a></svg>`,
	}
	for name, s := range bad {
		if _, err := sniff([]byte(s)); err == nil {
			t.Errorf("%s: unsafe SVG accepted", name)
		}
	}
	if m, err := sniff(svgOK); err != nil || m != "image/svg+xml" {
		t.Errorf("safe SVG rejected: %v", err)
	}
	ts, _ := site(t, `<link rel="icon" href="/i.svg" type="image/svg+xml">`, map[string][]byte{
		"/i.svg": []byte(bad["script"]), "/favicon.ico": icoBytes,
	})
	uri, err := testResolver(t).Resolve(context.Background(), ts.URL)
	if err != nil || mimeOf(uri) != "image/x-icon" {
		t.Errorf("unsafe svg should be skipped for the ico: %.40s %v", uri, err)
	}
}

func TestSniffFormats(t *testing.T) {
	cases := map[string][]byte{
		"image/gif":  []byte("GIF89a......"),
		"image/jpeg": {0xFF, 0xD8, 0xFF, 0xE0, 0, 0},
		"image/webp": []byte("RIFF\x00\x00\x00\x00WEBPVP8 "),
	}
	for want, b := range cases {
		if got, err := sniff(b); err != nil || got != want {
			t.Errorf("sniff %s = %s %v", want, got, err)
		}
	}
	if _, err := sniff([]byte("hello world")); err == nil {
		t.Error("text accepted as image")
	}
}

func TestSSRFRefused(t *testing.T) {
	ts, _ := site(t, `<link rel="icon" href="/i.png">`, map[string][]byte{"/i.png": pngBytes})
	r := &Resolver{CacheDir: t.TempDir()} // AllowPrivate false
	_, err := r.Resolve(context.Background(), ts.URL)
	if !errors.Is(err, ErrBlocked) && (err == nil || !strings.Contains(err.Error(), "not allowed")) {
		t.Fatalf("loopback fetch not refused: %v", err)
	}
	if _, err := r.Embed(context.Background(), ts.URL+"/i.png"); err == nil {
		t.Fatal("Embed of a loopback URL not refused")
	}
	for _, s := range []string{"ftp://example.com/x.ico", "file:///etc/passwd"} {
		if _, err := r.Embed(context.Background(), s); err == nil {
			t.Errorf("%s accepted", s)
		}
	}
	// Allowed explicitly.
	r2 := &Resolver{CacheDir: t.TempDir(), AllowPrivate: true}
	if _, err := r2.Resolve(context.Background(), ts.URL); err != nil {
		t.Fatalf("allowed private fetch failed: %v", err)
	}
}

func TestPublicIP(t *testing.T) {
	for ip, want := range map[string]bool{
		"8.8.8.8": true, "2606:4700::1111": true,
		"127.0.0.1": false, "10.1.2.3": false, "192.168.1.1": false, "172.16.0.1": false,
		"169.254.169.254": false, "100.64.0.1": false, "0.0.0.0": false, "::1": false,
		"fd00::1": false, "fe80::1": false, "::ffff:127.0.0.1": false, "224.0.0.1": false,
	} {
		if got := publicIP(net.ParseIP(ip)); got != want {
			t.Errorf("publicIP(%s) = %v, want %v", ip, got, want)
		}
	}
}

func TestCacheAndApply(t *testing.T) {
	ts, hits := site(t, `<link rel="icon" href="/i.png" sizes="64x64">`, map[string][]byte{"/i.png": pngBytes})
	r := testResolver(t)
	rep := &model.Report{Meta: model.Meta{SiteURL: ts.URL}}
	if !Needed(rep) || !r.Apply(context.Background(), rep) || !strings.HasPrefix(rep.Meta.FaviconURL, "data:image/png;base64,") {
		t.Fatalf("Apply: %.40s", rep.Meta.FaviconURL)
	}
	if Needed(rep) {
		t.Error("data URI should not need fetching")
	}
	n := hits.Load()
	rep2 := &model.Report{Meta: model.Meta{SiteURL: ts.URL}}
	r.Apply(context.Background(), rep2)
	if hits.Load() != n {
		t.Error("cache not used")
	}

	// Negative results are cached for an hour.
	dead, deadHits := site(t, "", nil)
	rn := testResolver(t)
	if _, err := rn.Resolve(context.Background(), dead.URL); err == nil {
		t.Fatal("expected failure")
	}
	n = deadHits.Load()
	if _, err := rn.Resolve(context.Background(), dead.URL); err == nil || deadHits.Load() != n {
		t.Error("negative result not cached")
	}
	rn.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	_, _ = rn.Resolve(context.Background(), dead.URL)
	if deadHits.Load() == n {
		t.Error("negative cache did not expire")
	}

	// "none" and explicit URLs.
	none := &model.Report{Meta: model.Meta{SiteURL: ts.URL, FaviconURL: "none"}}
	if r.Apply(context.Background(), none) || none.Meta.FaviconURL != "none" {
		t.Error("none must be kept")
	}
	explicit := &model.Report{Meta: model.Meta{FaviconURL: ts.URL + "/i.png"}}
	if !r.Apply(context.Background(), explicit) || !strings.HasPrefix(explicit.Meta.FaviconURL, "data:image/png") {
		t.Error("explicit URL not embedded")
	}
}

func TestRedirectLimit(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, ts.URL+r.URL.Path+"x", http.StatusFound)
	}))
	defer ts.Close()
	if _, err := testResolver(t).Embed(context.Background(), ts.URL+"/i"); err == nil {
		t.Error("infinite redirects accepted")
	}
}
