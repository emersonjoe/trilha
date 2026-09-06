package trilha

import (
	"net"
	"net/http"
	"strings"
)

// hostAllowed reports whether a request whose Host header is host may be
// served. An empty list allows everything, which is what an app that never
// heard of this setting gets.
//
// The comparison ignores the port, the case and the trailing dot of an FQDN.
// A pattern may start with "*." to allow one extra label — "*.example.com"
// allows "app.example.com" but neither "example.com" nor "a.b.example.com",
// because a wildcard that crosses dots turns one customer subdomain into a
// valid host for every app behind it.
//
// In Dev the loopback names are always allowed: copying the production list
// into a dev config must not break localhost.
func hostAllowed(allowed []string, host string, env Env) bool {
	if len(allowed) == 0 {
		return true
	}
	h := canonicalHost(host)
	if h == "" {
		return false
	}
	if env == Dev && (h == "localhost" || h == "127.0.0.1" || h == "::1") {
		return true
	}
	for _, pattern := range allowed {
		if hostMatches(canonicalHost(pattern), h) {
			return true
		}
	}
	return false
}

// canonicalHost drops the port, the brackets of an IPv6 literal, the trailing
// dot and the case.
func canonicalHost(host string) string {
	h := strings.TrimSpace(host)
	if hostOnly, _, err := net.SplitHostPort(h); err == nil {
		h = hostOnly
	} else if strings.HasPrefix(h, "[") && strings.HasSuffix(h, "]") {
		h = h[1 : len(h)-1]
	}
	return strings.ToLower(strings.TrimSuffix(h, "."))
}

func hostMatches(pattern, host string) bool {
	suffix, wildcard := strings.CutPrefix(pattern, "*")
	if !wildcard {
		return pattern != "" && pattern == host
	}
	label, ok := strings.CutSuffix(host, suffix)
	return ok && label != "" && !strings.Contains(label, ".")
}

// checkHost answers 400 and reports true when the request must not go on. It
// runs before anything else: a forged Host deserves no route, no probe and no
// CORS answer.
func (a *App) checkHost(w http.ResponseWriter, r *http.Request) bool {
	if hostAllowed(a.cfg.AllowedHosts, r.Host, a.cfg.Env) {
		return false
	}
	a.securityEventFor(r, "host", http.StatusBadRequest)
	http.Error(w, "bad host", http.StatusBadRequest)
	return true
}
