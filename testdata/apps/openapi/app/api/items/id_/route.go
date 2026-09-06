// Package id serves one item.
package id

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"example.com/openapi/internal/store"
)

// GET returns one item by id.
func GET(c *trilha.Ctx) error {
	it, ok := store.Get(c.Param("id"))
	if !ok {
		return trilha.ErrNotFound
	}
	return c.JSON(http.StatusOK, it)
}

// PUT replaces an item. Nothing here is deducible, so the comment says it.
//
// openapi:body store.Item
// openapi:response 200 store.Item
// openapi:response 409
func PUT(c *trilha.Ctx) error {
	return replace(c)
}

// DELETE removes an item and answers with no body.
func DELETE(c *trilha.Ctx) error {
	if !store.Delete(c.Param("id")) {
		return trilha.ErrNotFound
	}
	c.Writer().WriteHeader(http.StatusNoContent)
	return nil
}

func replace(c *trilha.Ctx) error { return nil }
