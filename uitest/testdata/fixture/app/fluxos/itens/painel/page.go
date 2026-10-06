// Package painel declares, from the server, which address rebuilds what a
// fragment answer drew (#293): a POST that creates and pushes, a link whose
// canonical address replaces its own, and searches with and without a
// history entry each.
package painel

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	doc := c.Query("doc")
	if c.Fragment() == "painel" && doc != "" && c.Query("x") != "" {
		c.ReplaceURL("/fluxos/itens/painel?doc=" + doc)
	}
	if c.Fragment() == "painel" {
		return painel(c, doc), nil
	}
	c.SetTitle("Panel")
	return h.Div(h.H1(h.Text("Panel")),
		h.A(h.ID("canonico"), h.Href("/fluxos/itens/painel?doc=3&x=1"), ui.Swap("painel"), h.Text("Doc 3")),
		h.Form(h.ID("busca"), h.Method("get"), ui.Swap("painel"), h.Input(h.ID("busca-q"), h.Name("doc")), h.Button(h.Type("submit"), h.Text("Search"))),
		h.Form(h.ID("busca-push"), h.Method("get"), ui.Swap("painel"), ui.PushHistory(),
			h.Input(h.ID("busca-push-q"), h.Name("doc")), h.Button(h.Type("submit"), h.Text("Search"))),
		h.Form(h.ID("criar"), h.Method("post"), h.Action("/fluxos/itens/painel"), ui.Swap("painel"), trilha.CSRFInput(c),
			h.Button(h.Type("submit"), h.Text("Create"))),
		painel(c, doc)), nil
}

func POST(c *trilha.Ctx) error {
	c.PushURL("/fluxos/itens/painel?doc=7")
	return c.Render(200, painel(c, "7"))
}

func painel(c *trilha.Ctx, doc string) h.Node {
	if doc == "" {
		return h.Div(h.ID("painel"), h.Text("no document"))
	}
	return h.Div(h.ID("painel"), h.Text("document "+doc))
}
