package pt

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/site/internal/custos"
)

// Page renders /pt/custos: the vitrine of the Tokens 70 goal, in Portuguese.
func Page(c *trilha.Ctx) (h.Node, error) { return custos.Page(c, "pt") }
