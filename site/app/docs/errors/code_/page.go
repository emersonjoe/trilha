package code

import (
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Page renders one entry from the runtime error catalog.
func Page(c *trilha.Ctx) (h.Node, error) {
	guide, ok := trilha.ErrorGuideByCode(strings.ToUpper(c.Param("code")))
	if !ok {
		return nil, trilha.ErrNotFound
	}
	c.SetTitle(guide.Code)
	return h.Article(h.Class("conteudo"),
		h.P(h.Class("secao"), h.A(h.Href(c.Base()+"/docs/errors"), h.Text("Error catalog"))),
		h.H1(h.Code(h.Text(guide.Code))),
		h.H2(h.Text(guide.Title)),
		h.P(h.Text(guide.Description)),
		h.H2(h.Text("Fix")),
		h.P(h.Text(guide.Repair)),
		h.P(h.A(h.Href(c.Base()+guide.Reference), h.Text("Read the related reference"))),
	), nil
}
