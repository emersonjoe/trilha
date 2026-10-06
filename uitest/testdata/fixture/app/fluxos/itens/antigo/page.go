// Package antigo is an address that moved: a link to it inside the region
// follows the redirect in one GET and shows the flash (#292).
package antigo

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	c.Flash("info", "This address moved")
	return nil, c.Redirect("/fluxos/itens")
}
