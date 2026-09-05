package slug

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/posts"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
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
	return h.Article(h.Class("ui-stack"),
		ui.Breadcrumb(ui.Crumb{Label: "Blog", Href: "/blog"}, ui.Crumb{Label: p.Title}),
		h.H1(h.Class("ui-h1"), h.Text(p.Title)),
		h.P(h.Text(p.Body)),
		ui.Row(
			ui.DialogTrigger("apagar", ui.Destructive(), ui.Sm(), ui.Icon("trash"), h.Text("Apagar")),
			ui.Badge(ui.Secondary(), h.Text(p.Created.Format("02/01/2006"))),
		),
		ui.Dialog("apagar", "Apagar este post?",
			ui.DialogDescription("Não dá para desfazer."),
			h.Form(h.Method("post"), h.Action("/blog/"+p.Slug), trilha.CSRFInput(c),
				ui.DialogFooter(ui.DialogClose(ui.Outline(), h.Text("Cancelar")), ui.Submit(ui.Destructive(), h.Text("Apagar")))),
		),
	), nil
}

// DELETE is exposed for API clients; the form above uses POST.
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
