package slug

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/site/internal/docs"
	"github.com/emersonjoe/trilha/site/internal/ui"
)

// Page renders /aprender/{slug}.
func Page(c *trilha.Ctx) (h.Node, error) {
	p, ok := docs.Get("aprender", c.Param("slug"))
	if !ok {
		return nil, trilha.ErrNotFound
	}
	return ui.DocPage(c, p)
}
