// Package trilha is a file-based web framework for Go: pages, layouts, API
// routes and middleware are discovered from the app/ directory tree by the
// trilha CLI, which generates a typed registration file; this package is the
// runtime those generated files call into.
package trilha

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/emersonjoe/trilha/h"
)

// Env is the runtime environment.
type Env string

const (
	Dev  Env = "dev"
	Prod Env = "prod"
)

// Config configures an App.
type Config struct {
	// Addr is the listen address (default ":3000").
	Addr string
	// Env selects dev (stack traces, live reload, no static cache) or prod.
	Env Env
	// MaxBodyBytes limits request bodies (default 1 MiB).
	MaxBodyBytes int64
	// Logger receives structured request logs (default slog.Default()).
	Logger *slog.Logger
	// Public serves static files at the root. nil disables static files.
	Public fs.FS
	// Mounts serve static trees at URL prefixes, for the app whose disk tree
	// is not shaped like its URL tree. They match before Public, longest
	// prefix first, and fall through to it when the file is not there.
	Mounts map[string]fs.FS
	// CSRFForAPI also enforces CSRF tokens on route.go handlers.
	CSRFForAPI bool
	// CSRF renames the double-submit cookie, the form field and the header
	// (zero value = CSRFCookie, CSRFField, CSRFHeader), for an app embedded
	// in a host that already uses those names.
	CSRF CSRF
	// BasePath is the URL prefix the app is served under (e.g. "/docs" on
	// GitHub Pages). Read it with Ctx.Base when building links.
	BasePath string
	// Security tunes the hardening headers (zero value = defaults).
	Security Security
	// TrustedProxies lists CIDRs whose X-Forwarded-For/Proto are honoured.
	TrustedProxies []string
	// AllowedHosts refuses a request whose Host is not in the list, with 400,
	// before the router (cache poisoning, password-reset links). Empty = no
	// check. The port and the case do not count; "*.example.com" allows one
	// extra label. Loopback always passes in Dev. TRILHA_ALLOWED_HOSTS=a,b.
	AllowedHosts []string
	// RateLimit enables a global per-client limit (zero = off).
	RateLimit RateLimit
	// Secret signs cookies (TRILHA_SECRET); PreviousSecret still verifies.
	Secret, PreviousSecret []byte
	// Timeouts protect the server from slow clients.
	Timeouts Timeouts
	// StaticCacheControl replaces the production Cache-Control of files in
	// Public (default "public, max-age=3600"; dev always sends no-cache).
	StaticCacheControl string
	// StaticHeaders runs for every file served from Public, after the
	// defaults, and may set any header (immutable for hashed assets, CORP...).
	StaticHeaders func(name string, hdr http.Header)
	// LogRequest decides, with the response already written, whether a
	// request enters the access log. nil logs every one of them.
	LogRequest func(c *Ctx, status int, dur time.Duration) bool
	// OnSecurityEvent is called for blocked requests (CSRF, 401/403, 413, 429, panic).
	OnSecurityEvent func(SecurityEvent)
	// DevReload controls the live-reload script injected in Dev pages; Off
	// disables it (snapshot tests, HTML diffs). TRILHA_DEV_RELOAD=off does the same.
	DevReload string
	// Observability configures the health probes, the metrics endpoint and
	// what each of them reveals.
	Observability Observability
	// CORS allows other origins to call this app (zero value = off).
	CORS CORS
}

// Timeouts are the http.Server limits. Zero fields get defaults; NoTimeout
// disables one (large uploads, long polls). Write applies to the whole
// response: streams should call Ctx.Stream or Ctx.NoWriteDeadline instead of
// disabling it globally.
type Timeouts struct {
	ReadHeader     time.Duration // 10s
	Read           time.Duration // 30s
	Write          time.Duration // 60s (use Ctx.NoWriteDeadline for streams)
	Idle           time.Duration // 120s
	MaxHeaderBytes int           // 64 KiB
	// Shutdown is how long ListenAndServe waits for in-flight requests after
	// SIGINT/SIGTERM before closing (5s).
	Shutdown time.Duration
}

// NoTimeout disables a Timeouts field (becomes 0 in http.Server).
const NoTimeout time.Duration = -1

// ConfigFromEnv builds a Config from ADDR/PORT and TRILHA_ENV.
func ConfigFromEnv() Config {
	cfg := Config{Addr: ":3000", Env: Prod}
	if p := os.Getenv("PORT"); p != "" {
		cfg.Addr = ":" + p
	}
	if a := os.Getenv("ADDR"); a != "" {
		cfg.Addr = a
	}
	if strings.EqualFold(os.Getenv("TRILHA_ENV"), "dev") {
		cfg.Env = Dev
	}
	if b := strings.TrimSuffix(os.Getenv("TRILHA_BASE_PATH"), "/"); b != "" && !strings.HasPrefix(b, "/") {
		cfg.BasePath = "/" + b
	} else {
		cfg.BasePath = b
	}
	cfg.Secret, cfg.PreviousSecret, _ = secretsFromEnv()
	if strings.EqualFold(os.Getenv("TRILHA_DEV_RELOAD"), Off) {
		cfg.DevReload = Off
	}
	if p := os.Getenv("TRILHA_TRUSTED_PROXIES"); p != "" {
		cfg.TrustedProxies = strings.Split(p, ",")
	}
	if hosts := os.Getenv("TRILHA_ALLOWED_HOSTS"); hosts != "" {
		for _, host := range strings.Split(hosts, ",") {
			if host = strings.TrimSpace(host); host != "" {
				cfg.AllowedHosts = append(cfg.AllowedHosts, host)
			}
		}
	}
	cfg.Observability.Token = os.Getenv("TRILHA_OBS_TOKEN")
	cfg.Observability.Metrics = os.Getenv("TRILHA_METRICS")
	if t := os.Getenv("TRILHA_OBS_TRUSTED"); t != "" {
		cfg.Observability.Trusted = strings.Split(t, ",")
	}
	return cfg
}

// PublicFS returns the static file system for the public directory: the
// embedded copy in prod, the on-disk directory in dev (so edits show up
// without a rebuild).
func PublicFS(embedded fs.FS, dir string) fs.FS {
	if strings.EqualFold(os.Getenv("TRILHA_ENV"), "dev") {
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			return os.DirFS(dir)
		}
	}
	if embedded == nil {
		return nil
	}
	sub, err := fs.Sub(embedded, dir)
	if err != nil {
		return nil
	}
	return sub
}

// Handler function types. Every handler receives a single *Ctx.
type (
	// Next continues the middleware chain.
	Next func() error
	// PageFunc renders a page (page.go: Page).
	PageFunc func(*Ctx) (h.Node, error)
	// LayoutFunc wraps rendered children (layout.go: Layout).
	LayoutFunc func(*Ctx, h.Node) (h.Node, error)
	// HandlerFunc handles an API method or a form submission.
	HandlerFunc func(*Ctx) error
	// MiddlewareFunc intercepts a subtree (middleware.go: Middleware).
	MiddlewareFunc func(*Ctx, Next) error
	// ErrorPageFunc renders the 500 page (error.go: Error).
	ErrorPageFunc func(*Ctx, error) (h.Node, error)
)

// Route is one entry produced by the generator for App.Register.
type Route struct {
	// Pattern is the path pattern, e.g. "/blog/{slug}" or "/docs/{path...}".
	Pattern string
	// Page renders GET for page routes; nil for API routes.
	Page PageFunc
	// Methods maps HTTP methods to handlers (route.go, or form methods in page.go).
	Methods map[string]HandlerFunc
	// Layouts wrap the page, innermost first.
	Layouts []LayoutFunc
	// Middlewares run before the handler, outermost first.
	Middlewares []MiddlewareFunc
	// MiddlewaresByMethod holds chains that guard a single method
	// (MiddlewareGET, MiddlewarePOST, ... in middleware.go). The chain of a
	// method is Middlewares followed by MiddlewaresByMethod[method].
	MiddlewaresByMethod map[string][]MiddlewareFunc
	// Kind decides how errors are rendered (HTML page or JSON) and whether
	// CSRF applies. KindAuto: page.go routes are pages; route.go routes are
	// APIs, except that a browser navigation (Accept: text/html, outside
	// /api/) gets HTML error pages. route.go may export `var Kind = trilha.KindPage`.
	Kind RouteKind
	// CORS is the cross-origin policy of this route alone, from
	// `var CORS = trilha.CORS{...}` in route.go. It answers the preflight and
	// adds the headers to every response of the route, leaving the rest of the
	// app same-origin. Nil means only the app-wide Config.CORS decides.
	CORS *CORS
}

// RouteKind is the error/CSRF behaviour of a Route; see Route.Kind.
type RouteKind int

const (
	// KindAuto derives the kind from the files (page.go → page, route.go → API).
	KindAuto RouteKind = iota
	// KindPage renders errors as HTML pages and enforces CSRF on body methods.
	KindPage
	// KindAPI renders errors as JSON, whatever the Accept header says.
	KindAPI
)

// App is a configured Trilha application.
type App struct {
	shutdown    []func(*App) error
	cfg         Config
	log         *slog.Logger
	mux         *http.ServeMux
	pathMux     *http.ServeMux
	routes      map[string]*Route
	values      map[string]any
	rootLayout  LayoutFunc
	notFound    PageFunc
	errorPage   ErrorPageFunc
	exportExtra []string
	proxies     []netip.Prefix
	limiter     *limiter
	cors        *corsPolicy
	signer      *Signer

	metrics    *Metrics
	instrument bool
	mReq       *Counter
	mDur       *Histogram
	mInFlight  *Gauge
	mPanics    *Counter
	mSec       *Counter

	obsHealth   string
	obsMetrics  string
	obsTrusted  []netip.Prefix
	obsWarned   bool
	checks      []healthCheck
	healthMu    sync.Mutex
	healthCache *HealthReport
	healthAt    time.Time

	mounts   []mount
	warnedMu sync.Mutex
	warned   map[string]bool

	assetMu     sync.RWMutex
	assets      map[string]assetVersion
	assetWarned map[string]bool
}

// New creates an App. Zero values in cfg receive defaults.
func New(cfg Config) *App {
	if cfg.Addr == "" {
		cfg.Addr = ":3000"
	}
	if cfg.Env == "" {
		cfg.Env = Prod
	}
	if cfg.MaxBodyBytes == 0 {
		cfg.MaxBodyBytes = 1 << 20
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	a := &App{
		cfg:     cfg,
		mux:     http.NewServeMux(),
		pathMux: http.NewServeMux(),
		routes:  map[string]*Route{},
		values:  map[string]any{},
	}
	a.metrics = newMetrics(cfg.Logger)
	a.registerDefaultMetrics()
	a.applyConfig()
	a.mux.HandleFunc("/", a.fallback)
	a.mux.HandleFunc("GET /_trilha/events", a.devEvents)
	return a
}

// applyConfig derives logger, proxies, limiter and signer from cfg. It runs
// in New and again when serving starts, so fields changed in Setup apply.
func (a *App) applyConfig() {
	cfg := &a.cfg
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	a.log = cfg.Logger
	a.metrics.log = cfg.Logger
	cfg.CSRF = cfg.CSRF.names()
	a.parseProxies()
	a.parseMounts()
	// A response without security headers has to be a decision someone can
	// find later, so it is written down once, at boot.
	if cfg.Security.Delegated {
		a.infoOnce("security-delegated", "trilha: security headers delegated to the host; this app writes none of them")
	}
	a.cors = newCORSPolicy(cfg.CORS, cfg.CSRF.Header)
	a.applyObservability()
	if cfg.RateLimit.RPS > 0 {
		if a.limiter == nil || a.limiter.cfg != cfg.RateLimit {
			a.limiter = newLimiter(cfg.RateLimit)
		}
	} else {
		a.limiter = nil
	}
	switch {
	case len(cfg.Secret) > 0:
		a.signer = NewSigner(cfg.Secret, cfg.PreviousSecret)
	case cfg.Env == Dev:
		if a.signer == nil || !a.signer.ephemeral {
			a.signer = NewSigner(randomSecret())
			a.signer.ephemeral = true
			a.log.Info("trilha: TRILHA_SECRET missing; using an ephemeral key (dev)")
		}
	default:
		if a.signer == nil || a.signer.ephemeral || len(a.signer.keys) > 0 {
			// No warning here: an app with its own session never signs a
			// cookie, and a WARN in every boot that never means anything is
			// what teaches a team to stop reading WARN. SetSigned warns.
			a.signer = NewSigner()
		}
	}
}

// Env returns the runtime environment.
func (a *App) Env() Env { return a.cfg.Env }

// Values is a process-wide bag filled by Setup (database pools, caches...).
// Provide and Use are the typed door to it; this one is the glue for what has
// no type of its own.
func (a *App) Values() map[string]any { return a.values }

// typeKey names the slot of T in the values bag. It carries the package, so
// *farol.Deps and *crm.Deps are different keys, and it is derived, not typed
// by hand, which is the whole point.
func typeKey[T any]() string { return reflect.TypeOf((*T)(nil)).Elem().String() }

// Provide stores v as the app's dependency of its type, for Use to find. The
// place for it is Setup, which runs once, before the server:
//
//	trilha.Provide(a, &Deps{Pool: pool, Cfg: cfg})
//
// The values live in the App, not in a package variable, so a test suite that
// boots one server per test gives each one its own pool. Providing the same
// type twice replaces it.
func Provide[T any](a *App, v T) { a.values[typeKey[T]()] = v }

// Bag is where Use reads from: a *Ctx in a handler, an *App in Setup or in a
// test that has to reach what it provided. Only those two implement it.
type Bag interface{ bag() map[string]any }

func (a *App) bag() map[string]any { return a.values }
func (c *Ctx) bag() map[string]any { return c.app.values }

// Use returns what Setup provided for T:
//
//	d := trilha.Use[*Deps](c)
//
// It panics, naming the type, when nothing was provided — the alternative is
// the zero value travelling until it is dereferenced somewhere else, which is
// a crash that says nothing about its cause.
func Use[T any](b Bag) T {
	k := typeKey[T]()
	v, ok := b.bag()[k]
	if !ok {
		panic("trilha: nothing provided for " + k + "; call trilha.Provide(a, ...) in Setup")
	}
	t, ok := v.(T)
	if !ok {
		panic("trilha: value provided for " + k + " is a " + reflect.TypeOf(v).String())
	}
	return t
}

// Logger returns the app logger.
func (a *App) Logger() *slog.Logger { return a.log }

// SetRootLayout sets the layout used by the not-found and error pages.
func (a *App) SetRootLayout(l LayoutFunc) { a.rootLayout = l }

// SetNotFound sets the page rendered on 404 (app/not_found.go).
func (a *App) SetNotFound(p PageFunc) { a.notFound = p }

// SetErrorPage sets the page rendered on 500 (app/error.go).
func (a *App) SetErrorPage(e ErrorPageFunc) { a.errorPage = e }

// routeOwnsCORS reports whether the path is served by a route with a policy of
// its own. Looked up only when the app has an app-wide policy to skip.
func (a *App) routeOwnsCORS(r *http.Request) bool {
	if a.cors == nil || r.Header.Get("Origin") == "" {
		return false
	}
	_, pat := a.pathMux.Handler(r)
	if pat == "" {
		return false
	}
	key := strings.TrimSuffix(pat, "{$}")
	if key != "/" {
		key = strings.TrimSuffix(key, "/")
	}
	rt, ok := a.routes[key]
	return ok && rt.CORS != nil
}

// Handler returns the root http.Handler (useful for tests and embedding).
// Like ListenAndServe, it reapplies Config changes made in Setup.
func (a *App) Handler() http.Handler {
	a.applyConfig()
	return http.HandlerFunc(a.serveHTTP)
}

// serveHTTP answers the observability endpoints first, then routes.
func (a *App) serveHTTP(w http.ResponseWriter, r *http.Request) {
	// Before everything: a Host the app does not answer for gets no route, no
	// probe and no CORS answer.
	if len(a.cfg.AllowedHosts) > 0 && a.checkHost(w, r) {
		return
	}
	// Before the router, so the preflight is answered for any route and for
	// the static files, which never reach wrap. A route that declared its own
	// var CORS is the exception: its policy decides alone, or an app-wide list
	// could never be widened for the three paths that need it.
	if a.cors != nil && !a.routeOwnsCORS(r) && a.cors.handle(w, r) {
		return
	}
	if (a.obsHealth != "" || a.obsMetrics != "") && a.serveObservability(w, r) {
		return
	}
	if a.instrument {
		a.mInFlight.Inc()
		defer a.mInFlight.Dec()
	}
	a.mux.ServeHTTP(w, r)
}

// Metrics returns the process metric registry. It always exists; set
// Config.Observability.Metrics to expose it over HTTP.
func (a *App) Metrics() *Metrics { return a.metrics }

// registerDefaultMetrics declares the series the framework itself fills.
func (a *App) registerDefaultMetrics() {
	m := a.metrics
	a.mReq = m.Counter("trilha_requests_total", "Requests served, by method, registered route and status.", "method", "route", "status")
	a.mDur = m.Histogram("trilha_request_duration_seconds", "Request duration, in seconds.", nil, "method", "route")
	a.mInFlight = m.Gauge("trilha_requests_in_flight", "Requests being served right now.")
	a.mPanics = m.Counter("trilha_panics_total", "Panics recovered at the server edge.")
	a.mSec = m.Counter("trilha_security_events_total", "Requests blocked, by kind of event.", "kind")
}

// ListenAndServe serves until SIGINT/SIGTERM, then shuts down gracefully.
func (a *App) ListenAndServe() error {
	a.applyConfig()
	t := a.cfg.Timeouts
	srv := &http.Server{
		Addr:              a.cfg.Addr,
		Handler:           http.HandlerFunc(a.serveHTTP),
		ReadHeaderTimeout: or(t.ReadHeader, 10*time.Second),
		ReadTimeout:       or(t.Read, 30*time.Second),
		WriteTimeout:      or(t.Write, 60*time.Second),
		IdleTimeout:       or(t.Idle, 120*time.Second),
		MaxHeaderBytes:    orInt(t.MaxHeaderBytes, 64<<10),
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	a.log.Info("trilha listening", "addr", a.cfg.Addr, "env", string(a.cfg.Env))
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), or(t.Shutdown, 5*time.Second))
		defer cancel()
		err := srv.Shutdown(shutdownCtx)
		return errors.Join(err, a.runShutdown())
	}
}

// OnShutdown registers fn to run after the server stopped accepting requests
// (close pools, flush logs). Hooks run in reverse registration order; setup.go
// may export func Shutdown(a *trilha.App) error, which the generated main
// registers for you.
func (a *App) OnShutdown(fn func(*App) error) { a.shutdown = append(a.shutdown, fn) }

func (a *App) runShutdown() error {
	var errs []error
	for i := len(a.shutdown) - 1; i >= 0; i-- {
		if err := a.shutdown[i](a); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func or(v, def time.Duration) time.Duration {
	switch {
	case v == NoTimeout:
		return 0
	case v == 0:
		return def
	}
	return v
}

func orInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

// Fatal logs a fatal error and exits, ignoring the normal server-closed error.
func Fatal(err error) {
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return
	}
	slog.Error("trilha: fatal", "err", err)
	os.Exit(1)
}

// Config returns the live configuration for adjustment in Setup. Every field
// may be changed there: per-request fields (Security, Public, MaxBodyBytes,
// CSRFForAPI, BasePath, OnSecurityEvent, Static*) are read on each request;
// derived fields (Logger, Secret/PreviousSecret, RateLimit, TrustedProxies)
// are reapplied when serving starts (ListenAndServe, Handler, Export); Addr
// and Timeouts are read by ListenAndServe. To build the Config before New,
// export func Config(cfg *trilha.Config) in app/setup.go.
func (a *App) Config() *Config { return &a.cfg }
