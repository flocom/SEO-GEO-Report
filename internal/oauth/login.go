package oauth

import (
	"net/http"
	"time"
)

// VerifyPassword checks the access password submitted on a login form other
// than /authorize (e.g. the /admin page). It shares the /authorize rate
// limiter: after too many failures from the same client IP it returns false
// and the remaining wait, without checking the password. Failures are logged
// (never the submitted value) and slowed down like on /authorize.
//
// When authentication is disabled it always returns false: callers are
// expected to skip the login step entirely in that case.
func (s *Server) VerifyPassword(r *http.Request, password string) (ok bool, retryAfter time.Duration) {
	if s.cfg.Disabled {
		return false, 0
	}
	ip := ClientIP(r)
	if wait := s.loginLimiter.blocked(ip); wait > 0 {
		return false, wait
	}
	if !s.checkPassword(password) {
		s.loginLimiter.fail(ip)
		s.log.Warn("wrong password on "+r.URL.Path, "ip", ip)
		time.Sleep(PasswordFailDelay)
		return false, 0
	}
	s.loginLimiter.reset(ip)
	return true, 0
}

// PasswordFailDelay slows down each wrong password submitted to
// VerifyPassword (tests may lower it).
var PasswordFailDelay = 400 * time.Millisecond
