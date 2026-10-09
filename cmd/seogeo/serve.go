package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/flocom/SEO-GEO-Report/internal/favicon"
	"github.com/flocom/SEO-GEO-Report/internal/mcpserver"
	"github.com/flocom/SEO-GEO-Report/internal/oauth"
	"github.com/flocom/SEO-GEO-Report/internal/pdf"
	"github.com/flocom/SEO-GEO-Report/internal/store"
	"github.com/flocom/SEO-GEO-Report/internal/update"
)

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}

func splitList(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func cmdServe(args []string) error {
	fs := newFlagSet("serve", "[--port 8080] [--data ./data]")
	port := fs.String("port", envOr("PORT", "8080"), "HTTP port (env PORT)")
	dataDir := fs.String("data", envOr("DATA_DIR", "./data"), "data directory (env DATA_DIR)")
	if _, err := parseInterspersed(fs, args); err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if err := os.MkdirAll(*dataDir, 0o755); err != nil {
		return fmt.Errorf("data directory %s: %w", *dataDir, err)
	}
	st, err := store.Open(*dataDir)
	if err != nil {
		return err
	}
	authDisabled := envBool("AUTH_DISABLED", false)
	var password string
	var source oauth.PasswordSource
	if !authDisabled {
		password, source, err = oauth.EnsurePassword(*dataDir, os.Getenv("ACCESS_PASSWORD"))
		if err != nil {
			return err
		}
	}
	publicURL := strings.TrimRight(envOr("PUBLIC_URL", ""), "/")
	auth, err := oauth.New(oauth.Config{
		DataDir:               *dataDir,
		Password:              password,
		Disabled:              authDisabled,
		PublicURL:             publicURL,
		ExtraRedirectPrefixes: splitList(os.Getenv("OAUTH_REDIRECT_ALLOWLIST")),
		Logger:                logger,
	})
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fav := favicon.New(*dataDir)
	fav.Logger = logger
	var updates *update.Checker
	if envBool("UPDATE_CHECK", true) {
		updates = update.New(version, logger)
	}
	svc := mcpserver.NewService(mcpserver.Config{
		Store:     st,
		PublicURL: publicURL,
		MCPApps:   envBool("MCP_APPS", true),
		Version:   version,
		Favicons:  fav,
		Updates:   updates,
	})
	handler := mcpserver.NewHTTPHandler(svc, auth, logger)

	addr := ":" + *port
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	absData, _ := filepath.Abs(*dataDir)
	banner(logger, *port, absData, publicURL, authDisabled, password, source)
	if updates != nil {
		updates.Start(ctx) // logs "New version ... available" when there is one
	}

	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		logger.Info("shutting down")
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}
	return nil
}

func banner(log *slog.Logger, port, dataDir, publicURL string, authDisabled bool, password string, source oauth.PasswordSource) {
	line := strings.Repeat("=", 72)
	var b strings.Builder
	fmt.Fprintf(&b, "\n%s\n  SEO & GEO Report %s — MCP server listening on port %s\n", line, version, port)
	if publicURL != "" {
		fmt.Fprintf(&b, "  MCP URL:   %s/mcp\n", publicURL)
	} else {
		fmt.Fprintf(&b, "  MCP URL:   http://localhost:%s/mcp locally, or https://<your-public-host>/mcp\n", port)
		b.WriteString("             (the public URL is detected automatically from each request)\n")
	}
	fmt.Fprintf(&b, "  Data:      %s\n", dataDir)
	if chrome, err := pdf.FindChrome(); err == nil {
		fmt.Fprintf(&b, "  PDF:       enabled (%s)\n", chrome)
	} else {
		b.WriteString("  PDF:       disabled — Chromium not found (HTML export still works; set CHROME_PATH)\n")
	}
	if authDisabled {
		b.WriteString("  Auth:      DISABLED (AUTH_DISABLED=true) — anyone reaching this server can use it\n")
	} else {
		switch source {
		case oauth.PasswordFromEnv:
			b.WriteString("  Password:  from ACCESS_PASSWORD\n")
		default:
			fmt.Fprintf(&b, "  ACCESS PASSWORD: %s\n", password)
			fmt.Fprintf(&b, "             (%s, saved in %s/access-password.txt;\n", map[oauth.PasswordSource]string{
				oauth.PasswordGenerated: "generated at first start",
				oauth.PasswordFromFile:  "generated earlier",
			}[source], dataDir)
			b.WriteString("              set ACCESS_PASSWORD to choose your own)\n")
		}
		b.WriteString("  claude.ai: Settings > Connectors > Add custom connector > paste the MCP URL,\n")
		b.WriteString("             then enter the access password on the login page.\n")
		b.WriteString("  Claude Code: claude mcp add --transport http seogeo <MCP URL> --header \"Authorization: Bearer <password>\"\n")
	}
	b.WriteString(line)
	fmt.Fprintln(os.Stderr, b.String())
	log.Info("ready", "port", port)
}

func cmdMCP(args []string) error {
	fs := newFlagSet("mcp", "[--data ./data]")
	dataDir := fs.String("data", envOr("DATA_DIR", "./data"), "data directory (env DATA_DIR)")
	if _, err := parseInterspersed(fs, args); err != nil {
		return err
	}
	// stdout carries the protocol: logs go to stderr.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))
	st, err := store.Open(*dataDir)
	if err != nil {
		return err
	}
	svc := mcpserver.NewService(mcpserver.Config{
		Store:     st,
		PublicURL: strings.TrimRight(envOr("PUBLIC_URL", ""), "/"),
		MCPApps:   envBool("MCP_APPS", true),
		Version:   version,
		Favicons:  favicon.New(*dataDir),
	})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err = svc.NewServer().Run(ctx, &mcp.StdioTransport{})
	if err != nil && ctx.Err() != nil {
		return nil
	}
	return err
}

func healthcheck(url string, timeout time.Duration) error {
	c := &http.Client{Timeout: timeout}
	resp, err := c.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned %s", resp.Status)
	}
	return nil
}

func cmdVersion() {
	fmt.Println("seogeo", version)
	if !envBool("UPDATE_CHECK", true) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c := update.New(version, nil)
	c.CheckNow(ctx)
	if r := c.Available(); r != nil {
		fmt.Printf("New version %s available: %s\n", r.Tag, r.URL)
		fmt.Println("Docker: docker compose pull && docker compose up -d (automatic with the bundled Watchtower service)")
	}
}
