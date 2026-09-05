package painel

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/posts"
	"github.com/emersonjoe/trilha/h"
)

// Page renders GET /painel.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Painel")
	return h.Fragment(h.H1(h.Text("Painel")), h.P(h.Textf("%d posts.", len(posts.All())))), nil
}
