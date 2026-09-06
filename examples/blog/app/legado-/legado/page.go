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
		// The write of this action lives in app/legado-/legado/apagar/route.go,
		// one folder per action, as the old app had it. It only enforces CSRF
		// because app/legado-/kind.go says the branch is pages.
		h.Form(h.Method("post"), h.Action("/legado/apagar"), trilha.CSRFInput(c),
			ui.Confirm("Apagar o registro?", "Não dá para desfazer."),
			h.Data("ui-confirm-cancel", "Cancelar"),
			ui.Submit(ui.Destructive(), ui.Sm(), h.Text("Apagar"))),
	), nil
}

// POST answers the form of the page above.
func POST(c *trilha.Ctx) error {
	c.Flash(ui.FlashSuccess, "Salvo "+c.Form("nome"))
	return c.Redirect("/legado")
}
