package oauth

import (
	"html/template"
	"net/http"
	"net/url"
)

var pageTmpl = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="fr"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>{{.Resource}} — Connexion / Sign in</title>
<style>
:root{--bg:#f1f5f9;--card:#fff;--fg:#0f172a;--muted:#64748b;--accent:#2563eb;--err:#dc2626;--line:#e2e8f0}
@media (prefers-color-scheme:dark){:root{--bg:#0b1220;--card:#111827;--fg:#e5e7eb;--muted:#94a3b8;--line:#1f2937}}
*{box-sizing:border-box}body{margin:0;min-height:100vh;display:grid;place-items:center;background:var(--bg);color:var(--fg);
font:15px/1.5 system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;padding:16px}
.card{background:var(--card);border:1px solid var(--line);border-radius:16px;max-width:420px;width:100%;padding:28px;box-shadow:0 10px 30px rgba(0,0,0,.08)}
.logo{display:flex;align-items:center;gap:10px;font-weight:700;font-size:18px;margin-bottom:6px}
.logo svg{flex:none}
h1{font-size:20px;margin:14px 0 4px}.en{color:var(--muted);font-size:13px;margin:0 0 14px}
.client{background:var(--bg);border-radius:10px;padding:10px 12px;font-size:13px;margin:12px 0 18px;word-break:break-all}
.client b{font-size:14px}
label{display:block;font-weight:600;margin-bottom:6px}
input[type=password]{width:100%;padding:11px 12px;border-radius:10px;border:1px solid var(--line);background:var(--card);color:var(--fg);font-size:16px}
input[type=password]:focus{outline:2px solid var(--accent);outline-offset:1px}
.row{display:flex;gap:10px;margin-top:16px}
button{flex:1;padding:11px;border-radius:10px;border:0;font-weight:600;font-size:15px;cursor:pointer}
.ok{background:var(--accent);color:#fff}.no{background:transparent;color:var(--muted);border:1px solid var(--line)}
.err{color:var(--err);font-weight:600;margin:10px 0 0}
.foot{color:var(--muted);font-size:12px;margin-top:18px}
</style></head><body><main class="card">
<div class="logo"><svg width="28" height="28" viewBox="0 0 28 28" aria-hidden="true"><rect width="28" height="28" rx="7" fill="#2563eb"/><path d="M6 19l5-6 4 3 6-8" stroke="#fff" stroke-width="2.4" fill="none" stroke-linecap="round" stroke-linejoin="round"/></svg>{{.Resource}}</div>
{{if .Error}}
<h1>{{.Title}}</h1><p class="en">{{.TitleEN}}</p>
{{else}}
<h1>Autoriser l'accès</h1>
<p class="en">Authorize access</p>
<div class="client"><b>{{if .ClientName}}{{.ClientName}}{{else}}Client MCP{{end}}</b><br>
souhaite accéder à vos rapports / wants to access your reports<br>
→ {{.RedirectHost}}</div>
<form method="post" action="/authorize" autocomplete="off">
{{range $k, $v := .Hidden}}<input type="hidden" name="{{$k}}" value="{{$v}}">{{end}}
<label for="pw">Mot de passe d'accès <span style="font-weight:400;color:var(--muted)">/ Access password</span></label>
<input id="pw" name="password" type="password" autofocus required autocomplete="current-password">
{{if .Message}}<p class="err">{{.Message}}</p>{{end}}
<div class="row"><button class="no" type="submit" name="action" value="deny" formnovalidate>Refuser / Deny</button>
<button class="ok" type="submit" name="action" value="allow">Autoriser / Allow</button></div>
</form>
<p class="foot">Le mot de passe est affiché dans les logs du serveur au démarrage (<code>docker compose logs seogeo</code>) ou défini par ACCESS_PASSWORD.<br>
The password is printed in the server logs at startup or set with ACCESS_PASSWORD.</p>
{{end}}
</main></body></html>`))

type pageData struct {
	Resource     string
	Error        bool
	Title        string
	TitleEN      string
	ClientName   string
	RedirectHost string
	Hidden       map[string]string
	Message      string
}

func (s *Server) renderLogin(w http.ResponseWriter, r *http.Request, p authorizeParams, c *Client, msg string, status int) {
	host := p.RedirectURI
	if u, err := url.Parse(p.RedirectURI); err == nil && u.Host != "" {
		host = u.Scheme + "://" + u.Host
	}
	hidden := map[string]string{
		"response_type":         p.ResponseType,
		"client_id":             p.ClientID,
		"redirect_uri":          p.RedirectURI,
		"state":                 p.State,
		"scope":                 p.Scope,
		"code_challenge":        p.CodeChallenge,
		"code_challenge_method": p.CodeChallengeMethod,
		"resource":              p.Resource,
	}
	for k, v := range hidden {
		if v == "" {
			delete(hidden, k)
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = pageTmpl.Execute(w, pageData{
		Resource:     s.cfg.ResourceName,
		ClientName:   c.Name,
		RedirectHost: host,
		Hidden:       hidden,
		Message:      msg,
	})
}

func (s *Server) renderError(w http.ResponseWriter, r *http.Request, status int, fr, en string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = pageTmpl.Execute(w, pageData{Resource: s.cfg.ResourceName, Error: true, Title: fr, TitleEN: en})
}
