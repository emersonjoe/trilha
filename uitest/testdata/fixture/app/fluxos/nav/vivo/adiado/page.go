// Package adiado is the deferred part of the live page.
package adiado

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	return h.Div(h.ID("adiado"), h.Text("Deferred part arrived")), nil
}
