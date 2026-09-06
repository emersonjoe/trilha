package slug

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/site/internal/ui"
)

// Page redirects the prefix-less path /receitas/{slug} to /pt/receitas/{slug}.
func Page(c *trilha.Ctx) (h.Node, error) { return ui.Legacy(c, "/receitas/"+c.Param("slug")) }
