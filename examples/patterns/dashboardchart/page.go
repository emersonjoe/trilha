// Package dashboardchart is the dashboard-chart pattern: the numbers on top,
// the drawings beside them — SVG from the server, no chart library, readable
// without JavaScript.
package dashboardchart

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Numbers is what the dashboard shows. The strings are already formatted:
// the store counts, the page does not.
type Numbers struct {
	Open, Paid string
	ByType     []ui.Datum
	Weekly     []float64
}

// Load computes the numbers — your store, one query per card at most.
var Load = func(c *trilha.Ctx) (Numbers, error) { return Numbers{}, nil }

// Page renders the dashboard.
func Page(c *trilha.Ctx) (h.Node, error) {
	n, err := Load(c)
	if err != nil {
		return nil, err
	}
	return ui.Stack(
		ui.PageHeader("Dashboard"),
		ui.Grid(ui.Cols(1, 2),
			ui.Card(ui.CardContent(ui.Stat("Open orders", n.Open,
				ui.SparklineTitle(n.Weekly, ui.SparkOpts{Width: 160}, ui.ChartTitle("Open orders, last 7 days"))))),
			ui.Card(ui.CardContent(ui.Stat("Paid this month", n.Paid))),
		),
		ui.Grid(ui.Cols(1, 2),
			ui.Card(ui.CardHeader(ui.CardTitle("Orders by type")),
				ui.CardContent(ui.Bars(n.ByType, ui.ChartTitle("Orders by type")))),
			ui.Card(ui.CardHeader(ui.CardTitle("Share by type")),
				ui.CardContent(ui.Donut(n.ByType, ui.ChartTitle("Share by type")))),
		),
	), nil
}
