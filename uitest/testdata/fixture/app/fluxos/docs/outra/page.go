// Package outra is another list, reached by client navigation while the
// panel is open.
package outra

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"

	"example.com/uitest/app/fluxos/docs"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Other list")
	return docs.Frame(c, "Other list", h.A(h.Href("/fluxos/docs"), h.Text("Back to documents"))), nil
}
