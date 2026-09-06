package trilha

import (
	"net/http"
	"runtime/debug"
	"sort"
	"strings"
	"time"
)

var bodyMethods = map[string]bool{"POST": true, "PUT": true, "PATCH": true, "DELETE": true}

// Register adds a route. It is normally called only by trilha_gen.go.
func (a *App) Register(r Route) {
	route := r
	pattern := strings.TrimSuffix(r.Pattern, "/")
	if pattern == "" {
		pattern = "/"
	}
	route.Pattern = pattern
	muxPat := pattern
	if pattern == "/" {
		muxPat = "/{$}"
	}
	a.routes[pattern] = &route
	a.pathMux.HandleFunc(muxPat, func(http.ResponseWriter, *http.Request) {})
	kind := kindOf(&route)
	// The route's own policy, built once: a preflight is answered before the
	// middleware chain, the same way Config.CORS answers before the router.
	var policy *corsPolicy
	if route.CORS != nil {
		policy = newCORSPolicy(*route.CORS, a.cfg.CSRF.Header)
	}
	handle := func(pat string, h http.Handler) { a.mux.Handle(pat, withCORS(policy, h)) }
	if route.Page != nil {
		handle("GET "+muxPat, a.wrap(&route, kind, chainFor(&route, "GET"), func(c *Ctx) error { return a.renderPage(c, &route) }))
	}
	methods := make([]string, 0, len(route.Methods))
	for m := range route.Methods {
		methods = append(methods, m)
	}
	sort.Strings(methods)
	for _, m := range methods {
		fn := route.Methods[m]
		handle(m+" "+muxPat, a.wrap(&route, kind, chainFor(&route, m), fn))
	}
	if policy != nil && route.Methods["OPTIONS"] == nil {
		// Nobody wrote the preflight handler, so the policy is the handler: a
		// bare OPTIONS (no Origin) still answers what the path accepts.
		allow := a.allowFor(&route)
		a.mux.Handle("OPTIONS "+muxPat, withCORS(policy, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Allow", allow)
			w.WriteHeader(http.StatusNoContent)
		})))
	}
}

// withCORS puts the route's policy in front of a handler: handle answers the
// preflight itself and, for every other request, writes the headers the browser
// needs before the handler runs.
func withCORS(p *corsPolicy, h http.Handler) http.Handler {
	if p == nil {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if p.handle(w, req) {
			return
		}
		h.ServeHTTP(w, req)
	})
}

// Routes lists registered patterns (sorted) with their methods.
func (a *App) Routes() map[string][]string {
	out := map[string][]string{}
	for p, r := range a.routes {
		var ms []string
		if r.Page != nil {
			ms = append(ms, "GET")
		}
		for m := range r.Methods {
			ms = append(ms, m)
		}
		sort.Strings(ms)
		out[p] = ms
	}
	return out
}

// chainFor is the middleware chain of one method: the route's own chain first,
// then whatever middleware.go declared for that method alone, outermost first.
func chainFor(r *Route, method string) []MiddlewareFunc {
	extra := r.MiddlewaresByMethod[method]
	if len(extra) == 0 {
		return r.Middlewares
	}
	out := make([]MiddlewareFunc, 0, len(r.Middlewares)+len(extra))
	out = append(out, r.Middlewares...)
	return append(out, extra...)
}

// wrap builds the http.Handler for one (route, method): middleware chain,
// CSRF for form methods, error mapping, recover and logging.
func (a *App) wrap(r *Route, kind routeKind, mws []MiddlewareFunc, final HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		raw := req.Body
		req.Body = http.MaxBytesReader(rw, req.Body, a.cfg.MaxBodyBytes)
		c := newCtx(a, rw, req, kind)
		c.rawBody = raw
		c.route = r
		if kind == kindAPI && r.Kind == KindAuto && prefersHTML(req) {
			c.kind = kindPage // browser navigation to a route.go: HTML error pages
		}
		a.applySecurity(c)
		rw.Header().Set("X-Request-ID", c.requestID)

		if a.limiter != nil {
			if err := a.limiter.check(c); err != nil {
				a.handleError(c, err)
				a.logRequest(c, rw, start)
				a.observe(req.Method, r.Pattern, rw.status, start)
				return
			}
		}
		err := a.run(c, mws, func(c *Ctx) (err error) {
			defer func() {
				if v := recover(); v != nil {
					if v == http.ErrAbortHandler {
						panic(v)
					}
					if a.instrument {
						a.mPanics.Inc()
					}
					err = &panicError{value: v, stack: string(debug.Stack())}
				}
			}()
			if bodyMethods[req.Method] && (kind == kindPage || a.cfg.CSRFForAPI) {
				if err := a.checkCSRF(c); err != nil {
					return err
				}
			}
			return final(c)
		})
		a.handleError(c, err)
		if !rw.wrote {
			// Handler returned nil without writing: treat as empty 204.
			rw.WriteHeader(http.StatusNoContent)
		}
		a.logRequest(c, rw, start)
		a.observe(req.Method, r.Pattern, rw.status, start)
	})
}

// run executes middlewares outermost-first, then final.
func (a *App) run(c *Ctx, mws []MiddlewareFunc, final func(*Ctx) error) error {
	i := 0
	var next Next
	next = func() error {
		if i < len(mws) {
			m := mws[i]
			i++
			return m(c, next)
		}
		i++
		return final(c)
	}
	return next()
}

// logRequest writes the access record (NIST SP 800-53 AU-3: what, where,
// outcome and correlation). It never carries the query string, the body or a
// header: those are where secrets travel.
func (a *App) logRequest(c *Ctx, rw *responseWriter, start time.Time) {
	elapsed := time.Since(start)
	if a.cfg.LogRequest != nil && !a.cfg.LogRequest(c, rw.status, elapsed) {
		return
	}
	dur := elapsed.Round(time.Microsecond).String()
	// Both paths: "path" is the concrete one, for whoever is looking into a
	// single case; "route" is the template, for whoever is counting. An app
	// with an id in the URL has one path per record and one route per screen.
	if tid := c.TraceID(); tid != "" {
		a.log.Info("request",
			"method", c.r.Method,
			"path", c.r.URL.Path,
			"route", c.Pattern(),
			"status", rw.status,
			"bytes", rw.bytes,
			"dur", dur,
			"request_id", c.requestID,
			"trace_id", tid,
		)
		return
	}
	a.log.Info("request",
		"method", c.r.Method,
		"path", c.r.URL.Path,
		"route", c.Pattern(),
		"status", rw.status,
		"bytes", rw.bytes,
		"dur", dur,
		"request_id", c.requestID,
	)
}

// fallback handles everything the typed routes did not: static files,
// trailing-slash redirects, 405 for known paths and the 404 page.
func (a *App) fallback(w http.ResponseWriter, req *http.Request) {
	rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
	if a.instrument {
		// Static files and unmatched paths share one label: the concrete
		// path is user input and would blow up the cardinality.
		start := time.Now()
		defer func() { a.observe(req.Method, "other", rw.status, start) }()
	}
	fc := newCtx(a, rw, req, kindPage)
	a.applySecurity(fc)
	if req.Method == http.MethodGet || req.Method == http.MethodHead {
		if a.serveStatic(rw, req) {
			return
		}
	}
	// Known path, wrong method → 405.
	if _, pat := a.pathMux.Handler(req); pat != "" {
		key := strings.TrimSuffix(pat, "{$}")
		if key != "/" {
			key = strings.TrimSuffix(key, "/")
		}
		if r, ok := a.routes[key]; ok {
			allow := a.allowFor(r)
			rw.Header().Set("Allow", allow)
			fc.kind = kindOf(r)
			if fc.kind == kindAPI && r.Kind == KindAuto && prefersHTML(req) {
				fc.kind = kindPage
			}
			a.handleError(fc, &HTTPError{Code: http.StatusMethodNotAllowed})
			return
		}
	}
	// Trailing slash → canonical path.
	if p := req.URL.Path; len(p) > 1 && strings.HasSuffix(p, "/") {
		probe := req.Clone(req.Context())
		probe.URL.Path = strings.TrimSuffix(p, "/")
		if _, pat := a.pathMux.Handler(probe); pat != "" {
			u := *req.URL
			u.Path = probe.URL.Path
			http.Redirect(rw, req, u.String(), http.StatusMovedPermanently)
			return
		}
	}
	c := fc
	// No route to ask for the kind, so the Accept decides; silent (curl sends
	// */*), the /api/ prefix is the last resort.
	switch {
	case prefersHTML(req):
	case prefersJSON(req), strings.HasPrefix(req.URL.Path, "/api/"):
		c.kind = kindAPI
	}
	a.handleError(c, ErrNotFound)
}

func kindOf(r *Route) routeKind {
	switch r.Kind {
	case KindPage:
		return kindPage
	case KindAPI:
		return kindAPI
	}
	if r.Page != nil {
		return kindPage
	}
	return kindAPI
}

func (a *App) allowFor(r *Route) string {
	var ms []string
	if r.Page != nil {
		ms = append(ms, "GET", "HEAD")
	}
	if r.CORS != nil && r.Methods["OPTIONS"] == nil {
		ms = append(ms, "OPTIONS")
	}
	for m := range r.Methods {
		ms = append(ms, m)
	}
	sort.Strings(ms)
	return strings.Join(ms, ", ")
}
