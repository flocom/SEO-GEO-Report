// Command seogeo turns Google Search Console, GA4 and GEO data supplied by the
// user into a bilingual, very visual HTML / PDF report. It is also an MCP
// server (stdio and Streamable HTTP with OAuth) usable from claude.ai,
// Claude Desktop and Claude Code.
package main

import (
	"fmt"
	"os"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

const usage = `seogeo — SEO & GEO visibility reports (Search Console + GA4 + AI assistants)

The tool never connects to Google: you (or Claude) supply the data, it formats it.

Usage:
  seogeo <command> [options]

Report commands:
  render     -i data.json -o report.html|report.pdf [--lang en] [--pdf] [--no-favicon]
             Render a report. Input "-" = stdin. The format follows the output
             extension (.pdf needs Chromium / Google Chrome); default output: stdout (HTML).
  validate   -i data.json          Check a report file (errors, warnings, completeness).
  example    [--lang fr|en]        Print a complete demo report (JSON) to start from.
  schema                           Print the JSON schema of the report format.
  import-gsc <export.zip|dir> [-o gsc.json] [--into data.json]
             Convert a Search Console export (zip or folder of CSV files).
  import-ga4 <file.csv>... [-o ga4.json] [--into data.json]
             Convert GA4 "Download file > CSV" exports.

Server commands:
  serve      [--port 8080] [--data ./data]   HTTP server: MCP endpoint /mcp (OAuth),
             hosted reports, home page with connection instructions.
  mcp        MCP server over stdio (Claude Desktop / Claude Code local config).
  healthcheck                      Exit 0 if the local HTTP server answers (Docker HEALTHCHECK).
  version                          Print the version (and whether a newer release exists).

Environment (all optional):
  DATA_DIR          data directory (default ./data, /data in Docker)
  PORT              HTTP port (default 8080)
  ACCESS_PASSWORD   access password (default: generated, saved in DATA_DIR, printed in the logs)
  PUBLIC_URL        force the public base URL (default: detected from each request)
  AUTH_DISABLED     true = no authentication (local use only)
  MCP_APPS          false = disable the inline report viewer for MCP Apps hosts
  CHROME_PATH       Chromium / Chrome executable for PDF export (auto-detected)
  UPDATE_CHECK      false = do not check GitHub for new releases (every 6 h)
  FAVICON_ALLOW_PRIVATE  true = allow fetching favicons from private / local addresses
  OAUTH_REDIRECT_ALLOWLIST  extra allowed OAuth redirect URI prefixes (comma separated)

Run "seogeo <command> -h" for the options of a command.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "render":
		err = cmdRender(args)
	case "validate":
		err = cmdValidate(args)
	case "example":
		err = cmdExample(args)
	case "schema":
		err = cmdSchema(args)
	case "import-gsc":
		err = cmdImportGSC(args)
	case "import-ga4":
		err = cmdImportGA4(args)
	case "serve":
		err = cmdServe(args)
	case "mcp":
		err = cmdMCP(args)
	case "healthcheck":
		err = cmdHealthcheck(args)
	case "version", "--version", "-v":
		cmdVersion()
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		if err == errSilent {
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
