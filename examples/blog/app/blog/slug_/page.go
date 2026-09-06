package slug

import (
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/posts"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders one post at GET /blog/{slug}.
func Page(c *trilha.Ctx) (h.Node, error) {
	p, ok := trilha.Use[*posts.Store](c).Get(c.Param("slug"))
	if !ok {
		return nil, trilha.ErrNotFound
	}
	if p.Slug == "boom" {
		return nil, trilha.Errorf(500, "post explodiu")
	}
	// O post é a única coisa nesta página que muda, então a data dele é a
	// versão dela. Num app com edição seria o `updated_at`; o que não serve é
	// o corpo renderizado, que traz um nonce novo a cada resposta.
	c.CacheControl("private, no-cache")
	if c.ETag(p.Created.UTC().Format(time.RFC3339Nano)) {
		return nil, nil
	}
	c.SetTitle(p.Title)
	return h.Article(h.Class("ui-stack"),
		ui.Breadcrumb(ui.Crumb{Label: "Blog", Href: "/blog"}, ui.Crumb{Label: p.Title}),
		h.H1(h.Class("ui-h1"), h.Text(p.Title)),
		h.P(h.Text(p.Body)),
		ui.Row(
			// O diálogo de confirmação é do formulário: sem JavaScript ele
			// envia direto, com JavaScript o ui.js pergunta antes.
			h.Form(h.Method("post"), h.Action("/blog/"+p.Slug), trilha.CSRFInput(c),
				ui.Confirm("Apagar este post?", "Não dá para desfazer."),
				h.Data("ui-confirm-cancel", "Cancelar"),
				ui.Submit(ui.Destructive(), ui.Sm(), ui.Icon("trash"), h.Text("Apagar"))),
			ui.Badge(ui.Secondary(), h.Text(p.Created.Format("02/01/2006"))),
		),
	), nil
}

// DELETE is exposed for API clients; the form above uses POST.
func DELETE(c *trilha.Ctx) error {
	if !trilha.Use[*posts.Store](c).Delete(c.Param("slug")) {
		return trilha.ErrNotFound
	}
	// O redirect apaga a notícia; o flash é como ela chega na página seguinte.
	c.Flash(ui.FlashSuccess, "Post apagado")
	return c.Redirect("/blog")
}

// POST handles the delete form (browsers only send GET/POST).
func POST(c *trilha.Ctx) error {
	return DELETE(c)
}
