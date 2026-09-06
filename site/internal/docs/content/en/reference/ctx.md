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
| `Pattern() string` | the template of the route that matched (`/blog/{slug}`), the aggregatable form of the path; `""` for what the fallback answered (static file, 404, trailing-slash redirect) |
| `Query(name) string` | first value of the query parameter |
| `Form(name) string` | form field (parses on demand, with a size limit) |
| `FormErr() error` | form parse error: 400 invalid, 413 too large |
| `BindJSON(&v) error` | decodes the JSON body; unknown fields are an error (400); 413 above the limit |
| `Cookie(name) (*http.Cookie, error)` | request cookie |
| `Accepts(offers...) string` | the offer the client prefers (`Accept`, ranked by `q`), or `""`; an absent or `*/*` header picks the first offer |
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
| `Flash(kind, text)` | queues a message for the next request, in a signed cookie: the news the redirect would eat. `ui.Flashes(c)` shows it. On a fragment answer it travels in the `Trilha-Flash` header instead, and `ui.js` shows it. Without `TRILHA_SECRET` nothing is written and the app says so once in the log |
| `Flashes() []Flash` | the messages left by the previous request plus the ones this one has not sent yet; reading them takes them, and reading twice gives the same list |
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
| `trilha.CSRFTokenFrom(r) string` | the same token, for a renderer that only receives the `*http.Request` (`html/template`, `templ`, a handler of your own); `""` outside a Trilha request |
| `trilha.NonceFrom(r) string` | the CSP nonce of the request, same reason and same rule ([Security](/reference/security)) |

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

## File

`File(field string, rules FileRules) (*Upload, error)` reads one file from a multipart form
and only answers with it if it passes the rules.

| Symbol | Role |
|---|---|
| `FileRules.MaxSize int64` | limit for this file, apart from `Config.MaxBodyBytes`; 0 leaves the body limit doing the work |
| `FileRules.Accept []string` | media types allowed, matched against the **detected** type: `"image/png"`, `"image/*"`, `"*/*"`; empty accepts anything |
| `FileRules.Optional bool` | an absent field returns `(nil, nil)` instead of an error |
| `FileRules.MaxFiles int` | ceiling on how many files `Files` accepts in one request; 0 is no ceiling |
| `Upload.Name` | sanitised name: no directory, no separator, no control character, at most 100 characters, never empty |
| `Upload.MIME` / `Upload.Ext` | type detected in the first 512 bytes, and the extension that matches it |
| `Upload.Size` / `Upload.File` | size in bytes, and the file itself positioned at the start |
| `up.Save(dir) (string, error)` | writes inside `dir` (mode 0600) under a free name and returns the path |
| `up.Close() error` | closes the file |

A rule that fails is `FieldErrors` under the field's name, like `Bind`; anything else (a
broken body, a full disk) comes back as itself. Messages come from `ValidationMessages`
(`required`, `filemax`, `filetype`, `filecount`) — see [Validation](/reference/validation).

`Files(field string, rules FileRules) ([]*Upload, error)` reads every file the field carries,
in the order the browser sent them, applying the same rules to each. An empty file input is
not a file: it is dropped before the count, so `Optional` still means "nobody chose anything".
A file that fails names its own position — `files[2]`, not `files` — so a form with one line
per file can put the message on the right line; the files that passed come back in the slice,
and closing them is the caller's job. Over `MaxFiles` the whole request is refused under the
field's own name, before a single byte is read.

## Sending a file

`File` receives; these five send. The name, the type and the two headers that keep a download
from becoming a page of your origin are the whole of it.

| Symbol | Role |
|---|---|
| `Attachment(name string, body io.Reader, ctype string) error` | download: `Content-Disposition: attachment` |
| `Inline(name string, body io.Reader, ctype string) error` | the browser opens it in place (a PDF in an `<iframe>`, an image) |
| `AttachmentFile(path, ctype string) error` | opens the file, sends it and closes it; absent is 404, a directory is an error |
| `InlineFile(path, ctype string) error` | the same, opened in place |
| `Pipe(res *http.Response) error` | hands another service's answer to the browser and closes its body |

The name is sanitised by the same function an upload's is: a path never becomes a filename,
and nothing in it can add a second line to the header. It goes out twice — `filename*=UTF-8''`
percent-encoded for a browser that reads RFC 5987, and a quoted ASCII `filename` for one that
does not — because a name with an accent breaks in half the browsers when it goes out once.

An empty `ctype` is detected from the first 512 bytes, the way `File` sniffs an upload, with
the extension allowed to sharpen it inside the same family (`text/plain` to `text/csv`) and
never to overrule it. `X-Content-Type-Options: nosniff` always goes out: a download the browser
is free to re-interpret is a download that can become a page of this origin.

`Inline` only accepts what a viewer renders — `application/pdf`, `image/*` (not SVG),
`audio/*`, `video/*`, `text/plain`, `text/csv`. HTML, SVG and XML come back as a programming
error, not as a response: they are documents with script, served from your own origin. The
`<iframe>` also needs the app to say so, because the default policy is `frame-ancestors 'none'`:

```go
cfg.Security.CSPExtra = map[string][]string{"frame-src": {"'self'"}}
```

`Inline` does not loosen that from below.

A `body` that is an `io.ReadSeeker` — a `bytes.Reader`, an `os.File` — is written by
`http.ServeContent`, so `Range`, `If-Range`, `304` and `HEAD` come for free and
`Accept-Ranges: bytes` is promised. Anything else is copied as a stream and promises nothing.
Every send clears the write deadline: a 50 MB file on a bad link is not a slow handler.

`Pipe` copies the status and a closed list of headers — `Content-Type`,
`Content-Disposition`, `Content-Length`, `Content-Encoding`, `Content-Range`, `Accept-Ranges`,
`Cache-Control`, `ETag`, `Last-Modified`, `Expires`, `Vary`, `Age` — and nothing else.
`Set-Cookie` in particular does not travel: the body of another service does not get to sit on
this one's session. For a whole prefix forwarded to another service, see
[Upstreams](/reference/upstreams); `Pipe` is the one response you fetched yourself.
