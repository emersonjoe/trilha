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
	all, err := trilha.Use[*posts.Store](c).Cached(c.Context())
	if err != nil {
		return nil, err
	}
	return ui.Stack(
		ui.H1(h.Text("Blog")),
		// The empty state is a component so that thirty-five screens do not
		// each invent one — and so that it says what to do next, which a bare
		// "nothing here" never does.
		h.If(len(all) == 0, ui.Empty(ui.EmptyOpts{
			Icon:   "info",
			Title:  "Nenhum post ainda",
			Hint:   "Escreva o primeiro e ele aparece aqui.",
			Action: ui.ButtonLink("/blog/novo", h.Text("Escrever post")),
		})),
		h.Ul(h.Class("posts"), h.Map(all, func(p posts.Post) h.Node {
			return h.Li(ui.Card(ui.CardHeader(
				h.A(h.Href("/blog/"+p.Slug), ui.CardTitle(p.Title)),
				ui.CardDescription(p.Created.Format("02/01/2006")),
			)))
		})),
	), nil
}
