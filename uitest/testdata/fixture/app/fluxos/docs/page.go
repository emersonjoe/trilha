// Package docs is a list in a client-navigation region with a reading panel
// beside it, outside the region, and a dialog that loads its body (#296).
package docs

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Documents")
	return Frame(c, "Documents",
		h.A(h.ID("ir-outra"), h.Href("/fluxos/docs/outra"), h.Text("Other list"))), nil
}

// Frame is what both lists share: the region with the links, and outside it
// the panel and the dialog, which survive a client navigation.
func Frame(c *trilha.Ctx, title string, extra ...h.Node) h.Node {
	return h.Div(ui.NavigateScript(c), ui.SheetScript(c),
		h.Section(h.ID("regiao"), ui.Navigate(""),
			h.H1(h.Text(title)),
			h.A(h.ID("doc-7"), h.Href("/fluxos/docs/7"), ui.SheetOpen("leitor"), h.Text("Document 7")),
			h.A(h.ID("doc-8"), h.Href("/fluxos/docs/8"), ui.SheetOpen("leitor"), h.Text("Document 8")),
			ui.DialogTrigger("ver", ui.Swap("ver-body"), h.ID("ver-7"), h.Href("/fluxos/docs/7"), h.Text("See 7")),
			h.Fragment(extra...)),
		ui.Sheet(c, "leitor", ui.SheetOpts{Title: "Reader"}),
		ui.Dialog("ver", "Document", h.Div(h.ID("ver-body"))))
}
