package receitas

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/site/internal/ui"
)

// Page redirects the prefix-less path /receitas to /pt/receitas.
func Page(c *trilha.Ctx) (h.Node, error) { return ui.Legacy(c, "/receitas") }
