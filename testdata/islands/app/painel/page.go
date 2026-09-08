package painel

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	return h.Div(
		c.Island("/clock.js", nil),
		c.Island("/chart.js", map[string]any{"points": 12}),
	), nil
}
