// Package assistenterevisao is step three: read it all, then commit.
package assistenterevisao

import (
	"github.com/emersonjoe/trilha"
	passos "github.com/emersonjoe/trilha/examples/cadastro/app/assistente"
	"github.com/emersonjoe/trilha/examples/cadastro/internal/assistente"
	"github.com/emersonjoe/trilha/examples/cadastro/internal/clientes"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	var r assistente.Rascunho
	if err := c.Draft(assistente.Nome).Load(&r); err != nil {
		return nil, c.Redirect("/assistente/dados")
	}
	c.SetTitle("Revisão")
	return h.Div(
		ui.Steps(passos.Passos, 3),
		ui.H1(h.Text("Confere?")),
		ui.Table(h.Tbody(
			linha("Nome", r.Dados.Nome),
			linha("E-mail", r.Dados.Email),
			linha("CEP", r.Endereco.CEP),
			linha("Cidade", r.Endereco.Cidade),
		)),
		h.Form(h.Method("post"), h.Class("ui-stack"), trilha.CSRFInput(c),
			ui.Submit(h.Text("Cadastrar"))),
	), nil
}

// POST is where the draft becomes a record and stops existing. Clear before
// the redirect, not after: the next visit to step two must start over instead
// of resuming something that already happened.
func POST(c *trilha.Ctx) error {
	var r assistente.Rascunho
	if err := c.Draft(assistente.Nome).Load(&r); err != nil {
		return c.Redirect("/assistente/dados")
	}
	clientes.Salvar(clientes.Cliente{
		Tipo:     "pf",
		Nome:     r.Dados.Nome,
		Email:    r.Dados.Email,
		Endereco: clientes.Endereco{CEP: r.Endereco.CEP, Cidade: r.Endereco.Cidade},
	})
	c.Draft(assistente.Nome).Clear()
	// ?ok=1 é a convenção que este exemplo já usa para o aviso da página
	// seguinte; o assistente não inventa uma segunda.
	return c.Redirect("/?ok=1")
}

func linha(rotulo, valor string) h.Node {
	return h.Tr(h.Th(h.Text(rotulo)), h.Td(h.Text(valor)))
}
