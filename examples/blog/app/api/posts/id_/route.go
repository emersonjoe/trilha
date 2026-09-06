package id

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/posts"
)

// GET returns one post by slug.
//
// openapi:response 429
func GET(c *trilha.Ctx) error {
	p, ok := trilha.Use[*posts.Store](c).Get(c.Param("id"))
	if !ok {
		return trilha.ErrNotFound
	}
	return c.JSON(http.StatusOK, p)
}

// DELETE removes a post.
//
// openapi:response 429
func DELETE(c *trilha.Ctx) error {
	if !trilha.Use[*posts.Store](c).Delete(c.Param("id")) {
		return trilha.ErrNotFound
	}
	c.Writer().WriteHeader(http.StatusNoContent)
	return nil
}
