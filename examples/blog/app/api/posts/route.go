// Package posts exposes the JSON API at /api/posts.
package posts

import (
	"net/http"
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/posts"
)

// GET lists posts.
func GET(c *trilha.Ctx) error {
	return c.JSON(http.StatusOK, posts.All())
}

// POST creates a post from JSON {"title": "...", "body": "..."}.
func POST(c *trilha.Ctx) error {
	var in struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := c.BindJSON(&in); err != nil {
		return err
	}
	if strings.TrimSpace(in.Title) == "" {
		return trilha.Errorf(http.StatusUnprocessableEntity, "title is required")
	}
	p := posts.Create(in.Title, in.Body)
	c.Header("Location", "/api/posts/"+p.Slug)
	return c.JSON(http.StatusCreated, p)
}
