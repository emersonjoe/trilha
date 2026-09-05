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
