package admin

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/flocom/SEO-GEO-Report/internal/oauth"
	"github.com/flocom/SEO-GEO-Report/internal/update"
)

const (
	testPassword = "correct-horse-battery-staple"
	testToken    = "wt-secret-token-0123456789"
)

// fakeWatchtower records the calls made to the Watchtower API.
type fakeWatchtower struct {
	*httptest.Server
	mu      sync.Mutex
	calls   []string // "METHOD path?query"
	auth    []string
	updates atomic.Int32
	status  int           // answer of POST /v1/update (default 200)
	block   chan struct{} // when set, POST /v1/update waits for it
}

func newFakeWatchtower(t *testing.T) *fakeWatchtower {
	f := &fakeWatchtower{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.calls = append(f.calls, r.Method+" "+r.URL.RequestURI())
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		status, block := f.status, f.block
		f.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer "+testToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/status":
			_, _ = io.WriteString(w, `{"summary":{"scanned":2,"updated":0,"failed":0,"restarted":0,"skipped":0},"timestamp":"2026-11-02T10:00:00Z","api_version":"v1"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/update":
			f.updates.Add(1)
			if block != nil {
				<-block
			}
			if status != 0 {
				w.WriteHeader(status)
				return
			}
			_, _ = io.WriteString(w, `{"summary":{"scanned":1,"updated":0,"failed":0,"restarted":0,"skipped":0},"timing":{"duration_ms":120},"timestamp":"2026-11-02T12:00:01Z","api_version":"v1"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeWatchtower) lastUpdateCall() (string, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.calls) - 1; i >= 0; i-- {
		if strings.HasPrefix(f.calls[i], "POST ") {
			return f.calls[i], f.auth[i]
		}
	}
	return "", ""
}

type fakeGitHub struct {
	*httptest.Server
	hits atomic.Int32
}

func newFakeGitHub(t *testing.T, tag string) *fakeGitHub {
	g := &fakeGitHub{}
	g.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.hits.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name":     tag,
			"name":         tag,
			"html_url":     "https://github.com/flocom/SEO-GEO-Report/releases/tag/" + tag,
			"body":         "## Added\n- /admin page <script>alert(1)</script>",
			"published_at": "2026-11-02T08:00:00Z",
		})
	}))
	t.Cleanup(g.Close)
	return g
}

type env struct {
	t      *testing.T
	h      *Handler
	auth   *oauth.Server
	wt     *fakeWatchtower
	gh     *fakeGitHub
	mu     sync.Mutex
	now    time.Time
	checks *update.Checker
}

type envOpts struct {
	authDisabled bool
	mode         string
	wtURL        string // override (e.g. unreachable)
	noToken      bool
	tag          string
}

func newEnv(t *testing.T, o envOpts) *env {
	t.Helper()
	oauth.PasswordFailDelay = 0
	jobDelay = 0
	dir := t.TempDir()
	pw := testPassword
	if o.authDisabled {
		pw = ""
	}
	auth, err := oauth.New(oauth.Config{DataDir: dir, Password: pw, Disabled: o.authDisabled, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	if o.tag == "" {
		o.tag = "v0.3.0"
	}
	e := &env{t: t, auth: auth, wt: newFakeWatchtower(t), gh: newFakeGitHub(t, o.tag), now: time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)}
	tokenFile := filepath.Join(dir, "watchtower-token")
	if err := os.WriteFile(tokenFile, []byte(testToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	wt := &Watchtower{URL: e.wt.URL, Token: tokenFile}
	if o.wtURL != "" {
		wt.URL = o.wtURL
	}
	if o.noToken {
		wt.Token = filepath.Join(dir, "missing", "token")
	}
	if o.mode == "" {
		o.mode = ModeDocker
	}
	e.checks = &update.Checker{Current: "0.2.0", APIURL: e.gh.URL}
	e.h = New(Config{
		Auth: auth, Updates: e.checks, AutoCheck: true, Watchtower: wt,
		Version: "0.2.0", Started: e.now.Add(-90 * time.Minute), SessionKey: LoadSessionKey(dir, pw),
		Mode: o.mode, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now: e.clock,
	})
	return e
}

func (e *env) clock() time.Time {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.now
}

func (e *env) advance(d time.Duration) {
	e.mu.Lock()
	e.now = e.now.Add(d)
	e.mu.Unlock()
}

type reqOpt func(*http.Request)

func withCookie(c *http.Cookie) reqOpt { return func(r *http.Request) { r.AddCookie(c) } }
func withHeader(k, v string) reqOpt    { return func(r *http.Request) { r.Header.Set(k, v) } }

func (e *env) do(method, target string, form url.Values, opts ...reqOpt) *httptest.ResponseRecorder {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	r := httptest.NewRequest(method, target, body)
	r.RemoteAddr = "203.0.113.7:1234"
	r.Header.Set("Accept-Language", "fr-FR,fr;q=0.9,en;q=0.8")
	if form != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for _, o := range opts {
		o(r)
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	return w
}

func sessionCookieOf(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			return c
		}
	}
	t.Fatalf("no session cookie in %v", w.Header().Values("Set-Cookie"))
	return nil
}

func (e *env) login() *http.Cookie {
	e.t.Helper()
	w := e.do("POST", "/admin/login", url.Values{"password": {testPassword}})
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/admin" {
		e.t.Fatalf("login: %d %s", w.Code, w.Body)
	}
	return sessionCookieOf(e.t, w)
}

var csrfRe = regexp.MustCompile(`name="csrf" value="([^"]+)"`)

func (e *env) csrf(c *http.Cookie) string {
	e.t.Helper()
	w := e.do("GET", "/admin", nil, withCookie(c))
	m := csrfRe.FindStringSubmatch(w.Body.String())
	if m == nil {
		e.t.Fatalf("no csrf token in page: %d", w.Code)
	}
	return m[1]
}

func TestLogin(t *testing.T) {
	e := newEnv(t, envOpts{})
	w := e.do("GET", "/admin", nil)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/admin/login" {
		t.Fatalf("anonymous /admin: %d %q", w.Code, w.Header().Get("Location"))
	}
	w = e.do("GET", "/admin/login", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Se connecter") {
		t.Fatalf("login page: %d", w.Code)
	}
	if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Frame-Options") != "DENY" ||
		!strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Errorf("missing security headers: %v", w.Header())
	}

	w = e.do("POST", "/admin/login", url.Values{"password": {"nope"}})
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "Mot de passe incorrect") {
		t.Fatalf("wrong password: %d", w.Code)
	}
	if len(w.Result().Cookies()) != 0 {
		t.Error("cookie set on failure")
	}

	w = e.do("POST", "/admin/login", url.Values{"password": {testPassword}})
	c := sessionCookieOf(t, w)
	if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/admin" || c.Secure {
		t.Errorf("cookie attributes: %+v", c)
	}
	if strings.Contains(c.Value, testPassword) {
		t.Error("password in cookie")
	}
	// Behind an HTTPS proxy the cookie is Secure.
	w = e.do("POST", "/admin/login", url.Values{"password": {testPassword}}, withHeader("X-Forwarded-Proto", "https"))
	if !sessionCookieOf(t, w).Secure {
		t.Error("cookie not Secure behind https")
	}
	// Cross-site login attempt.
	w = e.do("POST", "/admin/login", url.Values{"password": {testPassword}}, withHeader("Origin", "https://evil.example"))
	if w.Code != http.StatusForbidden {
		t.Errorf("cross-site login: %d", w.Code)
	}

	w = e.do("GET", "/admin", nil, withCookie(c))
	body := w.Body.String()
	if w.Code != 200 || !strings.Contains(body, "v0.2.0") || !strings.Contains(body, "Mises à jour") {
		t.Fatalf("admin page: %d", w.Code)
	}
	for _, secret := range []string{testPassword, testToken} {
		if strings.Contains(body, secret) {
			t.Errorf("secret %q leaked in the page", secret)
		}
	}
	// Logged in: the login page redirects to /admin.
	if w := e.do("GET", "/admin/login", nil, withCookie(c)); w.Code != http.StatusSeeOther {
		t.Errorf("login page with session: %d", w.Code)
	}
}

func TestLoginRateLimit(t *testing.T) {
	e := newEnv(t, envOpts{})
	for i := range 8 {
		if w := e.do("POST", "/admin/login", url.Values{"password": {"wrong"}}); w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: %d", i, w.Code)
		}
	}
	w := e.do("POST", "/admin/login", url.Values{"password": {testPassword}})
	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" || !strings.Contains(w.Body.String(), "Trop de tentatives") {
		t.Fatalf("rate limited login: %d %s", w.Code, w.Header())
	}
	// Another IP is not affected.
	w = e.do("POST", "/admin/login", url.Values{"password": {testPassword}}, func(r *http.Request) { r.RemoteAddr = "198.51.100.1:1" })
	if w.Code != http.StatusSeeOther {
		t.Errorf("other IP: %d", w.Code)
	}
}

func TestSessionExpiryAndLogout(t *testing.T) {
	e := newEnv(t, envOpts{})
	c := e.login()
	e.advance(DefaultSessionTTL - time.Minute)
	if w := e.do("GET", "/admin", nil, withCookie(c)); w.Code != 200 {
		t.Fatalf("session before expiry: %d", w.Code)
	}
	e.advance(2 * time.Minute)
	if w := e.do("GET", "/admin", nil, withCookie(c)); w.Code != http.StatusSeeOther {
		t.Fatalf("expired session accepted: %d", w.Code)
	}
	if w := e.do("GET", "/admin/api/status", nil, withCookie(c)); w.Code != http.StatusUnauthorized {
		t.Fatalf("expired session on API: %d", w.Code)
	}

	// Tampered cookie.
	c = e.login()
	bad := *c
	bad.Value = strings.Replace(c.Value, ".", ".x", 1)
	if w := e.do("GET", "/admin", nil, withCookie(&bad)); w.Code != http.StatusSeeOther {
		t.Errorf("tampered cookie accepted: %d", w.Code)
	}

	// Logout revokes the session.
	tok := e.csrf(c)
	w := e.do("POST", "/admin/logout", url.Values{"csrf": {tok}}, withCookie(c))
	if w.Code != http.StatusSeeOther || !strings.HasPrefix(w.Header().Get("Location"), "/admin/login") {
		t.Fatalf("logout: %d", w.Code)
	}
	if w := e.do("GET", "/admin", nil, withCookie(c)); w.Code != http.StatusSeeOther {
		t.Errorf("session still valid after logout: %d", w.Code)
	}
}

func TestCSRF(t *testing.T) {
	e := newEnv(t, envOpts{})
	c := e.login()
	tok := e.csrf(c)

	cases := []struct {
		name string
		form url.Values
		opts []reqOpt
	}{
		{"no token", url.Values{}, nil},
		{"wrong token", url.Values{"csrf": {"forged"}}, nil},
		{"cross-site origin", url.Values{"csrf": {tok}}, []reqOpt{withHeader("Origin", "https://evil.example")}},
		{"cross-site fetch", url.Values{"csrf": {tok}}, []reqOpt{withHeader("Sec-Fetch-Site", "cross-site")}},
	}
	for _, tc := range cases {
		w := e.do("POST", "/admin/update", tc.form, append([]reqOpt{withCookie(c)}, tc.opts...)...)
		if w.Code != http.StatusForbidden {
			t.Errorf("%s: %d", tc.name, w.Code)
		}
	}
	// Another session's token is rejected too.
	other := e.login()
	if w := e.do("POST", "/admin/update", url.Values{"csrf": {e.csrf(other)}}, withCookie(c)); w.Code != http.StatusForbidden {
		t.Errorf("token of another session: %d", w.Code)
	}
	// JSON API with the session cookie but without X-CSRF-Token.
	if w := e.do("POST", "/admin/api/update", nil, withCookie(c)); w.Code != http.StatusForbidden {
		t.Errorf("API without csrf header: %d", w.Code)
	}
	if n := e.wt.updates.Load(); n != 0 {
		t.Fatalf("Watchtower called %d times despite CSRF failures", n)
	}
	// Without session: back to the login page.
	if w := e.do("POST", "/admin/update", url.Values{"csrf": {tok}}); w.Code != http.StatusSeeOther {
		t.Errorf("no session: %d", w.Code)
	}
}

func TestUpdateTriggersWatchtower(t *testing.T) {
	e := newEnv(t, envOpts{})
	c := e.login()
	tok := e.csrf(c)
	w := e.do("POST", "/admin/update", url.Values{"csrf": {tok}}, withCookie(c), withHeader("Origin", "http://example.com"))
	if w.Code != http.StatusSeeOther {
		t.Fatalf("update: %d %s", w.Code, w.Body)
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "started=") || !strings.Contains(loc, "from=0.2.0") {
		t.Fatalf("redirect %q", loc)
	}
	e.waitJob()
	call, authz := e.wt.lastUpdateCall()
	if call != "POST /v1/update" || authz != "Bearer "+testToken {
		t.Fatalf("watchtower call %q auth %q", call, authz)
	}
	// The job result is exposed to the polling script.
	var st statusView
	_ = json.Unmarshal(e.do("GET", "/admin/api/status", nil, withCookie(c)).Body.Bytes(), &st)
	if st.Job == nil || st.Job.State != "done" || st.Job.Summary == nil || st.Job.Summary.Scanned != 1 || !strings.Contains(loc, "started="+itoa(st.Job.ID)) {
		t.Fatalf("job: %+v", st.Job)
	}
	// The page shows the progress panel with the polling script.
	page, _, _ := strings.Cut(loc, "#") // browsers do not send the fragment
	w = e.do("GET", page, nil, withCookie(c))
	if !strings.Contains(w.Body.String(), "Mise à jour lancée") || !strings.Contains(w.Body.String(), "/healthz") {
		t.Errorf("progress panel missing (%s): %d", loc, w.Code)
	}
	nonce := regexp.MustCompile(`<script nonce="([^"]+)"`).FindStringSubmatch(w.Body.String())
	if nonce == nil || !strings.Contains(w.Header().Get("Content-Security-Policy"), "'nonce-"+nonce[1]+"'") {
		t.Errorf("script nonce / CSP mismatch")
	}

	// Second request within MinUpdateInterval is refused without calling Watchtower.
	w = e.do("POST", "/admin/update", url.Values{"csrf": {tok}}, withCookie(c))
	if !strings.Contains(w.Header().Get("Location"), "m=update_wait") || e.wt.updates.Load() != 1 {
		t.Fatalf("second update: %q, %d calls", w.Header().Get("Location"), e.wt.updates.Load())
	}
	e.advance(MinUpdateInterval + time.Second)

	// JSON API with the access password as bearer token.
	w = e.do("POST", "/admin/api/update", nil, withHeader("Authorization", "Bearer "+testPassword))
	if w.Code != http.StatusAccepted || !strings.Contains(w.Body.String(), `"job_id"`) {
		t.Fatalf("API update: %d %s", w.Code, w.Body)
	}
	if e.waitJob(); e.wt.updates.Load() != 2 {
		t.Fatalf("watchtower calls: %d", e.wt.updates.Load())
	}
	// Busy Watchtower.
	e.advance(MinUpdateInterval + time.Second)
	e.wt.mu.Lock()
	e.wt.status = http.StatusTooManyRequests
	e.wt.mu.Unlock()
	w = e.do("POST", "/admin/api/update", nil, withHeader("Authorization", "Bearer "+testPassword))
	if w.Code != http.StatusAccepted {
		t.Fatalf("update with busy Watchtower: %d %s", w.Code, w.Body)
	}
	// Watchtower's answer is reported through the job, polled by the page.
	if j := e.waitJob(); j.State != "failed" || !strings.Contains(j.Error, "already running") {
		t.Fatalf("busy job: %+v", j)
	}
	// Wrong bearer.
	if w := e.do("POST", "/admin/api/update", nil, withHeader("Authorization", "Bearer nope")); w.Code != http.StatusUnauthorized {
		t.Errorf("wrong bearer: %d", w.Code)
	}
}

func TestStatusJSON(t *testing.T) {
	e := newEnv(t, envOpts{})
	if w := e.do("GET", "/admin/api/status", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status: %d", w.Code)
	}
	e.checks.CheckNow(t.Context())
	w := e.do("GET", "/admin/api/status", nil, withHeader("Authorization", "Bearer "+testPassword))
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status: %d", w.Code)
	}
	if strings.Contains(w.Body.String(), testToken) || strings.Contains(w.Body.String(), testPassword) {
		t.Fatal("secret leaked in the status JSON")
	}
	var st statusView
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if st.Version != "0.2.0" || st.Mode != ModeDocker || st.UptimeSeconds != 5400 || !st.CanUpdate {
		t.Errorf("status: %+v", st)
	}
	if st.Update.Status != "available" || st.Update.Latest == nil || st.Update.Latest.Tag != "v0.3.0" || st.Update.LastCheck == nil {
		t.Errorf("update: %+v", st.Update)
	}
	if st.Watchtower.State != wtOK || st.Watchtower.LastScan == nil || st.Watchtower.LastScan.Scanned != 2 {
		t.Errorf("watchtower: %+v", st.Watchtower)
	}
	// Session cookie works too (used by the page's polling script).
	c := e.login()
	if w := e.do("GET", "/admin/api/status", nil, withCookie(c)); w.Code != 200 {
		t.Errorf("status with session: %d", w.Code)
	}
}

func TestWatchtowerUnreachable(t *testing.T) {
	dead := httptest.NewServer(http.NotFoundHandler())
	deadURL := dead.URL
	dead.Close()
	e := newEnv(t, envOpts{wtURL: deadURL})
	c := e.login()
	w := e.do("GET", "/admin", nil, withCookie(c))
	body := w.Body.String()
	if !strings.Contains(body, "Watchtower injoignable") || !strings.Contains(body, "docker compose pull && docker compose up -d") ||
		!regexp.MustCompile(`type="submit" disabled`).MatchString(body) {
		t.Fatalf("unreachable page:\n%s", body)
	}
	w = e.do("POST", "/admin/api/update", nil, withHeader("Authorization", "Bearer "+testPassword))
	if w.Code != http.StatusConflict {
		t.Fatalf("API update while unreachable: %d %s", w.Code, w.Body)
	}
	// Direct client call reports the connection error.
	if _, err := (&Watchtower{URL: deadURL, Token: "x"}).Run(t.Context()); err == nil || !strings.Contains(err.Error(), "unreachable") {
		t.Errorf("Trigger: %v", err)
	}
}

func TestWatchtowerNotConfigured(t *testing.T) {
	e := newEnv(t, envOpts{noToken: true})
	c := e.login()
	body := e.do("GET", "/admin", nil, withCookie(c)).Body.String()
	if !strings.Contains(body, "Activer la mise à jour en un clic") || !strings.Contains(body, "WATCHTOWER_HTTP_API_PERIODIC_POLLS") {
		t.Fatal("enable instructions missing")
	}
	// A missing token file is never sent as a literal token.
	if (&Watchtower{Token: "/run/watchtower/token"}).Configured() {
		t.Error("missing token file considered configured")
	}
	if tok := (&Watchtower{Token: "literal-token"}).token(); tok != "literal-token" {
		t.Errorf("literal token: %q", tok)
	}
	if e.wt.updates.Load() != 0 {
		t.Error("watchtower called")
	}
}

func TestSourceBuild(t *testing.T) {
	e := newEnv(t, envOpts{mode: ModeSource})
	c := e.login()
	body := e.do("GET", "/admin", nil, withCookie(c), withHeader("Accept-Language", "en-US,en;q=0.9")).Body.String()
	if !strings.Contains(body, "Built from source") || !strings.Contains(body, "docker-compose.build.yml up -d --build") {
		t.Fatal("source build instructions missing")
	}
	w := e.do("POST", "/admin/api/update", nil, withHeader("Authorization", "Bearer "+testPassword))
	if w.Code != http.StatusConflict || e.wt.updates.Load() != 0 {
		t.Fatalf("update on a source build: %d", w.Code)
	}
	if DetectMode("dev") != ModeSource || DetectMode("local") != ModeSource {
		t.Error("DetectMode")
	}
}

func TestGitHubCheck(t *testing.T) {
	e := newEnv(t, envOpts{})
	c := e.login()
	tok := e.csrf(c)
	w := e.do("POST", "/admin/check", url.Values{"csrf": {tok}}, withCookie(c))
	if w.Header().Get("Location") != "/admin?m=checked" || e.gh.hits.Load() != 1 {
		t.Fatalf("check: %q hits=%d", w.Header().Get("Location"), e.gh.hits.Load())
	}
	body := e.do("GET", "/admin?m=checked", nil, withCookie(c)).Body.String()
	for _, want := range []string{"Vérification effectuée", "Mise à jour disponible", "v0.3.0", "2 novembre 2026", "&lt;script&gt;alert(1)&lt;/script&gt;", "Mettre à jour maintenant"} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %q", want)
		}
	}
	if strings.Contains(body, "<script>alert(1)") {
		t.Error("release notes not escaped")
	}
	// Immediate second check: refused, GitHub not called again.
	w = e.do("POST", "/admin/check", url.Values{"csrf": {tok}}, withCookie(c))
	if !strings.HasPrefix(w.Header().Get("Location"), "/admin?m=check_wait") || e.gh.hits.Load() != 1 {
		t.Fatalf("second check: %q hits=%d", w.Header().Get("Location"), e.gh.hits.Load())
	}
	w = e.do("POST", "/admin/api/check", nil, withHeader("Authorization", "Bearer "+testPassword))
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("API check too soon: %d", w.Code)
	}
}

func TestUpToDate(t *testing.T) {
	e := newEnv(t, envOpts{tag: "v0.2.0"})
	e.checks.CheckNow(t.Context())
	c := e.login()
	body := e.do("GET", "/admin?lang=en", nil, withCookie(c)).Body.String()
	for _, want := range []string{"Up to date", "Force update", "No new version is announced"} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %q", want)
		}
	}
}

func TestAuthDisabled(t *testing.T) {
	e := newEnv(t, envOpts{authDisabled: true})
	w := e.do("GET", "/admin", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "AUTH_DISABLED") {
		t.Fatalf("auth disabled page: %d", w.Code)
	}
	c := sessionCookieOf(t, w) // CSRF protection still needs a session
	m := csrfRe.FindStringSubmatch(w.Body.String())
	if w := e.do("POST", "/admin/update", url.Values{"csrf": {m[1]}}, withCookie(c)); w.Code != http.StatusSeeOther {
		t.Fatalf("update: %d", w.Code)
	}
	if e.waitJob(); e.wt.updates.Load() != 1 {
		t.Fatalf("watchtower calls: %d", e.wt.updates.Load())
	}
	if w := e.do("POST", "/admin/update", url.Values{}, withCookie(c)); w.Code != http.StatusForbidden {
		t.Errorf("csrf with auth disabled: %d", w.Code)
	}
}

func TestLanguage(t *testing.T) {
	cases := map[string]string{
		"fr-FR,fr;q=0.9":          "fr",
		"en-US,en;q=0.9,fr;q=0.5": "en",
		"de-DE":                   "en",
		"":                        "fr",
		"de,fr;q=0.3":             "fr",
	}
	for h, want := range cases {
		if got := acceptLang(h); got != want {
			t.Errorf("acceptLang(%q) = %s, want %s", h, got, want)
		}
	}
	e := newEnv(t, envOpts{})
	w := e.do("GET", "/admin/login?lang=en", nil)
	if !strings.Contains(w.Body.String(), "Admin sign in") {
		t.Error("?lang=en ignored")
	}
	var lc *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == langCookie {
			lc = c
		}
	}
	if lc == nil || lc.Value != "en" {
		t.Fatal("lang cookie not set")
	}
	if !strings.Contains(e.do("GET", "/admin/login", nil, withCookie(lc)).Body.String(), "Admin sign in") {
		t.Error("lang cookie ignored")
	}
}

func TestSessionKeyPersisted(t *testing.T) {
	dir := t.TempDir()
	k1 := LoadSessionKey(dir, "pw-one-long-enough")
	k2 := LoadSessionKey(dir, "pw-one-long-enough")
	k3 := LoadSessionKey(dir, "another-password!!")
	if string(k1) != string(k2) {
		t.Error("session key not persisted")
	}
	if string(k1) == string(k3) {
		t.Error("password change must invalidate sessions")
	}
	if fi, err := os.Stat(filepath.Join(dir, sessionKeyFile)); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("key file: %v %v", fi, err)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func TestUpdateJobRunning(t *testing.T) {
	e := newEnv(t, envOpts{})
	block := make(chan struct{})
	e.wt.mu.Lock()
	e.wt.block = block
	e.wt.mu.Unlock()
	bearer := withHeader("Authorization", "Bearer "+testPassword)
	w := e.do("POST", "/admin/api/update", nil, bearer)
	if w.Code != http.StatusAccepted {
		t.Fatalf("update: %d %s", w.Code, w.Body)
	}
	var st statusView
	_ = json.Unmarshal(e.do("GET", "/admin/api/status", nil, bearer).Body.Bytes(), &st)
	if st.Job == nil || st.Job.State != "running" {
		t.Fatalf("job while Watchtower works: %+v", st.Job)
	}
	// A second request while the job runs is refused without calling Watchtower.
	e.advance(MinUpdateInterval + time.Second)
	if w := e.do("POST", "/admin/api/update", nil, bearer); w.Code != http.StatusTooManyRequests {
		t.Errorf("concurrent update: %d", w.Code)
	}
	close(block)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if j := e.h.currentJob(); j != nil && j.State == "done" {
			if e.wt.updates.Load() != 1 {
				t.Errorf("watchtower called %d times", e.wt.updates.Load())
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job never finished")
}

// waitJob waits for the background update job to finish.
func (e *env) waitJob() *updateJob {
	e.t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if j := e.h.currentJob(); j != nil && j.State != "running" {
			return j
		}
		time.Sleep(5 * time.Millisecond)
	}
	e.t.Fatal("update job never finished")
	return nil
}
