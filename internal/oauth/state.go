package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Client is a registered OAuth client (RFC 7591).
type Client struct {
	ID               string    `json:"client_id"`
	SecretHash       string    `json:"secret_hash,omitempty"`
	Name             string    `json:"client_name,omitempty"`
	RedirectURIs     []string  `json:"redirect_uris"`
	TokenAuthMethod  string    `json:"token_endpoint_auth_method"`
	GrantTypes       []string  `json:"grant_types,omitempty"`
	Scope            string    `json:"scope,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	SecretIssuedAtTS int64     `json:"client_id_issued_at,omitempty"`
}

// token is an issued access or refresh token, stored by hash.
type token struct {
	ClientID  string    `json:"client_id"`
	Scope     string    `json:"scope,omitempty"`
	Resource  string    `json:"resource,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
	// Pair links an access token to its refresh token (hash) and back.
	Pair string `json:"pair,omitempty"`
}

// persisted is the content of <data>/oauth.json.
type persisted struct {
	Clients map[string]*Client `json:"clients"`
	Access  map[string]*token  `json:"access_tokens"`
	Refresh map[string]*token  `json:"refresh_tokens"`
}

// authCode is a short-lived authorization code (kept in memory only).
type authCode struct {
	ClientID    string
	RedirectURI string
	Challenge   string
	Scope       string
	Resource    string
	ExpiresAt   time.Time
}

type state struct {
	path string
	mu   sync.Mutex
	data persisted
}

func loadState(path string) (*state, error) {
	s := &state{path: path, data: persisted{
		Clients: map[string]*Client{},
		Access:  map[string]*token{},
		Refresh: map[string]*token{},
	}}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		return nil, fmt.Errorf("oauth: corrupted state file %s: %w", path, err)
	}
	if s.data.Clients == nil {
		s.data.Clients = map[string]*Client{}
	}
	if s.data.Access == nil {
		s.data.Access = map[string]*token{}
	}
	if s.data.Refresh == nil {
		s.data.Refresh = map[string]*token{}
	}
	return s, nil
}

// saveLocked prunes expired tokens and writes the state atomically.
func (s *state) saveLocked() error {
	now := time.Now()
	for k, t := range s.data.Access {
		if now.After(t.ExpiresAt) {
			delete(s.data.Access, k)
		}
	}
	for k, t := range s.data.Refresh {
		if now.After(t.ExpiresAt) {
			delete(s.data.Refresh, k)
		}
	}
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(s.path, b, 0o600)
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
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
	return os.Rename(tmp, path)
}

// randomString returns n random bytes, base64url encoded.
func randomString(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func hashToken(t string) string {
	h := sha256.Sum256([]byte(t))
	return hex.EncodeToString(h[:])
}

// PasswordSource tells where the access password comes from.
type PasswordSource string

const (
	PasswordFromEnv       PasswordSource = "env"
	PasswordFromFile      PasswordSource = "file"
	PasswordGenerated     PasswordSource = "generated"
	passwordFileName                     = "access-password.txt"
	minPasswordLen                       = 8
	generatedPasswordSize                = 18 // bytes -> 24 base64url chars
)

// EnsurePassword returns the access password: envValue when set, otherwise
// the content of <dataDir>/access-password.txt, otherwise a newly generated
// strong password which is persisted there (mode 0600).
func EnsurePassword(dataDir, envValue string) (string, PasswordSource, error) {
	if v := strings.TrimSpace(envValue); v != "" {
		if len(v) < minPasswordLen {
			return "", "", fmt.Errorf("ACCESS_PASSWORD must be at least %d characters", minPasswordLen)
		}
		return v, PasswordFromEnv, nil
	}
	p := filepath.Join(dataDir, passwordFileName)
	if b, err := os.ReadFile(p); err == nil {
		if v := strings.TrimSpace(string(b)); len(v) >= minPasswordLen {
			return v, PasswordFromFile, nil
		}
	}
	v := randomString(generatedPasswordSize)
	if err := writeFileAtomic(p, []byte(v+"\n"), 0o600); err != nil {
		return "", "", fmt.Errorf("cannot persist generated password in %s: %w", p, err)
	}
	return v, PasswordGenerated, nil
}
