package admin

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// sessionKeyFile holds the random secret signing the admin session cookies,
// so that sessions survive restarts (and therefore updates).
const sessionKeyFile = "admin-session.key"

// LoadSessionKey returns the key signing admin sessions: a random secret
// persisted in dataDir, mixed with the access password so that changing the
// password invalidates every session. When the data directory is not
// writable, an ephemeral key is used (sessions then end at restart).
func LoadSessionKey(dataDir, password string) []byte {
	secret := readOrCreateSecret(filepath.Join(dataDir, sessionKeyFile))
	m := hmac.New(sha256.New, secret)
	m.Write([]byte("seogeo admin session v1\x00"))
	m.Write([]byte(password))
	return m.Sum(nil)
}

func readOrCreateSecret(path string) []byte {
	if b, err := os.ReadFile(path); err == nil {
		if k, err := hex.DecodeString(strings.TrimSpace(string(b))); err == nil && len(k) >= 32 {
			return k
		}
	}
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		panic(err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(hex.EncodeToString(k)+"\n"), 0o600); err == nil {
		if os.Rename(tmp, path) != nil {
			_ = os.Remove(tmp)
		}
	}
	return k
}

// session is the content of the signed cookie.
type session struct {
	ID  string // random, identifies the session (CSRF token, logout)
	Exp time.Time
}

var errBadSession = errors.New("invalid session")

// sessions signs and verifies stateless session cookies. Logged-out session
// IDs are remembered until they expire.
type sessions struct {
	key []byte
	ttl time.Duration
	now func() time.Time

	mu      sync.Mutex
	revoked map[string]time.Time
}

func newSessions(key []byte, ttl time.Duration, now func() time.Time) *sessions {
	return &sessions{key: key, ttl: ttl, now: now, revoked: map[string]time.Time{}}
}

func (s *sessions) mac(parts ...string) []byte {
	m := hmac.New(sha256.New, s.key)
	for _, p := range parts {
		m.Write([]byte(p))
		m.Write([]byte{0})
	}
	return m.Sum(nil)
}

// issue returns a new session and its cookie value.
func (s *sessions) issue() (session, string) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	ss := session{ID: base64.RawURLEncoding.EncodeToString(b), Exp: s.now().Add(s.ttl).Truncate(time.Second)}
	payload := "v1." + ss.ID + "." + strconv.FormatInt(ss.Exp.Unix(), 10)
	return ss, payload + "." + base64.RawURLEncoding.EncodeToString(s.mac("cookie", payload))
}

// parse verifies a cookie value.
func (s *sessions) parse(v string) (session, error) {
	parts := strings.Split(v, ".")
	if len(parts) != 4 || parts[0] != "v1" {
		return session{}, errBadSession
	}
	payload := strings.Join(parts[:3], ".")
	sig, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil || !hmac.Equal(sig, s.mac("cookie", payload)) {
		return session{}, errBadSession
	}
	exp, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return session{}, errBadSession
	}
	ss := session{ID: parts[1], Exp: time.Unix(exp, 0)}
	if !s.now().Before(ss.Exp) {
		return session{}, errBadSession
	}
	s.mu.Lock()
	_, gone := s.revoked[ss.ID]
	s.mu.Unlock()
	if gone {
		return session{}, errBadSession
	}
	return ss, nil
}

// revoke invalidates a session (logout) until its natural expiry.
func (s *sessions) revoke(ss session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for id, exp := range s.revoked {
		if now.After(exp) {
			delete(s.revoked, id)
		}
	}
	s.revoked[ss.ID] = ss.Exp
}

// csrf returns the anti-CSRF token bound to a session.
func (s *sessions) csrf(ss session) string {
	return base64.RawURLEncoding.EncodeToString(s.mac("csrf", ss.ID))
}

func (s *sessions) validCSRF(ss session, token string) bool {
	if token == "" {
		return false
	}
	return hmac.Equal([]byte(token), []byte(s.csrf(ss)))
}
