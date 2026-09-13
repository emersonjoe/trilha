// Package inscricao is the workshop sign-up under the organiser's own
// address: GET /oficinas/{slug}/inscricao.
//
// It is here for what it overlaps with. /{secao}/inscricao/{id} matches
// "/oficinas/inscricao/inscricao" too, and neither pattern is a specialisation
// of the other — the pair the Go mux refuses to register. Trilha reads the
// segments left to right, so the literal "oficinas" wins the first position
// and this page answers (spec 148).
package inscricao

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page shows the form of one workshop.
func Page(c *trilha.Ctx) (h.Node, error) {
	oficina := c.Param("slug")
	c.SetTitle("Inscrição: " + oficina)
	return h.Article(h.Class("ui-stack"),
		ui.Breadcrumb(ui.Crumb{Label: "Oficinas"}, ui.Crumb{Label: oficina}),
		h.H1(h.Class("ui-h1"), h.Text("Inscrição na oficina "+oficina)),
		h.P(h.Text("A oficina é "+oficina+", e o endereço é o da organizadora.")),
	), nil
}
