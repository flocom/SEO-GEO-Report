package oauth

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
)

// BaseURL returns the public base URL (scheme://host[:port], no trailing
// slash) the request was addressed to.
//
// When override is non-empty (PUBLIC_URL) it wins. Otherwise the URL is
// derived, in order of precedence, from the RFC 7239 Forwarded header, the
// X-Forwarded-Proto / X-Forwarded-Host / X-Forwarded-Port headers, the
// Cloudflare CF-Visitor header, and finally the Host header and the TLS state
// of the connection. This makes the server work unchanged behind Caddy,
// Traefik, nginx, a Cloudflare tunnel or directly on localhost.
func BaseURL(r *http.Request, override string) string {
	if override != "" {
		return strings.TrimRight(override, "/")
	}
	proto, host := "", ""

	if fwd := r.Header.Get("Forwarded"); fwd != "" {
		// Only the first (client-facing) element matters.
		first := strings.SplitN(fwd, ",", 2)[0]
		for _, pair := range strings.Split(first, ";") {
			k, v, ok := strings.Cut(strings.TrimSpace(pair), "=")
			if !ok {
				continue
			}
			v = strings.Trim(strings.TrimSpace(v), `"`)
			switch strings.ToLower(k) {
			case "proto":
				proto = v
			case "host":
				host = v
			}
		}
	}
	if proto == "" {
		proto = firstValue(r.Header.Get("X-Forwarded-Proto"))
	}
	if proto == "" {
		proto = firstValue(r.Header.Get("X-Forwarded-Scheme"))
	}
	if proto == "" {
		if v := r.Header.Get("CF-Visitor"); v != "" {
			var cf struct {
				Scheme string `json:"scheme"`
			}
			if json.Unmarshal([]byte(v), &cf) == nil {
				proto = cf.Scheme
			}
		}
	}
	if host == "" {
		host = firstValue(r.Header.Get("X-Forwarded-Host"))
		if host != "" && !strings.Contains(stripBrackets(host), ":") {
			if port := firstValue(r.Header.Get("X-Forwarded-Port")); port != "" && validPort(port) {
				host = joinPort(host, port, proto)
			}
		}
	}
	if host == "" || !validHost(host) {
		host = r.Host
	}
	if !validHost(host) {
		host = "localhost"
	}

	proto = strings.ToLower(proto)
	if proto != "http" && proto != "https" {
		if r.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	// Drop default ports.
	if h, p, err := net.SplitHostPort(host); err == nil {
		if (proto == "https" && p == "443") || (proto == "http" && p == "80") {
			host = h
			if strings.Contains(h, ":") {
				host = "[" + h + "]"
			}
		}
	}
	return proto + "://" + strings.ToLower(host)
}

func firstValue(v string) string {
	return strings.TrimSpace(strings.SplitN(v, ",", 2)[0])
}

func joinPort(host, port, proto string) string {
	if (proto == "https" && port == "443") || (proto == "http" && port == "80") {
		return host
	}
	return host + ":" + port
}

func stripBrackets(h string) string {
	if i := strings.LastIndex(h, "]"); strings.HasPrefix(h, "[") && i > 0 {
		return h[1:i]
	}
	return h
}

func validPort(p string) bool {
	if p == "" || len(p) > 5 {
		return false
	}
	for _, c := range p {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// validHost accepts host names, IPv4, bracketed IPv6 and an optional port,
// rejecting anything that could be used for header or HTML injection.
func validHost(h string) bool {
	if h == "" || len(h) > 255 {
		return false
	}
	for _, c := range h {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '.', c == '-', c == ':', c == '[', c == ']', c == '_':
		default:
			return false
		}
	}
	return true
}

// ClientIP returns the best guess of the client IP, for rate limiting.
func ClientIP(r *http.Request) string {
	for _, h := range []string{"CF-Connecting-IP", "X-Real-IP"} {
		if v := strings.TrimSpace(r.Header.Get(h)); v != "" {
			return v
		}
	}
	if v := firstValue(r.Header.Get("X-Forwarded-For")); v != "" {
		return v
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
