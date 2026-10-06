// Package pre is a region with prefetch on intent (#295): a plain link, one
// with a short TTL, one kept out, and one whose route redirects.
package pre

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Prefetch")
	return h.Div(ui.NavigateScript(c),
		h.Section(h.ID("regiao"), ui.Navigate(""), ui.Prefetch(),
			h.H1(h.Text("Prefetch")),
			h.P(h.ID("longe"), h.Text("somewhere else to rest the pointer")),
			h.A(h.ID("alvo"), h.Href("/fluxos/pre/alvo"), h.Text("Target")),
			h.A(h.ID("curto"), h.Href("/fluxos/pre/curto"), ui.PrefetchTTL(300), h.Text("Short")),
			h.A(h.ID("fora"), h.Href("/fluxos/pre/fora"), ui.NoPrefetch(), h.Text("Kept out")),
			h.A(h.ID("sai"), h.Href("/fluxos/pre/sai"), h.Text("Moved")))), nil
}
