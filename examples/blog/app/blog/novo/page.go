package novo

import (
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/posts"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders the form at GET /blog/novo.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Novo post")
	errMsg := c.Query("erro")
	var titleOpts []ui.FieldOpt
	var titleAttrs []h.Node
	if errMsg != "" {
		titleOpts = append(titleOpts, ui.Error(errMsg))
		titleAttrs = append(titleAttrs, ui.Invalid())
	}
	return ui.Card(
		ui.CardHeader(h.H1(h.Class("ui-card-title"), h.Text("Novo post")), ui.CardDescription("POST → redirect → GET, com CSRF.")),
		ui.CardContent(h.Form(h.Method("post"), h.Action("/blog/novo"), h.Class("ui-stack"),
			trilha.CSRFInput(c),
			ui.Field("titulo", "Título", ui.Input(append([]h.Node{h.ID("titulo"), h.Name("titulo"), h.Required(), h.Autofocus()}, titleAttrs...)...), titleOpts...),
			ui.Field("corpo", "Texto", ui.Textarea(h.ID("corpo"), h.Name("corpo"), h.Rows("6")), ui.Help("Markdown não é interpretado neste exemplo.")),
			h.Div(ui.Submit(h.Text("Publicar")), h.Text(" "), ui.ButtonLink("/blog", ui.Ghost(), h.Text("Cancelar"))),
		)),
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
