package pt

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/site/internal/home"
)

// Page renders /pt: the home page in Portuguese.
func Page(c *trilha.Ctx) (h.Node, error) { return home.Page(c, "pt") }
