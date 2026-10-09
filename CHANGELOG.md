# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.2] - 2026-10-09

### Fixed
- Docker image builds no longer depend on Docker Hub (base images pulled from
  mirror.gcr.io): the v0.1.1 image could not be published because of Docker
  Hub rate limits. v0.1.2 ships the v0.1.1 changes as a Docker image.

## [0.1.1] - 2026-10-09

### Changed
- Editorial redesign: sober typography (serif headings), neutral palette with
  a navy default accent, flat cards, discreet chart captions, insights as a
  clean list.
- Key indicators on a single row on desktop; every card grid is balanced
  automatically (no holes, never a lone card on its row), on screen and in print.

### Added
- Hover tooltips on every chart (lines, bars, columns, donut, stacked bars,
  gauges, sparklines) in pure CSS: they work in the downloaded HTML file too.

## [0.1.0] - 2026-10-09

### Added
- Bilingual (French / English) SEO & GEO visibility report: beginner-friendly visual overview and detailed technical analysis (Search Console, GA4, AI assistants).
- Automatic insights, progress index and AI-assistant traffic detection.
- HTML (hosted and standalone) and PDF export (headless Chromium).
- MCP server (Streamable HTTP and stdio) with 20 tools, a guided prompt, resources and an MCP Apps inline viewer.
- OAuth 2.1 authorization server for claude.ai custom connectors (dynamic client registration, PKCE, refresh token rotation), access password also usable as a bearer token.
- Public URL auto-detection from proxy headers.
- Automatic favicon embedding with SSRF protection.
- Search Console and GA4 CSV importers.
- CLI: `render`, `validate`, `example`, `schema`, `import-gsc`, `import-ga4`, `serve`, `mcp`, `version`.
- Zero-config Docker image (`ghcr.io/flocom/seo-geo-report`, amd64 / arm64), Watchtower auto-updates and in-app new-version notice.

[Unreleased]: https://github.com/flocom/SEO-GEO-Report/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/flocom/SEO-GEO-Report/releases/tag/v0.1.0
