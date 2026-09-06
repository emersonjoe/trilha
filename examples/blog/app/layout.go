package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Layout is the root layout: the <html> document around every page.
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	title := c.Title()
	if title == "" {
		title = "Trilha Blog"
	} else {
		title += " · Trilha Blog"
	}
	cur := c.Request().URL.Path
	return h.Html(h.Lang("pt-BR"),
		h.Head(
			h.Meta(h.Charset("utf-8")),
			h.Meta(h.Name("viewport"), h.Content("width=device-width, initial-scale=1")),
			h.Title(h.Text(title)),
			ui.Head(c),
			h.Link(h.Rel("stylesheet"), h.Href(c.Asset("/style.css"))),
		),
		h.Body(ui.Body(),
			ui.Header(
				ui.Brand("/", "Trilha Blog"),
				ui.Nav(
					ui.NavLink("/blog", "Blog", cur == "/blog"),
					ui.NavLink("/docs/guia/rotas", "Docs", cur == "/docs/guia/rotas"),
					ui.NavLink("/precos", "Preços", cur == "/precos"),
					ui.NavLink("/painel", "Painel", cur == "/painel"),
					ui.NavLink("/admin", "Admin", cur == "/admin"),
				),
				ui.Spacer(),
				ui.ThemeToggle(),
			),
			h.Main(h.ID("conteudo"), ui.Container(children)),
			h.Footer(ui.Container(ui.Muted(h.Textf("request %s", c.RequestID())))),
			ui.Flashes(c),
		),
	), nil
}
