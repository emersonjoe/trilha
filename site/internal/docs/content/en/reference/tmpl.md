---
title: Package tmpl
description: Use html/template inside the pages and layouts pipeline.
---

```go
import "github.com/emersonjoe/trilha/tmpl"
```

| Function | Description |
|---|---|
| `Node(t *template.Template, name string, data any) h.Node` | node that executes the named template; an execution error becomes a render error (500), with no partial output |
| `Must(fsys fs.FS, patterns ...string) *template.Template` | `template.ParseFS` that panics on error; call it at package level to fail at startup |
| `Wrap(t *template.Template, name, slot string) *Shell` | prepares a shell template to receive an `h.Node` where it calls `{{template slot .}}`; call it at package level — it clones the set, and `html/template` only clones a set that has not executed yet |
| `(*Shell) Node(data any, children h.Node) h.Node` | renders the shell with `data` and `children` in the slot; a shell that never reaches the slot is a render error |
| `HTML(n h.Node) (template.HTML, error)` | the node as `template.HTML`, for data given to a template the app executes itself |

## Usage

```go
//go:embed *.html
var files embed.FS

var t = tmpl.Must(files, "*.html")

func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Report")
	return tmpl.Node(t, "report", data), nil
}
```

The template uses `{{define "report"}}...{{end}}` and receives `data` as `.`. Escaping is
the contextual escaping of `html/template`. The node can be combined with the DSL:
`h.Section(h.Class("x"), tmpl.Node(t, "part", d))`.

## The other direction: an h.Node inside a template

The shell of an app that already exists in `html/template`, with the new screens written in
`h`, is the halfway house of every migration. `Wrap` ties the two together:

```go
var shell = tmpl.Wrap(tmpl.Must(files, "*.html"), "shell", "content")

func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	return shell.Node(page(c.Request()), children), nil
}
```

The template keeps the shape it had — the slot is the `{{template "content" .}}` that was
already there — and nothing in the app converts anything to `template.HTML`: what `h`
rendered was escaped on the way in, and `tmpl` is the single place that says so. The data of
the shell can be built from the `*http.Request` alone, including
[`trilha.NonceFrom(r)` and `trilha.CSRFTokenFrom(r)`](/reference/ctx). `examples/blog` has
the whole thing in `app/legado-`.

`Wrap` clones the set, so call it at package level: `html/template` refuses to clone a set
that has already executed. A shell that never reaches the slot — a `{{if}}` that hid it, the
wrong slot name — fails the render with `tmpl: template %q never rendered the slot` instead
of quietly answering a page with no content.

`HTML(n)` is the low-level way out, for a template the app executes itself.
