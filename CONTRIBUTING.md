# Contributing

Thanks for your interest in SEO & GEO Report! Bug reports, ideas and pull requests are welcome.

## Development setup

Requirements: Go 1.26+, optionally Docker and Chromium / Google Chrome (for PDF tests).

```bash
git clone https://github.com/flocom/SEO-GEO-Report.git
cd SEO-GEO-Report
go test ./...
go run ./cmd/seogeo example --lang en > /tmp/data.json
go run ./cmd/seogeo render -i /tmp/data.json -o /tmp/report.html
AUTH_DISABLED=true go run ./cmd/seogeo serve   # http://localhost:8080
```

Build and run the Docker image from source:

```bash
docker compose -f docker-compose.yml -f docker-compose.build.yml up -d --build
```

## Project layout

| Path | Role |
|---|---|
| `cmd/seogeo` | CLI and servers (`serve`, `mcp`) |
| `internal/model` | report data model (JSON + schema tags) |
| `internal/render`, `internal/charts`, `internal/i18n` | HTML report, SVG charts, translations |
| `internal/insights` | automatic analysis |
| `internal/importer` | Search Console / GA4 CSV import |
| `internal/mcpserver` | MCP tools, HTTP routes, report hosting |
| `internal/oauth` | OAuth 2.1 authorization server, URL detection |
| `internal/store` | file-based report storage |
| `internal/pdf`, `internal/favicon`, `internal/update` | PDF export, favicon fetching, release check |

## Guidelines

- Run `gofmt`, `go vet ./...` and `go test -race ./...` before opening a pull request (CI does the same).
- Add tests for new behavior; keep dependencies minimal (standard library first).
- Every user-facing text must exist in French and English.
- The tool must never fetch analytics data itself: data always comes from the user.
- Keep the Docker setup zero-config: new settings must be optional with sensible defaults.
- Describe user-visible changes in `CHANGELOG.md`.

## Releases

Maintainers tag `vX.Y.Z`; the release workflow tests, publishes the multi-arch image to `ghcr.io/flocom/seo-geo-report` (`X.Y.Z`, `X.Y`, `latest`) and attaches binaries to the GitHub release. Watchtower then updates running installations.
