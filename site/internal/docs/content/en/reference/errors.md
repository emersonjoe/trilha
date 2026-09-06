---
title: Errors
description: The error values Trilha understands and how each one becomes a response.
---

Handlers return `error`. Trilha translates:

| Value | Page (`page.go`) | API (`route.go`) |
|---|---|---|
| `nil` | response written by the handler; 204 if nothing was written | same |
| `trilha.ErrNotFound` (or an error wrapping it) | 404 with `not_found.go` | 404 `problem+json`, `"title":"Not Found"` |
| `*trilha.RedirectError` via `trilha.Redirect(url)` (303) or `trilha.RedirectCode(url, code)` | redirect | redirect |
| `*trilha.HTTPError` via `trilha.Errorf(code, fmt, a...)` | simple page with the status and the message (4xx) | the status, with the message in `detail` (4xx) |
| any other `error` | 500 with `error.go`; details only in dev | 500, `detail` only in dev |
| `*trilha.Problem` | simple page with the status and `Detail` | the problem, as it was written |
| `panic` in the handler | recovered and handled as 500; stack only in dev | same |

### Page or problem+json?

The column is decided per route; the `Accept` header is the tie-breaker, ranked by `q`:

- `page.go` → always a page. A fragment swapped into the page needs HTML even when the
  `fetch` says otherwise.
- `route.go` → `problem+json`, **except** when `Accept` prefers `text/html` over
  `application/json` — a browser in the address bar. The path plays no part: a `route.go`
  under `/api/` shows the error page to a browser just like any other.
- An absent `Accept`, or `*/*` (`fetch`, `curl`), is not a preference: the kind of the route
  decides.
- `route.go` can pin the behaviour by exporting `var Kind = trilha.KindPage` (always a page,
  and CSRF required on `POST`/`PUT`/`PATCH`/`DELETE`) or `trilha.KindAPI` (always
  `problem+json`, whatever `Accept` says).
- With no route at all (404), there is no kind to ask: `Accept` decides, and when it is
  silent the `/api/` prefix is the last resort.

### Answering on your own

`not_found.go`, `error.go` and `page.go` may write the whole response and return
`(nil, nil)`: Trilha adds nothing on top. It serves a plain-text 404
(`http.NotFound(c.Writer(), c.Request())`), another `Content-Type` or another status. If the
function returns `nil` **without** writing, the framework's simple page applies (404/500);
in `page.go`, 204.

`HTTPError` messages with a 5xx code are never shown to the client. Every 5xx error goes to
the log with the `request_id`.

```go
if ev, ok := events.Find(slug); !ok {
	return trilha.ErrNotFound
}
if seats < 0 {
	return trilha.Errorf(422, "seats cannot be negative")
}
return c.Redirect("/events/" + ev.Slug)
```

Errors from `c.BindJSON` and `c.FormErr` are already `HTTPError` (400 or 413): just return
them.

## Problem

API errors are [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) problem details, sent as
`application/problem+json`:

```json
{"type":"about:blank","title":"Unprocessable Entity","status":422,
 "instance":"/api/posts","request_id":"01J…","fields":{"title":"required"}}
```

Return a `*trilha.Problem` to say more than a status:

```go
return &trilha.Problem{
	Type:   "https://example.com/probs/out-of-credit",
	Title:  "Out of credit",
	Status: http.StatusPaymentRequired,
	Detail: "The account has $3 and the operation costs $10.",
	Extra:  map[string]any{"balance": 300},
}
```

| Field | Role |
|---|---|
| `Type` | URI naming the kind of problem; default `about:blank` |
| `Title` | short summary, the same for every occurrence; default the status text |
| `Status` | HTTP status |
| `Detail` | what happened **this** time; read by a person |
| `Instance` | this occurrence; default the request path |
| `Fields` | the `FieldErrors` of a 422 |
| `Extra` | extension members, written at the top level (`balance` above) |

`trilha.ProblemType` (a `func(status int) string`) fills `Type` for every problem that does
not set one — for an app that documents its errors at a URL of its own.

In production a 5xx never carries `Detail`, and the message goes to the log with the
`request_id`; in `Dev` it comes in the response. A `Detail` **you** wrote is yours and is
always sent: the rule is about what the framework would leak, not about what you decided to
say.

## Content negotiation

`c.Accepts(offers...)` returns the offer the client prefers, ranked by the `q` values in
`Accept`, or `""` when it accepts none of them. An absent or `*/*` `Accept` is not a
preference, so put your default first:

```go
switch c.Accepts("text/html", "application/json") {
case "application/json":
	return c.JSON(200, ev)
default:
	return c.Render(200, page(ev))
}
```

## FieldErrors

`trilha.FieldErrors` is a `map[string]string` (field → message) that implements `error`.
Returned from a handler it answers **422**: JSON with `"fields"` in API routes, an error page
in pages. A form usually does not return it: it validates and, on error, calls
`c.Render(422, …)` showing each message in its field (`ui.Errors`, `ui.InvalidIf`).

| Method | Role |
|---|---|
| `Add(field, msg)` | records (the first message for a field wins) |
| `Has(field) bool`, `Get(field) string` | lookup |
| `Any() bool` | are there errors? |
| `OrNil() error` | `nil` when empty, for `return errs.OrNil()` |
