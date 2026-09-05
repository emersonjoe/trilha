// Package trilha is a file-based web framework for Go in the spirit of
// Next.js: pages, layouts, API routes and middleware are discovered from the
// app/ directory tree by the trilha CLI, which generates a typed registration
// file; this package is the runtime those generated files call into.
package trilha

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	// CSRFForAPI also enforces CSRF tokens on route.go handlers.
	CSRFForAPI bool
}

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
}

// App is a configured Trilha application.
type App struct {
	cfg        Config
	log        *slog.Logger
	mux        *http.ServeMux
	pathMux    *http.ServeMux
	routes     map[string]*Route
	values     map[string]any
	rootLayout LayoutFunc
	notFound   PageFunc
	errorPage  ErrorPageFunc
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
		log:     cfg.Logger,
		mux:     http.NewServeMux(),
		pathMux: http.NewServeMux(),
		routes:  map[string]*Route{},
		values:  map[string]any{},
	}
	a.mux.HandleFunc("/", a.fallback)
	a.mux.HandleFunc("GET /_trilha/events", a.devEvents)
	return a
}

// Env returns the runtime environment.
func (a *App) Env() Env { return a.cfg.Env }

// Values is a process-wide bag filled by Setup (database pools, caches...).
// Prefer package-level variables in your own packages; this exists for glue.
func (a *App) Values() map[string]any { return a.values }

// Logger returns the app logger.
func (a *App) Logger() *slog.Logger { return a.log }

// SetRootLayout sets the layout used by the not-found and error pages.
func (a *App) SetRootLayout(l LayoutFunc) { a.rootLayout = l }

// SetNotFound sets the page rendered on 404 (app/not_found.go).
func (a *App) SetNotFound(p PageFunc) { a.notFound = p }

// SetErrorPage sets the page rendered on 500 (app/error.go).
func (a *App) SetErrorPage(e ErrorPageFunc) { a.errorPage = e }

// Handler returns the root http.Handler (useful for tests and embedding).
func (a *App) Handler() http.Handler { return a.mux }

// ListenAndServe serves until SIGINT/SIGTERM, then shuts down gracefully.
func (a *App) ListenAndServe() error {
	srv := &http.Server{
		Addr:              a.cfg.Addr,
		Handler:           a.mux,
		ReadHeaderTimeout: 10 * time.Second,
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
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// Fatal logs a fatal error and exits, ignoring the normal server-closed error.
func Fatal(err error) {
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return
	}
	slog.Error("trilha: fatal", "err", err)
	os.Exit(1)
}
