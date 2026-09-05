---
title: HTML with the h package
description: Elements as functions, escaping by default, conditionals, lists and when to use templates.
---

The `h` package produces HTML without template files: each element is a Go function that
accepts attributes and children in any order. Everything is checked by the compiler and
escaped on output.

## Elements, attributes and text

```go
h.Article(h.Class("event", "featured"),
	h.H2(h.Text(ev.Name)),
	h.P(h.Textf("%s, %d seats", ev.City, ev.Seats)),
	h.A(h.Href("/events/"+ev.Slug), h.Text("Details")),
)
```

- `h.Text` and `h.Textf` escape. `h.Raw` does not, and it is the only door for ready-made
  HTML.
- Attributes (`h.Class`, `h.Href`, `h.ID`, `h.Data("x", v)`, `h.Attr("name", v)`) may come
  after the children; they always end up in the opening tag.
- Void elements (`h.Br`, `h.Img`, `h.Input`, `h.Meta`) do not close.
- Boolean attributes are functions without arguments: `h.Required()`, `h.Disabled()`.
- When a name collides with an element, the attribute gets the `Attr` suffix: `h.StyleAttr`,
  `h.TitleAttr`, `h.LabelAttr`.

@demo escape

## Conditionals and lists

```go
h.Ul(
	h.If(len(events) == 0, h.Li(h.Em(h.Text("no events")))),
	h.Map(events, func(ev Event) h.Node {
		return h.Li(h.Text(ev.Name))
	}),
)
```

`h.If` returns an empty node when the condition is false; `h.IfElse` picks one of two;
`h.Map` applies a function to each item; `h.Fragment` groups several nodes without a
wrapping element. `nil` as a child is ignored, so a `func() h.Node` returning `nil` is safe
too.

@demo lista

## Components are functions

There is no "component" type. A function returning `h.Node` already is one:

```go
func EventCard(ev Event) h.Node {
	return h.Article(h.Class("card"),
		h.H3(h.Text(ev.Name)),
		h.P(h.Text(ev.City)),
	)
}

// in the page
h.Div(h.Class("grid"), h.Map(events, EventCard))
```

## Prefer templates?

The `tmpl` package plugs `html/template` into the same pipeline. The files sit next to the
page and are embedded in the binary:

```go
package report

import (
	"embed"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/tmpl"
)

//go:embed report.html
var files embed.FS

var t = tmpl.Must(files, "*.html") // fails at startup, never during a request

func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Report")
	return tmpl.Node(t, "report", data), nil
}
```

Layouts, title and errors work the same. Escaping is the contextual escaping of
`html/template` itself.

## Challenge

Write a component `Seats(n int) h.Node` that shows "sold out" in italics when `n == 0`,
"1 seat" in the singular and "N seats" in the plural, and use it in the events list.

:::solution
```go
func Seats(n int) h.Node {
	switch {
	case n == 0:
		return h.Em(h.Text("sold out"))
	case n == 1:
		return h.Text("1 seat")
	default:
		return h.Textf("%d seats", n)
	}
}
```
:::
