---
title: Forms
description: POST in the same page.go, automatic CSRF protection and the redirect-after-write pattern.
---

A page can receive forms by exporting `POST` (or `PUT`, `PATCH`, `DELETE`) next to `Page`.
Trilha verifies the CSRF token before calling your function.

## The page with the form

`app/events/new/page.go`:

```go
package new

import (
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"agenda/internal/events"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("New event")
	msg := c.Query("error")
	return h.Fragment(
		h.H1(h.Text("New event")),
		h.If(msg != "", h.P(h.Class("error"), h.Text(msg))),
		h.Form(h.Method("post"), h.Action("/events/new"),
			trilha.CSRFInput(c),
			h.Label(h.For("name"), h.Text("Name")),
			h.Input(h.ID("name"), h.Name("name"), h.Required(), h.Autofocus()),
			h.Label(h.For("city"), h.Text("City")),
			h.Input(h.ID("city"), h.Name("city")),
			h.Button(h.Type("submit"), h.Text("Publish")),
		),
	), nil
}

func POST(c *trilha.Ctx) error {
	if err := c.FormErr(); err != nil {
		return err // 400 on an invalid form, 413 if it exceeded the limit
	}
	name := strings.TrimSpace(c.Form("name"))
	if name == "" {
		return c.Redirect("/events/new?error=Enter+a+name")
	}
	ev := events.Create(name, c.Form("city"))
	return c.Redirect("/events/" + ev.Slug)
}
```

@demo form

## What happens on submit

1. The browser sends `POST /events/new` with the fields and the `_csrf`.
2. Trilha compares `_csrf` with the `trilha_csrf` cookie (constant time). Different or
   missing: **403**, and `POST` does not even run.
3. `POST` runs and returns `c.Redirect(...)`: a **303 See Other** response. The browser does
   a `GET` on the new URL. Reloading the page does not resubmit the form.

`trilha.CSRFInput(c)` creates the cookie on the first render and the hidden field.
JavaScript clients may send the same value in the `X-CSRF-Token` header.

## Validation and messages

The example above returns the error through the query string, which keeps the POST →
redirect → GET pattern and works without JavaScript. For larger forms, keep the typed values
in a short-lived cookie or render the page again with `return c.HTML(422, Page...)`, without
redirecting.

## Methods the browser does not send

HTML forms only send GET and POST. For "delete", export `DELETE` for API clients and make the
page's `POST` call the same logic:

```go
func DELETE(c *trilha.Ctx) error {
	if !events.Delete(c.Param("slug")) {
		return trilha.ErrNotFound
	}
	return c.Redirect("/events")
}

func POST(c *trilha.Ctx) error { return DELETE(c) }
```

## Limits

The request body is limited to 1 MiB by default (`Config.MaxBodyBytes`). Above that the
response is 413 before your code runs.

## Challenge

Add a numeric `seats` field to the form and reject negative values with a message, without
losing the redirect pattern.

:::solution
```go
seats, err := strconv.Atoi(c.Form("seats"))
if err != nil || seats < 0 {
	return c.Redirect("/events/new?error=Seats+must+be+a+positive+number")
}
```
:::
