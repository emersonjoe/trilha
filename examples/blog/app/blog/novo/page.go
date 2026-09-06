package novo

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/posts"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// entrada is the form. What is always true about a post lives in the tag; a
// rule that depended on the rest of the app would live in Go, next to it.
type entrada struct {
	Titulo string `form:"titulo" validate:"required,min=3,max=80"`
	Corpo  string `form:"corpo" validate:"required"`
}

// Page renders the form at GET /blog/novo.
func Page(c *trilha.Ctx) (h.Node, error) {
	return formulario(c, entrada{}, nil), nil
}

// POST handles the form: on success, POST → redirect → GET; on error, the same
// page comes back with 422 and the messages beside the fields.
func POST(c *trilha.Ctx) error {
	var in entrada
	if err := c.Bind(&in); err != nil {
		errs, ok := err.(trilha.FieldErrors)
		if !ok {
			return err
		}
		return c.Render(http.StatusUnprocessableEntity, formulario(c, in, errs))
	}
	p := trilha.Use[*posts.Store](c).Create(in.Titulo, in.Corpo)
	return c.Redirect("/blog/" + p.Slug)
}

func formulario(c *trilha.Ctx, in entrada, errs trilha.FieldErrors) h.Node {
	c.SetTitle("Novo post")
	return ui.Card(
		ui.CardHeader(h.H1(h.Class("ui-card-title"), h.Text("Novo post")), ui.CardDescription("POST → redirect → GET, com CSRF.")),
		ui.CardContent(h.Form(h.Method("post"), h.Action("/blog/novo"), h.Class("ui-stack"),
			trilha.CSRFInput(c),
			ui.Field("titulo", "Título",
				ui.Input(h.ID("titulo"), h.Name("titulo"), h.Required(), h.Autofocus(), h.Value(in.Titulo), ui.InvalidIf(errs, "titulo")),
				ui.Errors(errs, "titulo")),
			// Ilha: o trecho interativo da página. O servidor manda o
			// formulário pronto e os dados da ilha; o módulo em public/
			// assume no cliente. Sem script, o campo continua sendo um
			// campo — a contagem e a prévia é que não aparecem.
			c.Island("/ilha-editor.js", map[string]any{"palavrasPorMinuto": 200},
				h.Class("ui-stack"),
				ui.Field("corpo", "Texto",
					ui.Textarea(h.ID("corpo"), h.Name("corpo"), h.Rows("6"), ui.InvalidIf(errs, "corpo"), h.Text(in.Corpo)),
					ui.Help("Markdown não é interpretado neste exemplo."), ui.Errors(errs, "corpo")),
				h.P(h.Data("info", ""), h.Class("ui-muted"), h.Hidden()),
				h.Div(h.Data("previa", ""), h.Class("ui-prose"), h.Hidden()),
			),
			h.Div(ui.Submit(h.Text("Publicar")), h.Text(" "), ui.ButtonLink("/blog", ui.Ghost(), h.Text("Cancelar"))),
		)),
	)
}
