package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Layout is the root layout.
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	title := c.Title()
	if title == "" {
		title = "Orçamento · Trilha"
	}
	cur := c.Request().URL.Path
	return h.Html(h.Lang("pt-BR"),
		h.Head(
			h.Meta(h.Charset("utf-8")),
			h.Meta(h.Name("viewport"), h.Content("width=device-width, initial-scale=1")),
			h.Title(h.Text(title)),
			ui.Head(c),
			h.Link(h.Rel("stylesheet"), h.Href(c.Asset("/style.css"))),
			h.Script(h.Src(c.Asset("/app.js")), h.Defer()),
		),
		h.Body(ui.Body(),
			ui.Header(
				ui.Brand("/", "Orçamento"),
				ui.Nav(ui.NavLink("/", "Visão geral", cur == "/"), ui.NavLink("/lancamentos", "Lançar", cur == "/lancamentos"), h.A(h.Href("/api/relatorio.csv?mes="+mes(c)), ui.Icon("download"), h.Text("CSV"))),
				ui.Spacer(),
				ui.ThemeToggle(),
			),
			h.Main(ui.Container(children)),
			ui.Toaster(h.If(c.Query("ok") == "1", ui.Toast("success", "Lançamento registrado.", 4000))),
		),
	), nil
}
