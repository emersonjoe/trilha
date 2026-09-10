// Package orders serves an order and the rows inside it.
package orders

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"example.com/openapi/app/api/items"
)

// Row is one line of the order.
type Row struct {
	Name string `form:"name" json:"name" validate:"required,max=40"`
	Qty  int    `form:"qty" json:"qty" validate:"min=1"`
}

// POST creates an order. The rows arrive as items[0].name, items[1].name…, and
// the permissions as perm[docs]: a list and a map, described as such.
func POST(c *trilha.Ctx) error {
	var in struct {
		Items []Row          `form:"items" json:"items" validate:"minitems=1,maxitems=50"`
		Perm  map[string]int `form:"perm" json:"perm"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	items.Hooks.Emit(c, "pedido.criado", in)
	return c.JSON(http.StatusCreated, in)
}
