// Package lista serves the list-with-filter pattern as it is.
package lista

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/patterns/listwithfilter"
	"github.com/emersonjoe/trilha/h"
)

func Page(c *trilha.Ctx) (h.Node, error) { return listwithfilter.Page(c) }
