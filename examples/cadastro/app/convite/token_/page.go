// Package convite is the flow that happens with no login: somebody outside
// receives a link and fills in one form with it.
package convite

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/cadastro/internal/clientes"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// dados is what the invited person fills in — their own, and nothing about the
// client that invited them: the link says which client that is, signed, and a
// field would be a field somebody could change.
type dados struct {
	Nome       string `form:"nome"       validate:"required,max=80"`
	Nascimento string `form:"nascimento" validate:"required"`
}

// Page opens the form. There is no session here and there is not supposed to
// be one: the link is the credential, and it says what it is for.
func Page(c *trilha.Ctx) (h.Node, error) {
	link, err := c.Claim("convite")
	if err != nil {
		return nil, err
	}
	c.SetTitle("Convite")
	return tela(c, link, dados{}, nil), nil
}

// POST records the dependant and spends the link. Consume comes after the work
// and not before: a link burned by a validation error is a link somebody has
// to ask for again because they typed a date wrong.
func POST(c *trilha.Ctx) error {
	link, err := c.Claim("convite")
	if err != nil {
		return err
	}
	var d dados
	if err := c.Bind(&d); err != nil {
		errs, ok := err.(trilha.FieldErrors)
		if !ok {
			return err
		}
		return c.Render(http.StatusUnprocessableEntity, tela(c, link, d, errs))
	}
	if err := clientes.AddDependente(link.Data["cliente"], clientes.Dependente{
		Nome: d.Nome, Nascimento: d.Nascimento,
	}); err != nil {
		return err
	}
	if err := link.Consume(); err != nil {
		return err
	}
	return c.Render(http.StatusOK, h.Div(
		ui.H1(h.Text("Pronto")),
		ui.Muted(h.Text("Recebemos os dados. Este link não vale mais."))))
}

func tela(c *trilha.Ctx, link *trilha.Link, d dados, errs trilha.FieldErrors) h.Node {
	return h.Div(
		ui.H1(h.Text("Complete seus dados")),
		ui.Muted(h.Text("Convite de "+link.Data["quem"]+" · vale uma vez.")),
		h.Form(h.Method("post"), h.Class("ui-stack"), h.Attr("novalidate", ""), trilha.CSRFInput(c),
			ui.Field("nome", "Nome", ui.Input(h.ID("nome"), h.Name("nome"), h.Value(d.Nome), ui.InvalidIf(errs, "nome")), ui.Errors(errs, "nome")),
			ui.Field("nascimento", "Nascimento", ui.Input(h.ID("nascimento"), h.Name("nascimento"), h.Type("date"), h.Value(d.Nascimento), ui.InvalidIf(errs, "nascimento")), ui.Errors(errs, "nascimento")),
			ui.Submit(h.Text("Enviar"))),
	)
}
