// Package itens is a list inside a client-navigation region: a GET filter
// that is a plain form, a link to a route that redirects, and the door to
// the forms that post (#291, #292).
package itens

import (
	"fmt"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"

	"example.com/uitest/internal/itens"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Items")
	q := c.Query("q")
	var li []h.Node
	for _, n := range itens.Find(q) {
		li = append(li, h.Li(h.Text(n)))
	}
	return h.Div(ui.NavigateScript(c),
		h.Section(h.ID("regiao"), ui.Navigate(""),
			h.H1(h.Text("Items")),
			h.P(h.ID("visitas"), h.Text(fmt.Sprintf("views: %d", itens.View()))),
			h.Form(h.ID("filtro"), h.Method("get"), h.Action("/fluxos/itens"),
				h.Input(h.ID("q"), h.Name("q"), h.Value(q)), h.Button(h.Type("submit"), h.Text("Filter"))),
			h.Ul(h.ID("lista"), h.Fragment(li...)),
			h.A(h.ID("ir-novo"), h.Href("/fluxos/itens/novo"), h.Text("New item")),
			h.A(h.ID("ir-antigo"), h.Href("/fluxos/itens/antigo"), h.Text("Old address")))), nil
}
