// Package listwithfilter is the list-with-filter pattern: a sortable table, a
// search box and one filter, paged — all of it in the address.
package listwithfilter

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Order is one row. Replace it with your type.
type Order struct {
	ID, Customer, Status string
	Total                float64
}

// Query is what the address says: the listing plus this screen's filter.
type Query struct {
	trilha.ListParams
	Status string `form:"status"`
}

// Orders is where the rows come from — your store, filtered, ordered and
// paged. It answers one page and the total.
var Orders = func(c *trilha.Ctx, q Query) ([]Order, int, error) { return nil, 0, nil }

var columns = ui.Columns[Order]{
	{Key: "customer", Label: "Customer", Sort: true, Cell: func(o Order) h.Node { return h.Text(o.Customer) }},
	{Key: "status", Label: "Status", Cell: func(o Order) h.Node { return ui.Badge(ui.Outline(), h.Text(o.Status)) }},
	{Key: "total", Label: "Total", Sort: true, Num: true, Cell: func(o Order) h.Node { return h.Textf("%.2f", o.Total) }},
}

// Page answers the whole screen, or only the table when the kit asks for it.
func Page(c *trilha.Ctx) (h.Node, error) {
	var q Query
	if err := c.Bind(&q); err != nil {
		return nil, err
	}
	if q.Status != "open" && q.Status != "paid" {
		q.Status = "" // only a value this screen offers reaches the store
	}
	rows, total, err := Orders(c, q)
	if err != nil {
		return nil, err
	}
	status := ui.Select(h.Name("status"), h.Aria("label", "Status"), ui.SelectOptions([]ui.Option{
		{Value: "", Label: "All"}, {Value: "open", Label: "Open"}, {Value: "paid", Label: "Paid"}}, q.Status))
	table := ui.DataTable(c, columns, rows, ui.ListState{Params: q.ListParams, Total: total, ID: "orders",
		Search: "Search customers", Filters: status, Caption: "Orders", Cards: true,
		Empty: ui.Empty(ui.EmptyOpts{Title: "No orders match"})})
	if c.Fragment() == "orders" {
		return table, nil
	}
	return ui.Stack(ui.PageHeader("Orders"), table), nil
}
