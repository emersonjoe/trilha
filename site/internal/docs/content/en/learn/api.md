---
title: API routes
description: route.go with one function per HTTP method, JSON in and out, and errors with status codes.
---

A folder with `route.go` answers JSON instead of HTML. Each HTTP method is an exported
function with the usual shape: `func(c *trilha.Ctx) error`.

## List and create

`app/api/events/route.go`:

```go
package events

import (
	"net/http"
	"strings"

	"github.com/emersonjoe/trilha"
	"agenda/internal/events"
)

func GET(c *trilha.Ctx) error {
	return c.JSON(http.StatusOK, events.All())
}

func POST(c *trilha.Ctx) error {
	var in struct {
		Name string `json:"name"`
		City string `json:"city"`
	}
	if err := c.BindJSON(&in); err != nil {
		return err // 400 on invalid JSON, 413 above the limit
	}
	if strings.TrimSpace(in.Name) == "" {
		return trilha.Errorf(http.StatusUnprocessableEntity, "name is required")
	}
	ev := events.Create(in.Name, in.City)
	c.Header("Location", "/api/events/"+ev.Slug)
	return c.JSON(http.StatusCreated, ev)
}
```

```bash
curl -s localhost:3000/api/events
curl -s -X POST localhost:3000/api/events -d '{"name":"HTTP Workshop","city":"Recife"}'
curl -s -X PUT localhost:3000/api/events     # 405 with Allow: GET, POST
```

## One resource per slug

`app/api/events/slug_/route.go` answers `/api/events/{slug}`:

```go
func GET(c *trilha.Ctx) error {
	ev, ok := events.Find(c.Param("slug"))
	if !ok {
		return trilha.ErrNotFound // 404 problem+json
	}
	return c.JSON(200, ev)
}

func DELETE(c *trilha.Ctx) error {
	if !events.Delete(c.Param("slug")) {
		return trilha.ErrNotFound
	}
	c.Writer().WriteHeader(http.StatusNoContent)
	return nil
}
```

## Errors become status codes

| You return | Response |
|---|---|
| `nil` | whatever you wrote; 204 if you wrote nothing |
| `trilha.ErrNotFound` | 404 as JSON |
| `trilha.Errorf(422, "msg")` | 422 with `"detail":"msg"` |
| `c.Redirect(url)` | 303 |
| any other `error` | 500 with `"title":"Internal Server Error"`; the real message goes to the log |
| `&trilha.Problem{…}` | exactly the problem you described |

The body is [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) problem details, sent as
`application/problem+json` — the format generated clients, gateways and contract tests
already read:

```json
{"type":"about:blank","title":"Not Found","status":404,
 "instance":"/api/events/nope","request_id":"01J…"}
```

`fields` is still there on a 422, unchanged, so the form that reads it keeps working. When a
status is not enough, describe the problem yourself:

```go
return &trilha.Problem{
	Type:   "https://example.com/probs/sold-out",
	Title:  "Sold out",
	Status: http.StatusConflict,
	Detail: "The last seat went 4 minutes ago.",
	Extra:  map[string]any{"waitlist": "/api/events/" + ev.Slug + "/waitlist"},
}
```

Which format comes out follows the kind of route, with `Accept` as the tie-breaker: a
`route.go` answers `problem+json`, unless the client prefers `text/html` — a browser in the
address bar gets the error page, wherever the route lives. See
[Errors](/reference/errors).

## The OpenAPI document

`trilha openapi` writes the OpenAPI 3.1 document of your API routes. There is nothing to
annotate and keep in sync: the source of the document is the code that answers the request.

```bash
trilha openapi                    # writes openapi.json
trilha openapi -o - | jq .paths   # to stdout
trilha openapi --check            # in the CI: fails when the file drifted from the code
```

What it reads by itself:

| In the code | In the document |
|---|---|
| the folder under `app/api/` | the path, with `id_` as a path parameter |
| exported `GET`, `POST`, `PUT`, `PATCH`, `DELETE` | one operation each |
| the doc comment | `summary` (first sentence) and `description` |
| `c.Bind(&in)` / `c.BindJSON(&in)` | `requestBody` with the schema of `in`, plus a 422 |
| `c.JSON(status, v)` | that status with the schema of `v` |
| `c.Writer().WriteHeader(204)` | that status with no body |
| `c.Header("Content-Type", …)` | the media type of the response |
| `trilha.ErrNotFound`, `trilha.Errorf(status, …)`, `&trilha.Problem{Status: …}` | that status as `problem+json` |
| `json` and `validate` tags | property names, `required`, `maxLength`, `enum`, `format` |

The schema comes out of the same `validate` tag `Bind` reads, so the document cannot promise
something the validation refuses. Every operation also carries the `default` response with
the [`Problem`](/reference/errors) schema: since 0.21.0 that is the shape of every API error.

Only `route.go` routes are described. A page answers HTML to a browser; there is no contract
there for a client to hold you to.

### When the deduction does not reach

A middleware, a `c.Query` or a folder with a dot in its name are outside what reading the
handler can tell. Write it in the doc comment:

```go
// GET writes the month as CSV.
//
// openapi:query mes string  month to export, AAAA-MM (default: the current one)
// openapi:response 429
// openapi:tag report
func GET(c *trilha.Ctx) error { … }
```

| Directive | What it does |
|---|---|
| `openapi:response <status> [type]` | adds the response; without a type, `problem+json` |
| `openapi:body <type>` | the request body, when it is not a `Bind` |
| `openapi:query <name> <type> [description]` | a query parameter |
| `openapi:tag <name>` | the tag of the operation (default: the last fixed path segment) |

A type nobody declares is an error naming the file and the handler, not an empty schema
published as if it were right.

## CSRF in APIs

By default `route.go` does **not** require a CSRF token: APIs are usually called with a
session token or a bearer token, and the `SameSite=Lax` cookie already blocks automatic
submission by the browser. If your API is called by the site itself with cookies, turn on
`Config.CSRFForAPI` and send `X-CSRF-Token`.

## Challenge

Add `PATCH` to `/api/events/{slug}` that updates only the fields present in the JSON and
answers 200 with the new event. JSON with unknown fields must return 400.

:::solution
`c.BindJSON` already rejects unknown fields. For "only the fields present", use pointers:

```go
func PATCH(c *trilha.Ctx) error {
	var in struct {
		Name *string `json:"name"`
		City *string `json:"city"`
	}
	if err := c.BindJSON(&in); err != nil {
		return err
	}
	ev, ok := events.Find(c.Param("slug"))
	if !ok {
		return trilha.ErrNotFound
	}
	if in.Name != nil {
		ev.Name = *in.Name
	}
	if in.City != nil {
		ev.City = *in.City
	}
	events.Save(ev)
	return c.JSON(200, ev)
}
```
:::
