// Package painel is a route group for the app area (/painel, /relatorio).
package painel

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Layout wraps the app area with a sidebar.
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	area, _ := c.Get("area").(string)
	cur := c.Request().URL.Path
	return h.Section(h.Class("app"), h.Data("area", area),
		ui.Sidebar(ui.Nav(
			ui.NavLink("/painel", "Painel", cur == "/painel"),
			ui.NavLink("/relatorio", "Relatório", cur == "/relatorio"),
		)),
		h.Div(h.Class("app-content"), children),
	), nil
}
