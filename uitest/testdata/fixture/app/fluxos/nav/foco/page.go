// Package foco asks the client navigation to keep the focus on the region,
// as before #297, instead of moving it to the heading.
package foco

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Region focus")
	return h.Div(ui.NavigateScript(c),
		h.Section(h.ID("regiao"), ui.Navigate(""), ui.NavigateFocus("region"),
			h.H1(h.Text("Region focus")))), nil
}
