// Package id_ is one document: the panel's body, the dialog's body, or the
// whole page without JavaScript — the same route, three answers.
package id_

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"

	"example.com/uitest/internal/conta"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	id := c.Param("id")
	conta.Hit(c)
	switch c.Fragment() {
	case "leitor-body":
		return ui.SheetBody("leitor", leitor(c, id)), nil
	case "ver-body":
		return h.Div(h.ID("ver-body"), h.Text("Document "+id+" in a dialog")), nil
	}
	c.SetTitle("Document " + id)
	return h.Div(h.H1(h.Text("Document "+id)), leitor(c, id)), nil
}

func leitor(c *trilha.Ctx, id string) h.Node {
	return h.Div(h.H3(h.Class("doc"), h.Text("Document "+id)),
		ui.Preview(c, "/fluxos/docs/"+id+"/arquivo", ui.PreviewOpts{Title: "doc-" + id + ".txt", Type: "text/plain", Height: "10rem"}))
}
