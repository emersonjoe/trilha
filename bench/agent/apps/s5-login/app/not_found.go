package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// NotFound renders the 404 page inside the root layout.
func NotFound(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Não encontrado")
	return ui.Stack(ui.H1(h.Text("404")), ui.Lead(h.Text("Página não encontrada.")), h.Div(ui.ButtonLink("/", h.Text("Início")))), nil
}
