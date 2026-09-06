// Package tela is a new screen inside the old shell: the page is written in h
// and the layout above it comes from an html/template file.
package tela

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders GET /legado.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Área migrada")
	return ui.Card(
		ui.CardHeader(h.H1(h.Class("ui-card-title"), h.Text("Tela nova, casca velha"))),
		h.Form(h.Method("post"), h.Action("/legado"), trilha.CSRFInput(c),
			ui.Field("nome", "Nome", ui.Input(h.ID("nome"), h.Name("nome"), h.Maxlength("30"), h.Autocomplete("name"))),
			ui.Submit(h.Text("Salvar"))),
	), nil
}

// POST answers the form of the page above.
func POST(c *trilha.Ctx) error {
	c.Flash(ui.FlashSuccess, "Salvo "+c.Form("nome"))
	return c.Redirect("/legado")
}
