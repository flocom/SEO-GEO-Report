package render

import (
	"bytes"
	"html/template"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// md is a GitHub-flavoured Markdown converter in safe mode: raw HTML is
// dropped and dangerous link schemes (javascript:, vbscript:, data: other
// than images) are neutralized by goldmark.
var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(html.WithHardWraps()),
)

// Markdown converts user Markdown into sanitized HTML. Headings are shifted so
// that "#" becomes an <h3> inside the report hierarchy, and tables are wrapped
// in a horizontally scrollable container.
func Markdown(src string) template.HTML {
	src = strings.TrimSpace(src)
	if src == "" {
		return ""
	}
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return template.HTML("<p>" + template.HTMLEscapeString(src) + "</p>")
	}
	out := buf.String()
	// Safe: raw HTML is never emitted by the converter, so these literal tags
	// can only come from goldmark itself.
	out = strings.ReplaceAll(out, "<table>", `<div class="md-table"><table>`)
	out = strings.ReplaceAll(out, "</table>", "</table></div>")
	out = shiftHeadings(out)
	out = strings.ReplaceAll(out, "<!-- raw HTML omitted -->", "")
	out = strings.ReplaceAll(out, `<a href="http`, `<a rel="noopener noreferrer" href="http`)
	return template.HTML(out) //nolint:gosec // produced by goldmark in safe mode
}

func shiftHeadings(s string) string {
	r := strings.NewReplacer(
		"<h1", "<h3", "</h1>", "</h3>",
		"<h2", "<h4", "</h2>", "</h4>",
		"<h3", "<h5", "</h3>", "</h5>",
		"<h4", "<h6", "</h4>", "</h6>",
		"<h5", "<h6", "</h5>", "</h6>",
	)
	return r.Replace(s)
}
