// Package feito is where the island's post lands.
package feito

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	return h.Div(h.H1(h.ID("feito"), h.Text("Finished"))), nil
}
