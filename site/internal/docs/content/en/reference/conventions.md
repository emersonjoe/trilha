---
title: File conventions
description: Complete table of what each file and folder name in app/ means.
---

## Files

| File | Exported function | Signature | Scope |
|---|---|---|---|
| `page.go` | `Page` | `func(c *trilha.Ctx) (h.Node, error)` | the folder's GET route |
| `page.go` | `POST`, `PUT`, `PATCH`, `DELETE` (optional) | `func(c *trilha.Ctx) error` | forms; CSRF required |
| `route.go` | `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `OPTIONS` (at least one) | `func(c *trilha.Ctx) error` | the folder's JSON API |
| `kind.go`, or any file (optional) | `Kind` | `var Kind = trilha.KindPage` or `KindAPI` | subtree: how errors are rendered and whether CSRF applies (see [Errors](/reference/errors)) |
| `route.go` (optional) | `CORS` | `var CORS = trilha.CORS{...}` | cross-origin policy of this route alone, preflight included |
| `layout.go` | `Layout` | `func(c *trilha.Ctx, children h.Node) (h.Node, error)` | subtree |
| `middleware.go` | `Middleware` | `func(c *trilha.Ctx, next trilha.Next) error` | subtree |
| `middleware.go` (optional) | `MiddlewareGET`, `MiddlewarePOST`, `MiddlewarePUT`, `MiddlewarePATCH`, `MiddlewareDELETE`, `MiddlewareOPTIONS` | `func(c *trilha.Ctx, next trilha.Next) error` | subtree, that method only |
| `not_found.go` (root only) | `NotFound` | `func(c *trilha.Ctx) (h.Node, error)` | the app's 404 |
| `error.go` (root only) | `Error` | `func(c *trilha.Ctx, err error) (h.Node, error)` | every error status but 404 |
| `setup.go` (root only) | `Setup` | `func(a *trilha.App) error` | before serving |
| `setup.go` (optional) | `Config` | `func(cfg *trilha.Config)` or `func(cfg *trilha.Config) error` | before `trilha.New`; an error stops the boot |
| `setup.go` (optional) | `Shutdown` | `func(a *trilha.App) error` | after the app stops accepting requests (close pool, queue, flush logs) |

`page.go` and `route.go` in the same folder is an error. The function may live in any file of
the package; the file name is what binds the convention.

### Kind follows the subtree

`Kind` is a variable, not a function, and it is inherited like `Layout` and `Middleware`:
declared in the package of a folder, it decides that folder and everything below it, and the
deepest declaration wins. `kind.go` is the file name for a folder that has no `route.go` of
its own — a subtree root has to be able to speak without owning a route:

```go
// app/painel/kind.go — this branch is browser pages, so its writes enforce CSRF
package painel

var Kind = trilha.KindPage
```

This matters more than error rendering: **`Kind` is what turns CSRF on**. A `route.go` is an
API by default, and an API does not check the token, so the same form action moved from a
`page.go` into a `route.go` starts accepting a POST from another site — silently. One line at
the root of the branch covers every leaf, including the leaf someone adds next month.
`trilha audit` reports a write route that no `Kind` reaches in an app that also serves pages.

A `page.go` route is a page whatever the branch above it says: an inherited `KindAPI` never
turns a page into JSON.

## Folders

| Name | Becomes | Example |
|---|---|---|
| `events` | literal segment | `/events` |
| `slug_` | parameter `{slug}` | `/events/{slug}` → `c.Param("slug")` |
| `path__` | catch-all `{path...}`; must be a leaf | `/docs/{path...}` |
| `organizer-` | route group; not part of the URL | layout/middleware for the subtree |
| `app.css`, `robots.txt` | fixed path with an extension (dot in the middle of the name) | `/app.css`, `/manifest.webmanifest`, `/sw.js` |
| `.well-known` | the one dot folder that is a route | `/.well-known/security.txt` |
| `_x`, `.x`, `testdata` | ignored | — |

A folder with a dot in its name serves a fixed path with an extension. Since `app.css` is
not a Go identifier, declare another package name in the file (`package appcss`); the
generator imports everything with an alias, so the package name does not matter.

Folders that **start** with a dot stay ignored, with a single exception: `.well-known`,
where RFC 8414, RFC 9728, RFC 8555, RFC 9116 and OpenID Discovery publish their documents.
Inside it the conventions are the usual ones — `app/.well-known/security.txt/route.go`
answers `/.well-known/security.txt`. A `page.go` or `route.go` inside any *other* dot folder
is now an `E_HIDDEN_ROUTE` error instead of a 404 nobody can explain; to park a folder out
of the routing on purpose, start its name with `_`.

The Go tool does not match a path with a dot in `./...`, so `go vet ./...` and `go test
./...` skip that package as a target. It still compiles: `trilha_gen.go` imports it by its
explicit path.

## Cross-origin on one route

`Config.CORS` is the policy of the whole app. When only a few paths are public — the
discovery documents under `/.well-known/`, fetched from another origin by a client that has
no session yet — the route carries its own:

```go
package oauthresource

// Only this route. The other routes stay same-origin.
var CORS = trilha.CORS{Origins: []string{"*"}, Methods: []string{"GET"}}

func GET(c *trilha.Ctx) error { ... }
```

The framework answers the preflight from the policy (204 with `Access-Control-Allow-*`, or
403 for an origin or method that is not on the list) and adds the headers to every response
of that route. A route that declares its own policy decides alone: the app-wide list neither
widens nor narrows it. Writing `func OPTIONS` in the same file takes the preflight back —
the common case is declarative, the odd one is still yours.

`HEAD` is not a handler name: since Go 1.22 the router answers HEAD with the `GET` handler.

Precedence: literal beats parameter, which beats catch-all. Two sibling dynamic folders are
an error. Two folders producing the same URL (through groups) are an error.

## Other project folders

| Folder | Role |
|---|---|
| `public/` | static files served at the root; embedded in the binary in production |
| `trilha_gen.go` | generated; committed; never edited by hand; carries the package the folder declares (see [CLI](/reference/cli)) |
| `.trilha/` | temporary binaries of `dev` and `export`; ignored by git |

## Execution order for `GET /a/b`

```text
middleware(app) → middleware(app/a) → middleware(app/a/b)
  → middlewareGET(app) → middlewareGET(app/a) → middlewareGET(app/a/b)
  → Page (or method)
  → layout(app/a/b) → layout(app/a) → layout(app)
```

The per-method chain runs inside the route-wide one: a rule for a single method refines what
the route already decided. For `POST` it is `MiddlewarePOST`, and so on; a method with no
chain of its own just runs the route's.

## Generation errors

| Code | Cause |
|---|---|
| `E_PAGE_AND_ROUTE` | `page.go` and `route.go` in the same folder |
| `E_NO_PAGE_FUNC` | `page.go` without `Page` |
| `E_NO_METHOD` | `route.go` without an exported method |
| `E_NO_LAYOUT_FUNC`, `E_NO_MIDDLEWARE_FUNC`, `E_NO_NOT_FOUND_FUNC`, `E_NO_ERROR_FUNC`, `E_NO_SETUP_FUNC` | file without the expected function |
| `E_AMBIGUOUS_SEGMENT` | two dynamic folders at the same level |
| `E_CATCHALL_NOT_LEAF` | routes below an `x__` folder |
| `E_BAD_SEGMENT` | invalid parameter name or dynamic group (`x_-`) |
| `E_DUPLICATE_ROUTE` | two folders producing the same URL |
| `E_UNUSED_METHOD_MIDDLEWARE` | `MiddlewareX` that reaches no route serving `X` |
| `E_PARSE` | Go file that does not compile |
| `E_NO_APP` | there is no `app/` folder |
| `E_HIDDEN_ROUTE` | `page.go` or `route.go` inside a folder whose name starts with a dot |
| `E_UNROUTABLE_METHOD` | `func HEAD`, `TRACE` or `CONNECT`: the router does not take those from a file |
| `E_CORS_ON_PAGE` | `var CORS` in a `page.go` |
