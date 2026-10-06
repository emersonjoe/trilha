// Package novo posts a new item from forms of every kind: ui.Swap that
// follows the redirect (#291), that reloads (RedirectReload), whose
// destination has no region, and a plain form inside the region (#292).
package novo

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"

	"example.com/uitest/internal/itens"
)

func Page(c *trilha.Ctx) (h.Node, error) { return page(c, false), nil }

func POST(c *trilha.Ctx) error {
	nome := c.Form("name")
	if nome == "" {
		return c.Render(http.StatusUnprocessableEntity, page(c, true))
	}
	itens.Add(nome)
	c.Flash("success", "Item added: "+nome)
	switch c.Form("modo") {
	case "reload":
		return c.RedirectReload("/fluxos/itens")
	case "fora":
		return c.Redirect("/fluxos/semregiao")
	}
	return c.Redirect("/fluxos/itens")
}

func page(c *trilha.Ctx, invalid bool) h.Node {
	c.SetTitle("New item")
	form := func(id, modo string, swap bool) h.Node {
		attrs := []h.Node{h.ID(id), h.Method("post"), h.Action("/fluxos/itens/novo"), trilha.CSRFInput(c),
			h.Input(h.Type("hidden"), h.Name("modo"), h.Value(modo))}
		if swap {
			attrs = append(attrs, ui.Swap(id))
		}
		name := []h.Node{h.ID(id + "-name"), h.Name("name")}
		if invalid && !swap {
			name = append(name, ui.InvalidIf(map[string]string{"name": "required"}, "name"))
		}
		attrs = append(attrs, h.Input(name...), h.Button(h.Type("submit"), h.Text("Add")))
		return h.Form(attrs...)
	}
	return h.Div(ui.NavigateScript(c),
		h.Section(h.ID("regiao"), ui.Navigate(""),
			h.H1(h.Text("New item")),
			form("form-segue", "", true),
			form("form-recarrega", "reload", true),
			form("form-fora", "fora", true),
			form("form-simples", "", false)))
}
