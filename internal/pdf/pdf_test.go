package pdf

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func fakeFinder(env map[string]string, inPath map[string]string, files map[string]bool) finder {
	return finder{
		getenv: func(k string) string { return env[k] },
		lookPath: func(name string) (string, error) {
			if p, ok := inPath[name]; ok {
				return p, nil
			}
			return "", errors.New("not found")
		},
		exists: func(p string) bool { return files[p] },
	}
}

func TestFindChrome(t *testing.T) {
	tests := []struct {
		name    string
		f       finder
		want    string
		wantErr bool
	}{
		{"env wins", fakeFinder(map[string]string{"CHROME_PATH": "/opt/chrome"}, map[string]string{"chromium": "/usr/bin/chromium"}, map[string]bool{"/opt/chrome": true}), "/opt/chrome", false},
		{"env name in PATH", fakeFinder(map[string]string{"CHROME_PATH": "mychrome"}, map[string]string{"mychrome": "/x/mychrome"}, nil), "/x/mychrome", false},
		{"env invalid", fakeFinder(map[string]string{"CHROME_PATH": "/nope"}, map[string]string{"chromium": "/usr/bin/chromium"}, nil), "", true},
		{"PATH order", fakeFinder(nil, map[string]string{"google-chrome": "/usr/bin/google-chrome", "chromium-browser": "/usr/bin/chromium-browser"}, nil), "/usr/bin/chromium-browser", false},
		{"mac app", fakeFinder(nil, nil, map[string]bool{"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome": true}), "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", false},
		{"none", fakeFinder(nil, nil, nil), "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.f.find()
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
	if _, err := fakeFinder(nil, nil, nil).find(); !errors.Is(err, ErrNoChrome) {
		t.Errorf("want ErrNoChrome, got %v", err)
	}
}

func TestFromHTML(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	if !Available() {
		t.Skip("no Chromium available")
	}
	if os.Getenv("SEOGEO_SKIP_CHROME_TEST") != "" {
		t.Skip("SEOGEO_SKIP_CHROME_TEST set (PDF export is tested inside the Docker image)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	html := []byte(`<!doctype html><html><head><meta charset="utf-8"><style>@page{size:A4;margin:12mm}</style></head>
<body><h1>Rapport SEO &amp; GEO — Été 📈</h1><svg width="200" height="50"><rect width="120" height="30" fill="#2563eb"/></svg></body></html>`)
	out, err := FromHTML(ctx, html)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF")) {
		t.Fatalf("output does not start with %%PDF: %q", out[:min(len(out), 20)])
	}
}
