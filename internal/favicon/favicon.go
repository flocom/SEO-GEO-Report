// Package favicon finds a website's icon and returns it as a data: URI, so
// reports can embed it without any external request.
//
// Fetching is protected against SSRF: only http/https, and connections to
// private, loopback, link-local (cloud metadata) or otherwise non-public IP
// addresses are refused after DNS resolution (set FAVICON_ALLOW_PRIVATE=true
// to allow them). Results, including failures, are cached on disk per host.
package favicon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

const (
	maxPageBytes  = 1 << 20
	maxIconBytes  = 256 << 10
	maxRedirects  = 5
	fetchTimeout  = 5 * time.Second
	positiveTTL   = 7 * 24 * time.Hour
	negativeTTL   = time.Hour
	userAgent     = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36 seogeo-favicon/1.0"
	resolveBudget = 12 * time.Second
)

// ErrCached wraps a failure remembered from a previous attempt (negative cache).
var ErrCached = errors.New("favicon: cached failure")

// ErrBlocked is returned when a URL points to a non-public address.
var ErrBlocked = errors.New("favicon: address not allowed (private, loopback or link-local)")

// Resolver fetches and caches favicons.
type Resolver struct {
	// CacheDir holds one JSON file per host ("" = no cache).
	CacheDir string
	// AllowPrivate disables the SSRF protection (tests, intranet sites).
	AllowPrivate bool
	Logger       *slog.Logger

	client *http.Client
	now    func() time.Time
}

// New returns a resolver caching in <dataDir>/favicons. AllowPrivate follows
// the FAVICON_ALLOW_PRIVATE environment variable.
func New(dataDir string) *Resolver {
	r := &Resolver{AllowPrivate: envTrue("FAVICON_ALLOW_PRIVATE")}
	if dataDir != "" {
		r.CacheDir = filepath.Join(dataDir, "favicons")
	}
	return r
}

func envTrue(k string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(k))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

var defaultResolver = New("")

// Resolve finds the favicon of siteURL with a default, cache-less resolver.
func Resolve(ctx context.Context, siteURL string) (string, error) {
	return defaultResolver.Resolve(ctx, siteURL)
}

func (r *Resolver) log() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}

func (r *Resolver) clock() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}

// httpClient builds the SSRF-safe client lazily.
func (r *Resolver) httpClient() *http.Client {
	if r.client != nil {
		return r.client
	}
	dialer := &net.Dialer{Timeout: fetchTimeout, KeepAlive: 30 * time.Second}
	if !r.AllowPrivate {
		// Control runs after DNS resolution with the actual IP being dialed,
		// which also defeats DNS rebinding.
		dialer.Control = func(network, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || !publicIP(ip) {
				return ErrBlocked
			}
			return nil
		}
	}
	tr := &http.Transport{
		Proxy:                 nil, // a proxy would bypass the IP check
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   fetchTimeout,
		ResponseHeaderTimeout: fetchTimeout,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
	}
	r.client = &http.Client{
		Transport: tr,
		Timeout:   fetchTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return errors.New("favicon: too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return errors.New("favicon: redirect to a non-http URL")
			}
			return nil
		},
	}
	return r.client
}

var cgnat = mustCIDR("100.64.0.0/10")
var benchmark = mustCIDR("198.18.0.0/15")
var thisNet = mustCIDR("0.0.0.0/8")

func mustCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

// publicIP reports whether ip is a globally routable unicast address.
func publicIP(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
		if cgnat.Contains(ip) || benchmark.Contains(ip) || thisNet.Contains(ip) || ip4[0] >= 224 {
			return false
		}
	}
	return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified())
}

// ---------------------------------------------------------------------------
// Cache

type cacheEntry struct {
	DataURI   string    `json:"data_uri,omitempty"`
	Error     string    `json:"error,omitempty"`
	FetchedAt time.Time `json:"fetched_at"`
}

var unsafeChars = regexp.MustCompile(`[^a-z0-9.-]`)

func (r *Resolver) cachePath(key string) string {
	h := sha256.Sum256([]byte(key))
	safe := unsafeChars.ReplaceAllString(strings.ToLower(key), "_")
	if len(safe) > 60 {
		safe = safe[:60]
	}
	return filepath.Join(r.CacheDir, safe+"-"+hex.EncodeToString(h[:4])+".json")
}

func (r *Resolver) cacheGet(key string) (*cacheEntry, bool) {
	if r.CacheDir == "" {
		return nil, false
	}
	b, err := os.ReadFile(r.cachePath(key))
	if err != nil {
		return nil, false
	}
	var e cacheEntry
	if json.Unmarshal(b, &e) != nil {
		return nil, false
	}
	ttl := positiveTTL
	if e.DataURI == "" {
		ttl = negativeTTL
	}
	if r.clock().Sub(e.FetchedAt) > ttl {
		return nil, false
	}
	return &e, true
}

func (r *Resolver) cachePut(key string, e cacheEntry) {
	if r.CacheDir == "" {
		return
	}
	if err := os.MkdirAll(r.CacheDir, 0o755); err != nil {
		return
	}
	b, _ := json.Marshal(e)
	p := r.cachePath(key)
	tmp := p + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, p)
	}
}

// ---------------------------------------------------------------------------
// Public API

// Resolve finds the best icon of siteURL and returns it as a data URI.
func (r *Resolver) Resolve(ctx context.Context, siteURL string) (string, error) {
	u, err := parseHTTPURL(siteURL)
	if err != nil {
		return "", err
	}
	key := "site:" + u.Host
	if e, ok := r.cacheGet(key); ok {
		if e.DataURI == "" {
			return "", fmt.Errorf("%w: %s", ErrCached, e.Error)
		}
		return e.DataURI, nil
	}
	ctx, cancel := context.WithTimeout(ctx, resolveBudget)
	defer cancel()
	uri, err := r.resolve(ctx, u)
	if ctx.Err() != nil && errors.Is(err, context.Canceled) {
		return "", err // caller gave up: do not cache
	}
	e := cacheEntry{DataURI: uri, FetchedAt: r.clock().UTC()}
	if err != nil {
		e.Error = err.Error()
	}
	r.cachePut(key, e)
	return uri, err
}

// Embed downloads an icon URL (http/https) and returns it as a data URI.
func (r *Resolver) Embed(ctx context.Context, iconURL string) (string, error) {
	u, err := parseHTTPURL(iconURL)
	if err != nil {
		return "", err
	}
	key := "url:" + u.String()
	if e, ok := r.cacheGet(key); ok {
		if e.DataURI == "" {
			return "", fmt.Errorf("%w: %s", ErrCached, e.Error)
		}
		return e.DataURI, nil
	}
	ctx, cancel := context.WithTimeout(ctx, resolveBudget)
	defer cancel()
	uri, err := r.download(ctx, u)
	e := cacheEntry{DataURI: uri, FetchedAt: r.clock().UTC()}
	if err != nil {
		e.Error = err.Error()
	}
	r.cachePut(key, e)
	return uri, err
}

// Apply fills r.Meta.FaviconURL following the report conventions:
//   - "none" or a data: URI: unchanged;
//   - an http(s) URL: downloaded and replaced by a data URI;
//   - empty with a site_url: resolved from the site.
//
// It returns true when the report was changed. Failures are logged only.
func (r *Resolver) Apply(ctx context.Context, rep *model.Report) bool {
	if r == nil || rep == nil {
		return false
	}
	cur := strings.TrimSpace(rep.Meta.FaviconURL)
	switch {
	case strings.EqualFold(cur, "none"), strings.HasPrefix(cur, "data:"):
		return false
	case strings.HasPrefix(cur, "http://") || strings.HasPrefix(cur, "https://"):
		uri, err := r.Embed(ctx, cur)
		if err != nil {
			if !errors.Is(err, ErrCached) {
				r.log().Info("favicon: cannot embed icon", "url", cur, "err", err)
			}
			return false
		}
		rep.Meta.FaviconURL = uri
		return true
	case cur == "" && strings.TrimSpace(rep.Meta.SiteURL) != "":
		uri, err := r.Resolve(ctx, rep.Meta.SiteURL)
		if err != nil {
			if !errors.Is(err, ErrCached) {
				r.log().Info("favicon: not found", "site", rep.Meta.SiteURL, "err", err)
			}
			return false
		}
		rep.Meta.FaviconURL = uri
		return true
	}
	return false
}

// Needed reports whether Apply would try to fetch something.
func Needed(rep *model.Report) bool {
	cur := strings.TrimSpace(rep.Meta.FaviconURL)
	if cur == "" {
		return strings.TrimSpace(rep.Meta.SiteURL) != ""
	}
	return strings.HasPrefix(cur, "http://") || strings.HasPrefix(cur, "https://")
}

func parseHTTPURL(s string) (*url.URL, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("favicon: empty URL")
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("favicon: invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("favicon: only http and https URLs are supported")
	}
	if u.Hostname() == "" {
		return nil, errors.New("favicon: URL without host")
	}
	u.User = nil
	u.Fragment = ""
	return u, nil
}

// ---------------------------------------------------------------------------
// Resolution

type candidate struct {
	url   *url.URL
	score int
}

func (r *Resolver) resolve(ctx context.Context, site *url.URL) (string, error) {
	home := &url.URL{Scheme: site.Scheme, Host: site.Host, Path: "/"}
	if site.Path != "" && site.Path != "/" {
		home.Path = site.Path
	}
	var cands []candidate
	page, finalURL, err := r.fetch(ctx, home, maxPageBytes)
	if err == nil {
		cands = parseIcons(page, finalURL)
	} else if errors.Is(err, ErrBlocked) {
		return "", err
	}
	base := home
	if finalURL != nil {
		base = finalURL
	}
	cands = append(cands, candidate{url: &url.URL{Scheme: base.Scheme, Host: base.Host, Path: "/favicon.ico"}, score: 1})
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].score > cands[j].score })

	var lastErr error
	seen := map[string]bool{}
	for _, c := range cands {
		if seen[c.url.String()] {
			continue
		}
		seen[c.url.String()] = true
		uri, err := r.download(ctx, c.url)
		if err == nil {
			return uri, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
	}
	if lastErr == nil {
		lastErr = errors.New("favicon: no icon found")
	}
	return "", lastErr
}

// fetch GETs u and returns at most limit bytes (error if larger) and the
// final URL after redirects.
func (r *Resolver) fetch(ctx context.Context, u *url.URL, limit int64) ([]byte, *url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,image/*;q=0.9,*/*;q=0.8")
	resp, err := r.httpClient().Do(req)
	if err != nil {
		if errors.Is(err, ErrBlocked) {
			return nil, nil, ErrBlocked
		}
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, resp.Request.URL, fmt.Errorf("favicon: %s returned %s", u, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, resp.Request.URL, err
	}
	if int64(len(b)) > limit {
		return nil, resp.Request.URL, fmt.Errorf("favicon: %s is larger than %d bytes", u, limit)
	}
	return b, resp.Request.URL, nil
}

func (r *Resolver) download(ctx context.Context, u *url.URL) (string, error) {
	b, _, err := r.fetch(ctx, u, maxIconBytes)
	if err != nil {
		return "", err
	}
	mime, err := sniff(b)
	if err != nil {
		return "", fmt.Errorf("%w (%s)", err, u)
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b), nil
}

var (
	linkRe  = regexp.MustCompile(`(?is)<link\b[^>]*>`)
	baseRe  = regexp.MustCompile(`(?is)<base\b[^>]*>`)
	attrRe  = regexp.MustCompile(`(?is)([a-z][a-z0-9:_-]*)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+))`)
	sizesRe = regexp.MustCompile(`(\d+)\s*[xX]\s*(\d+)`)
)

func attrs(tag string) map[string]string {
	m := map[string]string{}
	for _, a := range attrRe.FindAllStringSubmatch(tag, -1) {
		v := a[2] + a[3] + a[4]
		m[strings.ToLower(a[1])] = strings.TrimSpace(htmlUnescape(v))
	}
	return m
}

func htmlUnescape(s string) string {
	return strings.NewReplacer("&amp;", "&", "&quot;", `"`, "&#39;", "'", "&lt;", "<", "&gt;", ">").Replace(s)
}

// parseIcons extracts icon links from an HTML page, scored by preference.
func parseIcons(page []byte, pageURL *url.URL) []candidate {
	// Only the head matters; cut the body to speed up parsing.
	s := string(page)
	if i := strings.Index(strings.ToLower(s), "</head>"); i > 0 {
		s = s[:i]
	}
	base := pageURL
	if m := baseRe.FindString(s); m != "" {
		if href := attrs(m)["href"]; href != "" {
			if bu, err := pageURL.Parse(href); err == nil && (bu.Scheme == "http" || bu.Scheme == "https") {
				base = bu
			}
		}
	}
	var out []candidate
	for _, tag := range linkRe.FindAllString(s, -1) {
		a := attrs(tag)
		href := a["href"]
		if href == "" || strings.HasPrefix(href, "data:") {
			continue
		}
		rels := strings.Fields(strings.ToLower(a["rel"]))
		kind := ""
		for _, rel := range rels {
			switch rel {
			case "apple-touch-icon", "apple-touch-icon-precomposed":
				kind = "apple"
			case "icon":
				if kind == "" {
					kind = "icon"
				}
			case "mask-icon":
				if kind == "" {
					kind = "mask"
				}
			}
		}
		if kind == "" {
			continue
		}
		u, err := base.Parse(href)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			continue
		}
		size := 0
		anySize := false
		for _, m := range sizesRe.FindAllStringSubmatch(a["sizes"], -1) {
			if n, _ := strconv.Atoi(m[1]); n > size {
				size = n
			}
		}
		if strings.Contains(strings.ToLower(a["sizes"]), "any") {
			anySize = true
		}
		lowerHref := strings.ToLower(u.Path)
		typ := strings.ToLower(a["type"])
		isSVG := strings.Contains(typ, "svg") || strings.HasSuffix(lowerHref, ".svg")
		isICO := strings.Contains(typ, "icon") && !strings.Contains(typ, "png") || strings.HasSuffix(lowerHref, ".ico")
		score := 0
		switch kind {
		case "apple":
			score = 300
			if size == 0 {
				size = 180
			}
		case "icon":
			switch {
			case isSVG || anySize:
				score = 260
			case isICO:
				score = 100
			case size >= 32:
				score = 200
			default:
				score = 120
			}
		case "mask":
			score = 40 // monochrome, usually a poor fit
		}
		if size > 256 {
			size = 256
		}
		out = append(out, candidate{url: u, score: score + size/4})
	}
	return out
}

// sniff identifies the image type from its magic bytes and validates SVGs.
func sniff(b []byte) (string, error) {
	switch {
	case len(b) < 4:
		return "", errors.New("favicon: empty or truncated image")
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png", nil
	case bytes.HasPrefix(b, []byte{0, 0, 1, 0}):
		return "image/x-icon", nil
	case bytes.HasPrefix(b, []byte("GIF87a")), bytes.HasPrefix(b, []byte("GIF89a")):
		return "image/gif", nil
	case bytes.HasPrefix(b, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg", nil
	case len(b) >= 12 && bytes.HasPrefix(b, []byte("RIFF")) && string(b[8:12]) == "WEBP":
		return "image/webp", nil
	}
	if looksLikeSVG(b) {
		if err := checkSVG(b); err != nil {
			return "", err
		}
		return "image/svg+xml", nil
	}
	return "", errors.New("favicon: not a supported image (png, ico, svg, gif, jpeg, webp)")
}

func looksLikeSVG(b []byte) bool {
	head := b
	if len(head) > 2048 {
		head = head[:2048]
	}
	head = bytes.TrimPrefix(head, []byte("\xEF\xBB\xBF"))
	return bytes.Contains(bytes.ToLower(head), []byte("<svg"))
}

var (
	svgEventRe = regexp.MustCompile(`(?i)[\s"'/]on[a-z]+\s*=`)
	svgHrefRe  = regexp.MustCompile(`(?i)(?:xlink:)?href\s*=\s*["']?\s*([^"'\s>]*)`)
	svgURLRe   = regexp.MustCompile(`(?i)url\(\s*['"]?\s*([^'")\s]*)`)
)

// checkSVG rejects SVG files that could run code or load external content.
func checkSVG(b []byte) error {
	s := strings.ToLower(string(b))
	for _, bad := range []string{"<script", "javascript:", "<foreignobject", "<!entity", "<iframe", "<embed", "<object", "@import", "vbscript:"} {
		if strings.Contains(s, bad) {
			return fmt.Errorf("favicon: unsafe SVG (%s)", bad)
		}
	}
	if svgEventRe.MatchString(s) {
		return errors.New("favicon: unsafe SVG (event handler attribute)")
	}
	for _, m := range svgHrefRe.FindAllStringSubmatch(s, -1) {
		if v := m[1]; v != "" && !strings.HasPrefix(v, "#") && !strings.HasPrefix(v, "data:image/") {
			return errors.New("favicon: unsafe SVG (external reference)")
		}
	}
	for _, m := range svgURLRe.FindAllStringSubmatch(s, -1) {
		if v := m[1]; v != "" && !strings.HasPrefix(v, "#") && !strings.HasPrefix(v, "data:image/") {
			return errors.New("favicon: unsafe SVG (external url())")
		}
	}
	return nil
}
