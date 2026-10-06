// Package vivo2 brings the same script file as the live page: navigated in
// sequence, the file runs once per document, not once per page.
package vivo2

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Second live page")
	return h.Div(ui.NavigateScript(c),
		h.Section(h.ID("regiao"), ui.Navigate(""),
			h.H1(h.Text("Second live page")),
			h.Script(h.Src(c.Asset("/conta-execucoes.js")), h.Defer()))), nil
}
