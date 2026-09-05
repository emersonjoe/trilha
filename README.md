# Trilha

> 🇺🇸 English · [🇧🇷 Português](README.pt-BR.md)

[![ci](https://github.com/emersonjoe/trilha/actions/workflows/ci.yml/badge.svg)](https://github.com/emersonjoe/trilha/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/emersonjoe/trilha.svg)](https://pkg.go.dev/github.com/emersonjoe/trilha)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**Web framework for Go with file-based routing.** Nested layouts, API routes, per-folder
middleware, a dev server with live reload and a single production binary. Zero dependencies
outside the standard library. The folder layout follows the model popularized by Next.js\*,
translated into Go conventions.

```
app/
├── layout.go            → root <html> (wraps everything)
├── page.go              → GET /
├── middleware.go        → runs on every request
├── not_found.go         → 404 page
├── error.go             → 500 page
├── setup.go             → startup (database, cache...)
├── blog/
│   ├── layout.go        → wraps /blog/**
│   ├── page.go          → GET /blog
│   ├── new/page.go      → GET /blog/new  (+ the form's POST)
│   └── slug_/page.go    → GET /blog/{slug}
├── docs/path__/page.go  → GET /docs/{path...}
├── marketing-/          → group: not part of the URL
│   ├── layout.go        → wraps /pricing and /about
│   ├── pricing/page.go  → GET /pricing
│   └── about/page.go    → GET /about
├── admin/
│   ├── middleware.go    → only for /admin/**
│   └── page.go
└── api/posts/route.go   → GET/POST /api/posts
public/style.css         → served at /style.css
```

## Documentation

**<https://emersonjoe.github.io/trilha>** — the "Learn" track (from `trilha new` to deploy,
with challenges) and a per-package "Reference". The site is a Trilha app, exported with
`trilha export`, in English (`/`) and Portuguese (`/pt`).

## Getting started

```bash
go install github.com/emersonjoe/trilha/cmd/trilha@latest
trilha new my-app && cd my-app
trilha dev              # → http://localhost:3000, reloads on save
trilha build            # → bin/my-app, with public/ embedded
```

Not published yet? Use a local copy: `trilha new my-app --trilha-dir ../trilha`. The CLI
speaks English by default and Portuguese with `TRILHA_LANG=pt` (or a `pt_*` `LANG`);
`trilha new --lang pt` generates the project texts in Portuguese.

## Conventions

| File | Exports | Signature |
|------|---------|-----------|
| `page.go` | `Page` | `func(c *trilha.Ctx) (h.Node, error)` |
| `page.go` | `POST`, `PUT`, `PATCH`, `DELETE` (optional) | `func(c *trilha.Ctx) error` — forms, with CSRF |
| `route.go` | `GET`, `POST`, `PUT`, `PATCH`, `DELETE` | `func(c *trilha.Ctx) error` — API |
| `layout.go` | `Layout` | `func(c *trilha.Ctx, children h.Node) (h.Node, error)` |
| `middleware.go` | `Middleware` | `func(c *trilha.Ctx, next trilha.Next) error` |
| `not_found.go` (root) | `NotFound` | `func(c *trilha.Ctx) (h.Node, error)` |
| `error.go` (root) | `Error` | `func(c *trilha.Ctx, err error) (h.Node, error)` |
| `setup.go` (root) | `Setup` | `func(a *trilha.App) error` |
| `setup.go` (optional) | `Config` | `func(cfg *trilha.Config)`, before `trilha.New` |

Folders become segments: `blog` → `/blog`; `slug_` → `/{slug}`; `path__` → `/{path...}`
(catch-all, must be a leaf); `marketing-` → **route group**: not part of the URL, but its
`layout.go`/`middleware.go` apply to everything below (the equivalent of Next.js's
`(marketing)`). `[slug]` and `(group)` are not valid in a Go import path, hence the `_` and
`-` suffixes. Folders starting with `_` or `.` are ignored. Two folders producing the same
URL are a generation error (`E_DUPLICATE_ROUTE`).

Execution order for `GET /admin`:
`middleware(app) → middleware(app/admin) → Page → layout(app/admin)? → layout(app)`.

## A page

```go
package about

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("About")
	return h.Main(
		h.H1(h.Text("About")),
		h.P(h.Textf("You are request %s.", c.RequestID())),
	), nil
}
```

`h` is a typed HTML DSL: elements and attributes are functions, text is escaped by default
and `h.Raw` is the only unescaped door. `h.If`, `h.Map` and `h.Fragment` cover control flow.

Prefer `html/template`? The `tmpl` package plugs templates into the same pipeline (layouts,
title, the contextual escaping of `html/template` itself):

```go
//go:embed report.html
var files embed.FS
var t = tmpl.Must(files, "*.html") // fails at startup, never during a request

func Page(c *trilha.Ctx) (h.Node, error) {
	return tmpl.Node(t, "report", data), nil
}
```

## Forms and APIs

```go
// app/blog/new/page.go
func Page(c *trilha.Ctx) (h.Node, error) {
	return h.Form(h.Method("post"), trilha.CSRFInput(c),
		h.Input(h.Name("title")), h.Button(h.Text("Publish"))), nil
}

func POST(c *trilha.Ctx) error {
	p := posts.Create(c.Form("title"), c.Form("body"))
	return c.Redirect("/blog/" + p.Slug) // 303: POST → redirect → GET
}

// app/api/posts/route.go
func GET(c *trilha.Ctx) error { return c.JSON(200, posts.All()) }
func POST(c *trilha.Ctx) error {
	var in struct{ Title string `json:"title"` }
	if err := c.BindJSON(&in); err != nil { return err } // 400 / 413
	return c.JSON(201, posts.Create(in.Title, ""))
}
```

Errors are values: `trilha.ErrNotFound` → 404 (HTML or JSON depending on the route),
`trilha.Redirect(url)` → 303, `trilha.Errorf(422, "...")` → status with a message, any other
`error` → 500 with a stack only in dev. Methods you do not export answer 405 with `Allow`.

## Middleware

```go
// app/admin/middleware.go
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	if ck, err := c.Cookie("session"); err != nil || ck.Value != "ok" {
		return trilha.RedirectCode("/login", 302)
	}
	c.Set("user", "admin") // pages read it with c.Get("user")
	return next()
}
```

## UI

New projects ship with the `ui` kit: typed components (`ui.Button`, `ui.Card`, `ui.Field`,
`ui.Tabs`, `ui.Dialog`...) over a prefixed CSS and 200 lines of JS, both copied to `public/`
and yours to edit. The theme uses the same variables as [shadcn/ui](https://ui.shadcn.com)
(MIT): paste a ready-made theme into `public/ui.theme.css` and nothing in Go changes.
`trilha ui` updates the kit without touching your theme.

```go
ui.Card(
	ui.CardHeader(ui.CardTitle("New post")),
	ui.CardContent(h.Form(h.Method("post"), trilha.CSRFInput(c),
		ui.Field("title", "Title", ui.Input(h.ID("title"), h.Name("title"), h.Required())),
		ui.Submit(h.Text("Publish")))),
)
```

## Examples

| Level | Folder | Teaches |
|---|---|---|
| Basic | `examples/blog` | conventions, layouts, API, middleware, session |
| Medium | `examples/cadastro` | a form with rules: conditional fields, per-field validation (`c.Bind`, `trilha.FieldErrors`, `c.Render`), dependent select, disappearing toast |
| Complex | `examples/orcamento` | tree-shaped chart of accounts, drill-down, recursive components, dialog, CSV |
| SSO | `examples/sso` | OpenID Connect login (Entra ID/Keycloak), protected area, required role |
| AI | `examples/assistente` | streaming chat, agent with tools, MCP |

The example apps are written in Portuguese (identifiers and UI texts); the code is the same
Trilha documented here in English.

## AI and agents

`ai` speaks OpenAI's chat protocol (works with OpenAI, Groq, Mistral, OpenRouter, Ollama,
LM Studio, vLLM...), with typed tools, agents, handoffs and streaming; `ai/mcp` uses and
exposes tools through the Model Context Protocol. All without external dependencies.

```go
weather := ai.NewTool("weather", "Temperature in a city.",
    ai.Schema(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`),
    ai.Typed(func(ctx context.Context, in struct{ City string }) (string, error) {
        return fetchTemperature(ctx, in.City)
    }))
agent := &ai.Agent{Name: "Assistant", Instructions: "Answer briefly.", Tools: []*ai.Tool{weather}}
res, err := ai.Run(ctx, ai.NewFromEnv(), agent, "Is it cold in Curitiba?")
```

See `examples/assistente` (streaming chat with `c.Stream()`, handoff to a translator and an
MCP server at `/mcp`) and the chapter
[AI and agents](https://emersonjoe.github.io/trilha/learn/ai-and-agents).

## How it works

`trilha gen` scans `app/` with `go/ast` and writes `trilha_gen.go` (committed): a
`package main` that imports each route package and calls `a.Register(...)` with types
checked by the compiler. No `reflect`, no runtime magic; `go build .` works without the CLI.
The router is Go 1.22+'s `http.ServeMux`.

`trilha dev` listens on `:3000`, compiles the app on an internal port, proxies to it and
injects a live-reload script (SSE). On save: regenerate, recompile, swap the process and
notify the browser — about 1 s in the example. Changes only in `public/` do not recompile:
the browser reloads in tens of milliseconds. A compile error becomes a page with the output
of `go build` that goes away on its own when you fix it.

Secure by default: HTML escaping, `nosniff`/`X-Frame-Options`/`Referrer-Policy`, body limit
(1 MiB), CSRF by double-submit cookie on forms, static files without path traversal, `slog`
logs without body or cookies.

## Health and observability

Every app already answers `/_trilha/health/live` (the process is up) and
`/_trilha/health/ready` (the dependencies you registered with `a.Check` answer, with a
deadline and a cache). For anyone not authorized the response is only `{"status":"fail"}`:
dependency names and error messages stay in the log, not on the wire.

```go
a.Check("db", func(ctx context.Context) error { return db.PingContext(ctx) })
```

The metrics endpoint is opt-in (`TRILHA_METRICS` or `Observability.Metrics`) and requires a
token or a trusted network. It speaks the Prometheus text format, with requests, latency,
in-flight requests, security events, panics and the Go runtime — plus your own
(`a.Metrics().Counter(...)`). The route label is always the registered pattern, never the
concrete path. Details in
[Health and observability](https://emersonjoe.github.io/trilha/learn/observability).

## Corporate login

The `auth` package does OpenID Connect (Entra ID, Keycloak or any conforming provider) with
the standard library: PKCE, `state`, `nonce`, `id_token` validation with JWKS and key
rotation, session in a signed cookie and roles read from wherever each provider keeps them.

```go
// app/login/route.go — the whole flow is three two-line routes
func GET(c *trilha.Ctx) error { return sso.Start(c) }

// app/dashboard/middleware.go
func Middleware(c *trilha.Ctx, next trilha.Next) error { return sso.Require(c, next) }
```

An anonymous browser goes to the login; an API call gets 401. Someone logged in without the
required role gets 403. Runnable example in `examples/sso`, details in
[Authentication](https://emersonjoe.github.io/trilha/learn/authentication).

## Performance

`make bench` measures Trilha's cost over `net/http` + `html/template` (separate `bench/`
module). Summary on the reference machine: `h` renders the example page ~34 % faster than
`html/template`; the fixed cost per request (id, CSP nonce, headers, structured logging) is
~3 µs, and the metrics instrumentation, when on, adds no allocation. Methodology, numbers and a comparison of approach with other frameworks in
[Performance and comparison](https://emersonjoe.github.io/trilha/reference/performance).

## Where it is going

[`ROADMAP.md`](ROADMAP.md) (in Portuguese) answers an external review item by item: what
already exists, what is planned (issues labeled `roadmap`, grouped by phase) and what we
decided **not** to do, with the reason. The biggest acknowledged gap is interactivity without
turning into an SPA; OIDC authentication (spec 016) is already in.

## Out of scope (for now)

Client components/hydration and parallel routes. Client-side interactivity lives in
`public/*.js` (or htmx).

## License

MIT (`LICENSE`). The spec-kit files in `.specify/` and `.claude/skills` are MIT by GitHub,
Inc.; see `THIRD_PARTY_NOTICES.md`.

\* Next.js is a trademark of Vercel, Inc. Trilha is an independent project, not affiliated,
and contains no Next.js code.

Contributions are welcome: see [CONTRIBUTING.md](CONTRIBUTING.md), the
[code of conduct](CODE_OF_CONDUCT.md), the [security policy](SECURITY.md) and the
[governance](GOVERNANCE.md) (Portuguese translations in `docs/pt-BR/`). Behavior changes
follow the spec-kit flow in `specs/`.

## Development

```bash
make test        # gofmt + vet + go test ./... (includes the CLI e2e and the examples)
make dev-example # trilha dev in examples/blog
make reload      # measures the edit→see cycle
```

Spec-kit driven project: see `specs/` (one folder per spec, from 001 core to 015 i18n) and
`.specify/memory/constitution.md`. Specs and the constitution are written in Brazilian
Portuguese; everything public (site, README, CLI) is English by default with a Portuguese
translation.
