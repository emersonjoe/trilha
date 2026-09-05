package blog

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/posts"
	"github.com/emersonjoe/trilha/h"
)

// Page lists posts at GET /blog.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Blog")
	all := posts.All()
	return h.Fragment(
		h.H1(h.Text("Blog")),
		h.If(len(all) == 0, h.P(h.Text("Nenhum post ainda."))),
		h.Ul(h.Class("posts"), h.Map(all, func(p posts.Post) h.Node {
			return h.Li(h.A(h.Href("/blog/"+p.Slug), h.Text(p.Title)),
				h.Text(" "), h.Time(h.Datetime(p.Created.Format("2006-01-02")), h.Text(p.Created.Format("02/01/2006"))))
		})),
	), nil
}
