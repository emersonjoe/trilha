# Changelog

Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); semantic
versioning. This file is written in English only.

## Unreleased

## 0.17.0 — 2026-09-05

### Added
- HTTP cache on the `Ctx` ([#26](https://github.com/emersonjoe/trilha/issues/26)):
  `c.ETag(tag)`, `c.LastModified(t)` and `c.CacheControl(v)`. The two first write their
  header and report whether the request already had that version; `true` means the `304`
  is written, so the handler returns `nil, nil` and the body never travels. Only `GET` and
  `HEAD` answer `304`; `If-None-Match` accepts a list, `*` and weak tags (RFC 9110
  §8.8.3.2), and when both are declared it is the one that decides, with the date left as
  metadata.
- Files under `static/` now carry an `ETag`: the same content fingerprint that goes in the
  `?v=` of the URL, so a second visit costs a `304` and no bytes. Two deploys of the same
  file keep the same tag.
- The post page in `examples/blog` revalidates with the post's date, alongside
  `Cache-Control: private, no-cache`.

### Notes
- Trilha does not compute an ETag from the rendered body: every response carries a fresh
  CSP nonce, so such a tag would never match twice. The version is the data's, and the
  handler is the one that knows it.

## 0.16.0 — 2026-09-05

### Added
- `cache` package ([#25](https://github.com/emersonjoe/trilha/issues/25)): an in-memory
  cache with expiry, tags and bulk invalidation. `cache.New(cache.Options{Name, MaxEntries,
  Metrics})` is created by the app, not by the framework, and the ceiling is mandatory
  (default 10 000, LRU eviction) — a cache without one is a memory leak that takes a week
  to show up. `Set`/`Get`/`Delete`/`Invalidate`/`Clear`/`Len`/`Stats` are the untyped half;
  `cache.Get[T]`, `cache.Do[T]` and `cache.Once[T]` are the typed half, because Go does not
  allow type parameters on methods.
- `cache.Do(ctx, c, key, fn)` returns the cached value or produces it with `fn`, with one
  flight per name: whoever arrives during a fetch waits for it instead of piling onto the
  database, which is what the first request after an `Invalidate` would otherwise do. An
  error is returned to everyone waiting and cached for nobody, and the cache lock is not
  held while `fn` runs, so a nested `Do` works.
- `cache.Once(c, name, fn)` answers a question once per request and forgets it with the
  response — for what a layout, a page and three components all need to know. It is not the
  cache, and cannot outlive the request the way a per-user value in a shared cache would.
- Four metric series with `Options.Metrics`, labelled by the cache's name:
  `trilha_cache_hits_total`, `trilha_cache_misses_total`, `trilha_cache_evictions_total`
  and `trilha_cache_entries`.

### Changed
- The HELP text of the five framework metrics is now in English, like the rest of the code.
  The series names are untouched, so no dashboard or alert changes.

## 0.15.0 — 2026-09-05

### Added
- `Ctx.Hijack()`, `Ctx.AllowBody(n)` and `Ctx.NoReadDeadline()`
  ([#24](https://github.com/emersonjoe/trilha/issues/24)): the two things a long connection
  and a large body were missing. Trilha's response now implements `http.Hijacker` (libraries
  type-assert on it), and `Hijack` clears the deadlines a hijacked connection would otherwise
  inherit from the server, marks the request as hijacked so the framework writes nothing more
  on it, and logs 101. `AllowBody` replaces `Config.MaxBodyBytes` for one request — call it
  from the route's `middleware.go`, since form CSRF reads the body before the handler runs —
  and going over the new limit is still a 413.
- `ui.UploadTo(id)`, `ui.UploadBar()` and `ui.UploadScript(c)`: a file upload with a progress
  bar, off until a form asks for it. The form is an ordinary
  `multipart/form-data` form with a CSRF field; with JavaScript on, the kit sends it with
  XHR, fills the `<progress>` from the browser's own progress event, fires `trilha:upload`,
  and swaps the answer in through `Trilha-Fragment` — so the same handler answers the piece
  or the whole page. A 5xx, a network error or a piece without the id submits the form for
  real. The behavior ships as a separate `public/ui.upload.js` that `ui.Head` does not load.

### Changed
- `ui.Files` now lists five names; `trilha ui --js-only` writes all three `.js` files.

### Notes
- **WebSocket stays out of core, and that is the decision.** The protocol is transport: it
  touches no route, no layout and no render, while fragmentation, control frames, the close
  handshake, UTF-8 validation, masking, backpressure and `permessage-deflate` are a few
  hundred lines the Autobahn suite tests in 500+ cases. Your app can add `coder/websocket` to
  its own go.mod — principle II binds the framework, not the app — but it cannot take those
  lines out of the framework. `Hijack` is the door, and a real handshake is covered end to
  end in `hijack_test.go`.

## 0.14.0 — 2026-09-05

### Added
- `ui.Navigate(id)`, `ui.NoNavigate()` and `ui.NavigateScript(c)`
  ([#23](https://github.com/emersonjoe/trilha/issues/23)): client navigation, off until a
  region asks for it. A click on a same-origin link inside a marked region fetches the next
  page and replaces one element of the current one, so the header, the sidebar and the
  scroll position around it do not blink. Nothing moves to the client: the address in the
  bar is the one a normal navigation would use, the route answers the same whole document,
  and without JavaScript the link is a link. Back and Forward work and restore the scroll
  position of the entry they return to; `Cmd`-click, `target`, `download` and links to
  another origin are untouched; the region gets `aria-busy` while it waits, focus moves to
  what came in, and `trilha:swap` fires, which is how an island on the new page mounts. One
  request at a time — a second click aborts the first — and a 5xx, a network error, a
  redirect or a page without that id gives up and navigates for real. The behavior ships as
  a separate `public/ui.nav.js` that `ui.Head` does not load, so an app that does not
  navigate this way downloads nothing for it.

### Changed
- `ui.Files` now lists four names and `trilha ui --js-only` writes both `.js` files.

## 0.13.0 — 2026-09-05

### Added
- `Ctx.Island(src, props, children...)`
  ([#22](https://github.com/emersonjoe/trilha/issues/22)): an interactive region inside a
  page that stays static. The server renders the children as the fallback and the browser
  loads one ES module from `public/`, whose default export is the mount function, called with
  the element and the props. The props travel as an escaped attribute and come back through
  `JSON.parse`, so a value from the database is data and never markup; props that do not
  serialize warn once and leave the fallback alone. No bundler, no global hydration: the
  module is addressed through `Asset` (content hash in the URL), only the islands on the page
  are mounted, each once, and the loader is a single inline script carrying the request nonce
  — which is what lets the default CSP accept it without `unsafe-inline`. An island that
  arrives inside a fragment mounts too: the loader listens for `trilha:swap`.

## 0.12.0 — 2026-09-05

The five oldest open issues, all from the same place: an app already running on Trilha
(Partiu, 76 routes) reporting what hurts *after* adoption.

### Added
- `func Config(cfg *trilha.Config) error` is now an accepted form in `app/setup.go`
  ([#15](https://github.com/emersonjoe/trilha/issues/15)): reading the app's own
  configuration is the operation that most often fails on boot, and it can finally fail
  where it happens — the generated file stops the boot with your message. The form without a
  return keeps working, like `Setup` (with error) and `Layout` (without) already did.
- `Config.Mounts map[string]fs.FS` ([#17](https://github.com/emersonjoe/trilha/issues/17)):
  static trees served at URL prefixes, tried before `Public`, longest prefix first, falling
  through when the file is not there. An app that already exists almost never has its disk
  tree shaped like its URL tree, and the two ways out were reorganizing the disk to please
  the router or writing an overlay `fs.FS` by hand.
- `Config.LogRequest func(c *Ctx, status int, dur time.Duration) bool`
  ([#16](https://github.com/emersonjoe/trilha/issues/16)): decides per request, with the
  response already written, what enters the access log. In the reported measurement, 74% of
  the lines were static files answered with 200. It also covers "do not log the health
  check" and "sample 1% of the traffic".
- `trilha gen --check` ([#18](https://github.com/emersonjoe/trilha/issues/18)): generates in
  memory, compares with the committed file and exits 1 showing the differing lines — one
  line in the CI, and a folder added to `app/` without `trilha gen` stops being a 404 nobody
  can explain. The generated file now also carries `//go:generate trilha gen`, and
  `trilha audit` warns when the CLI version differs from the library's in `go.mod`.

### Changed
- The missing-`TRILHA_SECRET` warning moved from every boot to the moment a cookie is
  actually signed ([#19](https://github.com/emersonjoe/trilha/issues/19)), once per cookie
  and naming it and the route. An app with its own session never signs one, and a WARN that
  appears always and never means anything is what teaches a team to stop reading WARN.
- `Asset` fingerprints files in `Mounts` too, and the `name` given to `StaticHeaders` is now
  the URL name, which is what tells one mount from another.

### Fixed
- `trilha audit` never checked calls to `auth.Cognito(...)`: 0.11.0 taught `secretArg` where
  the secret sits but the scan still looked only for `OIDC`, `EntraID` and `Keycloak`, so a
  literal Cognito secret went unreported.

## 0.11.0 — 2026-09-05

### Added
- `auth.Cognito(region, userPoolID, clientID, clientSecret, redirectURL)` (spec 020, part of
  [#41](https://github.com/emersonjoe/trilha/issues/41)): builds the issuer
  `https://cognito-idp.<region>.amazonaws.com/<userPoolID>` and reads roles from
  `cognito:groups`, with no configuration.
- `Provider.LogoutDomain`, for the one thing Amazon Cognito does outside the standard: it
  publishes no `end_session_endpoint`, so ending the session there is `GET /logout` on the
  managed login domain, with `logout_uri` instead of `post_logout_redirect_uri`. Set the
  domain and `Logout` federates; leave it empty and `Logout` clears the local session, logs
  that this is all it did, and does not pretend otherwise. Other providers ignore the field.
- `trilha audit` knows where the client secret sits in `Cognito(...)`, so a literal secret in
  that call is caught like any other.

### Changed
- The authentication chapter and the `auth` reference document the Cognito shortcut in both
  locales, and record why **Clerk** has none: its public documentation describes
  `/.well-known/jwks.json` and an `id_token` with `org_id`, but neither a
  `/.well-known/openid-configuration` — where `auth` reads every endpoint — nor a claim
  carrying the role in the organization. A shortcut built on a guess would be worse than
  none; [#41](https://github.com/emersonjoe/trilha/issues/41) stays open for it.

## 0.10.0 — 2026-09-05

### Added
- Fragments (spec 018, issues #20 and #21): `Ctx.Fragment()` returns the id the client wants
  to swap (the `Trilha-Fragment` header). On a fragment request the same route answers with
  no layouts, no document envelope and no dev server script; every HTML response now carries
  `Vary: Trilha-Fragment`, and a redirect becomes **204 with `Trilha-Location`** so the
  client navigates for real. Middleware, CSRF and status behave as before.
- `ui.Swap(id)` and `ui.NoPush()`: a marked `<a>` or `<form>` swaps element `#id` only, with
  `aria-busy` while it waits, focus on the first `[aria-invalid=true]` on 422, focus and
  caret handed back to the field in use otherwise, hydration of what came in and a
  `trilha:swap` event. On 5xx, a network error or a fragment without the id, the kit gives up
  and navigates or submits normally — with JavaScript off, link and form work as they always
  did. `window.ui.swap` and `window.ui.hydrate` do the swap by hand.
- `examples/cadastro`: a search that filters the list and a form that saves without
  reloading, with tests covering both paths (with and without the header).
- "Interactivity" chapter on the site (`/learn/interactivity`, `/pt/aprender/interatividade`)
  and reference entries for `Ctx.Fragment`, `ui.Swap` and `ui.NoPush`.

## 0.9.0 — 2026-09-05

### Added
- Internationalization (spec 015). Everything public is English by default with a Brazilian
  Portuguese translation:
  - Site: English at `/`, `/learn`, `/reference`; Portuguese at `/pt`, `/pt/aprender`,
    `/pt/referencia`. `<html lang>`, `hreflang` alternates (`en`, `pt-BR`, `x-default`) and a
    language switcher on every page. The old `/aprender/...` and `/referencia/...` URLs answer
    301 to `/pt/...`. A test keeps both locales in sync (same pages, same demos).
  - `README.md` in English + `README.pt-BR.md`; `CONTRIBUTING`, `GOVERNANCE`, `SECURITY`,
    `SUPPORT` and `CODE_OF_CONDUCT` in English with translations in `docs/pt-BR/`; issue and
    PR templates in English.
  - CLI messages in English by default, Portuguese when `TRILHA_LANG` (or `LC_ALL`,
    `LC_MESSAGES`, `LANG`) starts with `pt`. `trilha new --lang en|pt` picks the language of
    the generated texts and `<html lang>`; the default follows the CLI language.
- `App.Export` writes an HTML redirect stub (`meta refresh` + canonical + `noindex`) for
  pages that answer a same-site 3xx, so renamed URLs keep working on static hosts. Redirects
  to another origin or to the page itself are export errors.

### Changed
- Every message from the runtime, scanner, generator, dev server, scaffold, `auth`, health
  and metrics is now in English (they end up in your code, logs and terminal). `trilha.BindInvalid` defaults to
  `"invalid value"`; the export marker file says `generated by trilha export`.
- `scaffold.UIResult.Action` uses the English constants `UICreated`, `UIUpdated`, `UIKept`,
  `UIKeptTheme`, `UIModified`; the CLI translates them for display.
- Constitution 1.2.0: "English by default, Portuguese as a translation" replaces "public
  texts in Portuguese". Specs and the constitution itself stay in Portuguese; the `examples/`
  apps stay in Portuguese and the documentation says so.

## 0.8.0 — 2026-09-05

### Added
- `Ctx.Asset` and `App.Asset` (spec 017): the address of a file in `Config.Public` carries
  the content hash (`/site.css?v=8f3a1c92`), with `BasePath` applied. A request whose
  version matches gets `public, max-age=31536000, immutable`; a wrong or missing version
  keeps the previous behavior, and in `dev` nothing is immutable. The file is read once in
  production; in `dev` a `Stat` decides whether to re-read it. A path that does not exist
  comes back unversioned, with a warning.
- `trilha audit`: warning when `immutable` shows up in a project that does not use `Asset`.

### Changed
- `ui.Head`, the site layout, the examples and `trilha new` now link assets through
  `c.Asset`. This fixes the problem that started the spec: publishing the site left, for up
  to ten minutes, new HTML with old CSS and JS.

## 0.7.0 — 2026-09-05

### Added
- OpenID Connect authentication (spec 016) in the `auth` package, with no external
  dependency: `auth.OIDC`, `auth.EntraID` and `auth.Keycloak`; `Start`/`Callback`/`Logout`
  with PKCE (S256), `state` and `nonce` in 10-minute signed cookies; `id_token` validation
  against the provider's JWKS (RS256/384/512 and ES256/384, `kid` required, one-hour cache
  with key rotation throttled to one fetch per minute); session in a signed cookie with an
  absolute deadline, idle window and a new identifier on every login; optional `Store`
  (`auth.NewMemoryStore`) for immediate revocation; `Require`, `RequireRole`, `Optional` and
  `User`; roles read from each provider's place (`roles`/`groups`/`wids` on Entra ID,
  `realm_access` and `resource_access[client]` on Keycloak) and from `Options.RoleClaims`;
  federated logout when the provider publishes `end_session_endpoint`.
- `examples/sso`: protected area, required role, API answering 401 as JSON and logout, with
  integration tests. Chapter and reference on the site.
- `trilha audit`: client secret written in the code and `redirect_uri` over `http://`
  outside `localhost` (both critical).

## 0.6.0 — 2026-09-05

### Added
- Observability (spec 014): `/_trilha/health/live` and `/_trilha/health/ready` probes with
  `App.Check` (deadline, parallel execution, cache and `application/health+json` response),
  `App.HealthReport`; metrics registry `App.Metrics()` (`Counter`/`Gauge`/`Histogram` with
  labels and a cardinality cap) exposed in the Prometheus text format when
  `Observability.Metrics` is configured; framework metrics (requests, latency, in flight,
  security events, panics, runtime and `trilha_build_info`); `Ctx.TraceID` and `Ctx.Log`
  with `traceparent` propagation (W3C Trace Context); `Config.Observability` with a
  gatekeeper by token (`TRILHA_OBS_TOKEN`, constant-time comparison) or trusted network;
  three new items in `trilha audit`; chapter and reference on the site.

### Changed
- Health details and metrics are closed by default outside `dev`: without a token or a
  trusted network, the metrics endpoint answers 401 and health returns only `status`
  (NIST SP 800-53 AU-9, OWASP API Security 2023 API8).

## 0.5.3 — 2026-09-05

### Fixed
- Site: the "Form with CSRF in one line" demo did not react to submit — the
  `onclick="return false"` on the `<form>` itself cancelled every click inside it (spec 013).
  The submit is now intercepted by `tema.js` and shows the `POST → 303 → GET /eventos/<slug>`
  flow; without JavaScript the form just reloads the page.
- No inline event handlers on the site or in the examples (Trilha's default CSP blocks them):
  a leftover `onchange=""` was removed from the budget example, and a test sweeps every page
  to keep them out.

### Added
- Spec 012 (documented backlog): reduce the fixed per-request cost measured in spec 011 (CSP
  rebuilt on every request, nonce drawn even for API routes, value map always allocated, log
  formatted even when discarded).

## 0.5.2 — 2026-09-05

### Added
- Benchmarks (spec 011): `bench/` module comparing against the standard library (page, JSON,
  static file, 200 routes, middlewares), `make bench`/`make bench-results`,
  `bench/RESULTS.md`, the "Performance and comparison" page on the site and a CI job.

## 0.5.1 — 2026-09-05

### Added
- Statistics (spec 010): cookie-free page counts on the site via GoatCounter, enabled by the
  `SITE_ANALYTICS` variable; `scripts/traffic.sh` and the `traffic` workflow (daily snapshot
  of repository traffic on the `stats` branch, optional through `TRAFFIC_TOKEN`).

## 0.5.0 — 2026-09-05

### Added
- Examples (spec 009): `examples/cadastro` (medium) and `examples/orcamento` (complex), with
  READMEs and tests; "Examples" chapter on the site.
- `c.Bind(&struct)` (form or JSON, nested structs with a prefix), `trilha.FieldErrors` (422
  with `fields` in APIs), `c.Render(code, node)` (page with layouts from a POST); in the kit:
  `ui.Errors`, `ui.InvalidIf`, `ui.SelectOptions`, `ui.Checked`.

## 0.4.0 — 2026-09-05

### Added
- UI kit (spec 006): `ui` package (components, variants, Lucide icons), assets
  `public/ui.theme.css`/`ui.css`/`ui.js` copied by `trilha new` and `trilha ui`, theme
  contract compatible with shadcn/ui v4; `blog` and `assistente` examples restyled; live
  demos on the site. `h`: repeated `class` attributes are merged into one.

## 0.3.0 — 2026-09-05

### Added
- Issues #10–#14 (spec 008): `Config.DevReload` and `TRILHA_DEV_RELOAD=off` disable the
  reload script in dev; `Route.Kind` (`KindAuto`/`KindPage`/`KindAPI`) and
  `var Kind = trilha.KindPage` in `route.go`; `App.OnShutdown`, `Timeouts.Shutdown` and an
  optional `func Shutdown(a *trilha.App) error` in `setup.go`; the generator omits `main()`
  when the package already has one; folders with a dot in the name (`app.css/` → `/app.css`)
  documented and tested.

### Fixed
- `not_found.go`/`error.go`/`page.go` that write the response and return `(nil, nil)` no
  longer get a second document on top (#11).
- A `route.go` route reached by a browser (`Accept: text/html`, outside `/api/`) gets the
  HTML error page instead of JSON (#12).

## 0.2.0 — 2026-09-05

### Added
- Adoption (spec 007, issues #6–#9): optional `func Config(cfg *trilha.Config)` in
  `setup.go`, called before `trilha.New`; derived fields (`Logger`, `Secret`, `RateLimit`,
  `TrustedProxies`) reapplied when serving starts, so changes in `Setup` count;
  `trilha.NoTimeout`; `Config.StaticCacheControl` and `Config.StaticHeaders`;
  `Ctx.SetContext` and `Ctx.SetRequest`.
- AI (spec 005): `ai` package (OpenAI-compatible client with `Chat`/`Stream`, `Tool`/`Typed`,
  `Agent` with `Run`/`RunStream`, handoffs, `AsTool`, `Parallel`, `Chain`) and `ai/mcp` (MCP
  client and server over stdio and Streamable HTTP); `c.Stream()` for Server-Sent Events;
  `examples/assistente`.
- Security (spec 004): `Config.Security` (CSP with a per-request nonce, HSTS,
  Permissions-Policy, COOP), `Config.TrustedProxies` and `c.ClientIP()`, `Config.RateLimit`
  and `trilha.Limit`, signed cookies (`c.SetSigned`/`c.Signed`,
  `TRILHA_SECRET`/`TRILHA_SECRET_PREVIOUS`), `Config.Timeouts`, security events
  (`Config.OnSecurityEvent`), `c.Nonce()`/`trilha.NonceAttr`, `c.NoWriteDeadline()`,
  `a.Config()`, and the `trilha audit` command.
- `trilha export` and `App.Export`: export of static routes to HTML, with `404.html` and a
  copy of `public/`; `App.AddExportPath` for dynamic routes; `Ctx.Base()` and
  `TRILHA_BASE_PATH` for sites under a subpath.
- `trilha.Run(app)`: the generated `main` now calls it (serve or export).
- Documentation site in `site/`, built with Trilha itself and published on GitHub Pages.
- Community files: CONTRIBUTING, CODE_OF_CONDUCT, SECURITY, SUPPORT, GOVERNANCE, issue and
  PR templates, CODEOWNERS, Dependabot.

### Changed
- `X-Forwarded-Proto`/`X-Forwarded-For` are only honored when coming from `TrustedProxies`
  (before, `X-Forwarded-Proto: https` from any origin marked cookies as `Secure`).
- The generated file calls `trilha.Run(newApp())` instead of `ListenAndServe` directly
  (regenerate with `trilha gen`).

## 0.1.0 — 2026-09-05

### Added
- File-based routing in `app/`: `page.go`, `route.go`, `layout.go`, `middleware.go`,
  `not_found.go`, `error.go`, `setup.go`; `name_` segments, `name__` catch-all, `name-`
  groups.
- Runtime: `App`, `Ctx`, errors as values, CSRF, security headers, body limit, embedded
  static files, `slog` logs.
- `h` DSL and `tmpl` adapter for `html/template`.
- CLI: `new`, `gen`, `dev` (proxy + SSE reload, no rebuild for `public/`), `build`, `routes`.
- `examples/blog` and the test suite (unit, golden, integration, e2e).
