// Package id_ is one sign-up seen from the section it belongs to:
// GET /{secao}/inscricao/{id}.
//
// The pair with /oficinas/{slug}/inscricao is the one the Go mux turns down:
// both match "/oficinas/inscricao/inscricao" and neither is more specific by
// the mux's rule. This one answers everything whose middle segment is the
// literal "inscricao" — "/eventos/inscricao/42" — and gives the other one the
// paths that start with "oficinas" (spec 148).
package id_

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page shows the sign-up receipt.
func Page(c *trilha.Ctx) (h.Node, error) {
	secao, id := c.Param("secao"), c.Param("id")
	c.SetTitle("Inscrição " + id)
	return h.Article(h.Class("ui-stack"),
		ui.Breadcrumb(ui.Crumb{Label: secao}, ui.Crumb{Label: "Inscrição " + id}),
		h.H1(h.Class("ui-h1"), h.Text("Inscrição "+id)),
		h.P(h.Text("Seção "+secao+", inscrição "+id+".")),
	), nil
}
