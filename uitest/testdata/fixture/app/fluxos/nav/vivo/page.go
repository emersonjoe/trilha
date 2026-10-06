// Package vivo is a page whose region brings its own scripts: an island,
// the live runtime with a deferred part, a script of the app and an inline
// one. A client navigation into it has to run the files and only the files
// (#290).
package vivo

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Live page")
	return h.Div(ui.NavigateScript(c),
		h.Section(h.ID("regiao"), ui.Navigate(""),
			h.H1(h.Text("Live page")),
			ui.LiveScript(c),
			h.Script(h.Src(c.Asset("/conta-execucoes.js")), h.Defer()),
			h.Script(trilha.NonceAttr(c), h.Raw(`window.__inlineRan = (window.__inlineRan || 0) + 1;`)),
			c.Island("/ilha-contador.js", map[string]int{"start": 41}, h.P(h.ID("contador"), h.Text("fallback"))),
			ui.Defer(c, "adiado", "/fluxos/nav/vivo/adiado"),
			h.A(h.ID("ir-vivo2"), h.Href("/fluxos/nav/vivo2"), h.Text("to the second live page")))), nil
}
