// Package sai redirects, as a session that expired would: a prefetch of it is
// not kept, and the click takes the normal path.
package sai

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"

	"example.com/uitest/internal/conta"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	conta.Hit(c)
	return nil, c.Redirect("/fluxos/semregiao")
}
