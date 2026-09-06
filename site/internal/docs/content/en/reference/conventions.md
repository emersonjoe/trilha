---
title: File conventions
description: Complete table of what each file and folder name in app/ means.
---

## Files

| File | Exported function | Signature | Scope |
|---|---|---|---|
| `page.go` | `Page` | `func(c *trilha.Ctx) (h.Node, error)` | the folder's GET route |
| `page.go` | `POST`, `PUT`, `PATCH`, `DELETE` (optional) | `func(c *trilha.Ctx) error` | forms; CSRF required |
| `route.go` | `GET`, `POST`, `PUT`, `PATCH`, `DELETE` (at least one) | `func(c *trilha.Ctx) error` | the folder's JSON API |
| `route.go` (optional) | `Kind` | `var Kind = trilha.KindPage` or `KindAPI` | how errors are rendered and whether CSRF applies (see [Errors](/reference/errors)) |
| `layout.go` | `Layout` | `func(c *trilha.Ctx, children h.Node) (h.Node, error)` | subtree |
| `middleware.go` | `Middleware` | `func(c *trilha.Ctx, next trilha.Next) error` | subtree |
| `middleware.go` (optional) | `MiddlewareGET`, `MiddlewarePOST`, `MiddlewarePUT`, `MiddlewarePATCH`, `MiddlewareDELETE` | `func(c *trilha.Ctx, next trilha.Next) error` | subtree, that method only |
| `not_found.go` (root only) | `NotFound` | `func(c *trilha.Ctx) (h.Node, error)` | the app's 404 |
| `error.go` (root only) | `Error` | `func(c *trilha.Ctx, err error) (h.Node, error)` | every error status but 404 |
| `setup.go` (root only) | `Setup` | `func(a *trilha.App) error` | before serving |
| `setup.go` (optional) | `Config` | `func(cfg *trilha.Config)` or `func(cfg *trilha.Config) error` | before `trilha.New`; an error stops the boot |
| `setup.go` (optional) | `Shutdown` | `func(a *trilha.App) error` | after the app stops accepting requests (close pool, queue, flush logs) |

`page.go` and `route.go` in the same folder is an error. The function may live in any file of
the package; the file name is what binds the convention.

## Folders

| Name | Becomes | Example |
|---|---|---|
| `events` | literal segment | `/events` |
| `slug_` | parameter `{slug}` | `/events/{slug}` → `c.Param("slug")` |
| `path__` | catch-all `{path...}`; must be a leaf | `/docs/{path...}` |
| `organizer-` | route group; not part of the URL | layout/middleware for the subtree |
| `app.css`, `robots.txt` | fixed path with an extension (dot in the middle of the name) | `/app.css`, `/manifest.webmanifest`, `/sw.js` |
| `_x`, `.x`, `testdata` | ignored | — |

A folder with a dot in its name serves a fixed path with an extension. Since `app.css` is
not a Go identifier, declare another package name in the file (`package appcss`); the
generator imports everything with an alias, so the package name does not matter. Folders
that **start** with a dot stay ignored.

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
