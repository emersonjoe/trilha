package blog

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Layout wraps everything under /blog.
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	return h.Section(h.Class("blog"),
		h.Aside(h.A(h.Href("/blog/novo"), h.Text("+ novo post"))),
		children,
	), nil
}
