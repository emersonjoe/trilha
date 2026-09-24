package admin

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Page answers GET / — or what used to be the home page: this file was left
// behind by a rename and now serves the same pattern the root page does.
func GET(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Admin")
	return h.Div(h.H1(h.Text("Admin"))), nil
}
