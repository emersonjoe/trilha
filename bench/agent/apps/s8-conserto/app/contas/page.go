// Package contas is the screen of the register: a form, the mistakes the
// person made back beside the fields, and the list of what is already there.
package contas

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// entrada is the form. What is always true about a conta lives in the tag.
type entrada struct {
	Nome  string `form:"nome" validate:"required,max=80"`
	Email string `form:"email" validate:"obrigatorio,email"`
}

// Page renders GET /contas: the form and the list.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Contas")
	return ui.Stack(
		ui.PageHeader("Contas"),
		formulario(c, entrada{}, nil),
		ui.Card(
			ui.CardHeader(ui.CardTitle("Cadastradas")),
			ui.CardContent(contasList(trilha.Use[*Store](c).Todas())),
		),
	), nil
}

// POST saves a conta: on error the same page comes back with 422 and the
// messages beside the fields; on success, POST → redirect → GET.
func POST(c *trilha.Ctx) error {
	var in entrada
	if err := c.Bind(&in); err != nil {
		errs, ok := err.(trilha.FieldErrors)
		if !ok {
			return err
		}
		return c.Render(http.StatusUnprocessableEntity, formulario(c, in, errs))
	}
	trilha.Use[*Store](c).Nova(Conta{Nome: in.Nome, Email: in.Email})
	return c.Redirect("/contas")
}

func formulario(c *trilha.Ctx, in entrada, errs trilha.FieldErrors) h.Node {
	return ui.Card(
		ui.CardHeader(ui.CardTitle("Nova conta")),
		ui.CardContent(h.Form(h.Method("post"), h.Action("/contas"), h.Class("ui-stack"),
			ui.Field("nome", "Nome",
				ui.Input(h.ID("nome"), h.Name("nome"), h.Value(in.Nome), ui.InvalidIf(errs, "nome")),
				ui.Errors(errs, "nome")),
			ui.Field("email", "E-mail",
				ui.Input(h.ID("email"), h.Name("email"), h.Type("email"), h.Value(in.Email), ui.InvalidIf(errs, "email")),
				ui.Errors(errs, "email")),
			h.Div(ui.Submit(h.Text("Cadastrar"))),
		)),
	)
}

func contasList(rows []Conta) h.Node {
	if len(rows) == 0 {
		return ui.Lead(h.Text("Nenhuma conta cadastrada."))
	}
	itens := make([]h.Node, 0, len(rows))
	for _, r := range rows {
		itens = append(itens, h.Li(h.Text(r.Nome+" — "+r.Email)))
	}
	return h.Ul(itens...)
}
