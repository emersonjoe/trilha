---
title: Errors
description: The error values Trilha understands and how each one becomes a response.
---

Handlers return `error`. Trilha translates:

| Value | Page (`page.go`) | API (`route.go`) |
|---|---|---|
| `nil` | response written by the handler; 204 if nothing was written | same |
| `trilha.ErrNotFound` (or an error wrapping it) | 404 with `not_found.go` | `{"error":"Not Found","status":404}` |
| `*trilha.RedirectError` via `trilha.Redirect(url)` (303) or `trilha.RedirectCode(url, code)` | redirect | redirect |
| `*trilha.HTTPError` via `trilha.Errorf(code, fmt, a...)` | simple page with the status and the message (4xx) | `{"error":"message","status":code}` |
| any other `error` | 500 with `error.go`; details only in dev | `{"error":"Internal Server Error","status":500}` |
| `panic` in the handler | recovered and handled as 500; stack only in dev | same |

### Page or JSON?

The column is decided per route, with a per-request tie-breaker:

- `page.go` → always a page.
- `route.go` → JSON, **except** on a browser navigation: `Accept` with `text/html` and
  without `application/json`, outside `/api/`. So a `route.go` that serves HTML shows the
  error page instead of `{"error":...}`; `fetch` without `Accept` (`*/*`), `curl` and JSON
  clients keep receiving JSON.
- `route.go` can pin the behavior by exporting `var Kind = trilha.KindPage` (always a page,
  and CSRF required on `POST`/`PUT`/`PATCH`/`DELETE`) or `trilha.KindAPI` (always JSON).

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
