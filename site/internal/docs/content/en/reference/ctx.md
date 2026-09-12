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
| `Stream() *Stream` | Server-Sent Events response: `Send(event, data)`, `JSON(event, v)`, `Comment(s)`, `Flush()`, `Done()`; disables the *write timeout* ([AI and agents](/learn/ai-and-agents)) |
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
`(el, props, island)`. `props` is anything `encoding/json` serializes, or `nil`; it travels as an
escaped attribute and is read back with `JSON.parse`, so it is data and never markup. Props
that do not serialize warn once and leave the fallback alone. The loader is a single inline
script with the request nonce, emitted with the first island of the response
([Interactivity](/learn/interactivity)).

### The way back: the `island` object

The third argument is the channel to the server. It exists so an island does not have to
rediscover the token, the headers and the error shape the rest of the framework already
agreed on:

| Member | What it does |
|---|---|
| `island.csrf()` | the token of this response — the same one `CSRFInput` puts in every form |
| `island.signal` | an `AbortSignal`, aborted when the element leaves the page |
| `island.get(url)` | reads JSON |
| `island.post(url, data)` | sends JSON with the token on it, returns what the route answered |
| `island.send(method, url, data)` | the same, for `PUT`, `PATCH` and `DELETE` |
| `island.swap(url, id)` | replaces a fragment, the way a link with a target does |

A route that answers `422` comes back as an `IslandInvalid` whose `.fields` is the same
object a form would have shown; any other failure is an `IslandError` with `.status` and
`.detail`. A `Trilha-Location` on the response is followed as a navigation, so
POST → redirect → GET works from an island too.

The double-submit cookie is `HttpOnly`, so the token reaches the island written into the
element, as `data-trilha-csrf`. That is what `island.post` sends as `X-CSRF-Token`.

A `route.go` is an API, and an API does not check the token — its client carries a bearer
token, not a cookie. An island is the exception, because its client is the page:

```go
// app/blog/novo/rascunho/middleware.go
func MiddlewarePOST(c *trilha.Ctx, next trilha.Next) error {
	return trilha.RequireCSRF(c, next)
}
```

The route stays an API, so its errors stay problem+json — which is what the island can read.

### The types the module sees

The script the page loads for all of this is `trilha.IslandRuntime` (`/ui.island.js`), written
into `public/` by `trilha ui` along with the rest of the kit; `trilha check` says so when a
project mounts an island without it.

`trilha gen` writes `public/islands.d.ts` from the `c.Island` calls it finds in `app/`: one
interface per props struct, plus the `island` object and the mount signature. Point the
module at it and an editor checks both sides of the boundary:

```js
/** @type {import("/islands.d.ts").IslandMount<"/editor.js">} */
export default function (el, props, island) { … }
```

Props given as a map literal or a variable have no name to hang a type on: the island is
still declared, typed `unknown`, and `trilha gen` says so. `gen --check` compares the file
like it compares `trilha_gen.go`, and an app whose last island is gone loses the file.

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
`application/json`). Fields match by the `form:"name"` tag, then by `json:"name"`, then by the field name — a struct
that came from an API carries json tags and no form tags, and a form that posted `nome` to a field
the binder was calling `Nome` made `required` fire on a value somebody did type; types:
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

## Drafts

`Draft(name string) *Draft` is a form in progress, kept between one request and the next. It is
the answer to the only hard question a multi-step form asks: where does step one live while
somebody is on step two.

| Symbol | Role |
|---|---|
| `c.Draft(name)` | names the form — not the person; the draft travels in their own cookie |
| `d.Save(v any, ttl time.Duration) error` | writes the draft and starts its clock |
| `d.Load(v any) error` | fills v, or `ErrNoDraft` — never saved, finished, expired, tampered with, or another browser |
| `d.Clear()` | the draft became a record and stops existing |
| `Config.Drafts DraftStore` | where a draft over 2 KB of JSON goes; nil means the cookie is all there is |

Under 2 KB of JSON the draft is a **signed cookie**: nothing to configure, nothing to clean up,
and it expires on its own. Above that it needs `Config.Drafts` — three methods over whatever the
app already runs — and without one, `Save` returns an error naming that field rather than setting
a cookie the browser would drop without a word. (The limit is 2 KB and not the 3 KB the issue
proposed, because what goes in the cookie is base64 of the draft plus an expiry and a signature,
and 3 KB of JSON crosses the browser's 4 KB.)

`ErrNoDraft` is an answer, not a failure: it is what sends somebody back to step one. A draft
written by an older version of the struct answers the same way — the field was renamed between
deploys, and starting over beats a 500 in the middle of somebody's form.

A draft is signed, so it cannot be edited by hand, and it is **not secret**: what is in a cookie
travels to the browser and can be read there. Keep a price or somebody else's name behind
`Config.Drafts`, with only the key in the cookie. Signing needs `TRILHA_SECRET`; without it
`Save` says so instead of failing quietly.

`ui.Steps` draws the indicator — see [A form in steps](/cookbook/wizard) for the whole flow.

## Spreadsheets

`CSV(name string, rows any) error` writes a slice — or a receive-only channel — as a file the
person can open, and `BindCSV(r io.Reader, dst any, rules ...CSVRules) (CSVResult, error)`
reads one back saying which cell is wrong.

| Symbol | Role |
|---|---|
| `c.CSV(name string, rows any) error` | download with a UTF-8 BOM, the locale's separator and CRLF lines |
| `csv:"Heading"` | the column's heading; without a tag it is the field name, and `csv:"-"` leaves the field out |
| `BindCSV(r io.Reader, dst any, rules ...CSVRules)` | reads the file into a `*[]T`, validating each row with its `validate` tags |
| `CSVRules.MaxRows int` | ceiling on the file; default 100,000, and above it an error rather than a truncation |
| `CSVRules.Separator rune` | forces the delimiter; zero detects it from the header |
| `CSVResult.Rows int` | how many data lines were read, blank ones aside |
| `CSVResult.Errors []CSVError` | `{Line, Column, Message}` in file order; the header is line 1 |
| `CSVResult.Warnings []string` | what was odd and not fatal — a heading no field claims |
| `res.OK() bool` | no errors: every row can be used |

`Config.Locale` decides the separator (`,` in en, `;` in pt-BR), the date (`2006-01-02 15:04`
against `02/01/2006 15:04`), the decimal mark and the word for a boolean (`yes`/`no`,
`sim`/`não`); `Config.TimeZone` decides which day a timestamp falls on. The BOM is not
optional and not configurable: without it Excel reads every accent as mojibake, and that is
the first thing anybody notices.

A channel is what a two-hundred-thousand-row export wants — rows are written as they arrive
and the write deadline is cleared — but the producer is yours to end: select on
`c.Context().Done()`, or a browser that closed the connection leaves a goroutine behind.
(`iter.Seq` is not accepted: this module builds on Go 1.22.)

On the way in, the separator and the BOM are detected, the header matches by tag in any order
ignoring case and surrounding space, a blank line is skipped, and a date is read as
`dd/mm/yyyy` as well as ISO. Only rows that pass are appended, so after `res.OK()` the slice
is the whole file. A required column missing from the header is one message at line 1 rather
than the same message on every row; a heading no field claims is a warning, because a
spreadsheet grows a column all the time. `ui.CSVErrors` renders the list — see
[CSV](/cookbook/csv) for the whole round trip.

Together with `ui.Flashes(c)` — the summary line, and the table of what to fix underneath — this
is the answer an import screen gives without a person opening the spreadsheet to find one bad
date by eye.

@demo ui-avisos-csv

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
error, not as a response: they are documents with script, served from your own origin.
`trilha.CanInline(ctype)` answers the same question, so a screen can offer the "view" link only
where there is something to view.

**The response says it may be framed by a page of this origin**, and that is the half everybody
gets wrong. The default hardening sends `X-Frame-Options: DENY` and `frame-ancestors 'none'` on
every answer; a document carrying those cannot be shown in place, whatever the page around it
declares. Adding `frame-src` to the framing page changes nothing — the browser is refusing on
behalf of the framed answer, and it says so in the console. `Inline` relaxes those two headers
on that one response, and leaves an app that wrote its own `Security.CSP` exactly as it is.

Drawing it is [`ui.Preview`](/reference/ui): the bar, the frame, the fallback for a type nobody
can show, and an `<img>` instead of a frame for an image.

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

## Public links

`c.Link(name, opts)` builds a signed URL that works with no session, and `c.Claim(name)` is what
the route on the other side calls.

| Symbol | Role |
|---|---|
| `c.Link(name, LinkOpts{...})` | the URL; `name` is the purpose, and a link for one purpose does not open another |
| `LinkOpts.Data` | travels inside the token: **signed, not secret** |
| `LinkOpts.TTL` | zero is an hour; negative is an error, not a default |
| `LinkOpts.Uses` | zero is unlimited and needs no storage at all |
| `c.Claim(name)` | checks signature, purpose, deadline and remaining uses |
| `link.Consume()` | spends one use — after the work, never before |
| `Config.Links` | a `trilha.LinkStore` that counts the uses of limited links — one `INCR` compared against the limit, which is what makes Redis the natural one; nil counts in the process, which is honest about one replica and said once in the log |
| `trilha.ErrNoLink` | what `Claim` answers for a token that is invalid, not this link's, expired or used up — one error for all four on purpose |

**Every way a link can fail answers the same 404.** Wrong signature, wrong purpose, expired,
already spent: telling a stranger which of the four happened tells them how close they are. A
wrong token also costs the address a point of a small budget, because guessing a token in a URL
is brute force.

That budget is a **refilling one and not a lockout**, which is a deliberate difference from the
issue that asked for this: an hour of blocking keyed by address turns one clumsy person behind an
office NAT into an outage for everybody behind it, and the property that matters — guessing
becomes infeasible — is the same either way.

**With `Uses: 0` there is no state.** No row, no lookup, no cleanup: verifying is a signature
check. That is the case of a verification code printed on a document, and it is why the common
flow needs no table.

**Reading is not spending.** `Claim` checks that a use is left; only `Consume` takes one. A
reload would otherwise burn the link somebody is still filling in, and a validation error would
cost them the invitation.

See [A public link](/cookbook/public-link) for the whole flow.
