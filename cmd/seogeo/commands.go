package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/flocom/SEO-GEO-Report/internal/example"
	"github.com/flocom/SEO-GEO-Report/internal/favicon"
	"github.com/flocom/SEO-GEO-Report/internal/importer"
	"github.com/flocom/SEO-GEO-Report/internal/mcpserver"
	"github.com/flocom/SEO-GEO-Report/internal/model"
	"github.com/flocom/SEO-GEO-Report/internal/pdf"
	"github.com/flocom/SEO-GEO-Report/internal/render"
)

// errSilent signals a failure already reported to the user.
var errSilent = errors.New("silent")

// parseInterspersed parses flags placed before or after positional arguments.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return pos, nil
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

func newFlagSet(name, synopsis string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: seogeo %s %s\n\nOptions:\n", name, synopsis)
		fs.PrintDefaults()
	}
	return fs
}

func readInput(path string) ([]byte, error) {
	if path == "" {
		return nil, errors.New("missing input: use -i data.json (or -i - for stdin)")
	}
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

// decodeReport decodes a report strictly (unknown fields are errors, which
// catches typos in field names).
func decodeReport(data []byte) (*model.Report, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var r model.Report
	if err := dec.Decode(&r); err != nil {
		return nil, fmt.Errorf("invalid report JSON: %w (see 'seogeo schema' and 'seogeo example')", err)
	}
	return &r, nil
}

func writeOutput(path string, data []byte) error {
	if path == "" || path == "-" {
		_, err := os.Stdout.Write(data)
		return err
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0o644)
}

func cmdRender(args []string) error {
	fs := newFlagSet("render", "-i data.json -o report.html|report.pdf [--lang fr|en] [--pdf]")
	in := fs.String("i", "", "input report JSON (\"-\" = stdin)")
	out := fs.String("o", "", "output file; .pdf renders a PDF (default: stdout, HTML)")
	lang := fs.String("lang", "", "override the report language: fr or en")
	asPDF := fs.Bool("pdf", false, "render a PDF regardless of the output extension (needs Chromium / Chrome)")
	noFavicon := fs.Bool("no-favicon", false, "do not fetch the site favicon from meta.site_url")
	if _, err := parseInterspersed(fs, args); err != nil {
		return err
	}
	data, err := readInput(*in)
	if err != nil {
		return err
	}
	r, err := decodeReport(data)
	if err != nil {
		return err
	}
	errs, warns := r.Validate()
	for _, w := range warns {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, "error:", e)
		}
		return errSilent
	}
	if !*noFavicon && favicon.Needed(r) {
		cache := ""
		if dir, err := os.UserCacheDir(); err == nil {
			cache = filepath.Join(dir, "seogeo")
		}
		fav := favicon.New(cache)
		fav.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		fav.Apply(ctx, r)
		cancel()
	}
	wantPDF := *asPDF || strings.EqualFold(filepath.Ext(*out), ".pdf")
	html, err := render.HTML(r, render.Options{Lang: *lang})
	if err != nil {
		return fmt.Errorf("rendering failed: %w", err)
	}
	if !wantPDF {
		if err := writeOutput(*out, html); err != nil {
			return err
		}
		if *out != "" && *out != "-" {
			fmt.Fprintf(os.Stderr, "HTML report written to %s (%d KB)\n", *out, len(html)/1024)
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), pdf.DefaultTimeout)
	defer cancel()
	b, err := pdf.FromHTML(ctx, html)
	if err != nil {
		hint := ""
		if *out != "" {
			hint = fmt.Sprintf("\nThe HTML export works without Chromium: seogeo render -i %s -o %s", *in, strings.TrimSuffix(*out, filepath.Ext(*out))+".html")
		}
		return fmt.Errorf("%w%s", err, hint)
	}
	if err := writeOutput(*out, b); err != nil {
		return err
	}
	if *out != "" && *out != "-" {
		fmt.Fprintf(os.Stderr, "PDF report written to %s (%d KB)\n", *out, len(b)/1024)
	}
	return nil
}

func cmdValidate(args []string) error {
	fs := newFlagSet("validate", "-i data.json")
	in := fs.String("i", "", "input report JSON (\"-\" = stdin)")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	if *in == "" && len(pos) == 1 {
		*in = pos[0]
	}
	data, err := readInput(*in)
	if err != nil {
		return err
	}
	r, err := decodeReport(data)
	if err != nil {
		return err
	}
	errs, warns := r.Validate()
	for _, e := range errs {
		fmt.Println("ERROR   ", e)
	}
	for _, w := range warns {
		fmt.Println("WARNING ", w)
	}
	c := mcpserver.CheckCompleteness(r)
	fmt.Println("Filled: ", strings.Join(c.Filled, ", "))
	fmt.Println("Missing:", strings.Join(c.Missing, ", "))
	for _, s := range c.Suggestions {
		fmt.Println("Tip:    ", s)
	}
	if len(errs) > 0 {
		return errSilent
	}
	fmt.Println("OK: the report can be rendered.")
	return nil
}

func cmdExample(args []string) error {
	fs := newFlagSet("example", "[--lang fr|en] > data.json")
	lang := fs.String("lang", "fr", "language of the demo texts: fr or en")
	if _, err := parseInterspersed(fs, args); err != nil {
		return err
	}
	b, err := example.DemoJSON(*lang)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(b)
	return err
}

func cmdSchema(args []string) error {
	fs := newFlagSet("schema", "> schema.json")
	if _, err := parseInterspersed(fs, args); err != nil {
		return err
	}
	b, err := mcpserver.SchemaJSON()
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(append(b, '\n'))
	return err
}

// mergeInto writes v at key in the report file `into` (when set) and returns
// the JSON to output.
func mergeInto(into, key string, v any) ([]byte, error) {
	if into == "" {
		return json.MarshalIndent(v, "", "  ")
	}
	data, err := os.ReadFile(into)
	if err != nil {
		return nil, err
	}
	r, err := decodeReport(data)
	if err != nil {
		return nil, err
	}
	switch key {
	case "search_console":
		r.SearchConsole = v.(*model.SearchConsole)
	case "analytics":
		r.Analytics = v.(*model.Analytics)
	}
	return json.MarshalIndent(r, "", "  ")
}

func cmdImportGSC(args []string) error {
	fs := newFlagSet("import-gsc", "<export.zip|dir> [-o gsc.json] [--into data.json]")
	out := fs.String("o", "", "output file (default stdout)")
	into := fs.String("into", "", "report JSON to merge the data into (search_console is replaced); the result goes to -o")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		fs.Usage()
		return errSilent
	}
	sc, err := importer.ImportGSCExport(pos[0])
	if err != nil {
		return err
	}
	if sc == nil {
		return errors.New("no Search Console data found in " + pos[0])
	}
	b, err := mergeInto(*into, "search_console", sc)
	if err != nil {
		return err
	}
	return writeOutput(*out, append(b, '\n'))
}

func cmdImportGA4(args []string) error {
	fs := newFlagSet("import-ga4", "<file.csv>... [-o ga4.json] [--into data.json]")
	out := fs.String("o", "", "output file (default stdout)")
	into := fs.String("into", "", "report JSON to merge the data into (analytics is updated); the result goes to -o")
	kind := fs.String("kind", "", "table kind for every file: channels, sources, landing_pages or daily (default: guessed)")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(pos) == 0 {
		fs.Usage()
		return errSilent
	}
	var a *model.Analytics
	if *into != "" {
		data, err := os.ReadFile(*into)
		if err != nil {
			return err
		}
		r, err := decodeReport(data)
		if err != nil {
			return err
		}
		a = r.Analytics
	}
	for _, p := range pos {
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		a, err = importer.ParseGA4CSV(data, importer.GA4Kind(*kind), a)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
	}
	if a == nil {
		return errors.New("no GA4 data found")
	}
	b, err := mergeInto(*into, "analytics", a)
	if err != nil {
		return err
	}
	return writeOutput(*out, append(b, '\n'))
}

func cmdHealthcheck(args []string) error {
	fs := newFlagSet("healthcheck", "[--url http://127.0.0.1:8080/healthz]")
	url := fs.String("url", "http://127.0.0.1:"+envOr("PORT", "8080")+"/healthz", "health URL")
	if _, err := parseInterspersed(fs, args); err != nil {
		return err
	}
	return healthcheck(*url, 3*time.Second)
}
