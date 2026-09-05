---
title: Pages and routes
description: How folders become URLs, including dynamic segments, catch-all and groups.
---

You have seen that `app/events/page.go` answers `/events`. This chapter covers the rest of
the mapping: URL parameters, paths of variable length and folders that group pages without
showing up in the URL.

## Dynamic segment: `name_`

Each event gets its own page at `/events/go-meetup`. Instead of one folder per event, create
a folder whose name ends with `_`:

```text
app/events/slug_/page.go   →   GET /events/{slug}
```

Inside the page, the value comes from `c.Param`:

```go
package slug

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	slug := c.Param("slug")
	c.SetTitle("Event " + slug)
	return h.H1(h.Textf("Event: %s", slug)), nil
}
```

The parameter name is the folder name without the `_`. A folder `id_` gives `c.Param("id")`.

:::note
Why not `[slug]` like other frameworks? Because the folder becomes a **Go package**, and a
package import path accepts neither brackets, braces nor dollar signs. The `_` suffix is
legal, shows up in `go list ./...` and does not confuse the shell.
:::

## Catch-all: `name__`

Two underscores at the end capture everything that follows, inner slashes included:

```text
app/docs/path__/page.go   →   GET /docs/{path...}
```

`GET /docs/guide/install` arrives with `c.Param("path") == "guide/install"`. A catch-all
folder must be a leaf: nothing can exist below it.

## Who wins on a tie

Literal routes beat dynamic ones. With `app/events/new/page.go` and
`app/events/slug_/page.go`, `/events/new` goes to the first and `/events/anything-else` to
the second. Two sibling dynamic folders (`a_` and `b_` at the same level) are a generation
error, because there would be no way to choose.

## Route groups: `name-`

Sometimes you want several pages to share a layout or a middleware without that showing in
the URL. A folder ending in `-` is a **group**:

```text
app/organizer-/middleware.go     ← applies to everything below
app/organizer-/dashboard/page.go → GET /dashboard   (no "organizer" in the URL)
app/organizer-/events/page.go    → GET /events  ✗ conflicts with app/events/page.go
```

The generator refuses two folders that produce the same URL (`E_DUPLICATE_ROUTE`), so the
second example above does not compile.

## What the generator does with this

Run `trilha routes` at any time to see the table:

```text
METHODS   PATTERN            SOURCE
GET       /                  app/page.go
GET       /events            app/events/page.go
GET       /events/{slug}     app/events/slug_/page.go
GET       /dashboard         app/organizer-/dashboard/page.go
```

That table becomes Go code in `trilha_gen.go`: one `a.Register(trilha.Route{...})` per
line, importing each package. If you rename `Page`, the compiler complains, not the server
in production.

## Challenge

Create the detail page `app/events/slug_/page.go` showing the slug, and a page
`app/events/today/page.go`. Confirm with `trilha routes` that `/events/today` points to the
literal folder and not to the dynamic one.

:::solution
Both pages follow the `Page` shape. The output of `trilha routes` must contain:

```text
GET  /events/today      app/events/today/page.go
GET  /events/{slug}     app/events/slug_/page.go
```

Alphabetical order puts `/events/today` first, but what decides precedence is the router:
literal before dynamic, always.
:::
