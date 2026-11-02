package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/flocom/SEO-GEO-Report/internal/model"
	"github.com/flocom/SEO-GEO-Report/internal/oauth"
	"github.com/flocom/SEO-GEO-Report/internal/pdf"
	"github.com/flocom/SEO-GEO-Report/internal/store"
)

// MaxRequestBytes bounds MCP request bodies (CSV imports can be large).
const MaxRequestBytes = 32 << 20

// NewHTTPHandler returns the complete HTTP application:
//
//	/mcp                     Streamable HTTP MCP endpoint (bearer auth)
//	/.well-known/...         OAuth metadata, /register, /authorize, /token
//	/r/<token>               hosted report (HTML, ?lang=fr|en, ?view=essential|advanced)
//	/r/<token>/download      standalone HTML file (attachment, both versions)
//	/r/<token>.pdf           PDF (also /r/<token>/pdf), ?download=1 for attachment,
//	                         ?view=essential (short) or advanced (full report)
//	/healthz                 health check
//	/admin                   update page (when Config.Admin is set)
//	/                        home page with connection instructions
func NewHTTPHandler(svc *Service, auth *oauth.Server, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	h := &httpApp{svc: svc, auth: auth, log: logger}
	srv := svc.NewServer()
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, &mcp.StreamableHTTPOptions{
		Stateless:           true,
		JSONResponse:        true,
		MaxRequestBodyBytes: MaxRequestBytes,
		// Requests are authenticated with bearer tokens, which already
		// defeats DNS rebinding; the check would also break reverse proxies
		// running on the same host.
		DisableLocalhostProtection: !auth.Disabled(),
	})

	mux := http.NewServeMux()
	auth.Routes(mux)
	mcpEndpoint := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		oauth.SetCORS(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		auth.Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Del(BaseURLHeader)
			r.Header.Set(BaseURLHeader, auth.BaseURL(r))
			mcpHandler.ServeHTTP(w, r)
		})).ServeHTTP(w, r)
	})
	mux.Handle("/mcp", mcpEndpoint)
	mux.Handle("/mcp/", mcpEndpoint)
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /r/{token}", h.report)
	mux.HandleFunc("GET /r/{token}/download", h.download)
	mux.HandleFunc("GET /r/{token}/pdf", h.pdf)
	mux.HandleFunc("GET /{$}", h.home)
	if svc.cfg.Admin != nil {
		mux.Handle("/admin", svc.cfg.Admin)
		mux.Handle("/admin/", svc.cfg.Admin)
	}
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write([]byte(faviconSVG))
	})
	return h.logRequests(mux)
}

type httpApp struct {
	svc  *Service
	auth *oauth.Server
	log  *slog.Logger
}

const faviconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 28 28"><rect width="28" height="28" rx="7" fill="#2563eb"/><path d="M6 19l5-6 4 3 6-8" stroke="#fff" stroke-width="2.4" fill="none" stroke-linecap="round" stroke-linejoin="round"/></svg>`

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

func (h *httpApp) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)
		if r.URL.Path == "/healthz" {
			return
		}
		path := r.URL.Path
		if strings.HasPrefix(path, "/r/") && len(path) > 11 {
			path = path[:11] + "…" // do not log share tokens
		}
		h.log.Info("http", "method", r.Method, "path", path, "status", rec.status,
			"ms", time.Since(start).Milliseconds(), "ip", oauth.ClientIP(r))
	})
}

// processStart identifies the running process in /healthz ("started"), so
// that the /admin page can tell when the container has been restarted.
var processStart = time.Now().UTC().Format(time.RFC3339)

func (h *httpApp) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "version": h.svc.cfg.Version, "started": processStart})
}

// ---------------------------------------------------------------------------
// Report hosting

// record looks up the report of the request and its language and version
// (?lang= and ?view=, the report's defaults otherwise).
func (h *httpApp) record(w http.ResponseWriter, r *http.Request) (*store.Record, string, string, bool) {
	tok := strings.TrimSuffix(r.PathValue("token"), ".html")
	rec, err := h.svc.store.ByShareToken(tok)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			h.log.Error("report lookup", "err", err)
		}
		http.Error(w, "Rapport introuvable / Report not found", http.StatusNotFound)
		return nil, "", "", false
	}
	rec = h.svc.EnsureFavicon(r.Context(), rec)
	lang := rec.Report.Lang()
	if l := r.URL.Query().Get("lang"); l != "" {
		lang = normLang(l)
	}
	view := normView(r.URL.Query().Get("view"), defaultView(rec))
	return rec, lang, view, true
}

func reportHeaders(w http.ResponseWriter) {
	hd := w.Header()
	hd.Set("X-Robots-Tag", "noindex, nofollow")
	hd.Set("Referrer-Policy", "no-referrer")
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Cache-Control", "private, no-cache")
	// Self-contained page: no network access needed except https images
	// (logo). Framing is allowed so MCP Apps hosts can display it inline.
	hd.Set("Content-Security-Policy", "default-src 'none'; img-src https: data:; style-src 'unsafe-inline'; script-src 'unsafe-inline'; font-src data:; base-uri 'none'; form-action 'none'")
}

func (h *httpApp) report(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.PathValue("token"), ".pdf") {
		h.pdf(w, r)
		return
	}
	rec, lang, view, ok := h.record(w, r)
	if !ok {
		return
	}
	links := LinksFor(h.auth.BaseURL(r), rec, lang, view)
	html, err := h.svc.RenderHTML(rec, lang, view, links)
	if err != nil {
		h.log.Error("render", "id", rec.ID, "err", err)
		http.Error(w, "Rendering failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	reportHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(html)
}

func (h *httpApp) download(w http.ResponseWriter, r *http.Request) {
	rec, lang, view, ok := h.record(w, r)
	if !ok {
		return
	}
	// The downloaded file holds both versions (opening on the requested one)
	// and keeps absolute links to the PDFs.
	links := LinksFor(h.auth.BaseURL(r), rec, lang, view)
	links.HTMLDownload = ""
	html, err := h.svc.RenderHTML(rec, lang, view, links)
	if err != nil {
		http.Error(w, "Rendering failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	reportHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, DownloadName(rec, lang, "", "html")))
	_, _ = w.Write(html)
}

func (h *httpApp) pdf(w http.ResponseWriter, r *http.Request) {
	r.SetPathValue("token", strings.TrimSuffix(r.PathValue("token"), ".pdf"))
	rec, lang, view, ok := h.record(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), pdf.DefaultTimeout)
	defer cancel()
	b, _, err := h.svc.RenderPDF(ctx, rec, lang, view)
	if err != nil {
		h.log.Error("pdf", "id", rec.ID, "err", err)
		status := http.StatusInternalServerError
		if errors.Is(err, pdf.ErrNoChrome) {
			status = http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(status)
		fmt.Fprintf(w, "Le PDF n'a pas pu être généré / The PDF could not be generated.\n\n%v\n\nL'export HTML reste disponible / The HTML export is still available: %s\n",
			err, LinksFor(h.auth.BaseURL(r), rec, lang, view).HTMLDownload)
		return
	}
	disp := "inline"
	if r.URL.Query().Get("download") != "" {
		disp = "attachment"
	}
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, disp, DownloadName(rec, lang, view, "pdf")))
	_, _ = w.Write(b)
}

// DownloadName returns a friendly file name, e.g.
// rapport-seo-geo-acme-2026-01-01_2026-03-31.html; a PDF of the essential
// version ends with -essentiel / -essential (view "" = no suffix).
func DownloadName(rec *store.Record, lang, view, ext string) string {
	prefix := "rapport-seo-geo"
	if lang == "en" {
		prefix = "seo-geo-report"
	}
	name := prefix
	if s := store.Slugify(rec.Report.Meta.SiteName, 40); s != "" {
		name += "-" + s
	}
	p := rec.Report.Meta.Period
	if safeDate(p.Start) && safeDate(p.End) {
		name += "-" + p.Start + "_" + p.End
	}
	if model.NormView(view) == model.ViewEssential {
		if lang == "en" {
			name += "-essential"
		} else {
			name += "-essentiel"
		}
	}
	return name + "." + ext
}

func safeDate(s string) bool {
	if len(s) != 10 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Home page

var homeTmpl = template.Must(template.New("home").Parse(`<!doctype html>
<html lang="fr"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex"><title>SEO &amp; GEO Report — MCP</title>
<link rel="icon" href="/favicon.ico" type="image/svg+xml">
<style>
:root{color-scheme:only light;--bg:#f7f8fa;--card:#fff;--fg:#0e1117;--muted:#636a75;--line:#e3e5e9;--accent:#2563eb;--warn:#b45309;--code:#eff1f4}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--fg);font:15px/1.6 "Inter",-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,"Helvetica Neue",Arial,sans-serif}
main{max-width:860px;margin:0 auto;padding:32px 16px 64px}
.top{display:flex;align-items:center;gap:12px}.top h1{margin:0;font-size:24px;font-weight:700;letter-spacing:-.02em}
.lead{color:var(--muted);margin:6px 0 24px}
.card{background:var(--card);border:1px solid var(--line);border-radius:16px;padding:22px 24px;margin:16px 0;box-shadow:0 1px 2px rgba(16,24,40,.045)}
h2{font-size:16px;font-weight:650;letter-spacing:-.01em;margin:0 0 10px}
.url{display:flex;gap:8px;align-items:center;flex-wrap:wrap}
code,pre{background:var(--code);border-radius:8px;font:13px/1.5 ui-monospace,SFMono-Regular,Menlo,monospace}
code{padding:2px 6px}pre{padding:12px;overflow:auto;white-space:pre-wrap;word-break:break-all;margin:8px 0}
.big{font-size:16px;padding:8px 12px;word-break:break-all}
button{border:1px solid var(--line);background:var(--card);color:var(--fg);border-radius:999px;padding:6px 14px;cursor:pointer;font:inherit;font-weight:600}button:hover{border-color:var(--muted)}
ol{padding-left:20px;margin:8px 0}li{margin:4px 0}
.warn{color:var(--warn);font-weight:600}.muted{color:var(--muted);font-size:13px}
.pill{display:inline-block;border-radius:999px;padding:2px 10px;font-size:12px;font-weight:600;background:var(--card);border:1px solid var(--line);margin-right:6px;color:var(--fg)}
</style></head><body><main>
<div class="top"><svg width="36" height="36" viewBox="0 0 28 28" aria-hidden="true"><rect width="28" height="28" rx="7" fill="#2563eb"/><path d="M6 19l5-6 4 3 6-8" stroke="#fff" stroke-width="2.4" fill="none" stroke-linecap="round" stroke-linejoin="round"/></svg>
<h1>SEO &amp; GEO Report</h1></div>
<p class="lead">Serveur MCP qui transforme vos exports Google Search Console, GA4 et vos relevés de visibilité dans les IA en un rapport visuel (HTML + PDF), avec Claude.<br>
<span class="muted">MCP server turning your Search Console, GA4 and AI-visibility data into a visual report (HTML + PDF), with Claude. The tool never connects to Google.</span></p>
{{with .Update}}<div class="card" style="border-color:var(--accent)"><b>Nouvelle version {{.Tag}} disponible / New version {{.Tag}} available</b> — <a href="{{.URL}}">notes</a><br>
<span class="muted">Avec le docker-compose.yml fourni, Watchtower l'installe automatiquement dans l'heure. Sinon / Otherwise: <code>docker compose pull &amp;&amp; docker compose up -d</code></span>{{if $.Admin}}<br><a href="/admin">Mettre à jour maintenant / Update now →</a>{{end}}</div>{{end}}
<p><span class="pill">v{{.Version}}</span><span class="pill">{{if .AuthDisabled}}auth: off{{else}}auth: OAuth + password{{end}}</span><span class="pill">PDF: {{if .PDF}}ready{{else}}unavailable (no Chromium){{end}}</span>{{if .Admin}}<a class="pill" href="/admin">Mises à jour / Updates</a>{{end}}</p>

<div class="card"><h2>URL du serveur MCP / MCP server URL</h2>
<div class="url"><code class="big" id="u">{{.MCPURL}}</code><button onclick="navigator.clipboard.writeText(document.getElementById('u').textContent)">Copier / Copy</button></div>
{{if .Local}}<p class="warn">Cette adresse est locale : claude.ai a besoin d'une URL publique en HTTPS (tunnel Cloudflare, Caddy, Traefik…). Claude Code et Claude Desktop fonctionnent en local.<br>
<span class="muted">This is a local address: claude.ai needs a public HTTPS URL (Cloudflare tunnel, Caddy, Traefik…). Claude Code works locally.</span></p>{{end}}
<p class="muted">Détectée automatiquement depuis les en-têtes de la requête (X-Forwarded-Host / Forwarded / Host). Detected automatically from the request headers.</p></div>

<div class="card"><h2>claude.ai (et Claude Desktop / mobile)</h2>
<ol><li>Settings → Connectors → <b>Add custom connector</b> (Paramètres → Connecteurs → Ajouter un connecteur personnalisé).</li>
<li>Nom / Name : <code>SEO GEO Report</code> — URL : <code>{{.MCPURL}}</code> — laissez les champs OAuth vides / leave OAuth fields empty.</li>
<li>Cliquez sur <b>Connect</b> : une page s'ouvre et demande le mot de passe d'accès / a page asks for the access password.</li>
<li>Dans une conversation, activez le connecteur et demandez : « Fais-moi un rapport SEO &amp; GEO de mon site pour le trimestre ».</li></ol>
{{if not .AuthDisabled}}<p class="muted">Le mot de passe est affiché dans les logs au démarrage : <code>docker compose logs seogeo | grep -A3 -i password</code> (ou fichier <code>access-password.txt</code> du volume de données, ou variable ACCESS_PASSWORD).<br>
The password is printed in the logs at startup, stored in access-password.txt in the data volume, or set with ACCESS_PASSWORD.</p>{{end}}</div>

<div class="card"><h2>Claude Code</h2>
<pre>claude mcp add --transport http seogeo {{.MCPURL}}{{if not .AuthDisabled}} --header "Authorization: Bearer VOTRE_MOT_DE_PASSE"{{end}}</pre>
{{if not .AuthDisabled}}<p class="muted">Sans <code>--header</code>, lancez <code>/mcp</code> dans Claude Code pour vous authentifier via OAuth. Without --header, run /mcp in Claude Code to sign in with OAuth.</p>{{end}}</div>

<div class="card"><h2>Claude Desktop (fichier de configuration / config file)</h2>
<p class="muted">Recommandé : ajoutez-le comme connecteur (voir claude.ai ci-dessus). Alternative via <code>mcp-remote</code> dans <code>claude_desktop_config.json</code> :</p>
<pre>{
  "mcpServers": {
    "seogeo": {
      "command": "npx",
      "args": ["-y", "mcp-remote", "{{.MCPURL}}"{{if not .AuthDisabled}}, "--header", "Authorization: Bearer VOTRE_MOT_DE_PASSE"{{end}}]
    }
  }
}</pre></div>

<div class="card"><h2>Comment ça marche / How it works</h2>
<ol><li>Exportez vos données : Search Console (Performances → Exporter), GA4 (Acquisition → Télécharger le fichier CSV), et notez quelques tests de citations dans ChatGPT / Perplexity / Gemini.</li>
<li>Collez ou joignez-les dans la conversation : Claude remplit le rapport et rédige l'analyse.</li>
<li>Claude vous donne le lien du rapport en ligne, le fichier HTML autonome et le PDF.</li></ol></div>
<p class="muted">Endpoints : <code>/mcp</code> · <code>/.well-known/oauth-protected-resource</code> · <code>/.well-known/oauth-authorization-server</code> · <code>/healthz</code></p>
</main></body></html>`))

func (h *httpApp) home(w http.ResponseWriter, r *http.Request) {
	base := h.auth.BaseURL(r)
	host := strings.TrimPrefix(strings.TrimPrefix(base, "https://"), "http://")
	hostname := host
	if i := strings.LastIndex(hostname, ":"); i > 0 && !strings.HasSuffix(hostname, "]") {
		hostname = hostname[:i]
	}
	local := strings.HasPrefix(base, "http://") || hostname == "localhost" || strings.HasPrefix(hostname, "127.") ||
		strings.HasPrefix(hostname, "192.168.") || strings.HasPrefix(hostname, "10.") || hostname == "[::1]"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Frame-Options", "DENY")
	_ = homeTmpl.Execute(w, map[string]any{
		"MCPURL":       base + "/mcp",
		"Version":      h.svc.cfg.Version,
		"AuthDisabled": h.auth.Disabled(),
		"PDF":          pdf.Available(),
		"Local":        local,
		"Update":       h.svc.cfg.Updates.Available(),
		"Admin":        h.svc.cfg.Admin != nil,
	})
}
