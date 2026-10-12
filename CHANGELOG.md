# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- Bing Webmaster Tools section ("Performance dans Bing" / "Bing Search
  performance"), placed after Google's countries and devices: Bing clicks,
  impressions, CTR and position with daily charts, a Google vs Bing comparison,
  top keywords and pages, and the AI Performance report (citations of the site
  in Microsoft Copilot and Bing AI answers, cited pages, grounding queries with
  intent and topic).
- `bing` in the report format, MCP tools `set_bing` and `import_bing_csv`, CLI
  command `import-bing` (zip, folder or single CSV; Search Performance and AI
  Performance exports, French or English headers).
- Bing clicks and AI citations feed the KPI tones, the automatic insights and
  the progress index; the demo reports include Bing data.

### Fixed
- Rendering no longer fails when a table compares a value with a previous
  value of 0 (e.g. impressions rising from 0).
- A report rendered right after an update is no longer seen as out of date
  because of the file system's coarse timestamps (the HTML cache was thrown
  away, and a store test failed intermittently).
- CSV import reads every date of a file in the same day / month order.

### Changed
- Search Console CSV import: change columns ("Clicks change") are ignored
  instead of being read as comparison values, and "Keyword" / "URL" headers
  are recognized.

## [0.1.3] - 2026-10-09

### Fixed
- Release pipeline: buildx, QEMU and SBOM helper images are pulled from
  mirror.gcr.io too, so Docker Hub outages no longer block the image. This is
  the first published image with the v0.1.1 redesign.

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
