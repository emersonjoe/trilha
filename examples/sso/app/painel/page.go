package painel

import (
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/sso/internal/sso"
	"github.com/emersonjoe/trilha/h"
)

// Page mostra o que veio do provedor. Nada aqui checa sessão: o middleware
// já garantiu que existe uma.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Painel")
	u := sso.User(c)
	papeis := strings.Join(u.Roles, ", ")
	if papeis == "" {
		papeis = "nenhum"
	}
	return h.Div(h.Class("cartao"),
		h.H1(h.Text("Painel")),
		h.Dl(
			h.Dt(h.Text("Identificador")), h.Dd(h.Code(h.Text(u.Subject))),
			h.Dt(h.Text("Nome")), h.Dd(h.Text(u.Name)),
			h.Dt(h.Text("E-mail")), h.Dd(h.Text(u.Email)),
			h.Dt(h.Text("Papéis")), h.Dd(h.Text(papeis)),
			h.Dt(h.Text("Expira em")), h.Dd(h.Text(u.ExpiresAt.Format("02/01/2006 15:04"))),
		),
	), nil
}
