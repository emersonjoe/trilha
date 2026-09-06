package cookbook

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/site/internal/docs"
	"github.com/emersonjoe/trilha/site/internal/ui"
)

// Page renders the index of the "cookbook" section (en).
func Page(c *trilha.Ctx) (h.Node, error) {
	p, _ := docs.Get("en", "cookbook", "")
	return ui.DocPage(c, p)
}
