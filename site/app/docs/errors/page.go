package errors

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Page lists the stable runtime error codes and their repair pages.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Error catalog")
	return h.Article(h.Class("conteudo"),
		h.P(h.Class("secao"), h.Text("Reference")),
		h.H1(h.Text("Error catalog")),
		h.P(h.Class("descricao"), h.Text("Stable error codes with the reason and the next action.")),
		h.Ul(h.Map(trilha.ErrorGuides(), func(guide trilha.ErrorGuide) h.Node {
			return h.Li(h.A(h.Href(c.Base()+"/docs/errors/"+guide.Code),
				h.Code(h.Text(guide.Code)), h.Text(" — "+guide.Title)))
		})),
	), nil
}
