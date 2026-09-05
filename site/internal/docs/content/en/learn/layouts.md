---
title: Nested layouts
description: One layout per folder, from the innermost to the outermost, and how the title travels between them.
---

A `layout.go` wraps every page in its folder and in the folders below. The layout in
`app/` is the root and is usually the only one that writes `<html>`.

## The signature

```go
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error)
```

`children` is the page already rendered as a node, or the innermost layout already applied.
You decide where to place it.

## A layout for the agenda

Create `app/events/layout.go`:

```go
package events

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	return h.Section(h.Class("agenda"),
		h.Nav(
			h.A(h.Href("/events"), h.Text("All")),
			h.A(h.Href("/events/new"), h.Text("New event")),
		),
		children,
	), nil
}
```

Now `/events`, `/events/new` and `/events/anything` appear inside that `<section>`, which in
turn appears inside the `<main>` of the root layout.

@demo layout

## Execution order

For `GET /events/go-meetup`:

1. `app/events/slug_/page.go` → `Page` produces the page node.
2. `app/events/layout.go` → receives that node as `children`.
3. `app/layout.go` → receives the result of step 2.

Inside out. A folder without `layout.go` simply does not take part.

## Title and other page data for the layout

The page runs **before** the layouts. That is why `c.SetTitle("Events")` in the page works
in the root layout, which reads `c.Title()` to build the `<title>`. The same goes for any
value you store with `c.Set(key, value)` and read with `c.Get(key)`.

```go
// in the page
c.SetTitle("Go Meetup")
c.Set("description", "An evening of talks in Campinas")

// in the root layout
h.Title(h.Text(c.Title())),
h.Meta(h.Name("description"), h.Content(str(c.Get("description")))),
```

:::tip
If there is no `app/layout.go`, Trilha wraps the page in a minimal `<html>`. Handy in the
first minutes; create your own as soon as you want CSS.
:::

## Layouts in route groups

A group (`organizer-/`) may have a layout. It applies to the pages of the group and counts as
one level in the order: `page → group layout → root layout`.

## Challenge

Make the layout of `app/events/` show, below the navigation, a `<p>` with the title of the
current page, to confirm that the title set in `Page` is already available there.

:::solution
```go
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	return h.Section(h.Class("agenda"),
		h.Nav(
			h.A(h.Href("/events"), h.Text("All")),
			h.A(h.Href("/events/new"), h.Text("New event")),
		),
		h.P(h.Class("crumb"), h.Text(c.Title())),
		children,
	), nil
}
```
:::
