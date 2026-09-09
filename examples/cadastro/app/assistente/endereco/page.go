// Package assistenteendereco is step two: where the person is.
package assistenteendereco

import (
	"net/http"
	"time"

	"github.com/emersonjoe/trilha"
	passos "github.com/emersonjoe/trilha/examples/cadastro/app/assistente"
	"github.com/emersonjoe/trilha/examples/cadastro/internal/assistente"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page needs step one to have happened. Without a draft there is nothing to
// continue — expired, finished, or somebody who typed the address of a wizard
// they never started — and the answer is step one, not an empty form that
// would lose whatever they type here.
func Page(c *trilha.Ctx) (h.Node, error) {
	var r assistente.Rascunho
	if err := c.Draft(assistente.Nome).Load(&r); err != nil {
		return nil, c.Redirect("/assistente/dados")
	}
	c.SetTitle("Endereço")
	return tela(c, r, nil), nil
}

func POST(c *trilha.Ctx) error {
	var r assistente.Rascunho
	if err := c.Draft(assistente.Nome).Load(&r); err != nil {
		return c.Redirect("/assistente/dados")
	}
	if err := c.Bind(&r.Endereco); err != nil {
		errs, ok := err.(trilha.FieldErrors)
		if !ok {
			return err
		}
		// O passo 1 continua no rascunho: um 422 aqui não custa o que a pessoa
		// já digitou lá atrás.
		return c.Render(http.StatusUnprocessableEntity, tela(c, r, errs))
	}
	if err := c.Draft(assistente.Nome).Save(r, 30*time.Minute); err != nil {
		return err
	}
	return c.Redirect("/assistente/revisao")
}

func tela(c *trilha.Ctx, r assistente.Rascunho, errs trilha.FieldErrors) h.Node {
	return h.Div(
		ui.Steps(passos.Passos, 2),
		ui.H1(h.Text("Endereço")),
		ui.Muted(h.Text(r.Dados.Nome+" · "+r.Dados.Email)),
		h.Form(h.Method("post"), h.Class("ui-stack"), h.Attr("novalidate", ""), trilha.CSRFInput(c),
			passos.Campo("cep", "CEP", r.Endereco.CEP, errs, h.Attr("inputmode", "numeric")),
			passos.Campo("cidade", "Cidade", r.Endereco.Cidade, errs),
			ui.Submit(h.Text("Continuar"))),
	)
}
