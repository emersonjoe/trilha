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
		title = "Assistente · Trilha"
	}
	return h.Html(h.Lang("pt-BR"),
		h.Head(
			h.Meta(h.Charset("utf-8")),
			h.Meta(h.Name("viewport"), h.Content("width=device-width, initial-scale=1")),
			h.Title(h.Text(title)),
			ui.Head(c),
			h.Link(h.Rel("stylesheet"), h.Href("/style.css")),
			h.Script(h.Src("/chat.js"), h.Defer()),
		),
		h.Body(ui.Body(),
			ui.Header(
				ui.Brand("/", "Assistente"),
				ui.Nav(h.A(h.Href("https://emersonjoe.github.io/trilha/aprender/ia-e-agentes"), h.Text("Como funciona"))),
				ui.Spacer(),
				ui.ThemeToggle(),
			),
			h.Main(ui.Container(children)),
		),
	), nil
}
