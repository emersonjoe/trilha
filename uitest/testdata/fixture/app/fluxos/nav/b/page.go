// Package b is one side of the client navigation of the fixture. Its title
// is not its heading, so the route announcer has something to say that the
// focus on the heading does not.
package b

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Second page")
	return h.Div(ui.NavigateScript(c),
		h.Section(h.ID("regiao"), ui.Navigate(""),
			h.H1(h.Text("Page B")),
			h.A(h.ID("ir"), h.Href("/fluxos/nav"), h.Text("to A")))), nil
}
