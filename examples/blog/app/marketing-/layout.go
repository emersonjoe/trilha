// Package marketing is a route group: its layout wraps /precos and /sobre
// without adding a URL segment (folder name ends with "-").
package marketing

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Layout wraps every page in the marketing group.
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	cur := c.Request().URL.Path
	return h.Section(h.Class("marketing"),
		h.Nav(h.Class("sub ui-nav"), ui.NavLink("/precos", "Preços", cur == "/precos"), ui.NavLink("/sobre", "Sobre", cur == "/sobre")),
		children,
	), nil
}
