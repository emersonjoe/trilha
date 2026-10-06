// Package nav is one side of the client navigation of the fixture, and the
// door to the pages that test what a navigation carries in.
package nav

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("First page")
	return h.Div(ui.NavigateScript(c),
		h.Section(h.ID("regiao"), ui.Navigate(""),
			h.H1(h.Text("Page A")),
			h.A(h.ID("ir"), h.Href("/fluxos/nav/b"), h.Text("to B")),
			h.A(h.ID("ir-vivo"), h.Href("/fluxos/nav/vivo"), h.Text("to the live page")),
			h.A(h.ID("ir-foco"), h.Href("/fluxos/nav/foco"), h.Text("to the region page")))), nil
}
