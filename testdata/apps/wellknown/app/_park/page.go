// A leading underscore is how a page is parked out of the routing on purpose.
package park

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func Page(c *trilha.Ctx) (h.Node, error) { return h.Div(), nil }
