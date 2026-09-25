// Package nav is one side of the client navigation of the fixture.
package nav

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	return h.Div(ui.NavigateScript(c),
		h.Section(h.ID("regiao"), ui.Navigate(""),
			h.H1(h.Text("Page A")),
			h.A(h.ID("ir"), h.Href("/fluxos/nav/b"), h.Text("to B")))), nil
}
