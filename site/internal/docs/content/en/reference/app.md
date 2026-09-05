---
title: App and Config
description: What the generated file builds and what you can adjust in setup.go.
---

## Config

```go
type Config struct {
	Addr         string       // ":3000"; PORT/ADDR in the environment
	Env          Env          // Dev | Prod; TRILHA_ENV
	MaxBodyBytes int64        // 1 MiB
	Logger       *slog.Logger // slog.Default()
	Public       fs.FS        // static files; nil turns them off
	CSRFForAPI   bool         // require CSRF in route.go too
	BasePath     string       // URL prefix; TRILHA_BASE_PATH
	Security     Security     // headers (see Security)
	TrustedProxies []string   // CIDRs; TRILHA_TRUSTED_PROXIES
	RateLimit    RateLimit    // global per-client limit
	Secret, PreviousSecret []byte // TRILHA_SECRET, TRILHA_SECRET_PREVIOUS
	Timeouts     Timeouts     // http.Server limits (trilha.NoTimeout disables one)
	StaticCacheControl string // Cache-Control of static files in prod ("public, max-age=3600")
	StaticHeaders func(name string, hdr http.Header) // headers per static file
	OnSecurityEvent func(SecurityEvent)
	DevReload    string       // trilha.Off disables the reload script in dev; TRILHA_DEV_RELOAD=off
}
```

`trilha.ConfigFromEnv()` reads the variables; `trilha.PublicFS(embedded, "public")` picks
between the embedded copy (prod) and the folder on disk (dev).

### Where to configure

The generated file does `cfg := trilha.ConfigFromEnv()`, calls `app.Config(&cfg)` if
`app/setup.go` exports `func Config(cfg *trilha.Config)`, and then `trilha.New(cfg)` and
`app.Setup(a)`. You may change any field in either one; the only difference is *when* the
value is read:

| Fields | Read at | `Config` | `Setup` (via `a.Config()`) |
|---|---|---|---|
| `Security`, `Public`, `MaxBodyBytes`, `CSRFForAPI`, `BasePath`, `OnSecurityEvent`, `StaticCacheControl`, `StaticHeaders` | every request | ✓ | ✓ |
| `Logger`, `Secret`/`PreviousSecret`, `RateLimit`, `TrustedProxies` | derived in `New` and **reapplied** when serving starts (`ListenAndServe`, `Handler`, `Export`) | ✓ | ✓ |
| `Addr`, `Timeouts` | `ListenAndServe` | ✓ | ✓ |
| `Env` | `New` (ephemeral key in dev) and per request | ✓ | partial |

Use `Config` when you want to build the configuration from your own package (file, Vault,
flags) instead of the environment.

### Timeouts

`Timeouts.Shutdown` (5 s) is how long `ListenAndServe` waits for in-flight requests after
`SIGINT`/`SIGTERM`. Zero means "default"; `trilha.NoTimeout` disables the limit (large
uploads on a slow network, long polling). `Write` applies to the whole response: instead of
disabling it globally, a streaming route should use `c.Stream()` (SSE) or
`c.NoWriteDeadline()`.

```go
func Config(cfg *trilha.Config) {
	cfg.Timeouts.Read = trilha.NoTimeout // 32 MB uploads from a phone
}
```

### Static files

`StaticCacheControl` replaces the production `Cache-Control` (dev always sends `no-cache`).
`StaticHeaders(name, headers)` runs afterwards, per file, and may change any header:

```go
cfg.StaticCacheControl = "public, max-age=31536000, immutable" // safe with c.Asset
cfg.StaticHeaders = func(name string, h http.Header) {
	if name == "robots.txt" { h.Set("Cache-Control", "no-store") }
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
}
```

### Version in the address (`Asset`)

```go
func (a *App) Asset(path string) string
func (c *Ctx) Asset(path string) string // same thing; it is what the layout uses
```

`c.Asset("/site.css")` returns `/site.css?v=8f3a1c92`, where `v` is the FNV-1a hash of the
file's content in `Config.Public` (prefixed with `BasePath`, like `c.Base()`). Since the
address changes when the file changes, a deploy never leaves anyone with new HTML and old
CSS — the browser asks for a URL it has never seen.

A request whose `v` matches gets `public, max-age=31536000, immutable`, whatever the
`StaticCacheControl`; a wrong or missing `v` falls under the normal rule, and in `dev`
nothing is immutable. The file is read once in production; in `dev` a `Stat` decides whether
to re-read it, so editing the CSS and refreshing the page is enough.

A path that does not exist in `Public` comes back unversioned, with a warning in the log: a
typo in the layout does not take the page down. `ui.Head` and the examples already use
`Asset`.

## App

| Method | Description |
|---|---|
| `New(cfg) *App` | creates the application |
| `Register(Route)` | registers a route (called by the generated file) |
| `SetRootLayout`, `SetNotFound`, `SetErrorPage` | wire the root files |
| `Values() map[string]any` | global values set in `Setup` |
| `Logger() *slog.Logger` | the logger |
| `Env() Env` | environment |
| `Handler() http.Handler` | the root mux, for tests and for embedding in another server |
| `ListenAndServe() error` | serves with graceful shutdown on SIGINT/SIGTERM; then runs the `OnShutdown` hooks |
| `OnShutdown(func(*App) error)` | registers what to close on exit (pool, queue, flush); `setup.go` may export `Shutdown`, which the generated file registers |
| `Routes() map[string][]string` | registered patterns and their methods |
| `AddExportPath(paths...)` | extra paths for `Export` |
| `ExportPaths() []string` | what `Export` will render |
| `Export(dir) error` | writes the static site |
| `BasePath() string` | URL prefix |
| `Security() *Security` | headers, adjustable in `Setup` |
| `Config() *Config` | the whole configuration, adjustable in `Setup` (see "Where to configure") |

`trilha.Run(a)` is what the generated `main` calls: it exports if `TRILHA_EXPORT` is set,
otherwise it serves. `trilha.Fatal(err)` logs and exits, ignoring `http.ErrServerClosed`.

### Your own `main`

If any file in the project's `main` package already declares `func main()`, the generator
omits its own and writes only `newApp()`. You keep control of the lifecycle:

```go
func main() {
	a := newApp()
	if err := migrate(a); err != nil { // between Setup and the server
		trilha.Fatal(err)
	}
	trilha.Run(a)
}
```

`public/` is optional: the `//go:embed` is only generated when the folder has files.

## Testing an app

The generated file defines `newApp()`. A test in the project's `main` package can use it:

```go
func TestHome(t *testing.T) {
	rec := httptest.NewRecorder()
	newApp().Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 {
		t.Fatal(rec.Code)
	}
}
```
