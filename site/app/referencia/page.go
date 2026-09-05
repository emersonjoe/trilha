package referencia

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/site/internal/ui"
)

// Page redirects the pre-i18n path /referencia to /pt/referencia.
func Page(c *trilha.Ctx) (h.Node, error) { return ui.Legacy(c, "/referencia") }
