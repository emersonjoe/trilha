package painel

import (
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/local-login/internal/sessao"
	"github.com/emersonjoe/trilha/h"
)

// Page shows the session and what it carries for the API.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Painel")
	u := sessao.Flow.User(c)
	return h.Div(h.Class("cartao"),
		h.H1(h.Text("Olá, "+u.Name)),
		h.P(h.Text("Papéis: "+strings.Join(u.Roles, ", "))),
		// The token itself never reaches the page: what the browser needs to
		// know is that the call goes out with it.
		h.P(h.Text("As chamadas a /api/ saem com o token desta sessão.")),
	), nil
}
