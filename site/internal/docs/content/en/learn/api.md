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
