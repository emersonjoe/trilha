package precos

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Page renders GET /precos (inside the marketing group layout).
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Preços")
	return h.Fragment(h.H1(h.Text("Preços")), h.P(h.Text("Grátis. É um exemplo."))), nil
}
