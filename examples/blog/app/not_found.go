package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// NotFound renders the 404 page inside the root layout.
func NotFound(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Não encontrado")
	return h.Fragment(
		h.H1(h.Text("404")),
		h.P(h.Textf("Nada em %s.", c.Request().URL.Path)),
		h.A(h.Href("/"), h.Text("Voltar ao início")),
	), nil
}
