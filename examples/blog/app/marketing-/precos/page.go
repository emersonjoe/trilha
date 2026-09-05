package precos

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders GET /precos (inside the marketing group layout).
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Preços")
	plan := func(name, price, desc string, primary bool) h.Node {
		btn := ui.Button(ui.Outline(), h.Text("Escolher"))
		if primary {
			btn = ui.Button(h.Text("Escolher"))
		}
		return ui.Card(ui.CardHeader(ui.CardTitle(name), ui.CardDescription(desc)),
			ui.CardContent(h.Span(h.Class("ui-h2"), h.Text(price))), ui.CardFooter(btn))
	}
	return ui.Stack(h.H1(h.Class("ui-h1"), h.Text("Preços")), h.P(h.Text("Grátis. É um exemplo.")),
		ui.Grid(plan("Hobby", "R$ 0", "Para aprender", false), plan("Pro", "R$ 0", "Também grátis", true), plan("Time", "R$ 0", "Idem", false))), nil
}
