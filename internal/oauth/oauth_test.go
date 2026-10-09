package oauth

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testPassword = "correct-horse-battery"

func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	s, err := New(Config{DataDir: t.TempDir(), Password: testPassword})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	s.Routes(mux)
	mux.Handle("/mcp", s.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	})))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return s, ts
}

var noRedirect = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func pkce() (verifier, challenge string) {
	verifier = randomString(40)
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

func register(t *testing.T, ts *httptest.Server, body string) map[string]any {
	t.Helper()
	resp, err := http.Post(ts.URL+"/register", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d %v", resp.StatusCode, out)
	}
	return out
}

// authorize posts the login form and returns the redirect Location.
func authorize(t *testing.T, ts *httptest.Server, clientID, redirect, challenge, password string) *http.Response {
	t.Helper()
	form := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirect},
		"state":                 {"xyz"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"resource":              {ts.URL + "/mcp"},
		"password":              {password},
		"action":                {"allow"},
	}
	resp, err := noRedirect.PostForm(ts.URL+"/authorize", form)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

func postToken(t *testing.T, ts *httptest.Server, form url.Values) (int, map[string]any) {
	t.Helper()
	resp, err := http.PostForm(ts.URL+"/token", form)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func callMCP(t *testing.T, ts *httptest.Server, bearer string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("POST", ts.URL+"/mcp", strings.NewReader("{}"))
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

func TestFullFlow(t *testing.T) {
	_, ts := newTestServer(t)
	const redirect = "https://claude.ai/api/mcp/auth_callback"

	// Discovery.
	resp := callMCP(t, ts, "")
	if resp.StatusCode != 401 {
		t.Fatalf("no token: status %d", resp.StatusCode)
	}
	wa := resp.Header.Get("WWW-Authenticate")
	if !strings.Contains(wa, `resource_metadata="`+ts.URL+`/.well-known/oauth-protected-resource"`) {
		t.Fatalf("WWW-Authenticate = %q", wa)
	}

	c := register(t, ts, `{"redirect_uris":["`+redirect+`"],"client_name":"Claude","token_endpoint_auth_method":"none","grant_types":["authorization_code","refresh_token"]}`)
	clientID := c["client_id"].(string)
	if _, ok := c["client_secret"]; ok {
		t.Error("public client must not get a secret")
	}

	// Login page renders.
	verifier, challenge := pkce()
	q := url.Values{"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {redirect},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"}, "state": {"xyz"}}
	g, err := http.Get(ts.URL + "/authorize?" + q.Encode())
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(g.Body)
	g.Body.Close()
	if g.StatusCode != 200 || !strings.Contains(string(page), "Claude") || !strings.Contains(string(page), `name="password"`) {
		t.Fatalf("login page: %d", g.StatusCode)
	}

	// Wrong password.
	if r := authorize(t, ts, clientID, redirect, challenge, "wrong"); r.StatusCode != 401 {
		t.Fatalf("wrong password: status %d", r.StatusCode)
	}

	r := authorize(t, ts, clientID, redirect, challenge, testPassword)
	if r.StatusCode != http.StatusFound {
		t.Fatalf("authorize status %d", r.StatusCode)
	}
	loc, _ := url.Parse(r.Header.Get("Location"))
	if !strings.HasPrefix(loc.String(), redirect+"?") || loc.Query().Get("state") != "xyz" || loc.Query().Get("iss") != ts.URL {
		t.Fatalf("redirect = %s", loc)
	}
	code := loc.Query().Get("code")

	status, tok := postToken(t, ts, url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {clientID},
		"redirect_uri": {redirect}, "code_verifier": {verifier}, "resource": {ts.URL + "/mcp"}})
	if status != 200 {
		t.Fatalf("token: %d %v", status, tok)
	}
	access := tok["access_token"].(string)
	refresh := tok["refresh_token"].(string)
	if tok["token_type"] != "Bearer" {
		t.Errorf("token_type = %v", tok["token_type"])
	}

	// Code is single use.
	if status, _ := postToken(t, ts, url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {clientID},
		"redirect_uri": {redirect}, "code_verifier": {verifier}}); status != 400 {
		t.Errorf("code reuse: status %d", status)
	}

	if r := callMCP(t, ts, access); r.StatusCode != 200 {
		t.Fatalf("access token rejected: %d", r.StatusCode)
	}
	if r := callMCP(t, ts, "garbage"); r.StatusCode != 401 || !strings.Contains(r.Header.Get("WWW-Authenticate"), "invalid_token") {
		t.Fatalf("invalid token: %d %q", r.StatusCode, r.Header.Get("WWW-Authenticate"))
	}
	// The password works as a static bearer token.
	if r := callMCP(t, ts, testPassword); r.StatusCode != 200 {
		t.Fatalf("password bearer rejected: %d", r.StatusCode)
	}

	// Refresh with rotation.
	status, tok2 := postToken(t, ts, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {clientID}})
	if status != 200 {
		t.Fatalf("refresh: %d %v", status, tok2)
	}
	if tok2["refresh_token"] == refresh || tok2["access_token"] == access {
		t.Error("tokens not rotated")
	}
	if status, _ := postToken(t, ts, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {clientID}}); status != 400 {
		t.Errorf("old refresh token still valid: %d", status)
	}
	if r := callMCP(t, ts, access); r.StatusCode != 401 {
		t.Errorf("old access token still valid after rotation: %d", r.StatusCode)
	}
	if r := callMCP(t, ts, tok2["access_token"].(string)); r.StatusCode != 200 {
		t.Errorf("new access token rejected: %d", r.StatusCode)
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	s1, _ := New(Config{DataDir: dir, Password: testPassword})
	c := &Client{ID: "c1", RedirectURIs: []string{"http://localhost/cb"}, TokenAuthMethod: "none"}
	s1.state.mu.Lock()
	s1.state.data.Clients["c1"] = c
	s1.state.mu.Unlock()
	rec := httptest.NewRecorder()
	s1.issueTokens(rec, c, Scope, "")
	var tok map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &tok)

	s2, err := New(Config{DataDir: dir, Password: testPassword})
	if err != nil {
		t.Fatal(err)
	}
	if !s2.validBearer(tok["access_token"].(string)) {
		t.Error("access token lost after restart")
	}
	if fi, _ := os.Stat(filepath.Join(dir, "oauth.json")); fi.Mode().Perm() != 0o600 {
		t.Errorf("oauth.json mode = %v", fi.Mode().Perm())
	}
}

func TestPKCEFailure(t *testing.T) {
	_, ts := newTestServer(t)
	const redirect = "http://localhost:33418/callback"
	c := register(t, ts, `{"redirect_uris":["`+redirect+`"],"token_endpoint_auth_method":"none"}`)
	id := c["client_id"].(string)
	_, challenge := pkce()
	r := authorize(t, ts, id, redirect, challenge, testPassword)
	loc, _ := url.Parse(r.Header.Get("Location"))
	code := loc.Query().Get("code")
	status, out := postToken(t, ts, url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {id},
		"redirect_uri": {redirect}, "code_verifier": {randomString(40)}})
	if status != 400 || out["error"] != "invalid_grant" {
		t.Fatalf("bad verifier accepted: %d %v", status, out)
	}

	// PKCE missing at /authorize -> error redirect.
	form := url.Values{"response_type": {"code"}, "client_id": {id}, "redirect_uri": {redirect}, "state": {"s"}}
	resp, _ := noRedirect.Get(ts.URL + "/authorize?" + form.Encode())
	loc, _ = url.Parse(resp.Header.Get("Location"))
	if resp.StatusCode != 302 || loc.Query().Get("error") != "invalid_request" || loc.Query().Get("state") != "s" {
		t.Fatalf("missing PKCE: %d %s", resp.StatusCode, loc)
	}
	// plain method refused.
	form.Set("code_challenge", challenge)
	form.Set("code_challenge_method", "plain")
	resp, _ = noRedirect.Get(ts.URL + "/authorize?" + form.Encode())
	if loc, _ := url.Parse(resp.Header.Get("Location")); loc.Query().Get("error") != "invalid_request" {
		t.Fatalf("plain PKCE accepted: %s", loc)
	}
}

func TestRedirectURIValidation(t *testing.T) {
	_, ts := newTestServer(t)
	// Registration refuses unknown redirect hosts.
	resp, _ := http.Post(ts.URL+"/register", "application/json", strings.NewReader(`{"redirect_uris":["https://evil.example/cb"]}`))
	if resp.StatusCode != 400 {
		t.Errorf("evil redirect registered: %d", resp.StatusCode)
	}
	resp.Body.Close()

	c := register(t, ts, `{"redirect_uris":["https://claude.ai/api/mcp/auth_callback"],"token_endpoint_auth_method":"none"}`)
	id := c["client_id"].(string)
	_, challenge := pkce()
	r := authorize(t, ts, id, "https://claude.com/api/mcp/auth_callback", challenge, testPassword)
	if r.StatusCode != 400 || r.Header.Get("Location") != "" {
		t.Fatalf("mismatching redirect_uri must not redirect: %d %q", r.StatusCode, r.Header.Get("Location"))
	}

	// Loopback port may differ (RFC 8252).
	c2 := register(t, ts, `{"redirect_uris":["http://127.0.0.1:1234/cb"],"token_endpoint_auth_method":"none"}`)
	r = authorize(t, ts, c2["client_id"].(string), "http://127.0.0.1:5555/cb", challenge, testPassword)
	if r.StatusCode != 302 {
		t.Errorf("loopback with other port refused: %d", r.StatusCode)
	}
}

func TestConfidentialClient(t *testing.T) {
	_, ts := newTestServer(t)
	const redirect = "https://claude.ai/api/mcp/auth_callback"
	c := register(t, ts, `{"redirect_uris":["`+redirect+`"],"token_endpoint_auth_method":"client_secret_post"}`)
	id, secret := c["client_id"].(string), c["client_secret"].(string)
	verifier, challenge := pkce()
	r := authorize(t, ts, id, redirect, challenge, testPassword)
	loc, _ := url.Parse(r.Header.Get("Location"))
	code := loc.Query().Get("code")
	base := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {id}, "redirect_uri": {redirect}, "code_verifier": {verifier}}
	if status, _ := postToken(t, ts, base); status != 401 {
		t.Errorf("missing secret accepted: %d", status)
	}
	// The code was consumed by the failed attempt? No: client auth fails before.
	base.Set("client_secret", secret)
	if status, out := postToken(t, ts, base); status != 200 {
		t.Errorf("secret_post: %d %v", status, out)
	}
}

func TestRateLimit(t *testing.T) {
	_, ts := newTestServer(t)
	const redirect = "https://claude.ai/api/mcp/auth_callback"
	c := register(t, ts, `{"redirect_uris":["`+redirect+`"],"token_endpoint_auth_method":"none"}`)
	id := c["client_id"].(string)
	_, challenge := pkce()
	for range 8 {
		authorize(t, ts, id, redirect, challenge, "nope")
	}
	if r := authorize(t, ts, id, redirect, challenge, testPassword); r.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after repeated failures, got %d", r.StatusCode)
	}
}

func TestMetadataReflectsHost(t *testing.T) {
	s, _ := New(Config{DataDir: t.TempDir(), Password: testPassword})
	mux := http.NewServeMux()
	s.Routes(mux)

	req := httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil)
	req.Host = "internal:8080"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "seo.example.com")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var md map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &md)
	if md["issuer"] != "https://seo.example.com" || md["token_endpoint"] != "https://seo.example.com/token" {
		t.Errorf("metadata = %v", md)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS")
	}

	req = httptest.NewRequest("GET", "/.well-known/oauth-protected-resource/mcp", nil)
	req.Host = "localhost:8080"
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	_ = json.Unmarshal(rec.Body.Bytes(), &md)
	if md["resource"] != "http://localhost:8080/mcp" {
		t.Errorf("resource = %v", md["resource"])
	}
}

func TestBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		headers  map[string]string
		tls      bool
		override string
		want     string
	}{
		{"plain host", "localhost:8080", nil, false, "", "http://localhost:8080"},
		{"tls", "example.com", nil, true, "", "https://example.com"},
		{"x-forwarded", "seogeo:8080", map[string]string{"X-Forwarded-Proto": "https", "X-Forwarded-Host": "seo.example.com"}, false, "", "https://seo.example.com"},
		{"x-forwarded lists", "seogeo:8080", map[string]string{"X-Forwarded-Proto": "https, http", "X-Forwarded-Host": "a.example.com, b"}, false, "", "https://a.example.com"},
		{"x-forwarded port", "seogeo:8080", map[string]string{"X-Forwarded-Proto": "https", "X-Forwarded-Host": "a.example.com", "X-Forwarded-Port": "8443"}, false, "", "https://a.example.com:8443"},
		{"default port dropped", "seogeo", map[string]string{"X-Forwarded-Proto": "https", "X-Forwarded-Host": "a.example.com:443"}, false, "", "https://a.example.com"},
		{"rfc7239", "seogeo:8080", map[string]string{"Forwarded": `for=1.2.3.4;proto=https;host="r.example.org", for=5.6.7.8`}, false, "", "https://r.example.org"},
		{"proto only", "tunnel.example.net", map[string]string{"X-Forwarded-Proto": "https"}, false, "", "https://tunnel.example.net"},
		{"cf-visitor", "tunnel.example.net", map[string]string{"CF-Visitor": `{"scheme":"https"}`}, false, "", "https://tunnel.example.net"},
		{"override", "x", map[string]string{"X-Forwarded-Host": "y"}, false, "https://fixed.example.com/", "https://fixed.example.com"},
		{"injection rejected", "good.example.com", map[string]string{"X-Forwarded-Host": `evil.com/"><script>`}, false, "", "http://good.example.com"},
		{"ipv6", "[::1]:8080", nil, false, "", "http://[::1]:8080"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.Host = tt.host
			for k, v := range tt.headers {
				r.Header.Set(k, v)
			}
			if tt.tls {
				r.TLS = &tls.ConnectionState{}
			}
			if got := BaseURL(r, tt.override); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEnsurePassword(t *testing.T) {
	dir := t.TempDir()
	p1, src, err := EnsurePassword(dir, "")
	if err != nil || src != PasswordGenerated || len(p1) < 20 {
		t.Fatalf("generate: %q %v %v", p1, src, err)
	}
	p2, src, _ := EnsurePassword(dir, "")
	if p2 != p1 || src != PasswordFromFile {
		t.Errorf("reload: %q %v", p2, src)
	}
	p3, src, _ := EnsurePassword(dir, "my-env-password")
	if p3 != "my-env-password" || src != PasswordFromEnv {
		t.Errorf("env: %q %v", p3, src)
	}
	if _, _, err := EnsurePassword(dir, "short"); err == nil {
		t.Error("short env password accepted")
	}
}

func TestDisabled(t *testing.T) {
	s, err := New(Config{DataDir: t.TempDir(), Disabled: true})
	if err != nil {
		t.Fatal(err)
	}
	h := s.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/mcp", nil))
	if rec.Code != 204 {
		t.Errorf("auth disabled: %d", rec.Code)
	}
}
