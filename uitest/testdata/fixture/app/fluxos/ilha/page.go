// Package ilha has one island, with a fallback for when the script is not
// there yet.
package ilha

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	return h.Div(h.H1(h.Text("Island")),
		c.Island("/ilha-contador.js", map[string]int{"start": 41}, h.P(h.ID("contador"), h.Text("fallback")))), nil
}
