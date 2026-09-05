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
		return trilha.ErrNotFound // {"error":"Not Found","status":404}
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
| `trilha.Errorf(422, "msg")` | 422 with `{"error":"msg"}` |
| `c.Redirect(url)` | 303 |
| any other `error` | 500 with `{"error":"Internal Server Error"}`; the real message goes to the log |

In API routes errors come out as JSON; in pages, as HTML. The format follows the kind of
route, not the `Accept` header.

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
