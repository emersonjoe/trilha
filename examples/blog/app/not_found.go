package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// NotFound renders the 404 page inside the root layout.
func NotFound(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Não encontrado")
	return ui.Stack(
		h.H1(h.Text("404")),
		ui.Lead(h.Textf("Nada em %s.", c.Request().URL.Path)),
		h.Div(ui.ButtonLink("/", h.Text("Voltar ao início"))),
	), nil
}
