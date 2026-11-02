// Package admin serves the small administration page of the HTTP server
// (/admin): installed version, latest GitHub release, "check for updates"
// and "update now" buttons. The update itself is delegated to the Watchtower
// container through its HTTP API, so this container never needs the Docker
// socket.
//
// Routes (all under /admin, all authenticated):
//
//	GET  /admin             status page (redirects to /admin/login without a session)
//	GET  /admin/login       login form (access password)
//	POST /admin/login       sign in → signed session cookie
//	POST /admin/logout      sign out
//	POST /admin/check       force a GitHub check
//	POST /admin/update      ask Watchtower to update now
//	GET  /admin/api/status  JSON status (session cookie or Authorization: Bearer)
//	POST /admin/api/check   JSON, force a GitHub check
//	POST /admin/api/update  JSON, ask Watchtower to update now
package admin

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/flocom/SEO-GEO-Report/internal/oauth"
	"github.com/flocom/SEO-GEO-Report/internal/update"
)

const (
	sessionCookie = "seogeo_admin"
	langCookie    = "seogeo_lang"
	// DefaultSessionTTL is the lifetime of an admin session.
	DefaultSessionTTL = 8 * time.Hour
	// MinUpdateInterval between two update requests sent to Watchtower.
	MinUpdateInterval = 30 * time.Second
	maxNotes          = 4000
	maxFormBytes      = 16 << 10
)

// Installation modes.
const (
	ModeDocker = "docker" // release image in a container: Watchtower can update it
	ModeSource = "source" // built from source (non-release version)
	ModeBinary = "binary" // release binary outside a container
)

// Config configures the admin pages.
type Config struct {
	// Auth provides the access password check (shared rate limiter), the
	// bearer validation of the API and the public base URL. Required.
	Auth *oauth.Server
	// Updates checks GitHub releases. Required.
	Updates *update.Checker
	// AutoCheck tells whether Updates runs periodically (UPDATE_CHECK).
	AutoCheck bool
	// Watchtower is the Watchtower API client (nil = one-click update off).
	Watchtower *Watchtower
	// Version is the running version.
	Version string
	// Started is the process start time.
	Started time.Time
	// SessionKey signs the session cookies (see LoadSessionKey).
	SessionKey []byte
	// SessionTTL defaults to DefaultSessionTTL.
	SessionTTL time.Duration
	// Mode is the installation mode ("" = detected, see DetectMode).
	Mode   string
	Logger *slog.Logger
	// Now is the clock (tests).
	Now func() time.Time
}

// Handler serves /admin.
type Handler struct {
	cfg  Config
	sess *sessions
	mux  *http.ServeMux
	log  *slog.Logger

	mu          sync.Mutex
	lastTrigger time.Time
	job         *updateJob
}

//go:embed page.html
var pageFS embed.FS

var pageTmpl = template.Must(template.New("").Funcs(template.FuncMap{}).ParseFS(pageFS, "page.html"))

// New returns the admin handler.
func New(cfg Config) *Handler {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = DefaultSessionTTL
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Started.IsZero() {
		cfg.Started = cfg.Now()
	}
	if cfg.Mode == "" {
		cfg.Mode = DetectMode(cfg.Version)
	}
	if len(cfg.SessionKey) == 0 {
		k := make([]byte, 32)
		_, _ = rand.Read(k)
		cfg.SessionKey = k
	}
	h := &Handler{cfg: cfg, log: cfg.Logger, sess: newSessions(cfg.SessionKey, cfg.SessionTTL, cfg.Now)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin", h.page)
	mux.HandleFunc("GET /admin/{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /admin/login", h.loginPage)
	mux.HandleFunc("POST /admin/login", h.login)
	mux.HandleFunc("POST /admin/logout", h.form(h.logout))
	mux.HandleFunc("POST /admin/check", h.form(h.checkForm))
	mux.HandleFunc("POST /admin/update", h.form(h.updateForm))
	mux.HandleFunc("GET /admin/api/status", h.api(h.apiStatus))
	mux.HandleFunc("POST /admin/api/check", h.api(h.apiCheck))
	mux.HandleFunc("POST /admin/api/update", h.api(h.apiUpdate))
	h.mux = mux
	return h
}

// DetectMode returns ModeSource for non-release versions ("dev", "local"...),
// ModeDocker inside a container and ModeBinary otherwise.
func DetectMode(version string) string {
	if !update.Newer(version, "0.0.0") {
		return ModeSource
	}
	for _, f := range []string{"/.dockerenv", "/run/.containerenv"} {
		if _, err := os.Stat(f); err == nil {
			return ModeDocker
		}
	}
	return ModeBinary
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hd := w.Header()
	hd.Set("Cache-Control", "no-store")
	hd.Set("Pragma", "no-cache")
	hd.Set("X-Robots-Tag", "noindex, nofollow")
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("X-Frame-Options", "DENY")
	// same-origin (not no-referrer): with no-referrer browsers send
	// "Origin: null" on form posts, which sameOrigin rejects.
	hd.Set("Referrer-Policy", "same-origin")
	h.mux.ServeHTTP(w, r)
}

// ---------------------------------------------------------------------------
// Sessions, CSRF, origin checks

func (h *Handler) authDisabled() bool { return h.cfg.Auth == nil || h.cfg.Auth.Disabled() }

func (h *Handler) secure(r *http.Request) bool {
	return r.TLS != nil || strings.HasPrefix(h.cfg.Auth.BaseURL(r), "https://")
}

func (h *Handler) currentSession(r *http.Request) (session, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return session{}, false
	}
	ss, err := h.sess.parse(c.Value)
	return ss, err == nil
}

func (h *Handler) setSession(w http.ResponseWriter, r *http.Request) session {
	ss, v := h.sess.issue()
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: v, Path: "/admin",
		Expires: ss.Exp, MaxAge: int(h.cfg.SessionTTL.Seconds()),
		HttpOnly: true, Secure: h.secure(r), SameSite: http.SameSiteStrictMode,
	})
	return ss
}

func (h *Handler) clearSession(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/admin", MaxAge: -1,
		HttpOnly: true, Secure: h.secure(r), SameSite: http.SameSiteStrictMode,
	})
}

// sameOrigin rejects cross-site POSTs (defense in depth on top of the CSRF
// token and the SameSite=Strict cookie).
func (h *Handler) sameOrigin(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "", "same-origin", "none":
	default:
		return false
	}
	o := r.Header.Get("Origin")
	if o == "" {
		return true
	}
	u, err := url.Parse(o)
	if err != nil || u.Host == "" {
		return false
	}
	host := strings.ToLower(u.Host)
	if host == strings.ToLower(r.Host) {
		return true
	}
	if b, err := url.Parse(h.cfg.Auth.BaseURL(r)); err == nil && host == strings.ToLower(b.Host) {
		return true
	}
	return false
}

// form wraps the POST form actions: same origin, valid session, CSRF token.
func (h *Handler) form(fn func(http.ResponseWriter, *http.Request, session)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lang := pickLang(r)
		r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
		if !h.sameOrigin(r) {
			h.renderMessage(w, r, lang, http.StatusForbidden, tr(lang, "flash_csrf"))
			return
		}
		ss, ok := h.currentSession(r)
		if !ok {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		if err := r.ParseForm(); err != nil || !h.sess.validCSRF(ss, r.PostForm.Get("csrf")) {
			h.renderMessage(w, r, lang, http.StatusForbidden, tr(lang, "flash_csrf"))
			return
		}
		fn(w, r, ss)
	}
}

// api wraps the JSON endpoints: either "Authorization: Bearer <password or
// OAuth access token>" (scripts), or the session cookie plus, for POST, the
// X-CSRF-Token header (the page itself).
func (h *Handler) api(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			h.cfg.Auth.Protect(fn).ServeHTTP(w, r)
			return
		}
		ss, ok := h.currentSession(r)
		if !ok {
			if h.authDisabled() && r.Method == http.MethodGet {
				fn(w, r)
				return
			}
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized",
				"error_description": "sign in on /admin or send Authorization: Bearer <access password>"})
			return
		}
		if r.Method != http.MethodGet && (!h.sameOrigin(r) || !h.sess.validCSRF(ss, r.Header.Get("X-CSRF-Token"))) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "csrf", "error_description": "missing or invalid X-CSRF-Token header"})
			return
		}
		fn(w, r)
	}
}

// ---------------------------------------------------------------------------
// Login / logout

func (h *Handler) loginPage(w http.ResponseWriter, r *http.Request) {
	lang := h.langCookie(w, r)
	if _, ok := h.currentSession(r); ok || h.authDisabled() {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	msg := ""
	if r.URL.Query().Get("m") == "logged_out" {
		msg = tr(lang, "flash_logged_out")
	}
	h.render(w, r, http.StatusOK, "login", h.baseData(r, lang, ""), func(d *pageData) { d.Info = msg })
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	lang := pickLang(r)
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if !h.sameOrigin(r) {
		h.renderMessage(w, r, lang, http.StatusForbidden, tr(lang, "flash_csrf"))
		return
	}
	if h.authDisabled() {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	ok, wait := h.cfg.Auth.VerifyPassword(r, r.PostForm.Get("password"))
	if !ok {
		status, msg := http.StatusUnauthorized, tr(lang, "wrong_password")
		if wait > 0 {
			status, msg = http.StatusTooManyRequests, tr(lang, "too_many", int(wait.Minutes())+1)
			w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		}
		h.render(w, r, status, "login", h.baseData(r, lang, ""), func(d *pageData) { d.Error = msg })
		return
	}
	h.setSession(w, r)
	h.log.Info("admin: signed in", "ip", oauth.ClientIP(r))
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request, ss session) {
	h.sess.revoke(ss)
	h.clearSession(w, r)
	http.Redirect(w, r, "/admin/login?m=logged_out", http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// Actions

func (h *Handler) checkForm(w http.ResponseWriter, r *http.Request, _ session) {
	wait, err := h.cfg.Updates.ForceCheck(r.Context())
	switch {
	case errors.Is(err, update.ErrTooSoon):
		redirectFlash(w, r, "check_wait", int(wait.Seconds())+1)
	case err != nil:
		redirectFlash(w, r, "check_failed", 0)
	default:
		redirectFlash(w, r, "checked", 0)
	}
}

// updateJob is the last update requested from this page.
type updateJob struct {
	ID         int64        `json:"id"`    // start time, Unix seconds
	State      string       `json:"state"` // running, done, failed
	StartedAt  time.Time    `json:"started_at"`
	FinishedAt *time.Time   `json:"finished_at,omitempty"`
	Summary    *ScanSummary `json:"summary,omitempty"`
	Error      string       `json:"error,omitempty"`
}

// jobDelay postpones the call to Watchtower so that the browser has loaded
// the progress page (served by this very process) before Watchtower may stop
// the container. Errors (busy, rejected token...) are then reported through
// the job state, polled by the page.
var jobDelay = 2 * time.Second

// trigger asks Watchtower for an update in the background. It returns a
// flash/API code ("started" or an error code), the wait before a retry and
// the job ID. Watchtower reachability and token are checked beforehand
// (canUpdate).
func (h *Handler) trigger(ctx context.Context, r *http.Request) (string, time.Duration, int64) {
	if !h.canUpdate(ctx) {
		return "update_unavailable", 0, 0
	}
	h.mu.Lock()
	if wait := MinUpdateInterval - h.cfg.Now().Sub(h.lastTrigger); !h.lastTrigger.IsZero() && wait > 0 {
		h.mu.Unlock()
		return "update_wait", wait, 0
	}
	if h.job != nil && h.job.State == "running" {
		h.mu.Unlock()
		return "update_busy", 30 * time.Second, 0
	}
	now := h.cfg.Now()
	h.lastTrigger = now
	job := &updateJob{ID: now.Unix(), State: "running", StartedAt: now.UTC()}
	h.job = job
	h.mu.Unlock()

	h.log.Info("admin: update requested from Watchtower", "version", h.cfg.Version, "ip", oauth.ClientIP(r))
	go func() {
		time.Sleep(jobDelay)
		// Not bound to the HTTP request: the update outlives it.
		jctx, cancel := context.WithTimeout(context.Background(), 11*time.Minute)
		defer cancel()
		sum, err := h.cfg.Watchtower.Run(jctx)
		fin := h.cfg.Now().UTC()
		h.mu.Lock()
		job.FinishedAt = &fin
		if err != nil {
			job.State, job.Error = "failed", shortErr(err)
		} else {
			job.State, job.Summary = "done", sum
		}
		h.mu.Unlock()
		if err != nil {
			h.log.Warn("admin: update failed", "err", err)
		} else {
			h.log.Info("admin: update finished", "scanned", sum.Scanned, "updated", sum.Updated, "failed", sum.Failed)
		}
	}()
	return "started", 0, job.ID
}

// currentJob returns a copy of the last update job (nil if none).
func (h *Handler) currentJob() *updateJob {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.job == nil {
		return nil
	}
	j := *h.job
	return &j
}

func (h *Handler) updateForm(w http.ResponseWriter, r *http.Request, _ session) {
	code, wait, id := h.trigger(r.Context(), r)
	if code == "started" {
		q := url.Values{"started": {strconv.FormatInt(id, 10)}, "from": {h.cfg.Version}}
		http.Redirect(w, r, "/admin?"+q.Encode()+"#progress", http.StatusSeeOther)
		return
	}
	redirectFlash(w, r, code, int(wait.Seconds()))
}

func redirectFlash(w http.ResponseWriter, r *http.Request, code string, seconds int) {
	q := url.Values{"m": {code}}
	if seconds > 0 {
		q.Set("s", strconv.Itoa(seconds))
	}
	http.Redirect(w, r, "/admin?"+q.Encode(), http.StatusSeeOther)
}

// canUpdate reports whether the one-click update is possible.
func (h *Handler) canUpdate(ctx context.Context) bool {
	if h.cfg.Mode != ModeDocker || !h.cfg.Watchtower.Configured() {
		return false
	}
	st := h.cfg.Watchtower.Status(ctx, 15*time.Second)
	return st.State == wtOK
}

// ---------------------------------------------------------------------------
// JSON API

type statusView struct {
	Version       string           `json:"version"`
	GoVersion     string           `json:"go_version"`
	Platform      string           `json:"platform"`
	Revision      string           `json:"revision,omitempty"`
	BuildTime     string           `json:"build_time,omitempty"`
	StartedAt     time.Time        `json:"started_at"`
	UptimeSeconds int64            `json:"uptime_seconds"`
	Mode          string           `json:"mode"`
	Update        updateView       `json:"update"`
	Watchtower    WatchtowerStatus `json:"watchtower"`
	CanUpdate     bool             `json:"can_update"`
	// Job is the last update requested from /admin since the start of this
	// process (it is lost when the update restarts the container).
	Job *updateJob `json:"job,omitempty"`
}

type updateView struct {
	// Status: up_to_date, available, unknown (never checked), not_comparable
	// (development build).
	Status    string       `json:"status"`
	Latest    *releaseView `json:"latest,omitempty"`
	LastCheck *time.Time   `json:"last_check,omitempty"`
	LastError string       `json:"last_error,omitempty"`
	AutoCheck bool         `json:"auto_check"`
}

type releaseView struct {
	Tag            string    `json:"tag"`
	Name           string    `json:"name,omitempty"`
	URL            string    `json:"url"`
	PublishedAt    time.Time `json:"published_at,omitzero"`
	Notes          string    `json:"notes,omitempty"`
	NotesTruncated bool      `json:"notes_truncated,omitempty"`
}

const releasesURL = "https://github.com/" + update.Repo + "/releases"

func (h *Handler) status(ctx context.Context) statusView {
	now := h.cfg.Now()
	v := statusView{
		Version:       h.cfg.Version,
		GoVersion:     runtime.Version(),
		Platform:      runtime.GOOS + "/" + runtime.GOARCH,
		StartedAt:     h.cfg.Started.UTC(),
		UptimeSeconds: int64(now.Sub(h.cfg.Started).Seconds()),
		Mode:          h.cfg.Mode,
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				v.Revision = s.Value
				if len(v.Revision) > 12 {
					v.Revision = v.Revision[:12]
				}
			case "vcs.time":
				v.BuildTime = s.Value
			}
		}
	}
	u := updateView{AutoCheck: h.cfg.AutoCheck, Status: "unknown"}
	if t, err := h.cfg.Updates.LastCheck(); !t.IsZero() {
		t := t.UTC()
		u.LastCheck = &t
		if err != nil {
			u.LastError = shortErr(err)
		}
	}
	if rel := h.cfg.Updates.Latest(); rel != nil {
		rv := &releaseView{Tag: rel.Tag, Name: rel.Name, URL: rel.URL, PublishedAt: rel.PublishedAt}
		if !strings.HasPrefix(rv.URL, "https://github.com/") {
			rv.URL = releasesURL
		}
		rv.Notes = strings.TrimSpace(strings.ReplaceAll(rel.Body, "\r\n", "\n"))
		if utf8.RuneCountInString(rv.Notes) > maxNotes {
			rv.Notes = string([]rune(rv.Notes)[:maxNotes]) + "…"
			rv.NotesTruncated = true
		}
		u.Latest = rv
		switch {
		case !update.Newer(h.cfg.Version, "0.0.0"):
			u.Status = "not_comparable"
		case update.Newer(rel.Tag, h.cfg.Version):
			u.Status = "available"
		default:
			u.Status = "up_to_date"
		}
	} else if !update.Newer(h.cfg.Version, "0.0.0") {
		u.Status = "not_comparable"
	}
	v.Update = u
	if h.cfg.Watchtower != nil {
		v.Watchtower = h.cfg.Watchtower.Status(ctx, 3*time.Second)
	} else {
		v.Watchtower = WatchtowerStatus{State: wtNotConfigured}
	}
	v.CanUpdate = h.cfg.Mode == ModeDocker && v.Watchtower.State == wtOK
	v.Job = h.currentJob()
	return v
}

func (h *Handler) apiStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.status(r.Context()))
}

func (h *Handler) apiCheck(w http.ResponseWriter, r *http.Request) {
	wait, err := h.cfg.Updates.ForceCheck(r.Context())
	switch {
	case errors.Is(err, update.ErrTooSoon):
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "too_soon", "retry_after": int(wait.Seconds()) + 1})
	case err != nil:
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "github", "error_description": shortErr(err)})
	default:
		writeJSON(w, http.StatusOK, h.status(r.Context()))
	}
}

func (h *Handler) apiUpdate(w http.ResponseWriter, r *http.Request) {
	code, wait, id := h.trigger(r.Context(), r)
	status := map[string]int{
		"started":             http.StatusAccepted,
		"update_unavailable":  http.StatusConflict,
		"update_wait":         http.StatusTooManyRequests,
		"update_busy":         http.StatusTooManyRequests,
		"update_unreachable":  http.StatusServiceUnavailable,
		"update_unauthorized": http.StatusBadGateway,
		"update_failed":       http.StatusBadGateway,
	}[code]
	body := map[string]any{"status": "started", "job_id": id, "from_version": h.cfg.Version}
	if code != "started" {
		body = map[string]any{"error": strings.TrimPrefix(code, "update_"), "error_description": tr("en", "flash_"+code)}
	}
	if wait > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		body["retry_after"] = int(wait.Seconds()) + 1
	}
	writeJSON(w, status, body)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// ---------------------------------------------------------------------------
// HTML pages

type pageData struct {
	Lang, OtherLang string
	Nonce           string
	CSRF            string
	AuthDisabled    bool
	Error, Info     string
	S               statusView
	Mode            string
	Uptime          string
	StartedAt       string
	BootID          string
	LastCheck       string
	Published       string
	JobTime         string
	StatusText      string
	StatusClass     string
	Flash           string
	FlashKind       string
	// Progress panel after an update request.
	Progress     bool
	FromVersion  string
	StartedUnix  int64
	ProgressDone string
	Snippet      string
}

// T translates a key (template helper).
func (d pageData) T(key string, args ...any) string { return tr(d.Lang, key, args...) }

func (h *Handler) baseData(r *http.Request, lang, csrf string) pageData {
	other := "en"
	if lang == "en" {
		other = "fr"
	}
	nonce := make([]byte, 16)
	_, _ = rand.Read(nonce)
	return pageData{Lang: lang, OtherLang: other, Nonce: hex.EncodeToString(nonce), CSRF: csrf, AuthDisabled: h.authDisabled(), Snippet: composeSnippet}
}

func (h *Handler) render(w http.ResponseWriter, r *http.Request, status int, name string, d pageData, edit func(*pageData)) {
	if edit != nil {
		edit(&d)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src 'self' data:; script-src 'nonce-"+d.Nonce+"'; connect-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	w.WriteHeader(status)
	if err := pageTmpl.ExecuteTemplate(w, name, d); err != nil {
		h.log.Error("admin: template", "err", err)
	}
}

func (h *Handler) renderMessage(w http.ResponseWriter, r *http.Request, lang string, status int, msg string) {
	h.render(w, r, status, "message", h.baseData(r, lang, ""), func(d *pageData) { d.Error = msg })
}

// langCookie applies ?lang= and remembers it.
func (h *Handler) langCookie(w http.ResponseWriter, r *http.Request) string {
	lang := pickLang(r)
	if l := normLang(r.URL.Query().Get("lang")); l != "" {
		http.SetCookie(w, &http.Cookie{Name: langCookie, Value: l, Path: "/admin", MaxAge: 365 * 24 * 3600,
			HttpOnly: true, Secure: h.secure(r), SameSite: http.SameSiteLaxMode})
	}
	return lang
}

func (h *Handler) page(w http.ResponseWriter, r *http.Request) {
	lang := h.langCookie(w, r)
	ss, ok := h.currentSession(r)
	if !ok {
		if !h.authDisabled() {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		ss = h.setSession(w, r)
	}
	st := h.status(r.Context())
	d := h.baseData(r, lang, h.sess.csrf(ss))
	d.S = st
	d.Mode = h.cfg.Mode
	d.Uptime = formatDuration(lang, time.Duration(st.UptimeSeconds)*time.Second)
	d.StartedAt = formatTime(lang, h.cfg.Started)
	d.BootID = h.cfg.Started.UTC().Format(time.RFC3339)
	d.LastCheck = tr(lang, "never")
	if st.Update.LastCheck != nil {
		d.LastCheck = formatTime(lang, *st.Update.LastCheck)
	}
	if st.Update.Latest != nil {
		d.Published = formatDate(lang, st.Update.Latest.PublishedAt)
	}
	if st.Job != nil {
		d.JobTime = formatTime(lang, st.Job.StartedAt)
	}
	switch st.Update.Status {
	case "available":
		d.StatusText, d.StatusClass = tr(lang, "status_available"), "warn"
	case "up_to_date":
		d.StatusText, d.StatusClass = tr(lang, "status_uptodate"), "ok"
	case "not_comparable":
		d.StatusText, d.StatusClass = tr(lang, "status_dev", h.cfg.Version), "muted"
	default:
		d.StatusText, d.StatusClass = tr(lang, "status_unknown"), "muted"
	}
	q := r.URL.Query()
	if m := q.Get("m"); m != "" {
		if _, known := messages["flash_"+m]; known {
			s, _ := strconv.Atoi(q.Get("s"))
			if strings.Contains(messages["flash_"+m][0], "%d") {
				d.Flash = tr(lang, "flash_"+m, max(s, 1))
			} else {
				d.Flash = tr(lang, "flash_"+m)
			}
			d.FlashKind = "ok"
			if m != "checked" {
				d.FlashKind = "err"
			}
		}
	}
	if started, err := strconv.ParseInt(q.Get("started"), 10, 64); err == nil {
		age := h.cfg.Now().Sub(time.Unix(started, 0))
		if age >= -time.Minute && age < 15*time.Minute {
			d.Progress = true
			d.StartedUnix = started
			d.FromVersion = q.Get("from")
			if d.FromVersion != "" && d.FromVersion != h.cfg.Version {
				d.ProgressDone = tr(lang, "progress_done", strings.TrimPrefix(h.cfg.Version, "v"))
			}
		}
	}
	h.render(w, r, http.StatusOK, "admin", d, nil)
}

// composeSnippet enables one-click updates in an existing compose file.
const composeSnippet = `services:
  seogeo:
    environment:
      WATCHTOWER_HTTP_API_TOKEN: ${WATCHTOWER_HTTP_API_TOKEN:-/run/watchtower/token}
    volumes:
      - watchtower-api:/run/watchtower:ro
  watchtower:
    environment:
      WATCHTOWER_HTTP_API_ENDPOINTS: update,metrics
      WATCHTOWER_HTTP_API_PERIODIC_POLLS: "true"
      WATCHTOWER_HTTP_API_TOKEN: ${WATCHTOWER_HTTP_API_TOKEN:-/run/watchtower/token}
    volumes:
      - watchtower-api:/run/watchtower:ro
    depends_on:
      watchtower-token:
        condition: service_completed_successfully
  watchtower-token:
    image: mirror.gcr.io/library/busybox:1.37
    command: ["sh", "-c", "umask 077; f=/run/watchtower/token; [ -s $$f ] || head -c 32 /dev/urandom | od -An -tx1 | tr -dc 0-9a-f > $$f; chown 10001:10001 $$f"]
    volumes:
      - watchtower-api:/run/watchtower
    restart: "no"
volumes:
  watchtower-api:`
