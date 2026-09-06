package receitas

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/site/internal/docs"
	"github.com/emersonjoe/trilha/site/internal/ui"
)

// Page renders the index of the "receitas" section (pt).
func Page(c *trilha.Ctx) (h.Node, error) {
	p, _ := docs.Get("pt", "receitas", "")
	return ui.DocPage(c, p)
}
