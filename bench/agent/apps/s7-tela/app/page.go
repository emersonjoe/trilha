package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders GET /: the welcome screen of an empty project.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Início")
	return ui.Stack(
		ui.PageHeader("Início"),
		ui.Card(ui.CardContent(
			ui.Lead(h.Text("Um app vazio: a primeira feature começa daqui.")),
		)),
	), nil
}
