// Package masterdetail is the master-detail pattern: the list on one side and
// the chosen row on the other, the choice in the address so it can be shared
// and survives a reload.
package masterdetail

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Customer is one row. Replace it with your type.
type Customer struct{ ID, Name, Email, Plan string }

// Query is the listing plus the chosen row.
type Query struct {
	trilha.ListParams
	ID string `form:"id"`
}

// Customers answers one page and the total; Find answers one row or
// trilha.ErrNotFound — both from your store.
var (
	Customers = func(c *trilha.Ctx, q trilha.ListParams) ([]Customer, int, error) { return nil, 0, nil }
	Find      = func(c *trilha.Ctx, id string) (Customer, error) { return Customer{}, trilha.ErrNotFound }
)

var columns = ui.Columns[Customer]{
	{Key: "name", Label: "Name", Sort: true, Cell: func(x Customer) h.Node { return h.Text(x.Name) }},
	{Key: "plan", Label: "Plan", Cell: func(x Customer) h.Node { return ui.Badge(ui.Outline(), h.Text(x.Plan)) }},
}

// Page renders the list and, when the address names one, its detail.
func Page(c *trilha.Ctx) (h.Node, error) {
	var q Query
	if err := c.Bind(&q); err != nil {
		return nil, err
	}
	rows, total, err := Customers(c, q.ListParams)
	if err != nil {
		return nil, err
	}
	list := ui.DataTable(c, columns, rows, ui.ListState{Params: q.ListParams, Total: total, Caption: "Customers",
		RowHref: func(i int) string { return q.Href("id", rows[i].ID) }})
	var detail h.Node = ui.Empty(ui.EmptyOpts{Title: "Pick a customer to see the details"})
	if q.ID != "" {
		x, err := Find(c, q.ID)
		if err != nil {
			return nil, err
		}
		detail = ui.Card(ui.CardHeader(ui.CardTitle(x.Name), ui.CardDescription(x.Email)),
			ui.CardContent(ui.Badge(h.Text(x.Plan))))
	}
	return ui.Stack(ui.PageHeader("Customers"),
		ui.Grid(ui.Cols(1, 2), list, h.Section(h.Aria("label", "Details"), detail))), nil
}
