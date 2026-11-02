package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// DefaultWatchtowerURL is the Watchtower HTTP API in the provided compose file
// (internal network only, never published).
const DefaultWatchtowerURL = "http://watchtower:8080"

// Watchtower states reported to the page and the API.
const (
	wtOK            = "ok"             // API reachable, token accepted
	wtNotConfigured = "not_configured" // no token configured
	wtUnreachable   = "unreachable"    // connection failed
	wtUnauthorized  = "unauthorized"   // token rejected
	wtError         = "error"          // unexpected answer
)

// Watchtower talks to the HTTP API of a Watchtower container
// (https://github.com/nicholas-fedor/watchtower). The token never leaves the
// server.
type Watchtower struct {
	// URL of the API, e.g. http://watchtower:8080.
	URL string
	// Token is WATCHTOWER_HTTP_API_TOKEN: the token itself or, like in
	// Watchtower, the path of a file containing it (re-read at each call so
	// that a file created after startup is picked up).
	Token  string
	Client *http.Client

	mu     sync.Mutex
	cached *WatchtowerStatus
}

// WatchtowerStatus is the result of a probe.
type WatchtowerStatus struct {
	State     string    `json:"state"`
	URL       string    `json:"url,omitempty"`
	Detail    string    `json:"detail,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
	// LastScan is the summary of the last Watchtower run (/v1/status), when
	// the status endpoint is enabled and a run happened.
	LastScan *ScanSummary `json:"last_scan,omitempty"`
}

// ScanSummary mirrors the Watchtower /v1/status and /v1/update summary.
type ScanSummary struct {
	Scanned    int       `json:"scanned"`
	Updated    int       `json:"updated"`
	Failed     int       `json:"failed"`
	Restarted  int       `json:"restarted"`
	Skipped    int       `json:"skipped"`
	ScanFailed bool      `json:"scan_failed,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// Configured reports whether a token is available.
func (w *Watchtower) Configured() bool {
	return w != nil && w.token() != ""
}

func (w *Watchtower) token() string {
	v := strings.TrimSpace(w.Token)
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, "/") {
		if fi, err := os.Stat(v); err == nil && fi.Mode().IsRegular() {
			b, err := os.ReadFile(v)
			if err != nil {
				return ""
			}
			return strings.TrimSpace(string(b))
		}
		if strings.Count(v, "/") > 1 {
			// Looks like a path that does not exist (yet): not a token.
			return ""
		}
	}
	return v
}

func (w *Watchtower) client() *http.Client {
	if w.Client != nil {
		return w.Client
	}
	return &http.Client{} // bounded by the request contexts (an update can take minutes)
}

func (w *Watchtower) base() string {
	u := strings.TrimRight(strings.TrimSpace(w.URL), "/")
	if u == "" {
		u = DefaultWatchtowerURL
	}
	return u
}

func (w *Watchtower) request(ctx context.Context, method, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, w.base()+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+w.token())
	req.Header.Set("Accept", "application/json")
	return w.client().Do(req)
}

// Status probes the API, caching the answer for maxAge.
func (w *Watchtower) Status(ctx context.Context, maxAge time.Duration) WatchtowerStatus {
	if w == nil {
		return WatchtowerStatus{State: wtNotConfigured}
	}
	w.mu.Lock()
	if c := w.cached; c != nil && time.Since(c.CheckedAt) < maxAge {
		st := *c
		w.mu.Unlock()
		return st
	}
	w.mu.Unlock()
	st := w.probe(ctx)
	w.mu.Lock()
	w.cached = &st
	w.mu.Unlock()
	return st
}

func (w *Watchtower) invalidate() {
	w.mu.Lock()
	w.cached = nil
	w.mu.Unlock()
}

func (w *Watchtower) probe(ctx context.Context) WatchtowerStatus {
	st := WatchtowerStatus{URL: w.base(), CheckedAt: time.Now()}
	if !w.Configured() {
		st.State = wtNotConfigured
		return st
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	resp, err := w.request(ctx, http.MethodGet, "/v1/status")
	if err != nil {
		st.State, st.Detail = wtUnreachable, shortErr(err)
		return st
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		st.State = wtOK
		var body struct {
			Summary   ScanSummary `json:"summary"`
			Timestamp time.Time   `json:"timestamp"`
		}
		if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body) == nil && !body.Timestamp.IsZero() {
			s := body.Summary
			s.Timestamp = body.Timestamp
			st.LastScan = &s
		}
	case http.StatusNoContent:
		st.State = wtOK // no run yet
	case http.StatusNotFound, http.StatusMethodNotAllowed:
		// API reachable but the status endpoint is not enabled (older
		// configuration with only the update endpoint): updates still work.
		st.State, st.Detail = wtOK, "status endpoint disabled"
	case http.StatusUnauthorized, http.StatusForbidden:
		st.State = wtUnauthorized
	default:
		st.State, st.Detail = wtError, resp.Status
	}
	return st
}

// Errors returned by Run.
var (
	ErrWatchtowerNotConfigured = errors.New("watchtower: no API token configured")
	ErrWatchtowerUnreachable   = errors.New("watchtower: API unreachable")
	ErrWatchtowerUnauthorized  = errors.New("watchtower: API token rejected")
	ErrWatchtowerBusy          = errors.New("watchtower: an update is already running")
)

// Run asks Watchtower to check and update the monitored containers now
// (POST /v1/update) and waits for the result (bounded by ctx and by
// Watchtower's own update timeout, 10 minutes by default). When Watchtower
// updates this very container, the process is stopped before Run returns.
//
// The synchronous call is used on purpose: it is the only way to know the
// outcome ("nothing to update"), as the timestamp of /v1/status is the time
// of the request, not of the last run.
func (w *Watchtower) Run(ctx context.Context) (*ScanSummary, error) {
	if !w.Configured() {
		return nil, ErrWatchtowerNotConfigured
	}
	defer w.invalidate()
	resp, err := w.request(ctx, http.MethodPost, "/v1/update")
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrWatchtowerUnreachable, shortErr(err))
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	switch {
	case resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted:
		var out struct {
			Summary   ScanSummary `json:"summary"`
			Timestamp time.Time   `json:"timestamp"`
		}
		if json.Unmarshal(body, &out) != nil {
			return &ScanSummary{Timestamp: time.Now().UTC()}, nil // older Watchtower: empty body
		}
		s := out.Summary
		s.Timestamp = out.Timestamp
		return &s, nil
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, ErrWatchtowerUnauthorized
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, ErrWatchtowerBusy
	default:
		return nil, fmt.Errorf("watchtower: unexpected answer %s", resp.Status)
	}
}

func shortErr(err error) string {
	s := err.Error()
	if len(s) > 160 {
		s = s[:160] + "…"
	}
	return s
}
