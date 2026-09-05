package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/site/internal/home"
)

// Page renders the home page (English, the default locale).
func Page(c *trilha.Ctx) (h.Node, error) { return home.Page(c, "en") }
