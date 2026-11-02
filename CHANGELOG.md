# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.3.0] - 2026-11-02

### Added
- Google Business Profile section ("Fiche Google (Google Business Profile)" /
  "Google Business Profile (local visibility)"), displayed only when Business
  Profile data is provided, between Google Shopping and AI visibility: a
  plain-language explanation of the Google profile, views, interactions,
  calls, direction requests and website clicks with a daily chart, where
  people found the profile (Google Search or Maps, mobile or desktop), what
  they did (calls, website, directions, messages, bookings), the local
  searches that show the profile ("<15" thresholds kept), reviews (big
  average rating with stars, star breakdown, new reviews, reply rate, latest
  reviews kept as customers wrote them), the profile checklist and publishing
  activity, and a comparison table when the business has several locations.
  Part 1 gets a "Google profile actions" card (the key indicators then hold
  up to eight cards, still on one row on desktop).
- `business_profile` in the report format (locations, totals, daily series,
  search keywords, reviews, posts, photos, profile completeness, checklist;
  views and interactions are derived from their parts), MCP tools
  `set_business_profile` and `import_business_profile_csv`, CLI command
  `import-gbp` (Performance / bulk insights export per location, daily
  metrics, search keywords, reviews; French or English headers).
- Business Profile figures feed the KPI tones, the automatic insights (views
  and actions trend, fastest-growing action, Maps vs Search and mobile
  shares, rating and new reviews, unanswered reviews, top local search,
  missing profile information) and the progress index with a modest weight
  (interactions, 0.05); up to three automatic insights are kept for the
  section. Glossary entries for the local terms (shown only with Business
  Profile data); the demo reports include the Lyon showroom profile.
- The guide, the server instructions and the prompt tell the assistant to
  put Google Business Profile, Bing and Merchant Center figures in their
  native sections, never in a free-text section.

- /admin page to check for updates and update in one click: sign-in with
  the access password (same rate limit as the OAuth login, signed
  `HttpOnly` / `SameSite=Strict` session cookie, CSRF tokens, origin checks,
  `no-store`), French or English. It shows the installed version, build and
  uptime, the latest GitHub release (date, release notes, link), "Up to date"
  / "Update available", the last check and whether Watchtower is reachable.
  "Check for updates" queries GitHub right away (at most once a minute);
  "Update now" asks Watchtower to pull the new image and restart the service,
  then the page follows the restart and shows "Updated to vX". When one-click
  updates are not available (older compose file, source build, binary), the
  page explains the manual steps. JSON API: `GET /admin/api/status`,
  `POST /admin/api/update` (Bearer access password). Linked from the home
  page; `/healthz` now also returns the process start time.
- `docker-compose.yml` enables the Watchtower HTTP API (update + status
  endpoints, hourly polls kept), reachable only on the internal compose
  network and protected by a random token generated at first start by a
  one-shot `watchtower-token` service (shared `watchtower-api` volume):
  nothing to configure, and the seogeo container still has no access to the
  Docker socket. Optional `WATCHTOWER_HTTP_API_TOKEN` and `WATCHTOWER_URL`.

- Two report versions on the same page: Essential and Advanced. A segmented
  control at the top of the cover ("Essentiel | Avancé" / "Essential |
  Advanced", one line of description each) switches between the Essential
  version (simplified cover, Part 1, a short glossary limited to the terms it
  uses and a "go further" note listing what the advanced version adds; no
  dense tables) and the Advanced version (the full report, as before). Pure
  CSS (hidden radio buttons and sibling selectors): it works without
  JavaScript, in the downloaded HTML file, with the keyboard (focus ring,
  arrow keys, labelled radios), in dark mode, and printing prints the version
  on screen; the table of contents and the section numbering adapt, and
  links to a part of the advanced version still land on it. A small optional
  script syncs `?view=` / `#essentiel` / `#avance` and remembers the reader's
  choice.
- `options.default_view` (`essential`, the default, or `advanced`; settable
  with `set_options`), `render.Options.View`, `?view=essential|advanced` on
  hosted pages (combined with `?lang=`), `/r/<token>.pdf?view=essential` (a
  short PDF: cover and about four pages) and `?view=advanced` (full report),
  cached separately per language and version. The header offers "PDF
  essentiel" and "PDF complet" next to the HTML download (one interactive
  file with both versions); essential PDFs are named `…-essentiel.pdf` /
  `…-essential.pdf`. CLI: `seogeo render --view essential|advanced`.
- `render_report` / `generate_report` take an optional `view` and return
  `view_essential_url`, `view_advanced_url`, `pdf_essential_url`,
  `pdf_advanced_url` and `default_view` next to `html_url`, `pdf_url` and
  `html_download_url` (unchanged, they open the default version); the text
  summary, tool descriptions, guide, server instructions, prompt and inline
  viewer tell the assistant to give the user both versions.

### Changed
- Beginner-friendly Part 1, visual first: a compact color code shown once
  (▲ good news, ▼ to watch, ● expected / normal, ▶ stable) replaces the long
  "How to read these indicators?" paragraph; the progress verdict is followed
  by a channel-by-channel scorecard (Google, your website, AI assistants,
  Bing, Google Shopping, Google profile, each with a pictogram, the key
  figure, a colored traffic light and a plain status such as "More visits
  from Google"; new channels register with one function call) and by big
  "Key takeaways" tiles (the three strongest wins and up to two points to
  watch). The executive summary becomes a lighter pull quote, and Part 1
  keeps at most four highlights and two concerns: the complete lists open
  Part 2 in a new "Highlights and points of attention" section.
- KPI cards use plain-language titles with the technical term below
  ("Visits from Google" / "Clicks (Search Console)", "Appearances in Google"
  / "Impressions", "Average ranking" / "Average position"...), a pictogram,
  a CTR explained in words ("Out of 100 appearances in Google, about 3 turn
  into a visit") and, without daily data, a before / after mini bar so that
  cards stay balanced.
- Readable changes everywhere (KPI cards, scorecard, tiles, table and chart
  badges, automatic insights), with one rule (`insights.ChangeKind`): a small
  base shows the absolute change ("+5" instead of "+500 %"), a figure
  multiplied by 3 or more a multiplier ("×3.8", "×47"); the percentage stays
  in the hover text. Visible percentages never exceed +200 %.
- Expected declines are no longer red: when impressions jump (> +50 %) with
  more ranking queries and stable or rising clicks, a worse average position
  and click-through rate are the normal dilution effect. Their cards get a
  short amber "Normal" pill (the explanation is in the hover text and for
  screen readers) and one amber line under the key indicators explains them
  once; card contents use fixed slots so that a row stays compact and
  aligned. The CTR alert insight is
  replaced by an explanation, and the position no longer lowers the progress
  index (`KPIStatus.Expected` / `KPIStatus.Note`, also for Bing).
- GA4 channel names are translated in French ("Organic Search" →
  "Recherche naturelle", "Direct" → "Accès direct", "Referral" → "Sites
  référents", "AI Assistant" → "Assistants IA"...) in charts, tables and
  insights (`i18n.ChannelName`; unknown custom channels are kept as is).
- Tiny volumes (under 5 a day) and, in Part 1, periods over 60 days are
  charted per week (with the week range in tooltips) instead of day by day,
  including KPI sparklines; Part 2 keeps daily charts when volumes allow.
- Part 1 AI section: with one or two assistants, big-number tiles (visits
  and change, share of traffic, citations found in AI answers or in Copilot)
  replace the lonely bar chart.
- Part 1 chart captions are one sentence; tests check that every message is
  translated (no empty or untranslated-looking text, including the messages
  merged from other files) and that no French UI text leaks into English
  reports and vice versa, whether the language comes from `meta.language` or
  from the render override.

## [0.2.0] - 2026-10-20

### Added
- Google Merchant Center section ("Google Shopping (Merchant Center)"),
  displayed only when Merchant Center data is provided, between the landing
  pages and AI visibility: product clicks, impressions, CTR, conversions and
  conversion value with a daily chart, clicks by program (free listings vs
  Shopping ads, Images, YouTube), top products and categories, product feed
  status (approved / limited / disapproved / pending) with the issues to fix
  sorted by severity, and price competitiveness against the benchmark price.
  Part 1 gets a "Google Shopping clicks" card (the key indicators then hold
  seven cards, still on one row on desktop).
- `merchant_center` in the report format, MCP tools `set_merchant_center` and
  `import_merchant_center_csv`, CLI command `import-gmc` (performance by
  product, day, program, category, brand or country; product statuses; feed
  issues; price competitiveness; French or English headers).
- Merchant Center clicks feed the KPI tones, the automatic insights (feed
  health and main issue, prices above the market, free listings growth...)
  and the progress index with a modest weight; Bing and Merchant Center keep
  up to three automatic insights each. Glossary entries for the Shopping
  terms (shown only with Merchant Center data); the demo reports include
  Merchant Center data.
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

[Unreleased]: https://github.com/flocom/SEO-GEO-Report/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/flocom/SEO-GEO-Report/compare/v0.1.3...v0.2.0
[0.1.3]: https://github.com/flocom/SEO-GEO-Report/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/flocom/SEO-GEO-Report/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/flocom/SEO-GEO-Report/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/flocom/SEO-GEO-Report/releases/tag/v0.1.0
