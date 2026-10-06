// Package curto is a destination of the prefetch page, counted by the server.
package curto

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"

	"example.com/uitest/internal/conta"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	conta.Hit(c)
	c.SetTitle("Destination curto")
	return h.Div(ui.NavigateScript(c),
		h.Section(h.ID("regiao"), ui.Navigate(""), ui.Prefetch(),
			h.H1(h.Text("Destination curto")))), nil
}
