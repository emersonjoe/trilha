// Package marketing is a route group: its layout wraps /precos and /sobre
// without adding a URL segment (folder name ends with "-").
package marketing

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Layout wraps every page in the marketing group.
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	return h.Section(h.Class("marketing"),
		h.Nav(h.Class("sub"), h.A(h.Href("/precos"), h.Text("Preços")), h.A(h.Href("/sobre"), h.Text("Sobre"))),
		children,
	), nil
}
