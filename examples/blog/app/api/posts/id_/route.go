package id

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/posts"
)

// GET returns one post by slug.
func GET(c *trilha.Ctx) error {
	p, ok := posts.Get(c.Param("id"))
	if !ok {
		return trilha.ErrNotFound
	}
	return c.JSON(http.StatusOK, p)
}

// DELETE removes a post.
func DELETE(c *trilha.Ctx) error {
	if !posts.Delete(c.Param("id")) {
		return trilha.ErrNotFound
	}
	c.Writer().WriteHeader(http.StatusNoContent)
	return nil
}
