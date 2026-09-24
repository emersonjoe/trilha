package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Page renders GET /.
func Page(c *trilha.Ctx) (h.Node, error) {
	return h.Div(h.H1(h.Text("Início"))), nil
}
