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
		title = "Cadastro · Trilha"
	}
	return h.Html(h.Lang("pt-BR"),
		h.Head(
			h.Meta(h.Charset("utf-8")),
			h.Meta(h.Name("viewport"), h.Content("width=device-width, initial-scale=1")),
			h.Title(h.Text(title)),
			ui.Head(c),
			h.Link(h.Rel("stylesheet"), h.Href(c.Asset("/style.css"))),
		),
		h.Body(ui.Body(),
			ui.Header(ui.Brand("/", "Cadastro"), ui.Nav(h.A(h.Href("/ficha"), h.Text("Ficha")), h.A(h.Href("/api/cidades?uf=SP"), h.Text("API"))), ui.Spacer(), ui.ThemeToggle()),
			h.Main(ui.Container(children)),
			ui.Toaster(h.If(c.Query("ok") == "1", ui.Toast("success", "Cadastro salvo!", 4000))),
			// O comportamento da árvore mora num arquivo à parte, como o do
			// upload: uma página sem árvore não baixa nada disso.
			ui.TreeScript(c),
		),
	), nil
}
