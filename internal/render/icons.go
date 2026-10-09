package render

import "html/template"

// Small stroke icons (24×24 grid, currentColor), inlined in the page.
func icon(paths string) template.HTML {
	return template.HTML(`<svg class="ico" viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">` + paths + `</svg>`)
}

var (
	icoSummary   = icon(`<path d="M12 3l2.6 5.3 5.9.9-4.3 4.1 1 5.8L12 16.4 6.8 19.1l1-5.8L3.5 9.2l5.9-.9z"/>`)
	icoKPI       = icon(`<rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/>`)
	icoSearch    = icon(`<circle cx="11" cy="11" r="7"/><path d="M20 20l-3.5-3.5"/>`)
	icoQuery     = icon(`<path d="M4 6h16M4 12h10M4 18h7"/><circle cx="18" cy="16" r="3"/><path d="M20.2 18.2L22 20"/>`)
	icoPage      = icon(`<path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z"/><path d="M14 3v5h5M9 13h6M9 17h4"/>`)
	icoGlobe     = icon(`<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3a14 14 0 0 1 0 18M12 3a14 14 0 0 0 0 18"/>`)
	icoChart     = icon(`<path d="M3 3v18h18"/><path d="M7 15l4-4 3 3 5-6"/>`)
	icoChannels  = icon(`<path d="M6 3v12M18 9v12M12 3v18"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="6" r="3"/>`)
	icoLanding   = icon(`<path d="M15 3h6v6M10 14L21 3M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/>`)
	icoAI        = icon(`<path d="M12 3l1.8 4.7L18.5 9.5l-4.7 1.8L12 16l-1.8-4.7L5.5 9.5l4.7-1.8z"/><path d="M19 15l.8 2.2L22 18l-2.2.8L19 21l-.8-2.2L16 18l2.2-.8z"/>`)
	icoTech      = icon(`<path d="M14.7 6.3a4 4 0 0 0 5 5L22 14l-2 2-2.4-2.4a4 4 0 0 0-5-5L10 6l2-2z"/><path d="M9.5 14.5L4 20"/>`)
	icoActions   = icon(`<path d="M9 11l3 3 8-8"/><path d="M20 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/>`)
	icoRecs      = icon(`<path d="M9 18h6M10 22h4"/><path d="M12 2a7 7 0 0 0-4 12.7V17h8v-2.3A7 7 0 0 0 12 2z"/>`)
	icoBook      = icon(`<path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20V3H6.5A2.5 2.5 0 0 0 4 5.5z"/><path d="M4 19.5A2.5 2.5 0 0 0 6.5 22H20v-5"/>`)
	icoText      = icon(`<path d="M4 6h16M4 10h16M4 14h10M4 18h7"/>`)
	icoCheck     = icon(`<path d="M20 6L9 17l-5-5"/>`)
	icoX         = icon(`<path d="M18 6L6 18M6 6l12 12"/>`)
	icoInfo      = icon(`<circle cx="12" cy="12" r="9"/><path d="M12 16v-4M12 8h.01"/>`)
	icoAlert     = icon(`<path d="M10.3 3.9L1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z"/><path d="M12 9v4M12 17h.01"/>`)
	icoTrendUp   = icon(`<path d="M3 17l6-6 4 4 8-8"/><path d="M14 7h7v7"/>`)
	icoTrendDown = icon(`<path d="M3 7l6 6 4-4 8 8"/><path d="M14 17h7v-7"/>`)
	icoBulb      = icon(`<path d="M9 18h6M10 22h4"/><path d="M12 2a7 7 0 0 0-4 12.7V17h8v-2.3A7 7 0 0 0 12 2z"/>`)
	icoPen       = icon(`<path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4z"/>`)
	icoClock     = icon(`<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 3"/>`)
	icoCalendar  = icon(`<rect x="3" y="4" width="18" height="18" rx="2"/><path d="M16 2v4M8 2v4M3 10h18"/>`)
	icoFlag      = icon(`<path d="M4 22V4M4 4h12l-2 4 2 4H4"/>`)
)

// Arrows used in delta badges (fallback when charts.DeltaArrow is empty).
var (
	arrowUp   = template.HTML(`<svg class="arr" viewBox="0 0 10 10" width="10" height="10" aria-hidden="true"><path d="M5 1.5L9 7.5H1z" fill="currentColor"/></svg>`)
	arrowDown = template.HTML(`<svg class="arr" viewBox="0 0 10 10" width="10" height="10" aria-hidden="true"><path d="M5 8.5L1 2.5h8z" fill="currentColor"/></svg>`)
	arrowFlat = template.HTML(`<svg class="arr" viewBox="0 0 10 10" width="10" height="10" aria-hidden="true"><rect x="1.5" y="4" width="7" height="2" rx="1" fill="currentColor"/></svg>`)
)

func sectionIcon(id string) template.HTML {
	switch id {
	case "summary":
		return icoSummary
	case "kpis":
		return icoKPI
	case "search_console":
		return icoSearch
	case "queries":
		return icoQuery
	case "pages":
		return icoPage
	case "countries_devices":
		return icoGlobe
	case "analytics":
		return icoChart
	case "channels":
		return icoChannels
	case "landing_pages":
		return icoLanding
	case "geo":
		return icoAI
	case "technical":
		return icoTech
	case "actions":
		return icoActions
	case "recommendations":
		return icoRecs
	case "glossary":
		return icoBook
	case idAIGlance:
		return icoAI
	case idPlanGlance:
		return icoActions
	}
	return icoText
}

var (
	icoDownload = icon(`<path d="M12 3v12M7 10l5 5 5-5"/><path d="M4 21h16"/>`)
	icoCompare  = icon(`<path d="M7 7h13M16 3l4 4-4 4"/><path d="M17 17H4M8 13l-4 4 4 4"/>`)
	icoUser     = icon(`<circle cx="12" cy="8" r="4"/><path d="M4 21a8 8 0 0 1 16 0"/>`)
	icoDB       = icon(`<ellipse cx="12" cy="5" rx="8" ry="3"/><path d="M4 5v14c0 1.7 3.6 3 8 3s8-1.3 8-3V5"/><path d="M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3"/>`)
	icoDot      = icon(`<circle cx="12" cy="12" r="3.5" fill="currentColor" stroke="none"/>`)
)

// namedIcon is exposed to templates as {{icon "name"}}.
func namedIcon(name string) template.HTML {
	switch name {
	case "spark":
		return icoAI
	case "file":
		return icoPage
	case "download":
		return icoDownload
	case "calendar":
		return icoCalendar
	case "compare":
		return icoCompare
	case "user":
		return icoUser
	case "pen":
		return icoPen
	case "db":
		return icoDB
	case "bulb":
		return icoBulb
	case "trend-up":
		return icoTrendUp
	case "trend-down":
		return icoTrendDown
	case "alert":
		return icoAlert
	case "check":
		return icoCheck
	case "dot":
		return icoDot
	case "info":
		return icoInfo
	}
	return ""
}
