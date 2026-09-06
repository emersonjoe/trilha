// testdata is Go's own fixture folder; a page.go here is a fixture, not a route.
package testdata

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func Page(c *trilha.Ctx) (h.Node, error) { return h.Div(), nil }
