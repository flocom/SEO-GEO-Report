// Package oauth implements a minimal OAuth 2.1 authorization server and
// resource-server middleware following the MCP authorization specification,
// so that claude.ai custom connectors (and other MCP clients) can connect
// on their own:
//
//   - RFC 9728 protected resource metadata (/.well-known/oauth-protected-resource)
//   - RFC 8414 authorization server metadata (/.well-known/oauth-authorization-server)
//   - RFC 7591 dynamic client registration (/register)
//   - authorization code grant with mandatory PKCE S256 (/authorize, /token)
//   - refresh tokens with rotation
//
// The resource owner authenticates with a single access password. The same
// password is also accepted directly as a bearer token, which keeps CLI
// clients (Claude Code, scripts) simple.
//
// Every URL is derived per request from the forwarded headers (see BaseURL),
// so no base URL needs to be configured.
package oauth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
)

// Scope is the single scope issued by this server.
const Scope = "seogeo"

// Config configures the server.
type Config struct {
	// DataDir is where oauth.json (clients and hashed tokens) is persisted.
	DataDir string
	// Password is the access password (required unless Disabled).
	Password string
	// Disabled turns authentication off entirely (local use / authless connector).
	Disabled bool
	// PublicURL optionally overrides the auto-detected base URL.
	PublicURL string
	// ExtraRedirectPrefixes are additional allowed redirect URI prefixes for
	// dynamic client registration. "*" allows any https redirect URI.
	ExtraRedirectPrefixes []string
	// AccessTTL and RefreshTTL default to 1 hour and 90 days.
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	// ResourceName is shown in metadata and on the login page.
	ResourceName string
	Logger       *slog.Logger
}

// Server is the OAuth authorization server and resource middleware.
type Server struct {
	cfg   Config
	log   *slog.Logger
	state *state

	codesMu sync.Mutex
	codes   map[string]*authCode // hash(code) -> code

	loginLimiter  *limiter
	bearerLimiter *limiter
}

// New creates the server, loading persisted state from cfg.DataDir.
func New(cfg Config) (*Server, error) {
	if !cfg.Disabled && cfg.Password == "" {
		return nil, errors.New("oauth: a password is required when authentication is enabled")
	}
	if cfg.AccessTTL <= 0 {
		cfg.AccessTTL = time.Hour
	}
	if cfg.RefreshTTL <= 0 {
		cfg.RefreshTTL = 90 * 24 * time.Hour
	}
	if cfg.ResourceName == "" {
		cfg.ResourceName = "SEO & GEO Report"
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	st, err := loadState(cfg.DataDir + "/oauth.json")
	if err != nil {
		return nil, err
	}
	return &Server{
		cfg:           cfg,
		log:           cfg.Logger,
		state:         st,
		codes:         map[string]*authCode{},
		loginLimiter:  newLimiter(8, 15*time.Minute),
		bearerLimiter: newLimiter(60, 15*time.Minute),
	}, nil
}

// Disabled reports whether authentication is off.
func (s *Server) Disabled() bool { return s.cfg.Disabled }

// BaseURL returns the public base URL for a request.
func (s *Server) BaseURL(r *http.Request) string { return BaseURL(r, s.cfg.PublicURL) }

// ResourceMetadataURL is the URL advertised in WWW-Authenticate.
func (s *Server) ResourceMetadataURL(r *http.Request) string {
	return s.BaseURL(r) + "/.well-known/oauth-protected-resource"
}

// Routes registers the OAuth endpoints on mux.
func (s *Server) Routes(mux *http.ServeMux) {
	prm := cors(http.HandlerFunc(s.handleProtectedResource))
	mux.Handle("/.well-known/oauth-protected-resource", prm)
	mux.Handle("/.well-known/oauth-protected-resource/", prm)
	asm := cors(http.HandlerFunc(s.handleAuthServerMetadata))
	mux.Handle("/.well-known/oauth-authorization-server", asm)
	mux.Handle("/.well-known/oauth-authorization-server/", asm)
	mux.Handle("/.well-known/openid-configuration", asm)
	mux.Handle("/.well-known/openid-configuration/", asm)
	mux.Handle("/register", cors(http.HandlerFunc(s.handleRegister)))
	mux.HandleFunc("/authorize", s.handleAuthorize)
	mux.Handle("/token", cors(http.HandlerFunc(s.handleToken)))
	mux.Handle("/revoke", cors(http.HandlerFunc(s.handleRevoke)))
}

// ---------------------------------------------------------------------------
// Metadata

func (s *Server) handleProtectedResource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w, "GET")
		return
	}
	base := s.BaseURL(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"resource":                 base + "/mcp",
		"authorization_servers":    []string{base},
		"bearer_methods_supported": []string{"header"},
		"scopes_supported":         []string{Scope},
		"resource_name":            s.cfg.ResourceName,
	})
}

func (s *Server) handleAuthServerMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w, "GET")
		return
	}
	base := s.BaseURL(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                         base,
		"authorization_endpoint":                         base + "/authorize",
		"token_endpoint":                                 base + "/token",
		"registration_endpoint":                          base + "/register",
		"revocation_endpoint":                            base + "/revoke",
		"response_types_supported":                       []string{"code"},
		"response_modes_supported":                       []string{"query"},
		"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":               []string{"S256"},
		"token_endpoint_auth_methods_supported":          []string{"none", "client_secret_post", "client_secret_basic"},
		"revocation_endpoint_auth_methods_supported":     []string{"none", "client_secret_post", "client_secret_basic"},
		"scopes_supported":                               []string{Scope},
		"authorization_response_iss_parameter_supported": true,
		"service_documentation":                          base + "/",
	})
}

// ---------------------------------------------------------------------------
// Dynamic client registration (RFC 7591)

type registerRequest struct {
	RedirectURIs            []string `json:"redirect_uris"`
	ClientName              string   `json:"client_name"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	Scope                   string   `json:"scope"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, "POST")
		return
	}
	var req registerRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "request body must be a JSON client metadata document")
		return
	}
	if len(req.RedirectURIs) == 0 {
		oauthError(w, http.StatusBadRequest, "invalid_redirect_uri", "redirect_uris is required")
		return
	}
	for _, u := range req.RedirectURIs {
		if !s.redirectAllowed(u) {
			oauthError(w, http.StatusBadRequest, "invalid_redirect_uri",
				fmt.Sprintf("redirect URI %q is not allowed (allowed: claude.ai / claude.com callbacks, http://localhost or loopback callbacks; more can be allowed with OAUTH_REDIRECT_ALLOWLIST)", u))
			return
		}
	}
	method := req.TokenEndpointAuthMethod
	if method == "" {
		method = "client_secret_basic" // RFC 7591 default
	}
	if !slices.Contains([]string{"none", "client_secret_post", "client_secret_basic"}, method) {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "unsupported token_endpoint_auth_method")
		return
	}
	for _, g := range req.GrantTypes {
		if g != "authorization_code" && g != "refresh_token" {
			oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "unsupported grant type "+g)
			return
		}
	}
	for _, rt := range req.ResponseTypes {
		if rt != "code" {
			oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "unsupported response type "+rt)
			return
		}
	}
	grants := req.GrantTypes
	if len(grants) == 0 {
		grants = []string{"authorization_code", "refresh_token"}
	}
	now := time.Now().UTC()
	c := &Client{
		ID:               "c_" + randomString(18),
		Name:             truncate(req.ClientName, 100),
		RedirectURIs:     req.RedirectURIs,
		TokenAuthMethod:  method,
		GrantTypes:       grants,
		Scope:            Scope,
		CreatedAt:        now,
		SecretIssuedAtTS: now.Unix(),
	}
	var secret string
	if method != "none" {
		secret = randomString(32)
		c.SecretHash = hashToken(secret)
	}
	s.state.mu.Lock()
	s.state.data.Clients[c.ID] = c
	err := s.state.saveLocked()
	s.state.mu.Unlock()
	if err != nil {
		s.log.Error("oauth: saving client", "err", err)
		oauthError(w, http.StatusInternalServerError, "server_error", "cannot persist client")
		return
	}
	s.log.Info("oauth: client registered", "client_id", c.ID, "name", c.Name, "redirect_uris", c.RedirectURIs)
	resp := map[string]any{
		"client_id":                  c.ID,
		"client_id_issued_at":        c.SecretIssuedAtTS,
		"client_name":                c.Name,
		"redirect_uris":              c.RedirectURIs,
		"token_endpoint_auth_method": c.TokenAuthMethod,
		"grant_types":                c.GrantTypes,
		"response_types":             []string{"code"},
		"scope":                      Scope,
	}
	if secret != "" {
		resp["client_secret"] = secret
		resp["client_secret_expires_at"] = 0
	}
	writeJSON(w, http.StatusCreated, resp)
}

// redirectAllowed implements the registration redirect URI policy.
func (s *Server) redirectAllowed(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Fragment != "" || u.Scheme == "" {
		return false
	}
	switch raw {
	case "https://claude.ai/api/mcp/auth_callback", "https://claude.com/api/mcp/auth_callback":
		return true
	}
	if u.Scheme == "http" && isLoopback(u.Hostname()) {
		return true
	}
	for _, p := range s.cfg.ExtraRedirectPrefixes {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if p == "*" && u.Scheme == "https" && u.Host != "" {
			return true
		}
		if strings.HasPrefix(raw, p) {
			return true
		}
	}
	return false
}

func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || strings.HasSuffix(host, ".localhost")
}

// redirectMatches compares the redirect_uri of a request with a registered
// one. Loopback redirects may use any port (RFC 8252 §7.3).
func redirectMatches(registered, given string) bool {
	if registered == given {
		return true
	}
	ru, err1 := url.Parse(registered)
	gu, err2 := url.Parse(given)
	if err1 != nil || err2 != nil {
		return false
	}
	return ru.Scheme == "http" && gu.Scheme == "http" &&
		isLoopback(ru.Hostname()) && ru.Hostname() == gu.Hostname() &&
		ru.Path == gu.Path && ru.RawQuery == gu.RawQuery
}

// ---------------------------------------------------------------------------
// Authorization endpoint

type authorizeParams struct {
	ResponseType        string
	ClientID            string
	RedirectURI         string
	State               string
	Scope               string
	CodeChallenge       string
	CodeChallengeMethod string
	Resource            string
}

func readAuthorizeParams(v url.Values) authorizeParams {
	return authorizeParams{
		ResponseType:        v.Get("response_type"),
		ClientID:            v.Get("client_id"),
		RedirectURI:         v.Get("redirect_uri"),
		State:               v.Get("state"),
		Scope:               v.Get("scope"),
		CodeChallenge:       v.Get("code_challenge"),
		CodeChallengeMethod: v.Get("code_challenge_method"),
		Resource:            v.Get("resource"),
	}
}

func (s *Server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src data:; form-action 'self' https: http:; frame-ancestors 'none'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")

	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		methodNotAllowed(w, "GET, POST")
		return
	}
	if err := r.ParseForm(); err != nil {
		s.renderError(w, r, http.StatusBadRequest, "Requête invalide.", "Invalid request.")
		return
	}
	p := readAuthorizeParams(r.Form)

	// Errors about the client or redirect URI must not redirect.
	s.state.mu.Lock()
	client := s.state.data.Clients[p.ClientID]
	s.state.mu.Unlock()
	if client == nil {
		s.renderError(w, r, http.StatusBadRequest,
			"Client inconnu. Supprimez puis ajoutez à nouveau le connecteur.",
			"Unknown client. Remove and add the connector again.")
		return
	}
	if p.RedirectURI == "" && len(client.RedirectURIs) == 1 {
		p.RedirectURI = client.RedirectURIs[0]
	}
	okRedirect := false
	for _, ru := range client.RedirectURIs {
		if redirectMatches(ru, p.RedirectURI) {
			okRedirect = true
			break
		}
	}
	if !okRedirect {
		s.renderError(w, r, http.StatusBadRequest,
			"L'adresse de retour (redirect_uri) ne correspond pas au client enregistré.",
			"The redirect_uri does not match the registered client.")
		return
	}

	// Other errors are returned to the client.
	if p.ResponseType != "code" {
		s.redirectError(w, r, p, "unsupported_response_type", "response_type must be code")
		return
	}
	if p.CodeChallenge == "" || p.CodeChallengeMethod != "S256" {
		s.redirectError(w, r, p, "invalid_request", "PKCE with code_challenge_method=S256 is required")
		return
	}
	if len(p.CodeChallenge) < 43 || len(p.CodeChallenge) > 128 {
		s.redirectError(w, r, p, "invalid_request", "invalid code_challenge")
		return
	}
	if p.Resource != "" && !s.resourceOK(r, p.Resource) {
		s.redirectError(w, r, p, "invalid_target", "unknown resource")
		return
	}

	if r.Method == http.MethodGet {
		s.renderLogin(w, r, p, client, "", http.StatusOK)
		return
	}

	// POST: check the password.
	ip := ClientIP(r)
	if wait := s.loginLimiter.blocked(ip); wait > 0 {
		s.renderLogin(w, r, p, client, fmt.Sprintf("Trop de tentatives. Réessayez dans %d min. / Too many attempts. Try again in %d min.",
			int(wait.Minutes())+1, int(wait.Minutes())+1), http.StatusTooManyRequests)
		return
	}
	if r.PostForm.Get("action") == "deny" {
		s.redirectError(w, r, p, "access_denied", "the user denied access")
		return
	}
	if !s.checkPassword(r.PostForm.Get("password")) {
		s.loginLimiter.fail(ip)
		s.log.Warn("oauth: wrong password on /authorize", "ip", ip, "client_id", client.ID)
		time.Sleep(400 * time.Millisecond)
		s.renderLogin(w, r, p, client, "Mot de passe incorrect. / Wrong password.", http.StatusUnauthorized)
		return
	}
	s.loginLimiter.reset(ip)

	code := randomString(32)
	scope := p.Scope
	if scope == "" {
		scope = Scope
	}
	s.codesMu.Lock()
	now := time.Now()
	for k, c := range s.codes {
		if now.After(c.ExpiresAt) {
			delete(s.codes, k)
		}
	}
	s.codes[hashToken(code)] = &authCode{
		ClientID:    client.ID,
		RedirectURI: p.RedirectURI,
		Challenge:   p.CodeChallenge,
		Scope:       scope,
		Resource:    p.Resource,
		ExpiresAt:   now.Add(5 * time.Minute),
	}
	s.codesMu.Unlock()
	s.log.Info("oauth: authorization granted", "client_id", client.ID, "name", client.Name)

	q := url.Values{}
	q.Set("code", code)
	if p.State != "" {
		q.Set("state", p.State)
	}
	q.Set("iss", s.BaseURL(r))
	http.Redirect(w, r, appendQuery(p.RedirectURI, q), http.StatusFound)
}

func (s *Server) resourceOK(r *http.Request, res string) bool {
	u, err := url.Parse(res)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	path := strings.TrimRight(u.Path, "/")
	// The host is not compared strictly: the browser and the MCP client may
	// reach the server through different hostnames (LAN vs tunnel).
	return path == "" || path == "/mcp"
}

func (s *Server) redirectError(w http.ResponseWriter, r *http.Request, p authorizeParams, code, desc string) {
	q := url.Values{}
	q.Set("error", code)
	q.Set("error_description", desc)
	if p.State != "" {
		q.Set("state", p.State)
	}
	q.Set("iss", s.BaseURL(r))
	http.Redirect(w, r, appendQuery(p.RedirectURI, q), http.StatusFound)
}

func appendQuery(rawURL string, q url.Values) string {
	sep := "?"
	if strings.Contains(rawURL, "?") {
		sep = "&"
	}
	return rawURL + sep + q.Encode()
}

func (s *Server) checkPassword(given string) bool {
	if given == "" || s.cfg.Password == "" {
		return false
	}
	a := sha256.Sum256([]byte(given))
	b := sha256.Sum256([]byte(s.cfg.Password))
	return subtle.ConstantTimeCompare(a[:], b[:]) == 1
}

// ---------------------------------------------------------------------------
// Token endpoint

func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	if r.Method != http.MethodPost {
		methodNotAllowed(w, "POST")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_request", "cannot parse form body")
		return
	}
	client, ok := s.authenticateClient(w, r)
	if !ok {
		return
	}
	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		s.grantAuthorizationCode(w, r, client)
	case "refresh_token":
		s.grantRefreshToken(w, r, client)
	case "":
		oauthError(w, http.StatusBadRequest, "invalid_request", "grant_type is required")
	default:
		oauthError(w, http.StatusBadRequest, "unsupported_grant_type", "supported: authorization_code, refresh_token")
	}
}

// authenticateClient identifies the client from HTTP Basic or form
// credentials. Public clients only send client_id.
func (s *Server) authenticateClient(w http.ResponseWriter, r *http.Request) (*Client, bool) {
	id, secret, basic := r.BasicAuth()
	if basic {
		if v, err := url.QueryUnescape(id); err == nil {
			id = v
		}
		if v, err := url.QueryUnescape(secret); err == nil {
			secret = v
		}
	} else {
		id = r.PostForm.Get("client_id")
		secret = r.PostForm.Get("client_secret")
	}
	if formID := r.PostForm.Get("client_id"); basic && formID != "" && formID != id {
		oauthError(w, http.StatusBadRequest, "invalid_request", "conflicting client_id")
		return nil, false
	}
	s.state.mu.Lock()
	client := s.state.data.Clients[id]
	s.state.mu.Unlock()
	if client == nil {
		w.Header().Set("WWW-Authenticate", `Basic realm="seogeo"`)
		oauthError(w, http.StatusUnauthorized, "invalid_client", "unknown client")
		return nil, false
	}
	if client.SecretHash != "" {
		if secret == "" || subtle.ConstantTimeCompare([]byte(hashToken(secret)), []byte(client.SecretHash)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="seogeo"`)
			oauthError(w, http.StatusUnauthorized, "invalid_client", "invalid client credentials")
			return nil, false
		}
	}
	return client, true
}

func (s *Server) grantAuthorizationCode(w http.ResponseWriter, r *http.Request, client *Client) {
	code := r.PostForm.Get("code")
	verifier := r.PostForm.Get("code_verifier")
	if code == "" || verifier == "" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "code and code_verifier are required")
		return
	}
	s.codesMu.Lock()
	ac := s.codes[hashToken(code)]
	delete(s.codes, hashToken(code)) // single use, even on failure
	s.codesMu.Unlock()
	if ac == nil || time.Now().After(ac.ExpiresAt) {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "invalid or expired authorization code")
		return
	}
	if ac.ClientID != client.ID {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "code was issued to another client")
		return
	}
	if ru := r.PostForm.Get("redirect_uri"); ru != "" && ru != ac.RedirectURI {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "redirect_uri mismatch")
		return
	}
	if !verifyPKCE(verifier, ac.Challenge) {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "PKCE verification failed")
		return
	}
	resource := ac.Resource
	if res := r.PostForm.Get("resource"); res != "" {
		if !s.resourceOK(r, res) {
			oauthError(w, http.StatusBadRequest, "invalid_target", "unknown resource")
			return
		}
		resource = res
	}
	s.issueTokens(w, client, ac.Scope, resource)
}

func (s *Server) grantRefreshToken(w http.ResponseWriter, r *http.Request, client *Client) {
	rt := r.PostForm.Get("refresh_token")
	if rt == "" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "refresh_token is required")
		return
	}
	h := hashToken(rt)
	s.state.mu.Lock()
	old := s.state.data.Refresh[h]
	if old == nil || time.Now().After(old.ExpiresAt) || old.ClientID != client.ID {
		s.state.mu.Unlock()
		oauthError(w, http.StatusBadRequest, "invalid_grant", "invalid or expired refresh token")
		return
	}
	// Rotation: the old refresh token and its access token are revoked.
	delete(s.state.data.Refresh, h)
	if old.Pair != "" {
		delete(s.state.data.Access, old.Pair)
	}
	s.state.mu.Unlock()
	s.issueTokens(w, client, old.Scope, old.Resource)
}

func (s *Server) issueTokens(w http.ResponseWriter, client *Client, scope, resource string) {
	access := randomString(32)
	refresh := randomString(32)
	now := time.Now()
	ah, rh := hashToken(access), hashToken(refresh)
	s.state.mu.Lock()
	s.state.data.Access[ah] = &token{ClientID: client.ID, Scope: scope, Resource: resource, ExpiresAt: now.Add(s.cfg.AccessTTL), Pair: rh}
	s.state.data.Refresh[rh] = &token{ClientID: client.ID, Scope: scope, Resource: resource, ExpiresAt: now.Add(s.cfg.RefreshTTL), Pair: ah}
	err := s.state.saveLocked()
	s.state.mu.Unlock()
	if err != nil {
		s.log.Error("oauth: saving tokens", "err", err)
		oauthError(w, http.StatusInternalServerError, "server_error", "cannot persist tokens")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  access,
		"token_type":    "Bearer",
		"expires_in":    int(s.cfg.AccessTTL.Seconds()),
		"refresh_token": refresh,
		"scope":         scope,
	})
}

func verifyPKCE(verifier, challenge string) bool {
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(computed), []byte(challenge)) == 1
}

// handleRevoke implements RFC 7009 token revocation.
func (s *Server) handleRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, "POST")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_request", "cannot parse form body")
		return
	}
	client, ok := s.authenticateClient(w, r)
	if !ok {
		return
	}
	h := hashToken(r.PostForm.Get("token"))
	s.state.mu.Lock()
	for _, m := range []map[string]*token{s.state.data.Access, s.state.data.Refresh} {
		if t := m[h]; t != nil && t.ClientID == client.ID {
			delete(s.state.data.Access, t.Pair)
			delete(s.state.data.Refresh, t.Pair)
			delete(m, h)
		}
	}
	_ = s.state.saveLocked()
	s.state.mu.Unlock()
	w.WriteHeader(http.StatusOK)
}

// ---------------------------------------------------------------------------
// Resource server middleware

// Protect requires a valid bearer token (an OAuth access token issued by this
// server, or the access password itself) on every request to next.
func (s *Server) Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Disabled || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		ip := ClientIP(r)
		authz := r.Header.Get("Authorization")
		scheme, tok, _ := strings.Cut(strings.TrimSpace(authz), " ")
		tok = strings.TrimSpace(tok)
		if !strings.EqualFold(scheme, "Bearer") || tok == "" {
			s.unauthorized(w, r, "")
			return
		}
		if wait := s.bearerLimiter.blocked(ip); wait > 0 {
			w.Header().Set("Retry-After", fmt.Sprint(int(wait.Seconds())+1))
			http.Error(w, "too many invalid tokens", http.StatusTooManyRequests)
			return
		}
		if s.validBearer(tok) {
			next.ServeHTTP(w, r)
			return
		}
		s.bearerLimiter.fail(ip)
		s.unauthorized(w, r, "invalid_token")
	})
}

func (s *Server) validBearer(tok string) bool {
	if s.checkPassword(tok) {
		return true
	}
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	t := s.state.data.Access[hashToken(tok)]
	return t != nil && time.Now().Before(t.ExpiresAt)
}

func (s *Server) unauthorized(w http.ResponseWriter, r *http.Request, errCode string) {
	v := fmt.Sprintf(`Bearer resource_metadata=%q`, s.ResourceMetadataURL(r))
	if errCode != "" {
		v += fmt.Sprintf(`, error=%q, error_description="the access token is invalid or expired"`, errCode)
	}
	w.Header().Set("WWW-Authenticate", v)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized","error_description":"Authorization: Bearer <token> required. OAuth metadata: ` + s.ResourceMetadataURL(r) + `"}`))
}

// ---------------------------------------------------------------------------
// helpers

// cors allows browser-based MCP clients (MCP Inspector...) to use public
// endpoints. Credentials are never cookies, so a wildcard origin is safe.
func cors(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SetCORS(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// SetCORS writes permissive CORS headers suitable for bearer-token APIs.
func SetCORS(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, Mcp-Session-Id, Mcp-Protocol-Version, Last-Event-ID")
	h.Set("Access-Control-Expose-Headers", "WWW-Authenticate, Mcp-Session-Id, Mcp-Protocol-Version")
	h.Set("Access-Control-Max-Age", "86400")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func oauthError(w http.ResponseWriter, status int, code, desc string) {
	writeJSON(w, status, map[string]string{"error": code, "error_description": desc})
}

func methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// ---------------------------------------------------------------------------
// rate limiting

type limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	m      map[string]*limitEntry
}

type limitEntry struct {
	fails int
	start time.Time
}

func newLimiter(max int, window time.Duration) *limiter {
	return &limiter{max: max, window: window, m: map[string]*limitEntry{}}
}

// blocked returns how long the key is still blocked (0 = allowed).
func (l *limiter) blocked(key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.m[key]
	if e == nil {
		return 0
	}
	if time.Since(e.start) > l.window {
		delete(l.m, key)
		return 0
	}
	if e.fails >= l.max {
		return l.window - time.Since(e.start)
	}
	return 0
}

func (l *limiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if len(l.m) > 10000 { // bound memory
		for k, e := range l.m {
			if now.Sub(e.start) > l.window {
				delete(l.m, k)
			}
		}
	}
	e := l.m[key]
	if e == nil || now.Sub(e.start) > l.window {
		e = &limitEntry{start: now}
		l.m[key] = e
	}
	e.fails++
}

func (l *limiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.m, key)
}
