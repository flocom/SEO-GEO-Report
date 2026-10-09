# SEO & GEO Report

🇫🇷 [Version française](README.fr.md)

[![CI](https://github.com/flocom/SEO-GEO-Report/actions/workflows/ci.yml/badge.svg)](https://github.com/flocom/SEO-GEO-Report/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/flocom/SEO-GEO-Report?sort=semver)](https://github.com/flocom/SEO-GEO-Report/releases)
[![Docker image](https://img.shields.io/badge/ghcr.io-flocom%2Fseo--geo--report-2496ED?logo=docker&logoColor=white)](https://github.com/flocom/SEO-GEO-Report/pkgs/container/seo-geo-report)
[![License: MIT](https://img.shields.io/github/license/flocom/SEO-GEO-Report)](LICENSE)
[![Go version](https://img.shields.io/github/go-mod/go-version/flocom/SEO-GEO-Report)](go.mod)

**Beautiful, bilingual (French / English) visibility reports for websites — SEO (Google Search Console), traffic (Google Analytics 4) and GEO (visibility in AI assistants: ChatGPT, Perplexity, Gemini, Google AI Overviews…) — built with Claude.**

You export your data (or paste a table, or attach a screenshot), Claude reads it, fills the report and writes the analysis. The server turns it into a polished report: a hosted web page with a shareable link, a standalone HTML file and an A4 PDF.

> The tool **never connects to Google** or any analytics account. It only formats the data you give it.

## Features

- 📊 **Two reports in one**
  - **Part 1 – at a glance**: a visual, beginner-friendly summary for clients and managers: progress gauge, KPI cards with trends, plain-language takeaways, "how to read this chart" hints, glossary.
  - **Part 2 – detailed analysis**: the technical part for SEO specialists: Google performance, queries, pages, countries and devices, traffic channels, landing pages, AI visibility, Core Web Vitals, indexing, actions and recommendations.
- 🤖 **GEO, built in**: traffic from AI assistants is detected automatically in your GA4 sources (chatgpt.com, perplexity.ai, gemini.google.com…), and the report also covers AI citation checks, share of voice, AI Overviews and AI crawler hits.
- 🌍 **French and English**: one data file, rendered in either language (`?lang=en`).
- 📄 **HTML and PDF export**: hosted page, self-contained HTML file (inline CSS and SVG charts, works offline) and print-ready PDF (Chromium is included in the Docker image).
- 🧠 **Automatic insights**: trends, wins, alerts and a progress index computed from the numbers, on top of the narrative Claude writes.
- 🔌 **MCP server for claude.ai**: paste the URL into *Settings → Connectors* and it connects on its own (OAuth 2.1 with dynamic client registration and PKCE). Also works with Claude Desktop and Claude Code, over HTTP or stdio. On hosts that support MCP Apps, the report is shown inline.
- 🐳 **Docker, zero config**: `docker compose up -d`, no required variable, password generated and printed in the logs. Multi-arch images (amd64 / arm64).
- 🔄 **Automatic updates**: Watchtower pulls new releases from GHCR, and the home page tells you when a new version is out.
- 🧭 **URL auto-detection**: no base URL to configure. The public URL comes from `X-Forwarded-*` / `Forwarded` / `Host` headers, so it works behind Cloudflare Tunnel, Caddy, Traefik or nginx.
- 🖼️ **Favicon**: the site's icon is fetched once and embedded in the report, with SSRF protection.
- 🧰 **CLI**: render, validate and import Search Console / GA4 CSV exports from the command line.

## Screenshots

| Part 1 – at a glance | Part 2 – detailed analysis |
|---|---|
| ![Overview](docs/screenshot-overview.png) | ![Detailed analysis](docs/screenshot-details.png) |
| **AI visibility (GEO)** | **Cover & contents** |
| ![GEO](docs/screenshot-geo.png) | ![Cover](docs/screenshot-cover.png) |

## Quick start

```bash
mkdir seogeo && cd seogeo
curl -fsSLO https://raw.githubusercontent.com/flocom/SEO-GEO-Report/main/docker-compose.yml
docker compose up -d
docker compose logs seogeo   # read the generated access password
```

Or clone the repository: `git clone https://github.com/flocom/SEO-GEO-Report.git && cd SEO-GEO-Report && docker compose up -d`.

On first start, a strong password is generated, saved in the data volume (`/data/access-password.txt`) and printed in the logs at every start:

```
  ACCESS PASSWORD: K79fGXWc1r-KrbXgzbFddyZN
```

Open <http://localhost:8080>. The home page shows the detected MCP URL and copy-paste instructions for each client.

Every variable is optional (see [`.env.example`](.env.example)): `ACCESS_PASSWORD`, `PUBLIC_URL`, `PORT`, `AUTH_DISABLED`, `MCP_APPS`, `UPDATE_CHECK`, `CHROME_PATH`, `FAVICON_ALLOW_PRIVATE`, `WATCHTOWER_POLL_INTERVAL`, `OAUTH_REDIRECT_ALLOWLIST`.

To build from source instead of pulling the image:

```bash
docker compose -f docker-compose.yml -f docker-compose.build.yml up -d --build
```

## Expose it publicly (required for claude.ai)

claude.ai must reach the server over **HTTPS**. The server detects its public URL from the proxy headers, so there is nothing to configure.

**Cloudflare Tunnel** (no open port). Uncomment the `cloudflared` service in `docker-compose.yml`:

- *Named tunnel* (stable URL, free account): Zero Trust → Networks → Tunnels → create a tunnel, put the token in `.env` as `TUNNEL_TOKEN=...`, and add a public hostname (e.g. `seo.example.com`) pointing to `http://seogeo:8080`.
- *Quick tunnel* (no account, for testing): a random `https://xxx.trycloudflare.com` URL appears in `docker compose logs cloudflared`. It changes at every restart.

**Caddy** (automatic certificates):

```
seo.example.com {
    reverse_proxy localhost:8080
}
```

**Traefik** (labels on the `seogeo` service):

```yaml
labels:
  - traefik.enable=true
  - traefik.http.routers.seogeo.rule=Host(`seo.example.com`)
  - traefik.http.routers.seogeo.entrypoints=websecure
  - traefik.http.routers.seogeo.tls.certresolver=letsencrypt
  - traefik.http.services.seogeo.loadbalancer.server.port=8080
```

**nginx**: `proxy_pass http://127.0.0.1:8080;` with `proxy_set_header Host $host;` and `proxy_set_header X-Forwarded-Proto $scheme;`.

## Connect Claude

### claude.ai (and Claude Desktop / mobile)

1. **Settings → Connectors → Add custom connector**.
2. Name: `SEO GEO Report`. URL: `https://seo.example.com/mcp`. Leave the advanced OAuth fields empty.
3. Click **Connect**. The server's sign-in page opens: enter the access password and click **Allow**.
4. In a chat, enable the connector and ask, for example: *"Build the SEO & GEO report of example.com for last quarter."* The `create_seo_geo_report` prompt is also available.

Clients and tokens survive container restarts: they are stored hashed in `/data/oauth.json`. Access tokens are refreshed automatically. Once added, the connector also shows up in Claude Desktop and on mobile.

### Claude Code

```bash
claude mcp add --transport http seogeo https://seo.example.com/mcp \
  --header "Authorization: Bearer YOUR_ACCESS_PASSWORD"
```

The access password also works as a bearer token. Without `--header`, run `/mcp` in Claude Code to sign in with OAuth. Locally, use `http://localhost:8080/mcp`.

### Claude Desktop (manual config)

Prefer adding it as a connector (see above). Alternatively, in `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "seogeo": {
      "command": "npx",
      "args": ["-y", "mcp-remote", "https://seo.example.com/mcp",
               "--header", "Authorization: Bearer YOUR_ACCESS_PASSWORD"]
    }
  }
}
```

or fully local over stdio:

```json
{
  "mcpServers": {
    "seogeo": {
      "command": "docker",
      "args": ["run", "-i", "--rm", "-v", "seogeo_seogeo-data:/data", "ghcr.io/flocom/seo-geo-report:latest", "mcp"]
    }
  }
}
```

## How it works with Claude

> **You:** Build the SEO & GEO report of maison-lumen.fr for July–September, compared with the previous quarter, in English.
>
> **Claude:** *(reads the guide)* I need: 1) your Search Console export (Performance → Search results → Compare dates → Export); 2) your GA4 export (Traffic acquisition by channel and by source / medium, landing pages); 3) if you have them, a few checks in ChatGPT / Perplexity / Gemini: is your site cited?
>
> **You:** *(attaches the Search Console zip, two GA4 CSV files and pastes a table of AI checks)*
>
> **Claude:** *(create_report → import_search_console_csv → import_analytics_csv → set_geo → set_narrative → validate_report → render_report)* Here is your report: online view, HTML and PDF. Key takeaways: Google clicks +36 %, AI-assistant traffic ×2.6 (led by ChatGPT), click-through rate slipping on your most viewed pages…

**Tools:** `get_report_guide`, `create_report`, `update_meta`, `set_search_console`, `set_analytics`, `set_geo`, `set_narrative`, `add_text_section`, `update_text_section`, `remove_text_section`, `set_options`, `import_search_console_csv`, `import_analytics_csv`, `validate_report`, `render_report`, `generate_report`, `get_report`, `list_reports`, `duplicate_report`, `delete_report`.

**Prompt:** `create_seo_geo_report`.

**Resources:** `seogeo://guide`, `seogeo://schema`, `seogeo://example/fr`, `seogeo://example/en`. With MCP Apps there is also `ui://seogeo/report-viewer.html` (disable with `MCP_APPS=false`).

## Exports

| URL | Content |
|---|---|
| `/r/<token>` | online report (`?lang=fr` or `?lang=en`) |
| `/r/<token>/download` | standalone HTML file, no external dependency |
| `/r/<token>.pdf` (or `/r/<token>/pdf`) | A4 PDF (`?download=1` to force download) |

PDFs are rendered by headless Chromium, which ships in the image with Noto, DejaVu and emoji fonts. They are cached and regenerated when the report changes. Outside Docker, Chromium or Google Chrome is auto-detected (`CHROME_PATH` to force one). Without it, the HTML export still works and the PDF error is explained.

## Command line

```bash
seogeo example --lang en > data.json        # complete demo report
seogeo schema > schema.json                 # JSON schema of the format
seogeo validate -i data.json                # errors, warnings, completeness
seogeo render -i data.json -o report.html   # standalone HTML
seogeo render -i data.json -o report.pdf    # PDF (needs Chromium / Chrome)
seogeo render -i data.json -o report.html --lang fr --no-favicon
seogeo import-gsc gsc-export.zip --into data.json -o data.json
seogeo import-ga4 channels.csv sources.csv --into data.json -o data.json
seogeo serve                                # HTTP server (MCP /mcp, OAuth, hosted reports)
seogeo mcp                                  # MCP over stdio
seogeo version                              # version + update check
```

Binaries for Linux, macOS and Windows (amd64 / arm64) are attached to every [release](https://github.com/flocom/SEO-GEO-Report/releases). To build from source: `go build -o seogeo ./cmd/seogeo` (Go 1.26).

## Data format

A report is a JSON document. Run `seogeo schema` for the full schema; [`examples/demo-en.json`](examples/demo-en.json) and [`examples/demo-fr.json`](examples/demo-fr.json) are complete examples.

| Key | Content |
|---|---|
| `meta` | site, period, comparison period, language, author, client, logo, brand color, `favicon_url` (empty = fetched from `site_url`, `none` = disabled) |
| `search_console` | totals, daily series, queries, pages, countries, devices, search appearance, brand split, indexing, Core Web Vitals |
| `analytics` | totals (all channels and organic), daily series, channels, source / medium, landing pages |
| `geo` | AI-assistant traffic, citation checks, share of voice, AI Overviews, AI crawlers |
| `narrative` | executive summary, highlights, concerns, actions, recommendations, per-section notes (Markdown) |
| `sections`, `options` | free Markdown sections; display options |

Conventions:

- **Percentages are in percent**: 3.2 % is written `3.2`.
- Dates are `YYYY-MM-DD` and durations are in seconds.
- Metrics are `{"current": …, "previous": …}`. The previous value drives the progress visuals.

## Updating

The bundled `docker-compose.yml` runs [Watchtower](https://github.com/nicholas-fedor/watchtower), the maintained fork of the archived containrrr/watchtower.

- Every hour (`WATCHTOWER_POLL_INTERVAL`, in seconds), it checks for a new `ghcr.io/flocom/seo-geo-report:latest` image.
- It restarts **only** the seogeo container, the one labelled `com.centurylinklabs.watchtower.enable=true`. Data stays in the volume.
- The server also checks GitHub releases every 6 hours and shows "New version vX available" on the home page and in the logs (`UPDATE_CHECK=false` to disable).

To update manually:

```bash
docker compose pull && docker compose up -d
```

Image tags:

| Tag | Content |
|---|---|
| `latest` | latest stable release |
| `X.Y.Z` / `X.Y` | a pinned version |
| `edge` | latest commit on `main` |

## Security

- **`/mcp` requires a bearer token**: either an OAuth access token issued by the server, or the access password.
  - OAuth 2.1 with mandatory PKCE S256 and single-use 5-minute codes. Refresh tokens rotate.
  - Tokens are opaque and stored as SHA-256 hashes.
  - Dynamic registration is limited to claude.ai / claude.com / localhost callbacks.
- **Wrong passwords are rate-limited**: 8 attempts per 15 minutes per IP. Use a long password if you set `ACCESS_PASSWORD`; the generated one is 24 random characters.
- **Report links are public but unguessable**: `/r/<token>` needs no authentication, and the token is 192 random bits. Anyone with the link can read the report, like a "anyone with the link" share. Deleting the report disables the link. Pages are `noindex`.
- **Favicon fetching is SSRF-protected**: http/https only, private, loopback, link-local and cloud-metadata addresses are refused after DNS resolution, sizes are capped, and SVGs with scripts or external references are rejected.
- **`AUTH_DISABLED=true`** opens the server to anyone who can reach it. Use it locally only.
- **Watchtower has access to the Docker socket.** Remove that service if you prefer manual updates.
- The container runs as a non-root user.

See [SECURITY.md](SECURITY.md) to report a vulnerability.

## FAQ

**Does it connect to my Google account?**
No. Claude, or you, supply the numbers from exports, copy-paste or screenshots. The server only formats them.

**Do I need a server?**
To use it from claude.ai, yes: any machine running Docker with an HTTPS URL. A Cloudflare quick tunnel is enough to try it. Claude Code and the CLI also work locally.

**Which exports should I give Claude?**
- Search Console: Performance → Search results, with "Compare" dates, then Export.
- GA4: Traffic acquisition (channel group and source / medium), Landing pages, with a comparison range.
- Optionally, a few AI citation checks.

Claude asks for what's missing.

**Can I brand the report?**
Yes: `meta.logo_url`, `meta.brand_color`, `meta.prepared_by` and `meta.prepared_for`. The site favicon is added automatically.

**The PDF fails outside Docker.**
Install Chromium or Google Chrome, or set `CHROME_PATH`. The HTML export always works.

**claude.ai says it can't connect.**
Check that `https://your-host/.well-known/oauth-protected-resource` shows your public HTTPS URL. If it shows `http://` or an internal host, make your proxy forward `X-Forwarded-Proto` / `X-Forwarded-Host`, or set `PUBLIC_URL`.

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) and the [changelog](CHANGELOG.md).

## License

[MIT](LICENSE) © 2026 flocom
