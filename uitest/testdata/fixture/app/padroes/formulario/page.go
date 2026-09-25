// Package formulario serves the async-form pattern as it is.
package formulario

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/patterns/asyncform"
	"github.com/emersonjoe/trilha/h"
)

func Page(c *trilha.Ctx) (h.Node, error) { return asyncform.Page(c) }

func POST(c *trilha.Ctx) error { return asyncform.POST(c) }
