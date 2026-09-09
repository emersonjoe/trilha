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
| `*trilha.HTTPError` via `trilha.Errorf(code, fmt, a...)` | the status, with `error.go` (4xx) | the status, with the message in `detail` (4xx) |
| any other `error` | 500 with `error.go`; details only in dev | 500, `detail` only in dev |
| `*trilha.Problem` | the status, with `error.go` | the problem, as it was written |
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
- `var Kind = trilha.KindPage` (always a page, and CSRF required on
  `POST`/`PUT`/`PATCH`/`DELETE`) or `trilha.KindAPI` (always `problem+json`, whatever `Accept`
  says) pins the behaviour. It is inherited by the whole subtree, so a `kind.go` at the root
  of a branch decides every `route.go` below it; see
  [File conventions](/reference/conventions#kind-follows-the-subtree).
- With no route at all (404), there is no kind to ask: `Accept` decides, and when it is
  silent the `/api/` prefix is the last resort.

### One page for every status but 404

`app/error.go` answers **every** error status, not only the 5xx: a 403 in an app with roles
is the most common answer after 200, and it deserves the app's menu, wording and layout.
`app/not_found.go` keeps the 404 — it exists and it is the place.

The signature does not change; the status comes from the error:

```go
func Error(c *trilha.Ctx, err error) (h.Node, error) {
	switch trilha.StatusOf(err) {
	case http.StatusForbidden:
		return panel.Denied(c), nil
	default:
		return panel.Broke(c), nil
	}
}
```

`trilha.StatusOf(err)` reports the status the framework will send — the same classification
the table above describes. (`c.Status` is a setter; the page receives the error, not the
code, which is why the function exists.)

The framework's own page stays as the net, with the text it always had: for an app with no
`error.go`, and for an `error.go` that itself fails. API routes (`KindAPI`) are untouched:
`problem+json` as before.

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

## Errors that teach: trilha.Hint

The framework already teaches at build time — the scanner's `E_` codes come with a fix line —
and in `trilha audit`. A `Hint` is the middle: the error that only happens once the app is
running, where the answer used to be a 500 and a stack trace of framework internals.

```go
return trilha.NewHint(trilha.ErrRedirectAbsolute, err).
	Fix("Redirect takes a path; to leave the site, RedirectExternal.").
	Doc("/reference/errors")
```

An ordinary error with three things added: a **code** somebody can paste into a search box, a
sentence saying **what to do instead**, and a **link**. In `Env: Dev` the error page shows all
three; in production it shows what it showed before, because the repair is for whoever writes
the code and the person on the other side did not write it.

It wraps, so `errors.Is` and `errors.As` reach straight through: code that already handled an
error does not start handling a different one. `trilha.HintOf(err)` finds it, or answers nil.

### E_REDIRECT_ABSOLUTE

`Redirect` takes a path and refuses an address that leaves the site.

```go
c.Redirect("/painel")                          // fine
c.Redirect(c.Query("next"))                    // "https://…" is an error, not a redirect
c.RedirectExternal("https://gov.example/pay")  // deliberate, and written down
```

The destination of a redirect almost always came from a form or a query string — `?next=` is how
somebody gets back to the page that asked them to log in — and a redirect that follows it
anywhere is an open redirect: a phishing link on your own domain, with your own certificate.

Sanitising instead of refusing is how an open redirect gets written with more steps.
`//evil.com`, `/\evil.com` and `https:/\evil.com` all exist because each of them walked past a
check somebody thought was enough. What passes here is a path: it starts with `/` and does not
start with `//` or `/\`.

`RedirectExternal` is a separate function so that leaving is written down. A reviewer reading it
knows somebody meant it; a reviewer reading `Redirect` knows nobody could have done it by
accident. It does not check the address, because there is nothing to check — it is a literal in
your code, not a value from a request.

:::warning
This changed in 0.67.0. An app that redirected to an absolute URL gets an error now; the repair
is one word — `RedirectExternal` — wherever the destination is your own literal. Wherever the
destination came from the request, the error is the bug being found.
:::

### E_SECRET_SHORT

`ListenAndServe` refuses to listen when `Env` is not `Dev` and the signing key is shorter than
32 bytes:

```text
trilha: TRILHA_SECRET has 5 bytes and the minimum is 32 (E_SECRET_SHORT)
Generate one with: trilha secret
```

Everything the signature protects — the session cookie, the flash, a public link, a sealed
column — is worth exactly what the key is worth, and a key somebody typed is worth an afternoon.
The number is in the message because "too short" leaves somebody counting characters, and the
command is in it because "generate one" without saying how is half an instruction.

Three deliberate limits on that refusal:

- **In dev it is a log line, once.** A development secret is a development secret, and refusing
  to start would be the framework getting in the way in the one place where it should not.
- **It is `ListenAndServe`, not `New`.** `New` is what a test calls: somebody standing up a
  server is standing up, and somebody building a `Handler()` in a suite is not.
- **No secret at all is a different thing**, and already has an answer: signing fails loudly with
  `ErrNoSecret` wherever it is attempted, and `trilha audit` is critical about it for an app that
  signs and quiet for an app that does not. Refusing here too would stop an app that signs
  nothing, which owes the framework nothing.

`trilha secret` prints one: thirty-two bytes from `crypto/rand`, base64 — the length HMAC-SHA256
uses as a key without folding it, in an encoding that survives a shell, a YAML file and a paste
into a deployment console.
