package trilha

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

// Observability configures the health, metrics and tracing surface. The zero
// value serves the health probes and nothing else: metrics have to be turned
// on explicitly, and outside dev the detailed health needs authorization
// (NIST SP 800-53 AU-9, OWASP API Security 2023 API8).
type Observability struct {
	// Health is the base path of the probes (default "/_trilha/health",
	// with /live and /ready under it). Off removes them.
	Health string
	// Metrics is the path of the Prometheus endpoint. Empty (the default)
	// means no endpoint and no request instrumentation.
	Metrics string
	// Token authorizes the detailed health and the metrics
	// (Authorization: Bearer ...). It must have at least 32 bytes; shorter
	// tokens never authorize anything. Read from TRILHA_OBS_TOKEN.
	Token string
	// Trusted lists CIDRs (or plain IPs) that skip the token, for a scraper
	// on a private network. "0.0.0.0/0" plus "::/0" opens it to everyone:
	// only do that when something in front already restricts access.
	Trusted []string
	// Details is Off to never reveal check names and errors, even to an
	// authorized client. Empty means dev shows them and prod requires
	// authorization.
	Details string
	// Timeout is the deadline of each readiness check (default 2s).
	// NoTimeout waits forever (not recommended: the probe holds a connection).
	Timeout time.Duration
	// CacheFor is how long a readiness result is reused (default 1s), so a
	// flood of probes cannot amplify into a flood of database queries.
	// NoTimeout disables the cache.
	CacheFor time.Duration
}

// minTokenLen is the smallest accepted observability token, in bytes.
const minTokenLen = 32

// applyObservability derives the paths and the trusted networks. It runs with
// applyConfig, so changes made in Setup are honoured.
func (a *App) applyObservability() {
	o := &a.cfg.Observability
	a.obsHealth = pick(o.Health, "/_trilha/health")
	a.obsMetrics = o.Metrics
	if a.obsMetrics == Off {
		a.obsMetrics = ""
	}
	a.instrument = a.obsMetrics != ""
	a.obsTrusted = a.obsTrusted[:0]
	for _, s := range o.Trusted {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if p, err := netip.ParsePrefix(s); err == nil {
			a.obsTrusted = append(a.obsTrusted, p)
			continue
		}
		if ip, err := netip.ParseAddr(s); err == nil {
			a.obsTrusted = append(a.obsTrusted, netip.PrefixFrom(ip, ip.BitLen()))
		}
	}
	if a.obsMetrics != "" && !a.obsWarned && a.cfg.Env != Dev && len(a.obsTrusted) == 0 && len(o.Token) < minTokenLen {
		a.obsWarned = true
		a.log.Warn("trilha: métricas configuradas sem token nem rede confiável; o endereço responde 401",
			"path", a.obsMetrics, "hint", "defina TRILHA_OBS_TOKEN (32+ bytes) ou Observability.Trusted")
	}
}

// serveObservability answers the probe and metrics endpoints before the
// router sees the request. They run outside the middleware chain on purpose:
// no CSRF, no layouts and, above all, no rate limiter — a liveness probe that
// gets a 429 kills a healthy process.
func (a *App) serveObservability(w http.ResponseWriter, r *http.Request) bool {
	path := r.URL.Path
	metrics := a.obsMetrics != "" && path == a.obsMetrics
	health := a.obsHealth != "" && (path == a.obsHealth ||
		path == a.obsHealth+"/live" || path == a.obsHealth+"/ready")
	if !metrics && !health {
		return false
	}
	start := time.Now()
	rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
	hdr := rw.Header()
	hdr.Set("Cache-Control", "no-store")
	hdr.Set("X-Robots-Tag", "noindex")
	hdr.Set("X-Content-Type-Options", "nosniff")
	switch {
	case r.Method != http.MethodGet && r.Method != http.MethodHead:
		hdr.Set("Allow", "GET, HEAD")
		rw.WriteHeader(http.StatusMethodNotAllowed)
	case metrics:
		a.writeMetrics(rw, r)
	default:
		a.writeHealth(rw, r, strings.HasSuffix(path, "/live"))
	}
	// Debug, not Info: a probe every second must not bury the audit trail
	// (NIST SP 800-92).
	a.log.Debug("probe", "method", r.Method, "path", path, "status", rw.status,
		"dur", time.Since(start).Round(time.Microsecond).String())
	return true
}

func (a *App) writeMetrics(w *responseWriter, r *http.Request) {
	if !a.obsAuthorized(r) {
		a.denyObservability(w, r)
		return
	}
	w.Header().Set("Content-Type", metricsContentType)
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	a.metrics.write(w)
}

func (a *App) writeHealth(w *responseWriter, r *http.Request, live bool) {
	rep := HealthReport{Status: StatusPass}
	if !live {
		rep = a.HealthReport(r.Context())
	}
	code := http.StatusOK
	if rep.Status != StatusPass {
		code = http.StatusServiceUnavailable
		w.Header().Set("Retry-After", "5")
		for _, ck := range rep.Checks {
			if ck.Status != StatusPass {
				a.log.Warn("health", "check", ck.Name, "status", ck.Status, "err", ck.Error)
			}
		}
	}
	body := rep
	if !a.showDetails(r) {
		// OWASP ASVS V7.4.1: the cause goes to the log, not to the wire.
		body = HealthReport{Status: rep.Status}
	}
	w.Header().Set("Content-Type", healthMediaType)
	w.WriteHeader(code)
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(body)
}

// showDetails reports whether this client may see check names and errors.
func (a *App) showDetails(r *http.Request) bool {
	if a.cfg.Observability.Details == Off {
		return false
	}
	return a.obsAuthorized(r)
}

// obsAuthorized accepts a bearer token (compared in constant time) or a
// request coming from a trusted network. Dev is always open.
func (a *App) obsAuthorized(r *http.Request) bool {
	if a.cfg.Env == Dev {
		return true
	}
	if tok := a.cfg.Observability.Token; len(tok) >= minTokenLen {
		if got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok &&
			subtle.ConstantTimeCompare([]byte(got), []byte(tok)) == 1 {
			return true
		}
	}
	if len(a.obsTrusted) > 0 {
		if ip, err := netip.ParseAddr(clientIPOf(a, r)); err == nil {
			for _, p := range a.obsTrusted {
				if p.Contains(ip) {
					return true
				}
			}
		}
	}
	return false
}

// denyObservability answers a scrape without credentials. It never says
// whether a token exists, and it is deliberately silent about the reason.
func (a *App) denyObservability(w *responseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="trilha"`)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	if r.Method != http.MethodHead {
		_, _ = w.Write([]byte(`{"error":"unauthorized"}` + "\n"))
	}
	a.log.Warn("security", "event", "security", "kind", "auth", "status", http.StatusUnauthorized,
		"method", r.Method, "path", r.URL.Path, "ip", clientIPOf(a, r))
	if a.instrument {
		a.mSec.With("auth").Inc()
	}
}

// observe records one finished request. route is the registered pattern,
// never the concrete path: a path carries identifiers, tokens in the query
// string and unbounded cardinality.
func (a *App) observe(method, route string, status int, start time.Time) {
	if !a.instrument {
		return
	}
	a.mReq.addTo(1, method, route, statusText(status))
	a.mDur.observeTo(time.Since(start).Seconds(), method, route)
}

// statusText avoids an allocation for the statuses that actually occur.
func statusText(code int) string {
	switch code {
	case 200:
		return "200"
	case 204:
		return "204"
	case 303:
		return "303"
	case 304:
		return "304"
	case 400:
		return "400"
	case 401:
		return "401"
	case 403:
		return "403"
	case 404:
		return "404"
	case 422:
		return "422"
	case 429:
		return "429"
	case 500:
		return "500"
	case 503:
		return "503"
	}
	return strconv.Itoa(code)
}
