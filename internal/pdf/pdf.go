// Package pdf converts a self-contained HTML report into a PDF using a
// headless Chromium / Google Chrome found on the machine.
//
// The page size and margins come from the page's own @page CSS (A4).
// The executable is auto-detected; set CHROME_PATH to force one.
package pdf

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// ErrNoChrome is returned when no Chromium-based browser can be found.
var ErrNoChrome = errors.New("PDF export needs Chromium or Google Chrome: none was found " +
	"(install chromium, or set CHROME_PATH to the browser executable; the Docker image already includes it)")

// DefaultTimeout bounds one conversion when ctx has no deadline.
const DefaultTimeout = 90 * time.Second

// sem limits concurrent Chromium processes.
var sem = make(chan struct{}, 2)

// candidates are executable names looked up in PATH, in order.
var candidates = []string{
	"chromium", "chromium-browser", "google-chrome", "google-chrome-stable",
	"chrome", "headless-shell", "chrome-headless-shell", "microsoft-edge", "brave-browser",
}

// appPaths are well-known absolute install locations.
var appPaths = []string{
	"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	"/Applications/Chromium.app/Contents/MacOS/Chromium",
	"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
	"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
	"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
	"/usr/lib/chromium/chromium",
	"/usr/bin/chromium",
	"/usr/bin/chromium-browser",
	"/snap/bin/chromium",
	`C:\Program Files\Google\Chrome\Application\chrome.exe`,
	`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
	`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
}

// finder abstracts the environment for tests.
type finder struct {
	getenv   func(string) string
	lookPath func(string) (string, error)
	exists   func(string) bool
}

func (f finder) find() (string, error) {
	if p := f.getenv("CHROME_PATH"); p != "" {
		if f.exists(p) {
			return p, nil
		}
		if lp, err := f.lookPath(p); err == nil {
			return lp, nil
		}
		return "", fmt.Errorf("CHROME_PATH=%q does not point to an executable", p)
	}
	for _, name := range candidates {
		if p, err := f.lookPath(name); err == nil {
			return p, nil
		}
	}
	for _, p := range appPaths {
		if f.exists(p) {
			return p, nil
		}
	}
	return "", ErrNoChrome
}

var osFinder = finder{
	getenv:   os.Getenv,
	lookPath: exec.LookPath,
	exists: func(p string) bool {
		fi, err := os.Stat(p)
		return err == nil && !fi.IsDir()
	},
}

// FindChrome returns the path of the browser used for PDF export.
func FindChrome() (string, error) { return osFinder.find() }

// Available reports whether PDF export can work on this machine.
func Available() bool {
	_, err := FindChrome()
	return err == nil
}

// FromHTML renders html (a complete, self-contained document) to PDF.
func FromHTML(ctx context.Context, html []byte) ([]byte, error) {
	chrome, err := FindChrome()
	if err != nil {
		return nil, err
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultTimeout)
		defer cancel()
	}
	select {
	case sem <- struct{}{}:
		defer func() { <-sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	dir, err := os.MkdirTemp("", "seogeo-pdf-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	in := filepath.Join(dir, "report.html")
	out := filepath.Join(dir, "report.pdf")
	if err := os.WriteFile(in, html, 0o600); err != nil {
		return nil, err
	}
	profile := filepath.Join(dir, "profile")

	args := []string{
		"--headless=new",
		"--disable-gpu",
		"--no-sandbox",
		"--disable-dev-shm-usage",
		"--disable-extensions",
		"--disable-background-networking",
		"--disable-component-update",
		"--disable-default-apps",
		"--disable-sync",
		"--disable-breakpad",
		"--disable-crash-reporter",
		"--no-first-run",
		"--no-default-browser-check",
		"--mute-audio",
		"--hide-scrollbars",
		"--font-render-hinting=none",
		"--run-all-compositor-stages-before-draw",
		"--user-data-dir=" + profile,
		"--no-pdf-header-footer",
		"--print-to-pdf=" + out,
		"--virtual-time-budget=5000",
		fileURL(in),
	}
	cmd := exec.Command(chrome, args...)
	// Chromium wants a writable HOME (crashpad, fontconfig cache) even when
	// running as an unprivileged user without a home directory.
	cmd.Env = append(os.Environ(), "HOME="+dir, "XDG_CONFIG_HOME="+dir, "XDG_CACHE_HOME="+dir)
	var stderr lockedBuffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stderr
	setProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("pdf: cannot start %s: %w", chrome, err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	defer func() {
		// Some Chrome builds keep running after writing the PDF: make sure the
		// whole process tree is gone before the temp dir is removed.
		killProcessGroup(cmd)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	}()

	tick := time.NewTicker(150 * time.Millisecond)
	defer tick.Stop()
	var lastSize int64 = -1
	var runErr error
	exited := false
	for !exited {
		select {
		case runErr = <-done:
			exited = true
			done <- runErr // let the deferred wait return immediately
		case <-ctx.Done():
			msg := stderr.String()
			if len(msg) > 1000 {
				msg = msg[len(msg)-1000:]
			}
			return nil, fmt.Errorf("pdf: Chromium timed out: %w\n%s", ctx.Err(), msg)
		case <-tick.C:
			// The PDF is complete when it ends with %%EOF and stopped growing.
			if fi, err := os.Stat(out); err == nil && fi.Size() > 0 {
				if fi.Size() == lastSize {
					if b, err := os.ReadFile(out); err == nil && complete(b) {
						return b, nil
					}
				}
				lastSize = fi.Size()
			}
		}
	}

	pdf, readErr := os.ReadFile(out)
	if readErr == nil && complete(pdf) {
		return pdf, nil
	}
	msg := stderr.String()
	if len(msg) > 2000 {
		msg = msg[len(msg)-2000:]
	}
	if runErr != nil {
		return nil, fmt.Errorf("pdf: Chromium failed (%s): %v\n%s", chrome, runErr, msg)
	}
	return nil, fmt.Errorf("pdf: Chromium produced no PDF (%s)\n%s", chrome, msg)
}

func complete(b []byte) bool {
	if !bytes.HasPrefix(b, []byte("%PDF")) {
		return false
	}
	tail := b[max(0, len(b)-64):]
	return bytes.Contains(tail, []byte("%%EOF"))
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.b.Len() > 1<<20 {
		return len(p), nil
	}
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func fileURL(p string) string {
	p = filepath.ToSlash(p)
	if runtime.GOOS == "windows" {
		return "file:///" + p
	}
	return "file://" + p
}
