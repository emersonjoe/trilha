// Package items serves the item collection.
package items

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/webhook"
	"example.com/openapi/internal/store"
)

// Hooks sends what happens here to whoever subscribed.
var Hooks = webhook.New(webhook.Options{})

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
	// Whoever subscribed hears about it. The event name is written here, which
	// is why the document can name it.
	Hooks.Emit(c, "item.criado", it)
	// And this one cannot be documented: a name that only exists at run time
	// is not a contract.
	evento := "item." + in.Kind
	Hooks.Emit(c, evento, it)
	return c.JSON(http.StatusCreated, it)
}
