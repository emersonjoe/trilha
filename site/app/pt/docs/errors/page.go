package errors

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Page lista os códigos estáveis dos erros de runtime.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Catálogo de erros")
	return h.Article(h.Class("conteudo"),
		h.P(h.Class("secao"), h.Text("Referência")),
		h.H1(h.Text("Catálogo de erros")),
		h.P(h.Class("descricao"), h.Text("Códigos estáveis com a causa e o próximo passo.")),
		h.Ul(h.Map(trilha.ErrorGuides(), func(guide trilha.ErrorGuide) h.Node {
			return h.Li(h.A(h.Href(c.Base()+"/pt/docs/errors/"+guide.Code),
				h.Code(h.Text(guide.Code)), h.Text(" — "+guide.Title)))
		})),
	), nil
}
