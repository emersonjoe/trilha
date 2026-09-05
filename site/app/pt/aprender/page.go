package aprender

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/site/internal/docs"
	"github.com/emersonjoe/trilha/site/internal/ui"
)

// Page renders the index of the "aprender" section (pt).
func Page(c *trilha.Ctx) (h.Node, error) {
	p, _ := docs.Get("pt", "aprender", "")
	return ui.DocPage(c, p)
}
