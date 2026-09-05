// Package painel is a route group for the app area (/painel, /relatorio).
package painel

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Layout wraps the app area with a sidebar.
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	area, _ := c.Get("area").(string)
	return h.Section(h.Class("app"), h.Data("area", area),
		h.Aside(h.A(h.Href("/painel"), h.Text("Painel")), h.A(h.Href("/relatorio"), h.Text("Relatório"))),
		children,
	), nil
}
