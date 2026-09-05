---
title: Ctx
description: Everything a route function can do with the request context.
---

`*trilha.Ctx` wraps the request and the response. It is created per request and must not be
used by another goroutine after the handler returns.

## Request

| Method | Description |
|---|---|
| `Request() *http.Request` | the original request |
| `SetContext(ctx)` | replaces the request context: a middleware passes values to code that only receives `*http.Request` |
| `SetRequest(*http.Request)` | replaces the request (rewritten URL, wrapped body) |
| `Context() context.Context` | request context (cancellation) |
| `Param(name) string` | route parameter (`slug_` → `"slug"`) |
| `Query(name) string` | first value of the query parameter |
| `Form(name) string` | form field (parses on demand, with a size limit) |
| `FormErr() error` | form parse error: 400 invalid, 413 too large |
| `BindJSON(&v) error` | decodes the JSON body; unknown fields are an error (400); 413 above the limit |
| `Cookie(name) (*http.Cookie, error)` | request cookie |
| `RequestID() string` | received `X-Request-ID` or a generated id |
| `Env() trilha.Env` | `trilha.Dev` or `trilha.Prod` |
| `Base() string` | URL prefix (`TRILHA_BASE_PATH`), without trailing slash |
| `App() *trilha.App` | the application |

## Response

| Method | Description |
|---|---|
| `JSON(code, v) error` | writes JSON with the right `Content-Type` |
| `Text(code, s) error` | writes plain text |
| `HTML(code, node) error` | writes a node as a whole document, without layouts |
| `Redirect(url) error` | returns the 303 redirect error (use with `return`) |
| `Status(code)` | status the next page render will use |
| `Header(k, v)` | sets a response header |
| `SetCookie(*http.Cookie)` | adds `Set-Cookie` |
| `Render(code, node) error` | writes the page **with the route's layouts** (like GET): for a `POST` to return the form with errors (422) |
| `Stream() *Stream` | Server-Sent Events response: `Send(event, data)`, `JSON(event, v)`, `Comment(s)`, `Done()`; disables the *write timeout* ([AI and agents](/learn/ai-and-agents)) |
| `Writer() http.ResponseWriter` | direct access (long downloads, WebSocket) |
| `Written() bool` | whether the response has started |

## Between page and layout

| Method | Description |
|---|---|
| `SetTitle(s)` / `Title() string` | page title, read by layouts |
| `Set(key, v)` / `Get(key) any` | per-request values (middleware → page → layout) |

## Security

| Method | Description |
|---|---|
| `CSRFToken() string` | the request's token; creates the cookie on the first call |
| `trilha.CSRFInput(c) h.Node` | `<input type="hidden" name="_csrf">` for forms |

The token is verified automatically on `POST`, `PUT`, `PATCH` and `DELETE` of `page.go` (and
of `route.go` if `Config.CSRFForAPI` is on), through the `_csrf` field or the
`X-CSRF-Token` header.

## Bind

`Bind(v any) error` fills a struct from the form (or from JSON, when the `Content-Type` is
`application/json`). Fields match by the `form:"name"` tag (or by the field name); types:
`string`, `[]string`, `bool` (`on`/`true`/`1`), `int`, `int64`, `float64` (comma or dot),
`time.Time` (`2006-01-02` or `2006-01-02T15:04`) and pointers (nil when absent). A nested
struct is flattened, with the tag as prefix (`Billing Address `+"`form:\"bill_\"`"+` reads
`bill_zip`…). Values that do not convert become `FieldErrors` (message `trilha.BindInvalid`,
adjustable) after every field has been tried.
