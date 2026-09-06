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
| `Fragment() string` | id the client wants to swap (`Trilha-Fragment` header), or `""` on a normal navigation ([Interactivity](/learn/interactivity)) |

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
| `Render(code, node) error` | writes the page **with the route's layouts** (like GET): for a `POST` to return the form with errors (422); on a fragment, without the layouts |
| `Stream() *Stream` | Server-Sent Events response: `Send(event, data)`, `JSON(event, v)`, `Comment(s)`, `Done()`; disables the *write timeout* ([AI and agents](/learn/ai-and-agents)) |
| `Writer() http.ResponseWriter` | direct access (long downloads, WebSocket) |
| `Written() bool` | whether the response has started |

## HTTP cache

| Method | Description |
|---|---|
| `ETag(tag) bool` | writes `ETag` (quoting it if needed) and reports whether the request already had it |
| `LastModified(t) bool` | writes `Last-Modified` and reports whether the copy is current |
| `CacheControl(v)` | writes `Cache-Control` verbatim |

`true` means the `304` is already written: return `nil, nil` and write nothing else. Only `GET` and
`HEAD` answer `304`; on other methods the headers are written and the answer is always `false`. An
empty tag or a zero date writes nothing. When both are declared, `If-None-Match` decides and the
date stays as metadata, as RFC 9110 asks. Files under `static/` already carry an ETag: the content
fingerprint that goes in `?v=`.

## Between page and layout

| Method | Description |
|---|---|
| `SetTitle(s)` / `Title() string` | page title, read by layouts |
| `Set(key, v)` / `Get(key) any` | per-request values (middleware → page → layout) |

## Islands

```go
func (c *Ctx) Island(src string, props any, children ...h.Node) h.Node
```

Renders `<div data-trilha-island="…" data-trilha-props="…">` with the children as the
server-rendered fallback. `src` is a module in `public/` (addressed through `Asset`, so it
carries the content hash) whose **default export** is the mount function, called once with
`(el, props)`. `props` is anything `encoding/json` serializes, or `nil`; it travels as an
escaped attribute and is read back with `JSON.parse`, so it is data and never markup. Props
that do not serialize warn once and leave the fallback alone. The loader is a single inline
script with the request nonce, emitted with the first island of the response
([Interactivity](/learn/interactivity)).

## Long connections and large bodies

| Method | Description |
|---|---|
| `AllowBody(n int64)` | body limit for **this** request, in place of `Config.MaxBodyBytes` |
| `NoReadDeadline() error` | drops this request's read deadline (a slow upload is not an error) |
| `NoWriteDeadline() error` | drops the write deadline (long download, SSE) |
| `Hijack() (net.Conn, *bufio.ReadWriter, error)` | takes the connection over: deadlines cleared, and Trilha writes nothing more on it |

The default limit belongs to the app; the exception belongs to the route. Raise it in the
route's `middleware.go`, not in the handler — form CSRF reads the body before the handler
runs, so the decision has to come first:

```go
// app/anexos/middleware.go
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	if c.Request().Method == "POST" {
		c.AllowBody(8 << 20) // this request only; every other route keeps the app's limit
		c.NoReadDeadline()
	}
	return next()
}
```

Going over the limit is still a 413 with the usual message, through `FormErr`, `Bind*` or a
direct read of `Request().Body`.

### WebSocket

Trilha has no WebSocket of its own, and that is a decision. The protocol is transport: it
touches no route, no layout and no render. What it does need — fragmentation and
continuation frames, control frames interleaved with a message, the close handshake with a
deadline, UTF-8 validation, masking, size limits, concurrent writes, backpressure,
`permessage-deflate` — is a few hundred lines that the Autobahn suite tests in 500+ cases.
The asymmetry decides it: your app can add `coder/websocket` to **its** go.mod (principle II
binds the framework, not the app), but it cannot take those lines out of the framework.

What was missing was the door, and `Hijack` is it:

```go
func WS(c *trilha.Ctx) error {
	conn, _, err := c.Hijack() // read and write deadlines already cleared
	if err != nil {
		return err
	}
	defer conn.Close()
	return meuWebsocket.Serve(conn) // coder/websocket, gorilla, whatever you picked
}
```

After `Hijack` the connection is yours: the framework writes no header, no error page and no
body on it, and the access log records 101.

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
adjustable) after every field has been tried. The `validate:"..."` tag of each field is
applied right after, in the same pass: see [Validation](/reference/validation).
