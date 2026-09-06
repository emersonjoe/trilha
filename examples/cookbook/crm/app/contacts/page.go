package contacts

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/cookbook/crm/internal/contacts"
	"github.com/emersonjoe/trilha/h"
)

// Page is a route of the embedded app. The folder is still the address,
// /contacts, and the binary that hosts it never learns that. The store comes
// from Use, so the page holds no state of its own.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Contacts")
	items := h.Group()
	for _, p := range trilha.Use[*contacts.Store](c).All() {
		items = h.Group(items, h.Li(h.Text(p.Name)))
	}
	return h.Group(h.H1(h.Text("Contacts")), h.Ul(items)), nil
}
