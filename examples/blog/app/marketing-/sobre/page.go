package sobre

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Page renders GET /sobre (inside the marketing group layout).
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Sobre")
	return h.Div(h.Class("ui-stack"), h.H1(h.Class("ui-h1"), h.Text("Sobre")), h.P(h.Text("Trilha é um framework web para Go."))), nil
}
