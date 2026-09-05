package slug

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/site/internal/docs"
	"github.com/emersonjoe/trilha/site/internal/ui"
)

// Page renders one page of the "referencia" section (pt).
func Page(c *trilha.Ctx) (h.Node, error) {
	p, ok := docs.Get("pt", "referencia", c.Param("slug"))
	if !ok {
		return nil, trilha.ErrNotFound
	}
	return ui.DocPage(c, p)
}
