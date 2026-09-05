package relatorio

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/sso/internal/sso"
	"github.com/emersonjoe/trilha/h"
)

// Page é a área restrita ao papel.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Relatório")
	return h.Div(h.Class("cartao"),
		h.H1(h.Text("Relatório")),
		h.P(h.Text("Só quem tem o papel "+sso.AdminRole()+" chega aqui.")),
	), nil
}
