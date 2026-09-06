package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

var CORS = trilha.CORS{Origins: []string{"*"}}

func Page(c *trilha.Ctx) (h.Node, error) { return h.Div(), nil }
