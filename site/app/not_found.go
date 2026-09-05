package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// NotFound renders the 404 page (also exported as 404.html).
func NotFound(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Página não encontrada")
	b := c.Base()
	return h.Section(h.Class("heroi"),
		h.H1(h.Text("Essa trilha não existe")),
		h.P(h.Class("lema"), h.Text("A página que você procurou não está aqui. Talvez o capítulo tenha mudado de nome.")),
		h.Div(h.Class("acoes"), h.A(h.Class("botao primario"), h.Href(b+"/aprender"), h.Text("Ir para Aprender")), h.A(h.Class("botao"), h.Href(b+"/"), h.Text("Início"))),
	), nil
}
