---
title: Middleware
description: Intercept a subtree of routes, pass values to pages and protect areas.
---

A `middleware.go` runs before any route in its folder and in the folders below. The one at
the root runs on every request; the one in a group, only on the group's routes.

## The signature

```go
func Middleware(c *trilha.Ctx, next trilha.Next) error
```

Call `next()` to continue. Do not call it to stop. Return an error so the default handling
answers (redirect, 404, 500).

## Timing every route

`app/middleware.go`:

```go
package app

import (
	"time"

	"github.com/emersonjoe/trilha"
)

func Middleware(c *trilha.Ctx, next trilha.Next) error {
	start := time.Now()
	err := next()
	c.Header("Server-Timing", "app;dur="+time.Since(start).String())
	return err
}
```

The header is written after `next()` but before the response is sent, because pages are
rendered in memory and written at once.

## Protecting the organizer's area

A route group is the natural place to require login without polluting the URL:

```text
app/organizer-/middleware.go
app/organizer-/dashboard/page.go  → /dashboard
app/organizer-/report/page.go     → /report
```

```go
package organizer

import "github.com/emersonjoe/trilha"

func Middleware(c *trilha.Ctx, next trilha.Next) error {
	ck, err := c.Cookie("session")
	if err != nil || !session.Valid(ck.Value) {
		return trilha.RedirectCode("/login?next="+c.Request().URL.Path, 302)
	}
	c.Set("user", session.User(ck.Value))
	return next()
}
```

In the page, `c.Get("user")` returns the value. Values live only during the request.

## Order

For `GET /dashboard`:

```text
middleware(app) → middleware(app/organizer-) → Page → layouts
```

Outside in. If a middleware does not call `next()`, the inner ones and the page do not run,
but the outer ones finish normally (the timing one above still writes its header).

## Short-circuit with your own response

A middleware may answer directly and return `nil`:

```go
if c.Request().Header.Get("X-Maintenance") == "1" {
	return c.Text(503, "under maintenance")
}
```

Since the response has already started, Trilha does not try to write another one.

## Challenge

Create `app/api/middleware.go` requiring the `Authorization: Bearer <key>` header across the
whole API and answering 401 as JSON when it is missing, without affecting the HTML pages.

:::solution
```go
package api

import (
	"net/http"
	"strings"

	"github.com/emersonjoe/trilha"
)

func Middleware(c *trilha.Ctx, next trilha.Next) error {
	auth := c.Request().Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") || !keys.Valid(strings.TrimPrefix(auth, "Bearer ")) {
		return trilha.Errorf(http.StatusUnauthorized, "invalid key")
	}
	return next()
}
```

Because the folder is `app/api/`, only API routes go through it, and the error comes out as
JSON because the route is a `route.go`.
:::
