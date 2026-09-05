package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/site/internal/ui"
)

// NotFound renders the 404 page (also exported as 404.html, so it is served
// for any unknown path in either language).
func NotFound(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle(ui.T(c, "notfound.title"))
	b := c.Base()
	return h.Section(h.Class("heroi"),
		h.H1(h.Text("This trail does not exist")),
		h.P(h.Class("lema"), h.Text("The page you looked for is not here. Maybe the chapter was renamed.")),
		h.P(h.Class("lema"), h.Lang("pt-BR"), h.Text("A página que você procurou não está aqui. Talvez o capítulo tenha mudado de nome.")),
		h.Div(h.Class("acoes"),
			h.A(h.Class("botao primario"), h.Href(b+"/learn"), h.Text("Go to Learn")),
			h.A(h.Class("botao"), h.Href(b+"/pt/aprender"), h.Lang("pt-BR"), h.Text("Ir para Aprender")),
			h.A(h.Class("botao"), h.Href(b+"/"), h.Text("Home")),
		),
	), nil
}
