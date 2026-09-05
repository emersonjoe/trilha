package slug

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/posts"
	"github.com/emersonjoe/trilha/h"
)

// Page renders one post at GET /blog/{slug}.
func Page(c *trilha.Ctx) (h.Node, error) {
	p, ok := posts.Get(c.Param("slug"))
	if !ok {
		return nil, trilha.ErrNotFound
	}
	if p.Slug == "boom" {
		return nil, trilha.Errorf(500, "post explodiu")
	}
	c.SetTitle(p.Title)
	return h.Article(
		h.H1(h.Text(p.Title)),
		h.P(h.Text(p.Body)),
		h.Form(h.Method("post"), h.Action("/blog/"+p.Slug), trilha.CSRFInput(c),
			h.Button(h.Type("submit"), h.Text("Apagar"))),
	), nil
}

// DELETE is exposed for API clients; the form above uses POST with _method.
func DELETE(c *trilha.Ctx) error {
	if !posts.Delete(c.Param("slug")) {
		return trilha.ErrNotFound
	}
	return c.Redirect("/blog")
}

// POST handles the delete form (browsers only send GET/POST).
func POST(c *trilha.Ctx) error {
	return DELETE(c)
}
