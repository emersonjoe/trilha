package trilha

import (
	"net/http"
	"strings"
	"time"
)

// The Go mux routes the whole app, and it refuses to register two patterns
// when neither is more specific than the other — "/o/{slug}/login" and
// "/{lang}/cards/{deckId}" both match "/o/cards/login", so registering the two
// panics. An app ported from a file-router has both URL trees already written
// and neither is negotiable, so the kit resolves those by the rule such a
// router uses: a static segment beats a wildcard, position by position, left
// to right. Only the patterns the mux turned down are dispatched here; every
// other request goes through the mux, which decides the same way (spec 148).

type segKind uint8

const (
	segStatic segKind = iota // "cards"
	segParam                 // "{deckId}"
	segRest                  // "{path...}", always the last one
)

// patSeg is one segment of a registered pattern.
type patSeg struct {
	kind segKind
	text string // the literal, or the wildcard name
}

// routeMatcher is a registered pattern in the form this file compares and
// matches. handlers is nil for the patterns the Go mux accepted: those it only
// needs in order to answer "which pattern owns this path".
type routeMatcher struct {
	pattern  string
	segs     []patSeg
	wildcard bool // has at least one {name} or {path...}
	route    *Route
	handlers map[string]http.Handler
}

// parsePattern splits a normalized pattern ("/o/{slug}/login", never with a
// trailing slash) into segments.
func parsePattern(pattern string) []patSeg {
	trimmed := strings.Trim(pattern, "/")
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, "/")
	segs := make([]patSeg, 0, len(parts))
	for _, p := range parts {
		switch {
		case strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}"):
			name := p[1 : len(p)-1]
			if strings.HasSuffix(name, "...") {
				segs = append(segs, patSeg{kind: segRest, text: strings.TrimSuffix(name, "...")})
				continue
			}
			segs = append(segs, patSeg{kind: segParam, text: name})
		default:
			segs = append(segs, patSeg{kind: segStatic, text: p})
		}
	}
	return segs
}

func hasWildcard(segs []patSeg) bool {
	for _, s := range segs {
		if s.kind != segStatic {
			return true
		}
	}
	return false
}

// pathSegs splits a request path the way the mux does: "/docs/" keeps the
// empty segment at the end, so "/docs" does not match "/docs/{path...}".
func pathSegs(p string) []string {
	if p == "" || p == "/" {
		return nil
	}
	return strings.Split(strings.TrimPrefix(p, "/"), "/")
}

// match reports whether the pattern answers path and, when it does, the value
// of each wildcard. A {name} takes one non-empty segment; a {path...} takes
// everything that is left, slashes included, and there has to be something
// left for it to take.
func (m *routeMatcher) match(segs []string) (map[string]string, bool) {
	var vals map[string]string
	for i, s := range m.segs {
		if s.kind == segRest {
			if i > len(segs)-1 {
				return nil, false
			}
			if vals == nil {
				vals = map[string]string{}
			}
			vals[s.text] = strings.Join(segs[i:], "/")
			return vals, true
		}
		if i >= len(segs) {
			return nil, false
		}
		switch s.kind {
		case segStatic:
			if segs[i] != s.text {
				return nil, false
			}
		case segParam:
			if segs[i] == "" {
				return nil, false
			}
			if vals == nil {
				vals = map[string]string{}
			}
			vals[s.text] = segs[i]
		}
	}
	if len(segs) != len(m.segs) {
		return nil, false
	}
	return vals, true
}

// moreSpecific reports whether m answers a path that both m and other match.
// Static beats {name}, which beats {path...}, position by position; a tie on
// every comparable position goes to the longer pattern, which is the one that
// spells more of the URL out.
func (m *routeMatcher) moreSpecific(other *routeMatcher) bool {
	n := min(len(m.segs), len(other.segs))
	for i := range n {
		if a, b := m.segs[i].kind, other.segs[i].kind; a != b {
			return a < b
		}
	}
	return len(m.segs) > len(other.segs)
}

// bestRoute returns the pattern that answers path under the rule above, nil
// when no registered pattern matches it.
func (a *App) bestRoute(path string) (*routeMatcher, map[string]string) {
	segs := pathSegs(path)
	var best *routeMatcher
	var bestVals map[string]string
	for _, m := range a.matchers {
		vals, ok := m.match(segs)
		if !ok {
			continue
		}
		if best == nil || m.moreSpecific(best) {
			best, bestVals = m, vals
		}
	}
	return best, bestVals
}

// matchedPattern is the pattern that will answer this request: the mux's own
// answer, or the one the kit dispatches when it is more specific.
func (a *App) matchedPattern(req *http.Request) string {
	if len(a.conflicting) > 0 {
		m, _ := a.bestRoute(req.URL.Path)
		if m == nil {
			return ""
		}
		return m.pattern
	}
	_, pat := a.pathMux.Handler(req)
	return pat
}

// serveConflicting answers the request when the most specific pattern is one
// the Go mux refused to register. The handlers are the ones the mux would have
// received — same middleware chain, same CORS, same wrap — so this changes how
// the route is chosen and nothing about how it runs.
func (a *App) serveConflicting(w http.ResponseWriter, req *http.Request) bool {
	m, vals := a.bestRoute(req.URL.Path)
	if m == nil || m.handlers == nil {
		return false
	}
	h, ok := m.handlers[req.Method]
	if !ok && req.Method == http.MethodHead {
		h, ok = m.handlers[http.MethodGet]
	}
	if !ok {
		a.writeMethodNotAllowed(w, req, m.route)
		return true
	}
	for name, v := range vals {
		req.SetPathValue(name, v)
	}
	h.ServeHTTP(w, req)
	return true
}

// writeMethodNotAllowed answers 405 for a path that exists with a method it
// does not have, in the shape the fallback answers it.
func (a *App) writeMethodNotAllowed(w http.ResponseWriter, req *http.Request, r *Route) {
	rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
	c := newCtx(a, rw, req, kindPage)
	a.applySecurity(c)
	rw.Header().Set("Allow", a.allowFor(r))
	c.kind = kindOf(r)
	if c.kind == kindAPI && r.Kind == KindAuto && prefersHTML(req) {
		c.kind = kindPage
	}
	a.handleError(c, &HTTPError{Code: http.StatusMethodNotAllowed})
}

// staticBeforeRoutes serves a file from Mounts or Public when the route that
// would answer has a wildcard in it. Without this, an app with "/{lang}" at
// the root never serves "/ui.css": the mux hands the stylesheet to the page,
// and the page answers it (issue #203). A route spelled out in full still
// wins — that address was written by hand.
func (a *App) staticBeforeRoutes(w http.ResponseWriter, req *http.Request) bool {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		return false
	}
	if a.cfg.Public == nil && len(a.mounts) == 0 {
		return false
	}
	if !a.wildcardRoutes {
		return false
	}
	pat := a.matchedPattern(req)
	if pat == "" || !hasWildcard(parsePattern(strings.TrimSuffix(pat, "{$}"))) {
		// No route (the fallback serves the file, as it always did) or a
		// literal one, which owns the address.
		return false
	}
	if _, _, ok := a.staticFile(req.URL.Path); !ok {
		return false
	}
	// From here the answer is the file, and it is written the way the fallback
	// writes it: the hardening headers on, and one metric label for every
	// static path, because the concrete path is user input.
	rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
	if a.instrument {
		start := time.Now()
		defer func() { a.observe(req.Method, "other", rw.status, start) }()
	}
	a.applySecurity(newCtx(a, rw, req, kindPage))
	return a.serveStatic(rw, req)
}

// dispatch is the router: the file that a wildcard route would swallow, then
// the patterns the Go mux turned down, then the mux.
func (a *App) dispatch(w http.ResponseWriter, req *http.Request) {
	if a.staticBeforeRoutes(w, req) {
		return
	}
	if len(a.conflicting) > 0 && a.serveConflicting(w, req) {
		return
	}
	a.mux.ServeHTTP(w, req)
}
