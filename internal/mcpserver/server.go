// Package mcpserver exposes the report builder as an MCP server (tools,
// prompt, resources and an optional MCP Apps inline viewer) and serves the
// HTTP side: Streamable HTTP endpoint, OAuth, report hosting, home page.
package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	viewerURI      = "ui://seogeo/report-viewer.html"
	appMIMEType    = "text/html;profile=mcp-app"
	appsExtension  = "io.modelcontextprotocol/ui"
	guideURI       = "seogeo://guide"
	schemaURI      = "seogeo://schema"
	exampleURIBase = "seogeo://example/"
)

// NewServer builds the MCP server with every tool, prompt and resource.
func (s *Service) NewServer() *mcp.Server {
	caps := &mcp.ServerCapabilities{
		Tools:     &mcp.ToolCapabilities{},
		Prompts:   &mcp.PromptCapabilities{},
		Resources: &mcp.ResourceCapabilities{},
	}
	if s.cfg.MCPApps {
		caps.Extensions = map[string]any{appsExtension: map[string]any{}}
	}
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "seogeo",
		Title:   "SEO & GEO Report",
		Version: s.cfg.Version,
	}, &mcp.ServerOptions{
		Instructions: Instructions,
		Capabilities: caps,
	})
	s.registerTools(srv)
	s.registerPrompts(srv)
	s.registerResources(srv)
	return srv
}

func (s *Service) registerPrompts(srv *mcp.Server) {
	srv.AddPrompt(&mcp.Prompt{
		Name:        "create_seo_geo_report",
		Title:       "Create a SEO & GEO report",
		Description: "Guided workflow: collect Search Console, GA4 and AI-visibility data from the user and produce a visual report (HTML + PDF).",
		Arguments: []*mcp.PromptArgument{
			{Name: "site", Description: "Website name or URL", Required: false},
			{Name: "language", Description: "Report language: fr or en (default fr)", Required: false},
			{Name: "period", Description: "Analysed period, e.g. 'last 3 months' or '2026-01-01 to 2026-03-31'", Required: false},
		},
	}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		args := map[string]string{}
		if req.Params != nil && req.Params.Arguments != nil {
			args = req.Params.Arguments
		}
		site := strings.TrimSpace(args["site"])
		lang := normLang(args["language"])
		period := strings.TrimSpace(args["period"])
		var b strings.Builder
		b.WriteString("I want a SEO & GEO visibility report")
		if site != "" {
			fmt.Fprintf(&b, " for %s", site)
		}
		if period != "" {
			fmt.Fprintf(&b, " covering %s", period)
		}
		if lang == "en" {
			b.WriteString(", in English.")
		} else {
			b.WriteString(", in French.")
		}
		b.WriteString(`

Use the seogeo tools. First call get_report_guide. Then ask me, in one short message, for whatever is missing among: site URL, exact period and comparison period, my Google Search Console export (Performance report with comparison: totals, dates, queries, pages, countries, devices), my GA4 export (traffic acquisition by channel and by source / medium, landing pages, totals, with comparison), and any AI-visibility checks (prompts tested in ChatGPT, Perplexity, Gemini, AI Overviews: cited or not). Explain briefly how to export each one. I can paste tables, attach CSV files or screenshots.
Never invent numbers. When I have sent the data: create_report, fill it (set_* or import_*_csv), write the narrative from the numbers in the report language (executive summary with figures, highlights, concerns, actions, recommendations), validate_report, render_report, then give me the links (online view, HTML and PDF downloads) and comment the key takeaways.`)
		return &mcp.GetPromptResult{
			Description: "SEO & GEO report workflow",
			Messages: []*mcp.PromptMessage{{
				Role:    "user",
				Content: &mcp.TextContent{Text: b.String()},
			}},
		}, nil
	})
}

func (s *Service) registerResources(srv *mcp.Server) {
	srv.AddResource(&mcp.Resource{
		URI: guideURI, Name: "guide", Title: "Report guide",
		Description: "Workflow, field conventions, section ids and what to ask the user.",
		MIMEType:    "text/markdown",
	}, func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
			URI: guideURI, MIMEType: "text/markdown", Text: GuideMarkdown(false, "en"),
		}}}, nil
	})
	srv.AddResource(&mcp.Resource{
		URI: schemaURI, Name: "schema", Title: "Report JSON schema",
		Description: "JSON Schema (2020-12) of the complete report accepted by generate_report.",
		MIMEType:    "application/schema+json",
	}, func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		b, err := SchemaJSON()
		if err != nil {
			return nil, err
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
			URI: schemaURI, MIMEType: "application/schema+json", Text: string(b),
		}}}, nil
	})
	for _, lang := range []string{"fr", "en"} {
		uri := exampleURIBase + lang
		srv.AddResource(&mcp.Resource{
			URI: uri, Name: "example-" + lang, Title: "Example report (" + lang + ")",
			Description: "A complete, realistic demo report (fictional site) to copy the structure from.",
			MIMEType:    "application/json",
		}, func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			b, err := ExampleJSON(lang)
			if err != nil {
				return nil, err
			}
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
				URI: uri, MIMEType: "application/json", Text: string(b),
			}}}, nil
		})
	}
	if s.cfg.MCPApps {
		srv.AddResource(&mcp.Resource{
			URI: viewerURI, Name: "report-viewer", Title: "Report viewer",
			Description: "Inline viewer displaying a rendered report (MCP Apps).",
			MIMEType:    appMIMEType,
		}, func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			ui := map[string]any{"prefersBorder": true}
			base := ""
			if req != nil && req.Extra != nil && req.Extra.Header != nil {
				base = req.Extra.Header.Get(BaseURLHeader)
			}
			if base == "" {
				base = strings.TrimRight(s.cfg.PublicURL, "/")
			}
			if base != "" {
				ui["csp"] = map[string]any{"frameDomains": []string{base}}
			}
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
				URI: viewerURI, MIMEType: appMIMEType, Text: viewerHTML,
				Meta: mcp.Meta{"ui": ui},
			}}}, nil
		})
	}
}

// viewerHTML is the MCP Apps view (protocol 2026-01-26): it performs the
// ui/initialize handshake, waits for ui/notifications/tool-result and shows
// the rendered report in an iframe, with buttons to open / download it.
// Without MCP Apps support the host simply ignores it and the model gives
// the links.
const viewerHTML = `<!doctype html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<style>
:root{--bg:#fff;--fg:#0f172a;--muted:#64748b;--line:#e2e8f0;--accent:#2563eb}
@media (prefers-color-scheme:dark){:root{--bg:#0f172a;--fg:#e2e8f0;--muted:#94a3b8;--line:#1e293b}}
*{box-sizing:border-box}html,body{margin:0;background:var(--bg);color:var(--fg);font:14px/1.45 system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
header{display:flex;flex-wrap:wrap;gap:8px;align-items:center;justify-content:space-between;padding:10px 12px;border-bottom:1px solid var(--line)}
h1{font-size:15px;margin:0}small{color:var(--muted)}
.btns{display:flex;gap:6px;flex-wrap:wrap}
a.b,button{appearance:none;border:1px solid var(--line);background:transparent;color:var(--fg);border-radius:8px;padding:6px 10px;font:inherit;cursor:pointer;text-decoration:none}
.primary{background:var(--accent)!important;border-color:var(--accent)!important;color:#fff!important}
iframe{display:block;width:100%;height:900px;border:0;background:#fff}
#msg{padding:16px;color:var(--muted)}ul{margin:6px 0;padding-left:18px}
</style></head><body>
<header><div><h1 id="title">SEO &amp; GEO report</h1><small id="sub"></small></div><div class="btns" id="btns"></div></header>
<div id="msg">Rendering the report…</div>
<iframe id="frame" title="Report" hidden></iframe>
<script>
(function(){
  var nextId = 1, pending = {};
  function post(m){ window.parent.postMessage(m, "*"); }
  function request(method, params){
    return new Promise(function(resolve, reject){
      var id = nextId++; pending[id] = {resolve: resolve, reject: reject};
      post({jsonrpc: "2.0", id: id, method: method, params: params || {}});
    });
  }
  function notify(method, params){ post({jsonrpc: "2.0", method: method, params: params || {}}); }
  function resize(){ notify("ui/notifications/size-changed", {height: document.documentElement.scrollHeight}); }
  function openLink(url){
    request("ui/open-link", {url: url}).catch(function(){ window.open(url, "_blank", "noopener"); });
  }
  function button(label, url, primary){
    var b = document.createElement("button");
    b.textContent = label; if (primary) b.className = "primary";
    b.onclick = function(){ openLink(url); };
    document.getElementById("btns").appendChild(b);
  }
  function esc(s){ return String(s).replace(/[&<>"]/g, function(c){ return {"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;"}[c]; }); }
  function show(result){
    var sc = (result && result.structuredContent) || {};
    var links = sc.links || {};
    var msg = document.getElementById("msg"), frame = document.getElementById("frame");
    document.getElementById("title").textContent = sc.site_name ? sc.site_name : "SEO & GEO report";
    document.getElementById("sub").textContent = sc.period || "";
    document.getElementById("btns").innerHTML = "";
    if (links.html_url) {
      button("Ouvrir / Open", links.html_url, true);
      if (links.pdf_url) button("PDF", links.pdf_url);
      if (links.html_download_url) button("HTML", links.html_download_url);
      frame.src = links.html_url; frame.hidden = false; msg.hidden = true;
    } else if (result && result._meta && result._meta["github.com/flocom/SEO-GEO-Report/html_base64"]) {
      var bin = atob(result._meta["github.com/flocom/SEO-GEO-Report/html_base64"]), bytes = new Uint8Array(bin.length);
      for (var i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
      frame.srcdoc = new TextDecoder("utf-8").decode(bytes); frame.hidden = false; msg.hidden = true;
    } else if (result && result.isError) {
      msg.textContent = "The report could not be rendered."; msg.hidden = false;
    } else {
      var html = "Report rendered.";
      if (sc.html_file) html += "<br>File: <code>" + esc(sc.html_file) + "</code>";
      var k = (sc.summary && sc.summary.kpis) || [];
      if (k.length) { html += "<ul>"; k.forEach(function(x){ html += "<li>" + esc(x.label) + ": " + esc(x.current) + (x.delta_pct != null ? " (" + (x.delta_pct > 0 ? "+" : "") + esc(x.delta_pct) + " %)" : "") + "</li>"; }); html += "</ul>"; }
      msg.innerHTML = html; msg.hidden = false;
    }
    setTimeout(resize, 50);
  }
  window.addEventListener("message", function(e){
    var m = e.data;
    if (!m || m.jsonrpc !== "2.0") return;
    if (m.id != null && !m.method) {
      var p = pending[m.id]; if (!p) return; delete pending[m.id];
      if (m.error) p.reject(m.error); else p.resolve(m.result);
      return;
    }
    if (m.method === "ui/notifications/tool-result") { show(m.params); return; }
    if (m.method === "ui/resource-teardown" && m.id != null) { post({jsonrpc: "2.0", id: m.id, result: {}}); return; }
    if (m.id != null && m.method) { post({jsonrpc: "2.0", id: m.id, error: {code: -32601, message: "Method not found"}}); }
  });
  request("ui/initialize", {
    protocolVersion: "2026-01-26",
    appInfo: {name: "seogeo-report-viewer", version: "1.0.0"},
    appCapabilities: {}
  }).then(function(){ notify("ui/notifications/initialized", {}); resize(); })
    .catch(function(){ document.getElementById("msg").textContent = "Viewer not supported by this host: use the links in the conversation."; });
})();
</script></body></html>`
