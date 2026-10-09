// Package update checks GitHub for a newer release of seogeo. It is purely
// informative (home page and logs) and never fails the server: errors and
// offline machines are ignored.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Repo is the GitHub repository of the project.
const Repo = "flocom/SEO-GEO-Report"

// DefaultInterval between two checks.
const DefaultInterval = 6 * time.Hour

// Release is the latest published release.
type Release struct {
	Tag     string    `json:"tag_name"`
	URL     string    `json:"html_url"`
	Name    string    `json:"name"`
	Checked time.Time `json:"-"`
}

// Checker polls the GitHub releases API.
type Checker struct {
	Current  string
	Interval time.Duration
	// APIURL defaults to https://api.github.com/repos/<Repo>/releases/latest.
	APIURL string
	Client *http.Client
	Logger *slog.Logger

	mu     sync.Mutex
	latest *Release
}

// New returns a checker for the running version.
func New(current string, logger *slog.Logger) *Checker {
	return &Checker{Current: current, Logger: logger}
}

// Start checks now and then every Interval until ctx is done.
func (c *Checker) Start(ctx context.Context) {
	if c.Interval <= 0 {
		c.Interval = DefaultInterval
	}
	go func() {
		t := time.NewTicker(c.Interval)
		defer t.Stop()
		for {
			c.CheckNow(ctx)
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
}

// CheckNow queries GitHub once. Errors are logged at debug level only.
func (c *Checker) CheckNow(ctx context.Context) {
	api := c.APIURL
	if api == "" {
		api = "https://api.github.com/repos/" + Repo + "/releases/latest"
	}
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api, nil)
	if err != nil {
		return
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "github.com/flocom/SEO-GEO-Report/"+c.Current)
	resp, err := client.Do(req)
	if err != nil {
		c.debug("update check failed", "err", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.debug("update check failed", "status", resp.Status)
		return
	}
	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil || rel.Tag == "" {
		return
	}
	rel.Checked = time.Now()
	c.mu.Lock()
	prev := c.latest
	c.latest = &rel
	c.mu.Unlock()
	if Newer(rel.Tag, c.Current) && (prev == nil || prev.Tag != rel.Tag) && c.Logger != nil {
		c.Logger.Warn(fmt.Sprintf("New version %s available (running %s): %s — with the provided docker-compose.yml, Watchtower installs it automatically within the hour; otherwise run: docker compose pull && docker compose up -d", rel.Tag, c.Current, rel.URL))
	}
}

func (c *Checker) debug(msg string, args ...any) {
	if c.Logger != nil {
		c.Logger.Debug(msg, args...)
	}
}

// Latest returns the last known release (nil if unknown).
func (c *Checker) Latest() *Release {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.latest == nil {
		return nil
	}
	r := *c.latest
	return &r
}

// Available returns the newer release, or nil when up to date / unknown.
func (c *Checker) Available() *Release {
	if c == nil {
		return nil
	}
	r := c.Latest()
	if r == nil || !Newer(r.Tag, c.Current) {
		return nil
	}
	return r
}

// Newer reports whether version a is strictly greater than b. Non semantic
// versions (e.g. "dev") are never considered older than a release, so
// development builds do not nag.
func Newer(a, b string) bool {
	va, ok1 := parse(a)
	vb, ok2 := parse(b)
	if !ok1 || !ok2 {
		return false
	}
	for i := range 3 {
		if va[i] != vb[i] {
			return va[i] > vb[i]
		}
	}
	return false
}

func parse(v string) ([3]int, bool) {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
