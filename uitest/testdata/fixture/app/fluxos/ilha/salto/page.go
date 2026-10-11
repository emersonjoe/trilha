// Package salto has an island that imports a module of its own by a relative
// path and posts to a route.go that redirects (spec 176).
package salto

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	return h.Div(h.H1(h.Text("Jump")),
		c.Island("/salto/ilha.js", map[string]string{"to": "/fluxos/ilha/concluir"}, h.P(h.Text("fallback")))), nil
}
