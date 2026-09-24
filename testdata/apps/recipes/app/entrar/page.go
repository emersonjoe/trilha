package entrar

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Page renders GET /entrar.
func Page(c *trilha.Ctx) (h.Node, error) {
	return h.Div(h.H1(h.Text("Entrar"))), nil
}
