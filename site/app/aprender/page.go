package aprender

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/site/internal/ui"
)

// Page redirects the pre-i18n path /aprender to /pt/aprender.
func Page(c *trilha.Ctx) (h.Node, error) { return ui.Legacy(c, "/aprender") }
