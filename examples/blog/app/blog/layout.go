package blog

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Layout wraps everything under /blog.
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	return h.Section(h.Class("blog"),
		h.Aside(ui.ButtonLink("/blog/novo", ui.Sm(), ui.Icon("plus"), h.Text("Novo post"))),
		children,
	), nil
}
