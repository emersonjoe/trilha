// Package items serves the item collection.
package items

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"example.com/openapi/internal/store"
)

// GET lists every item.
//
// openapi:query q string  filter by name
func GET(c *trilha.Ctx) error {
	return c.JSON(http.StatusOK, store.All())
}

// POST creates an item. The body is the same struct the form uses, so the
// schema cannot drift from the validation.
func POST(c *trilha.Ctx) error {
	var in struct {
		Name string `json:"name" validate:"required,max=40"`
		Kind string `json:"kind" validate:"oneof=book tool"`
	}
	if err := c.BindJSON(&in); err != nil {
		return err
	}
	it := store.Create(in.Name)
	return c.JSON(http.StatusCreated, it)
}
