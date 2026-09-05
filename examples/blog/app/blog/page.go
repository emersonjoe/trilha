package blog

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/posts"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page lists posts at GET /blog.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Blog")
	all := posts.All()
	return ui.Stack(
		ui.H1(h.Text("Blog")),
		h.If(len(all) == 0, ui.Muted(h.Text("Nenhum post ainda."))),
		h.Ul(h.Class("posts"), h.Map(all, func(p posts.Post) h.Node {
			return h.Li(ui.Card(ui.CardHeader(
				h.A(h.Href("/blog/"+p.Slug), ui.CardTitle(p.Title)),
				ui.CardDescription(p.Created.Format("02/01/2006")),
			)))
		})),
	), nil
}
