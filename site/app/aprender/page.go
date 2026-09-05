package aprender

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/site/internal/docs"
	"github.com/emersonjoe/trilha/site/internal/ui"
)

// Page renders /aprender: the first chapter (início rápido).
func Page(c *trilha.Ctx) (h.Node, error) {
	p, _ := docs.Get("aprender", "")
	return ui.DocPage(c, p)
}
