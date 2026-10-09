// Package store keeps report drafts on disk.
//
// Layout (inside the data directory):
//
//	reports/<id>.json          the report record (metadata + model.Report)
//	reports/<id>.<lang>.html   the last rendered HTML for a language
//
// Every write is atomic (temporary file + rename) and the store is safe for
// concurrent use within a process.
package store

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/flocom/SEO-GEO-Report/internal/model"
)

// ErrNotFound is returned when a report does not exist.
var ErrNotFound = errors.New("report not found")

// Record is a stored report with its metadata.
type Record struct {
	ID         string       `json:"id"`
	ShareToken string       `json:"share_token"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
	RenderedAt *time.Time   `json:"rendered_at,omitempty"`
	Report     model.Report `json:"report"`
}

// Summary is a short description of a stored report.
type Summary struct {
	ID        string       `json:"id"`
	Title     string       `json:"title,omitempty"`
	SiteName  string       `json:"site_name"`
	SiteURL   string       `json:"site_url,omitempty"`
	Language  string       `json:"language,omitempty"`
	Period    model.Period `json:"period"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// Store is a file-based report store.
type Store struct {
	dir string

	mu     sync.Mutex
	shares map[string]string // share token -> id (lazy cache)
}

// Open creates the directory if needed and returns a Store rooted at
// <dataDir>/reports.
func Open(dataDir string) (*Store, error) {
	dir := filepath.Join(dataDir, "reports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	return &Store{dir: dir}, nil
}

// Dir returns the directory holding the report files.
func (s *Store) Dir() string { return s.dir }

// Create stores a new report and returns its record.
func (s *Store) Create(r *model.Report) (*Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createLocked(r)
}

func (s *Store) createLocked(r *model.Report) (*Record, error) {
	if r == nil {
		r = &model.Report{}
	}
	var id string
	for range 10 {
		id = NewID(r.Meta.SiteName)
		if _, err := os.Stat(s.jsonPath(id)); errors.Is(err, os.ErrNotExist) {
			break
		}
	}
	now := time.Now().UTC().Truncate(time.Second)
	rec := &Record{
		ID:         id,
		ShareToken: randomToken(24),
		CreatedAt:  now,
		UpdatedAt:  now,
		Report:     *r,
	}
	if err := s.writeLocked(rec); err != nil {
		return nil, err
	}
	if s.shares != nil {
		s.shares[rec.ShareToken] = rec.ID
	}
	return rec, nil
}

// Get loads a report by id.
func (s *Store) Get(id string) (*Record, error) {
	if !validID(id) {
		return nil, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readLocked(id)
}

// Save writes a record (UpdatedAt is refreshed).
func (s *Store) Save(rec *Record) error {
	if rec == nil || !validID(rec.ID) {
		return ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(s.jsonPath(rec.ID)); err != nil {
		return ErrNotFound
	}
	rec.UpdatedAt = time.Now().UTC()
	return s.writeLocked(rec)
}

// Update loads a report, applies fn and saves the result atomically with
// respect to other store operations. If fn returns an error nothing is saved.
func (s *Store) Update(id string, fn func(r *model.Report) error) (*Record, error) {
	if !validID(id) {
		return nil, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.readLocked(id)
	if err != nil {
		return nil, err
	}
	if err := fn(&rec.Report); err != nil {
		return nil, err
	}
	rec.UpdatedAt = time.Now().UTC()
	if err := s.writeLocked(rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// MarkRendered records the render time without touching UpdatedAt.
func (s *Store) MarkRendered(id string, at time.Time) error {
	if !validID(id) {
		return ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.readLocked(id)
	if err != nil {
		return err
	}
	at = at.UTC()
	rec.RenderedAt = &at
	return s.writeLocked(rec)
}

// List returns all reports, most recently updated first.
func (s *Store) List() ([]Summary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	out := []Summary{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		if !validID(id) {
			continue
		}
		rec, err := s.readLocked(id)
		if err != nil {
			continue
		}
		out = append(out, rec.Summary())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

// Delete removes a report and its rendered files.
func (s *Store) Delete(id string) error {
	if !validID(id) {
		return ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.readLocked(id)
	if err != nil {
		return err
	}
	if err := os.Remove(s.jsonPath(id)); err != nil {
		return err
	}
	matches, _ := filepath.Glob(filepath.Join(s.dir, id+".*.html"))
	pdfs, _ := filepath.Glob(filepath.Join(s.dir, id+".*.pdf"))
	for _, m := range append(matches, pdfs...) {
		_ = os.Remove(m)
	}
	if s.shares != nil {
		delete(s.shares, rec.ShareToken)
	}
	return nil
}

// DuplicateOptions configure Duplicate.
type DuplicateOptions struct {
	// KeepData keeps search_console, analytics and geo data. When false (the
	// default) data is cleared so the copy can be filled for the next period.
	KeepData bool
	// KeepNarrative keeps the narrative texts as they are. When false, the
	// narrative structure is kept (recommendations become the starting point,
	// section notes are cleared, actions are kept only when not done).
	KeepNarrative bool
	// Period and ComparisonPeriod, when set, replace the copied periods.
	Period           *model.Period
	ComparisonPeriod *model.Period
	Title            string
}

// Duplicate copies a report (typically to prepare the next period).
func (s *Store) Duplicate(id string, opts DuplicateOptions) (*Record, error) {
	if !validID(id) {
		return nil, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	src, err := s.readLocked(id)
	if err != nil {
		return nil, err
	}
	// Deep copy through JSON.
	var r model.Report
	b, _ := json.Marshal(src.Report)
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	if !opts.KeepData {
		r.SearchConsole, r.Analytics, r.GEO = nil, nil, nil
	}
	if !opts.KeepNarrative && r.Narrative != nil {
		n := r.Narrative
		var pending []model.Action
		for _, a := range n.Actions {
			if a.Status == "in_progress" || a.Status == "planned" {
				pending = append(pending, a)
			}
		}
		r.Narrative = &model.Narrative{
			Actions:         pending,
			Recommendations: n.Recommendations,
		}
	}
	if opts.Period != nil {
		r.Meta.Period = *opts.Period
	}
	if opts.ComparisonPeriod != nil {
		r.Meta.ComparisonPeriod = opts.ComparisonPeriod
	}
	if opts.Title != "" {
		r.Meta.Title = opts.Title
	}
	return s.createLocked(&r)
}

// ByShareToken finds a report from its public share token.
func (s *Store) ByShareToken(token string) (*Record, error) {
	if token == "" || len(token) > 128 {
		return nil, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.shares[token]; ok {
		if rec, err := s.readLocked(id); err == nil && rec.ShareToken == token {
			return rec, nil
		}
	}
	// Rebuild the cache.
	s.shares = map[string]string{}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var found *Record
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		rec, err := s.readLocked(strings.TrimSuffix(name, ".json"))
		if err != nil {
			continue
		}
		s.shares[rec.ShareToken] = rec.ID
		if rec.ShareToken == token {
			found = rec
		}
	}
	if found == nil {
		return nil, ErrNotFound
	}
	return found, nil
}

// HTMLPath returns the path of the rendered HTML for a language.
func (s *Store) HTMLPath(id, lang string) string {
	return filepath.Join(s.dir, id+"."+cleanLang(lang)+".html")
}

// SaveHTML writes the rendered HTML atomically and returns its path.
func (s *Store) SaveHTML(id, lang string, html []byte) (string, error) {
	if !validID(id) {
		return "", ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.HTMLPath(id, lang)
	return p, writeFileAtomic(p, html, 0o644)
}

// CachedHTML returns the rendered HTML if it is at least as recent as the
// report itself.
func (s *Store) CachedHTML(rec *Record, lang string) ([]byte, bool) {
	p := s.HTMLPath(rec.ID, lang)
	fi, err := os.Stat(p)
	if err != nil || stale(fi.ModTime(), rec) {
		return nil, false
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	return b, true
}

// PDFPath returns the path of the generated PDF for a language.
func (s *Store) PDFPath(id, lang string) string {
	return filepath.Join(s.dir, id+"."+cleanLang(lang)+".pdf")
}

// SavePDF writes the generated PDF atomically and returns its path.
func (s *Store) SavePDF(id, lang string, pdf []byte) (string, error) {
	if !validID(id) {
		return "", ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.PDFPath(id, lang)
	return p, writeFileAtomic(p, pdf, 0o644)
}

// CachedPDF returns the generated PDF if it is at least as recent as the
// report itself (saving the report invalidates it).
func (s *Store) CachedPDF(rec *Record, lang string) ([]byte, bool) {
	p := s.PDFPath(rec.ID, lang)
	fi, err := os.Stat(p)
	if err != nil || stale(fi.ModTime(), rec) {
		return nil, false
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	return b, true
}

// processStart invalidates files rendered by a previous version of the binary.
var processStart = time.Now()

func stale(mod time.Time, rec *Record) bool {
	return mod.Before(rec.UpdatedAt) || mod.Before(processStart)
}

// Summary returns the summary of a record.
func (rec *Record) Summary() Summary {
	return Summary{
		ID:        rec.ID,
		Title:     rec.Report.Meta.Title,
		SiteName:  rec.Report.Meta.SiteName,
		SiteURL:   rec.Report.Meta.SiteURL,
		Language:  rec.Report.Meta.Language,
		Period:    rec.Report.Meta.Period,
		CreatedAt: rec.CreatedAt,
		UpdatedAt: rec.UpdatedAt,
	}
}

// ---------------------------------------------------------------------------

func (s *Store) jsonPath(id string) string { return filepath.Join(s.dir, id+".json") }

func (s *Store) readLocked(id string) (*Record, error) {
	b, err := os.ReadFile(s.jsonPath(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var rec Record
	if err := json.Unmarshal(b, &rec); err != nil {
		return nil, fmt.Errorf("store: corrupted report %s: %w", id, err)
	}
	rec.ID = id
	return &rec, nil
}

func (s *Store) writeLocked(rec *Record) error {
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(s.jsonPath(rec.ID), b, 0o644)
}

// writeFileAtomic writes data to a temporary file in the same directory and
// renames it over path.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() {
		if tmp != "" {
			_ = os.Remove(tmp)
		}
	}()
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, perm); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	tmp = ""
	return nil
}

// WriteFileAtomic is exported for other packages persisting small files.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	return writeFileAtomic(path, data, perm)
}

var b32 = base32.NewEncoding("abcdefghijkmnpqrstuvwxyz23456789").WithPadding(base32.NoPadding)

// NewID returns a human-friendly id: a slug of the site name plus a random
// suffix, e.g. "acme-shop-k3x9qa".
func NewID(siteName string) string {
	slug := Slugify(siteName, 32)
	if slug == "" {
		slug = "report"
	}
	b := make([]byte, 5)
	_, _ = rand.Read(b)
	return slug + "-" + b32.EncodeToString(b)[:6]
}

// Slugify lowercases s, strips accents and keeps [a-z0-9-].
func Slugify(s string, max int) string {
	s = strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(s), "https://"), "http://")
	s = strings.TrimPrefix(s, "www.")
	var b strings.Builder
	dash := false
	for _, r := range s {
		if f, ok := fold[r]; ok {
			r = f
		}
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
		if b.Len() >= max {
			break
		}
	}
	return strings.Trim(b.String(), "-")
}

var fold = map[rune]rune{
	'à': 'a', 'á': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a', 'å': 'a', 'ç': 'c',
	'è': 'e', 'é': 'e', 'ê': 'e', 'ë': 'e', 'ì': 'i', 'í': 'i', 'î': 'i', 'ï': 'i',
	'ñ': 'n', 'ò': 'o', 'ó': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o', 'ø': 'o',
	'ù': 'u', 'ú': 'u', 'û': 'u', 'ü': 'u', 'ý': 'y', 'ÿ': 'y', 'œ': 'o', 'æ': 'a',
}

func randomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func validID(id string) bool {
	if id == "" || len(id) > 80 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

func cleanLang(l string) string {
	if strings.HasPrefix(strings.ToLower(l), "en") {
		return "en"
	}
	return "fr"
}
