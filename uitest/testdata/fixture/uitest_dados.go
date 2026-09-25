package main

import (
	"fmt"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/patterns/listwithfilter"
)

// The patterns of spec 163 read their rows from package vars; the fixture
// answers them with fixed data, enough rows for three pages.
func init() {
	listwithfilter.Orders = func(c *trilha.Ctx, q listwithfilter.Query) ([]listwithfilter.Order, int, error) {
		const total = 45
		per := q.Limit()
		var rows []listwithfilter.Order
		for i := q.Offset(); i < total && len(rows) < per; i++ {
			rows = append(rows, listwithfilter.Order{ID: fmt.Sprintf("o-%d", i+1),
				Customer: fmt.Sprintf("Customer %02d", i+1), Status: "open", Total: float64(i + 1)})
		}
		return rows, total, nil
	}
}
