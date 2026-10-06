// Package semregiao has another frame: no client-navigation region, so a
// redirect that lands here has to load the page whole.
package semregiao

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Outside")
	return h.Div(h.H1(h.Text("Outside the region"))), nil
}
