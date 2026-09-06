package contacts

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Page is a route of the embedded app. The folder is still the address,
// /contacts, and the binary that hosts it never learns that.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Contacts")
	return h.H1(h.Text("Contacts")), nil
}
