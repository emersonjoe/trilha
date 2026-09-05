package novo

import (
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/posts"
	"github.com/emersonjoe/trilha/h"
)

// Page renders the form at GET /blog/novo.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Novo post")
	errMsg := c.Query("erro")
	return h.Fragment(
		h.H1(h.Text("Novo post")),
		h.If(errMsg != "", h.P(h.Class("erro"), h.Text(errMsg))),
		h.Form(h.Method("post"), h.Action("/blog/novo"),
			trilha.CSRFInput(c),
			h.Label(h.For("titulo"), h.Text("Título")),
			h.Input(h.ID("titulo"), h.Name("titulo"), h.Required(), h.Autofocus()),
			h.Label(h.For("corpo"), h.Text("Texto")),
			h.Textarea(h.ID("corpo"), h.Name("corpo"), h.Rows("6")),
			h.Button(h.Type("submit"), h.Text("Publicar")),
		),
	), nil
}

// POST handles the form (POST → redirect → GET).
func POST(c *trilha.Ctx) error {
	if err := c.FormErr(); err != nil {
		return err
	}
	title := strings.TrimSpace(c.Form("titulo"))
	if title == "" {
		return c.Redirect("/blog/novo?erro=" + "T%C3%ADtulo+obrigat%C3%B3rio")
	}
	p := posts.Create(title, c.Form("corpo"))
	return c.Redirect("/blog/" + p.Slug)
}
