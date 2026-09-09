// Package assistentedados is step one: who is being registered.
package assistentedados

import (
	"net/http"
	"time"

	"github.com/emersonjoe/trilha"
	passos "github.com/emersonjoe/trilha/examples/cadastro/app/assistente"
	"github.com/emersonjoe/trilha/examples/cadastro/internal/assistente"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page shows the form with whatever the draft already holds — somebody who
// went back a step finds what they typed, which is the whole reason the draft
// exists.
func Page(c *trilha.Ctx) (h.Node, error) {
	var r assistente.Rascunho
	_ = c.Draft(assistente.Nome).Load(&r) // sem rascunho é a primeira visita
	c.SetTitle("Novo cadastro")
	return tela(c, r.Dados, nil), nil
}

// POST validates this step alone and saves it. Nothing else in the wizard is
// checked here: the address has not been typed yet, and a message about it
// would be a message about a screen nobody has seen.
func POST(c *trilha.Ctx) error {
	var r assistente.Rascunho
	_ = c.Draft(assistente.Nome).Load(&r)
	if err := c.Bind(&r.Dados); err != nil {
		errs, ok := err.(trilha.FieldErrors)
		if !ok {
			return err
		}
		return c.Render(http.StatusUnprocessableEntity, tela(c, r.Dados, errs))
	}
	if err := c.Draft(assistente.Nome).Save(r, 30*time.Minute); err != nil {
		return err
	}
	return c.Redirect("/assistente/endereco")
}

func tela(c *trilha.Ctx, d assistente.Passo1, errs trilha.FieldErrors) h.Node {
	return h.Div(
		ui.Steps(passos.Passos, 1),
		ui.H1(h.Text("Novo cadastro")),
		h.Form(h.Method("post"), h.Class("ui-stack"), h.Attr("novalidate", ""), trilha.CSRFInput(c),
			passos.Campo("nome", "Nome", d.Nome, errs),
			passos.Campo("email", "E-mail", d.Email, errs, h.Type("email")),
			ui.Submit(h.Text("Continuar"))),
	)
}
