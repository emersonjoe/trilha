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
	Mounts       map[string]fs.FS // static trees by URL prefix, before Public
	CSRFForAPI   bool         // require CSRF in route.go too
	CSRF         CSRF         // cookie, field and header names of the token
	BasePath     string       // URL prefix; TRILHA_BASE_PATH
	Security     Security     // headers (see Security)
	TrustedProxies []string   // CIDRs; TRILHA_TRUSTED_PROXIES
	RateLimit    RateLimit    // global per-client limit
	Secret, PreviousSecret []byte // TRILHA_SECRET, TRILHA_SECRET_PREVIOUS
	Timeouts     Timeouts     // http.Server limits (trilha.NoTimeout disables one)
	StaticCacheControl string // Cache-Control of static files in prod ("public, max-age=3600")
	StaticHeaders func(name string, hdr http.Header) // headers per static file
	LogRequest   func(c *Ctx, status int, dur time.Duration) bool // nil logs every request
	OnSecurityEvent func(SecurityEvent)
	DevReload    string       // trilha.Off disables the reload script in dev; TRILHA_DEV_RELOAD=off
	Observability Observability // health probes and the metrics endpoint
	CORS         CORS         // origins allowed to call the app (zero value = off)
}
```

`trilha.ConfigFromEnv()` reads the variables; `trilha.PublicFS(embedded, "public")` picks
between the embedded copy (prod) and the folder on disk (dev).

### Where to configure

The generated file does `cfg := trilha.ConfigFromEnv()`, calls `app.Config(&cfg)` if
`app/setup.go` exports `func Config(cfg *trilha.Config)`, and then `trilha.New(cfg)` and
`app.Setup(a)`. `Config` may also be written as `func Config(cfg *trilha.Config) error`, and
then the generated file stops the boot with your message — reading the app's own
configuration is what fails on startup, and it should fail where it happens. You may change any field in either one; the only difference is *when* the
value is read:

| Fields | Read at | `Config` | `Setup` (via `a.Config()`) |
|---|---|---|---|
| `Security`, `Public`, `MaxBodyBytes`, `CSRFForAPI`, `BasePath`, `OnSecurityEvent`, `StaticCacheControl`, `StaticHeaders` | every request | ✓ | ✓ |
| `Logger`, `Secret`/`PreviousSecret`, `RateLimit`, `TrustedProxies`, `CORS` | derived in `New` and **reapplied** when serving starts (`ListenAndServe`, `Handler`, `Export`) | ✓ | ✓ |
| `Addr`, `Timeouts` | `ListenAndServe` | ✓ | ✓ |
| `Env` | `New` (ephemeral key in dev) and per request | ✓ | partial |

Use `Config` when you want to build the configuration from your own package (file, Vault,
flags) instead of the environment.

### CSRF names

The token travels under three names, and every one of them is a default, not a rule:

| Field | Default |
|---|---|
| `CSRF.Cookie` | `trilha_csrf` |
| `CSRF.Field` | `_csrf` |
| `CSRF.Header` | `X-CSRF-Token` |

```go
cfg.CSRF = trilha.CSRF{Cookie: "billing_csrf", Field: "_billing_csrf", Header: "X-Billing-CSRF"}
```

Rename them when the app is not alone on the page: mounted inside a server that already
writes `_csrf`, two hidden fields with the same name reach the handler and the browser sends
whichever cookie it likes. An empty field keeps its default, so renaming one is one line. The
name given here is the one `CSRFInput`, `CSRFToken`, the check, the CORS allow-list and the
test client all use — there is no second place to keep in step.

### CORS

`CORS` is off while `Origins` is empty: no header is added, and `OPTIONS` keeps reaching the
router.

| Field | Meaning |
|---|---|
| `Origins []string` | exact origins (`https://app.example.com`), or the single entry `"*"` |
| `Methods []string` | default `GET, HEAD, POST, PUT, PATCH, DELETE` |
| `Headers []string` | what the client may send; default `Content-Type, Authorization, X-CSRF-Token, Trilha-Fragment` |
| `Expose []string` | response headers the other origin's script may read |
| `Credentials bool` | allows cookies and `Authorization`; incompatible with `"*"` |
| `MaxAge time.Duration` | how long the browser caches the preflight; zero omits the header |

An unsafe or malformed policy panics in `New` (`"*"` with `Credentials`, `"*"` mixed with
other origins, an origin with a path, a trailing slash or no scheme). See
[Security](/learn/security) for why.

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

### Static trees outside `public/`

`Public` serves one tree at the root, which requires the folders on disk to be shaped like
the URLs. When they are not — an icon generator that writes elsewhere, a folder shared with
another build — `Mounts` maps prefix to tree:

```go
cfg.Mounts = map[string]fs.FS{
	"/icons/": sub(embedded, "static/public/icons"),
	"/js/":    sub(embedded, "static/js"),
}
```

Mounts are tried before `Public`, longest prefix first; a prefix that matches without the
file falls through to the next one and then to `Public`, so nothing has to be exhaustive.
`StaticCacheControl`, `StaticHeaders` and `Asset` treat a mounted file like any other, and
the `name` given to `StaticHeaders` is the one from the URL (`icons/icon-192.png`), which is
what tells one mount from another.

### The request log

Every request matched by a route is logged. An app that serves its own static files sees
most of that volume say "a file was served with 200" — and a log nobody reads protects
nobody. `LogRequest` decides, with the response already written:

```go
cfg.LogRequest = func(c *trilha.Ctx, status int, _ time.Duration) bool {
	return status >= 400 || c.Pattern() != ""
}
```

It also covers "do not log the health check" and "sample 1% of the traffic". Files served
from `Public` or `Mounts` never went through this log.

The record carries both addresses: `path` is the concrete one (`/v/cmtk…/budget`), for
whoever is looking into a single case, and `route` is the template
(`/v/{tripId}/budget`), for whoever is counting. An app with an id in the URL has one path
per record and one route per screen, and rebuilding the second from the first with a regular
expression outside the app is the cardinality problem this field exists to avoid.
[`c.Pattern()`](/reference/ctx) is the same value inside the handler, and it is empty for
what the fallback answered — which is what the example above uses to keep static files out
of the log.

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
| `trilha.Provide[T](a, v)` | files a dependency under its type (see "Dependencies") |
| `trilha.Use[T](b) T` | reads it back, from a `*Ctx` or from the `*App` |
| `Values() map[string]any` | global values set in `Setup`, by name and untyped |
| `Logger() *slog.Logger` | the logger |
| `Env() Env` | environment |
| `Handler() http.Handler` | the root mux, for tests and for embedding in another server |
| `ListenAndServe() error` | serves with graceful shutdown on SIGINT/SIGTERM; then runs the `OnShutdown` hooks |
| `OnShutdown(func(*App) error)` | registers what to close on exit (pool, queue, flush); `setup.go` may export `Shutdown`, which the generated file registers |
| `Routes() map[string][]string` | registered patterns and their methods |
| `AddExportPath(paths...)` | extra paths for `Export`; a last segment with a dot exports as that file, not as `index.html` |
| `ExportPaths() []string` | what `Export` will render |
| `Export(dir) error` | writes the static site |
| `BasePath() string` | URL prefix |
| `Security() *Security` | headers, adjustable in `Setup` |
| `Config() *Config` | the whole configuration, adjustable in `Setup` (see "Where to configure") |

`trilha.Run(a)` is what the generated `main` calls: it exports if `TRILHA_EXPORT` is set,
otherwise it serves. `trilha.Fatal(err)` logs and exits, ignoring `http.ErrServerClosed`.

### Dependencies

A page needs the store, the pool, the mailer. Keeping them in package variables works right
up to the day there are two apps in one process — a host that mounts two of them, or a test
that builds a second one — and then both read the same globals and the second test to run
sees the first one's data.

```go
func Setup(a *trilha.App) error {
	store := posts.New()
	trilha.Provide(a, store)
	return nil
}
```

```go
func Page(c *trilha.Ctx) (h.Node, error) {
	store := trilha.Use[*posts.Store](c)
	...
}
```

`Provide` files the value under its type; `Use[T]` reads it back, and takes either the `*Ctx`
of a handler or the `*App` itself — which is what `Setup` and a test have in hand. A type
nobody provided panics at the call, naming the type, instead of turning up later as a nil
somewhere else.

The type is the key, so a seam is declared by writing it out: `trilha.Provide[Mailer](a,
SMTPMailer{...})` files an interface, and the handler asking `Use[Mailer](c)` never learns
which implementation it got. Without the type argument the key would be `SMTPMailer`, and the
handler would be asking for something else.

`Values()` is still there for glue by name, and `c.Get`/`c.Set` are the per-request values a
middleware leaves behind — a different question, answered in
[Middleware](/learn/middleware).

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

### An app inside another binary

When the folder declares a package other than `main`, the generated file follows it and
exports the constructor:

```go
// internal/crm/trilha_gen.go → package crm, func NewApp() *trilha.App

mux := http.NewServeMux()
mux.HandleFunc("/legacy", legacy.Handler)
mux.Handle("/", crm.NewApp().Handler())
http.ListenAndServe(":8080", mux)
```

`Handler()` returns the `http.Handler` of the whole app — routing, static files, middlewares
and error pages — so the host mounts it like any other handler. `trilha gen` needs nothing
beyond the package the folder already declares; see
[CLI](/reference/cli#an-app-inside-a-binary-that-already-exists).

## Testing an app

The generated file defines `newApp()`, and `package trilha` ships the test client, so a test
in the project's `main` package goes through the real app with no plumbing of its own:

```go
func TestHome(t *testing.T) {
	trilha.TestRequest(t, newApp(), "GET", "/").WantStatus(200).WantContains("<h1>")
}
```

| Symbol | Role |
|---|---|
| `TestingT` | `Helper()` and `Fatalf(...)`: what the helpers use from `*testing.T`, so the package never imports `testing` |
| `TestRequest(t, a *App, method, target string, opts ...TestOption) *TestResponse` | one request against the whole app |
| `TestRoute(t, r Route, method, target string, opts ...TestOption) *TestResponse` | one `route.go`, with its middlewares |
| `TestPage(t, r Route, target string, opts ...TestOption) *TestResponse` | one page, with its layouts; `Node` comes filled in |
| `NewTestClient(t, a *App) *TestClient` | the client with a cookie jar |
| `(*TestClient) Request / Get / PostForm / PostJSON` | the requests |
| `TestOption` | `WithApp`, `WithHeader`, `WithCookie`, `WithSigned`, `WithForm`, `WithJSON`, `WithBody`, `WithoutCSRF` |
| `TestResponse` | `Node`, `WantStatus`, `WantContains`, `WantHeader`, `JSON(&v)`, `Cookie(name)`; embeds `*httptest.ResponseRecorder` |

Every request carries the CSRF cookie and, on a method with a body, the matching
`X-CSRF-Token` header: cookie and token come from the same client, which is exactly what
double submit asks a browser for. `WithoutCSRF()` is how a test proves the refusal.

No assertion returns an `error` — in a test, the value of an error is stopping with the right
message, so a failure prints the target, the status and the body. Anything the ready-made
assertions do not cover is an `if` over the embedded recorder. See
[Testing](/learn/testing) for the whole trail.
