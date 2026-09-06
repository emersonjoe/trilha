// Package entrar is the login screen: a form, a check against the users table
// and the session.
package entrar

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/local-login/internal/sessao"
	"github.com/emersonjoe/trilha/examples/local-login/internal/usuarios"
	"github.com/emersonjoe/trilha/h"
)

// Page renders GET /entrar.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Entrar")
	return formulario(c, "", ""), nil
}

// POST checks the password and opens the session. The message is the same for
// a wrong e-mail and for a wrong password.
func POST(c *trilha.Ctx) error {
	email, senha := c.Form("email"), c.Form("senha")
	u, err := trilha.Use[*usuarios.Store](c).Verify(email, senha)
	if err != nil {
		return c.Render(422, formulario(c, email, "E-mail ou senha inválidos."))
	}
	return sessao.Entrar(c, u)
}

func formulario(c *trilha.Ctx, email, erro string) h.Node {
	return h.Div(h.Class("cartao"),
		h.H1(h.Text("Entrar")),
		h.If(erro != "", h.P(h.Class("erro"), h.Text(erro))),
		h.Form(h.Method("post"), h.Action("/entrar"), trilha.CSRFInput(c),
			h.Label(h.For("email"), h.Text("E-mail")),
			h.Input(h.ID("email"), h.Name("email"), h.Type("email"), h.Value(email), h.Autocomplete("username"), h.Required()),
			h.Label(h.For("senha"), h.Text("Senha")),
			h.Input(h.ID("senha"), h.Name("senha"), h.Type("password"), h.Autocomplete("current-password"), h.Required()),
			h.Button(h.Type("submit"), h.Class("botao"), h.Text("Entrar")),
		),
	)
}
