package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Layout is the root layout: the <html> document around every page.
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	title := c.Title()
	if title == "" {
		title = "Trilha Blog"
	} else {
		title += " · Trilha Blog"
	}
	return h.Html(h.Lang("pt-BR"),
		h.Head(
			h.Meta(h.Charset("utf-8")),
			h.Meta(h.Name("viewport"), h.Content("width=device-width, initial-scale=1")),
			h.Title(h.Text(title)),
			h.Link(h.Rel("stylesheet"), h.Href("/style.css")),
		),
		h.Body(
			h.Header(h.Nav(
				h.A(h.Href("/"), h.Text("Início")),
				h.A(h.Href("/blog"), h.Text("Blog")),
				h.A(h.Href("/docs/guia/rotas"), h.Text("Docs")),
				h.A(h.Href("/admin"), h.Text("Admin")),
			)),
			h.Main(h.ID("conteudo"), children),
			h.Footer(h.Small(h.Textf("request %s", c.RequestID()))),
		),
	), nil
}
